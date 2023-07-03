/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/util/send"
)

var (
	db  *Database
	cfg *Configuration
)

func TestMain(m *testing.M) {
	cfg, _, _, _ = LoadConfig("")
	ctx := context.Background()
	var err error
	db, err = NewDatabase(ctx, cfg.PostgresConnectionString)
	if err != nil {
		fmt.Printf("failed to connect to database %v\n", err)
	}
	m.Run()
}

func TestDatabase_Init(t *testing.T) {
	ctx := context.Background()
	db, err := NewDatabase(ctx, cfg.PostgresConnectionString)
	if err != nil {
		t.Error(err)
		return
	}

	if err = db.InitDatabase(); err != nil {
		t.Error(err)
		return
	}
	fmt.Println("done")
}

func TestDatabase_GetFiles(t *testing.T) {
	fl, err := db.GetUniqueFiles([]*send.ImageLayer{{Serial: 33}, {Serial: 34}, {Serial: 35}})
	if err != nil {
		t.Error(err)
	}
	for _, f := range fl {
		fmt.Println(f)
	}
}

func TestDatabase_GetFilesWithRanks(t *testing.T) {
	fl, err := db.GetFilesWithRanks(5)
	if err != nil {
		t.Error(err)
	}
	for _, f := range fl {
		fmt.Println(f)
	}
}

func TestDatabase_UpdateFileRanks(t *testing.T) {
	t.Skip("for dev only")

	p := "/tmp/group-optimize.json"
	b, err := os.ReadFile(p)
	if err != nil {
		t.Error(err)
		return
	}
	var col *fs.TraceCollection
	err = json.Unmarshal(b, &col)

	var arr [][][]int64
	arr, err = db.UpdateFileRanks(col)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(arr)
}
