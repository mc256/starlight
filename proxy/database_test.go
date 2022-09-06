/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"testing"
)

func TestInsertImage(t *testing.T) {
	cfg := GetConfig()
	db, err := NewDatabase(cfg.PostgresConnectionString)
	if err != nil {
		t.Error(err)
	}
	_, _ = db.InsertImage("testimg", "3.2.3", "123123qwe", &v1.ConfigFile{}, &v1.Manifest{})
}
