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
	db *Database
)

func TestMain(m *testing.M) {
	cfg, _, _, _ := LoadConfig("")
	ctx := context.Background()
	var err error
	db, err = NewDatabase(ctx, cfg.PostgresConnectionString)
	if err != nil {
		fmt.Printf("failed to connect to database %v\n", err)
	}
	m.Run()
}

func TestDatabase_Init(t *testing.T) {
	t.Skip("for dev only")

	ctx := context.Background()
	db, err := NewDatabase(ctx, "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
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
	t.Skip("for dev only")

	// TOCEntry to
	fl, err := db.GetUniqueFiles([]*send.ImageLayer{{Serial: 201}, {Serial: 211}, {Serial: 203}})
	if err != nil {
		t.Error(err)
	}
	for _, f := range fl {
		fmt.Println(f)
	}

}

func TestDatabase_GetFilesWithRanks(t *testing.T) {
	t.Skip("for dev only")

	// TOCEntry to
	fl, err := db.GetFilesWithRanks(45)
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
