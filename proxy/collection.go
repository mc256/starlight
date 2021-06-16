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

package proxy

import (
	"context"
	"encoding/json"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/fs"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	bolt "go.etcd.io/bbolt"
	"sort"
)

// Collection defines the operations between set of files
type Collection struct {
	ctx context.Context
	db  *bolt.DB

	Images []*util.ImageRef `json:"i"`

	Table []*util.OptimizedTraceableEntry `json:"t"`

	DigestList []*util.TraceableBlobDigest `json:"dl"`

	// ImageDigestReference maps Collection.Images 's layers to the
	// position in the Collection.DigestList.
	ImageDigestReference [][]int `json:"idr"`
}

func (c *Collection) AddOptimizeTrace(opt *fs.OptimizedGroup) {
	tableMap := make(map[string]*util.OptimizedTraceableEntry)
	for _, ent := range c.Table {
		tableMap[ent.Key()] = ent
	}
	for _, h := range opt.History {
		if ent, ok := tableMap[h.Key()]; ok {
			ent.AddRanking(h.Rank)
		}
	}
}

func (c *Collection) SaveMergedApp() error {
	tx, err := c.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Commit()

	store, err := tx.CreateBucketIfNotExists([]byte("collection"))
	if err != nil {
		return err
	}
	_ = tx.DeleteBucket([]byte(util.ByImageName(c.Images).String()))

	buf, err := json.Marshal(c)
	if err != nil {
		return err
	}
	err = store.Put([]byte(util.ByImageName(c.Images).String()), buf)
	if err != nil {
		return err
	}
	return nil
}

func newCollection(ctx context.Context, db *bolt.DB, imageList []*util.ImageRef) (*Collection, error) {
	log.G(ctx).WithField("collection", util.ByImageName(imageList).String()).Info("build from merged images")

	// Load from new
	c := &Collection{
		ctx: ctx,
		db:  db,

		Images:               make([]*util.ImageRef, 0),
		Table:                make([]*util.OptimizedTraceableEntry, 0),
		DigestList:           make([]*util.TraceableBlobDigest, 0),
		ImageDigestReference: make([][]int, 0),
	}

	digestMap := make(map[string]int, 0)
	for siidx, img := range imageList {
		// load merged images
		overlay, err := merger.LoadMergedImage(ctx, db, img.ImageName, img.ImageTag)
		if err != nil {
			return nil, err
		}

		c.Images = append(c.Images, img.DeepCopy())
		layerRef := make([]int, 0)

		// layer mapping
		layerUpdateMap := make(map[int]int, 0)
		layerUpdateMap[-1] = -1
		for i, dig := range overlay.DigestList {
			if idx, ok := digestMap[dig.String()]; ok {
				layerUpdateMap[i] = idx
				layerRef = append(layerRef, idx)
			} else {
				idx := len(c.DigestList)
				digestMap[dig.String()] = idx
				layerUpdateMap[i] = idx
				layerRef = append(layerRef, idx)
				c.DigestList = append(c.DigestList, dig)
			}
		}
		c.ImageDigestReference = append(c.ImageDigestReference, layerRef)

		for _, file := range overlay.EntryMap {
			ent := &util.OptimizedTraceableEntry{
				TraceableEntry: file,
				SourceImage:    siidx,
				AccessCount:    0,
				SumRank:        0,
				SumSquaredRank: 0,
			}
			ent.SetSourceLayer(layerUpdateMap[ent.GetSourceLayer()])
			c.Table = append(c.Table, ent)
		}

	}

	// shift source layer if necessary
	sort.Sort(util.ByFilename(c.Table))
	return c, nil
}

// LoadCollection creates a new operator that combine
// imageList should be sorted inorder
func LoadCollection(ctx context.Context, db *bolt.DB, imageList []*util.ImageRef) (*Collection, error) {

	// Check database
	tx, err := db.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	store := tx.Bucket([]byte("collection"))
	if store == nil {
		return newCollection(ctx, db, imageList)
	}

	log.G(ctx).WithField("collection", util.ByImageName(imageList).String()).Info("load collection from db")
	buf := store.Get([]byte(util.ByImageName(imageList).String()))
	if buf == nil {
		return newCollection(ctx, db, imageList)
	}
	c := &Collection{}
	err = json.Unmarshal(buf, c)
	if err != nil {
		return nil, err
	}
	c.ctx = ctx
	c.db = db

	return c, nil
}
