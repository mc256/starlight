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
	"net/http"
	"testing"

	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/test"
	"github.com/mc256/starlight/util/common"
)

//
// To run this test, you need to set the environment in <PROJECT_ROOT>/sandbox/etc/starlight/starlight-proxy.json
// you will need a running postgresql database with container images imported by starlight-proxy
//

// InitDatabase initializes the database for testing
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

func PopulateDatabase(server *Server, images ...string) (err error) {
	fmt.Printf("populate database with metadata\n")

	// TODO: found some concurrency issue with the database
	//       we cannot cache the same LAYER concurrently
	//       this needs to be fixed

	//var errGrp errgroup.Group
	for _, img := range images {
		//img := img
		//errGrp.Go(func() error {
		// cache metadata to metadata database
		ext, err := NewExtractor(server, img, true)
		if err != nil {
			return err
		}

		_, err = ext.SaveToC()
		if err != nil {
			return err
		}
		//return nil
		//})
	}

	//return errGrp.Wait()
	return nil
}

func TestDatabase_GetImageByDigest(t *testing.T) {
	_, _, server := InitDatabase()
	b := &Builder{
		server: server,
	}

	if err := PopulateDatabase(server, "starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a"); err != nil {
		t.Error(err)
		return
	}

	// if it is a multi-arch image, the digest is the platform digest
	// docker pull registry.yuri.moe/starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a
	i, err := b.getImageByDigest("starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a")
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

	if err := PopulateDatabase(server, "starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	i, err := b.getImage("starlight/redis:7.0.5", "linux/amd64")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(i)
}

func TestNewBuilder(t *testing.T) {
	_, _, server := InitDatabase()

	if err := PopulateDatabase(server,
		"starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a",
		"starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server,
		"starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a",
		"starlight/redis:7.0.5",
		"linux/amd64",
		false)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilderWithDisabledSorting(t *testing.T) {
	_, _, server := InitDatabase()

	if err := PopulateDatabase(server,
		"starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a",
		"starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server,
		"starlight/redis@sha256:1a98eb2e5ef8dcbb007d3b821a62a96c93744db78581e99a669cee3ef1e0917a",
		"starlight/redis:7.0.5",
		"linux/amd64",
		true)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder2(t *testing.T) {
	_, _, server := InitDatabase()

	if err := PopulateDatabase(server,
		"starlight/mariadb@sha256:a5c4423aed41c35e45452a048b467eb80ddec1856cbf76edbe92d42699268798",
		"starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	// docker pull registry.yuri.moe/starlight/mariadb@sha256:a5c4423aed41c35e45452a048b467eb80ddec1856cbf76edbe92d42699268798
	b, err := NewBuilder(server,
		"starlight/mariadb@sha256:a5c4423aed41c35e45452a048b467eb80ddec1856cbf76edbe92d42699268798",
		"starlight/redis:7.0.5",
		"linux/amd64",
		false)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(b)
}

func TestNewBuilder3(t *testing.T) {
	_, _, server := InitDatabase()

	if err := PopulateDatabase(server,
		"starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server,
		"",
		"starlight/redis:7.0.5",
		"linux/amd64",
		false)
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

	if err := PopulateDatabase(server,
		"starlight/mariadb@sha256:9c0c61b8c8c7e406f48ab2c9fb73181e2f0e07ec327f6a8409f7b64c8fc0a0d6",
		"starlight/mariadb:10.11.4"); err != nil {
		t.Error(err)
		return
	}

	// docker pull registry.yuri.moe/starlight/mariadb@sha256:9c0c61b8c8c7e406f48ab2c9fb73181e2f0e07ec327f6a8409f7b64c8fc0a0d6
	b, err := NewBuilder(server,
		"starlight/mariadb@sha256:9c0c61b8c8c7e406f48ab2c9fb73181e2f0e07ec327f6a8409f7b64c8fc0a0d6",
		"starlight/mariadb:10.11.4", "linux/amd64", false)
	if err != nil {
		t.Error(err)
		return
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

	if err := PopulateDatabase(server,
		"starlight/mariadb@sha256:9c0c61b8c8c7e406f48ab2c9fb73181e2f0e07ec327f6a8409f7b64c8fc0a0d6",
		"starlight/mariadb:10.11.4"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server,
		"starlight/mariadb@sha256:9c0c61b8c8c7e406f48ab2c9fb73181e2f0e07ec327f6a8409f7b64c8fc0a0d6",
		"starlight/mariadb:10.11.4",
		"linux/amd64", false)
	if err != nil {
		t.Error(err)
		return
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

	if err := PopulateDatabase(server,
		"starlight/mariadb:10.11.4"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server, "", "starlight/mariadb:10.11.4", "linux/amd64", false)
	if err != nil {
		t.Error(err)
		return
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

	if err := PopulateDatabase(server,
		"starlight/redis:7.0.5"); err != nil {
		t.Error(err)
		return
	}

	b, err := NewBuilder(server, "", "starlight/redis:7.0.5", "linux/amd64", false)
	if err != nil {
		t.Error(err)
		return
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
