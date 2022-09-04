package proxy

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/test"
	"io"
	"testing"
)

const (
	srcRef = "harbor.yuri.moe/public/mariadb:10.9.2"
	dstRef = "harbor.yuri.moe/public/mariadb:10.9.2a"
)

func TestConvertorConstructor(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c)
	return
}

func TestReadImage(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.readImage(); err != nil {
		t.Fatal(err)
	}
	return
}

func TestReadImage2(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{})
	if err != nil {
		t.Fatal(err)
	}
	if img, err := c.readImage(); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println(img)
		config, _ := img.ConfigFile()
		test.PrettyPrintJson(config)
		manifest, _ := img.Manifest()
		test.PrettyPrintJson(manifest)
	}
	return
}

func TestReadImageLayers(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{})
	if err != nil {
		t.Fatal(err)
	}
	if img, err := c.readImage(); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println(img)
		layers, err := img.Layers()
		if err != nil {
			t.Fatal(err)
		}

		imgReader, err := layers[2].Uncompressed()
		if err != nil {
			t.Fatal(err)
		}
		defer imgReader.Close()

		tarReader := tar.NewReader(imgReader)
		for {
			if header, err := tarReader.Next(); err != nil {
				if err == io.EOF {
					break
				} else {
					fmt.Printf("Error: %s\n", err)
				}
			} else {
				fmt.Printf("FileName: %s \n", header.Name)
			}
		}
	}
	return
}

func TestToStarlightImage(t *testing.T) {
	ctx := context.Background()

	c, err := NewConvertor(ctx, srcRef, dstRef, []name.Option{}, []name.Option{}, []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	})
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
