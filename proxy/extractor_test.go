/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"net/http"
	"testing"
)

func TestNewExtractor(t *testing.T) {
	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")
	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
}

func TestExtractor_SaveToC(t *testing.T) {
	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")
	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}
	if db, err := NewDatabase(ctx, cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
	res, err := ext.SaveToC()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(res)
}
