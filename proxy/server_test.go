/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"net/http"
	"testing"
)

func TestNewLayerCache(t *testing.T) {

	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")
	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}

	//sha256:984d2c1b279e64cdbfdc8979865943de17cfa4bb2e657c0f63a5254d0bd08c17
	d, _ := name.NewDigest("harbor.yuri.moe/public/mariadb@sha256:984d2c1b279e64cdbfdc8979865943de17cfa4bb2e657c0f63a5254d0bd08c17")
	lc := NewLayerCache(&ImageLayer{
		stackIndex: 7,
		size:       988,
		Hash:       "sha256:984d2c1b279e64cdbfdc8979865943de17cfa4bb2e657c0f63a5254d0bd08c17",
		digest:     d,
	})
	lc.Load(server)
	fmt.Println(lc)

}
