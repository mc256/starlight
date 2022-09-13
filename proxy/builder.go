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
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
)

// DeltaBundleBuilder deprecated feature
type DeltaBundleBuilder struct {
	ctx      context.Context
	registry string

	// layerReaders stores the layer needed to build the delta bundle
	// key is only the digest (no image name)
	layerReaders     map[string]*io.SectionReader
	layerReadersLock sync.Mutex

	client http.Client
}

func (ib *DeltaBundleBuilder) WriteBody(w io.Writer, c *util.ProtocolTemplate, wg *sync.WaitGroup) (err error) {
	wg.Wait()

	readers := make(map[int]*io.SectionReader, len(c.DigestList)+1)
	for i, d := range c.DigestList {
		if c.RequiredLayer[i+1] {
			readers[i+1] = ib.layerReaders[d.Digest.String()]
		}
	}
	for _, ent := range c.OutputQueue {
		log.G(ib.ctx).WithFields(logrus.Fields{
			"offset": ent.SourceOffset,
			"length": ent.CompressedSize,
			"source": ent.Source,
		}).Trace("request range")
		sr := io.NewSectionReader(readers[ent.Source], ent.SourceOffset, ent.CompressedSize)

		_, err := io.CopyN(w, sr, ent.CompressedSize)
		if err != nil {
			log.G(ib.ctx).WithFields(logrus.Fields{
				"Error": err,
			}).Warn("write body error")
			return err
		}
	}
	log.G(ib.ctx).Info("wrote image body")
	return nil
}

func (ib *DeltaBundleBuilder) WriteHeader(w io.Writer, c *util.ProtocolTemplate, wg *sync.WaitGroup, beautified bool) (headerSize int64, contentLength int64, err error) {

	for i, d := range c.DigestList {
		if c.RequiredLayer[i+1] {
			wg.Add(1)
			ib.fetchLayer(d.ImageName, d.Digest.String(), wg)
		}
	}

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
}

func (ib *DeltaBundleBuilder) fetchLayer(imageName, digest string, wg *sync.WaitGroup) {
	skip := false
	func() {
		ib.layerReadersLock.Lock()
		defer ib.layerReadersLock.Unlock()

		if _, ok := ib.layerReaders[digest]; ok {
			skip = true
			wg.Done()
		}
	}()
	if skip {
		return
	}
	go func() {
		url := ib.registry + path.Join("/v2", imageName, "blobs", digest)
		log.G(ib.ctx).WithFields(logrus.Fields{
			"url": url,
		}).Debug("resolving blob")

		// parse image name
		ctx, cf := context.WithTimeout(ib.ctx, 3600*time.Second)
		defer cf()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.G(ib.ctx).WithError(err).Error("request error")
			return
		}

		resp, err := ib.client.Do(req)
		if err != nil {
			log.G(ib.ctx).WithError(err).Error("fetch blob error")
			return
		}

		length, err := strconv.ParseInt(resp.Header["Content-Length"][0], 10, 64)
		if err != nil {
			log.G(ib.ctx).WithError(err).Error("blob no length information")
		}

		log.G(ib.ctx).WithFields(logrus.Fields{
			"url":  url,
			"code": resp.StatusCode,
			"size": length,
		}).Debug("resolved blob")

		buf := new(bytes.Buffer)
		if _, err = io.CopyN(buf, resp.Body, length); err != nil {
			log.G(ib.ctx).WithError(err).Error("blob read")
			return
		}

		func() {
			ib.layerReadersLock.Lock()
			defer ib.layerReadersLock.Unlock()

			ib.layerReaders[digest] = io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, length)
			wg.Done()
		}()

	}()
}

func NewDeltaBundleBuilder(ctx context.Context, registry string) *DeltaBundleBuilder {
	ib := &DeltaBundleBuilder{
		ctx:              ctx,
		registry:         registry,
		layerReaders:     make(map[string]*io.SectionReader, 0),
		layerReadersLock: sync.Mutex{},
		client:           http.Client{},
	}

	return ib
}

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

	From, To *Image
}

func (b *Builder) WriteHeader() error {
	return nil
}

func (b *Builder) WriteBody() error {
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

	return img, nil
}

func NewBuilder(server *Server, from, to string) (b *Builder, err error) {
	b = &Builder{
		server: server,
	}
	if from != "" {
		if b.From, err = b.getImage(from); err != nil {
			return nil, errors.Wrapf(err, "failed to obtain from image")
		}
	}

	if b.To, err = b.getImage(to); err != nil {
		return nil, errors.Wrapf(err, "failed to obtain to image")
	}

	allLayers := make([]*ImageLayer, 0)
	if b.From != nil {
		allLayers = append(allLayers, b.From.Layers...)
	}
	allLayers = append(allLayers, b.To.Layers...)

	var errGrp errgroup.Group
	for _, layer := range allLayers {
		layer := layer
		errGrp.Go(func() error {
			return b.fetchLayers(layer)
		})
	}

	if err := errGrp.Wait(); err != nil {
		return nil, errors.Wrapf(err, "failed to load all compressed layer")
	}

	return b, nil
}
