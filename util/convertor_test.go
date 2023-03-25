package util

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	srcRef = "docker.io/library/redis:6.2.1"
	dstRef = "harbor.yuri.moe/public/redis:6.2.1"
)

// TestConvertorConstructor does not requires any network connection
func TestConvertorConstructor(t *testing.T) {
	ctx := context.Background()

	const (
		srcRef = "docker.io/library/redis:6.2.1"
		dstRef = "harbor.yuri.moe/public/redis:6.2.1"
	)

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{}, "all")

	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c)
	return
}

// TestToStarlightImage Image does requires network connection
func TestToStarlightImage(t *testing.T) {
	t.Skip("for dev only")

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
