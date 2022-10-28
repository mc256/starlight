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
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"strings"
	"time"
)

type Client struct {
	ctx    context.Context
	cfg    *Configuration
	client *containerd.Client
}

func escapeSlashes(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "/", "\\/")
}

func getImageFilter(ref string) string {
	return fmt.Sprintf(
		"name~=/^%s.*/,labels.%s==%s",
		escapeSlashes(ref),
		util.ImagePullerLabel, "starlight",
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

func (c *Client) storeManifest(cfgName, d, ref string, man []byte) (err error) {
	pd := digest.Digest(d)

	// create content store
	cs := c.client.ContentStore()

	err = content.WriteBlob(
		c.ctx, cs, pd.Hex(), bytes.NewReader(man),
		v1.Descriptor{Size: int64(len(man)), Digest: pd},
		content.WithLabels(map[string]string{
			util.ImagePullerLabel:          "starlight",
			util.StarlightProxyMediaType:   "manifest",
			getDistributionSource(cfgName): ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for manifest")
	}
	return nil
}

func (c *Client) handleConfig(buf *bytes.Buffer) (config *v1.ImageConfig, b []byte, err error) {
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
	cs := c.client.ContentStore()

	err = content.WriteBlob(
		c.ctx, cs, pd.Hex(), bytes.NewReader(cfg),
		v1.Descriptor{Size: int64(len(cfg)), Digest: pd},
		content.WithLabels(map[string]string{
			util.ImagePullerLabel:          "starlight",
			util.StarlightProxyMediaType:   "config",
			getDistributionSource(cfgName): ref,
		}))
	if err != nil {
		return errors.Wrapf(err, "failed to open writer for config")
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
	body, mSize, cSize, sSize, md, err := p.DeltaImage(baseRef, ref, platform)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pull image %s", ref)
	}
	defer body.Close()

	log.G(c.ctx).
		WithField("manifest", mSize).
		WithField("config", cSize).
		WithField("starlight", sSize).
		WithField("digest", md).
		Infof("pulling image %s", ref)

	var (
		buf      *bytes.Buffer
		man      []byte
		manifest *v1.Manifest
		newImg   images.Image
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
	err = c.storeManifest(pcn, md, ref, man)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to store manifest")
	}

	// config
	buf, err = c.readBody(body, cSize)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config")
	}
	_, cFile, err := c.handleConfig(buf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to handle config")
	}
	err = c.storeConfig(pcn, ref, manifest.Config.Digest, cFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to store config")
	}

	// create image
	is := c.client.ImageService()
	newImg, err = is.Create(c.ctx, images.Image{
		Name: ref,
		Target: v1.Descriptor{
			MediaType: util.MediaTypeManifestV2,
			Digest:    digest.Digest(md),
			Size:      int64(len(man)),
		},
		Labels: map[string]string{
			util.ImagePullerLabel: "starlight",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// send a ready signal
	close(*ready)
	// keep going and download layers

	fmt.Println("---------------------")
	fmt.Println(manifest, newImg, err)

	return
}

func (c *Client) Close() {
	_ = c.client.Close()
}

func NewClient(ctx context.Context, cfg *Configuration) (c *Client, err error) {
	c = &Client{
		ctx: ctx,
		cfg: cfg,
	}

	// containerd client
	c.client, err = containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		return nil, err
	}

	// starlight proxy api
	c.client.ContentStore()

	return c, nil
}
