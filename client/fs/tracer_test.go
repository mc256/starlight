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

package fs

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/mc256/starlight/util"
)

// Test trace collection
func TestLoadTraces(t *testing.T) {
	ctx := util.ConfigLogger()
	tc, err := NewTraceCollection(ctx, os.TempDir())
	if err != nil {
		t.Fatalf("failed to create trace collection: %v", err)
	}


	buf, err := json.MarshalIndent(tc, "", "\t")
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	_ = ioutil.WriteFile(path.Join(os.TempDir(), "group-optimize.json"), buf, 0644)

	// check if file exists
	_, err = os.Stat(path.Join(os.TempDir(), "group-optimize.json"))
	if err != nil {
		t.Fatalf("file does not exists: %v", err)
	}
}
