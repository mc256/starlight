/*
   Copyright The starlight Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   file created by maverick in 2021
*/

package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/mc256/starlight/test"
	"github.com/pkg/errors"
	"net/http"
	"testing"
)

func InitDatabase() (context.Context, *Configuration, *Server) {
	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")
	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
		cache:  make(map[string]*LayerCache),
	}
	if db, err := NewDatabase(cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}
	return ctx, cfg, server
}

func TestNewBuilder(t *testing.T) {
	//ctx, cfg, server := InitDatabase()
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:35a01e19bf90bf187278c49e5efa04e89fc03a6f5ce5ce9245eb3d98bbc6d895")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder2(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder3(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestBuilder_GetManifestAndConfig(t *testing.T) {
	_, _, server := InitDatabase()

	var err error

	src := "public/mariadb:10.8.4d"
	dst := "public/mariadb:10.9.2a"

	builder := &Builder{
		server: server,
	}

	// Build
	if builder.Source, err = builder.getImage(src); err != nil {
		t.Error(errors.Wrapf(err, "failed to obtain src image"))
	}

	if builder.Destination, err = builder.getImage(dst); err != nil {
		t.Error(errors.Wrapf(err, "failed to obtain dst image"))
	}

	var unavailableLayers, availableLayers []*ImageLayer
	if builder.Source != nil {
		availableLayers = builder.Source.Layers
		unavailableLayers, err = builder.getUnavailableLayers()
		if err != nil {
			t.Error(err)
		}
	} else {
		availableLayers = []*ImageLayer{}
		unavailableLayers = builder.Destination.Layers
	}

	for _, a := range availableLayers {
		a.available = true
	}
	for _, u := range unavailableLayers {
		u.available = false
	}

	c, m, err := builder.getManifestAndConfig(builder.Destination.Serial)
	if err != nil {
		t.Error(err)
	}

	var (
		manifest v1.Manifest
		config   v1.ConfigFile
	)

	err = json.Unmarshal(m, &manifest)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(manifest.MediaType)

	err = json.Unmarshal(c, &config)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(config.Config.Cmd)
}

func TestBuilder_ComputeDifferences(t *testing.T) {
	_, _, server := InitDatabase()

	var err error

	src := "public/mariadb:10.8.4d"
	dst := "public/mariadb:10.9.2a"

	builder := &Builder{
		server: server,
	}

	// Build
	if builder.Source, err = builder.getImage(src); err != nil {
		t.Error(errors.Wrapf(err, "failed to obtain src image"))
	}

	if builder.Destination, err = builder.getImage(dst); err != nil {
		t.Error(errors.Wrapf(err, "failed to obtain dst image"))
	}

	var unavailableLayers, availableLayers []*ImageLayer
	if builder.Source != nil {
		availableLayers = builder.Source.Layers
		unavailableLayers, err = builder.getUnavailableLayers()
		if err != nil {
			t.Error(err)
		}
	} else {
		availableLayers = []*ImageLayer{}
		unavailableLayers = builder.Destination.Layers
	}

	for _, a := range availableLayers {
		a.available = true
	}
	for _, u := range unavailableLayers {
		u.available = false
	}

	err = builder.computeDelta()
	if err != nil {
		t.Error(err)
	}

	fmt.Println(builder.contentLength)
}

func TestBuilder_WriteHeader(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)

	w := test.NewFakeResponseWriter("header")
	err = b.WriteHeader(w, &http.Request{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(w.Header())
}

func TestBuilder_WriteBody(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)

	w := test.NewFakeResponseWriter("body")
	err = b.WriteBody(w, &http.Request{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(w.Header())

}
