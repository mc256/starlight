/*
   file created by Junlin Chen in 2022

*/

package util

import (
	"fmt"
	"math/rand"
	"time"
)

var (
	letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

func randSequence(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func GetRandomId(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, randSequence(10))
}
