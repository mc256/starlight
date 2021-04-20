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
	"testing"
)

func TestCacheToc(t *testing.T) {
	const (
		ContainerRegistry = "http://10.219.31.127:5000"
	)

	ctx := util.ConfigLogger()
	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	if err = CacheToc(ctx, db, "ubuntu", "18.04-starlight", ContainerRegistry); err != nil {
		t.Fatal(err)
		return
	}

}
