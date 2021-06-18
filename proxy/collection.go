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
// This data structure only use for data storage in the database
type Collection struct {
	ctx context.Context
	db  *bolt.DB

	Images []*util.ImageRef `json:"i"`

	Table []*util.OptimizedTraceableEntry `json:"t"`

	// DigestList starting from 0
	DigestList []*util.TraceableBlobDigest `json:"dl"`

	// ImageDigestReference maps Collection.Images 's layers to the
	// position in the Collection.DigestList.
	ImageDigestReference [][]int `json:"idr"`

	Configs []string `json:"cfg"`
}

func (c *Collection) Minus(old *Collection) {
	shift := len(c.DigestList)
	i, j := 0, 0
	for {
		if i >= len(c.Table) {
			break
		}
		if j >= len(old.Table) {
			break
		}
		want := c.Table[i]
		have := old.Table[j]
		res := util.CompareByFilename(want, have)
		want.UpdateMeta = 0 // send file content by default

		if res == 1 { // want < have
			i++
		} else if res == -1 { // want > have
			j++
		} else if res == 0 { // found the same file
			if have.Digest == want.Digest {
				if have.IsDataType() && have.Size > 0 {
					want.UpdateMeta = 1
					want.ConsolidatedSource = have.GetSourceLayer() + shift
				}
			}

			i++
			j++
		}
	}

	c.DigestList = append(c.DigestList, old.DigestList...)
}

func (c *Collection) getOutputQueue() (outputQueue []*util.OutputEntry, outputOffsets []int64, requiredLayers []bool) {
	// Deduplicate
	uniqueMap := make(map[string]*util.OptimizedTraceableEntries)
	for _, ent := range c.Table {
		if ent.IsDataType() && ent.Size > 0 {
			if ent.UpdateMeta == 0 {
				if m, ok := uniqueMap[ent.Digest]; ok {
					m.List = append(m.List, ent)
					r := ent.ComputeRank()
					if r < m.Ranking {
						m.Ranking = r
					}
				} else {
					uniqueMap[ent.Digest] = &util.OptimizedTraceableEntries{
						List:    []*util.OptimizedTraceableEntry{ent},
						Ranking: ent.ComputeRank(),
					}
				}
			}
		}
	}

	// Sort
	uniqueList := make([]*util.OptimizedTraceableEntries, 0, len(uniqueMap))
	for _, v := range uniqueMap {
		uniqueList = append(uniqueList, v)
	}
	sort.Sort(util.ByRanking(uniqueList))

	// Output Queue
	requiredLayers = make([]bool, len(c.DigestList)+1)

	// Update offsets in the entries
	offset := int64(0)
	for _, v := range uniqueList {
		prevOffset := offset
		sample := v.List[0]

		// Update the offset to match the output queue
		// There might be multiple chunks, therefore we have to calculate
		// the sum of chunks
		// ---|------------------|--------------------
		//    ^ prevOffset       ^ offset
		var offsets []int64
		if sample.HasChunk() { // multiple parts
			offsets = make([]int64, len(sample.Chunks))
			for i, c := range sample.Chunks {
				offsets[i] = offset
				offset += c.CompressedSize
			}
		} else { // single part
			offsets = []int64{offset}
			offset += sample.CompressedSize
		}
		sample.SetDeltaOffset(&offsets)
		requiredLayers[sample.GetSourceLayer()] = true

		// push to output queue
		outputEnt := &util.OutputEntry{
			Source:         sample.GetSourceLayer(),
			SourceOffset:   sample.Offset,
			Offset:         prevOffset,
			CompressedSize: offset - prevOffset,
		}

		outputQueue = append(outputQueue, outputEnt)

		// set offset for the rest of entries
		// point the reference to the first one.
		for i := 1; i < len(v.List); i++ {
			ent := v.List[i]

			// offset
			ent.SetDeltaOffset(&offsets)

			// update meta data
			ent.ChunkSize = sample.ChunkSize
			ent.CompressedSize = sample.CompressedSize
			ent.Offset = sample.Offset
			if sample.GetSourceLayer() != ent.GetSourceLayer() {
				ent.ConsolidatedSource = sample.GetSourceLayer()
			} // same file might appear multiple times in the same layer

			// update chunk information
			if ent.Chunks != nil {
				for i, c := range ent.Chunks {
					c.Digest = ""
					c.Offset = sample.Chunks[i].Offset
					c.ChunkOffset = sample.Chunks[i].ChunkOffset
					c.ChunkSize = sample.Chunks[i].ChunkSize
					c.ChunkDigest = sample.Chunks[i].ChunkDigest
					c.CompressedSize = sample.Chunks[i].CompressedSize
				}
			}
		}

		outputOffsets = append(outputOffsets, prevOffset)
	}
	outputOffsets = append(outputOffsets, offset) // last one is the content-length
	return
}

func (c *Collection) getClientFsTemplates() [][]*util.TraceableEntry {
	pool := make([][]*util.TraceableEntry, len(c.Images))
	for _, ent := range c.Table {
		pool[ent.SourceImage] = append(pool[ent.SourceImage], ent.TraceableEntry)
	}
	return pool
}

func (c *Collection) ComposeDeltaBundle() (out *util.OutputCollection) {
	outQueue, outOffsets, requiredLayer := c.getOutputQueue()
	out = &util.OutputCollection{
		Image:                c.Images,
		Table:                c.getClientFsTemplates(),
		Config:               c.Configs,
		DigestList:           c.DigestList,
		ImageDigestReference: c.ImageDigestReference,
		Offsets:              outOffsets,
		OutputQueue:          outQueue,
		RequiredLayer:        requiredLayer,
	}

	return
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

func (c *Collection) RemoveMergedApp() error {
	tx, err := c.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Commit()

	store, err := tx.CreateBucketIfNotExists([]byte("collection"))
	if err != nil {
		return err
	}
	return store.Delete([]byte(util.ByImageName(c.Images).String()))
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
		Configs:              make([]string, 0),
	}

	// digestMap indexes starting from Zero therefore we need to add 1
	digestMap := make(map[string]int, 0)
	for sourceIdx, img := range imageList {
		// load merged images
		overlay, err := merger.LoadMergedImage(ctx, db, img.ImageName, img.ImageTag)
		if err != nil {
			return nil, err
		}

		c.Images = append(c.Images, img.DeepCopy())
		layerRef := make([]int, 1, 1)

		// layer mapping
		layerUpdateMap := make(map[int]int, 0)
		layerUpdateMap[-1] = -1
		layerUpdateMap[0] = 0
		for i, dig := range overlay.DigestList {
			if idx, ok := digestMap[dig.String()]; ok {
				layerUpdateMap[i+1] = idx + 1
				layerRef = append(layerRef, idx)
			} else {
				idx := len(c.DigestList) + 1
				digestMap[dig.String()] = idx
				layerUpdateMap[i+1] = idx
				layerRef = append(layerRef, idx)
				c.DigestList = append(c.DigestList, dig)
			}
		}
		c.ImageDigestReference = append(c.ImageDigestReference, layerRef)

		for _, file := range overlay.EntryMap {
			ent := &util.OptimizedTraceableEntry{
				TraceableEntry: file,
				SourceImage:    sourceIdx,
				AccessCount:    0,
				SumRank:        0,
				SumSquaredRank: 0,
			}
			ent.SetSourceLayer(layerUpdateMap[ent.GetSourceLayer()])
			c.Table = append(c.Table, ent)
		}

		c.Configs = append(c.Configs, string(overlay.Config[:]))
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
	buf := store.Get([]byte(util.ByImageName(imageList).String()))
	if buf == nil {
		return newCollection(ctx, db, imageList)
	}

	log.G(ctx).WithField("collection", util.ByImageName(imageList).String()).Info("load collection from db")
	c := &Collection{}
	err = json.Unmarshal(buf, c)
	if err != nil {
		return nil, err
	}
	c.ctx = ctx
	c.db = db

	for _, ent := range c.Table {
		ent.SetSourceLayer(ent.Source)
	}

	return c, nil
}
