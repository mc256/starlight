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
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"testing"
)

func TestOverlayToc(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.214:5000"
	)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	// --------------------------------------------------------------
	ob2 := merger.NewOverlayBuilder(ctx, db)
	if err = ob2.AddImage(ImageName, ImageTag); err != nil {
		t.Fatal(err)
		return
	}
	fh2, err := os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.overlaytoc2.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer fh2.Close()
	if err := ob2.ExportTOC(fh2); err != nil {
		t.Fatal(err)
		return
	}
	// --------------------------------------------------------------
	err = ob2.SaveMergedImage()
	if err != nil {
		t.Fatal(err)
		return
	}
	// --------------------------------------------------------------
	ob3, err := merger.LoadMergedImage(ctx, db, ImageName, ImageTag)
	if err != nil {
		t.Fatal(err)
		return
	}
	fh3, err := os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.overlaytoc3.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer fh3.Close()
	if err := ob3.ExportTOC(fh3); err != nil {
		t.Fatal(err)
		return
	}

}

func TestOverlayDelta(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.214:5000"
	)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	// --------------------------------------------------------------
	ob2 := merger.NewOverlayBuilder(ctx, db)
	if err = ob2.AddImage(ImageName, ImageTag); err != nil {
		t.Fatal(err)
		return
	}
	fh2, err := os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.delta2.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer fh2.Close()
	dt2 := merger.GetDelta(ctx, merger.NewOverlayBuilder(ctx, db), ob2)
	if err := dt2.ExportTOC(fh2, true); err != nil {
		t.Fatal(err)
		return
	}
	// --------------------------------------------------------------
	// --------------------------------------------------------------
	ob3, err := merger.LoadMergedImage(ctx, db, ImageName, ImageTag)
	if err != nil {
		t.Fatal(err)
		return
	}
	fh3, err := os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.delta3.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer fh3.Close()
	dt3 := merger.GetDelta(ctx, merger.NewOverlayBuilder(ctx, db), ob3)
	if err := dt3.ExportTOC(fh3, true); err != nil {
		t.Fatal(err)
		return
	}

}

func TestImageBuilder_WriteHeader(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	var (
		fh         *os.File
		ib         *ImageBuilder
		lenH, lenC int64
	)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	ob1 := merger.NewOverlayBuilder(ctx, db)
	ob3, err := merger.LoadMergedImage(ctx, db, ImageName, ImageTag)
	if err != nil {
		t.Fatal(err)
		return
	}

	d := merger.GetDelta(ctx, ob1, ob3)

	c := merger.NewConsolidator(ctx)
	if err = c.AddDelta(d); err != nil {
		t.Fatal(err)
		return
	}

	if fh, err = os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.starlight.toc_3.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	); err != nil {
		t.Fatal(err)
		return
	}
	defer fh.Close()

	if ib, err = NewPreloadImageBuilder(ctx, c, ContainerRegistry); err != nil {
		t.Fatal(err)
		return
	}

	buf := bytes.NewBuffer([]byte{})
	if lenH, lenC, err = ib.WriteHeader(buf, true); err != nil {
		t.Fatal(err)
		return
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"header":  lenH,
			"content": lenC,
		}).Infof("wrote gzip toc")
	}

	gr, _ := gzip.NewReader(buf)
	defer gr.Close()

	written, err := io.Copy(fh, gr)

	log.G(ctx).WithFields(logrus.Fields{
		"written": written,
		"err":     err,
	}).Infof("done")

}

func TestImageBuilder_WriteHeader2(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	var (
		fh         *os.File
		ib         *ImageBuilder
		lenH, lenC int64
	)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	ob1 := merger.NewOverlayBuilder(ctx, db)
	ob2 := merger.NewOverlayBuilder(ctx, db)
	if err = ob2.AddImage(ImageName, ImageTag); err != nil {
		t.Fatal(err)
		return
	}

	d := merger.GetDelta(ctx, ob1, ob2)
	c := merger.NewConsolidator(ctx)
	if err = c.AddDelta(d); err != nil {
		t.Fatal(err)
		return
	}

	if fh, err = os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.starlight.toc_2.json",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	); err != nil {
		t.Fatal(err)
		return
	}
	defer fh.Close()

	if ib, err = NewPreloadImageBuilder(ctx, c, ContainerRegistry); err != nil {
		t.Fatal(err)
		return
	}

	buf := bytes.NewBuffer([]byte{})
	if lenH, lenC, err = ib.WriteHeader(buf, true); err != nil {
		t.Fatal(err)
		return
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"header":  lenH,
			"content": lenC,
		}).Infof("wrote gzip toc")
	}

	gr, _ := gzip.NewReader(buf)
	_, _ = io.Copy(fh, gr)
}

func TestImageBuilder_WriteBody(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.214:5000"
	)
	var (
		fh         *os.File
		fb         *os.File
		ib         *ImageBuilder
		lenH, lenC int64
	)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	ob1 := merger.NewOverlayBuilder(ctx, db)
	ob2 := merger.NewOverlayBuilder(ctx, db)
	if err = ob2.AddImage(ImageName, ImageTag); err != nil {
		t.Fatal(err)
		return
	}

	d := merger.GetDelta(ctx, ob1, ob2)
	c := merger.NewConsolidator(ctx)
	if err = c.AddDelta(d); err != nil {
		t.Fatal(err)
		return
	}

	if fh, err = os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.starlight.toc.json.gz",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	); err != nil {
		t.Fatal(err)
		return
	}
	defer fh.Close()

	if fb, err = os.OpenFile(
		fmt.Sprintf(
			"data/%s_%s.starlight.img",
			ImageName,
			ImageTag,
		),
		os.O_RDWR|os.O_CREATE,
		0755,
	); err != nil {
		t.Fatal(err)
		return
	}
	defer fh.Close()

	if ib, err = NewPreloadImageBuilder(ctx, c, ContainerRegistry); err != nil {
		t.Fatal(err)
		return
	}

	if lenH, lenC, err = ib.WriteHeader(fh, false); err != nil {
		t.Fatal(err)
		return
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"header":  lenH,
			"content": lenC,
		}).Infof("wrote gzip toc")
	}

	if err = ib.WriteBody(fb); err != nil {
		t.Fatal(err)
		return
	}
}
