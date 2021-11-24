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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mc256/starlight/util"
)

func TestCallbackFunction(t *testing.T) {
	var f func()
	fmt.Print("1.------")
	if f == nil {
		fmt.Println("test")
	}

	f = func() {
		fmt.Print("hello")
	}

	fmt.Print("2.------")
	f()

}

func TestNewReceiverFromFile(t *testing.T) {
	const (
		root = "/mnt/sandbox/receiver"
	)
	_ = os.MkdirAll(filepath.Join(root, "sfs"), 0755)

	ctx := util.ConfigLoggerWithLevel("trace")

	db, err := util.OpenDatabase(ctx, filepath.Join(root, "sfs"), "layer.db")
	if err != nil {
		t.Fatal(err)
		return
	}
	defer db.Close()

	layerStore, err := NewLayerStore(ctx, db, filepath.Join(root, "sfs"))
	if err != nil {
		t.Fatal(err)
		return
	}

	rec, err := NewReceiverFromFile(ctx, layerStore, "data/deltabundle-old.img", 498746)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Println(rec.name)
	//rec.ExtractFiles()
	rec.extractFiles()

}
