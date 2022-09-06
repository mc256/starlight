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
	"path"
)

type ExtractorResult struct {
	Serial int `json:"serial"`
}

// SaveImage stores container image to database
func SaveImage(s *StarlightProxyServer, ref name.Reference, img *v1.Image, nlayers int) (
	serial int, imageName, tag, digest string, existing bool, err error) {
	n := ref.Context().RepositoryStr()
	if ref.Context().RegistryStr() != s.config.DefaultRegistry {
		n = path.Join(ref.Context().RegistryStr(), n)
	}
	tag = ref.Identifier()
	hash, err := (*img).Digest()
	if err != nil {
		return
	}
	config, err := (*img).ConfigFile()
	if err != nil {
		return
	}
	manifest, err := (*img).Manifest()
	if err != nil {
		return
	}

	serial, existing, err = s.db.InsertImage(n, hash.String(), config, manifest, nlayers)
	if err != nil {
		return
	}

	return serial, n, tag, hash.String(), existing, nil
}

func SetImageTag(s *StarlightProxyServer, imageName, imageTag string, imageSerial int) error {
	return s.db.SetImageTag(imageName, imageTag, imageSerial)
}

func EnableImage(s *StarlightProxyServer, imageSerial int) error {
	return s.db.SetImageReady(true, imageSerial)
}

func SaveLayer(s *StarlightProxyServer, imageSerial, idx int, layer v1.Layer) error {
	txn, err := s.db.db.Begin()
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

	layerRef, existing, err := s.db.InsertLayer(txn, size, imageSerial, idx, digest.String())
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

		err = s.db.InsertFiles(txn, layerRef, entBuffer)
		if err != nil {
			return err
		}
	}

	return nil

}

func SaveToC(s *StarlightProxyServer, image string) (*ApiResponse, error) {
	if image == "" {
		return nil, fmt.Errorf("image cannot be nil")
	}

	ref, err := name.ParseReference(image,
		name.WithDefaultRegistry(s.config.DefaultRegistry), name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	img, err := desc.Image()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	layers, err := img.Layers()

	serial, imageName, imageTag, imageHash, existing, err := SaveImage(s, ref, &img, len(layers))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	if !existing {
		if err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}

		var errGrp errgroup.Group
		for idx, layer := range layers {
			idx, layer := idx, layer

			errGrp.Go(func() error {
				return SaveLayer(s, serial, idx, layer)
			})
		}

		if err = errGrp.Wait(); err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}

		if err = EnableImage(s, serial); err != nil {
			return nil, errors.Wrapf(err, "failed to cache ToC")
		}
	} else if serial == 0 && err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	if err = SetImageTag(s, imageName, imageTag, serial); err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"image":  imageName,
		"tag":    imageTag,
		"hash":   imageHash,
		"serial": serial,
	}).Info("saved ToC")

	return &ApiResponse{
		Status:    "OK",
		Code:      http.StatusOK,
		Message:   "cached ToC",
		Extractor: &ExtractorResult{Serial: serial},
	}, nil
}
