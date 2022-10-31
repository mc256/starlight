/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"encoding/json"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestManager_Extract(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	// /tmp/starlight-test.tar
	// /tmp/starlight-test.json

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
	err = m.InitFromProxy(&rc, cfg, &v1.Image{})
	if err != nil {
		t.Error(errors.Wrapf(err, "failed to process starlight header"))
		return
	}

	err = m.Extract()
	if err != nil {
		t.Error(errors.Wrapf(err, "failed to extract starlight image"))
		return
	}

}
