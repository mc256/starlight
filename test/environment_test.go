package test

import (
	"fmt"
	"testing"
)

func TestHasLoginRegistry(t *testing.T) {
	fmt.Printf("HasLoginAWSECR: %v\n", HasLoginAWSECR())
	fmt.Printf("HasLoginStarlightGoharbor: %v\n", HasLoginStarlightGoharbor())
}
