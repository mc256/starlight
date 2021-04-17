/*
   Copyright The starlight Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   file created by maverick in 2021
*/

package util

import (
	"context"
	"encoding/binary"
	"github.com/containerd/containerd/log"
	bolt "go.etcd.io/bbolt"
)

const (
	TocStorage = "data/tocStorage.db"
)

func OpenDatabase(ctx context.Context) *bolt.DB {
	// Open Database
	db, err := bolt.Open(TocStorage, 0600, nil)
	if err != nil {
		log.G(ctx).Fatal(err)
		return nil
	}
	return db
}

func Int32ToB(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func Int64ToB(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func BToInt32(v []byte) uint32 {
	return binary.BigEndian.Uint32(v)
}

func BToInt64(v []byte) uint64 {
	return binary.BigEndian.Uint64(v)
}
