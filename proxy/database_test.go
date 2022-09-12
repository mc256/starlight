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
	cfg := LoadConfig()
	var err error
	db, err = NewDatabase(cfg.PostgresConnectionString)
	if err != nil {
		fmt.Printf("failed to connect to database %v\n", err)
	}
	m.Run()
}

func TestGetImage(t *testing.T) {

}
