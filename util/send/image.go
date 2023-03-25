/*
   file created by Junlin Chen in 2022

*/

package send

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util/common"
)

type ImageLayer struct {
	StackIndex       int64  `json:"-"`
	UncompressedSize int64  `json:"s"`
	Serial           int64  `json:"f"`
	Hash             string `json:"h"`
	digest           name.Digest
	Available        bool               `json:"-"`
	Blob             *common.LayerCache `json:"-"`
}

func (il *ImageLayer) SetDigest(d name.Digest) {
	il.digest = d
}

func (il *ImageLayer) Digest() name.Digest {
	return il.digest
}

func (il *ImageLayer) Size() int64 {
	return il.UncompressedSize
}

func (il *ImageLayer) String() string {
	return fmt.Sprintf("[%05d:%02d]%s-%d", il.Serial, il.StackIndex, il.Hash, il.Size())
}

type Content struct {
	// Files used to find the highest rank, WILL NOT BE EXPORTED.
	// please use RequestedFiles instead
	Files []*RankedFile `json:"-"`

	// Rank WILL NOT BE EXPORTED. We do not want to send it to the client.
	// highest rank of all the files using this content
	Rank float64 `json:"-"`

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

type ByRank []*Content

func (b ByRank) Len() int {
	return len(b)
}

func (b ByRank) Less(i, j int) bool {
	return b[i].Rank < b[j].Rank
}

func (b ByRank) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type SignalContent struct {
	*Content
	Signal chan interface{}
}

func NewSignalContent(c *Content) *SignalContent {
	return &SignalContent{
		Content: c,
		Signal:  make(chan interface{}),
	}
}

type RankedFile struct {
	File

	// rank of the file, smaller has the higher priority
	Rank float64 `json:"-"`

	// Stack in the existing image from bottom to top
	Stack int64 `json:"S"`

	// if the file is available on the client then ReferenceFsId is non-zero,
	// expecting the file is available on the client and can be accessed using the File.Digest .
	ReferenceFsId int64 `json:"R,omitempty"`

	// if the file is not available on the client then ReferenceFsId is zero and ReferenceStack is non-zero,
	// expecting the file content in the delta bundle body
	ReferenceStack int64 `json:"T,omitempty"`
	// if the file is not available on the client then PayloadOrder is non-zero shows when this file can be ready
	PayloadOrder int `json:"O,omitempty"`
}

type FileChunk struct {
	Offset         int64 `json:"o"`
	ChunkOffset    int64 `json:"c"`
	ChunkSize      int64 `json:"h"`
	CompressedSize int64 `json:"s"`
}

type FileChunkParsing struct {
	Offset         int64 `json:"offset"`
	ChunkOffset    int64 `json:"chunkOffset"`
	ChunkSize      int64 `json:"chunkSize"`
	CompressedSize int64 `json:"compressedSize"`
}

type File struct {
	common.TOCEntry
	ParsingChunks []*FileChunkParsing `json:"chunks,omitempty"` // compatible with old version
	FsId          int64               `json:"-"`
}

type Image struct {
	Ref    name.Reference `json:"-"`
	Serial int64          `json:"s"`
	Layers []*ImageLayer  `json:"l"`
}

type LayerSource bool

const (
	FromDestination LayerSource = false
	FromSource                  = true
)

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
	RequestedFiles []*RankedFile `json:"rf"`
}
