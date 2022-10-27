/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"fmt"
	"testing"
)

var (
	db *Database
)

func TestMain(m *testing.M) {
	cfg, _, _, _ := LoadConfig("")
	var err error
	db, err = NewDatabase(cfg.PostgresConnectionString)
	if err != nil {
		fmt.Printf("failed to connect to database %v\n", err)
	}
	m.Run()
}

func TestDatabase_GetFiles(t *testing.T) {
	// TOCEntry to
	fl, err := db.GetUniqueFiles([]*ImageLayer{{Serial: 201}, {Serial: 211}, {Serial: 203}})
	if err != nil {
		t.Error(err)
	}
	for _, f := range fl {
		fmt.Println(f)
	}

}

func TestDatabase_GetFilesWithRanks(t *testing.T) {
	// TOCEntry to
	fl, err := db.GetFilesWithRanks(45)
	if err != nil {
		t.Error(err)
	}
	for _, f := range fl {
		fmt.Println(f)
	}

}
