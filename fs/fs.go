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
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/mc256/stargz-snapshotter/estargz"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	DebugTrace = true
)

type StarlightFsNode struct {
	//I is a pointer to fs.Inode
	fs.Inode

	// Ent is a pointer to delta.FsEntry via delta.TocEntry interface
	Ent *FsEntry
}

var _ = (fs.NodeLookuper)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"child":  name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("LOOKUP")
	}

	if child, hasChild := n.Ent.LookUp(name); hasChild {
		if inode, err := func() (*fs.Inode, syscall.Errno) {
			child.StateMu.Lock()
			defer child.StateMu.Unlock()
			if child.State == EnRwLayer {
				var st syscall.Stat_t
				if err := child.GetAttrFromFs(&st); err != 0 {
					return nil, err
				}
				out.FromStat(&st)
				return n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr()), 0
			} else {
				var attr fuse.Attr
				if err := child.GetAttrFromEntry(&attr); err != 0 {
					return nil, err
				}
				out.Attr = attr
				return n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr()), 0
			}
		}(); err == 0 {
			return inode, err
		} else if err != 0 {
			return nil, err
		}
	}

	return nil, syscall.ENOENT
}

var _ = (fs.NodeGetattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("GETATTR")
	}

	if n.Ent.AtomicGetFileState() == EnRwLayer {
		if fh != nil {
			return fh.(fs.FileGetattrer).Getattr(ctx, out)
		}

		var st syscall.Stat_t

		if err := n.Ent.GetAttrFromFs(&st); err != 0 {
			return err
		}
		out.FromStat(&st)

	} else {
		var attr fuse.Attr
		if err := n.Ent.GetAttrFromEntry(&attr); err != 0 {
			return err
		}
		out.Attr = attr
	}
	return 0
}

//var _ = (fs.NodeReader)((*StarlightFsNode)(nil))

var _ = (fs.NodeReaddirer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("READDIR")
	}

	children := n.Ent.GetChildren()
	cl := make([]fuse.DirEntry, 0, len(*children)+2)
	for key, ent := range *children {
		if strings.HasPrefix(key, ".wh.") {
			continue
		}
		func() {
			child := ent.(*FsEntry)
			child.StateMu.Lock()
			defer child.StateMu.Unlock()
			if child.State == EnRwLayer {
				var st syscall.Stat_t
				if err := child.GetAttrFromFs(&st); err != 0 {
					return
				}
				cl = append(cl, fuse.DirEntry{
					Mode: st.Mode,
					Name: key,
					Ino:  st.Ino,
				})
			} else {
				var attr fuse.Attr
				if err := child.GetAttrFromEntry(&attr); err != 0 {
					return
				}
				cl = append(cl, fuse.DirEntry{
					Mode: attr.Mode,
					Name: key,
					Ino:  attr.Ino,
				})
			}
		}()
	}

	// link to myself and parent
	// .
	attr := n.Ent.GetStableAttr()
	cl = append(cl, fuse.DirEntry{
		Mode: attr.Mode,
		Name: ".",
		Ino:  attr.Ino,
	})

	// ..
	attr = n.Ent.GetParent().GetStableAttr()
	cl = append(cl, fuse.DirEntry{
		Mode: attr.Mode,
		Name: "..",
		Ino:  attr.Ino,
	})

	return fs.NewListDirStream(cl), 0
}

var _ = (fs.NodeReadlinker)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("READLINK")
	}

	buf, err := syscall.ByteSliceFromString(n.Ent.LinkName)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return buf, 0
}

var _ = (fs.NodeLinker)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	if strings.HasPrefix(name, ".wh.") {
		return nil, syscall.EPERM
	}

	t := target.(*StarlightFsNode)

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"from":   name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
			"target": t.Ent.Name,
		}).Trace("LINK")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	var child *FsEntry
	var hasChild bool
	if child, hasChild = n.Ent.LookUp(name); hasChild {
		return nil, syscall.EEXIST
	}

	if err := n.promote(); err != 0 {
		return nil, err
	}

	child = NewFsEntry(n.Ent.fi, &TemplateEntry{
		Entry{
			TraceableEntry: &util.TraceableEntry{
				TOCEntry: &estargz.TOCEntry{
					Name: filepath.Join(n.Ent.Name, name),
					Type: "hardlink",
				},
			},
			parent: n.Ent,
			State:  EnRwLayer,
			ready:  nil,
		},
	})

	if err := syscall.Link(t.Ent.GetRwLayerPath(), child.GetRwLayerPath()); err != nil {
		return nil, fs.ToErrno(err)
	}

	// permission
	if err := n.preserveOwner(ctx, child.GetRwLayerPath()); err != nil {
		return nil, fs.ToErrno(err)
	}

	var stat syscall.Stat_t
	if err := syscall.Lstat(child.GetRwLayerPath(), &stat); err != nil {
		return nil, fs.ToErrno(err)
	}

	out.FromStat(&stat)

	child.stable.Ino = stat.Ino
	child.stable.Mode = stat.Mode

	ch := n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr())
	n.Ent.AddChild(name, child)
	return ch, 0
}

var _ = (fs.NodeUnlinker)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Unlink(ctx context.Context, name string) syscall.Errno {
	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("UNLINK")
	}

	// remove a child
	if child, hasChild := n.Ent.LookUp(name); hasChild {
		if err := func() (errno syscall.Errno) {
			child.StateMu.Lock()
			defer child.StateMu.Unlock()

			var err error
			if child.State == EnRwLayer {
				err = syscall.Unlink(child.GetRwLayerPath())
			} else if child.State == EnRoLayer {
				err = syscall.Unlink(child.GetRoLayerPath())
			}
			if err != nil {
				return fs.ToErrno(err)
			}

			n.Ent.RemoveChild(name)
			return 0
		}(); err != 0 {
			return err
		}
	} else {
		return syscall.ENOENT
	}
	return 0
}

var _ = (fs.NodeRmdirer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("RMDIR")
	}

	// remove a child
	if child, hasChild := n.Ent.LookUp(name); hasChild {
		if err := func() (errno syscall.Errno) {
			child.StateMu.Lock()
			defer child.StateMu.Unlock()

			var err error
			if child.State == EnRwLayer {
				err = syscall.Rmdir(child.GetRwLayerPath())
			} else if child.State == EnRoLayer {
				//err = syscall.Unlink(child.GetRoLayerPath())

			}
			if err != nil {
				return fs.ToErrno(err)
			}

			n.Ent.RemoveChild(name)
			return 0
		}(); err != 0 {
			return err
		}
	} else {
		return syscall.ENOENT
	}

	return 0
}

var _ = (fs.NodeOpener)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("OPEN-P")
	}

	if n.Ent.AtomicGetFileState() == EnEmpty {
		<-n.Ent.ready
		_ = n.Ent.AtomicSetFileState(EnEmpty, EnRoLayer)
	}

	isWrite := (flags&syscall.O_RDWR != 0) || (flags&syscall.O_WRONLY != 0)
	if isWrite {
		if fh, mode, err := func() (fs.FileHandle, uint32, syscall.Errno) {
			n.Ent.StateMu.Lock()
			defer n.Ent.StateMu.Unlock()
			if n.Ent.State == EnRoLayer {
				if errno := n.promote(); errno != 0 {
					return nil, 0, errno
				}
				if errno := n.setRWAttrFromEntry(); errno != 0 {
					return nil, 0, errno
				}
				n.Ent.State = EnRwLayer
			}
			return nil, 0, 0
		}(); err != 0 {
			return fh, mode, err
		}
	}
	p := n.Ent.AtomicGetRealPath()
	fd, err := syscall.Open(p, int(flags), 0)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("OPEN")
	}
	return fs.NewLoopbackFile(fd), 0, 0
}

func (n *StarlightFsNode) promote() syscall.Errno {
	if n.Ent.State == EnRwLayer {
		return 0
	}

	// Get Parents
	todos := make([]*FsEntry, 0)
	p := n.Ent.GetParent()
	for !p.IsRoot() {
		if p.AtomicGetFileState() == EnRwLayer {
			break
		}
		todos = append(todos, p)
		p = p.GetParent()
	}

	// Promote Parents
	for i := len(todos) - 1; i >= 0; i-- {
		t := todos[i]
		if err := func() syscall.Errno {
			t.StateMu.Lock()
			defer t.StateMu.Unlock()
			if t.State == EnRwLayer {
				return 0
			}
			if err := t.Promote(); err != 0 {
				return 0
			}
			t.State = EnRwLayer
			return 0
		}(); err != 0 {
			return err
		}
	}

	// Promote it self
	if err := n.Ent.Promote(); err != 0 {
		return err
	}

	return 0
}

func (n *StarlightFsNode) setRWAttrFromEntry() syscall.Errno {
	var attrIn fuse.SetAttrIn
	var attr fuse.Attr
	if errno := n.Ent.GetAttrFromEntry(&attr); errno != 0 {
		return errno
	}

	attrIn.Size = attr.Size
	attrIn.Mode = attr.Mode
	attrIn.Mtime = attr.Mtime
	attrIn.Atime = attr.Atime
	attrIn.Ctime = attr.Ctime
	attrIn.Owner = attr.Owner

	return n.setRWAttr(&attrIn)
}

func (n *StarlightFsNode) setRWAttr(in *fuse.SetAttrIn) syscall.Errno {
	p := n.Ent.GetRwLayerPath()
	// Mode
	if m, ok := in.GetMode(); ok {
		if err := syscall.Chmod(p, m); err != nil {
			return fs.ToErrno(err)
		}
	}

	// GID UID
	uid, uok := in.GetUID()
	gid, gok := in.GetGID()
	if uok || gok {
		suid := -1
		sgid := -1
		if uok {
			suid = int(uid)
		}
		if gok {
			sgid = int(gid)
		}
		if err := syscall.Lchown(p, suid, sgid); err != nil {
			return fs.ToErrno(err)
		}
	}

	// ATIME, MTIME
	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()

	if mok || aok {

		ap := &atime
		mp := &mtime
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}
		var ts [2]syscall.Timespec
		ts[0] = fuse.UtimeToTimespec(ap)
		ts[1] = fuse.UtimeToTimespec(mp)

		if err := syscall.UtimesNano(p, ts[:]); err != nil {
			return fs.ToErrno(err)
		}
	}

	// SIZE
	if sz, ok := in.GetSize(); ok {
		if err := syscall.Truncate(p, int64(sz)); err != nil {
			return fs.ToErrno(err)
		}
	}

	return 0
}

var _ = (fs.NodeSetattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("SETATTR")
	}

	if n.Ent.AtomicGetFileState() == EnEmpty {
		<-n.Ent.ready
		_ = n.Ent.AtomicSetFileState(EnEmpty, EnRoLayer)
	}

	if errno := func() syscall.Errno {
		n.Ent.StateMu.Lock()
		defer n.Ent.StateMu.Unlock()
		// Promote
		if n.Ent.State == EnRoLayer {
			if errno := n.promote(); errno != 0 {
				return errno
			}
			n.Ent.State = EnRwLayer
		}

		// Set attributes
		p := n.Ent.GetRwLayerPath()
		if fsa, ok := fh.(fs.FileSetattrer); ok && fsa != nil {
			_ = fsa.Setattr(ctx, in, out)
		} else {
			if errno := n.setRWAttr(in); errno != 0 {
				return errno
			}
		}

		// Get attribute
		if fga, ok := fh.(fs.FileGetattrer); ok && fga != nil {
			_ = fga.Getattr(ctx, out)
		} else {
			st := syscall.Stat_t{}
			err := syscall.Lstat(p, &st)
			if err != nil {
				return fs.ToErrno(err)
			}
			out.FromStat(&st)
		}
		return 0
	}(); errno != 0 {
		return errno
	}

	return 0
}

var _ = (fs.NodeSymlinker)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// should not be able to create whiteout file
	if strings.HasPrefix(name, ".wh.") {
		return nil, syscall.EPERM
	}

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"from":   name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
			"to":     target,
		}).Trace("SYMLINK")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	var child *FsEntry
	var hasChild bool
	if child, hasChild = n.Ent.LookUp(name); hasChild {
		return nil, syscall.EEXIST
	}

	if err := n.promote(); err != 0 {
		return nil, err
	}

	child = NewFsEntry(n.Ent.fi, &TemplateEntry{
		Entry{
			TraceableEntry: &util.TraceableEntry{
				TOCEntry: &estargz.TOCEntry{
					Name:     filepath.Join(n.Ent.Name, name),
					Type:     "symlink",
					LinkName: target,
				},
			},
			parent: n.Ent,
			State:  EnRwLayer,
			ready:  nil,
		},
	})

	if err := syscall.Symlink(target, child.GetRwLayerPath()); err != nil {
		return nil, fs.ToErrno(err)
	}

	// permission
	if err := n.preserveOwner(ctx, child.GetRwLayerPath()); err != nil {
		return nil, fs.ToErrno(err)
	}

	var stat syscall.Stat_t
	if err := syscall.Lstat(child.GetRwLayerPath(), &stat); err != nil {
		return nil, fs.ToErrno(err)
	}

	out.FromStat(&stat)

	child.stable.Ino = stat.Ino
	child.stable.Mode = stat.Mode

	ch := n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr())
	n.Ent.AddChild(name, child)
	return ch, 0
}

var _ = (fs.NodeMkdirer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// should not be able to create whiteout file
	if strings.HasPrefix(name, ".wh.") {
		return nil, syscall.EPERM
	}

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"from":   name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
		}).Trace("MKDIR")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	var child *FsEntry
	var hasChild bool
	if child, hasChild = n.Ent.LookUp(name); hasChild {
		return nil, syscall.EEXIST
	}

	if err := n.promote(); err != 0 {
		return nil, err
	}

	child = NewFsEntry(n.Ent.fi, &TemplateEntry{
		Entry{
			TraceableEntry: &util.TraceableEntry{
				TOCEntry: &estargz.TOCEntry{
					Name: filepath.Join(n.Ent.Name, name),
					Type: "dir",
				},
			},
			parent: n.Ent,
			State:  EnRwLayer,
			ready:  nil,
		},
	})

	// mkdir
	if err := syscall.Mkdir(child.GetRwLayerPath(), mode); err != nil {
		return nil, fs.ToErrno(err)
	}

	// permission
	if err := n.preserveOwner(ctx, child.GetRwLayerPath()); err != nil {
		return nil, fs.ToErrno(err)
	}

	// stat
	var stat syscall.Stat_t
	if err := syscall.Lstat(child.GetRwLayerPath(), &stat); err != nil {
		return nil, fs.ToErrno(err)
	}

	out.FromStat(&stat)

	child.stable.Ino = stat.Ino
	child.stable.Mode = stat.Mode
	child.parent = n.Ent

	ch := n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr())
	n.Ent.AddChild(name, child)
	return ch, 0
}

func (n *StarlightFsNode) preserveOwner(ctx context.Context, path string) error {
	if os.Getuid() != 0 {
		return nil
	}
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		return nil
	}
	return syscall.Lchown(path, int(caller.Uid), int(caller.Gid))
}

var _ = (fs.NodeCreater)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	// should not be able to create whiteout file
	if strings.HasPrefix(name, ".wh.") {
		return nil, nil, 0, syscall.EPERM
	}

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name":   n.Ent.Name,
			"child":  name,
			"source": n.Ent.Source,
			"state":  n.Ent.State,
			"mode":   fmt.Sprintf("%o", mode),
			"flags":  fmt.Sprintf("%o", flags),
		}).Trace("CREATE")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	var child *FsEntry
	var hasChild bool
	if child, hasChild = n.Ent.LookUp(name); hasChild {
		_ = n.Unlink(ctx, name)
	}

	if err := n.promote(); err != 0 {
		return nil, nil, 0, err
	}

	child = NewFsEntry(n.Ent.fi, &TemplateEntry{
		Entry{
			TraceableEntry: &util.TraceableEntry{
				TOCEntry: &estargz.TOCEntry{
					Name: filepath.Join(n.Ent.Name, name),
					Type: "reg",
				},
			},
			State: EnRwLayer,
			ready: nil,
		},
	})
	child.parent = n.Ent
	childRealPath := child.GetRwLayerPath()

	// Create File
	flags = flags &^ syscall.O_APPEND
	fd, err := syscall.Open(childRealPath, int(flags)|os.O_CREATE, mode)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	// Preserve Owner
	if err = n.preserveOwner(ctx, childRealPath); err != nil {
		_ = syscall.Unlink(childRealPath)
		return nil, nil, 0, fs.ToErrno(err)
	}

	// Get Stat
	var stat syscall.Stat_t
	err = syscall.Fstat(fd, &stat)
	if err != nil {
		_ = syscall.Close(fd)
		_ = syscall.Unlink(childRealPath)
		return nil, nil, 0, fs.ToErrno(err)
	}

	child.stable.Ino = stat.Ino
	child.stable.Mode = stat.Mode

	ch := n.NewInode(ctx, &StarlightFsNode{Ent: child}, *child.GetStableAttr())
	n.Ent.AddChild(name, child)

	return ch, fs.NewLoopbackFile(fd), 0, 0
}

var _ = (fs.NodeRenamer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	// should not be able to create whiteout file
	if strings.HasPrefix(newName, ".wh.") {
		return syscall.EPERM
	}

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"n":       n.Ent.Name,
			"name":    name,
			"newName": name,
			"source":  n.Ent.Source,
			"state":   n.Ent.State,
			"flags":   fmt.Sprintf("%o", flags),
		}).Trace("RENAME")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	target := newParent.(*StarlightFsNode)

	if n != target {
		target.Ent.StateMu.Lock()
		defer target.Ent.StateMu.Unlock()
	}

	var child *FsEntry
	var hasChild bool
	if child, hasChild = n.Ent.LookUp(name); hasChild {
		child.StateMu.Lock()
		defer child.StateMu.Unlock()

		//if _, hasTChild := target.Ent.LookUp(newName); hasTChild {
		//	return syscall.EPERM
		//}

		if n.Ent.State == EnRoLayer {
			if errno := n.promote(); errno != 0 {
				return syscall.EPERM
			}
			if errno := n.setRWAttrFromEntry(); errno != 0 {
				return syscall.EPERM
			}
			n.Ent.State = EnRwLayer
		}

		if target.Ent.State == EnRoLayer {
			if errno := target.promote(); errno != 0 {
				return syscall.EPERM
			}
			if errno := target.setRWAttrFromEntry(); errno != 0 {
				return syscall.EPERM
			}
			target.Ent.State = EnRwLayer
		}

		target.Ent.AddChild(newName, child)
		n.Ent.RemoveChild(name)

		oldAbsPath := child.GetRwLayerPath()
		child.Name = filepath.Join(target.Ent.Name, newName)
		newAbsPath := child.GetRwLayerPath()

		if err := syscall.Rename(oldAbsPath, newAbsPath); err != nil {
			return fs.ToErrno(err)
		}

		return 0
	} else {
		return syscall.ENOENT
	}
}

// XAttr

var _ = (fs.NodeGetxattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
			"attr": attr,
		}).Trace("GETXATTR")
	}

	if n.Ent.AtomicGetFileState() == EnRwLayer {
		sz, err := unix.Lgetxattr(n.Ent.GetRwLayerPath(), attr, dest)
		return uint32(sz), fs.ToErrno(err)
	}

	if val, hasVal := n.Ent.Xattrs[attr]; hasVal {
		dest = val
		return uint32(len(val)), 0
	}

	return 0, fs.ENOATTR
}

var _ = (fs.NodeListxattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
		}).Trace("LISTXATTR")
	}

	if n.Ent.AtomicGetFileState() == EnRwLayer {
		sz, err := unix.Llistxattr(n.Ent.GetRwLayerPath(), dest)
		return uint32(sz), fs.ToErrno(err)
	}

	kl := make([]string, len(n.Ent.Xattrs))
	for k, _ := range n.Ent.Xattrs {
		kl = append(kl, k)
	}
	res := strings.Join(kl, "\x00")
	dest = []byte(res)

	return uint32(len(res)), 0
}

var _ = (fs.NodeSetxattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
			"attr": attr,
			"val":  string(data),
		}).Trace("SETXATTR")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if errno := n.promote(); errno != 0 {
		return errno
	}

	err := unix.Lsetxattr(n.Ent.GetRwLayerPath(), attr, data, int(flags))
	return fs.ToErrno(err)
}

var _ = (fs.NodeRemovexattrer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Removexattr(ctx context.Context, attr string) syscall.Errno {

	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
			"attr": attr,
		}).Trace("REMOVEXATTR")
	}

	n.Ent.StateMu.Lock()
	defer n.Ent.StateMu.Unlock()

	if errno := n.promote(); errno != 0 {
		return errno
	}

	err := unix.Lremovexattr(n.Ent.GetRwLayerPath(), attr)
	return fs.ToErrno(err)
}

var _ = (fs.NodeFsyncer)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Fsync(ctx context.Context, f fs.FileHandle, flags uint32) syscall.Errno {
	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
		}).Trace("FSYNC")
	}

	if n.Ent.AtomicGetFileState() == EnEmpty {
		<-n.Ent.ready
		_ = n.Ent.AtomicSetFileState(EnEmpty, EnRoLayer)
	}

	p := n.Ent.AtomicGetRealPath()
	fd, err := syscall.Open(p, int(flags), 0)
	if err != nil {
		return fs.ToErrno(err)
	}
	f = fs.NewLoopbackFile(fd)

	return f.(fs.FileFsyncer).Fsync(ctx, flags)

}

var _ = (fs.NodeStatfser)((*StarlightFsNode)(nil))

func (n *StarlightFsNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
		}).Trace("STATFS-P")
	}
	s := syscall.Statfs_t{}
	err := syscall.Statfs(n.Ent.AtomicGetRealPath(), &s)
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStatfsT(&s)
	out.NameLen = 1<<32 - 1
	if DebugTrace {
		log.G(ctx).WithFields(logrus.Fields{
			"name": n.Ent.Name,
			"attr": out,
		}).Trace("STATFS")
	}
	return 0
}
