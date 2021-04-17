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
	"context"
	"encoding/json"
	"github.com/containerd/containerd/log"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	manifest2 "github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	regClient "github.com/docker/distribution/registry/client"
	"github.com/mc256/stargz-snapshotter/estargz"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"io"
	"net/http"
)

const (
	ImageArchitecture    = "amd64"
	ImageOS              = "linux"
	MediaTypeImage       = "application/vnd.oci.image.index.v1+json"
	MediaTypeDockerImage = "application/vnd.docker.distribution.manifest.v2+json"
)

// app->image->tag->manifest->layers->toc->tocEntry
// app(w platform) -> tag -> layers -> toc -> tocEntry

func getToc(ctx context.Context, repo distribution.Repository, imageName, imageTag string, tagBucket, blobBucket *bolt.Bucket, mani distribution.Manifest, maniDigest digest.Digest) error {
	mv2 := mani.(*manifest2.DeserializedManifest).Manifest

	// Manifset
	if err := tagBucket.Put([]byte("manifest"), []byte(maniDigest.String())); err != nil {
		return err
	}

	// Count
	if err := tagBucket.Put([]byte("count"), util.Int32ToB(uint32(len(mv2.Layers)))); err != nil {
		return err
	}

	//  Config
	if buf, err := repo.Blobs(ctx).Get(ctx, mv2.Config.Digest); err != nil {
		return err
	} else if err = tagBucket.Put([]byte("config"), buf); err != nil {
		return err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"name":      imageName,
		"tag":       imageTag,
		"mediaType": mv2.MediaType,
	}).Info("found image manifest")

	// Layers
	for i, layer := range mv2.Layers {
		// Get Layer
		log.G(ctx).WithFields(logrus.Fields{
			"type":   layer.MediaType,
			"digest": layer.Digest,
			"size":   layer.Size,
			"order":  i,
		}).Debug("found layer")
		err := tagBucket.Put(util.Int32ToB(uint32(i)), []byte(layer.Digest.String()))
		if err != nil {
			return err
		}

		// BLOBs
		buf, err := repo.Blobs(ctx).Get(ctx, layer.Digest)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(buf)
		sr := io.NewSectionReader(reader, 0, reader.Size())
		layerFile, err := estargz.Open(sr)
		if err != nil {
			return err
		}

		// Get TOC
		tocMap, chunks, _ := layerFile.GetTOC()
		layerId := util.TraceableBlobDigest{
			Digest:    layer.Digest,
			ImageName: imageName,
		}

		log.G(ctx).WithFields(logrus.Fields{
			"layerId": layerId,
		}).Debug("save layer")
		layerBucket, err := blobBucket.CreateBucketIfNotExists([]byte(layerId.String()))
		if err != nil {
			return err
		}

		// Save TOC
		err = SaveLayer(layerBucket, tocMap, chunks)
		if err != nil {
			return err
		}
	}
	return nil
}

func CacheToc(ctx context.Context, db *bolt.DB, imageName, imageTag, registry string) error {
	// database
	tx, err := db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	imageStore, err := tx.CreateBucketIfNotExists([]byte("image"))
	if err != nil {
		return err
	}
	blobBucket, err := tx.CreateBucketIfNotExists([]byte("blob"))
	if err != nil {
		return err
	}

	// parse image name and tag
	named, err := reference.Parse(imageName)
	if err != nil {
		return err
	}

	// image repository
	repo, err := regClient.NewRepository(named.(reference.Named), registry, http.DefaultTransport)
	if err != nil {
		return err
	}
	imageBucket, err := imageStore.CreateBucketIfNotExists([]byte(imageName))
	if err != nil {
		return err
	}

	// get image tag
	ti, err := repo.Tags(ctx).Get(ctx, imageTag)
	if err != nil {
		return err
	}

	tagBucket, err := imageBucket.CreateBucketIfNotExists([]byte(imageTag))
	if err != nil {
		return err
	}

	// check image type
	if ti.MediaType == MediaTypeImage {
		// list of manifests

		manSvc, err := repo.Manifests(ctx)
		if err != nil {
			return err
		}

		// manifest list
		maniListRaw, err := manSvc.Get(ctx, "", distribution.WithTag(imageTag))
		if err != nil {
			return err
		}
		maniList := maniListRaw.(*manifestlist.DeserializedManifestList).ManifestList

		log.G(ctx).WithFields(logrus.Fields{
			"name": imageName,
			"tag":  imageTag,
		}).Info("found image manifest list")

		// check manifest list
		matched := false
		for _, m := range maniList.Manifests {
			if ImageArchitecture != m.Platform.Architecture ||
				ImageOS != m.Platform.OS {
				continue
			}
			matched = true

			mani, err := manSvc.Get(ctx, m.Digest, []distribution.ManifestServiceOption{}...)
			if err != nil {
				log.G(ctx).Error(err)
				continue
			}

			if err = getToc(ctx, repo, imageName, imageTag, tagBucket, blobBucket, mani, m.Digest); err != nil {
				return err
			}
			break

		}
		if !matched {
			return util.ErrImagePlatform
		}

	} else if ti.MediaType == MediaTypeDockerImage {
		// single manifest
		// manifest service
		manSvc, err := repo.Manifests(ctx)
		if err != nil {
			return err
		}

		// manifest list
		mani, err := manSvc.Get(ctx, ti.Digest, distribution.WithTag(imageTag))
		if err != nil {
			return err
		}

		if err = getToc(ctx, repo, imageName, imageTag, tagBucket, blobBucket, mani, ti.Digest); err != nil {
			return err
		}

	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"repo": imageName,
			"tag":  imageTag,
			"type": ti.MediaType,
		}).Warn("unknown image type")
		return util.ErrImageMediaType
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil

}

// InitField save the TOC to the bucket
func SaveLayer(bucket *bolt.Bucket, entryMap map[string]*estargz.TOCEntry, chunks map[string][]*estargz.TOCEntry) error {

	// entries map
	entBuffer := make(map[string]*util.TraceableEntry)
	for k, v := range entryMap {
		entBuffer[k] = &util.TraceableEntry{
			TOCEntry: v,
			Landmark: v.Landmark(),
			Chunks:   make([]*estargz.TOCEntry, 0),
		}
	}

	// chunks
	for k, v := range chunks {
		extEntry := entBuffer[k]
		//fmt.Println(k)
		for _, c := range v {
			extEntry.Chunks = append(extEntry.Chunks, c)
			//fmt.Printf("%d %d %d\n", c.ChunkOffset, c.ChunkSize, c.CompressedSize)
		}
	}

	for k, v := range entBuffer {
		b, _ := json.Marshal(v)
		_ = bucket.Put([]byte(k), b)
	}

	return nil
}
