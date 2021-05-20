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
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"
)

type ImageBuilder struct {
	ctx context.Context

	c *merger.Consolidator

	layerReaders     []*io.SectionReader
	layerReadersLock sync.Mutex

	client http.Client
}

func (ib *ImageBuilder) WriteBody(w io.Writer) (err error) {
	q := ib.c.GetOutputQueue()
	for _, ent := range *q {
		sr := io.NewSectionReader(ib.layerReaders[ent.Source-1], ent.SourceOffset, ent.CompressedSize)
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

func (ib *ImageBuilder) WriteHeader(w io.Writer) (headerSize int64, contentLength int64, err error) {
	cw := util.GetCountWriter(w)
	gw, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return 0, 0, err
	}
	err = ib.c.ExportTOC(gw, false)
	if err != nil {
		return 0, 0, err
	}
	err = gw.Close()
	if err != nil {
		return 0, 0, err
	}
	headerSize = cw.GetWrittenSize()
	contentLength = headerSize + ib.c.Offsets[len(ib.c.Offsets)-1]
	log.G(ib.ctx).WithFields(logrus.Fields{
		"headerSize":    headerSize,
		"contentLength": contentLength,
	}).Info("wrote image header")
	return headerSize, contentLength, nil
}

func fetchLayer(wg *sync.WaitGroup, ib *ImageBuilder, registry, imageName, digest string, idx int) {
	defer wg.Done()

	url := registry + path.Join("/v2", imageName, "blobs", digest)
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

	//buf := bytes.NewBuffer(make([]byte, 0, length))
	buf := new(bytes.Buffer)
	if _, err = io.CopyN(buf, resp.Body, length); err != nil {
		log.G(ib.ctx).WithError(err).Error("blob read")
		return
	}

	ib.layerReadersLock.Lock()
	defer ib.layerReadersLock.Unlock()

	ib.layerReaders[idx] = io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, length)

}

func NewPreloadImageBuilder(ctx context.Context, c *merger.Consolidator, registry string) (*ImageBuilder, error) {
	err := c.PopulateOffset()
	if err != nil {
		return nil, err
	}

	ib := &ImageBuilder{
		ctx:              ctx,
		c:                c,
		layerReaders:     make([]*io.SectionReader, 0, len(c.Source)),
		layerReadersLock: sync.Mutex{},
		client:           http.Client{},
	}

	var wg sync.WaitGroup

	ib.layerReaders = make([]*io.SectionReader, len(c.Source))
	for idx, td := range c.Source {
		wg.Add(1)
		go fetchLayer(&wg, ib, registry, td.ImageName, td.Digest.String(), idx)
	}

	wg.Wait()

	return ib, nil
}
