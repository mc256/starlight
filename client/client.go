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
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	cs  content.Store

	// containerd
	client *containerd.Client

	// Snapshotter
	snServer   *grpc.Server
	snListener net.Listener

	// CLI
	cliServer   *grpc.Server
	cliListener net.Listener

	operator *snapshotter.Operator
	plugin   *snapshotter.Plugin

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

func escapeSlashes(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "/", "\\/")
}

func getImageFilter(ref string) string {
	return fmt.Sprintf(
		"name~=/^%s.*/,labels.%s==%s,%s",
		// choose images with the same name (just the tags are different)
		escapeSlashes(ref),
		// choose images that are pulled by starlight
		util.ImageLabelPuller, "starlight",
		// choose completed images
		"labels."+util.ContentLabelCompletion,
	)
}

func getDistributionSource(cfg string) string {
	return fmt.Sprintf("starlight.mc256.dev/distribution.source.%s", cfg)
}

func (c *Client) findImage(filter string) (img containerd.Image, err error) {
	var list []containerd.Image
	list, err = c.client.ListImages(c.ctx, filter)
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
func (c *Client) FindBaseImage(base, ref string) (img containerd.Image, err error) {
	var baseFilter string
	if base == "" {
		baseFilter = strings.Split(ref, ":")[0]
		if baseFilter == "" {
			return nil, fmt.Errorf("invalid image reference: %s, missing tag", ref)
		}
		baseFilter = getImageFilter(baseFilter)
	} else {
		baseFilter = getImageFilter(base)
	}

	img, err = c.findImage(baseFilter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find base image for %s", ref)
	}
	if img == nil && base != "" {
		return nil, fmt.Errorf("failed to find appointed base image %s", base)
	}

	return img, nil
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
//  - the manifest in object
//  - the manifest in bytes to store in the content store
//  - error in case of failure
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
func (c *Client) storeManifest(cfgName, d, ref, cfgd, sld string, man []byte) (err error) {
	pd := digest.Digest(d)

	// create content store
	err = content.WriteBlob(
		c.ctx, c.cs, pd.Hex(), bytes.NewReader(man),
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
func (c *Client) updateManifest(d string, chainIds []digest.Digest, t time.Time) (err error) {
	pd := digest.Digest(d)
	cs := c.client.ContentStore()

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
	log.G(c.ctx).WithField("digest", info.Digest).Info("download completed")
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

func (c *Client) storeConfig(cfgName, ref string, pd digest.Digest, cfg []byte) (err error) {
	// create content store

	err = content.WriteBlob(
		c.ctx, c.cs, pd.Hex(), bytes.NewReader(cfg),
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

func (c *Client) storeStarlightHeader(cfgName, ref, sld string, h []byte) (err error) {
	hd := digest.Digest(sld)

	// create content store
	err = content.WriteBlob(
		c.ctx, c.cs, hd.Hex(), bytes.NewReader(h),
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

func (c *Client) Notify(proxyCfg string, reference name.Reference) error {
	pc, _ := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	// send message
	if err := p.Notify(reference); err != nil {
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

// PullImage pulls an image from a registry and stores it in the content store
// it also stores the manager in memory.
// In case there exists another manager in memory, it removes it and re-pull the image
func (c *Client) PullImage(base containerd.Image, ref, platform, proxyCfg string, ready *chan bool) (img containerd.Image, err error) {
	// check local image
	reqFilter := getImageFilter(ref)
	img, err = c.findImage(reqFilter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check requested image %s", ref)
	}
	if img != nil {
		return nil, fmt.Errorf("requested image %s already exists", ref)
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
		return nil, errors.Wrapf(err, "failed to pull image %s", ref)
	}
	defer body.Close()

	log.G(c.ctx).
		WithField("manifest", mSize).
		WithField("config", cSize).
		WithField("starlight", sSize).
		WithField("digest", md).
		WithField("sl_digest", sld).
		Infof("pulling image %s", ref)

	c.managerMapLock.Lock()
	release := false
	defer func() {
		if !release {
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

	var (
		buf *bytes.Buffer

		man, con []byte

		ctrImg      images.Image
		manifest    *v1.Manifest
		imageConfig *v1.Image
	)

	// manifest
	buf, err = c.readBody(body, mSize)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read manifest")
	}
	manifest, man, err = c.handleManifest(buf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to handle manifest")
	}
	err = c.storeManifest(pcn, md, ref,
		manifest.Config.Digest.String(), sld,
		man)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to store manifest")
	}

	// config
	buf, err = c.readBody(body, cSize)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config")
	}
	imageConfig, con, err = c.handleConfig(buf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to handle config")
	}
	err = c.storeConfig(pcn, ref, manifest.Config.Digest, con)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to store config")
	}

	// starlight header
	buf, err = c.readBody(body, sSize)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read starlight header")
	}
	star, sta, err := c.handleStarlightHeader(buf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to handle starlight header")
	}
	err = c.storeStarlightHeader(pcn, ref, sld, sta)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to store starlight header")
	}

	// create image
	mdd := digest.Digest(md)
	is := c.client.ImageService()
	ctrImg, err = is.Create(c.ctx, images.Image{
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
	log.G(c.ctx).WithField("image", ctrImg.Name).Debugf("created image")

	// keep going and download layers
	star.Init(c.ctx, c.cfg, false, manifest, imageConfig, mdd)

	// create manager
	c.managerMap[md] = star
	log.G(c.ctx).WithField("manifest", md).Debugf("client: added manager")
	release = true
	c.managerMapLock.Unlock()

	// check optimizer
	// we should set optimizer before creating the filesystems
	if c.defaultOptimizer {
		if err = star.SetOptimizerOn(c.defaultOptimizeGroup); err != nil {
			return nil, errors.Wrapf(err, "failed to enable optimizer")
		}
	}

	if err = star.PrepareDirectories(c); err != nil {
		return nil, errors.Wrapf(err, "failed to initialize directories")
	}

	// Create Snapshots
	// should unlock the managerMapLock before calling CreateSnapshot
	var chainIds []digest.Digest
	if chainIds, err = star.CreateSnapshots(c); err != nil {
		return nil, errors.Wrapf(err, "failed to create snapshots")
	}

	// Image is ready (content is still on the way)
	close(*ready)

	// download content
	if err = star.Extract(&body); err != nil {
		return nil, errors.Wrapf(err, "failed to extract starlight image")
	}

	// mark as completed
	t := time.Now()

	// mark image as completed
	ctrImg.Labels[util.ContentLabelCompletion] = t.Format(time.RFC3339)
	if ctrImg, err = is.Update(c.ctx, ctrImg, "labels."+util.ContentLabelCompletion); err != nil {
		return nil, errors.Wrapf(err, "failed to mark image as completed")
	}

	// update garbage collection labels
	if err = c.updateManifest(md, chainIds, t); err != nil {
		return nil, errors.Wrapf(err, "failed to update manifest")
	}

	return
}

// LoadImage loads image manifest from content store to the memory,
// if it is in memory, return manager directly.
//
// This method should not use any snapshotter methods to avoid recursive lock.
func (c *Client) LoadImage(manifest digest.Digest) (manager *Manager, err error) {

	var (
		buf  []byte
		man  *v1.Manifest
		cfg  *v1.Image
		ii   content.Info
		star Manager
	)

	// manager cache
	c.managerMapLock.Lock()
	defer c.managerMapLock.Unlock()

	if m, ok := c.managerMap[manifest.String()]; ok {
		return m, nil
	}

	// no cache, load from store
	cs := c.client.ContentStore()
	ii, err = cs.Info(c.ctx, manifest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get manifest info")
	}

	if len(ii.Labels[util.ContentLabelCompletion]) == 0 {
		log.G(c.ctx).WithField("d", manifest.String()).Warn("using incomplete image")
		//return nil, errors.Errorf("incomplete image, remove and repull")
	}

	starlight := digest.Digest(ii.Labels[fmt.Sprintf("%s.starlight", util.ContentLabelContainerdGC)])

	if buf, err = content.ReadBlob(c.ctx, c.cs, v1.Descriptor{Digest: manifest}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &man); err != nil {
		return nil, err
	}

	if buf, err = content.ReadBlob(c.ctx, c.cs, v1.Descriptor{Digest: starlight}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &star); err != nil {
		return nil, err
	}

	if buf, err = content.ReadBlob(c.ctx, c.cs, v1.Descriptor{Digest: man.Config.Digest}); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf, &cfg); err != nil {
		return nil, err
	}

	star.Init(c.ctx, c.cfg, true, man, cfg, manifest)

	// save to cache
	c.managerMap[manifest.String()] = &star
	log.G(c.ctx).WithField("manifest", manifest.String()).Debugf("client: added manager")

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
	// containerd client
	_ = c.client.Close()

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

func (c *Client) PrepareManager(manifest digest.Digest) (err error) {
	_, err = c.LoadImage(manifest)
	return
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

func (s *StarlightDaemonAPIServer) Version(ctx context.Context, req *pb.Request) (*pb.Version, error) {
	return &pb.Version{
		Version: util.Version,
	}, nil
}

func (s *StarlightDaemonAPIServer) AddProxyProfile(ctx context.Context, req *pb.AuthRequest) (*pb.AuthResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"protocol": req.Protocol,
		"address":  req.Address,
		"username": req.Username,
	}).Trace("grpc: add proxy profile")

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

func (s *StarlightDaemonAPIServer) PullImage(ctx context.Context, ref *pb.ImageReference) (*pb.ImagePullResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"base": ref.Base,
		"ref":  ref.Reference,
	}).Trace("grpc: pull image")

	base, err := s.client.FindBaseImage(ref.Base, ref.Reference)
	if err != nil {
		return &pb.ImagePullResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.G(s.client.ctx).WithFields(logrus.Fields{
		"ref": ref.Reference,
	}).Info("pulling image")

	ready := make(chan bool)
	go func() {
		_, err = s.client.PullImage(
			base, ref.Reference,
			platforms.DefaultString(), ref.ProxyConfig,
			&ready)
		if err != nil {
			log.G(s.client.ctx).WithError(err).Errorf("failed to pull image")
			close(ready)
			return
		}
	}()

	<-ready

	baseImage := ""
	if base != nil {
		baseImage = fmt.Sprintf("%s@%s", base.Name(), base.Target().Digest)
	}

	if err != nil {
		return &pb.ImagePullResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ImagePullResponse{Success: true, Message: "ok", BaseImage: baseImage}, nil
}

func (s *StarlightDaemonAPIServer) SetOptimizer(ctx context.Context, req *pb.OptimizeRequest) (*pb.OptimizeResponse, error) {
	okRes, failRes := make(map[string]string), make(map[string]string)
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"enable": req.Enable,
	}).Trace("grpc: set optimizer")

	if req.Enable {
		s.client.optimizerLock.Lock()
		defer s.client.optimizerLock.Unlock()

		s.client.defaultOptimizer = true
		s.client.defaultOptimizeGroup = req.Group

		s.client.layerMapLock.Lock()
		defer s.client.layerMapLock.Unlock()

		for _, layer := range s.client.layerMap {
			if err := layer.manager.SetOptimizerOn(req.Group); err == nil {
				okRes[layer.manager.manifestDigest.String()] = time.Now().Format(time.RFC3339)
			} else {
				failRes[layer.manager.manifestDigest.String()] = err.Error()
			}
		}
	} else {
		s.client.optimizerLock.Lock()
		defer s.client.optimizerLock.Unlock()

		s.client.defaultOptimizer = false
		s.client.defaultOptimizeGroup = ""

		s.client.layerMapLock.Lock()
		defer s.client.layerMapLock.Unlock()

		for _, layer := range s.client.layerMap {
			if err := layer.manager.SetOptimizerOff(); err == nil {
				okRes[layer.manager.manifestDigest.String()] = time.Now().Format(time.RFC3339)
			} else {
				failRes[layer.manager.manifestDigest.String()] = err.Error()
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
	}).Trace("grpc: report")

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
	}).Trace("grpc: notify")

	reference, err := name.ParseReference(req.Reference)
	if err != nil {
		return &pb.NotifyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	err = s.client.Notify(req.Reference, reference)
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
	}).Trace("grpc: ping test")

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
		ctx:    ctx,
		cfg:    cfg,
		client: nil,

		layerMap:   make(map[string]*mountPoint),
		managerMap: make(map[string]*Manager),
	}

	// containerd client
	c.client, err = containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		return nil, err
	}

	// content store
	c.cs = c.client.ContentStore()
	c.operator = snapshotter.NewOperator(c.ctx, c, c.client.SnapshotService("starlight"))

	// scan existing filesystems
	c.operator.ScanExistingFilesystems()

	return c, nil
}
