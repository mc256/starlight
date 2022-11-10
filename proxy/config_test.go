/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, _, _, _ := LoadConfig("")
	fmt.Println(cfg)
}
