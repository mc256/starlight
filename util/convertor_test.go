package util

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/test"
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

}

// TestToStarlightImage Image does requires network connection
func TestToStarlightImageGoharbor(t *testing.T) {
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}

	const (
		srcRef = "docker.io/library/redis:6.2.1"
		dstRef = "harbor.yuri.moe/public/redis:6.2.1"
	)

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
}

func TestToStarlightImageECR(t *testing.T) {
	if test.HasLoginAWSECR() == false {
		t.Skip(">>>>> Skip: no container registry credentials for ECR")
	}

	test.LoadEnvironmentVariables()
	if os.Getenv("TEST_ECR_IMAGE_FROM") == "" {
		t.Skip(">>>>> Skip: no TEST_ECR_IMAGE_FROM environment variable")
	}
	if os.Getenv("TEST_ECR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no TEST_ECR_IMAGE_TO environment variable")
	}

	srcRef := os.Getenv("TEST_ECR_IMAGE_FROM")
	dstRef := os.Getenv("TEST_ECR_IMAGE_TO")

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
}
