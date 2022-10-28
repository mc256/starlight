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
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"io"
	"math"
	"net/http"
	"sort"
)

////////////////////////////////////////////////
type ImageLayer struct {
	stackIndex int64
	size       int64
	Serial     int64  `json:"f"`
	Hash       string `json:"h"`
	digest     name.Digest
	available  bool
	blob       *LayerCache
}

func (il ImageLayer) String() string {
	return fmt.Sprintf("[%05d:%02d]%s-%d", il.Serial, il.stackIndex, il.Hash, il.size)
}

type Content struct {
	files []*RankedFile

	// highest rank of all the files using this content
	rank float64

	// stack identify which layer should this content be placed, all the files will be referencing the content
	Stack int64 `json:"stack"`

	// offset is non-zero if the file is in the delta bundle body
	Offset int64 `json:"offset,omitempty"`

	// size is the size of the compressed content
	Size int64 `json:"size"`

	Chunks []*FileChunk `json:"chunks"`
}

type ByRank []*Content

func (b ByRank) Len() int {
	return len(b)
}

func (b ByRank) Less(i, j int) bool {
	return b[i].rank < b[j].rank
}

func (b ByRank) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type RankedFile struct {
	File

	// rank of the file, smaller has the higher priority
	rank float64

	// Stack in the existing image from bottom to top
	Stack int64 `json:"ST"`

	// if the file is available on the client then ReferenceFsId is non-zero,
	// expecting the file is available on the client and can be accessed using the File.Digest .
	ReferenceFsId int64 `json:"RFS,omitempty"`

	// if the file is not available on the client then ReferenceFsId is zero and ReferenceStack is non-zero,
	// expecting the file content in the delta bundle body
	ReferenceStack int64 `json:"RST,omitempty"`
	// if the file is not available on the client then PayloadOrder is non-zero shows when this file can be ready
	PayloadOrder int `json:"PO,omitempty"`
}

type FileChunk struct {
	Offset         int64  `json:"offset"`
	ChunkOffset    int64  `json:"chunkOffset"`
	ChunkSize      int64  `json:"chunkSize"`
	ChunkDigest    string `json:"chunkDigest"`
	CompressedSize int64  `json:"compressedSize"`
}

type File struct {
	util.TOCEntry
	Chunks []*FileChunk `json:"chunks,omitempty"`
	fsId   int64
}

type Image struct {
	ref    name.Reference
	Serial int64         `json:"s"`
	Layers []*ImageLayer `json:"l"`
}

type LayerSource bool

const (
	FromDestination LayerSource = false
	FromSource                  = true
)

func (i Image) String() string {
	return fmt.Sprintf("%d->%v", i.Serial, i.Layers)
}

type Builder struct {
	server *Server

	Source      *Image `json:"s"`
	Destination *Image `json:"d"`

	manifest, config []byte
	manifestDigest   string

	// contents and contentLength are computed by Builder.computeDelta()
	Contents      []*Content `json:"c"`
	contentLength int64

	// requestedFiles is the ToC with reference to the file content
	RequestedFiles []*RankedFile

	unavailableLayers, availableLayers []*ImageLayer
}

func (b *Builder) String() string {
	if b.Source != nil {
		return fmt.Sprintf("Builder (%s)->(%s)", b.Source.ref.String(), b.Destination.ref.String())
	}
	return fmt.Sprintf("Builder ()->(%s)", b.Destination.ref.String())
}

func (b *Builder) WriteHeader(w http.ResponseWriter, req *http.Request) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	cw := util.NewCountWriter(buf)

	// manifest
	gwManifest, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return err
	}
	_, err = gwManifest.Write(b.manifest)
	if err != nil {
		return err
	}
	err = gwManifest.Close()
	if err != nil {
		return err
	}
	manifestSize := cw.GetWrittenSize()

	// config
	gwConfig, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return err
	}
	_, err = gwConfig.Write(b.config)
	if err != nil {
		return err
	}
	err = gwConfig.Close()
	if err != nil {
		return err
	}
	configSize := cw.GetWrittenSize() - manifestSize

	// header
	h, err := json.Marshal(b)
	if err != nil {
		return err
	}
	gwHeader, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return err
	}
	_, err = gwHeader.Write(h)
	if err != nil {
		return err
	}
	err = gwHeader.Close()
	if err != nil {
		return err
	}
	headerSize := cw.GetWrittenSize() - manifestSize - configSize

	log.G(b.server.ctx).
		WithField("manifest", manifestSize).
		WithField("config", configSize).
		WithField("header", headerSize).
		Info("generated response header")

	// output header
	contentLength := cw.GetWrittenSize() + b.contentLength
	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	header.Set("Starlight-Header-Size", fmt.Sprintf("%d", headerSize))
	header.Set("Manifest-Size", fmt.Sprintf("%d", manifestSize))
	header.Set("Config-Size", fmt.Sprintf("%d", configSize))
	header.Set("Digest", b.manifestDigest)
	header.Set("Starlight-Version", util.Version)
	header.Set("Content-Disposition", `attachment; filename="starlight.tgz"`)
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	return err
}

func (b *Builder) WriteBody(w http.ResponseWriter, req *http.Request) error {
	layers := b.Destination.Layers

	// output body
	for _, c := range b.Contents {
		// fmt.Println(c.files[0].Name, len(c.Chunks), c.Stack, len(c.files), c.Offset, c.Size, layers[c.Stack].blob)
		if layer := layers[c.Stack]; layer.blob != nil {
			sr := io.NewSectionReader(layer.blob.buffer, 0, layer.size)
			for _, chunk := range c.Chunks {
				ssr := io.NewSectionReader(sr, chunk.ChunkOffset, chunk.CompressedSize)
				_, err := io.CopyN(w, ssr, chunk.CompressedSize)
				if err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("layer %d not found for file %s (ref:%d)", c.Stack, c.files[0].Name, len(c.files))
		}
	}
	return nil
}

func (b *Builder) fetchLayers(cache *ImageLayer) error {
	var (
		c   *LayerCache
		has bool
	)

	b.server.cacheMutex.Lock()
	if c, has = b.server.cache[cache.Hash]; has {
		b.server.cacheMutex.Unlock()

		sub := make(chan error)
		c.Subscribe(&sub)
		err := <-sub
		if err != nil {
			log.G(b.server.ctx).
				WithField("layer", cache.String()).
				Error(errors.Wrapf(err, "failed to load layer"))
			return err
		}
		log.G(b.server.ctx).
			WithField("layer", cache.String()).
			WithField("shared", true).
			Info("fetched layer")
	} else {
		c = NewLayerCache(cache)
		b.server.cache[cache.Hash] = c
		b.server.cacheMutex.Unlock()

		if err := c.Load(b.server); err != nil {
			log.G(b.server.ctx).
				WithField("layer", cache.String()).
				Error(errors.Wrapf(err, "failed to load layer"))
			return err
		}
		log.G(b.server.ctx).
			WithField("layer", cache.String()).
			WithField("shared", false).
			Info("fetched layer")
	}

	cache.blob = c

	return nil
}

// getImage returns the image with the given reference and the platform.
// if you need to find out the available image, you should better use getImageByDigest which returns
// the exact image that is available as tags might be changed but the digest will not change.
func (b *Builder) getImage(ref, platform string) (img *Image, err error) {
	img = &Image{}
	img.ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(b.server.config.DefaultRegistry),
		name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}
	refName, refTag := ParseImageReference(img.ref, b.server.config.DefaultRegistry)
	img.Serial, err = b.server.db.GetImage(refName, refTag, platform)
	if err != nil {
		return nil, err
	}

	// Load necessary layers
	img.Layers, err = b.server.db.GetLayers(img.Serial)
	if err != nil {
		return nil, err
	}
	for _, layer := range img.Layers {
		d := fmt.Sprintf("%s@%s", img.ref.Name(), layer.Hash)
		layer.digest, err = name.NewDigest(d)
	}

	return img, nil
}

// getImageByDigest returns a more precise image reference by using digest (not tag)
// this guarantees that the image is the exact image that is available on the client side.
func (b Builder) getImageByDigest(refWithDigest string) (img *Image, err error) {
	img = &Image{}
	img.ref, err = name.ParseReference(refWithDigest,
		name.WithDefaultRegistry(b.server.config.DefaultRegistry),
		name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}
	refName, refTag := ParseImageReference(img.ref, b.server.config.DefaultRegistry)
	img.Serial, err = b.server.db.GetImageByDigest(refName, refTag)
	if err != nil {
		return nil, err
	}

	// Load necessary layers
	img.Layers, err = b.server.db.GetLayers(img.Serial)
	if err != nil {
		return nil, err
	}
	for _, layer := range img.Layers {
		d := fmt.Sprintf("%s@%s", img.ref.Name(), layer.Hash)
		layer.digest, err = name.NewDigest(d)
	}

	return img, nil
}

// getManifestAndConfig returns the manifest and config of the image by its serial number.
// This is important for creating an valid container image on the client side.
func (b Builder) getManifestAndConfig(serial int64) (config, manifest []byte, digest string, err error) {
	return b.server.db.GetManifestAndConfig(serial)
}

func (b *Builder) getUnavailableLayers() (layers []*ImageLayer, err error) {
	dedup := make(map[string]bool, len(b.Destination.Layers))
	layers = make([]*ImageLayer, 0, len(b.Destination.Layers))
	if b.Source != nil {
		for _, v := range b.Source.Layers {
			dedup[v.Hash] = true
		}
	}
	for _, v := range b.Destination.Layers {
		if _, has := dedup[v.Hash]; !has {
			layers = append(layers, v)
		} else {
			dedup[v.Hash] = true
		}
	}

	return layers, nil
}

func (b *Builder) computeDelta() error {
	/*
		1. compute the set of existing files from available layers
		2. compute the set of requested files from non-existing layers
		3. identify the best reference to the file content
		4. sort file contents
	*/
	unavailable, available := make([]*ImageLayer, 0), make([]*ImageLayer, 0)
	availableIds := make(map[int64]LayerSource)

	if b.Source != nil {
		for _, layer := range b.Source.Layers {
			if layer.available {
				available = append(available, layer)
				availableIds[layer.Serial] = FromSource
			} else {
				unavailable = append(unavailable, layer)
			}
		}
	}

	for _, layer := range b.Destination.Layers {
		if layer.available {
			available = append(available, layer)
			availableIds[layer.Serial] = FromDestination
		} else {
			unavailable = append(unavailable, layer)
		}
	}

	// 1. compute the set of existing files from available layers
	existingFiles, err := b.server.db.GetUniqueFiles(available)
	if err != nil {
		return errors.Wrapf(err, "failed to get existing files")
	}

	deduplicatedExistingFiles := make(map[string]*File)
	for _, f := range existingFiles {
		// find the lowest layer, if available find it in the destination image, so we can create references within the
		// requested image and reduce future cleanup steps
		if fe, has := deduplicatedExistingFiles[f.Digest]; has && ((f.fsId < fe.fsId) || (availableIds[f.fsId] == FromDestination)) {
			continue
		}
		deduplicatedExistingFiles[f.Digest] = f
	}

	log.G(b.server.ctx).
		WithField("unique", len(deduplicatedExistingFiles)).
		WithField("total", len(existingFiles)).
		WithField("builder", b).
		WithField("_step", 1).
		Info("find existing file contents")

	// 2. compute the set of requested files from non-existing layers
	b.RequestedFiles, err = b.server.db.GetFilesWithRanks(b.Destination.Serial)
	if err != nil {
		return errors.Wrapf(err, "failed to get requested files")
	}

	deduplicatedRequestedContents := make(map[string]*Content)
	for _, f := range b.RequestedFiles {
		if f.Digest == "" {
			continue
		}
		if f.Size == 0 {
			continue
		}
		if _, has := availableIds[f.fsId]; has {
			continue
		}
		if fe, has := deduplicatedRequestedContents[f.Digest]; has {
			fe.files = append(fe.files, f)
			continue
		}
		deduplicatedRequestedContents[f.Digest] = &Content{
			files: []*RankedFile{f},
			rank:  0,
		}
	}

	log.G(b.server.ctx).
		WithField("unique", len(deduplicatedRequestedContents)).
		WithField("total", len(b.RequestedFiles)).
		WithField("builder", b).
		WithField("_step", 2).
		Info("find requested file contents")

	// 3. identify the best reference to the file content
	b.Contents = make([]*Content, 0)
	for digest, reqContent := range deduplicatedRequestedContents {
		if existingContent, has := deduplicatedExistingFiles[digest]; has {
			// found requested file in existing files
			for _, r := range reqContent.files {
				r.ReferenceFsId = existingContent.fsId
			}
		} else {
			// not found, add to the list of contents to be sent to the client
			b.Contents = append(b.Contents, reqContent)
			reqContent.rank = math.MaxFloat64
			reqContent.Stack = reqContent.files[0].Stack
			for _, f := range reqContent.files {
				if f.rank < reqContent.rank {
					// highest rank wins
					reqContent.rank = f.rank
				}
				if f.Stack < reqContent.Stack {
					// lowest stack wins
					reqContent.Stack = f.Stack
				}
			}
		}
	}

	sort.Sort(ByRank(b.Contents))

	b.contentLength = int64(0)
	for idx, c := range b.Contents {
		// update the reference to the content
		for _, f := range c.files {
			f.ReferenceStack = c.Stack
			f.PayloadOrder = idx
		}

		c.Offset = b.contentLength

		if len(c.files[0].Chunks) > 0 {
			for _, chunk := range c.files[0].Chunks {
				c.Size += chunk.CompressedSize
			}
			c.Chunks = c.files[0].Chunks
		} else {
			c.Size = c.files[0].CompressedSize
			c.Chunks = []*FileChunk{{
				Offset:         c.files[0].Offset,
				ChunkOffset:    c.files[0].ChunkOffset,
				ChunkSize:      c.files[0].ChunkSize,
				ChunkDigest:    c.files[0].ChunkDigest,
				CompressedSize: c.files[0].CompressedSize,
			}}
		}

		// calculate the total size of the compressed contents
		b.contentLength += c.Size
	}

	log.G(b.server.ctx).
		WithField("content", len(b.Contents)).
		WithField("builder", b).
		WithField("compressedSize", b.contentLength).
		WithField("_step", 3).
		Info("find the best file content references")

	return nil
}

func (b *Builder) Load() error {
	// Load compressed layers from registry
	var errGrp errgroup.Group
	for _, layer := range b.unavailableLayers {
		layer := layer
		errGrp.Go(func() error {
			return b.fetchLayers(layer)
		})
	}

	// Load manifest and config from proxy's database
	errGrp.Go(func() error {
		if c, m, digest, err := b.getManifestAndConfig(b.Destination.Serial); err != nil {
			return err
		} else {
			b.config = c
			b.manifest = m
			b.manifestDigest = digest
			return nil
		}
	})

	// Computer the difference between the requested and existing files
	errGrp.Go(b.computeDelta)

	// Wait for all jobs to end
	if err := errGrp.Wait(); err != nil {
		return errors.Wrapf(err, "failed to load all compressed layer")
	}

	return nil
}

func NewBuilder(server *Server, src, dst, plt string) (b *Builder, err error) {
	b = &Builder{
		server: server,
	}

	// Available Image:
	// It must be specified by the unique hash of the image, we don't want to refercing something that is
	// unknown (for security reasons). Therefore, platform is not required.
	if src != "" {
		if b.Source, err = b.getImageByDigest(src); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("source image %s not found", src)
			} else {
				return nil, errors.Wrapf(err, "failed to get source image")
			}
		}
	}

	// Requested Image:
	// requires image name tag as well as the platform to be specified
	// you could not download an image index for that because only one platform is needed to run the container
	if b.Destination, err = b.getImage(dst, plt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("requested image %s not found", dst)
		} else {
			return nil, errors.Wrapf(err, "failed to obtain requested image")
		}
	}

	if b.Source != nil {
		b.availableLayers = b.Source.Layers
		b.unavailableLayers, err = b.getUnavailableLayers()
		if err != nil {
			return nil, err
		}
	} else {
		b.availableLayers = []*ImageLayer{}
		b.unavailableLayers = b.Destination.Layers
	}

	for _, a := range b.availableLayers {
		a.available = true
	}
	for _, u := range b.unavailableLayers {
		u.available = false
	}

	return b, nil
}
