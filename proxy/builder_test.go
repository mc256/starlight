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
	"github.com/mc256/starlight/util/common"
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
		cache:  make(map[string]*common.LayerCache),
	}
	if db, err := NewDatabase(ctx, cfg.PostgresConnectionString); err != nil {
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

	i, err := b.getImageByDigest("starlight/redis@sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965")
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

	i, err := b.getImage("starlight/redis:7.0.5", "linux/amd64")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(i)
}

func TestNewBuilder(t *testing.T) {
	//ctx, cfg, server := InitDatabase()
	_, _, server := InitDatabase()

	b, err := NewBuilder(server,
		"starlight/redis@sha256:50a0f37293a4d0880a49e0c41dd71e1d556d06d8fa6c8716afc467b1c7c52965",
		"starlight/redis:7.0.5",
		"linux/amd64")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder2(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server,
		"starlight/mariadb@sha256:1115e2247474b2edb81fad9f5cba70c372c6cfa40130b041ee7f09c8bb726838",
		"starlight/redis:7.0.5",
		"linux/amd64")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder3(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server,
		"",
		"starlight/redis:7.0.5",
		"linux/amd64")
	if err != nil {
		t.Error(err)
		return
	}
	if err = b.Load(); err != nil {
		t.Error(err)
		return
	}

	fmt.Println(b)
}

func TestBuilder_WriteHeader(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "starlight/mariadb@sha256:1115e2247474b2edb81fad9f5cba70c372c6cfa40130b041ee7f09c8bb726838", "starlight/mariadb:10.9.2", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	if err = b.Load(); err != nil {
		t.Error(err)
		return
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

	b, err := NewBuilder(server, "starlight/mariadb@sha256:1115e2247474b2edb81fad9f5cba70c372c6cfa40130b041ee7f09c8bb726838", "starlight/mariadb:10.9.2", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	if err = b.Load(); err != nil {
		t.Error(err)
		return
	}

	fmt.Println(b)

	w := test.NewFakeResponseWriter("body")
	err = b.WriteBody(w, &http.Request{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(w.Header())
}

func TestBuilder_WriteBody2(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "", "starlight/mariadb:10.9.2", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	if err = b.Load(); err != nil {
		t.Error(err)
		return
	}

	fmt.Println(b)

	w := test.NewFakeResponseWriter("body")
	err = b.WriteBody(w, &http.Request{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(w.Header())
}

func TestBuilder_WriteBody3(t *testing.T) {
	_, _, server := InitDatabase()

	b, err := NewBuilder(server, "", "starlight/redis:6.2.7", "linux/amd64")
	if err != nil {
		t.Error(err)
	}

	if err = b.Load(); err != nil {
		t.Error(err)
		return
	}

	fmt.Println(b)

	w := test.NewFakeResponseWriter("body")
	err = b.WriteBody(w, &http.Request{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(w.Header())
}
