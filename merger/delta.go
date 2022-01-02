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
	"compress/gzip"
	"context"
	"encoding/json"
	"io"

	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
)

const (
	MaxLandmark = 2
)

type Delta struct {
	ctx context.Context

	// DigestList references to actual layer storage
	DigestList []*util.TraceableBlobDigest `json:"d,omitempty"`

	// start from the secondOffset are the layers that we want to transit to
	secondOffset int

	// Pool stores the list of pointer to all the TOC entries
	Pool [][]*util.TraceableEntry `json:"p,omitempty"`

	// file queue
	CheckPoint []int64

	consolidated bool

	ImageName string `json:"in,omitempty"`
	ImageTag  string `json:"it,omitempty"`

	Config string `json:"config"`
}

func (d *Delta) recursiveDelta(aDir, bDir *util.TOCEntry, bSource *Overlay) {
	if aDir == nil && bDir != nil {
		// copy b dir to the list recursively
		for _, entry := range bDir.Children() {
			n := bSource.EntryMap[entry.Name].DeepCopy()
			n.ShiftSource(d.secondOffset)
			d.Pool[n.Landmark] = append(d.Pool[n.Landmark], n)
			if entry.IsDir() {
				d.recursiveDelta(nil, entry, bSource)
			}
		}
	} else if aDir != nil && bDir != nil {
		opaque := true
		var pendingDelete []string
		numLinkDelta := 0

		// find difference
		for fileName, aChild := range aDir.Children() {
			if bChild, sameName := bDir.GetChild(fileName); sameName == true {
				opaque = false
				if bChild.Type != aChild.Type || bChild.Digest != aChild.Digest {
					n := bSource.EntryMap[bChild.Name].DeepCopy()
					n.ShiftSource(d.secondOffset)
					d.Pool[n.Landmark] = append(d.Pool[n.Landmark], n)
					if bChild.IsDir() {
						if aChild.IsDir() {
							d.recursiveDelta(aChild, bChild, bSource)
						} else {
							d.recursiveDelta(nil, bChild, bSource)
						}
					}
				} else {
					// These two files are the same but the timestamp might be different.
					// Update Metadata
					n := bSource.EntryMap[bChild.Name].DeepCopy()
					n.UpdateMeta = 1
					n.SetSourceLayer(aChild.GetSourceLayer())
					d.Pool[n.Landmark] = append(d.Pool[n.Landmark], n)

					if bChild.IsDir() {
						d.recursiveDelta(aChild, bChild, bSource)
					}
				}
			} else {
				pendingDelete = append(pendingDelete, fileName)
				if aChild.IsDir() {
					numLinkDelta--
				}
			}
		}

		// Remove files
		if opaque == true && len(aDir.Children()) != 0 {
			d.Pool[aDir.Landmark()] = append(d.Pool[aDir.Landmark()], util.ExtendEntry(util.MakeOpaqueWhiteoutFile(aDir.Name)))
		} else {
			for _, fileName := range pendingDelete {
				d.Pool[aDir.Landmark()] = append(d.Pool[aDir.Landmark()], util.ExtendEntry(util.MakeWhiteoutFile(fileName, aDir.Name)))
			}
		}

		// Add new files
		for fileName, bChild := range bDir.Children() {
			if _, sameName := aDir.GetChild(fileName); sameName != true {
				n := bSource.EntryMap[bChild.Name].DeepCopy()
				n.ShiftSource(d.secondOffset)
				d.Pool[n.Landmark] = append(d.Pool[n.Landmark], n)
				if bChild.IsDir() {
					numLinkDelta++
					d.recursiveDelta(nil, bChild, bSource)
				}
			}
		}

		if numLinkDelta != 0 {
			n := bSource.EntryMap[bDir.Name].DeepCopy()
			n.ShiftSource(d.secondOffset)
			d.Pool[n.Landmark] = append(d.Pool[n.Landmark], n)
		}
	} else { // aDir != nil && bDir == nil
		// remains
	}

}

// GetDelta moves from a to b.
// b is the targeted image
func GetDelta(ctx context.Context, a, b *Overlay) (d *Delta) {
	d = &Delta{
		ctx:          ctx,
		DigestList:   append(a.DigestList, b.DigestList...),
		secondOffset: len(a.DigestList),
		Pool:         make([][]*util.TraceableEntry, 0),
		CheckPoint:   make([]int64, 0),
		consolidated: false,
		ImageName:    b.ImageName,
		ImageTag:     b.ImageTag,
		Config:       string(b.Config),
	}

	for i := 0; i <= MaxLandmark; i++ {
		d.Pool = append(d.Pool, make([]*util.TraceableEntry, 0))
	}

	// Size Offset
	d.recursiveDelta(a.Root.TOCEntry, b.Root.TOCEntry, b)

	log.G(d.ctx).WithFields(logrus.Fields{
		"layerCount": len(d.DigestList),
	}).Info("delta toc prepared")

	return
}

func (d *Delta) setConsolidated() {
	d.consolidated = true
}

func (d *Delta) PopulateOffset() error {
	if d.consolidated {
		return util.ErrAlreadyConsolidated
	}

	offset := int64(0)
	for _, p := range d.Pool { // priority
		for _, item := range p { // entry
			_ = item.GetSourceLayer()
			if item.UpdateMeta == 1 || !item.IsDataType() {
				continue
			}

			var offsets []int64
			if item.Chunks != nil {
				offsets = make([]int64, len(item.Chunks))
				for i, c := range item.Chunks {
					offsets[i] = offset
					offset += c.CompressedSize
				}
			} else {
				offsets = []int64{
					offset,
				}
				offset += item.CompressedSize
			}
			item.SetDeltaOffset(&offsets)
		}
		d.CheckPoint = append(d.CheckPoint, offset)
	}
	d.consolidated = true

	return nil
}

// ExportTOC writes TOC for this image
// It must runs after the delta image has been consolidated (so that we have the correct offset point to the gzip chunks)
func (d *Delta) ExportTOC(w io.Writer, beautify bool) error {
	if d.consolidated == false && beautify == false {
		return util.ErrNotConsolidated
	}

	encoder := json.NewEncoder(w)
	if beautify {
		encoder.SetIndent("", "\t")
	}

	return encoder.Encode(d)
}

// OutputHeader writes compressed header for this delta image and returns the header size
// It must runs after the delta image has been consolidated (so that we have the correct offset point to the gzip chunks)
func (d *Delta) OutputHeader(w io.Writer) (headerSize int64, err error) {
	if d.consolidated == false {
		return 0, util.ErrNotConsolidated
	}

	cw := util.NewCountWriter(w)
	gw, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	err = d.ExportTOC(gw, false)
	if err != nil {
		return 0, err
	}
	err = gw.Close()
	if err != nil {
		return 0, err
	}
	headerSize = cw.GetWrittenSize()
	log.G(d.ctx).WithFields(logrus.Fields{
		"headerSize":  headerSize,
		"checkPoints": d.CheckPoint,
	}).Info("delta image header prepared")

	return
}
