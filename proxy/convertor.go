package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	goreg "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type StarlightLayer struct {
	R io.Reader

	// Diff is the hash of the uncompressed layer
	Diff goreg.Hash
	// Hash is hte hash of the compressed layer
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
	src, dst name.Reference
}

func GetConvertor(src, dst string, opts ...name.Option) (c *Convertor, err error) {
	c = &Convertor{}
	if c.src, err = name.ParseReference(src, opts...); err != nil {
		return nil, err
	}
	if c.dst, err = name.ParseReference(dst, opts...); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Convertor) readImage() (goreg.Image, error) {
	desc, err := remote.Get(c.src, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}
	return desc.Image()
}

func (c *Convertor) writeImage(image goreg.Image) error {
	return remote.Write(c.dst, image, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func (c *Convertor) toStarlightLayer(idx int, layer goreg.Layer, addendums *[]mutate.Addendum, mtx *sync.Mutex) error {
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
	if err != nil {
		return err
	}

	// modified version of stargz writer
	stargzWriter := util.NewWriterLevel(f, gzip.BestCompression)
	// TODO: we could change the chunk size here but let's keep it as 4KB
	if err := stargzWriter.AppendTar(l); err != nil {
		return err
	}
	tocDigest, err := stargzWriter.Close()
	if err != nil {
		return err
	}

	// create starlight layer
	sll, err := NewStarlightLayer(f, stargzWriter)
	if err != nil {
		return err
	}

	// add layer to the image
	mtx.Lock()
	defer mtx.Unlock()
	(*addendums)[idx] = mutate.Addendum{
		Layer: sll,
		Annotations: map[string]string{
			util.StarlightTOCDigestAnnotation: tocDigest.String(),
			// should we consider compatable with estargz
			util.TOCJSONDigestAnnotation: tocDigest.String(),
		},
	}

	return nil
}

func (c *Convertor) ToStarlightImage() (err error) {
	var img goreg.Image
	if img, err = c.readImage(); err != nil {
		return err
	}
	var layers []goreg.Layer
	if layers, err = img.Layers(); err != nil {
		return err
	}
	addendum := make([]mutate.Addendum, len(layers))
	var addendumMux sync.Mutex
	var egrp errgroup.Group
	for idx, layer := range layers {
		idx, layer := idx, layer
		egrp.Go(func() error {
			return c.toStarlightLayer(idx, layer, &addendum, &addendumMux)
		})
	}
	if err := egrp.Wait(); err != nil {
		return errors.Wrapf(err, "failed to convert the image to Starlight format")
	}

	// Copy image configuration file
	srcCfg, err := img.ConfigFile()
	if err != nil {
		return err
	}
	// Clean up things that will be changed
	srcCfg.RootFS.DiffIDs = []goreg.Hash{}
	srcCfg.History = []goreg.History{}
	// Set the configuration file to
	configuredImage, err := mutate.ConfigFile(empty.Image, srcCfg)
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
