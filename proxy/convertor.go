package proxy

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	goreg "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
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

// NewStarlightLayer mainly populate the hash values for the StarlightLayer object
func NewStarlightLayer(compressedLayer io.Reader, temp os.File) (goreg.Layer, error) {
	var (
		diff = sha256.New()
		h    = sha256.New()
		cw   = util.NewCountWriter(ioutil.Discard)
	)

	zr, err := gzip.NewReader(io.TeeReader(compressedLayer, io.MultiWriter(cw, h)))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	return StarlightLayer{}
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
	return nil
}

func nameIfChanged(mp *map[int]string, id int, name string) string {
	if name == "" {
		return ""
	}
	if *mp == nil {
		*mp = make(map[int]string)
	}
	if (*mp)[id] == name {
		return ""
	}
	(*mp)[id] = name
	return name
}

func formatModtime(t time.Time) string {
	if t.IsZero() || t.Unix() == 0 {
		return ""
	}
	return t.UTC().Round(time.Second).Format(time.RFC3339)
}

func (c *Convertor) toStarlightLayer(idx int, layer goreg.Layer, addendums *[]mutate.Addendum, addendumMu *sync.Mutex) error {
	// Temporary File for the Starlight Layer
	d, err := layer.Digest()
	if err != nil {
		return err
	}
	f, err := os.Create(path.Join(os.TempDir(), fmt.Sprintf("starlight-convert-%d-%s.sll", idx, d)))
	if err != nil {
		return err
	}
	cw := util.NewCountWriter(f)
	gw, err := gzip.NewWriterLevel(cw, gzip.BestCompression)
	if err != nil {
		return err
	}

	// User and Groups
	uidmap := map[int]string{}

	// Convert typical tar layer to starlight format
	r, err := layer.Uncompressed()
	if err != nil {
		return err
	}

	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error parsing tar format layer: %s - %v", h.Name, err)
		}

		xattrs := make(map[string][]byte)
		const xattrPAXRecordsPrefix = "SCHILY.xattr."
		if h.PAXRecords != nil {
			for k, v := range h.PAXRecords {
				if strings.HasPrefix(k, xattrPAXRecordsPrefix) {
					xattrs[k[len(xattrPAXRecordsPrefix):]] = []byte(v)
				}
			}
		}
		ent := &util.TOCEntry{
			Name:        h.Name,
			Mode:        h.Mode,
			UID:         h.Uid,
			GID:         h.Gid,
			Uname:       nameIfChanged(&uidmap, h.Uid, h.Uname),
			Gname:       nameIfChanged(&uidmap, h.Gid, h.Gname),
			ModTime3339: formatModtime(h.ModTime),
			Xattrs:      xattrs,
		}
		gw.condOpenGz()
		tw := tar.NewWriter(currentGzipWriter{w})
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeLink:
			ent.Type = "hardlink"
			ent.LinkName = h.Linkname
		case tar.TypeSymlink:
			ent.Type = "symlink"
			ent.LinkName = h.Linkname
		case tar.TypeDir:
			ent.Type = "dir"
		case tar.TypeReg:
			ent.Type = "reg"
			ent.Size = h.Size
		case tar.TypeChar:
			ent.Type = "char"
			ent.DevMajor = int(h.Devmajor)
			ent.DevMinor = int(h.Devminor)
		case tar.TypeBlock:
			ent.Type = "block"
			ent.DevMajor = int(h.Devmajor)
			ent.DevMinor = int(h.Devminor)
		case tar.TypeFifo:
			ent.Type = "fifo"
		default:
			return fmt.Errorf("unsupported input tar entry %q", h.Typeflag)
		}

		// We need to keep a reference to the TOC entry for regular files, so that we
		// can fill the digest later.
		var regFileEntry *util.TOCEntry
		var payloadDigest digest.Digester
		if h.Typeflag == tar.TypeReg {
			regFileEntry = ent
			payloadDigest = digest.Canonical.Digester()
		}

		if h.Typeflag == tar.TypeReg && ent.Size > 0 {
			var written int64
			totalSize := ent.Size // save it before we destroy ent
			tee := io.TeeReader(tr, payloadDigest.Hash())
			didWrite := false
			var prevEnt *util.TOCEntry
			prevEnt = nil
			for written < totalSize {
				if err := w.closeGz(); err != nil {
					return err
				}
				if prevEnt != nil {
					prevEnt.CompressedSize = w.cw.n - prevEnt.Offset
				}
				chunkSize := int64(w.chunkSize())
				remain := totalSize - written
				if remain < chunkSize {
					chunkSize = remain
				} else {
					ent.ChunkSize = chunkSize
				}
				ent.Offset = w.cw.n
				ent.ChunkOffset = written
				chunkDigest := digest.Canonical.Digester()

				w.condOpenGz()
				didWrite = true
				teeChunk := io.TeeReader(tee, chunkDigest.Hash())
				if _, err := io.CopyN(tw, teeChunk, chunkSize); err != nil {
					return fmt.Errorf("error copying %q: %v", h.Name, err)
				}
				ent.ChunkDigest = chunkDigest.Digest().String()
				prevEnt = ent
				w.toc.Entries = append(w.toc.Entries, ent)
				written += chunkSize
				ent = &util.TOCEntry{
					Name: h.Name,
					Type: "chunk",
				}
			}
			if didWrite {
				if err := w.closeGz(); err != nil {
					return err
				}
				if prevEnt != nil {
					prevEnt.CompressedSize = w.cw.n - prevEnt.Offset
				}
				w.condOpenGz()
			}
		} else {
			w.toc.Entries = append(w.toc.Entries, ent)
		}

		if regFileEntry != nil && payloadDigest != nil {
			regFileEntry.Digest = payloadDigest.Digest().String()
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// add layer to the addendum
	addendumMu.Lock()
	defer addendumMu.Unlock()
	(*addendums)[idx] = mutate.Addendum{
		Layer: sll,
		Annotations: map[string]string{
			util.StarlightTOCDigestAnnotation: sld,
			// should we consider compatable with estargz
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

	/*
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
	*/
	return nil
}
