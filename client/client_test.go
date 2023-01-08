/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/platforms"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/client/snapshotter"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io/fs"
	"io/ioutil"
	"os"
	"syscall"
	"testing"
)

func TestImageFilter(t *testing.T) {
	cfg, p, _, _ := LoadConfig("/sandbox/etc/starlight/starlight-daemon.json")
	fmt.Println("config path: ", p)
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("containerd: ", cfg.Containerd)
	fmt.Println("namespace: ", cfg.Namespace)
	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	imgFilterRef := "starlight-registry.default.svc.cluster.local:5000/starlight/redis:6.2.1"
	img, err := c.findImage(client, getImageFilter(imgFilterRef, true))
	if err != nil {
		t.Error(err)
		return
	}
	if img != nil {
		t.Error("image should be nil")
	}

	img, err = c.findImage(client, getImageFilter(imgFilterRef, false))
	if err != nil {
		t.Error(err)
		return
	}
	if img == nil {
		t.Error("image should not be nil")
	}

}

func TestClient_RemoveImage(t *testing.T) {
	cfg, p, _, _ := LoadConfig("/sandbox/etc/starlight/starlight-daemon.json")
	fmt.Println("config path: ", p)
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("containerd: ", cfg.Containerd)
	fmt.Println("namespace: ", cfg.Namespace)
	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	imgFilterRef := "starlight-registry.default.svc.cluster.local:5000/starlight/redis:6.2.1"

	is := client.ImageService()
	// remove image
	err = is.Delete(c.ctx, imgFilterRef)
	if err != nil {
		t.Error(err)
		return
	}

}

func TestClient_PullImageNotUpdate(t *testing.T) {
	// Standard image pull
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}

	operator := snapshotter.NewOperator(c.ctx, c, client.SnapshotService("starlight"))
	img, err := c.pullImageSync(client, nil,
		"harbor.yuri.moe/starlight/redis:6.2.7", "linux/amd64", "")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(img)
	t.Log(operator)
}

func TestClient_TestSnapshotter(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}

	svc := client.SnapshotService("starlight")

	op := snapshotter.NewOperator(c.ctx, c, svc)
	_ = op.ScanSnapshots()

}

func TestClient_TestSnapshotterAdd(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}

	svc := client.SnapshotService("starlight")
	mnt, err := svc.Prepare(c.ctx, "test", "")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(mnt)
}

func TestClient_FindBaseImage(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	img, err := c.FindBaseImage(client, "", "harbor.yuri.moe/starlight/redis:7.0.5")
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

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	base, err := c.FindBaseImage(client, "", "harbor.yuri.moe/starlight/redis:7.0.5")
	if err != nil {
		t.Error(err)
		return
	}

	img, err := c.pullImageSync(client, base,
		"harbor.yuri.moe/starlight/redis:7.0.5",
		"linux/amd64",
		"")
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

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}

	//plt := platforms.Format(platforms.DefaultSpec())
	//t.Log("pulling image", "platform", plt)
	img, err := c.pullImageSync(client, nil,
		"starlight/redis:6.2.7", "linux/amd64", "")
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

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	cs := client.ContentStore()

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

	//sn := client.SnapshotService("starlight")
	//op:= snapshotter.NewOperator(c.ctx, c, sn)

	c.ScanExistingFilesystems()
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

	c.ScanExistingFilesystems()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestClient_LoadImage(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	c, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Error(err)
		return
	}

	client, err := containerd.New(cfg.Containerd, containerd.WithDefaultNamespace(cfg.Namespace))
	if err != nil {
		t.Error(err)
		return
	}
	m, err := c.LoadImage(client,
		digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965"),
	)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(m)
}

func TestPlatform(t *testing.T) {
	fmt.Println(platforms.DefaultString())
}

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

func TestTransportEndpointNotConnected(t *testing.T) {
	_, err := os.Stat("/var/lib/starlight/mnt/4/slfs")
	if err.(*fs.PathError).Err == syscall.ENOTCONN {
		t.Log("not connected")
		return
	}
	t.Error(err)
}

func TestInsecureReference(t *testing.T) {
	n, _ := name.ParseReference("172.31.92.41:5000/redis:6.2.2-starlight")
	fmt.Println(n)

	n2, _ := name.ParseReference("172.31.92.41:5000/redis:6.2.2-starlight", name.Insecure)
	fmt.Println(n2)
}
