/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	iofs "io/fs"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/snapshots"
	"github.com/google/go-containerregistry/pkg/name"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/client/snapshotter"
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/mc256/starlight/util/common"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type mountPoint struct {
	// active mount point for snapshots
	fs      *fs.Instance
	manager *Manager
	stack   int64

	// chainIDs that are using the mount point
	snapshots map[string]*snapshots.Info
}

type Client struct {
	ctx context.Context
	cfg *Configuration

	// Snapshotter
	snServer   *grpc.Server
	snListener net.Listener
	plugin     *snapshotter.Plugin

	// CLI
	cliServer   *grpc.Server
	cliListener net.Listener

	// layer
	layerMapLock sync.Mutex
	layerMap     map[string]*mountPoint

	// manager cache
	managerMapLock sync.Mutex
	managerMap     map[string]*Manager

	// Optimizer
	optimizerLock        sync.Mutex
	defaultOptimizer     bool
	defaultOptimizeGroup string
}

// -----------------------------------------------------------------------------
// Base Image Searching

// getImageFilter returns a filter string for containerd to find the image
// if completed is set to true, it will only return fully downloaded container images
func getImageFilter(ref string, completed bool) string {
	c := ""
	if completed {
		c = "," + "labels." + util.ContentLabelCompletion
	}
	return fmt.Sprintf(
		"name~=^%s,labels.%s==%s%s",
		// choose images with the same name (just the tags are different)
		regexp.QuoteMeta(ref),
		// choose images that are pulled by starlight
		util.ImageLabelPuller, "starlight",
		// choose completed images
		c,
	)
}

func getDistributionSource(cfg string) string {
	return fmt.Sprintf("starlight.mc256.dev/distribution.source.%s", cfg)
}

func (c *Client) findImage(ctr *containerd.Client, filter string) (img containerd.Image, err error) {
	var list []containerd.Image
	list, err = ctr.ListImages(c.ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	if len(list) == 1 {
		return list[0], nil
	}
	newest := list[0]
	nt := newest.Metadata().CreatedAt
	for _, i := range list {
		cur := i.Metadata().CreatedAt
		if cur.After(nt) {
			newest = i
			nt = cur
		}
	}
	// get the newest image
	return newest, nil
}

// FindBaseImage find the closest available image for the requested image, if user appointed an image, then this
// function will be used for confirming the appointed image is available in the local storage
func (c *Client) FindBaseImage(ctr *containerd.Client, base, ref string) (img containerd.Image, err error) {
	var baseFilter string
	if base == "" {
		sp := strings.Split(ref, ":")
		if len(sp) > 1 {
			tag := sp[len(sp)-1]
			baseFilter = strings.TrimSuffix(ref, tag)
			baseFilter = getImageFilter(baseFilter, true)
		} else {
			return nil, fmt.Errorf("invalid image reference: %s, missing tag", ref)
		}
	} else {
		baseFilter = getImageFilter(base, true)
	}

	img, err = c.findImage(ctr, baseFilter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find base image for %s", ref)
	}
	if img == nil && base != "" {
		return nil, fmt.Errorf("failed to find appointed base image %s", base)
	}

	return img, nil
}

// -----------------------------------------------------------------------------
// ScanExistingFilesystems scans place where the extracted file content is stored
// in case the file system has not extracted fully (without the `complete.json` file),
// we will remove the directory.
func (c *Client) ScanExistingFilesystems() {
	var (
		err                    error
		dir1, dir2, dir3, dir4 []iofs.FileInfo
		x1, x2, x3             bool
	)
	log.G(c.ctx).
		WithField("root", c.GetFilesystemRoot()).
		Debug("scanning existing filesystems")

	dir1, err = ioutil.ReadDir(filepath.Join(c.GetFilesystemRoot(), "layers"))
	if err != nil {
		return
	}
	for _, d1 := range dir1 {
		x1 = false
		if d1.IsDir() && len(d1.Name()) == 1 {
			dir2, err = ioutil.ReadDir(filepath.Join(c.GetFilesystemRoot(), "layers",
				d1.Name(),
			))
			if err != nil {
				continue
			}
			for _, d2 := range dir2 {
				x2 = false
				if d2.IsDir() && len(d2.Name()) == 2 {
					dir3, err = ioutil.ReadDir(filepath.Join(c.GetFilesystemRoot(), "layers",
						d1.Name(), d2.Name(),
					))
					if err != nil {
						continue
					}
					for _, d3 := range dir3 {
						x3 = false
						if d3.IsDir() && len(d3.Name()) == 2 {
							dir4, err = ioutil.ReadDir(filepath.Join(c.GetFilesystemRoot(), "layers",
								d1.Name(), d2.Name(), d3.Name(),
							))
							if err != nil {
								continue
							}
							for _, d4 := range dir4 {
								if d4.IsDir() {
									d := filepath.Join(c.GetFilesystemRoot(), "layers",
										d1.Name(), d2.Name(), d3.Name(), d4.Name(),
									)
									h := fmt.Sprintf("sha256:%s%s%s%s",
										d1.Name(), d2.Name(), d3.Name(), d4.Name(),
									)
									completeFile := filepath.Join(d, "completed.json")
									if _, err = os.Stat(completeFile); err != nil {
										_ = os.RemoveAll(filepath.Join(c.GetFilesystemRoot(), "layers",
											d1.Name(), d2.Name(), d3.Name(), d4.Name(),
										))
										log.G(c.ctx).WithField("digest", h).Warn("removed incomplete layer")
									} else {
										x1, x2, x3 = true, true, true
										c.AddCompletedLayers(h)
										log.G(c.ctx).WithField("digest", h).Debug("found layer")
									}
								}
							}
						}
						if !x3 {
							_ = os.RemoveAll(filepath.Join(c.GetFilesystemRoot(), "layers",
								d1.Name(), d2.Name(), d3.Name(),
							))
						}
					}
				}
				if !x2 {
					_ = os.RemoveAll(filepath.Join(c.GetFilesystemRoot(), "layers",
						d1.Name(), d2.Name(),
					))
				}
			}
		}
		if !x1 {
			_ = os.RemoveAll(filepath.Join(c.GetFilesystemRoot(), "layers",
				d1.Name(),
			))
		}
	}

	return
}

// -----------------------------------------------------------------------------
// Image Pulling

// readBody is a helper function to read the body of a response and return it in a buffer
func (c *Client) readBody(body io.ReadCloser, s int64) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(make([]byte, 0, s))
	m, err := io.CopyN(buf, body, s)
	if err != nil {
		return nil, err
	}
	if m != s {
		return nil, fmt.Errorf("failed to read body, expected %d bytes, got %d", s, m)
	}
	return buf, nil
}

// handleManifest unmarshal the manifest.
// It returns
//   - the manifest in object
//   - the manifest in bytes to store in the content store
//   - error in case of failure
func (c *Client) handleManifest(buf *bytes.Buffer) (manifest *v1.Manifest, b []byte, err error) {
	// decompress manifest
	r, err := gzip.NewReader(buf)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decompress manifest")
	}
	man, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read manifest")
	}
	err = json.Unmarshal(man, &manifest)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to unmarshal manifest")
	}

	return manifest, man, nil
}

// storeManifest saves the manifest in the content store with necessary labels
func (c *Client) storeManifest(cs content.Store, cfgName, d, ref, cfgd, sld string, man []byte) (err error) {
	pd := digest.Digest(d)

	// create content store
	err = content.WriteBlob(
		c.ctx, cs, pd.Hex(), bytes.NewReader(man),
		v1.Descriptor{Size: int64(len(man)), Digest: pd},
		content.WithLabels(map[string]string{
			// identifier
			util.ImageLabelPuller:               "starlight",
			util.ContentLabelStarlightMediaType: "manifest",

			// garbage collection
			fmt.Sprintf("%s.config", util.ContentLabelContainerdGC):    cfgd,
			fmt.Sprintf("%s.starlight", util.ContentLabelContainerdGC): sld,

			// multiple starlight proxy support
			getDistributionSource(cfgName): ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for manifest")
	}
	return nil
}

// updateManifest marks the manifest as completed
func (c *Client) updateManifest(ctr *containerd.Client, d string, chainIds []digest.Digest, t time.Time) (err error) {
	pd := digest.Digest(d)
	cs := ctr.ContentStore()

	var info content.Info

	info, err = cs.Info(c.ctx, pd)
	if err != nil {
		return err
	}

	info.Labels[util.ContentLabelCompletion] = t.Format(time.RFC3339)

	// garbage collection tags, more info:
	// https://github.com/containerd/containerd/blob/83f44ddab5b17da74c5bd97dad7b2c5fa32871de/docs/garbage-collection.md
	for idx, id := range chainIds {
		info.Labels[fmt.Sprintf("%s/%d", util.ContentLabelSnapshotGC, idx)] = id.String()
	}

	info, err = cs.Update(c.ctx, info)

	if err != nil {
		return errors.Wrapf(err, "failed to mark manifest as completed")
	}
	log.G(c.ctx).WithField("m", info.Digest).Info("download completed")
	return nil
}

func (c *Client) handleConfig(buf *bytes.Buffer) (config *v1.Image, b []byte, err error) {
	// decompress config
	r, err := gzip.NewReader(buf)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decompress config")
	}
	cfg, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read config")
	}
	err = json.Unmarshal(cfg, &config)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to unmarshal config")
	}

	return config, cfg, nil
}

func (c *Client) storeConfig(cs content.Store, cfgName, ref string, pd digest.Digest, cfg []byte) (err error) {
	// create content store

	err = content.WriteBlob(
		c.ctx, cs, pd.Hex(), bytes.NewReader(cfg),
		v1.Descriptor{Size: int64(len(cfg)), Digest: pd},
		content.WithLabels(map[string]string{
			util.ImageLabelPuller:               "starlight",
			util.ContentLabelStarlightMediaType: "config",
			getDistributionSource(cfgName):      ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for config")
	}
	return nil
}

func (c *Client) handleStarlightHeader(buf *bytes.Buffer) (header *Manager, h []byte, err error) {
	// decompress starlight header
	r, err := gzip.NewReader(buf)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to decompress starlight header")
	}
	h, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read starlight header")
	}
	err = json.Unmarshal(h, &header)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to unmarshal starlight header")
	}
	return header, h, nil
}

func (c *Client) storeStarlightHeader(cs content.Store, cfgName, ref, sld string, h []byte) (err error) {
	hd := digest.Digest(sld)

	// create content store
	err = content.WriteBlob(
		c.ctx, cs, hd.Hex(), bytes.NewReader(h),
		v1.Descriptor{Size: int64(len(h)), Digest: hd},
		content.WithLabels(map[string]string{
			util.ImageLabelPuller:               "starlight",
			util.ContentLabelStarlightMediaType: "starlight",
			getDistributionSource(cfgName):      ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for starlight header")
	}
	return nil
}

func (c *Client) Notify(proxyCfg string, reference name.Reference, insecure bool) error {
	pc, _ := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	// send message
	if err := p.Notify(reference, insecure); err != nil {
		return errors.Wrapf(err, "failed to notify proxy")
	}

	return nil
}

func (c *Client) Ping(proxyCfg string) (int64, string, string, error) {
	pc, _ := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	// send message
	rtt, proto, url, err := p.Ping()
	if err != nil {
		return -1, "", "", errors.Wrapf(err, "failed to ping proxy")
	}

	return rtt, proto, url, nil
}

func (c *Client) UploadTraces(proxyCfg string, tc *fs.TraceCollection) error {
	// connect to proxy
	pc, _ := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	// upload traces
	buf, err := json.Marshal(tc)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal trace collection")
	}

	err = p.Report(bytes.NewBuffer(buf))
	if err != nil {
		return errors.Wrapf(err, "failed to upload trace collection")
	}

	return nil
}

type PullFinishedMessage struct {
	img  *images.Image
	base string
	err  error
}

func (c *Client) pullImageSync(ctr *containerd.Client, base containerd.Image,
	ref, platform, proxyCfg string) (img *images.Image, err error) {
	msg := make(chan PullFinishedMessage)
	c.PullImage(ctr, base, ref, platform, proxyCfg, &msg)
	ret := <-msg
	return ret.img, ret.err
}

func (c *Client) pullImageGrpc(ns, base, ref, proxy string, ret *chan PullFinishedMessage) {
	// connect to containerd
	ctr, err := containerd.New(c.cfg.Containerd, containerd.WithDefaultNamespace(ns))
	if err != nil {
		*ret <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to connect to containerd")}
		return
	}
	defer ctr.Close()

	// find base image
	var baseImg containerd.Image
	baseImg, err = c.FindBaseImage(ctr, base, ref)
	if err != nil {
		*ret <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to identify base image")}
		return
	}

	// pull image
	log.G(c.ctx).WithFields(logrus.Fields{
		"ref": ref,
	}).Info("pulling image")
	c.PullImage(ctr, baseImg, ref, platforms.DefaultString(), proxy, ret)
}

// PullImage pulls an image from a registry and stores it in the content store
// it also stores the manager in memory.
// In case there exists another manager in memory, it removes it and re-pull the image
func (c *Client) PullImage(
	ctr *containerd.Client, base containerd.Image,
	ref, platform, proxyCfg string, ready *chan PullFinishedMessage,
) {
	// init vars
	is := ctr.ImageService()
	localCtx := context.Background()

	// check local image
	reqFilter := getImageFilter(ref, false)
	img, err := c.findImage(ctr, reqFilter)
	if err != nil {
		*ready <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to check requested image %s", ref)}
		return
	}

	if img != nil {
		labels := img.Labels()
		if _, has := labels[util.ContentLabelCompletion]; has {
			if _, err = c.LoadImage(ctr, img.Target().Digest); err != nil {
				*ready <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to load image %s", ref)}
				return
			}
			meta := img.Metadata()
			*ready <- PullFinishedMessage{&meta, "", fmt.Errorf("requested image %s already exists", ref)}
			return

		}
		log.G(c.ctx).
			WithField("image", ref).
			Info("requested image found but incomplete, remove and re-pull")

		// remove image
		if err = is.Delete(localCtx, ref); err != nil {
			log.G(c.ctx).
				WithField("image", ref).
				Info("failed to remove incomplete image")
			*ready <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to remove unfinished image %s", ref)}
			return
		}
	}

	// connect to proxy
	pc, pcn := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	baseRef := ""
	if base != nil {
		baseRef = fmt.Sprintf("%s@%s", base.Name(), base.Target().Digest)
	}

	// pull image
	body, mSize, cSize, sSize, md, sld, err := p.DeltaImage(baseRef, ref, platform)
	if err != nil {
		*ready <- PullFinishedMessage{nil, "", errors.Wrapf(err, "failed to pull image %s", ref)}
		return
	}
	defer func() {
		if body != nil {
			err = body.Close()
			if err != nil {
				log.G(c.ctx).WithError(err).Errorf("failed to close body")
			}
		}
	}()

	log.G(c.ctx).
		WithField("manifest", mSize).
		WithField("config", cSize).
		WithField("starlight", sSize).
		WithField("digest", md).
		WithField("sl_digest", sld).
		Infof("pulling image %s", ref)

	// 1. check manager in memory, if it exists, remove it and re-pull the image
	// (This behavior is different from LoadImage() which does not remove the manager in memory)
	c.managerMapLock.Lock()
	defer func() {
		if _, ok := c.managerMap[md]; !ok {
			// something went wrong, the lock has not been released, unlock it
			c.managerMapLock.Unlock()
		}
	}()
	if _, ok := c.managerMap[md]; ok {
		log.G(c.ctx).
			WithField("manifest", mSize).
			WithField("sl_digest", sld).
			Warn("found in cache. remove and re-pull")
		delete(c.managerMap, md)
	}

	// check if the image is in containerd's image pool
	var (
		buf         *bytes.Buffer
		man, con    []byte
		ctrImg      images.Image
		manifest    *v1.Manifest
		imageConfig *v1.Image
	)

	// 2. load manifest, config, and starlight header

	cs := ctr.ContentStore()

	// manifest
	buf, err = c.readBody(body, mSize)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to read manifest")}
		return
	}
	manifest, man, err = c.handleManifest(buf)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to handle manifest")}
		return
	}
	err = c.storeManifest(cs, pcn, md, ref,
		manifest.Config.Digest.String(), sld,
		man)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to store manifest")}
		return
	}

	// config
	buf, err = c.readBody(body, cSize)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to read config")}
		return
	}
	imageConfig, con, err = c.handleConfig(buf)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to handle config")}
		return
	}
	err = c.storeConfig(cs, pcn, ref, manifest.Config.Digest, con)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to store config")}
		return
	}

	// starlight header
	buf, err = c.readBody(body, sSize)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to read starlight header")}
		return
	}
	star, sta, err := c.handleStarlightHeader(buf)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to handle starlight header")}
		return
	}
	err = c.storeStarlightHeader(cs, pcn, ref, sld, sta)
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to store starlight header")}
		return
	}

	// create image
	mdd := digest.Digest(md)
	ctrImg, err = is.Create(localCtx, images.Image{
		Name: ref,
		Target: v1.Descriptor{
			MediaType: util.ImageMediaTypeManifestV2,
			Digest:    mdd,
			Size:      int64(len(man)),
		},
		Labels: map[string]string{
			util.ImageLabelPuller:            "starlight",
			util.ImageLabelStarlightMetadata: sld,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	if err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to create image %s", ref)}
		return
	}

	log.G(c.ctx).
		WithField("image", ctrImg.Name).
		Debugf("created image")

	// 3. create manager
	// keep going and download layers
	star.Init(ctr, c, c.ctx, c.cfg, false, manifest, imageConfig, mdd)

	// create manager
	c.managerMap[md] = star
	log.G(c.ctx).
		WithField("manifest", md).
		Info("client: added manager")
	c.managerMapLock.Unlock()

	// check optimizer
	// we should set optimizer before creating the filesystems
	if c.defaultOptimizer {
		if _, err = star.SetOptimizerOn(c.defaultOptimizeGroup); err != nil {
			log.G(c.ctx).
				WithError(err).
				Error("failed to set optimizer on")
			*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to enable optimizer")}
			return
		}
	}

	if err = star.PrepareDirectories(c); err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to initialize directories")}
		return
	}

	// 4. update in-memory layer map
	// Create Snapshots
	// should unlock the managerMapLock before calling CreateSnapshot
	var chainIds []digest.Digest
	if chainIds, err = star.CreateSnapshots(c); err != nil {
		log.G(c.ctx).
			WithError(err).
			Error("failed to create snapshots")
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to create snapshots")}
		return
	}

	// ------------------------------------------------------------------------------------
	// LoadImage() does not have the following code
	//
	// 5. Send signal
	// Image is ready (content is still on the way)
	// close(*ready)
	*ready <- PullFinishedMessage{&ctrImg, baseRef, nil}

	// 6. Extract file content
	// download content
	log.G(c.ctx).
		WithField("m", md).
		Info("start decompressing content")
	if err = star.Extract(&body); err != nil {
		*ready <- PullFinishedMessage{nil, baseRef, errors.Wrapf(err, "failed to extract starlight image")}
		return
	}
	log.G(c.ctx).
		WithField("m", md).
		Info("content decompression completed")

	// 7. Mark image as completed
	// mark as completed
	t := time.Now()

	// mark image as completed
	ctrImg.Labels[util.ContentLabelCompletion] = t.Format(time.RFC3339)
	if ctrImg, err = is.Update(localCtx, ctrImg, "labels."+util.ContentLabelCompletion); err != nil {
		log.G(c.ctx).
			WithError(err).
			Error("failed to mark image as completed")
		return
	}

	// update garbage collection labels
	if err = c.updateManifest(ctr, md, chainIds, t); err != nil {
		log.G(c.ctx).
			WithError(err).
			Error("failed to update manifest")
		return
	}

	return
}

// LoadImage loads image manifest from content store to the memory,
// if it is in memory, return manager directly.
//
// This method should not use any snapshotter methods to avoid recursive lock.
// This method is similar to PullImage, but it only uses content store.
func (c *Client) LoadImage(ctr *containerd.Client, manifest digest.Digest) (manager *Manager, err error) {

	var (
		buf  []byte
		man  *v1.Manifest
		cfg  *v1.Image
		ii   content.Info
		star Manager
	)

	// 1. check manager in memory
	c.managerMapLock.Lock()
	defer c.managerMapLock.Unlock()

	if m, ok := c.managerMap[manifest.String()]; ok {
		return m, nil
	}

	// no cache, load from store
	cs := ctr.ContentStore()
	ii, err = cs.Info(c.ctx, manifest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get manifest info")
	}

	if len(ii.Labels[util.ContentLabelCompletion]) == 0 {
		log.G(c.ctx).WithField("d", manifest.String()).Warn("using incomplete image")
		//return nil, errors.Errorf("incomplete image, remove and repull")
	}

	starlight := digest.Digest(ii.Labels[fmt.Sprintf("%s.starlight", util.ContentLabelContainerdGC)])

	// 2. load manifest, config, and starlight header
	if buf, err = content.ReadBlob(c.ctx, cs, v1.Descriptor{Digest: manifest}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &man); err != nil {
		return nil, err
	}

	if buf, err = content.ReadBlob(c.ctx, cs, v1.Descriptor{Digest: starlight}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &star); err != nil {
		return nil, err
	}

	if buf, err = content.ReadBlob(c.ctx, cs, v1.Descriptor{Digest: man.Config.Digest}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &cfg); err != nil {
		return nil, err
	}

	// 3. create manager
	star.Init(ctr, c, c.ctx, c.cfg, true, man, cfg, manifest)

	// save to cache
	c.managerMap[manifest.String()] = &star
	log.G(c.ctx).
		WithField("manifest", manifest.String()).
		Info("client: added manager")

	// set optimizer
	if c.defaultOptimizer {
		if _, err = star.SetOptimizerOn(c.defaultOptimizeGroup); err != nil {
			return nil, errors.Wrapf(err, "failed to enable optimizer")
		}
	}

	// 4. update in-memory layer map
	// update layerMap
	c.layerMapLock.Lock()
	defer c.layerMapLock.Unlock()

	for idx, serial := range star.stackSerialMap {
		layer := star.layers[serial]
		if mp, has := c.layerMap[layer.Hash]; has {
			if mp.manager == nil {
				mp.manager = &star
				mp.stack = int64(idx)
			}
		} else {
			c.layerMap[layer.Hash] = &mountPoint{
				fs:        nil,
				manager:   &star,
				stack:     int64(idx),
				snapshots: make(map[string]*snapshots.Info),
			}
		}
	}

	return &star, nil
}

func (c *Client) Close() {
	// snapshotter server
	if c.snServer != nil {
		c.snServer.Stop()
	}
	// CLI server
	if c.cliServer != nil {
		c.cliServer.Stop()
	}

	// filesystems
	c.managerMapLock.Lock()
	defer c.managerMapLock.Unlock()
	for _, mg := range c.managerMap {
		log.G(c.ctx).
			WithField("manager", mg.manifestDigest.String()).
			Debugf("client: closing manager")
		mg.Teardown()
	}

	os.Exit(1)
}

// -----------------------------------------------------------------------------
// Operator interface

func (c *Client) GetFilesystemRoot() string {
	return c.cfg.FileSystemRoot
}

func (c *Client) AddCompletedLayers(compressedLayerDigest string) {
	c.layerMapLock.Lock()
	defer c.layerMapLock.Unlock()

	if _, has := c.layerMap[compressedLayerDigest]; !has {
		c.layerMap[compressedLayerDigest] = &mountPoint{
			fs:        nil,
			manager:   nil,
			stack:     -1,
			snapshots: make(map[string]*snapshots.Info),
		}
	}
}

// -----------------------------------------------------------------------------
// Plugin interface

func (c *Client) GetFilesystemPath(cd string) string {
	return filepath.Join(c.cfg.FileSystemRoot, "layers", cd[7:8], cd[8:10], cd[10:12], cd[12:])
}

func (c *Client) GetMountingPoint(ssId string) string {
	return filepath.Join(c.cfg.FileSystemRoot, "mnt", ssId)
}

func (c *Client) getStarlightFS(ssId string) string {
	return filepath.Join(c.GetFilesystemPath(ssId), "slfs")
}

func (c *Client) PrepareManager(namespace string, manifest digest.Digest) error {
	client, err := containerd.New(c.cfg.Containerd, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to containerd")
	}
	defer client.Close()

	_, err = c.LoadImage(client, manifest)
	if err != nil {
		return errors.Wrapf(err, "failed to load image")
	}
	return nil
}

// Mount returns the mountpoint for the given snapshot
// - md: manifest digest
// - ld: uncompressed layer digest
// - ssId: snapshot id
func (c *Client) Mount(ld digest.Digest, ssId string, sn *snapshots.Info) (mnt string, err error) {
	c.layerMapLock.Lock()
	defer c.layerMapLock.Unlock()

	if mp, has := c.layerMap[ld.String()]; has {
		// fs != nil, fs has already created
		if mp.fs != nil {
			mp.snapshots[sn.Name] = sn
			log.G(c.ctx).
				WithField("s", ssId).
				WithField("m", mp.manager.manifestDigest.String()).
				Debugf("mount: found fs")
			return mp.fs.GetMountPoint(), nil
		}
		// manager != nil but fs == nil
		// manager has been created but not yet mounted
		if mp.manager != nil {
			// create mounting point
			mnt = filepath.Join(c.GetMountingPoint(ssId), "slfs")
			mp.fs, err = mp.manager.NewStarlightFS(mnt, mp.stack, &fusefs.Options{}, false)
			if err != nil {
				return "", errors.Wrapf(err, "failed to mount filesystem")
			}

			// create filesystem and serve
			go mp.fs.Serve()
			mp.snapshots[sn.Name] = sn
			log.G(c.ctx).
				WithField("s", ssId).
				WithField("m", mp.manager.manifestDigest.String()).
				Debugf("mount: found manager")
			return mnt, nil
		}

		log.G(c.ctx).
			WithField("s", ssId).
			WithField("ld", ld.String()).
			Warn("mount: no manager found")

		return "", common.ErrNoManager
	} else {
		// if it does not exist,
		// it means the layer is neither in the process of downloading nor extracted

		return "", errors.New("layer incomplete or not found. remove and pull again")
	}

}

func (c *Client) Unmount(cd, sn string) error {
	c.layerMapLock.Lock()
	defer c.layerMapLock.Unlock()

	// found the layer
	layer, has := c.layerMap[cd]
	if !has {
		return nil
	}

	// found the snapshot
	if _, has = layer.snapshots[sn]; !has {
		return nil
	}

	// if there exists other snapshots, do not remove the layer
	delete(layer.snapshots, sn)
	if len(layer.snapshots) > 0 {
		return nil
	}

	// otherwise, remove the layer
	if layer.fs == nil {
		return nil
	}

	log.G(c.ctx).
		WithField("d", cd).
		WithField("mnt", layer.fs.GetMountPoint()).
		Debug("fs: unmount")

	if err := layer.fs.Teardown(); err != nil {
		return err
	}

	// remove the mounting directory
	_ = os.RemoveAll(layer.fs.GetMountPoint())

	return nil
}

// -----------------------------------------------------------------------------
// Snapshotter related

// InitSnapshotter initializes the snapshotter service
func (c *Client) InitSnapshotter() (err error) {
	log.G(c.ctx).
		Debug("starlight snapshotter service starting")
	c.snServer = grpc.NewServer()

	c.plugin, err = snapshotter.NewPlugin(c.ctx, c, c.cfg.Metadata)
	if err != nil {
		return errors.Wrapf(err, "failed to create snapshotter")
	}

	svc := snapshotservice.FromSnapshotter(c.plugin)
	if err = os.MkdirAll(filepath.Dir(c.cfg.Socket), 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q for socket", filepath.Dir(c.cfg.Socket))
	}

	// Try to remove the socket file to avoid EADDRINUSE
	if err = os.RemoveAll(c.cfg.Socket); err != nil {
		return errors.Wrapf(err, "failed to remove %q", c.cfg.Socket)
	}

	snapshotsapi.RegisterSnapshotsServer(c.snServer, svc)
	return nil
}

// StartSnapshotter starts the snapshotter service, should be run in a goroutine
func (c *Client) StartSnapshotter() {
	// Listen and serve
	var err error
	c.snListener, err = net.Listen("unix", c.cfg.Socket)
	if err != nil {
		log.G(c.ctx).WithError(err).Errorf("failed to listen on %q", c.cfg.Socket)
		return
	}

	log.G(c.ctx).
		WithField("socket", c.cfg.Socket).
		Info("starlight snapshotter service started")

	if err = c.snServer.Serve(c.snListener); err != nil {
		log.G(c.ctx).WithError(err).Errorf("failed to serve snapshotter")
		return
	}
}

// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// CLI gRPC server

type StarlightDaemonAPIServer struct {
	pb.UnimplementedDaemonServer
	client *Client
}

func (s *StarlightDaemonAPIServer) GetVersion(ctx context.Context, req *pb.Request) (*pb.Version, error) {
	return &pb.Version{
		Version: util.Version,
	}, nil
}

func (s *StarlightDaemonAPIServer) AddProxyProfile(ctx context.Context, req *pb.AuthRequest) (*pb.AuthResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"protocol": req.Protocol,
		"address":  req.Address,
		"username": req.Username,
	}).Debug("grpc: add proxy profile")

	s.client.cfg.Proxies[req.ProfileName] = &ProxyConfig{
		Protocol: req.Protocol,
		Address:  req.Address,
		Username: req.Username,
		Password: req.Password,
	}
	if err := s.client.cfg.SaveConfig(); err != nil {
		log.G(s.client.ctx).WithError(err).Errorf("failed to save config")
		return &pb.AuthResponse{
			Success: false,
			Message: "failed to save config",
		}, nil
	}
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"protocol": req.Protocol,
		"address":  req.Address,
		"username": req.Username,
	}).Info("add auth profile")
	return &pb.AuthResponse{
		Success: true,
	}, nil
}

func (s *StarlightDaemonAPIServer) GetProxyProfiles(ctx context.Context, req *pb.Request) (resp *pb.GetProxyProfilesResponse, err error) {
	log.G(s.client.ctx).Debug("grpc: get proxy profiles")

	profiles := []*pb.GetProxyProfilesResponse_Profile{}
	for name, proxy := range s.client.cfg.Proxies {
		profiles = append(profiles, &pb.GetProxyProfilesResponse_Profile{
			Name:     name,
			Protocol: proxy.Protocol,
			Address:  proxy.Address,
		})
	}
	return &pb.GetProxyProfilesResponse{
		Profiles: profiles,
	}, nil
}

func (s *StarlightDaemonAPIServer) PullImage(ctx context.Context, ref *pb.ImageReference) (resp *pb.ImagePullResponse, err error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"base":   ref.Base,
		"ref":    ref.Reference,
		"socket": s.client.cfg.Containerd,
	}).Debug("grpc: pull image")

	ns := ref.Namespace
	if ns == "" {
		ns = s.client.cfg.Namespace
	}

	ready := make(chan PullFinishedMessage)
	go s.client.pullImageGrpc(ns, ref.Base, ref.Reference, ref.ProxyConfig, &ready)
	ret := <-ready

	if ret.err != nil {
		if ret.img != nil {
			return &pb.ImagePullResponse{
				Success:   true,
				Message:   ret.err.Error(),
				BaseImage: ret.base,
			}, nil
		} else {
			return &pb.ImagePullResponse{
				Success:   false,
				Message:   ret.err.Error(),
				BaseImage: ret.base,
			}, nil
		}
	}

	return &pb.ImagePullResponse{Success: true, Message: "ok", BaseImage: ret.base}, nil
}

func (s *StarlightDaemonAPIServer) SetOptimizer(ctx context.Context, req *pb.OptimizeRequest) (*pb.OptimizeResponse, error) {
	okRes, failRes := make(map[string]string), make(map[string]string)
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"enable": req.Enable,
	}).Debug("grpc: set optimizer")

	s.client.optimizerLock.Lock()
	defer s.client.optimizerLock.Unlock()

	s.client.managerMapLock.Lock()
	defer s.client.managerMapLock.Unlock()

	if req.Enable {

		s.client.defaultOptimizer = true
		s.client.defaultOptimizeGroup = req.Group

		for d, m := range s.client.managerMap {
			if st, err := m.SetOptimizerOn(req.Group); err != nil {
				log.G(s.client.ctx).
					WithField("group", req.Group).
					WithField("md", d).
					WithField("start", st).
					WithError(err).
					Error("failed to set optimizer on")
				failRes[d] = err.Error()
			} else {
				okRes[d] = time.Now().Format(time.RFC3339)
			}
		}

	} else {

		s.client.defaultOptimizer = false
		s.client.defaultOptimizeGroup = ""

		for d, m := range s.client.managerMap {
			if et, err := m.SetOptimizerOff(); err != nil {
				log.G(s.client.ctx).
					WithField("md", d).
					WithField("duration", et).
					WithError(err).
					Error("failed to set optimizer off")
				failRes[d] = err.Error()
			} else {
				okRes[d] = fmt.Sprintf("collected %.3fs file access traces", et.Seconds())
			}
		}
	}

	return &pb.OptimizeResponse{
		Success: true,
		Message: "completed request",
		Okay:    okRes,
		Failed:  failRes,
	}, nil
}

func (s *StarlightDaemonAPIServer) ReportTraces(ctx context.Context, req *pb.ReportTracesRequest) (*pb.ReportTracesResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: report")

	tc, err := fs.NewTraceCollection(s.client.ctx, s.client.cfg.TracesDir)
	if err != nil {
		return &pb.ReportTracesResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	err = s.client.UploadTraces(req.ProxyConfig, tc)
	if err != nil {
		return &pb.ReportTracesResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ReportTracesResponse{
		Success: true,
		Message: "uploaded traces",
	}, nil
}

func (s *StarlightDaemonAPIServer) NotifyProxy(ctx context.Context, req *pb.NotifyRequest) (*pb.NotifyResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: notify")

	reference, err := name.ParseReference(req.Reference)
	if err != nil {
		return &pb.NotifyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	err = s.client.Notify(req.ProxyConfig, reference, true)
	if err != nil {
		return &pb.NotifyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.NotifyResponse{
		Success: true,
		Message: reference.String(),
	}, nil
}

func (s *StarlightDaemonAPIServer) PingTest(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: ping test")

	rtt, proto, server, err := s.client.Ping(req.ProxyConfig)
	if err != nil {
		return &pb.PingResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.PingResponse{
		Success: true,
		Message: fmt.Sprintf("ok! - %s://%s", proto, server),
		Latency: rtt,
	}, nil
}

func newStarlightDaemonAPIServer(client *Client) *StarlightDaemonAPIServer {
	c := &StarlightDaemonAPIServer{client: client}
	return c
}

func (c *Client) InitCLIServer() (err error) {
	log.G(c.ctx).
		Debug("starlight CLI service starting")
	c.cliServer = grpc.NewServer()

	if strings.HasPrefix(c.cfg.DaemonType, "unix") {
		if err = os.MkdirAll(filepath.Dir(c.cfg.Daemon), 0700); err != nil {
			return errors.Wrapf(err, "failed to create directory %q for socket", filepath.Dir(c.cfg.Daemon))
		}

		// Try to remove the socket file to avoid EADDRINUSE
		if err = os.RemoveAll(c.cfg.Daemon); err != nil {
			return errors.Wrapf(err, "failed to remove %q", c.cfg.Daemon)
		}
	}

	pb.RegisterDaemonServer(c.cliServer, newStarlightDaemonAPIServer(c))
	return nil
}

func (c *Client) StartCLIServer() {
	// Listen and serve
	var err error
	c.cliListener, err = net.Listen(c.cfg.DaemonType, c.cfg.Daemon)
	if err != nil {
		log.G(c.ctx).WithError(err).Errorf("failed to listen on %s using %s protocol", c.cfg.DaemonType, c.cfg.Daemon)
		return
	}

	log.G(c.ctx).
		WithField(c.cfg.DaemonType, c.cfg.Daemon).
		Info("starlight CLI service started")

	if err := c.cliServer.Serve(c.cliListener); err != nil {
		log.G(c.ctx).WithError(err).Errorf("failed to serve CLI server")
		return
	}
}

// -----------------------------------------------------------------------------

func NewClient(ctx context.Context, cfg *Configuration) (c *Client, err error) {
	c = &Client{
		ctx: ctx,
		cfg: cfg,

		layerMap:   make(map[string]*mountPoint),
		managerMap: make(map[string]*Manager),
	}

	// scan existing filesystems
	c.ScanExistingFilesystems()

	return c, nil
}
