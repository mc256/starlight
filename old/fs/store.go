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

package fs

import (
	"context"
	"fmt"
	"github.com/mc256/starlight/util/common"
	"os"
	"path"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

type LayerMeta struct {
	// Basic
	absPath  string
	complete bool
	writable bool

	completeMux sync.Mutex

	// Connection to Store
	store  *LayerStore
	digest common.TraceableBlobDigest
}

func (ls *LayerMeta) ToByte() []byte {
	bb := byte(0)
	if ls.writable {
		bb |= 0b01
	}
	if ls.complete {
		bb |= 0b10
	}

	return append(
		[]byte{bb},
		[]byte(ls.absPath)...,
	)
}

func (ls *LayerMeta) String() string {
	attr := ""

	if ls.complete {
		attr += "C"
	} else {
		attr += "_"
	}

	if ls.writable {
		attr += "W"
	} else {
		attr += "_"
	}

	return fmt.Sprintf("[%s]%s", attr, ls.absPath)
}

func (ls *LayerMeta) AtomicIsComplete() bool {
	ls.completeMux.Lock()
	defer ls.completeMux.Unlock()

	return ls.complete
}

func (ls *LayerMeta) AtomicSetCompleted() error {
	ls.completeMux.Lock()
	defer ls.completeMux.Unlock()

	if !ls.complete {
		ls.complete = true
		if ls.store != nil {
			return ls.store.saveLayer(ls.digest, ls, false)
		}
	}
	return nil
}

func (ls *LayerMeta) IsWritable() bool {
	return ls.writable
}

func (ls *LayerMeta) GetAbsPath() string {
	return ls.absPath
}

func newLayerMetaFromByte(b []byte, s *LayerStore, d common.TraceableBlobDigest) *LayerMeta {
	bb := b[0]

	return &LayerMeta{
		absPath:  string(b[1:]),
		writable: bb&0b01 == 0b01,
		complete: bb&0b10 == 0b10,

		completeMux: sync.Mutex{},

		store:  s,
		digest: d,
	}
}

func NewLayerMeta(absPath string, writable, complete bool) *LayerMeta {
	return &LayerMeta{
		absPath:  absPath,
		complete: complete,
		writable: writable,

		completeMux: sync.Mutex{},
	}
}

type LayerStore struct {
	ctx context.Context
	db  *bolt.DB

	cache map[common.TraceableBlobDigest]*LayerMeta

	workDir string
}

func (s *LayerStore) GetWorkDir() string {
	return s.workDir
}

func (s *LayerStore) saveLayer(digest common.TraceableBlobDigest, lay *LayerMeta, create bool) error {
	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	layers, err := tx.CreateBucketIfNotExists([]byte("layers"))
	if err != nil {
		return err
	}

	if err = layers.Put([]byte(digest.String()), lay.ToByte()); err != nil {
		return err
	}

	if create {
		if err = os.MkdirAll(lay.absPath, 0755); err != nil {
			return err
		}
	} else {
		log.G(s.ctx).WithFields(logrus.Fields{
			"digest": digest,
			"path":   lay.absPath,
		}).Debug("updated layer store")
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// =====================================================================================================================
// Register

func (s *LayerStore) RegisterLayerWithAbsolutePath(digest common.TraceableBlobDigest, lay *LayerMeta) (*LayerMeta, error) {
	lay.store = s
	lay.digest = digest

	if err := s.saveLayer(digest, lay, true); err != nil {
		return nil, err
	}
	s.cache[digest] = lay

	log.G(s.ctx).WithFields(logrus.Fields{
		"digest": digest,
		"path":   lay.absPath,
	}).Debug("registered layer")
	return lay, nil
}

func (s *LayerStore) RegisterLayerWithPath(digest common.TraceableBlobDigest, lay *LayerMeta) (*LayerMeta, error) {
	lay.absPath = path.Join(s.workDir, lay.absPath)
	return s.RegisterLayerWithAbsolutePath(digest, lay)
}

func (s *LayerStore) RegisterLayerWithPrefix(prefix string, count int, digest common.TraceableBlobDigest, writable, complete bool) (*LayerMeta, error) {
	lay := &LayerMeta{
		absPath:  path.Join(s.workDir, prefix, fmt.Sprintf("%d", count)),
		writable: writable,
		complete: complete,
	}
	return s.RegisterLayerWithAbsolutePath(digest, lay)
}

func (s *LayerStore) RegisterLayer(digest common.TraceableBlobDigest, writable, complete bool) (*LayerMeta, error) {
	lay := &LayerMeta{
		absPath:  path.Join(s.workDir, fmt.Sprintf("%d-%d", time.Now().Day(), time.Now().Nanosecond())),
		writable: writable,
		complete: complete,
	}
	return s.RegisterLayerWithAbsolutePath(digest, lay)
}

// =====================================================================================================================
// Lookup

func (s *LayerStore) FindLayer(digest common.TraceableBlobDigest) (*LayerMeta, error) {
	if lm, ok := s.cache[digest]; ok {
		return lm, nil
	}

	tx, err := s.db.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	layers := tx.Bucket([]byte("layers"))
	if layers == nil {
		return nil, common.ErrLayerNotFound
	}

	p := layers.Get([]byte(digest.String()))
	if p == nil {
		return nil, common.ErrLayerNotFound
	}

	lay := newLayerMetaFromByte(p, s, digest)

	s.cache[digest] = lay

	log.G(s.ctx).WithFields(logrus.Fields{
		"digest": digest,
		"path":   lay.absPath,
	}).Debug("found layer")

	return lay, nil
}

// =====================================================================================================================
// Remove

func (s *LayerStore) RemoveLayer(digest common.TraceableBlobDigest, removeFile bool) error {
	if l, hl := s.cache[digest]; hl {
		l.store = nil
		delete(s.cache, digest)
	}

	tx, err := s.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	layers, err := tx.CreateBucketIfNotExists([]byte("layers"))
	if err != nil {
		return err
	}

	p := layers.Get([]byte(digest.String()))
	if p == nil {
		return common.ErrLayerNotFound
	}
	pp := newLayerMetaFromByte(p, s, digest)

	if removeFile {
		err = os.RemoveAll(pp.absPath)
		if err != nil {
			return err
		}
	}

	if err = layers.Delete([]byte(digest.String())); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"digest": digest,
		"path":   pp.absPath,
		"del":    removeFile,
	}).Debug("removed layer")

	return nil
}

// =====================================================================================================================
// New Store

func NewLayerStore(ctx context.Context, db *bolt.DB, workDir string) (*LayerStore, error) {
	fs := &LayerStore{
		ctx:     ctx,
		db:      db,
		cache:   make(map[common.TraceableBlobDigest]*LayerMeta),
		workDir: workDir,
	}
	return fs, nil
}
