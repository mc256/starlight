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
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"math"
	"net/http"
	"sort"
)

////////////////////////////////////////////////
type ImageLayer struct {
	stackIndex int64
	size       int64
	serial     int64
	hash       string
	digest     name.Digest
	available  bool
}

func (il ImageLayer) String() string {
	return fmt.Sprintf("[%05d:%02d]%s-%d", il.serial, il.stackIndex, il.hash, il.size)
}

type Content struct {
	files []*RankedFile
	rank  float64
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
	// stack in the existing image
	stack int64
	// reference to other layers that has the content of the file
	reference int64
}

type File struct {
	util.TOCEntry
	FsId int64 `json:"fsId"`
}

type Image struct {
	ref    name.Reference
	Serial int64
	Layers []*ImageLayer
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

	Source, Destination *Image
	manifest, config    []byte

	contents []*Content
}

func (b *Builder) String() string {
	return fmt.Sprintf("Builder (%s)->(%s)", b.Source.ref.String(), b.Destination.ref.String())
}

func (b *Builder) WriteHeader(w http.ResponseWriter, req *http.Request) error {
	var (
		contentLength, headerSize int64
	)
	/*
			// Write Header
		cw := util.NewCountWriter(w)
		gw, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
		if err != nil {
			return 0, 0, err
		}
		err = c.Write(gw, beautified)
		if err != nil {
			return 0, 0, err
		}
		err = gw.Close()
		if err != nil {
			return 0, 0, err
		}
		headerSize = cw.GetWrittenSize()
		contentLength = headerSize + c.Offsets[len(c.Offsets)-1]
		log.G(ib.ctx).WithFields(logrus.Fields{
			"headerSize":    headerSize,
			"contentLength": contentLength,
		}).Info("wrote image header")
		return headerSize, contentLength, nil
	*/

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	header.Set("Starlight-Header-Size", fmt.Sprintf("%d", headerSize))
	header.Set("Starlight-Version", util.Version)
	header.Set("Content-Disposition", `attachment; filename="starlight.tgz"`)
	w.WriteHeader(http.StatusOK)

	return nil
}

func (b *Builder) WriteBody(w http.ResponseWriter, req *http.Request) error {

	return nil
}

func (b *Builder) fetchLayers(cache *ImageLayer) error {
	var (
		c   *LayerCache
		has bool
	)

	b.server.cacheMutex.Lock()
	if c, has = b.server.cache[cache.hash]; has {
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
		b.server.cache[cache.hash] = c
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

	return nil
}

func (b *Builder) getImage(imageStr string) (img *Image, err error) {
	img = &Image{}
	img.ref, err = name.ParseReference(imageStr,
		name.WithDefaultRegistry(b.server.config.DefaultRegistry),
		name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}
	refName, refTag := ParseImageReference(img.ref, b.server.config.DefaultRegistry)
	img.Serial, err = b.server.db.GetImage(refName, refTag)
	if err != nil {
		return nil, err
	}

	// Load necessary layers
	img.Layers, err = b.server.db.GetLayers(img.Serial)
	if err != nil {
		return nil, err
	}
	for _, layer := range img.Layers {
		d := fmt.Sprintf("%s@%s", img.ref.Name(), layer.hash)
		layer.digest, err = name.NewDigest(d)
	}

	// Get Manifests

	return img, nil
}

func (b Builder) getManifestAndConfig(serial int64) (config, manifest []byte, err error) {
	return b.server.db.GetManifestAndConfig(serial)
}

func (b *Builder) getUnavailableLayers() (layers []*ImageLayer, err error) {
	dedup := make(map[string]bool, len(b.Destination.Layers))
	layers = make([]*ImageLayer, 0, len(b.Destination.Layers))
	if b.Source != nil {
		for _, v := range b.Source.Layers {
			dedup[v.hash] = true
		}
	}
	for _, v := range b.Destination.Layers {
		if _, has := dedup[v.hash]; !has {
			layers = append(layers, v)
		} else {
			dedup[v.hash] = true
		}
	}

	return layers, nil
}

func (b *Builder) ComputeDifferences() error {
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
				availableIds[layer.serial] = FromSource
			} else {
				unavailable = append(unavailable, layer)
			}
		}
	}

	for _, layer := range b.Destination.Layers {
		if layer.available {
			available = append(available, layer)
			availableIds[layer.serial] = FromDestination
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
		if fe, has := deduplicatedExistingFiles[f.Digest]; has && ((f.FsId < fe.FsId) || (availableIds[f.FsId] == FromDestination)) {
			continue
		}
		deduplicatedExistingFiles[f.Digest] = f
	}

	log.G(b.server.ctx).
		WithField("unique", len(deduplicatedExistingFiles)).
		WithField("total", len(existingFiles)).
		WithField("builder", b).
		Info("step 1 find existing file contents")

	// 2. compute the set of requested files from non-existing layers
	var requestedFiles []*RankedFile
	requestedFiles, err = b.server.db.GetFilesWithRanks(b.Destination.Serial)
	if err != nil {
		return errors.Wrapf(err, "failed to get requested files")
	}

	deduplicatedRequestedFiles := make(map[string]*Content)
	for _, f := range requestedFiles {
		if f.Digest == "" {
			continue
		}
		if _, has := availableIds[f.FsId]; has {
			continue
		}
		if fe, has := deduplicatedRequestedFiles[f.Digest]; has {
			fe.files = append(fe.files, f)
			continue
		}
		deduplicatedRequestedFiles[f.Digest] = &Content{
			files: []*RankedFile{f},
			rank:  0,
		}
	}

	log.G(b.server.ctx).
		WithField("unique", len(deduplicatedRequestedFiles)).
		WithField("total", len(requestedFiles)).
		WithField("builder", b).
		Info("step 2 find requested file contents")

	// 3. identify the best reference to the file content
	b.contents = make([]*Content, 0)
	for digest, reqFiles := range deduplicatedRequestedFiles {
		if existingContent, has := deduplicatedExistingFiles[digest]; has {
			for _, r := range reqFiles.files {
				r.reference = existingContent.FsId
			}
		} else {
			b.contents = append(b.contents, reqFiles)
			reqFiles.rank = math.MaxFloat64
			for _, f := range reqFiles.files {
				if f.rank < reqFiles.rank {
					reqFiles.rank = f.rank
				}
			}
		}
	}

	sort.Sort(ByRank(b.contents))

	log.G(b.server.ctx).
		WithField("content", len(b.contents)).
		WithField("builder", b).
		Info("step 3 identify the best reference to the file content")

	return nil
}

func NewBuilder(server *Server, src, dst string) (builder *Builder, err error) {
	builder = &Builder{
		server: server,
	}

	// Build
	if src != "" {
		if builder.Source, err = builder.getImage(src); err != nil {
			return nil, errors.Wrapf(err, "failed to obtain src image")
		}
	}

	if builder.Destination, err = builder.getImage(dst); err != nil {
		return nil, errors.Wrapf(err, "failed to obtain dst image")
	}

	var unavailableLayers, availableLayers []*ImageLayer
	if builder.Source != nil {
		availableLayers = builder.Source.Layers
		unavailableLayers, err = builder.getUnavailableLayers()
		if err != nil {
			return nil, err
		}
	} else {
		availableLayers = []*ImageLayer{}
		unavailableLayers = builder.Destination.Layers
	}

	for _, a := range availableLayers {
		a.available = true
	}
	for _, u := range unavailableLayers {
		u.available = false
	}

	// Load compressed layers from registry
	var errGrp errgroup.Group
	for _, layer := range unavailableLayers {
		layer := layer
		errGrp.Go(func() error {
			return builder.fetchLayers(layer)
		})
	}

	errGrp.Go(func() error {
		if c, m, err := builder.getManifestAndConfig(builder.Destination.Serial); err != nil {
			return err
		} else {
			builder.config = c
			builder.manifest = m
			return nil
		}
	})

	fmt.Println(availableLayers)

	// Wait for all jobs to end
	if err := errGrp.Wait(); err != nil {
		return nil, errors.Wrapf(err, "failed to load all compressed layer")
	}

	return builder, nil
}
