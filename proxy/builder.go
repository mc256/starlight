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
	"io"
	"math"
	"net/http"
	"sort"

	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util"
	"github.com/mc256/starlight/util/common"
	"github.com/mc256/starlight/util/send"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

////////////////////////////////////////////////

type Builder struct {
	send.DeltaBundle

	// non-exported fields
	server *Server

	manifest, config []byte
	manifestDigest   string

	disableSorting bool

	unavailableLayers, availableLayers []*send.ImageLayer
}

func (b *Builder) String() string {
	if b.Source != nil {
		return fmt.Sprintf("Builder (%s)->(%s)", b.Source.Ref.String(), b.Destination.Ref.String())
	}
	return fmt.Sprintf("Builder ()->(%s)", b.Destination.Ref.String())
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

	slDigest := digest.FromBytes(h)

	log.G(b.server.ctx).
		WithField("manifest", manifestSize).
		WithField("config", configSize).
		WithField("header", headerSize).
		WithField("body", b.BodyLength).
		WithField("_digest", slDigest.String()).
		Info("generated response header")

	// output header
	//
	// Content-Length equeals
	// compressed(Starlight-Header-Size) + compressed(Manifest-Size) + compressed((Config-Size) + Payload-Size
	httpLength := cw.GetWrittenSize() + b.BodyLength

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Length", fmt.Sprintf("%d", httpLength))
	header.Set("Starlight-Header-Size", fmt.Sprintf("%d", headerSize))
	header.Set("Manifest-Size", fmt.Sprintf("%d", manifestSize))
	header.Set("Config-Size", fmt.Sprintf("%d", configSize))
	header.Set("Digest", b.manifestDigest)
	header.Set("Starlight-Digest", slDigest.String())
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
		if layer := layers[c.Stack]; layer.Blob != nil {
			sr := io.NewSectionReader(layer.Blob.Buffer, 0, layer.UncompressedSize)
			for _, chunk := range c.Chunks {
				ssr := io.NewSectionReader(sr, chunk.Offset, chunk.CompressedSize)
				_, err := io.CopyN(w, ssr, chunk.CompressedSize)
				if err != nil {
					return errors.Wrapf(err, "failed to copy chunk at [%d] for file [%s]", chunk.Offset, c.Files[0].Name)
				}
			}
		} else {
			return fmt.Errorf("layer %d not found for file %s (ref:%d)", c.Stack, c.Files[0].Name, len(c.Files))
		}
	}
	return nil
}

func (b *Builder) fetchLayers(cache *send.ImageLayer) error {
	var (
		c   *common.LayerCache
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
		c = common.NewLayerCache(cache)
		b.server.cache[cache.Hash] = c
		b.server.cacheMutex.Unlock()

		if err := c.Load(b.server.ctx); err != nil {
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

	cache.Blob = c

	return nil
}

// getImage returns the image with the given reference and the platform.
// if you need to find out the available image, you should better use getImageByDigest which returns
// the exact image that is available as tags might be changed but the digest will not change.
func (b *Builder) getImage(ref, platform string) (img *send.Image, err error) {
	img = &send.Image{}
	img.Ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(b.server.config.DefaultRegistry),
		name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}

	refName, refTag := ParseImageReference(img.Ref, b.server.config.DefaultRegistry, b.server.config.DefaultRegistryAlias)

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
		d := fmt.Sprintf("%s@%s", img.Ref.Name(), layer.Hash)
		var dd name.Digest
		dd, err = name.NewDigest(d)
		layer.SetDigest(dd)
	}

	return img, nil
}

// getImageByDigest returns a more precise image reference by using digest (not tag)
// this guarantees that the image is the exact image that is available on the client side.
func (b Builder) getImageByDigest(refWithDigest string) (img *send.Image, err error) {
	img = &send.Image{}
	img.Ref, err = name.ParseReference(refWithDigest,
		name.WithDefaultRegistry(b.server.config.DefaultRegistry),
		name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}
	refName, refTag := ParseImageReference(img.Ref, b.server.config.DefaultRegistry, b.server.config.DefaultRegistryAlias)
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
		d := fmt.Sprintf("%s@%s", img.Ref.Name(), layer.Hash)
		var dd name.Digest
		dd, err = name.NewDigest(d)
		layer.SetDigest(dd)
	}

	return img, nil
}

// getManifestAndConfig returns the manifest and config of the image by its serial number.
// This is important for creating an valid container image on the client side.
func (b Builder) getManifestAndConfig(serial int64) (config, manifest []byte, digest string, err error) {
	return b.server.db.GetManifestAndConfig(serial)
}

func (b *Builder) getUnavailableLayers() (layers []*send.ImageLayer, err error) {
	dedup := make(map[string]bool, len(b.Destination.Layers))
	layers = make([]*send.ImageLayer, 0, len(b.Destination.Layers))
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
	unavailable, available := make([]*send.ImageLayer, 0), make([]*send.ImageLayer, 0)
	availableIds := make(map[int64]send.LayerSource)

	if b.Source != nil {
		for _, layer := range b.Source.Layers {
			if layer.Available {
				available = append(available, layer)
				availableIds[layer.Serial] = send.FromSource
			} else {
				unavailable = append(unavailable, layer)
			}
		}
	}

	for _, layer := range b.Destination.Layers {
		if layer.Available {
			available = append(available, layer)
			availableIds[layer.Serial] = send.FromDestination
		} else {
			unavailable = append(unavailable, layer)
		}
	}

	log.G(b.server.ctx).
		WithField("available", len(available)).
		WithField("unavailable", len(unavailable)).
		Trace("compute delta")

	// 1. compute the set of existing files from available layers
	existingFiles, err := b.server.db.GetUniqueFiles(available)
	if err != nil {
		return errors.Wrapf(err, "failed to get existing files")
	}

	deduplicatedExistingFiles := make(map[string]*send.File)
	for _, f := range existingFiles {
		// find the lowest layer, if available find it in the destination image, so we can create references within the
		// requested image and reduce future cleanup steps

		if fe, has := deduplicatedExistingFiles[f.Digest]; has && ((f.FsId < fe.FsId) || (availableIds[f.FsId] == send.FromDestination)) {
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
	if b.disableSorting {
		b.RequestedFiles, err = b.server.db.GetFilesWithoutRanks(b.Destination.Serial)
	} else {
		b.RequestedFiles, err = b.server.db.GetFilesWithRanks(b.Destination.Serial)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get requested files")
	}

	deduplicatedRequestedContents := make(map[string]*send.Content)
	for _, f := range b.RequestedFiles {
		if f.Digest == "" {
			continue
		}
		if f.Size == 0 {
			continue
		}
		if _, has := availableIds[f.FsId]; has {
			continue
		}
		if fe, has := deduplicatedRequestedContents[f.Digest]; has {
			fe.Files = append(fe.Files, f)
			continue
		}
		deduplicatedRequestedContents[f.Digest] = &send.Content{
			Files: []*send.RankedFile{f},
			Rank:  0,
		}
	}

	log.G(b.server.ctx).
		WithField("unique", len(deduplicatedRequestedContents)).
		WithField("total", len(b.RequestedFiles)).
		WithField("builder", b).
		WithField("_step", 2).
		Info("find requested file contents")

	// 3. identify the best reference to the file content
	b.Contents = make([]*send.Content, 0)
	for d, reqContent := range deduplicatedRequestedContents {
		if existingContent, has := deduplicatedExistingFiles[d]; has {
			// found requested file in existing files
			for _, r := range reqContent.Files {
				r.ReferenceFsId = existingContent.FsId
			}
		} else {
			// not found, add to the list of contents to be sent to the client
			b.Contents = append(b.Contents, reqContent)
			reqContent.Rank = math.MaxFloat64
			reqContent.Stack = reqContent.Files[0].Stack
			for _, f := range reqContent.Files {
				if f.Rank < reqContent.Rank {
					// highest rank wins
					reqContent.Rank = f.Rank
				}
				if f.Stack < reqContent.Stack {
					// lowest stack wins
					reqContent.Stack = f.Stack
				}
			}
		}
	}

	if !b.disableSorting {
		sort.Sort(send.ByRank(b.Contents))
	} else {
		log.G(b.server.ctx).WithField("builder", b).Info("sorting disabled as requested")
	}

	b.BodyLength = int64(0)
	for idx, c := range b.Contents {
		// update the reference to the content
		for _, f := range c.Files {
			f.ReferenceStack = c.Stack
			f.PayloadOrder = idx
		}

		c.Offset = b.BodyLength
		c.Digest = c.Files[0].Digest
		if len(c.Files[0].ParsingChunks) > 0 {
			c.Chunks = make([]*send.FileChunk, 0, len(c.Files[0].ParsingChunks))
			for _, chunk := range c.Files[0].ParsingChunks {
				c.Size += chunk.CompressedSize
				c.Chunks = append(c.Chunks, &send.FileChunk{
					Offset:         chunk.Offset,
					ChunkOffset:    chunk.ChunkOffset,
					ChunkSize:      chunk.ChunkSize,
					CompressedSize: chunk.CompressedSize,
				})
			}
		} else {
			c.Size = c.Files[0].CompressedSize
			c.Chunks = []*send.FileChunk{{
				Offset:         c.Files[0].Offset,
				ChunkOffset:    c.Files[0].ChunkOffset,
				ChunkSize:      c.Files[0].ChunkSize,
				CompressedSize: c.Files[0].CompressedSize,
			}}
		}

		// calculate the total size of the compressed contents
		b.BodyLength += c.Size
	}

	log.G(b.server.ctx).
		WithField("content", len(b.Contents)).
		WithField("builder", b).
		WithField("compressedSize", b.BodyLength).
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
		if c, m, d, err := b.getManifestAndConfig(b.Destination.Serial); err != nil {
			return err
		} else {
			b.config = c
			b.manifest = m
			b.manifestDigest = d
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

func NewBuilder(server *Server, src, dst, plt string, disableSorting bool) (b *Builder, err error) {
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
		b.availableLayers = []*send.ImageLayer{}
		b.unavailableLayers = b.Destination.Layers
	}

	for _, a := range b.availableLayers {
		a.Available = true
	}
	for _, u := range b.unavailableLayers {
		u.Available = false
	}

	b.disableSorting = disableSorting

	return b, nil
}
