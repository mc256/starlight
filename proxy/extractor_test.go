/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/containerd/containerd/log"
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

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2", true)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
}

func TestExtractor_SaveToC(t *testing.T) {
	t.Skip("for dev only")

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

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2", true)
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
