/*
   file created by Junlin Chen in 2022

*/

package fs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"syscall"
	"time"
)

type ImageManager interface {
	GetPathByLayer(stack int64) string
	LookUpFile(stack int64, filename string) ReceivedFile
	LogTrace(stack int64, filename string, access, complete time.Time)
}

// Instance should be created using
type Instance struct {
	Root      ReceivedFile
	rootInode *StarlightFsNode

	stack      int64
	mountPoint string

	manager ImageManager
	server  *fuse.Server
}

func (fi *Instance) GetMountPoint() string   { return fi.mountPoint }
func (fi *Instance) GetServer() *fuse.Server { return fi.server }

// Teardown unmounts the file system and close the logging file if there is one writing
func (fi *Instance) Teardown() error {
	return fi.GetServer().Unmount()
}

func (fi *Instance) Serve() {
	fi.server.Serve()
}

func NewInstance(m ImageManager, root ReceivedFile, stack int64, dir string, options *fs.Options, debug bool) (fi *Instance, err error) {
	fi = &Instance{
		manager: m,
		stack:   stack,
	}

	fi.rootInode = &StarlightFsNode{fs.Inode{}, root, fi}
	fi.mountPoint = dir

	one := time.Second

	options.EntryTimeout = &one
	options.AttrTimeout = &one
	options.AllowOther = true
	options.Name = "starlightfs"
	options.Options = []string{"suid", "ro"}
	options.DirectMount = true
	options.NullPermissions = true
	options.RememberInodes = false

	if debug {
		options.Debug = true
	}

	_ = syscall.Unmount(dir, UnmountFlag)
	rawFs := fs.NewNodeFS(fi.rootInode, options)
	fi.server, err = fuse.NewServer(rawFs, dir, &options.MountOptions)

	return fi, err
}
