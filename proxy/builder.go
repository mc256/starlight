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
	"net/http"
)

////////////////////////////////////////////////
type ImageLayer struct {
	stackIndex int64
	size       int64
	hash       string
	digest     name.Digest
}

func (il ImageLayer) String() string {
	return fmt.Sprintf("[%02d]%s-%d", il.stackIndex, il.hash, il.size)
}

type RankedFile struct {
	File
	rank float64
}

type File struct {
	util.TOCEntry
	stack int64
}

type Image struct {
	ref    name.Reference
	Serial int64
	Layers []*ImageLayer
}

func (i Image) String() string {
	return fmt.Sprintf("%d->%v", i.Serial, i.Layers)
}

type Builder struct {
	server *Server
	layers []*LayerCache

	Source, Destination *Image

	manifest, config []byte
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

func (b *Builder) getRoughDeduplicatedLayers() (layers []*ImageLayer, err error) {
	layers, err = b.server.db.GetRoughDeduplicatedLayers(b.Source.Serial, b.Destination.Serial)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to obtain compute needed layers")
	}
	for _, layer := range layers {
		d := fmt.Sprintf("%s@%s", b.Destination.ref.Name(), layer.hash)
		layer.digest, err = name.NewDigest(d)
	}
	return layers, nil
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

	var layers []*ImageLayer
	if builder.Source != nil {
		layers, err = builder.getRoughDeduplicatedLayers()
		if err != nil {
			return nil, err
		}
	} else {
		layers = builder.Destination.Layers
	}

	// Load compressed layers from registry
	var errGrp errgroup.Group
	for _, layer := range layers {
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

	// Wait for all jobs to end
	if err := errGrp.Wait(); err != nil {
		return nil, errors.Wrapf(err, "failed to load all compressed layer")
	}

	return builder, nil
}
