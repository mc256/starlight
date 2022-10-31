/*
   file created by Junlin Chen in 2022

*/

package receive

import (
	"fmt"
	"github.com/mc256/starlight/util/common"
	"path/filepath"
)

type ImageLayer struct {
	Size   int64  `json:"s"`
	Serial int64  `json:"f"`
	Hash   string `json:"h"`

	// path to the local storage
	Local string
}

func (il ImageLayer) String() string {
	return fmt.Sprintf("[%05d:%02d]%s-%d", il.Serial, -1, il.Hash, il.Size)
}

type Content struct {
	Signal chan interface{} `json:"-"`

	// ------------------------------------------
	// stack identify which layer should this content be placed, all the files will be referencing the content
	Stack int64 `json:"t"`

	// offset is non-zero if the file is in the delta bundle body
	Offset int64 `json:"o,omitempty"`

	// size is the size of the compressed content
	Size int64 `json:"s"`

	Chunks []*FileChunk `json:"c"`

	Digest string `json:"d"`
}

func (c *Content) GetBaseDir() string {
	return filepath.Join(c.Digest[7:8], c.Digest[8:10], c.Digest[10:12])
}

func (c *Content) GetPath() string {
	return filepath.Join(c.GetBaseDir(), c.Digest[12:])
}

type ReferencedFile struct {
	File

	// Stack in the existing image from bottom to top
	Stack int64 `json:"S"`

	// if the file is available on the client then ReferenceFsId is non-zero,
	// expecting the file is available on the client and can be accessed using the File.Digest .
	// (This is Serial not Stack)
	ReferenceFsId int64 `json:"R,omitempty"`

	// if the file is not available on the client then ReferenceFsId is zero and ReferenceStack is non-zero,
	// expecting the file content in the delta bundle body
	// (This is Stack not Serial)
	ReferenceStack int64 `json:"T,omitempty"`

	// if the file is not available on the client then PayloadOrder is non-zero shows when this file can be ready
	PayloadOrder int `json:"O,omitempty"`

	Ready *chan interface{} `json:"-"`
}

func (r *ReferencedFile) ExistingFsIndex() (layerSerial int64, existing bool) {
	if r.ReferenceFsId > 0 {
		return r.ReferenceFsId, true
	}
	return -1, false
}

func (r *ReferencedFile) InPayload() (stack int64, inPayload bool) {
	if r.ReferenceStack > 0 {
		return r.ReferenceStack, true
	}
	return -1, false
}

type FileChunk struct {
	Offset         int64 `json:"o"`
	ChunkOffset    int64 `json:"c"`
	ChunkSize      int64 `json:"h"`
	CompressedSize int64 `json:"s"`
}

type File struct {
	common.TOCEntry
	Chunks []*FileChunk `json:"c,omitempty"`
	FsId   int64        `json:"-"`
}

type Image struct {
	Serial int64         `json:"s"`
	Layers []*ImageLayer `json:"l"`
}

func (i Image) String() string {
	return fmt.Sprintf("%d->%v", i.Serial, i.Layers)
}

type DeltaBundle struct {
	Source      *Image `json:"s"`
	Destination *Image `json:"d"`

	// contents and BodyLength are computed by Builder.computeDelta()
	Contents   []*Content `json:"c"`
	BodyLength int64      `json:"bl"`

	// RequestedFiles are all the files in the requested images
	// Use this to reconstruct the file system
	RequestedFiles []*ReferencedFile `json:"rf"`
}
