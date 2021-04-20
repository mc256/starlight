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
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"os"
	"testing"
)

func TestImageBuilder_WriteHeader(t *testing.T) {
	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.127:5000"
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

	if ib, err = NewPreloadImageBuilder(ctx, c, ContainerRegistry); err != nil {
		t.Fatal(err)
		return
	}

	if lenH, lenC, err = ib.WriteHeader(fh); err != nil {
		t.Fatal(err)
		return
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"header":  lenH,
			"content": lenC,
		}).Infof("wrote gzip toc")
	}
}

func TestImageBuilder_WriteBody(t *testing.T) {

	const (
		ImageName         = "ubuntu"
		ImageTag          = "18.04-starlight"
		ContainerRegistry = "http://10.219.31.127:5000"
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

	if lenH, lenC, err = ib.WriteHeader(fh); err != nil {
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
