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
	"testing"

	"github.com/mc256/starlight/test"
	"github.com/mc256/starlight/util"
)

// Before running these test cases, we need to prepare Starlight format container images in the registry.
// These test cases are testing redis:6.0-starlight and redis:5.0-starlight

func TestCacheToc1(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	ctx := util.ConfigLogger()
	db, err := util.OpenDatabase(
		ctx,
		test.GetSandboxDirectory(t),
		test.GetProxyDBName(),
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	if err = CacheToc(ctx, db, "redis", "6.0-starlight", containerRegistry); err != nil {
		t.Fatal(err)
		return
	}
}

func TestCacheToc2(t *testing.T) {
	containerRegistry := test.GetContainerRegistry(t)

	ctx := util.ConfigLogger()
	db, err := util.OpenDatabase(
		ctx,
		test.GetSandboxDirectory(t),
		test.GetProxyDBName(),
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	if err = CacheToc(ctx, db, "redis", "5.0-starlight", containerRegistry); err != nil {
		t.Fatal(err)
		return
	}

	if err = CacheToc(ctx, db, "redis", "6.0-starlight", containerRegistry); err != nil {
		t.Fatal(err)
		return
	}

}
