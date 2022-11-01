/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"encoding/json"
	"fmt"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/mc256/starlight/client/fs"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	m.Init(cfg, nil, nil, nil)

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
	m.Init(cfg, nil, nil, nil)

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
	m.Init(cfg, nil, nil, nil)
	opt := fusefs.Options{}
	stack := int64(2)
	fs, err := m.NewStarlightFS("/opt/test", stack, &opt, true)
	if err != nil {
		t.Error(errors.Wrapf(err, "failed to extract starlight image"))
		return
	}

	go func() {
		fs.Serve()
	}()
	time.Sleep(30 * time.Second)
	_ = fs.Teardown()
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
	m.Init(cfg, nil, nil, nil)
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
	m.Init(cfg, nil, nil, nil)
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
