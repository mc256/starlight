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

package fs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
)

type ImageReader struct {
	ctx        context.Context
	db         *bolt.DB
	layerStore *LayerStore

	// image are a list of gzip chunks (excluding the TOC header)
	image     io.Reader
	tocOffset int64

	// fsTemplate is a tree structure TOC. In case we want to create a new instance in fsInstances,
	// we must perform a deep-copy of this tree.
	fsTemplate []TocEntry
	// fsInstances that associates with this ImageReader.
	fsInstances []*FsInstance

	// imageLookupMap helps you to find the corresponding merger.Delta in imageMeta
	imageLookupMap map[string]int
	// layerLookupMap helps you to find the corresponding path to the directory
	layerLookupMap []*LayerMeta

	// entryMap allow us to flag whether a file is ready to read
	// structure:|-- [0] ------------------------[16]--------------------- ... from  imageMeta . Offsets
	//                |- []{entry1, entry2}       |-[]{entry3, entry4}
	entryMap     map[int64]*[]TocEntry
	chunkOffsets []int64

	// Image protocol
	name   string
	prefix string

	checkpointWaitPool map[string]*LandmarkEntry
}

//////////////////////////////////////////////////////////////////
// Protocol

// Name of the image
func (ir *ImageReader) Name() string {
	return ir.name
}

// Target descriptor for the image content
func (ir *ImageReader) Target() ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: util.ImageMediaType,
		Size:      ir.tocOffset,
		Platform: &ocispec.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
	}
}

// Labels of the image
func (ir *ImageReader) Labels() map[string]string {
	return map[string]string{}
}

// Unpack unpacks the image's content into a snapshot
func (ir *ImageReader) Unpack(context.Context, string, ...containerd.UnpackOpt) error {
	panic("Unpack not implemented")
}

// RootFS returns the unpacked diffids that make up images rootfs.
func (ir *ImageReader) RootFS(ctx context.Context) ([]digest.Digest, error) {
	return []digest.Digest{}, nil
}

func (ir *ImageReader) getUniqueDigest() digest.Digest {
	return digest.FromString(fmt.Sprintf("%s-%d", ir.name, ir.tocOffset))
}

// Size returns the total size of the image's packed resources.
func (ir *ImageReader) Size(ctx context.Context) (int64, error) {
	panic("Size not implemented")
}

// Usage returns a usage calculation for the image.
func (ir *ImageReader) Usage(context.Context, ...containerd.UsageOpt) (int64, error) {
	panic("Usage not implemented")
}

// Config descriptor for the image.
func (ir *ImageReader) Config(ctx context.Context) (ocispec.Descriptor, error) {
	return ir.Target(), nil
}

// IsUnpacked returns whether or not an image is unpacked.
func (ir *ImageReader) IsUnpacked(context.Context, string) (bool, error) {
	panic("IsUnpack not implemented")
}

// ContentStore provides a content store which contains image blob data
func (ir *ImageReader) ContentStore() content.Store {
	panic("ContentStore not implemented")
}

// Metadata returns the underlying image metadata
func (ir *ImageReader) Metadata() images.Image {
	panic("Metadata not implemented")
}

func (ir *ImageReader) GetLayerMounts() []mount.Mount {
	m := make([]mount.Mount, 0, len(ir.layerLookupMap))
	for _, p := range ir.layerLookupMap {
		m = append(m, mount.Mount{
			Type:   "bind",
			Source: p.GetAbsPath(),
			Options: []string{
				"ro",
				"rbind",
			},
		})
	}
	return m
}

/////////////////////////////////////////////////////////////////
// NewFsInstance creates new file system instance,
// - imageName, imageTag  should be available in ImageReader. imageLookupMap
// - snapshotId is the absolute path will be created to hold the rw layer and the mounting point.
// - checkpoint is the starting point for the image reader
func (ir *ImageReader) NewFsInstance(imageName, imageTag, snapshotId, checkpoint string) (*FsInstance, error) {
	// Fs ID
	randBuf := make([]byte, 64)
	_, _ = rand.Read(randBuf)
	d := digest.FromBytes(randBuf)

	// Prepare RW layer
	lm, err := ir.layerStore.RegisterLayerWithAbsolutePath(util.TraceableBlobDigest{
		Digest:    digest.FromString(snapshotId),
		ImageName: util.UserRwLayerText,
	}, NewLayerMeta(path.Join(snapshotId, "rw"), true, true))
	if err != nil {
		return nil, err
	}
	log.G(ir.ctx).WithField("rw-layer", lm).Info("prepared rw layer")

	// Build Fs Tree (deep copy)
	idx, hasImage := ir.imageLookupMap[imageName+":"+imageTag]
	if !hasImage {
		return nil, util.ErrImageNotFound
	}
	fsi := newFsInstance(ir, &ir.layerLookupMap, d, lm.absPath, imageName, imageTag)
	fsi.Root = ir.fsTemplate[idx].(*TemplateEntry).DeepCopy(fsi)

	// Wait for
	if checkpoint == "v1" || checkpoint == "v2" || checkpoint == "v3" {
		kw := fmt.Sprintf("Landmark-%s:%s-%s", imageName, imageTag, checkpoint)
		if ent, exist := ir.checkpointWaitPool[kw]; exist {
			log.G(ir.ctx).WithField("checkpoint", kw).Info("wait on checkpoint")
			<-ent.ready
			log.G(ir.ctx).WithField("checkpoint", kw).Info("finished waiting")
		}
	}
	return fsi, nil
}

// touchFile deal with the files with size 0
func (ir *ImageReader) touchFile(wg *sync.WaitGroup) {
	defer wg.Done()

	if _, has := ir.entryMap[-1]; !has {
		return
	}
	for _, entS := range *ir.entryMap[-1] {
		ent, isTemplate := entS.(*TemplateEntry)
		// send landmark signal
		if isTemplate == false {
			continue
		}

		if ent.Size != 0 {
			continue
		}

		// Source Layer
		src := ent.Source - 1
		if src < 0 {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
			}).Warn("entry src less than 0")
			continue
		}

		// File Name
		dir := path.Join(ir.layerLookupMap[src].GetAbsPath(), path.Dir(ent.Name))
		base := path.Base(ent.Name)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("mkdir error")
			continue
		}

		// Openfile
		f, err := os.OpenFile(path.Join(dir, base), os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("open error")
			continue
		}

		f.Close()

		/*
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"dir":      dir,
				"base":     base,
				"size":     ent.Size,
			}).Trace("extracted")
		*/

		close(ent.ready)
	}
}

func (ir *ImageReader) printLog(fn, dir, base, ref, comment string) {
	log.G(ir.ctx).WithFields(logrus.Fields{
		"filename": fn,
		"dir":      dir,
		"base":     base,
		"ref":      ref,
	}).Trace(comment)
}

func (ir *ImageReader) decompress(wg *sync.WaitGroup, cur, size int64, buf *bytes.Buffer) {
	defer wg.Done()

	// gzip reader
	gr, err := gzip.NewReader(buf)
	if err != nil {
		log.G(ir.ctx).WithField("error", err).Error("image reader error")
		return
	}

	// entry map
	if _, has := ir.entryMap[cur]; !has {
		_ = gr.Close()
		return
	}

	// extracted
	oldName := ""
	for _, entS := range *ir.entryMap[cur] {
		ent, isTemplate := entS.(*TemplateEntry)
		// send landmark signal
		if isTemplate == false {
			landmark := entS.(*LandmarkEntry)
			if landmark.checkpoint == "v3" {
				continue
			}
			close(landmark.ready)
			//ir.printLog(landmark.image, landmark.checkpoint, "", "", "landmark signal sent")
			log.G(ir.ctx).WithFields(logrus.Fields{
				"img":        landmark.image,
				"checkpoint": landmark.checkpoint,
			}).Info("landmark signal sent")
			continue
		}

		// skip size zero
		ir.printLog(ent.Name, "", "", oldName, "pre-extract")
		if ent.Size == 0 {
			continue
		}

		// Layer Position - this should never happen
		src := ent.Source - 1
		if src < 0 || src >= len(ir.layerLookupMap) {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"src":      src,
			}).Warn("entry src less than 0 or larger than expected")
			continue
		}

		// Path
		dir := path.Join(ir.layerLookupMap[src].GetAbsPath(), path.Dir(ent.Name))
		base := path.Base(ent.Name)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("mkdir error")
			continue
		}

		// Duplicated file uses hard link
		if oldName != "" { // has been extracted before
			if ir.layerLookupMap[src].AtomicIsComplete() {
				continue
			}

			if err = os.Link(oldName, path.Join(dir, base)); err != nil {
				log.G(ir.ctx).WithFields(logrus.Fields{
					"filename": ent.Name,
					"err":      err,
				}).Warn("ln error")
			}

			ir.printLog(ent.Name, dir, base, oldName, "hard linked")
			close(ent.ready)
			continue
		}

		// If the layer has been extracted
		if ir.layerLookupMap[src].AtomicIsComplete() {
			oldName = path.Join(dir, base)
			continue
		}

		// Decompress Gzip chunks
		var f *os.File
		if f, err = os.OpenFile(path.Join(dir, base), os.O_CREATE|os.O_WRONLY, 0644); err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("open error")
			continue
		}
		if _, err := io.Copy(f, gr); err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("copy error")
			continue
		}

		if err = gr.Close(); err != nil {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("gz close error")
			continue
		}
		f.Close()

		ir.printLog(ent.Name, dir, base, oldName, "extracted")

		close(ent.ready)
		oldName = path.Join(dir, base)
	}

}

func (ir *ImageReader) extractFiles() {
	if len(ir.chunkOffsets) == 0 {
		return
	}

	cur := ir.chunkOffsets[0]

	var wg sync.WaitGroup

	// empty files
	wg.Add(1)
	go ir.touchFile(&wg)

	// non-empty files
	for i := 1; i < len(ir.chunkOffsets); i++ {
		next := ir.chunkOffsets[i]

		// size is zero that means empty files
		size := next - cur
		if size == 0 { // this should not happen
			panic("0 size data file should be handled by another function")
		}

		// size is non-zero that means we have to extract this file
		buffer := bytes.NewBuffer(make([]byte, 0, size))
		if n, err := io.CopyN(buffer, ir.image, size); n != size || err != nil {
			log.G(ir.ctx).WithField("error", err).Error("image reader error")
			return
		}

		wg.Add(1)
		go ir.decompress(&wg, cur, size, buffer)

		cur = next
	}

	log.G(ir.ctx).WithFields(logrus.Fields{
		"name": ir.name,
	}).Info("received entire image")

	wg.Wait()

	for _, lm := range ir.layerLookupMap {
		if err := lm.AtomicSetCompleted(); err != nil {
			log.G(ir.ctx).WithError(err).Error("set layer to completed")
		} else {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"path": lm.absPath,
				"d":    lm.digest,
			}).Debug("layer is ready")
		}
	}

	log.G(ir.ctx).WithFields(logrus.Fields{
		"name": ir.name,
	}).Info("entire image extracted")

	for k, v := range ir.checkpointWaitPool {
		if strings.HasSuffix(k, "v3") {
			close(v.ready)
		}
	}
}

func (ir *ImageReader) ExtractFiles() {
	go ir.extractFiles()
}

// NewImageReader reads image from reader and save it to layerStore
//
// - ctx: context
// - layerStore: stores the layers
// - reader: image reader
// - tocOffset: size of the TOC in reader
// - prefix: to store the layers in one single folder
func NewImageReader(ctx context.Context, layerStore *LayerStore, reader io.Reader, tocOffset int64, prefix string) (*ImageReader, error) {
	tocBufGz := bytes.NewBuffer(make([]byte, 0, tocOffset))
	if n, err := io.CopyN(tocBufGz, reader, tocOffset); n != tocOffset || err != nil {
		return nil, err
	}
	tocR, err := gzip.NewReader(tocBufGz)
	if err != nil {
		return nil, err
	}
	tocBuf, err := ioutil.ReadAll(tocR)
	if err != nil {
		return nil, err
	}

	ir := &ImageReader{
		ctx:        context.Background(), // Need to test
		layerStore: layerStore,
		db:         nil,
		image:      reader,
		tocOffset:  0,

		fsInstances:    make([]*FsInstance, 0),
		imageLookupMap: make(map[string]int),

		prefix: prefix,
	}

	// Parse image header json
	var c merger.Consolidator
	err = json.Unmarshal(tocBuf, &c)
	if err != nil {
		return nil, err
	}

	// Save TOC for debugging
	/*
		if f, err := os.OpenFile("/tmp/toc.json", os.O_WRONLY|os.O_CREATE, 0755); err == nil {
			_, _ = f.Write(tocBuf)
			_ = f.Close()
		}
	*/

	imageNames := make([]string, 0, len(c.Deltas))
	for i, d := range c.Deltas {
		ir.imageLookupMap[d.ImageName+":"+d.ImageTag] = i
		imageNames = append(imageNames, d.ImageName+":"+d.ImageTag)
		log.G(ctx).WithFields(logrus.Fields{
			"checkpoints": d.CheckPoint,
			"name":        d.ImageName,
			"tag":         d.ImageTag,
		}).Debug("found delta image")

		// Save Image Config File
		if cf, err := os.OpenFile(path.Join(layerStore.GetWorkDir(), fmt.Sprintf("%s_%s.json", d.ImageName, d.ImageTag)), os.O_WRONLY|os.O_CREATE, 0644); err == nil {
			if _, err := cf.WriteString(d.Config); err != nil {
				return nil, err
			}
			_ = cf.Close()
		} else {
			return nil, err
		}

	}
	ir.name = strings.Join(imageNames, ",")

	// Prepare RO layers
	ir.layerLookupMap = make([]*LayerMeta, 0, len(c.Source))
	count := 0
	for _, bd := range c.Source {
		var lm *LayerMeta
		lm, err = layerStore.FindLayer(*bd)
		if err == util.ErrLayerNotFound {
			count++
			lm, err = layerStore.RegisterLayerWithPrefix(prefix, count, *bd, false, false)
		}
		if err != nil {
			return nil, err
		}
		ir.layerLookupMap = append(ir.layerLookupMap, lm)
	}

	// Prepare for extract contents
	ir.fsTemplate = make([]TocEntry, 0, len(c.Deltas))
	ir.entryMap = make(map[int64]*[]TocEntry, len(c.Offsets))
	ir.checkpointWaitPool = make(map[string]*LandmarkEntry, len(c.Deltas)*3)
	ir.chunkOffsets = c.Offsets

	// Connects
	for _, d := range c.Deltas {
		// root node
		root := &TemplateEntry{
			Entry{
				TraceableEntry: util.GetRootNode(),
				State:          EnRwLayer,
			},
		}
		dirBuffer := make(map[string]*TemplateEntry)
		dirBuffer["."] = root
		entBuffer := make([]*TemplateEntry, 0)

		ir.fsTemplate = append(ir.fsTemplate, root)

		// priorities
		for _, pl := range d.Pool {
			for _, ent := range pl {
				if ent.Name == "." { // weird bug should not happen
					continue
				}

				// create template entry
				temp := &TemplateEntry{
					Entry: Entry{
						TraceableEntry: ent,
						State:          EnRoLayer,
						ready:          nil,
					},
				}

				//put it into buffer if it is directory
				if temp.IsDir() {
					dirBuffer[ent.Name] = temp
				} else {
					entBuffer = append(entBuffer, temp)
				}

				// Get ready for file extraction
				if !temp.IsDataType() {
					continue
				}

				if temp.Size == 0 {
					if ent.Source <= 0 {
						temp.State = EnRwLayer
						// TODO: this wipe out file should be written to the RW layer, but not yet implemented
					} else if lm := ir.layerLookupMap[ent.Source-1]; lm.AtomicIsComplete() {
						temp.State = EnRoLayer
					} else {
						temp.State = EnEmpty
						temp.ready = make(chan bool, 1)
						if p, ok := ir.entryMap[-1]; ok {
							*p = append(*p, temp)
						} else {
							ir.entryMap[-1] = &[]TocEntry{temp}
						}
					}
					continue
				}

				if temp.DeltaOffset != nil && len(*temp.DeltaOffset) > 0 {
					if lm := ir.layerLookupMap[ent.Source-1]; lm.AtomicIsComplete() {
						temp.State = EnRoLayer
					} else {
						temp.State = EnEmpty
						temp.ready = make(chan bool, 1)
						offset := (*temp.DeltaOffset)[0]
						if p, ok := ir.entryMap[offset]; ok {
							*p = append(*p, temp)
						} else {
							ir.entryMap[offset] = &[]TocEntry{temp}
						}
					}
				}
			}

			//

		}

		// Connect children to parent and parent to children
		for fileName, ent := range dirBuffer {
			if fileName == "." {
				continue
			}
			dir := path.Dir(fileName)
			if parent, exist := dirBuffer[dir]; !exist {
				return nil, util.ErrOrphanNode
			} else {
				base := path.Base(fileName)
				parent.AddChild(base, ent)
				ent.parent = parent
			}

		}
		for _, ent := range entBuffer {
			dir := path.Dir(ent.Name)
			if parent, exist := dirBuffer[dir]; !exist {
				return nil, util.ErrOrphanNode
			} else {
				base := path.Base(ent.Name)
				parent.AddChild(base, ent)
				ent.parent = parent
			}
		}

		// Add Landmark Entry for signaling
		for landmark, offset := range d.CheckPoint {
			temp := &LandmarkEntry{
				image:      d.ImageName + ":" + d.ImageTag,
				checkpoint: fmt.Sprintf("v%d", landmark+1),
				ready:      make(chan bool),
			}
			ir.checkpointWaitPool[temp.String()] = temp
			if p, ok := ir.entryMap[offset]; ok {
				*p = append(*p, temp)
			} else {
				ir.entryMap[offset] = &[]TocEntry{temp}
			}
		}
	}

	//go ir.extractFiles()

	return ir, nil
}

func NewImageReaderFromFile(ctx context.Context, layerStore *LayerStore, filename string, tocOffset int64) (*ImageReader, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	sr := io.NewSectionReader(f, 0, fi.Size())
	return NewImageReader(ctx, layerStore, io.Reader(sr), tocOffset, "")
}
