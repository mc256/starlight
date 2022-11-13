/*
   file created by Junlin Chen in 2022

*/

package fs

import (
	"context"
	"github.com/containerd/containerd/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	iofs "io/fs"
	"os"
	"syscall"
	"time"
)

type ImageManager interface {
	GetPathByStack(stack int64) string
	GetPathBySerial(stack int64) string
	LookUpFile(stack int64, filename string) ReceivedFile
	LogTrace(stack int64, filename string, access, complete time.Time)
}

// Instance should be created using
type Instance struct {
	ctx context.Context

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
// should you need this function, please consider using Manager.Teardown instead.
func (fi *Instance) Teardown() error {
	return fi.GetServer().Unmount()
}

func (fi *Instance) Serve() {
	fi.server.Serve()
}

func NewInstance(ctx context.Context, m ImageManager, root ReceivedFile, stack int64, dir string, options *fs.Options, debug bool) (fi *Instance, err error) {
	fi = &Instance{
		ctx:     ctx,
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

	if _, err = os.Stat(dir); err != nil && err.(*iofs.PathError).Err == syscall.ENOTCONN {
		// if the directory exists, it means that the snapshot is already created
		// or there is a problem unmounting the snapshot
		// Transport endpoint is not connected
		err = syscall.Unmount(dir, UnmountFlag)
		log.G(ctx).
			WithError(err).
			WithField("dir", dir).
			Warn("fs:  cleanup disconnected mountpoint")
	}

	rawFs := fs.NewNodeFS(fi.rootInode, options)
	fi.server, err = fuse.NewServer(rawFs, dir, &options.MountOptions)

	return fi, err
}
