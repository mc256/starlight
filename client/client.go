/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"strings"
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

	// pull image
	pc := c.cfg.getProxy(proxyCfg)
	p := proxy.NewStarlightProxy(c.ctx, pc.Protocol, pc.Address)
	if pc.Username != "" {
		p.SetAuth(pc.Username, pc.Password)
	}

	baseRef := ""
	if base != nil {
		baseRef = fmt.Sprintf("%s@%s", base.Name(), base.Target().Digest)
	}
	_, err = p.DeltaImage(baseRef, ref, platform)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pull image %s", ref)
	}

	// extract image

	// write to content store: finished manifest close channel

	// create image

	// continue writing to content store:

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
