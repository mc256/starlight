package proxy

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/test"
)

const (
	TestImageName   = "redis"
	TestImageSrcTag = "6.0"
	TestImageDstTag = "6.0-sl-test"
)

func TestConvertorConstructor(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	// we don't need the protocol here whether it is http or https or not
	// it needs to be set in the options.
	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{name.Insecure})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c)
	return
}

func TestReadImage(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{name.Insecure})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.readImage(); err != nil {
		t.Fatal(err)
	}
	return
}

func TestReadImage2(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{name.Insecure})
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
	containerRegistry := test.GetContainerRegistry(t)

	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{name.Insecure})
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
	containerRegistry := test.GetContainerRegistry(t)

	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{name.Insecure})
	if err != nil {
		t.Fatal(err)
	}
	err = c.ToStarlightImage()
	if err != nil {
		t.Fatal(err)
	}

	return
}

func TestToStarlightImage2(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	srcRef := fmt.Sprintf("%s/%s:%s", strings.TrimPrefix(containerRegistry, "http://"), TestImageName, TestImageSrcTag)
	dstRef := fmt.Sprintf("%s/%s:%s", "registry.yuri.moe", TestImageName, TestImageDstTag)

	c, err := GetConvertor(srcRef, dstRef, []name.Option{name.Insecure}, []name.Option{})
	if err != nil {
		t.Fatal(err)
	}
	err = c.ToStarlightImage()
	if err != nil {
		t.Fatal(err)
	}

	return
}
