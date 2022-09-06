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
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"path"
)

type ExtractorResult struct {
	Serial int `json:"serial"`
}

func SaveImage(s *StarlightProxyServer, ref name.Reference, img *v1.Image) (int, error) {
	n := ref.Context().RepositoryStr()
	if ref.Context().RegistryStr() != s.config.DefaultRegistry {
		n = path.Join(ref.Context().RegistryStr(), n)
	}
	tag := ref.Identifier()
	hash, err := (*img).Digest()
	if err != nil {
		return 0, err
	}
	config, err := (*img).ConfigFile()
	if err != nil {
		return 0, err
	}
	manifest, err := (*img).Manifest()
	if err != nil {
		return 0, err
	}

	return s.db.InsertImage(n, tag, hash.String(), config, manifest)
}

func SaveLayer(s *StarlightProxyServer, imageSerial, idx int, layer v1.Layer) error {
	size, err := layer.Size()
	if err != nil {
		return err
	}
	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	layerRef, err := s.db.InsertLayer(size, imageSerial, idx, digest.String())
	if err != nil {
		return err
	}

	src, err := layer.Compressed()
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

	return s.db.InsertFiles(layerRef, entBuffer)
}

func SaveToC(s *StarlightProxyServer, image string) (*ApiResponse, error) {
	if image == "" {
		return nil, fmt.Errorf("image cannot be nil")
	}

	ref, err := name.ParseReference(image,
		name.WithDefaultRegistry(s.config.DefaultRegistry), name.WithDefaultTag("latest-starlight"),
	)
	if err != nil {
		return nil, err
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}

	img, err := desc.Image()
	if err != nil {
		return nil, err
	}

	serial, err := SaveImage(s, ref, &img)
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	var errGrp errgroup.Group
	for idx, layer := range layers {
		idx, layer := idx, layer

		errGrp.Go(func() error {
			return SaveLayer(s, serial, idx, layer)
		})
	}

	if err := errGrp.Wait(); err != nil {
		return nil, errors.Wrapf(err, "failed to cache ToC")
	}

	return &ApiResponse{
		Status:    "OK",
		Code:      http.StatusOK,
		Message:   "cached ToC",
		Extractor: &ExtractorResult{Serial: serial},
	}, nil
}
