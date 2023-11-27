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

func TestConvertorConstructor(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}
	if os.Getenv("TEST_DOCKER_SOURCE_IMAGE") == "" {
		t.Skip(">>>>> Skip: no TEST_DOCKER_SOURCE_IMAGE environment variable")
	}
	if os.Getenv("TEST_HARBOR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no TEST_HARBOR_IMAGE_TO environment variable")
	}
	srcRef := os.Getenv("TEST_DOCKER_SOURCE_IMAGE")
	dstRef := os.Getenv("TEST_HARBOR_IMAGE_TO")

	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{}, "all")

	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c)

}

func TestToStarlightImageGoharbor(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}
	if os.Getenv("TEST_DOCKER_SOURCE_IMAGE") == "" {
		t.Skip(">>>>> Skip: no TEST_DOCKER_SOURCE_IMAGE environment variable")
	}
	if os.Getenv("TEST_HARBOR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no TEST_HARBOR_IMAGE_TO environment variable")
	}
	srcRef := os.Getenv("TEST_DOCKER_SOURCE_IMAGE")
	dstRef := os.Getenv("TEST_HARBOR_IMAGE_TO")

	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}, "amd64")
	//}, "all")
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

func TestToStarlightImageGoharborWithMultiplePlatforms(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}
	if os.Getenv("TEST_DOCKER_SOURCE_IMAGE") == "" {
		t.Skip(">>>>> Skip: no TEST_DOCKER_SOURCE_IMAGE environment variable")
	}
	if os.Getenv("TEST_HARBOR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no TEST_HARBOR_IMAGE_TO environment variable")
	}
	srcRef := os.Getenv("TEST_DOCKER_SOURCE_IMAGE")
	dstRef := os.Getenv("TEST_HARBOR_IMAGE_TO")

	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}, "linux/arm64/v8,linux/arm/v7,linux/arm/v5")
	//}, "amd64,linux/arm64/v8,linux/arm/v7")

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

func TestToStarlightImageGoharborWithAllPlatforms(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}
	if os.Getenv("TEST_DOCKER_SOURCE_IMAGE") == "" {
		t.Skip(">>>>> Skip: no TEST_DOCKER_SOURCE_IMAGE environment variable")
	}
	if os.Getenv("TEST_HARBOR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no TEST_HARBOR_IMAGE_TO environment variable")
	}
	srcRef := os.Getenv("TEST_DOCKER_SOURCE_IMAGE")
	dstRef := os.Getenv("TEST_HARBOR_IMAGE_TO")

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

	test.LoadEnvironmentVariables()
	if test.HasLoginAWSECR() == false {
		t.Skip(">>>>> Skip: no container registry credentials for ECR")
	}
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
