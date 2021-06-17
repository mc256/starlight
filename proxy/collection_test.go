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
	"github.com/mc256/starlight/fs"
	"github.com/mc256/starlight/util"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

/* =============================================
	CMS:
	- "wordpress:5.7-apache-starlight"
	- "mariadb:10.4-starlight"

	Default:
	- "wordpress:php7.3-fpm-starlight"
	- "mariadb:10.4-starlight"
	- "mariadb:10.5-starlight"
  ============================================= */

func TestNewCollection1(t *testing.T) {
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	// --------------------------------------------------------------
	// Load Merged
	fso, err := LoadCollection(ctx, db, []*util.ImageRef{
		/*
			{ImageName: "mariadb",ImageTag:  "10.4-starlight"},
			{ImageName: "wordpress",ImageTag:  "5.7-apache-starlight"},
		*/
		{ImageName: "wordpress", ImageTag: "5.7-apache-starlight"},
		{ImageName: "wordpress", ImageTag: "5.7-apache-starlight"},
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	if err = util.ExportToJsonFile(fso, path.Join(os.TempDir(), "fso-table.json")); err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddOptimizeTrace(t *testing.T) {
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	// --------------------------------------------------------------
	buf, err := ioutil.ReadFile(path.Join(os.TempDir(), "group-optimize.json"))
	if err != nil {
		t.Fatal(err)
		return
	}

	tc, err := fs.NewTraceCollectionFromBuffer(buf)
	if err != nil {
		t.Fatal(err)
		return
	}

	for idx, grp := range tc.Groups {
		log.G(ctx).WithField("collection", grp.Images)
		fso, err := LoadCollection(ctx, db, grp.Images)
		if err != nil {
			t.Fatal(err)
			return
		}

		fso.AddOptimizeTrace(grp)

		if err = util.ExportToJsonFile(
			fso,
			path.Join(os.TempDir(), fmt.Sprintf("fso-table-%d.json", idx)),
		); err != nil {
			t.Fatal(err)
			return
		}

		if err := fso.SaveMergedApp(); err != nil {
			t.Fatal(err)
			return
		}
	}

	// --------------------------------------------------------------
	// Build Collection
}

func TestCollection_Minus(t *testing.T) {
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.5-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.4-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	// --------------------------------------------------------------
	// Build Collection
	fso1.Minus(fso2)

	if err = util.ExportToJsonFile(
		fso1,
		path.Join(os.TempDir(), fmt.Sprintf("fso-table-minus.json")),
	); err != nil {
		t.Fatal(err)
		return
	}
}

func TestCollection_RemoveMergedApp(t *testing.T) {
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.5-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.4-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	_ = fso1.RemoveMergedApp()
	_ = fso2.RemoveMergedApp()

}

func TestCollection_Minus_OutputQueue(t *testing.T) {
	// --------------------------------------------------------------
	// Connect to database
	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	fso2, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.5-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	fso1, err := LoadCollection(ctx, db, []*util.ImageRef{{"mariadb", "10.4-starlight"}})
	if err != nil {
		t.Fatal(err)
		return
	}

	// --------------------------------------------------------------
	// Build Collection
	fso1.Minus(fso2)
	outQueue, outOffsets, requiredLayer := fso1.GetOutputQueue()

	if err = util.ExportToJsonFile(
		outQueue,
		path.Join(os.TempDir(), fmt.Sprintf("fso-outQueue.json")),
	); err != nil {
		t.Fatal(err)
		return
	}
	if err = util.ExportToJsonFile(
		outOffsets,
		path.Join(os.TempDir(), fmt.Sprintf("fso-outOffsets.json")),
	); err != nil {
		t.Fatal(err)
		return
	}
	if err = util.ExportToJsonFile(
		requiredLayer,
		path.Join(os.TempDir(), fmt.Sprintf("fso-requiredLayer.json")),
	); err != nil {
		t.Fatal(err)
		return
	}
}
