package util

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/mc256/starlight/util/common"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/google/go-containerregistry/pkg/name"
	goreg "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

type StarlightLayer struct {
	R io.Reader

	// Diff is the hash of the uncompressed layer
	Diff goreg.Hash
	// Hash is the hash of the compressed layer
	Hash goreg.Hash

	SizeVal int64
}

func (l StarlightLayer) Digest() (goreg.Hash, error) {
	return l.Hash, nil
}

func (l StarlightLayer) Size() (int64, error) {
	return l.SizeVal, nil
}

func (l StarlightLayer) DiffID() (goreg.Hash, error) {
	return l.Diff, nil
}

func (l StarlightLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func (l StarlightLayer) Compressed() (io.ReadCloser, error) {
	// see issue https://github.com/google/go-containerregistry/pull/768
	return io.NopCloser(l.R), nil
}

func (l StarlightLayer) Uncompressed() (io.ReadCloser, error) {
	// There is no need to implement this method
	return nil, errors.New("unsupported")
}

func fileSectionReader(file *os.File) (*io.SectionReader, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return io.NewSectionReader(file, 0, info.Size()), nil
}

// NewStarlightLayer mainly populate the hash values for the StarlightLayer object
func NewStarlightLayer(f *os.File, stargzWriter *common.Writer) (goreg.Layer, error) {
	d, err := goreg.NewHash(stargzWriter.DiffID())
	if err != nil {
		return nil, err
	}

	h, err := goreg.NewHash(stargzWriter.Digest())
	if err != nil {
		return nil, err
	}

	sr, err := fileSectionReader(f)
	if err != nil {
		return nil, err
	}

	return StarlightLayer{
		R:       sr,
		Diff:    d,
		Hash:    h,
		SizeVal: sr.Size(),
	}, nil
}

type Convertor struct {
	// There might be multiple `images` associate with an `index`,
	// but we only implement image conversion here.
	src, dst   name.Reference
	ctx        context.Context
	optsRemote []remote.Option
	platforms  string
}

func NewConvertor(ctx context.Context, src, dst string, optsSrc, dstSrc []name.Option, optsRemote []remote.Option, platforms string) (c *Convertor, err error) {
	c = &Convertor{
		ctx:        ctx,
		optsRemote: optsRemote,
		platforms:  platforms,
	}
	if c.src, err = name.ParseReference(src, optsSrc...); err != nil {
		return nil, err
	}
	if c.dst, err = name.ParseReference(dst, dstSrc...); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Convertor) String() string {
	return fmt.Sprintf("Convertor{src=%s, dst=%s}", c.src, c.dst)
}

func (c *Convertor) GetSrc() name.Reference {
	return c.src
}

func (c *Convertor) GetDst() name.Reference {
	return c.dst
}

func (c *Convertor) readImageDescriptor() (*remote.Descriptor, error) {
	log.G(c.ctx).WithFields(logrus.Fields{"image": c.src}).Info("fetching container image")
	desc, err := remote.Get(c.src, c.optsRemote...)
	if err != nil {
		return nil, err
	}
	return desc, nil
}

func (c *Convertor) writeImage(image goreg.Image) error {
	log.G(c.ctx).WithFields(logrus.Fields{"image": c.dst}).Info("uploading converted container image")
	return remote.Write(c.dst, image, c.optsRemote...)
}

func (c *Convertor) writeImageIndex(imageIndex goreg.ImageIndex) error {
	log.G(c.ctx).WithFields(logrus.Fields{"imageIndex": c.dst}).Info("uploading converted container image")
	return remote.WriteIndex(c.dst, imageIndex, c.optsRemote...)
}

func (c *Convertor) toStarlightLayer(idx, layerIdx int, layers []goreg.Layer,
	addendums []mutate.Addendum, mtx *sync.Mutex, history goreg.History) error {

	if history.EmptyLayer {
		mtx.Lock()
		defer mtx.Unlock()
		addendums[idx] = mutate.Addendum{
			Layer:       nil,
			History:     history,
			Annotations: nil,
		}
		return nil
	}
	layer := layers[layerIdx]

	// Temporary File for the Starlight Layer
	d, err := layer.Digest()
	if err != nil {
		return err
	}
	fn := path.Join(os.TempDir(), fmt.Sprintf("starlight-convert-%d-%s.sll", idx, d))
	f, err := os.Create(fn)
	if err != nil {
		return err
	}

	l, err := layer.Uncompressed()
	log.G(c.ctx).WithFields(logrus.Fields{"layer": d}).Debug("decompressed layer")

	if err != nil {
		return err
	}
	// modified version of stargz writer
	w := common.NewWriterLevel(f, gzip.BestCompression)
	// TODO: we could change the chunk size here but let's keep it as 4KB
	if err := w.AppendTar(l); err != nil {
		return err
	}
	tocDigest, err := w.Close()
	if err != nil {
		return err
	}

	// create starlight layer
	sll, err := NewStarlightLayer(f, w)
	if err != nil {
		return err
	}
	log.G(c.ctx).WithFields(logrus.Fields{"layer": d}).Debug("converted layer")

	// add layer to the image
	mtx.Lock()
	defer mtx.Unlock()
	addendums[idx] = mutate.Addendum{
		Layer:   sll,
		History: history,
		Annotations: map[string]string{
			StarlightTOCDigestAnnotation:       tocDigest.String(),
			StarlightTOCCreationTimeAnnotation: time.Now().Format(time.RFC3339Nano),
			common.TOCJSONDigestAnnotation:     tocDigest.String(),
		},
	}

	return nil
}

func (c *Convertor) convertSingleImage(img goreg.Image) (goreg.Image, error) {
	// config
	cfg, err := img.ConfigFile()
	history := cfg.History
	if err != nil {
		return nil, err
	}
	addendum := make([]mutate.Addendum, len(history))

	// layer
	var layers []goreg.Layer
	if layers, err = img.Layers(); err != nil {
		return nil, err
	}
	layerMap := make([]int, len(history))
	count := 0
	for i, h := range history {
		if h.EmptyLayer {
			layerMap[i] = -1
		} else {
			layerMap[i] = count
			count += 1
		}
	}

	var addendumMux sync.Mutex
	var errGrp errgroup.Group

	for i, h := range history {
		i, h := i, h
		errGrp.Go(func() error {
			return c.toStarlightLayer(i, layerMap[i], layers, addendum, &addendumMux, h)
		})
	}

	if err := errGrp.Wait(); err != nil {
		return nil, errors.Wrapf(err, "failed to convert the image to Starlight format")
	}

	if err != nil {
		return nil, err
	}

	// Clean up things that will be changed
	cfg.RootFS.DiffIDs = []goreg.Hash{}
	cfg.History = []goreg.History{}

	// Set the configuration file to
	configuredImage, err := mutate.ConfigFile(empty.Image, cfg)
	if err != nil {
		return nil, err
	}

	// Write container image to the registry (or other places)
	if slImg, err := mutate.Append(configuredImage, addendum...); err != nil {
		return nil, err
	} else {
		return slImg, nil
	}
}

func (c *Convertor) ToStarlightImage() (err error) {
	// platform filter
	var (
		allPlatforms       = false
		requestedPlatforms []*goreg.Platform
	)
	if c.platforms != "" && c.platforms != "all" {
		ps := strings.Split(c.platforms, ",")
		for _, p := range ps {
			var plt v1.Platform
			plt, err = platforms.Parse(p)
			if err != nil {
				return errors.Wrapf(err, "failed to parse platform")
			}
			log.G(c.ctx).WithFields(logrus.Fields{"platform": p}).Info("requested platform")
			requestedPlatforms = append(requestedPlatforms, &goreg.Platform{
				Architecture: plt.Architecture,
				OS:           plt.OS,
				OSVersion:    plt.OSVersion,
				OSFeatures:   plt.OSFeatures,
				Variant:      plt.Variant,
				Features:     nil,
			})
		}
	} else {
		log.G(c.ctx).WithFields(logrus.Fields{"platform": "all"}).Info("requested platform")
		allPlatforms = true
	}
	hasPlatform := func(p *goreg.Platform) bool {
		if allPlatforms {
			return true
		}
		for _, plt := range requestedPlatforms {
			if plt.Equals(*p) {
				return true
			}
		}
		return false
	}

	// image descriptor
	var (
		imgDesc *remote.Descriptor
	)
	if imgDesc, err = c.readImageDescriptor(); err != nil {
		return errors.Wrapf(err, "failed to read image")
	}

	if imgDesc.MediaType == types.DockerManifestSchema2 {
		// single manifest image
		// "application/vnd.docker.distribution.manifest.v2+json"
		var (
			img goreg.Image
		)

		if img, err = imgDesc.Image(); err != nil {
			return errors.Wrapf(err, "failed to read image")
		}

		log.G(c.ctx).WithFields(logrus.Fields{}).Info("found single image")

		// convert image
		var slImg goreg.Image
		if slImg, err = c.convertSingleImage(img); err != nil {
			return errors.Wrapf(err, "failed to convert image format")
		}

		_, err = slImg.Digest()
		if err != nil {
			return err
		}

		// Write the image to the registry
		if err := c.writeImage(slImg); err != nil {
			return errors.Wrapf(err, "failed to upload image")
		}

		return nil

	} else {
		// image index
		// "application/vnd.docker.distribution.manifest.list.v2+json"
		var (
			imgIdx, retIdx goreg.ImageIndex
			idxMan         *goreg.IndexManifest
		)

		if imgIdx, err = imgDesc.ImageIndex(); err != nil {
			return errors.Wrapf(err, "failed to read image index")
		}

		if idxMan, err = imgIdx.IndexManifest(); err != nil {
			return errors.Wrapf(err, "failed to read index manifest")
		}

		retIdx = mutate.AppendManifests(empty.Index)
		var idxAddendumMux sync.Mutex
		var idxErrGrp errgroup.Group

		for _, m := range idxMan.Manifests {
			m := m
			idxErrGrp.Go(func() error {
				req := hasPlatform(m.Platform)
				log.G(c.ctx).WithFields(logrus.Fields{
					"platform": m.Platform,
					"digest":   m.Digest.String(),
					"size":     m.Size,
					"skip":     !req,
				}).Info("found platform")

				if !req {
					return nil
				}

				img, err := imgIdx.Image(m.Digest)
				if err != nil {
					return err
				}

				var (
					slImg goreg.Image
					h     goreg.Hash
				)

				if slImg, err = c.convertSingleImage(img); err != nil {
					return errors.Wrapf(err, "failed to convert image format")
				}

				h, err = slImg.Digest()
				if err != nil {
					return err
				}

				idxAddendumMux.Lock()
				defer idxAddendumMux.Unlock()

				retIdx = mutate.AppendManifests(retIdx, mutate.IndexAddendum{
					Add: slImg,
					Descriptor: goreg.Descriptor{
						MediaType:   m.MediaType,
						Size:        m.Size,
						Digest:      h,
						URLs:        m.URLs,
						Annotations: m.Annotations,
						Platform:    m.Platform,
					},
				})
				return nil
			})
		}

		if err := idxErrGrp.Wait(); err != nil {
			return errors.Wrapf(err, "failed to convert the OCI image to Starlight format")
		}

		return c.writeImageIndex(retIdx)
	}
}
