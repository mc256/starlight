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
	"github.com/mc256/starlight/util"
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
