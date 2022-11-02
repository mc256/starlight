/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io/ioutil"
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
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	err = c.InitSnapshotter()
	if err != nil {
		t.Error(err)
		return
	}
	//plt := platforms.Format(platforms.DefaultSpec())
	//t.Log("pulling image", "platform", plt)
	ready := make(chan bool)
	img, err := c.PullImage(nil,
		"harbor.yuri.moe/starlight/redis:6.2.7",
		"linux/amd64",
		"",
		&ready)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}

func TestClient_FindBaseImage(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	img, err := c.FindBaseImage("", "harbor.yuri.moe/starlight/redis:7.0.5")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}

func TestClient_PullImageWithUpdate(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	//plt := platforms.Format(platforms.DefaultSpec())
	//t.Log("pulling image", "platform", plt)
	//"harbor.yuri.moe/starlight/redis@sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965"

	base, err := c.FindBaseImage("", "harbor.yuri.moe/starlight/redis:7.0.5")
	if err != nil {
		t.Error(err)
		return
	}

	ready := make(chan bool)
	img, err := c.PullImage(base,
		"harbor.yuri.moe/starlight/redis:7.0.5",
		"linux/amd64",
		"",
		&ready)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}

func TestClient_CreateImageService(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	//plt := platforms.Format(platforms.DefaultSpec())
	//t.Log("pulling image", "platform", plt)
	ready := make(chan bool)
	img, err := c.PullImage(nil,
		"starlight/redis:6.2.7",
		"linux/amd64",
		"",
		&ready)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
}

func Test_WriteContent(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}
	cs := c.client.ContentStore()

	mf, err := ioutil.ReadFile("/root/manifest.json")
	if err != nil {
		t.Error(err)
		return
	}
	mfr := bytes.NewReader(mf)
	fmt.Println(len(mf))
	ref := "sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965"
	dg := digest.Digest(ref)
	if err != nil {
		t.Error(err)
		return
	}
	expectedSize := int64(3341)
	err = content.WriteBlob(c.ctx, cs, dg.Hex(), mfr, v1.Descriptor{
		Size:   expectedSize,
		Digest: dg,
		Annotations: map[string]string{
			"containerd.io/uncompressed": dg.Hex(),
		},
	}, content.WithLabels(map[string]string{
		"containerd.io/gasdft": "true",
	}))
	if err != nil {
		t.Error(err)
		return
	}
}

func TestClient_scanExistingFilesystems(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	c.scanExistingFilesystems()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestClient_scanSnapshots(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	c.scanSnapshots()
	if err != nil {
		t.Error(err)
		return
	}
}
