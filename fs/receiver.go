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
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/mc256/starlight/util"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sync"
)

type Receiver struct {
	ctx        context.Context
	layerStore *LayerStore

	// image are a list of gzip chunks (excluding the TOC header)
	image io.Reader

	// fsTemplate is a tree structure TOC. In case we want to create a new instance in fsInstances,
	// we must perform a deep-copy of this tree.
	fsTemplate []TocEntry
	// fsInstances that associates with this ImageReader.
	fsInstances []*FsInstance

	// imageMap helps you to find the corresponding index in other data structure
	imageMap map[string]int
	// layerMap helps you to find the corresponding path to the directory
	layerMap []*LayerMeta

	// entryMap allow us to flag whether a file is ready to read
	// structure:|-- [0] ------------------------[16]--------------------- ... from  imageMeta . Offsets
	//                |- []{entry1, entry2}       |-[]{entry3, entry4}
	// [-1] records all the files that has size 0
	entryMap map[int64]*[]TocEntry
	offsets  []int64

	// Image protocol
	name   string
	prefix string
}

func (r *Receiver) GetLayerMounts() []mount.Mount {
	m := make([]mount.Mount, 0, len(r.layerMap))
	for _, p := range r.layerMap {
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
// - optimize determines whether the file system should collect access traces for optimization
func (r *Receiver) NewFsInstance(imageName, imageTag, snapshotId string, optimize bool, optimizeGroup string) (*FsInstance, error) {
	// Fs ID
	randBuf := make([]byte, 64)
	_, _ = rand.Read(randBuf)
	d := digest.FromBytes(randBuf)

	// Prepare RW layer
	lm, err := r.layerStore.RegisterLayerWithAbsolutePath(util.TraceableBlobDigest{
		Digest:    digest.FromString(snapshotId),
		ImageName: util.UserRwLayerText,
	}, NewLayerMeta(path.Join(snapshotId, "rw"), true, true))
	if err != nil {
		return nil, err
	}
	log.G(r.ctx).WithField("rw-layer", lm).Info("prepared rw layer")

	// Build Fs Tree (deep copy)
	idx, hasImage := r.imageMap[imageName+":"+imageTag]
	if !hasImage {
		return nil, util.ErrImageNotFound
	}
	fsi := newFsInstance(r, &r.layerMap, d, lm.absPath, imageName, imageTag)

	// Optimize
	if optimize {
		if err = fsi.SetOptimizerOn(optimizeGroup); err != nil {
			log.G(r.ctx).WithError(err).Error("optimizer error")
		} else {
			log.G(r.ctx).WithField("group", optimizeGroup).Info("optimizer on")
		}
	} else {
		log.G(r.ctx).Info("optimizer off")
	}

	// Filesystem Instance
	fsi.Root = r.fsTemplate[idx].(*TemplateEntry).DeepCopy(fsi)

	return fsi, nil
}

func (r *Receiver) printLog(fn, dir, base, ref, comment string) {
	/*
		if superSuperHot[fn] {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": fn,
				"dir":      dir,
				"base":     base,
				"ref":      ref,
			}).Error(comment)
		}
	*/
	/*
		if comment == "pre-extract" {
			log.G(ir.ctx).WithFields(logrus.Fields{
				"filename": fn,
				"dir":      dir,
				"base":     base,
				"ref":      ref,
			}).Error(comment)
		}
	*/
}

// touchFile deal with the files with size 0
func (r *Receiver) touchFile(wg *sync.WaitGroup) {
	defer wg.Done()

	if _, has := r.entryMap[-1]; !has {
		return
	}
	for _, entS := range *r.entryMap[-1] {
		ent, isTemplate := entS.(*TemplateEntry)
		if isTemplate == false {
			continue
		}

		if ent.Size != 0 {
			continue
		}

		// Source Layer
		var src int
		if src := ent.Source - 1; src < 0 {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
			}).Warn("entry src less than 0")
			continue
		}

		// File Name
		dir := path.Join(r.layerMap[src].GetAbsPath(), path.Dir(ent.Name))
		base := path.Base(ent.Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("mkdir error")
			continue
		}

		// Openfile
		if f, err := os.OpenFile(path.Join(dir, base), os.O_CREATE|os.O_WRONLY, 0644); err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("open error")
			continue
		} else {
			f.Close()
		}
		close(ent.ready)
	}
}

func (r *Receiver) decompress(wg *sync.WaitGroup, cur, size int64, buf *bytes.Buffer) {
	defer wg.Done()

	// gzip reader
	gr, err := gzip.NewReader(buf)
	if err != nil {
		log.G(r.ctx).WithField("error", err).Error("image reader error")
		return
	}

	// entry map
	if _, has := r.entryMap[cur]; !has {
		_ = gr.Close()
		return
	}

	oldName := ""
	for _, entS := range *r.entryMap[cur] {
		ent, isTemplate := entS.(*TemplateEntry)
		// send landmark signal
		if isTemplate == false {
			continue
		}

		// skip size zero
		r.printLog(ent.Name, "", fmt.Sprintf("%d", cur), oldName, "pre-extract")
		if ent.Size == 0 {
			continue
		}

		// Layer Position - this should never happen
		src := ent.Source - 1
		if src < 0 || src >= len(r.layerMap) {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"src":      src,
			}).Warn("entry src less than 0 or larger than expected")
			continue
		}

		// Path
		dir := path.Join(r.layerMap[src].GetAbsPath(), path.Dir(ent.Name))
		base := path.Base(ent.Name)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("mkdir error")
			continue
		}

		// Duplicated file uses hard link
		if oldName != "" { // has been extracted before
			if r.layerMap[src].AtomicIsComplete() {
				continue
			}

			if err = os.Link(oldName, path.Join(dir, base)); err != nil {
				log.G(r.ctx).WithFields(logrus.Fields{
					"filename": ent.Name,
					"err":      err,
				}).Warn("ln error")
			}

			r.printLog(ent.Name, dir, base, oldName, "hard linked")
			close(ent.ready)
			continue
		}

		// If the layer has been extracted
		if r.layerMap[src].AtomicIsComplete() {
			oldName = path.Join(dir, base)
			continue
		}

		// Decompress Gzip chunks
		var f *os.File
		if f, err = os.OpenFile(path.Join(dir, base), os.O_CREATE|os.O_WRONLY, 0644); err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("open error")
			continue
		}
		if _, err := io.Copy(f, gr); err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("copy error")
			continue
		}

		if err = gr.Close(); err != nil {
			log.G(r.ctx).WithFields(logrus.Fields{
				"filename": ent.Name,
				"err":      err,
			}).Warn("gz close error")
			continue
		}
		f.Close()

		r.printLog(ent.Name, dir, base, oldName, "extracted")

		close(ent.ready)
		oldName = path.Join(dir, base)
	}
}

func (r *Receiver) extractFiles() {
	var wg sync.WaitGroup

	if len(r.offsets) == 0 {
		return
	}
	cur := r.offsets[0]

	// empty files
	wg.Add(1)
	go r.touchFile(&wg)

	// non-empty files
	for i := 1; i < len(r.offsets); i++ {
		next := r.offsets[i]

		// size is zero that means empty files
		size := next - cur
		if size == 0 {
			// this should not happen
			panic("0 size data file should be handled by another function")
		}

		// size is non-zero that means we have to extract this file
		buffer := bytes.NewBuffer(make([]byte, 0, size))
		if n, err := io.CopyN(buffer, r.image, size); n != size || err != nil {
			log.G(r.ctx).WithField("error", err).Error("image reader error")
			return
		}

		wg.Add(1)
		go r.decompress(&wg, cur, size, buffer)

		cur = next
	}

	log.G(r.ctx).WithFields(logrus.Fields{
		"name": r.name,
	}).Info("received entire image")

	wg.Wait()

	for _, lm := range r.layerMap {
		if err := lm.AtomicSetCompleted(); err != nil {
			log.G(r.ctx).WithError(err).Error("set layer to completed")
		} else {
			log.G(r.ctx).WithFields(logrus.Fields{
				"path": lm.absPath,
				"d":    lm.digest,
			}).Debug("layer is ready")
		}
	}

	log.G(r.ctx).WithFields(logrus.Fields{
		"name": r.name,
	}).Info("entire image extracted")
}

func (r *Receiver) ExtractFiles() {
	go r.extractFiles()
}

// NewReceiver reads image from reader and save it to layerStore
//
// - ctx: context
// - layerStore: stores the layers
// - reader: image reader
// - tocOffset: size of the TOC in reader
// - prefix: to store the layers in one single folder (snapshot id)
func NewReceiver(ctx context.Context, layerStore *LayerStore, reader io.Reader, headerOffset int64, prefix string) (*Receiver, error) {
	// Delta bundle header
	headerGzBuf := bytes.NewBuffer(make([]byte, 0, headerOffset))
	if n, err := io.CopyN(headerGzBuf, reader, headerOffset); n != headerOffset || err != nil {
		return nil, err
	}
	headerReader, err := gzip.NewReader(headerGzBuf)
	if err != nil {
		return nil, err
	}
	headerBuf, err := ioutil.ReadAll(headerReader)
	if err != nil {
		return nil, err
	}
	header := &util.Protocol{}
	err = json.Unmarshal(headerBuf, header)
	if err != nil {
		return nil, err
	}

	// Receiver
	r := &Receiver{
		ctx:        context.Background(),
		layerStore: layerStore,
		image:      reader,
		offsets:    header.Offsets,

		layerMap: make([]*LayerMeta, 0, len(header.DigestList)), // #2
		imageMap: make(map[string]int),                          // #1
		entryMap: make(map[int64]*[]TocEntry),                   // #3

		fsTemplate:  make([]TocEntry, 0, len(header.Images)), // #3
		fsInstances: make([]*FsInstance, 0),                  // Snapshotter

		name:   util.ByImageName(header.Images).String(),
		prefix: prefix,
	}

	// #1 -Image config file
	for i, ref := range header.Images {
		r.imageMap[ref.String()] = i

		log.G(ctx).WithFields(logrus.Fields{
			"name": ref.ImageName,
			"tag":  ref.ImageTag,
		}).Debug("found delta image")

		if cf, err := os.OpenFile(path.Join(layerStore.GetWorkDir(), ref.JsonConfigFile()), os.O_WRONLY|os.O_CREATE, 0644); err == nil {
			if _, err := cf.WriteString(header.Configs[i]); err != nil {
				return nil, err
			}
			if err = cf.Close(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// #2 - Prepare RO layers
	count := 0
	for _, d := range header.DigestList {
		var layer *LayerMeta
		layer, err = layerStore.FindLayer(*d)
		if err == util.ErrLayerNotFound {
			count++
			layer, err = layerStore.RegisterLayerWithPrefix(prefix, count, *d, false, false)
		} else if err != nil {
			return nil, err
		}

		r.layerMap = append(r.layerMap, layer)
	}

	// #3 Extract content
	for _, table := range header.Tables {
		root := &TemplateEntry{
			Entry{
				TraceableEntry: util.GetRootNode(),
				State:          EnRwLayer,
			},
		}

		dirBuffer := make(map[string]*TemplateEntry)
		dirBuffer["."] = root
		entBuffer := make([]*TemplateEntry, 0)

		r.fsTemplate = append(r.fsTemplate, root)

		for _, ent := range table {
			if ent.Name == "." {
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

			if temp.IsDir() {
				dirBuffer[temp.Name] = temp
			} else {
				entBuffer = append(entBuffer, temp)
			}

			if !temp.IsDataType() {
				continue
			}

			// Size 0 regular file
			if temp.Size == 0 {
				if ent.Source <= 0 {
					temp.State = EnRwLayer
				} else if layer := r.layerMap[ent.Source-1]; layer.AtomicIsComplete() {
					temp.State = EnRoLayer
				} else {
					temp.State = EnEmpty
					temp.ready = make(chan bool, 1)
					if n, ok := r.entryMap[-1]; ok {
						*n = append(*n, temp)
					} else {
						r.entryMap[-1] = &[]TocEntry{temp}
					}
				}
			}

			// Size non-0 regular file
			if temp.DeltaOffset != nil && len(*temp.DeltaOffset) > 0 {
				if ent.Source <= 0 {
					temp.State = EnRwLayer
				} else if layer := r.layerMap[ent.Source-1]; layer.AtomicIsComplete() {
					temp.State = EnRoLayer
				} else {
					temp.State = EnEmpty
					temp.ready = make(chan bool, 1)
					offset := (*temp.DeltaOffset)[0]
					if n, ok := r.entryMap[offset]; ok {
						*n = append(*n, temp)
					} else {
						r.entryMap[offset] = &[]TocEntry{temp}
					}
				}
			}
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

	}

	return r, nil
}

func NewReceiverFromFile(ctx context.Context, layerStore *LayerStore, filename string, headerOffset int64) (*Receiver, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	sr := io.NewSectionReader(f, 0, fi.Size())
	return NewReceiver(ctx, layerStore, io.Reader(sr), headerOffset, "")
}
