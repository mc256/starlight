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
	"github.com/mc256/starlight/test"
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

func TestDatabase_GetImageByDigest(t *testing.T) {
	_, _, server := InitDatabase()
	b := &Builder{
		server: server,
	}

	i, err := b.getImageByDigest("public/redis@sha256:85d9e600f0b05e086219964af6e5038626ef816160ada234fe820331f26d34d2")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(i)
}

func TestDatabase_GetImage(t *testing.T) {
	_, _, server := InitDatabase()
	b := &Builder{
		server: server,
	}

	i, err := b.getImage("public/redis@sha256:85d9e600f0b05e086219964af6e5038626ef816160ada234fe820331f26d34d2", "s390x")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(i)
}

func TestNewBuilder(t *testing.T) {
	//ctx, cfg, server := InitDatabase()
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb@sha256:35a01e19bf90bf187278c49e5efa04e89fc03a6f5ce5ce9245eb3d98bbc6d895", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder2(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder3(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "", "public/mariadb:10.9.2a", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestBuilder_WriteHeader(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a", "linux/amd64")
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

	b, err := NewBuilder(server, "public/mariadb:10.8.4d", "public/mariadb:10.9.2a", "linux/amd64")
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
