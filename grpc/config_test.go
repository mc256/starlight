/*
   file created by Junlin Chen in 2022

*/

package grpc

import (
	"fmt"
	"testing"
)

func TestParseProxyStrings1(t *testing.T) {
	k, c, err := ParseProxyStrings("starlight-shared,https,starlight.yuri.moe,,")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(k, c)
}
