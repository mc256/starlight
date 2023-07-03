/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"testing"
)

func TestStarlightProxy_Ping(t *testing.T) {
	proxy := NewStarlightProxy(context.TODO(), "https", "starlight.yuri.moe")
	//proxy.auth = *url.UserPassword("username", "password")

	if _, _, _, err := proxy.Ping(); err != nil {
		t.Error(err)
	}
}
