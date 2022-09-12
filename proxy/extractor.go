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
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
)

// Extractor extract ToC from starlight-formatted container image and save it to the databse
type Extractor struct {
	Serial     int    `json:"serial"`
	Digest     string `json:"digest"`
	Image      string `json:"requested-image"`
	NLayers    int    `json:"nlayers"`
	ParsedName string `json:"parsed-image-name"`
	ParsedTag  string `json:"parsed-tag"`

	server *StarlightProxyServer
	ref    name.Reference
}

// SaveImage stores container image to database
func (ex *Extractor) saveImage(img *v1.Image) (
	serial int, existing bool, err error,
) {

	d, err := (*img).Digest()
	if err != nil {
		return
	}
	ex.Digest = d.String()

	config, err := (*img).ConfigFile()
	if err != nil {
		return
	}

	manifest, err := (*img).Manifest()
	if err != nil {
		return
	}

	serial, existing, err = ex.server.db.InsertImage(ex.ParsedName, ex.Digest, config, manifest, ex.NLayers)
	if err != nil {
		return
	}

	return serial, existing, nil
}

func (ex *Extractor) setImageTag() error {
	return ex.server.db.SetImageTag(ex.ParsedName, ex.ParsedTag, ex.Serial)
}

func (ex *Extractor) enableImage() error {
	return ex.server.db.SetImageReady(true, ex.Serial)
}

func (ex *Extractor) saveLayer(imageSerial, idx int, layer v1.Layer) error {
	txn, err := ex.server.db.db.Begin()
	if err != nil {
		return err
	}
	defer txn.Commit()

	size, err := layer.Size()
	if err != nil {
		return err
	}
	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	layerRef, existing, err := ex.server.db.InsertLayer(txn, size, imageSerial, idx, digest.String())
	if err != nil {
		return err
	}

	if !existing {
		var src io.ReadCloser
		src, err = layer.Compressed()
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer([]byte{})
		_, err = io.Copy(buf, src)
		if err != nil {
			return err
		}

		reader := bytes.NewReader(buf.Bytes())
		sr := io.NewSectionReader(reader, 0, reader.Size())
		layerFile, err := util.OpenStargz(sr)
		if err != nil {
			return err
		}

		// Get TOC
		entryMap, chunks, _ := layerFile.GetTOC()

		// entries map
		entBuffer := make(map[string]*util.TraceableEntry)
		for k, v := range entryMap {
			entBuffer[k] = &util.TraceableEntry{
				TOCEntry: v,
				Landmark: v.Landmark(),
				Chunks:   make([]*util.TOCEntry, 0),
			}
		}

		// chunks
		for k, v := range chunks {
			extEntry := entBuffer[k]
			for _, c := range v {
				extEntry.Chunks = append(extEntry.Chunks, c)
			}
		}

		err = ex.server.db.InsertFiles(txn, layerRef, entBuffer)
		if err != nil {
			return err
		}
	}

	return nil

}

// SaveToC save ToC to the backend database and return ApiResponse if success.
// It does require the container registry is functioning correctly.
func (ex *Extractor) SaveToC() (res *ApiResponse, err error) {

	// Manifest and Config
	desc, err := remote.Get(ex.ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	// Container image
	img, err := desc.Image()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	// Layers
	layers, err := img.Layers()
	ex.NLayers = len(layers)

	// Insert into the "image" table
	existing := false
	ex.Serial, existing, err = ex.saveImage(&img)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	// Insert into the "layer" - "filesystem" - "file" tables
	if !existing {
		if err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}

		var errGrp errgroup.Group
		for idx, layer := range layers {
			idx, layer := idx, layer

			errGrp.Go(func() error {
				return ex.saveLayer(ex.Serial, idx, layer)
			})
		}

		if err = errGrp.Wait(); err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}

		if err = ex.enableImage(); err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}
	} else if ex.Serial == 0 && err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	// Insert into the "tag" table (tag.imageId - image.id)
	if err = ex.setImageTag(); err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	log.G(ex.server.ctx).WithFields(logrus.Fields{
		"image":  ex.ParsedName,
		"tag":    ex.ParsedTag,
		"hash":   ex.Digest,
		"serial": ex.Serial,
	}).Info("saved ToC")

	return &ApiResponse{
		Status:    "OK",
		Code:      http.StatusOK,
		Message:   "cached ToC",
		Extractor: ex,
	}, nil
}

func NewExtractor(s *StarlightProxyServer, image string) (r *Extractor, err error) {
	r = &Extractor{
		Serial:  0,
		Digest:  "",
		Image:   image,
		ref:     nil,
		NLayers: 0,
		server:  s,
	}

	if image == "" {
		return nil, fmt.Errorf("image cannot be nil")
	}

	r.ref, err = name.ParseReference(image,
		name.WithDefaultRegistry(s.config.DefaultRegistry), name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	r.ParsedName, r.ParsedTag = ParseImageReference(r.ref, r.server.config.DefaultRegistry)
	return
}
