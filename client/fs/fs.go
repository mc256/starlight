/*
   file created by Junlin Chen in 2022

*/

package fs

import (
	"context"
	"github.com/containerd/containerd/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	DebugTrace = false
)

type ReceivedFile interface {
	GetChildren() []ReceivedFile
	AppendChild(children ReceivedFile)
	IsReady() bool
	GetAttr(out *fuse.Attr) syscall.Errno
	GetXAttrs() map[string][]byte
	GetName() string
	GetStableAttr() *fs.StableAttr
	GetLinkName() string
	GetRealPath() string
	WaitForReady()
}

type StarlightFsNode struct {
	fs.Inode
	ReceivedFile
	instance *Instance
}

func (n *StarlightFsNode) getFile(p string) ReceivedFile {
	return n.instance.manager.LookUpFile(n.instance.stack, p)
}

func (n *StarlightFsNode) getRealPath() string {
	p := n.instance.manager.GetPathByLayer(n.instance.stack)
	pp := n.GetRealPath()
	return filepath.Join(p, pp)
}

func (n *StarlightFsNode) log(filename string, access, complete time.Time) {
	n.instance.manager.LogTrace(n.instance.stack, filename, access, complete)
}

var _ = (fs.NodeLookuper)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	f := n.getFile(filepath.Join(n.GetName(), name))
	if f == nil {
		return nil, syscall.ENOENT
	}

	var attr fuse.Attr
	if err := f.GetAttr(&attr); err != 0 {
		return nil, err
	}
	out.Attr = attr
	return n.NewInode(ctx, &StarlightFsNode{
		ReceivedFile: f,
		instance:     n.instance,
	}, *f.GetStableAttr()), 0
}

var _ = (fs.NodeGetattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	var attr fuse.Attr
	if err := n.GetAttr(&attr); err != 0 {
		return err
	}
	out.Attr = attr
	return 0
}

var _ = (fs.NodeGetxattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if val, hasVal := n.GetXAttrs()[attr]; hasVal {
		dest = val
		return uint32(len(val)), 0
	}

	return 0, fs.ENOATTR
}

var _ = (fs.NodeListxattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	xattrs := n.GetXAttrs()
	kl := make([]string, len(xattrs))
	for k := range xattrs {
		kl = append(kl, k)
	}
	res := strings.Join(kl, "\x00")
	dest = []byte(res)

	return uint32(len(res)), 0
}

var _ = (fs.NodeReaddirer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	children := n.GetChildren()
	cl := make([]fuse.DirEntry, 0, len(children)+2)
	for _, child := range children {
		var attr fuse.Attr
		if err := child.GetAttr(&attr); err != 0 {
			return nil, err
		}
		cl = append(cl, fuse.DirEntry{
			Mode: attr.Mode,
			Name: filepath.Base(child.GetName()),
			Ino:  attr.Ino,
		})
	}

	// link to myself and parent
	// .
	attr := n.GetStableAttr()
	cl = append(cl, fuse.DirEntry{
		Mode: attr.Mode,
		Name: ".",
		Ino:  attr.Ino,
	})

	// ..
	f := n.getFile(filepath.Join(n.GetName(), ".."))
	if f != nil {
		attr = f.GetStableAttr()
	}
	cl = append(cl, fuse.DirEntry{
		Mode: attr.Mode,
		Name: "..",
		Ino:  attr.Ino,
	})

	return fs.NewListDirStream(cl), 0
}

var _ = (fs.NodeReadlinker)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	buf, err := syscall.ByteSliceFromString(n.GetLinkName())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return buf, 0
}

var _ = (fs.NodeOpener)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	r := n.getRealPath()

	access := time.Now()
	if !n.IsReady() {
		n.WaitForReady()
	}
	complete := time.Now()
	name := n.GetName()
	n.log(name, access, complete)

	log.G(ctx).WithFields(logrus.Fields{
		"f":  name,
		"_s": n.instance.stack,
		"_r": r,
	}).Trace("open")

	fd, err := syscall.Open(r, int(flags), 0)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}
	return fs.NewLoopbackFile(fd), fuse.FOPEN_KEEP_CACHE, 0
}

var _ = (fs.NodeFsyncer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Fsync(ctx context.Context, f fs.FileHandle, flags uint32) syscall.Errno {
	r := n.getRealPath()
	fd, err := syscall.Open(r, int(flags), 0)
	if err != nil {
		return fs.ToErrno(err)
	}
	f = fs.NewLoopbackFile(fd)
	return f.(fs.FileFsyncer).Fsync(ctx, flags)

}
