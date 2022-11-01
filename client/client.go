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
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	ctx    context.Context
	cfg    *Configuration
	client *containerd.Client
	server *grpc.Server
	sn     *Snapshotter
	cs     content.Store
}

func escapeSlashes(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "/", "\\/")
}

func getImageFilter(ref string) string {
	return fmt.Sprintf(
		"name~=/^%s.*/,labels.%s==%s",
		escapeSlashes(ref),
		util.ImageLabelPuller, "starlight",
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
	return newest, nil
}

// FindBaseImage find the closest available image for the requested image, if user appointed an image, then this
// function will be used for looking up the appointed image
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

func (c *Client) storeManifest(cfgName, d, ref, cfgd, sld string, man []byte) (err error) {
	pd := digest.Digest(d)

	// create content store

	err = content.WriteBlob(
		c.ctx, c.cs, pd.Hex(), bytes.NewReader(man),
		v1.Descriptor{Size: int64(len(man)), Digest: pd},
		content.WithLabels(map[string]string{
			util.ImageLabelPuller:                                      "starlight",
			util.ContentLabelStarlightMediaType:                        "manifest",
			fmt.Sprintf("%s.config", util.ContentLabelContainerdGC):    cfgd,
			fmt.Sprintf("%s.starlight", util.ContentLabelContainerdGC): sld,
			getDistributionSource(cfgName):                             ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for manifest")
	}
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
	is := c.client.ImageService()
	ctrImg, err = is.Create(c.ctx, images.Image{
		Name: ref,
		Target: v1.Descriptor{
			MediaType: util.ImageMediaTypeManifestV2,
			Digest:    digest.Digest(md),
			Size:      int64(len(man)),
		},
		Labels: map[string]string{
			util.ImageLabelPuller:            "starlight",
			util.ImageLabelStarlightMetadata: sld,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// send a ready signal
	//close(*ready)

	/*
		// for debug purpose
		_ = ioutil.WriteFile("/tmp/starlight-test.json", sta, 0644)
		f, err := os.OpenFile("/tmp/starlight-test.tar.gz", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open file")
		}
		defer f.Close()
		_, err = io.Copy(f, body)

		_, _ = config, star
	*/

	// keep going and download layers
	star.Init(c.cfg, &ctrImg, manifest, imageConfig)
	err = star.Extract(&body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to extract starlight image")
	}

	return
}

func (c *Client) Close() {
	_ = c.client.Close()
	if c.server != nil {
		c.server.Stop()
	}
	os.Exit(1)
}

func (c *Client) StartSnapshotterService() {
	// snapshotter plugin
	rpc := grpc.NewServer()
	c.sn = NewSnapshotter(c.cfg.FileSystemRoot)
	svc := snapshotservice.FromSnapshotter(c.sn)
	if err := os.MkdirAll(filepath.Dir(c.cfg.Socket), 0700); err != nil {
		log.G(c.ctx).WithError(err).Fatalf("failed to create directory %q for socket\n", filepath.Dir(c.cfg.Socket))
		os.Exit(1)
		return
	}

	// Try to remove the socket file to avoid EADDRINUSE
	if err := os.RemoveAll(c.cfg.Socket); err != nil {
		log.G(c.ctx).WithError(err).Fatalf("failed to remove %q\n", c.cfg.Socket)
		os.Exit(1)
		return
	}
	snapshotsapi.RegisterSnapshotsServer(rpc, svc)

	// Listen and serve
	l, err := net.Listen("unix", c.cfg.Socket)
	if err != nil {
		log.G(c.ctx).WithError(err).Fatal("unix listen")
		os.Exit(1)
		return
	}
	log.G(c.ctx).
		WithField("addr", c.cfg.Socket).
		Info("starlight snapshotter service started")

	if err := rpc.Serve(l); err != nil {
		log.G(c.ctx).WithError(err).Fatal("rpc serve")
		os.Exit(1)
		return
	}
}

func NewClient(ctx context.Context, cfg *Configuration) (c *Client, err error) {
	c = &Client{
		ctx:    ctx,
		cfg:    cfg,
		client: nil,
		server: nil,
	}

	// containerd client
	c.client, err = containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		return nil, err
	}

	///var/lib/containerd/io.containerd.snapshotter.v1.starlight
	c.cs = c.client.ContentStore()

	return c, nil
}
