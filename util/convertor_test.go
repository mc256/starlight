package util

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"testing"
)

const (
	srcRef = "docker.io/library/redis:6.2.1"
	dstRef = "harbor.yuri.moe/public/redis:6.2.1"
)

func TestConvertorConstructor(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{}, "all")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c)
	return
}

func TestToStarlightImage(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}, "all")
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	err = c.ToStarlightImage()
	if err != nil {
		t.Fatal(err)
	}

	return
}
