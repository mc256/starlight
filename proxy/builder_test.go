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
	"fmt"
	"github.com/containerd/containerd/log"
	"net/http"
	"testing"
)

func InitDatabase() (context.Context, *ProxyConfiguration, *Server) {
	ctx := context.Background()
	cfg := LoadConfig()
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

	b, err := NewBuilder(server, "", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	if _, _, err := b.getManifestAndConfig(b.Destination.Serial); err != nil {
		t.Error(err)
	}
}

func TestBuilder_ComputeDifferences(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a")
	if err != nil {
		t.Error(err)
	}

	err = b.computeDelta()
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}
