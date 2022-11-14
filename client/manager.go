/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/snapshots"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/client/snapshotter"
	"github.com/mc256/starlight/util/receive"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager should be unmarshalled from a json file and then Populate() should be called to populate other fields
type Manager struct {
	receive.DeltaBundle
	ctx context.Context

	// non-exported fields
	// compressLayerDigest []digest.Digest
	// diffDigest          []digest.Digest

	cfg *Configuration

	//layers is a map from filesystem serial to receive.ImageLayer object
	//  - should it require files from layers that are not in this image,
	//    checkout layers using the serial number
	layers map[int64]*receive.ImageLayer

	//stackSerialMap is a map convert stack to filesystem serial (on the proxy side),
	// it then can be use by layers to get the receive.ImageLayer object
	stackSerialMap []int64
	completedStack []bool

	imageConfig    *v1.Image
	manifest       *v1.Manifest
	manifestDigest digest.Digest

	fileLookUpMap []map[string]fs.ReceivedFile

	tracerLock sync.Mutex
	tracer     *fs.Tracer

	operator *snapshotter.Operator

	fs map[int64]*fs.Instance
}

func (m *Manager) String() string {
	return fmt.Sprintf("Manager{Image: %s, Layers: %d}", m.manifestDigest, len(m.layers))
}

type completionMessage struct {
	Start    time.Time `json:"start"`
	Complete time.Time `json:"complete"`
}

func (m *Manager) getDirectory(layerHash string) string {
	return filepath.Join(m.cfg.FileSystemRoot, "layers", layerHash[7:8], layerHash[8:10], layerHash[10:12], layerHash[12:])
}

func (m *Manager) ignoreStack(stack int64) bool {
	return m.completedStack[stack]
}

func (m *Manager) GetPathByStack(stack int64) string {
	return m.GetPathBySerial(m.stackSerialMap[stack])
}

func (m *Manager) GetPathBySerial(serial int64) string {
	return m.layers[serial].Local
}

func (m *Manager) LookUpFile(stack int64, filename string) fs.ReceivedFile {
	if file, has := m.fileLookUpMap[stack][filename]; has {
		return file
	}
	return nil
}

func (m *Manager) LogTrace(stack int64, filename string, access, complete time.Time) {
	m.tracerLock.Lock()
	defer m.tracerLock.Unlock()
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

func (m *Manager) Extract(r *io.ReadCloser) (err error) {
	start := time.Now()
	if r == nil {
		return fmt.Errorf("cannot call extract if it has completed")
	}

	var (
		p, pp string
	)
	// Extract Contents
	for i, c := range m.Contents {
		// skip the layer if it already exists
		if m.ignoreStack(c.Stack) {
			totalCompressedSize := int64(0)
			for _, ch := range c.Chunks {
				totalCompressedSize += ch.CompressedSize
			}
			if n, err := io.CopyN(io.Discard, *r, totalCompressedSize); err != nil || n != totalCompressedSize {
				return errors.Wrapf(err, "failed to discard %d bytes", totalCompressedSize)
			}
			continue
		}

		// regular extraction
		p = m.GetPathByStack(c.Stack)
		if err := os.MkdirAll(filepath.Join(p, c.GetBaseDir()), 0755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", filepath.Join(p, c.GetBaseDir()))
		}
		pp = c.GetPath()
		err = func() error {
			f, err := os.Create(filepath.Join(p, pp))
			isClosed := false
			defer func() {
				if !isClosed {
					_ = f.Close()
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
			_ = f.Close()
			isClosed = true
			close(c.Signal) // send out signal that this content is ready
			return nil
		}()

		if err != nil {
			return err
		}
		log.G(m.ctx).
			WithField("l", p).
			WithField("f", pp).
			Trace("extracted")
	}

	complete := time.Now()
	for idx, layer := range m.Destination.Layers {
		msg := &completionMessage{
			Start:    start,
			Complete: complete,
		}
		buf, _ := json.Marshal(msg)
		if err := ioutil.WriteFile(filepath.Join(layer.Local, "completed.json"), buf, 0644); err != nil {
			return errors.Wrapf(err, "failed to mark layer %d-%s as completed", idx, layer.Hash)
		}
		m.completedStack[idx] = true
	}

	return nil
}

func (m *Manager) PrepareDirectories(c *Client) error {
	// create directories
	m.completedStack = make([]bool, len(m.stackSerialMap))
	for idx, layer := range m.Destination.Layers {

		// if it exists in the layerMap, it means the layer exists in the local filesystem, or
		// another manager process has already created it
		exists := func() bool {
			c.layerMapLock.Lock()
			defer c.layerMapLock.Unlock()
			if _, has := c.layerMap[layer.Hash]; has {
				return true
			}
			return false
		}()
		m.completedStack[idx] = exists
		if !exists {
			err := os.MkdirAll(layer.Local, 0755)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// CreateSnapshots for chainIds
// should unlock the managerMapLock before calling CreateSnapshot
func (m *Manager) CreateSnapshots(c *Client) (chainIds []digest.Digest, err error) {
	diffs := m.imageConfig.RootFS.DiffIDs
	chainIds = identity.ChainIDs(diffs)
	prev := ""

	for idx, chain := range chainIds {
		d := m.layers[m.stackSerialMap[idx]].Hash

		idx := idx
		func() {
			c.layerMapLock.Lock()
			defer c.layerMapLock.Unlock()

			if mp, has := c.layerMap[d]; has {
				if mp.manager == nil {
					// this must be loaded from the local storage
					mp.manager = m
					mp.stack = int64(idx)
				}
			} else {
				c.layerMap[d] = &mountPoint{
					fs:      nil,
					manager: m,
					stack:   int64(idx),

					snapshots: make(map[string]*snapshots.Info),
				}
			}
		}()

		_, err = m.operator.AddSnapshot(
			chain.String(), prev, m.manifestDigest.String(), d, int64(idx),
		)
		if err != nil {
			return nil, errors.Wrapf(err, "failed prepare new image snapshots %s", chain.String())
		}
		prev = chain.String()
	}
	return chainIds, nil
}

// Init populates the manager with the necessary information and data structures.
// Use json.Unmarshal to unmarshal the json file from data storage into a Manager struct.
//
//  - ready: if set to false, we will then use Extract() to get the content of the file
//  - cfg: configuration of the client
//  - image, manifest, imageConfig: information about the image (maybe we don't need this)
//
// do not change any outside state, only the manager itself
func (m *Manager) Init(ctr *containerd.Client, client *Client, ctx context.Context, cfg *Configuration, ready bool,
	manifest *v1.Manifest, imageConfig *v1.Image, manifestDigest digest.Digest) {
	// init variables
	m.ctx = ctx
	m.cfg = cfg
	m.stackSerialMap = make([]int64, 0, len(m.Destination.Layers))
	m.layers = make(map[int64]*receive.ImageLayer)
	m.fs = make(map[int64]*fs.Instance)
	m.manifest = manifest
	m.imageConfig = imageConfig
	m.manifestDigest = manifestDigest
	m.operator = snapshotter.NewOperator(ctx, client, ctr.SnapshotService("starlight"))

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

func (m *Manager) SetOptimizerOn(optimizeGroup string) (starTime time.Time, err error) {
	m.tracerLock.Lock()
	defer m.tracerLock.Unlock()

	if m.tracer == nil {
		log.G(m.ctx).Debug("manager: start tracer")
		m.tracer, err = fs.NewTracer(m.ctx, optimizeGroup, m.manifestDigest.String(), m.cfg.TracesDir)
	}
	if err != nil {
		return time.Time{}, err
	}
	return m.tracer.StartTime, nil
}

func (m *Manager) SetOptimizerOff() (duration time.Duration, err error) {
	m.tracerLock.Lock()
	defer m.tracerLock.Unlock()

	if m.tracer != nil {
		log.G(m.ctx).Debug("manager: stop tracer")
		duration, err = m.tracer.Close()
		if err != nil {
			return time.Duration(0), err
		}
		m.tracer = nil
	}
	return duration, nil
}

func (m *Manager) Teardown() {
	m.tracerLock.Lock()
	defer m.tracerLock.Unlock()

	if m.tracer != nil {
		log.G(m.ctx).Debug("manager: stop tracer")
		_, _ = m.tracer.Close()
	}
	for _, v := range m.fs {
		log.G(m.ctx).WithField("mnt", v.GetMountPoint()).Debug("manager: unmounting filesystem")
		_ = v.Teardown()
	}
}

// NewStarlightFS creates FUSE server and mount to provided mount directory
func (m *Manager) NewStarlightFS(mount string, stack int64, options *fusefs.Options, debug bool) (f *fs.Instance, err error) {
	has := false
	if f, has = m.fs[stack]; has {
		_ = f.Teardown()
	}
	f, err = fs.NewInstance(m.ctx, m, m.fileLookUpMap[stack]["."], stack, mount, options, debug)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new filesystem instance")
	}
	m.fs[stack] = f
	return
}
