package proxy

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	goreg "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/mc256/starlight/util"
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
	return ioutil.NopCloser(l.R), nil
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
func NewStarlightLayer(f *os.File, stargzWriter *util.Writer) (goreg.Layer, error) {
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
}

func NewConvertor(ctx context.Context, src, dst string, optsSrc, dstSrc []name.Option, optsRemote []remote.Option) (c *Convertor, err error) {
	c = &Convertor{
		ctx:        ctx,
		optsRemote: optsRemote,
	}
	if c.src, err = name.ParseReference(src, optsSrc...); err != nil {
		return nil, err
	}
	if c.dst, err = name.ParseReference(dst, dstSrc...); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Convertor) GetSrc() string {
	return c.src.String()
}

func (c *Convertor) GetDst() string {
	return c.dst.String()
}

func (c *Convertor) readImage() (goreg.Image, error) {
	log.G(c.ctx).WithFields(logrus.Fields{"image": c.src}).Debug("fetching container image")
	desc, err := remote.Get(c.src, c.optsRemote...)
	if err != nil {
		return nil, err
	}
	return desc.Image()
}

func (c *Convertor) writeImage(image goreg.Image) error {
	log.G(c.ctx).WithFields(logrus.Fields{"image": c.src}).Debug("uploading converted container image")
	return remote.Write(c.dst, image, c.optsRemote...)
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
	w := util.NewWriterLevel(f, gzip.BestCompression)
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
			util.StarlightTOCDigestAnnotation:       tocDigest.String(),
			util.StarlightTOCCreationTimeAnnotation: time.Now().Format(time.RFC3339Nano),
			util.TOCJSONDigestAnnotation:            tocDigest.String(),
		},
	}

	return nil
}

func (c *Convertor) ToStarlightImage() (err error) {
	// image
	var img goreg.Image
	if img, err = c.readImage(); err != nil {
		return err
	}

	// config
	cfg, err := img.ConfigFile()
	history := cfg.History
	if err != nil {
		return err
	}
	addendum := make([]mutate.Addendum, len(history))

	// layer
	var layers []goreg.Layer
	if layers, err = img.Layers(); err != nil {
		return err
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
		return errors.Wrapf(err, "failed to convert the image to Starlight format")
	}

	if err != nil {
		return err
	}

	// Clean up things that will be changed
	cfg.RootFS.DiffIDs = []goreg.Hash{}
	cfg.History = []goreg.History{}

	// Set the configuration file to
	configuredImage, err := mutate.ConfigFile(empty.Image, cfg)
	if err != nil {
		return err
	}

	// Write container image to the registry (or other places)
	slImg, err := mutate.Append(configuredImage, addendum...)
	if err != nil {
		return err
	}

	return c.writeImage(slImg)
}
