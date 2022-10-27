/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"github.com/containerd/containerd/platforms"
	"testing"
)

func TestNewClient(t *testing.T) {
	cfg, _, _, _ := LoadConfig("")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	img, err := c.findImage(getImageFilter("harbor.yuri.moe/public/redis:test2"))
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}

func TestClient_PullImageNotUpdate(t *testing.T) {
	cfg, _, _, _ := LoadConfig("")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	plt := platforms.Format(platforms.DefaultSpec())
	t.Log("pulling image", "platform", plt)
	img, err := c.PullImage(nil, "public/redis:test2", plt)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}
