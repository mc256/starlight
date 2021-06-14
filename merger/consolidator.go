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

package merger

import (
	"context"
	"encoding/json"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"sort"
)

const (
	strictHashCheck = false
)

type ConsolidatorEntry struct {
	// entries has the same file content
	// the first entry consist of
	entries []*util.TraceableEntry

	// smaller means higher priority, it should be lowest landmark
	landmarkScore float32
}

type OutputEntry struct {
	Source       int   // maps to Consolidator.source
	SourceOffset int64 // offset in the source layer

	Offset         int64
	CompressedSize int64

	sourceDelta map[int]bool
}

var (
	superSuperHot = map[string]bool{
		// 10s
		"etc/localtime":   true,
		"etc/passwd":      true,
		"etc/hosts":       true,
		"etc/group":       true,
		"etc/resolv.conf": true,

		// 3s
		"bin/dash":        true,
		"bin/readlink":    true,
		"etc/ld.so.cache": true,

		// 1s
		//"entrypoint.sh":                     true,
		//"lib/x86_64-linux-gnu/ld-2.28.so":   true,
		//"lib/x86_64-linux-gnu/libc-2.28.so": true,
	}
)

type ByPriority []*ConsolidatorEntry

func (b ByPriority) Len() int {
	return len(b)
}

func (b ByPriority) Less(i, j int) bool {
	// should return true is i has higher priority
	if b[i].landmarkScore != b[j].landmarkScore {
		return b[i].landmarkScore < b[j].landmarkScore
	}
	if len(b[i].entries) != len(b[j].entries) {
		return len(b[i].entries) > len(b[j].entries)
	}
	if b[i].entries[0].Source != b[j].entries[0].Source {
		return b[i].entries[0].Source > b[j].entries[0].Source
	}
	return b[i].entries[0].Offset < b[j].entries[0].Offset
}
func (b ByPriority) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type Consolidator struct {
	ctx context.Context

	// uniqueMap is a hashmap that maps file hashes to ConsolidatorEntry
	// which is a list of entries.
	uniqueMap     map[string]*ConsolidatorEntry
	Deltas        []*Delta `json:"delta,omitempty"`
	layerImageMap map[int]int

	// Source starts from index 0
	Source []*util.TraceableBlobDigest `json:"source,omitempty"`

	consolidated bool
	outputQueue  []*OutputEntry
	Offsets      []int64 `json:"offsets"`
}

func (c *Consolidator) GetOutputQueue() *[]*OutputEntry {
	return &c.outputQueue
}

func (c *Consolidator) PopulateOffset() error {
	if c.consolidated {
		return util.ErrAlreadyConsolidated
	}
	c.consolidated = true

	// create output queue for content of the delta image
	c.outputQueue = make([]*OutputEntry, 0, len(c.uniqueMap))

	// Check file has same hash but different sizes
	if strictHashCheck {
		for k, v := range c.uniqueMap {
			size := int64(0)
			diff := false

			for i, item := range v.entries {
				if i == 0 {
					size = item.Size
				} else {
					if item.Size != size {
						diff = true
					}
				}
			}

			if diff {
				log.G(c.ctx).WithFields(logrus.Fields{
					"hash": k,
				}).Error("found same hash but different size")
				return util.ErrHashCollision
			}
		}
	}

	// Landmark list
	checkPoints := make([][]int64, len(c.Deltas))
	for i := range checkPoints {
		checkPoints[i] = make([]int64, MaxLandmark+1)
	}

	// Sort by priority
	// O(nlog(n))
	uniqueList := make([]*ConsolidatorEntry, 0, len(c.uniqueMap))
	c.Offsets = make([]int64, 0, len(c.uniqueMap))

	for _, v := range c.uniqueMap {
		uniqueList = append(uniqueList, v)
	}
	sort.Sort(ByPriority(uniqueList))

	offset := int64(0)
	for _, v := range uniqueList {
		prevOffset := offset
		sample := v.entries[0]

		// Update the offset to match the output queue
		// There might be multiple chunks, therefore we have to calculate
		// the sum of chunks
		// ---|------------------|--------------------
		//    ^ prevOffset       ^ offset
		var offsets []int64
		if sample.HasChunk() {
			offsets = make([]int64, len(sample.Chunks))
			for i, c := range sample.Chunks {
				offsets[i] = offset
				offset += c.CompressedSize
			}
		} else {
			offsets = []int64{
				offset,
			}
			offset += sample.CompressedSize
		}
		sample.SetDeltaOffset(&offsets)
		sourceDelta := c.layerImageMap[sample.GetSourceLayer()]

		// push to output queue
		outputEnt := &OutputEntry{
			Source:         sample.GetSourceLayer(),
			SourceOffset:   sample.Offset,
			Offset:         prevOffset,
			CompressedSize: offset - prevOffset,

			sourceDelta: map[int]bool{
				sourceDelta: true,
			},
		}
		c.outputQueue = append(c.outputQueue, outputEnt)

		//update check point
		if checkPoints[sourceDelta][sample.Landmark] < outputEnt.Offset {
			checkPoints[sourceDelta][sample.Landmark] = outputEnt.Offset
		}

		// set offset for the rest of entries
		// point the reference to the first one.
		for i := 1; i < len(v.entries); i++ {
			ent := v.entries[i]

			// offset
			ent.SetDeltaOffset(&offsets)

			// source
			sourceDelta := c.layerImageMap[ent.GetSourceLayer()]
			outputEnt.sourceDelta[sourceDelta] = true

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

			//update check point
			if checkPoints[sourceDelta][sample.Landmark] < outputEnt.Offset {
				checkPoints[sourceDelta][sample.Landmark] = outputEnt.Offset
			}

		}

		c.Offsets = append(c.Offsets, prevOffset)
	}

	/////////////////////////////////////////////////////////////////////////
	//// update landmark
	for i, d := range c.Deltas {
		d.CheckPoint = checkPoints[i]
	}

	c.Offsets = append(c.Offsets, offset) // eof

	return nil
}

// AddDelta image to the pool of images, All the
func (c *Consolidator) AddDelta(delta *Delta) error {
	// lock the delta image
	if c.consolidated {
		return util.ErrAlreadyConsolidated
	}
	delta.setConsolidated()

	// require to shift the source layer to match the indexes
	layerMax := len(c.Source)

	// O(n), n is the number of the file
	for priority, pl := range delta.Pool {
		for _, item := range pl {
			layerNumber := item.GetSourceLayer()
			if layerNumber <= 0 {
				continue
			}
			item.SetSourceLayer(layerNumber + layerMax)

			// We dont want to include meta data update or data type update
			if item.UpdateMeta == 1 || !item.IsDataType() {
				continue
			}

			// We also don't want to include files has size 0.
			// just need to keep them in TOC
			if item.Size == 0 {
				continue
			}

			// calculate the landmark score. The smaller the score is means higher priority
			if ce, exist := c.uniqueMap[item.Digest]; exist {
				count := len(ce.entries)
				ce.entries = append(ce.entries, item)
				if superSuperHot[item.Name] {
					ce.landmarkScore = -1.0
				}
				if ce.landmarkScore != -1.0 {
					ce.landmarkScore = (ce.landmarkScore*float32(count) + float32(priority)) / float32(count+1)
				}
			} else {
				if superSuperHot[item.Name] {
					c.uniqueMap[item.Digest] = &ConsolidatorEntry{
						entries:       []*util.TraceableEntry{item},
						landmarkScore: -1.0,
					}
				} else {
					c.uniqueMap[item.Digest] = &ConsolidatorEntry{
						entries:       []*util.TraceableEntry{item},
						landmarkScore: float32(priority),
					}
				}
			}
		}
	}

	// Add layer list to consolidator's layer list
	// and shift the indexes by layerMax
	// before:
	//	       delta ( 0, 1, 2, .. n)
	//  consolidator ( 0, 1, 2, .. layerMax - 1)
	// after:
	//  consolidator ( 0, 1, 2, .. layerMax - 1, layerMax, ... layerMax + n)
	c.Source = append(c.Source, delta.DigestList...)
	for i := 1; i <= len(delta.DigestList); i++ {
		c.layerImageMap[i+layerMax] = len(c.Deltas)
	}

	// Add delta image to consolidator
	c.Deltas = append(c.Deltas, delta)

	return nil
}

func (c *Consolidator) ExportTOC(w io.Writer, beautified bool) error {
	if c.consolidated == false {
		return util.ErrNotConsolidated
	}

	encoder := json.NewEncoder(w)
	if beautified {
		encoder.SetIndent("", "\t")
	}
	return encoder.Encode(c)
}

func NewConsolidator(ctx context.Context) *Consolidator {
	c := &Consolidator{
		uniqueMap:     make(map[string]*ConsolidatorEntry),
		Deltas:        make([]*Delta, 0, 2),
		consolidated:  false,
		ctx:           ctx,
		layerImageMap: make(map[int]int),
	}
	return c
}
