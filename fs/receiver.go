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
	bolt "go.etcd.io/bbolt"
	"io"
)

type Receiver struct {
	ctx        context.Context
	db         *bolt.DB
	layerStore *LayerStore

	// image are a list of gzip chunks (excluding the TOC header)
	image     io.Reader
	tocOffset int64

	// fsTemplate is a tree structure TOC. In case we want to create a new instance in fsInstances,
	// we must perform a deep-copy of this tree.
	fsTemplate []TocEntry
	// fsInstances that associates with this ImageReader.
	fsInstances []*FsInstance

	// imageLookupMap helps you to find the corresponding merger.Delta in imageMeta
	imageLookupMap map[string]int
	// layerLookupMap helps you to find the corresponding path to the directory
	layerLookupMap []*LayerMeta

	// entryMap allow us to flag whether a file is ready to read
	// structure:|-- [0] ------------------------[16]--------------------- ... from  imageMeta . Offsets
	//                |- []{entry1, entry2}       |-[]{entry3, entry4}
	entryMap     map[int64]*[]TocEntry
	chunkOffsets []int64

	// Image protocol
	name   string
	prefix string
}
