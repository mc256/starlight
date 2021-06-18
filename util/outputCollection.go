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
	"encoding/json"
	"io"
)

type OutputEntry struct {
	Source       int   // maps to Consolidator.source
	SourceOffset int64 // offset in the source layer

	Offset         int64
	CompressedSize int64
}

type OutputCollection struct {
	Image                []*ImageRef            `json:"refs"`
	Table                [][]*TraceableEntry    `json:"tables"`
	Config               []string               `json:"config"`
	DigestList           []*TraceableBlobDigest `json:"digests"`
	ImageDigestReference [][]int                `json:"idr"`
	Offsets              []int64                `json:"offsets"`

	OutputQueue   []*OutputEntry `json:"-"`
	RequiredLayer []bool         `json:"-"` // requiredLayer index starts from 1 not 0
}

func (oc OutputCollection) Json() (buf []byte) {
	buf, _ = json.Marshal(oc)
	return
}

func (oc OutputCollection) Write(w io.Writer, beautify bool) error {
	encoder := json.NewEncoder(w)
	if beautify {
		encoder.SetIndent("", "\t")
	}
	return encoder.Encode(oc)
}
