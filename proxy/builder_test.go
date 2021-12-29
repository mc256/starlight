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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/containerd/containerd/log"
	"github.com/joho/godotenv"
	"github.com/mc256/starlight/util"
)

func init() {
	if err := godotenv.Load("../.env"); err != nil {
		fmt.Print("Failed to load environment variables from `.env` file. ")
	}
}

func TestDeltaBundleBuilder_WriteHeader(t *testing.T) {
	const (
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{
		{ImageName: "mariadb", ImageTag: "10.5-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1, err := LoadCollection(ctx, db, []*util.ImageRef{
		{ImageName: "mariadb", ImageTag: "10.4-starlight"},
		{ImageName: "wordpress", ImageTag: "5.7-apache-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1.Minus(fso2)
	deltaBundle := fso1.ComposeDeltaBundle()

	// --------------------------------------------------------------
	// Build Collection

	builder := NewBuilder(ctx, ContainerRegistry)
	out := bytes.NewBuffer([]byte{})

	wg := &sync.WaitGroup{}
	headerSize, contentSize, err := builder.WriteHeader(out, deltaBundle, wg, true)
	if err != nil {
		t.Fatal(err)
		return
	}

	log.G(ctx).WithField("header-size", headerSize).Info("header size")
	log.G(ctx).WithField("content-size", contentSize).Info("content size")

	gr, err := gzip.NewReader(out)
	gzOut := bytes.NewBuffer([]byte{})
	_, _ = io.Copy(gzOut, gr)
	_ = ioutil.WriteFile("./data/jun18_deltaHeader.json", gzOut.Bytes(), 0644)

	_ = util.ExportToJsonFile(deltaBundle.OutputQueue, "./data/jun18_outputqueue.json")
	_ = util.ExportToJsonFile(deltaBundle.RequiredLayer, "./data/jun18_requiredLayer.json")
}

func TestDeltaBundleBuilder_WriteBodyPre(t *testing.T) {
	const (
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{
		{ImageName: "mariadb", ImageTag: "10.5-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	deltaBundle := fso2.ComposeDeltaBundle()

	// --------------------------------------------------------------
	// Build Collection

	builder := NewBuilder(ctx, ContainerRegistry)

	f, err := os.OpenFile("./data/deltabundle-old.img", os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		t.Fatal(err)
		return
	}
	wg := &sync.WaitGroup{}
	headerSize, contentSize, err := builder.WriteHeader(f, deltaBundle, wg, false)
	if err != nil {
		t.Fatal(err)
		return
	}

	log.G(ctx).WithField("header-size", headerSize).Info("header size")
	log.G(ctx).WithField("content-size", contentSize).Info("content size")

	err = builder.WriteBody(f, deltaBundle, wg)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestDeltaBundleBuilder_WriteBody(t *testing.T) {
	const (
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{
		{ImageName: "mariadb", ImageTag: "10.5-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1, err := LoadCollection(ctx, db, []*util.ImageRef{
		{ImageName: "mariadb", ImageTag: "10.4-starlight"},
		{ImageName: "wordpress", ImageTag: "5.7-apache-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1.Minus(fso2)
	deltaBundle := fso1.ComposeDeltaBundle()

	// --------------------------------------------------------------
	// Build Collection

	builder := NewBuilder(ctx, ContainerRegistry)

	f, err := os.OpenFile("./data/deltabundle.img", os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		t.Fatal(err)
		return
	}

	wg := &sync.WaitGroup{}
	headerSize, contentSize, err := builder.WriteHeader(f, deltaBundle, wg, false)
	if err != nil {
		t.Fatal(err)
		return
	}

	log.G(ctx).WithField("header-size", headerSize).Info("header size")
	log.G(ctx).WithField("content-size", contentSize).Info("content size")

	err = builder.WriteBody(f, deltaBundle, wg)
	if err != nil {
		t.Fatal(err)
		return
	}
}
