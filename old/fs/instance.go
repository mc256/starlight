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
	"github.com/mc256/starlight/util/common"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/opencontainers/go-digest"
)

// Instance should be created using
type Instance struct {
	r    *Receiver
	Root *FsEntry

	layerLookupMap *[]*LayerMeta

	rwLayerHash digest.Digest
	rwLayerPath string

	name string
	tag  string

	rootInode  *StarlightFsNode
	mountPoint string

	server *fuse.Server

	optimize bool
	tracer   *Tracer
}

func (fi *Instance) GetRwTraceableBlobDigest() common.TraceableBlobDigest {
	return common.TraceableBlobDigest{
		Digest: fi.rwLayerHash, ImageName: fi.name,
	}
}

func (fi *Instance) GetRwLayerPath() string        { return fi.rwLayerPath }
func (fi *Instance) GetRwLayerHash() digest.Digest { return fi.rwLayerHash }
func (fi *Instance) GetImageName() string          { return fi.name }
func (fi *Instance) GetImageTag() string           { return fi.tag }
func (fi *Instance) GetMountPoint() string         { return fi.mountPoint }
func (fi *Instance) GetServer() *fuse.Server       { return fi.server }

func newFsInstance(r *Receiver, layerLookupMap *[]*LayerMeta, d digest.Digest, rwLayerPath, imageName, imageTag string) *Instance {
	return &Instance{
		r:              r,
		layerLookupMap: layerLookupMap,
		rwLayerHash:    d,
		rwLayerPath:    rwLayerPath,
		name:           imageName,
		tag:            imageTag,
		optimize:       false,
		tracer:         nil,
	}
}

// Teardown unmounts the file system and close the logging file if there is one writing
func (fi *Instance) Teardown() error {
	if fi.tracer != nil {
		_ = fi.tracer.Close()
	}
	return fi.GetServer().Unmount()
}

func (fi *Instance) SetOptimizerOn(optimizeGroup string) (err error) {
	fi.tracer, err = NewTracer(optimizeGroup, fi.name, fi.tag)
	fi.optimize = true
	if err != nil {
		return
	}
	return nil
}

func (fi *Instance) NewFuseServer(dir string, options *fs.Options, debug bool) (*fuse.Server, error) {
	fi.rootInode = &StarlightFsNode{
		Inode: fs.Inode{},
		Ent:   fi.Root,
	}
	fi.mountPoint = dir

	one := time.Second

	options.EntryTimeout = &one
	options.AttrTimeout = &one

	options.AllowOther = true
	options.FsName = "starlightfs"
	options.Options = []string{"suid"}
	options.NullPermissions = true

	if debug {
		options.Debug = true
	}

	rawFs := fs.NewNodeFS(fi.rootInode, options)
	var err error
	fi.server, err = fuse.NewServer(rawFs, dir, &options.MountOptions)
	return fi.server, err
}
