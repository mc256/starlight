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
	"github.com/mc256/stargz-snapshotter/estargz"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"io"
	"path"
)

const (
	SourceLayerUnboundedIndex = -1
	WhiteoutPrefix            = ".wh."
)

type Overlay struct {
	ctx context.Context
	db  *bolt.DB

	// root is the root of the file system
	Root *util.TraceableEntry `json:"-"`

	// pool stores the list of pointer to all the TOC entries
	EntryMap map[string]*util.TraceableEntry `json:"p,omitempty"`

	// digest list (reference to actual layer storage)
	// DigestList index range [0, n) where n is the total number of layers
	// proxy.TraceableEntry's "source" int is starting from 1 which needs to subtract one to
	// get the correct index
	DigestList []*util.TraceableBlobDigest `json:"d,omitempty"`

	ImageName string `json:"-"`
	ImageTag  string `json:"-"`

	config []byte
}

func NewOverlayBuilder(ctx context.Context, db *bolt.DB) (ov *Overlay) {
	root := util.GetRootNode()
	root.SetSourceLayer(SourceLayerUnboundedIndex)

	ov = &Overlay{
		ctx:  ctx,
		db:   db,
		Root: root,
		EntryMap: map[string]*util.TraceableEntry{
			".": root,
		},
		ImageName:  "",
		DigestList: make([]*util.TraceableBlobDigest, 0),
	}
	return ov
}

// Overlay functions

func (ov *Overlay) recursiveDelete(ent *estargz.TOCEntry) {
	delete(ov.EntryMap, ent.Name)
	for _, c := range ent.Children() {
		if c.IsDir() {
			ov.recursiveDelete(c)
		} else {
			delete(ov.EntryMap, c.Name)
		}
	}
	ent.RemoveAllChildren()
}

// recursiveAdd upper layer to the lower layer. Parameter lDir and uDir must be directories only.
func (ov *Overlay) recursiveAdd(lDir, uDir *estargz.TOCEntry, upperPool *map[string]*util.TraceableEntry) {
	// Merge all the child
	if lDir == nil {
		for _, upperChild := range uDir.Children() {
			ov.EntryMap[upperChild.Name] = (*upperPool)[upperChild.Name]
			if upperChild.IsDir() {
				ov.recursiveAdd(nil, upperChild, upperPool)
			}
		}
	} else {
		if uDir.HasChild(".wh..wh..opq") {
			// White out file uDir layer, ignore lDir layer
			for _, lowerChild := range lDir.Children() {
				ov.recursiveDelete(lowerChild)
			}
			lDir.RemoveAllChildren()
			for key, upperChild := range uDir.Children() {
				if upperChild.IsWhiteoutFile() {
					continue
				}
				lDir.AddChild(key, upperChild)
				ov.EntryMap[upperChild.Name] = (*upperPool)[upperChild.Name]
				if upperChild.IsDir() {
					// we have to continue so entries can be added to ov.EntryMap
					ov.recursiveAdd(nil, upperChild, upperPool)
				}
			}
			return
		}

		// We will decided whether to search in these directories later
		for key, lowerChild := range lDir.Children() {
			if lowerChild.IsWhiteoutFile() {
				lDir.RemoveChild(key)
				ov.recursiveDelete(lowerChild)
				continue
			}
			if _, hasWhiteOut := uDir.GetChild(WhiteoutPrefix + key); hasWhiteOut == true {
				lDir.RemoveChild(key)
				ov.recursiveDelete(lowerChild)
				continue
			}

			if upperChild, hasUpperChild := uDir.GetChild(key); hasUpperChild == true {
				if lowerChild.Type == upperChild.Type && lowerChild.Digest == upperChild.Digest {
					lowerChild.UpdateMetadataFrom(upperChild)
					if lowerChild.IsDir() {
						ov.recursiveAdd(lowerChild, upperChild, upperPool)
					}
				} else {
					lDir.RemoveChild(key)
					ov.recursiveDelete(lowerChild)
					lDir.AddChild(key, upperChild)
					ov.EntryMap[upperChild.Name] = (*upperPool)[upperChild.Name]
					if upperChild.IsDir() {
						// we have to continue so entries can be added to ov.EntryMap
						ov.recursiveAdd(nil, upperChild, upperPool)
					}
				}
			}

			// unchanged
		}

		for key, upperChild := range uDir.Children() {
			if upperChild.IsWhiteoutFile() {
				continue
			}

			// if we found the same file
			if _, hasLowerChild := lDir.GetChild(key); hasLowerChild == false {
				lDir.AddChild(key, upperChild)
				ov.EntryMap[upperChild.Name] = (*upperPool)[upperChild.Name]
				if upperChild.IsDir() {
					// we have to continue so entries can be added to ov.EntryMap
					ov.recursiveAdd(nil, upperChild, upperPool)
				}
			}

		}

	}

}

// Add overlays a single layer on top of what exists in the Overlay object.
func (ov *Overlay) Add(tb *util.TraceableBlobDigest) error {
	/*
		log.G(ov.ctx).WithFields(logrus.Fields{
			"digest": tb.Digest.String(),
			"image":  tb.ImageName,
		}).Debug("add layer to toc")
	*/

	tx, err := ov.db.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// read toc from storage
	blob := tx.Bucket([]byte("blob"))
	if blob == nil {
		return bolt.ErrBucketNotFound
	}
	layer := blob.Bucket([]byte(tb.String()))
	if layer == nil {
		return bolt.ErrBucketNotFound
	}

	// Read TOC
	pool := make(map[string]*util.TraceableEntry)
	ov.DigestList = append(ov.DigestList, tb)
	idx := len(ov.DigestList) // starting from 1
	err = layer.ForEach(func(k, v []byte) error {
		ent := &util.TraceableEntry{}
		err := json.Unmarshal(v, ent)
		fn := string(k[:])
		if fn == ".prefetch.landmark" || fn == ".no.prefetch.landmark" {
			return nil
		}
		pool[fn] = ent
		ent.SetSourceLayer(idx)
		return err
	})
	if err != nil {
		return err
	}

	root := util.GetRootNode()
	pool["."] = root

	// Rebuild Tree
	for k, v := range pool {
		if k != "." {
			parent := path.Dir(k)
			parentNode, _ := pool[parent]
			if parentNode.Landmark > v.Landmark {
				parentNode.Landmark = v.Landmark
			}
			parentNode.AddChild(path.Base(k), v.TOCEntry)
		}
	}

	// Recursively merge layers
	ov.recursiveAdd(ov.Root.TOCEntry, root.TOCEntry, &pool)

	return nil
}

// AddImage overlays an entire image on top of what exists in the Overlay object.
func (ov *Overlay) AddImage(imageName, imageTag string) error {
	log.G(ov.ctx).WithFields(logrus.Fields{
		"name": imageName,
		"tag":  imageTag,
	}).Info("add image to toc")

	ov.ImageName = imageName
	ov.ImageTag = imageTag

	tx, err := ov.db.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// read layers from database
	img := tx.Bucket([]byte("image"))
	if img == nil {
		return bolt.ErrBucketNotFound
	}
	tag := img.Bucket([]byte(imageName))
	if tag == nil {
		return bolt.ErrBucketNotFound
	}
	bucket := tag.Bucket([]byte(imageTag))
	if bucket == nil {
		return bolt.ErrBucketNotFound
	}
	n := int(util.BToInt32(bucket.Get([]byte("count"))))
	for i := 0; i < n; i++ {
		hash := bucket.Get(util.Int32ToB(uint32(i)))
		if hash == nil {
			return util.ErrLayerNotFound
		}
		d, err := digest.Parse(string(hash[:]))
		if err != nil {
			return err
		}
		err = ov.Add(&util.TraceableBlobDigest{Digest: d, ImageName: imageName})
		if err != nil {
			return err
		}
	}

	ov.config = bucket.Get([]byte("config"))

	return nil
}

// ExportTOC writes the TOC in json to a writer
func (ov *Overlay) ExportTOC(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "\t")
	return encoder.Encode(ov)
}
