/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"encoding/json"
	"fmt"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/mc256/starlight/client/fs"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	// setup code here
	ctx = context.Background()
	code := m.Run()
	// teardown code here
	os.Exit(code)
}

func TestManager_Extract(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// Delta bundle body
	f, err := os.OpenFile("/tmp/starlight-test.tar.gz", os.O_RDONLY, 0755)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	var rc io.ReadCloser
	rc = f

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)

	err = m.Extract(&rc)
	if err != nil {
		t.Error(errors.Wrapf(err, "failed to extract starlight image"))
		return
	}

}

func TestManager_Init(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	/*
	 1:home/redis/.wh..wh..opq
	 2:usr/share/zoneinfo/.wh..wh..opq
	 3:usr/src/.wh..wh..opq
	 4:data/.wh..wh..opq
	*/
	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)

}

func TestManager_NewStarlightFS(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)
	opt := fusefs.Options{}
	stack := int64(2)
	f, err := m.NewStarlightFS("/opt/test", stack, &opt, true)
	if err != nil {
		t.Error(errors.Wrapf(err, "failed to extract starlight image"))
		return
	}

	go func() {
		f.Serve()
	}()
	time.Sleep(30 * time.Second)
	_ = f.Teardown()
}

func TestManager_NewStarlightFSMultiple(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)
	opt := fusefs.Options{}

	fss := make([]*fs.Instance, 0)
	for i, _ := range m.Destination.Layers {
		p := fmt.Sprintf("/opt/test/%d", i)
		_ = os.MkdirAll(p, 0755)
		f, err := m.NewStarlightFS(p, int64(i), &opt, true)
		if err != nil {
			t.Error(errors.Wrapf(err, "failed to extract starlight image"))
			return
		}
		fss = append(fss, f)
	}

	for _, f := range fss {
		f := f
		go func() {
			f.Serve()
		}()
	}

	time.Sleep(180 * time.Second)
	for _, f := range fss {
		_ = f.Teardown()
	}
}

func TestManager_NewStarlightFSMultiple2(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)
	opt := fusefs.Options{}

	fss := make([]*fs.Instance, 0)
	for _, i := range []int{5} {
		p := fmt.Sprintf("/opt/test/%d", i)
		_ = os.MkdirAll(p, 0755)
		f, err := m.NewStarlightFS(p, int64(i), &opt, false)
		if err != nil {
			t.Error(errors.Wrapf(err, "failed to extract starlight image"))
			return
		}
		fss = append(fss, f)
	}

	for _, f := range fss {
		f := f
		go func() {
			f.Serve()
		}()
	}

	time.Sleep(5 * time.Minute)
	for _, f := range fss {
		_ = f.Teardown()
	}
}

func TestManager_NewStarlightFSMultiple3(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}

	var m *Manager
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, true, nil, nil, md)
	err = m.SetOptimizerOn("default")
	if err != nil {
		t.Error(err)
		return
	}

	opt := fusefs.Options{}

	fss := make([]*fs.Instance, 0)
	for i, _ := range m.Destination.Layers {
		p := fmt.Sprintf("/opt/test/%d", i)
		_ = os.MkdirAll(p, 0755)
		f, err := m.NewStarlightFS(p, int64(i), &opt, false)
		if err != nil {
			t.Error(errors.Wrapf(err, "failed to extract starlight image"))
			return
		}
		fss = append(fss, f)
	}

	for _, f := range fss {
		f := f
		go func() {
			f.Serve()
		}()
	}

	time.Sleep(5 * time.Minute)
	m.Teardown()
}

func TestManager_NewStarlightSnapshotterTest(t *testing.T) {
	ctx := context.TODO()
	cfg, _, _, _ := LoadConfig("/root/daemon.json")

	var (
		m          *Manager
		configFile *v1.Image
		manifest   *v1.Manifest
	)

	// Starlight header
	b, err := ioutil.ReadFile("/tmp/starlight-test.json")
	if err != nil {
		t.Error(err)
		return
	}
	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Error(err)
		return
	}

	// imageConfig
	b, err = ioutil.ReadFile("/tmp/starlight-test-config.json")
	if err != nil {
		t.Error(err)
		return
	}
	err = json.Unmarshal(b, &configFile)
	if err != nil {
		t.Error(err)
		return
	}

	// manifest
	b, err = ioutil.ReadFile("/tmp/starlight-test-manifest.json")
	if err != nil {
		t.Error(err)
		return
	}
	err = json.Unmarshal(b, &manifest)
	if err != nil {
		t.Error(err)
		return
	}

	// new client
	cc, err := NewClient(ctx, cfg)
	cc.StartSnapshotter()
	if err != nil {
		t.Error(err)
		return
	}

	// keep going and download layers
	md := digest.Digest("sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
	m.Init(ctx, cfg, false, manifest, configFile, md)
	_, _ = m.CreateSnapshots(cc)

}

func TestFilePath(t *testing.T) {
	fmt.Println(filepath.Dir("etc/hosts"))
	fmt.Println(filepath.Dir("etc"))
	fmt.Println(filepath.Dir(""))
	fmt.Println(filepath.Join(".", ".."))
}

func TestChannel(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 1
	close(ch)
	fmt.Println(<-ch)
	fmt.Printf("%p", ch)

}
