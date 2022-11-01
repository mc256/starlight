/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/containerd/containerd/images"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/util/receive"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Manager should be unmarshalled from a json file and then Populate() should be called to populate other fields
type Manager struct {
	receive.DeltaBundle

	// non-exported fields
	compressLayerDigest []digest.Digest
	diffDigest          []digest.Digest

	cfg *Configuration

	layers         map[int64]*receive.ImageLayer
	stackSerialMap []int64

	image       *images.Image
	imageConfig *v1.Image
	manifest    *v1.Manifest

	fileLookUpMap []map[string]fs.ReceivedFile

	tracer *fs.Tracer
	fs     map[int64]*fs.Instance
}

func (m *Manager) getDirectory(layerHash string) string {
	return filepath.Join(m.cfg.FileSystemRoot, "layers", layerHash[7:8], layerHash[8:10], layerHash[10:12], layerHash[12:])
}

func (m *Manager) getPathByStack(stack int64) string {
	if stack < 0 || stack >= int64(len(m.stackSerialMap)) {
		panic("stack lookup out of range")
	}
	return m.layers[m.stackSerialMap[stack]].Local
}

func (m *Manager) GetPathByLayer(stack int64) string {
	return m.getPathByStack(stack)
}

func (m *Manager) LookUpFile(stack int64, filename string) fs.ReceivedFile {
	if file, has := m.fileLookUpMap[stack][filename]; has {
		return file
	}
	return nil
}

func (m *Manager) LogTrace(stack int64, filename string, access, complete time.Time) {
	if m.tracer != nil {
		m.tracer.Log(filename, stack, access, complete)
	}
}

func (m *Manager) getPathBySerial(serial int64) string {
	if layer, has := m.layers[serial]; has {
		return layer.Local
	}
	panic("layer not found")
}

func (m *Manager) Extract(r *io.ReadCloser) error {
	if r == nil {
		return fmt.Errorf("cannot call extract if it has completed")
	}

	// create directories
	for _, layer := range m.Destination.Layers {
		err := os.MkdirAll(layer.Local, 0755)
		if err != nil {
			return err
		}
	}

	// Extract Contents
	for i, c := range m.Contents {
		p := m.getPathByStack(c.Stack)
		if err := os.MkdirAll(filepath.Join(p, c.GetBaseDir()), 0755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", filepath.Join(p, c.GetBaseDir()))
		}
		f, err := os.Create(filepath.Join(p, c.GetPath()))
		isClosed := false
		defer func() {
			if !isClosed {
				f.Close()
			}
		}()
		if err != nil {
			return errors.Wrapf(err, "failed to create file %s", filepath.Join(p, c.GetPath()))
		}
		for idx, ch := range c.Chunks {
			b := bytes.NewBuffer(make([]byte, 0, ch.CompressedSize))
			if n, err := io.CopyN(b, *r, ch.CompressedSize); err != nil || n != ch.CompressedSize {
				return errors.Wrapf(err, "failed to read content %d-%d at %d", i, idx, c.Offset)
			}
			gr, err := gzip.NewReader(b)
			if err != nil {
				return errors.Wrapf(err, "failed to create gzip reader for content %d-%d at %d", i, idx, c.Offset)
			}
			if _, err := io.CopyN(f, gr, ch.ChunkSize); err != nil {
				return errors.Wrapf(err, "failed to write content %d-%d at %d", i, idx, c.Offset)
			}
		}
		f.Close()
		isClosed = true
		close(c.Signal) // send out signal that this content is ready
	}

	return nil
}

// Init populates the manager with the necessary information and data structures.
// Use json.Unmarshal to unmarshal the json file from data storage into a Manager struct.
// - ready: if set to false, we will then use Extract() to get the content of the file
// - cfg: configuration of the client
// - image, manifest, imageConfig: information about the image (maybe we don't need this)
func (m *Manager) Init(cfg *Configuration, ready bool, image *images.Image, manifest *v1.Manifest, imageConfig *v1.Image) {
	// init variables
	m.cfg = cfg
	m.stackSerialMap = make([]int64, 0, len(m.Destination.Layers))
	m.layers = make(map[int64]*receive.ImageLayer)
	m.fs = make(map[int64]*fs.Instance)

	m.image = image
	m.manifest = manifest
	m.imageConfig = imageConfig

	// populate directory fields
	for _, layer := range m.Destination.Layers {
		layer.Local = m.getDirectory(layer.Hash)
		m.layers[layer.Serial] = layer
		m.stackSerialMap = append(m.stackSerialMap, layer.Serial)
	}
	if m.Source != nil {
		for _, layer := range m.Source.Layers {
			layer.Local = m.getDirectory(layer.Hash)
			m.layers[layer.Serial] = layer
		}
	}

	// create a list of signals
	if !ready {
		for _, content := range m.Contents {
			content.Signal = make(chan interface{})
		}
		// create filesystem template
		for _, f := range m.RequestedFiles {
			if _, isInPayload := f.InPayload(); isInPayload {
				f.Ready = &m.Contents[f.PayloadOrder].Signal
			} else {
				f.Ready = nil
			}
		}
	}

	// build filesystem tree
	m.fileLookUpMap = make([]map[string]fs.ReceivedFile, len(m.Destination.Layers))
	for i, _ := range m.fileLookUpMap {
		m.fileLookUpMap[i] = make(map[string]fs.ReceivedFile)
	}
	for _, f := range m.RequestedFiles {
		f.InitFuseStableAttr()
		f.InitModTime()
		m.fileLookUpMap[f.Stack][f.Name] = f
	}
	for _, layer := range m.fileLookUpMap {
		layer["."] = layer[""]
		delete(layer, "")
		layer["."].(*receive.ReferencedFile).Name = "."
		for fn, f := range layer {
			if fn == "." {
				continue
			}
			base := filepath.Base(fn)
			d := path.Dir(fn)
			if p, has := layer[d]; has {
				if base == ".wh..wh..opq" {
					if p.(*receive.ReferencedFile).Xattrs == nil {
						p.(*receive.ReferencedFile).Xattrs = make(map[string][]byte)
					}
					p.(*receive.ReferencedFile).Xattrs["trusted.overlay.opaque"] = []byte("y")
					delete(layer, fn)
					continue
				}

				p.AppendChild(f)
				if strings.HasPrefix(base, ".wh.") {
					f.(*receive.ReferencedFile).Name = filepath.Join(d, strings.TrimPrefix(base, ".wh."))
					f.(*receive.ReferencedFile).DevMinor = 0
					f.(*receive.ReferencedFile).DevMajor = 0
					f.(*receive.ReferencedFile).GID = 0
					f.(*receive.ReferencedFile).UID = 0
					f.(*receive.ReferencedFile).Mode = 0x2000
				}
			}

		}
	}

}

func (m *Manager) SetOptimizerOn(optimizeGroup, imageDigest string) (err error) {
	m.tracer, err = fs.NewTracer(optimizeGroup, imageDigest)
	return
}

func (m *Manager) Teardown() {
	if m.tracer != nil {
		_ = m.tracer.Close()
	}
	for _, v := range m.fs {
		_ = v.Teardown()
	}
}

// NewStarlightFS creates FUSE server and mount to provided mount directory
func (m *Manager) NewStarlightFS(mount string, stack int64, options *fusefs.Options, debug bool) (f *fs.Instance, err error) {
	has := false
	if f, has = m.fs[stack]; has {
		_ = f.Teardown()
	}
	f, err = fs.NewInstance(m, m.fileLookUpMap[stack]["."], stack, mount, options, debug)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new filesystem instance")
	}
	m.fs[stack] = f
	return
}
