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
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"
)

type layerReader struct {
	*io.SectionReader
	//ready chan bool
}

type DeltaBundleBuilder struct {
	ctx      context.Context
	registry string

	// layerReaders stores the layer needed to build the delta bundle
	// key is only the digest (no image name)
	layerReaders     map[string]*layerReader
	layerReadersLock sync.Mutex

	client http.Client
}

func (ib *DeltaBundleBuilder) WriteBody(w io.Writer, c *util.ProtocolTemplate, wg *sync.WaitGroup) (err error) {
	wg.Wait()

	readers := make([]*io.SectionReader, len(c.DigestList)+1)
	for i, d := range c.DigestList {
		if c.RequiredLayer[i+1] {
			readers[i+1] = ib.layerReaders[d.Digest.String()].SectionReader
		}
	}
	for _, ent := range c.OutputQueue {
		sr := io.NewSectionReader(readers[ent.Source], ent.SourceOffset, ent.CompressedSize)

		/*
			log.G(ib.ctx).WithFields(logrus.Fields{
				"offset": ent.SourceOffset,
				"length": ent.CompressedSize,
				"source": ent.Source,
			}).Trace("request range")
		*/

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
	cw := util.GetCountWriter(w)
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
		ib.layerReaders[digest] = &layerReader{
			SectionReader: nil,
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

			ib.layerReaders[digest].SectionReader = io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, length)
			wg.Done()
		}()

	}()
}

func NewBuilder(ctx context.Context, registry string) *DeltaBundleBuilder {
	ib := &DeltaBundleBuilder{
		ctx:              ctx,
		registry:         registry,
		layerReaders:     make(map[string]*layerReader, 0),
		layerReadersLock: sync.Mutex{},
		client:           http.Client{},
	}

	return ib
}
