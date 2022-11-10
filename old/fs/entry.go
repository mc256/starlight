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
	"archive/tar"
	"fmt"
	"github.com/mc256/starlight/util/common"
	"os"
	"path"
	"sync"
	"syscall"
	"unsafe"

	fuseFs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
)

const (
	EnEmpty   = int8(0)
	EnPartial = int8(1) // Planning not to implemented
	EnRoLayer = int8(2)
	EnRwLayer = int8(3)
)

//---------------------------------------------------------------------------------------
// TocEntry interface
// supports RootEntry and Entry
type TocEntry interface {
	GetInterface() TocEntry
	GetEntry() *Entry

	AddChild(base string, entry TocEntry)

	String() string
}

//---------------------------------------------------------------------------------------
// LandmarkEntry

type LandmarkEntry struct {
	image      string
	checkpoint string
	ready      chan bool
}

func (le *LandmarkEntry) GetInterface() TocEntry {
	panic("this is an landmark entry")
}

func (le *LandmarkEntry) GetEntry() *Entry {
	panic("this is an landmark entry")
}

func (le *LandmarkEntry) AddChild(base string, entry TocEntry) {
	panic("this is an landmark entry")
}

func (le *LandmarkEntry) String() string {
	return fmt.Sprintf("Landmark-%s-%s", le.image, le.checkpoint)
}

//---------------------------------------------------------------------------------------
// TemplateEntry
type TemplateEntry struct {
	Entry
}

func (t *TemplateEntry) recursiveDeepCopy(fi *Instance) *FsEntry {
	source := t.TraceableEntry.Source
	ent := NewFsEntry(fi, t)
	ent.Entry.SetSourceLayer(source)

	for base, child := range t.child {
		nc := child.(*TemplateEntry).recursiveDeepCopy(fi)
		nc.parent = ent
		ent.AddChild(base, nc)
	}

	return ent
}

func (t *TemplateEntry) DeepCopy(fi *Instance) *FsEntry {
	ent := &FsEntry{
		Entry: Entry{
			TraceableEntry: common.GetRootNode(),
			State:          EnRwLayer,
			ready:          t.ready,
		},
		fi:      fi,
		StateMu: sync.Mutex{},
	}
	ent.Source = 0
	ent.parent = ent
	ent.initStableAttr()

	for base, child := range t.child {
		nc := child.(*TemplateEntry).recursiveDeepCopy(fi)
		nc.parent = ent
		ent.AddChild(base, nc)
	}

	return ent
}

//---------------------------------------------------------------------------------------
// FsEntry
type FsEntry struct {
	Entry

	stable  fuseFs.StableAttr
	fi      *Instance
	StateMu sync.Mutex
}

func NewFsEntry(fi *Instance, t *TemplateEntry) *FsEntry {
	ent := &FsEntry{
		Entry: Entry{
			TraceableEntry: t.TraceableEntry.DeepCopy(),
			State:          t.State,
			ready:          t.ready,
		},
		fi:      fi,
		StateMu: sync.Mutex{},
	}
	ent.InitModTime()
	ent.initStableAttr()
	return ent
}

func (fe *FsEntry) IsRoot() bool { return fe.GetParent() == fe }

func (fe *FsEntry) initStableAttr() {
	fe.stable.Ino = uint64(uintptr(unsafe.Pointer(fe)))
	fe.stable.Gen = 0
	fe.stable.Mode = modeOfEntry(fe.GetEntry())
}

func (fe *FsEntry) LookUp(base string) (*FsEntry, bool) {
	if base == "." {
		return fe, true
	} else if base == ".." {
		return fe.parent.(*FsEntry), true
	} else if child, hasChild := fe.child[base]; hasChild {
		return child.(*FsEntry), true
	}
	return nil, false
}

func (fe *FsEntry) GetStableAttr() *fuseFs.StableAttr {
	return &fe.stable
}

// GetAttrFromEntry converts stargz's TOCEntry to go-fuse's Attr.
// From estargz
func (fe *FsEntry) GetAttrFromEntry(out *fuse.Attr) syscall.Errno {
	out.Ino = fe.stable.Ino
	out.Size = uint64(fe.Size)
	if fe.IsDir() {
		out.Size = 4096
	} else if fe.Type == "symlink" {
		out.Size = uint64(len(fe.LinkName))
	}
	fe.SetBlockSize(out)
	mtime := fe.ModTime()
	out.SetTimes(&mtime, &mtime, &mtime)
	out.Mode = fe.stable.Mode
	out.Owner = fuse.Owner{Uid: uint32(fe.UID), Gid: uint32(fe.GID)}
	out.Rdev = uint32(unix.Mkdev(uint32(fe.DevMajor), uint32(fe.DevMinor)))
	out.Nlink = uint32(fe.NumLink)
	if out.Nlink == 0 {
		out.Nlink = 1 // zero "NumLink" means one.
	}
	return 0
}

func (fe *FsEntry) GetAttrFromFs(st *syscall.Stat_t) syscall.Errno {
	if st == nil {
		st = &syscall.Stat_t{}
	}
	if err := syscall.Lstat(fe.GetRwLayerPath(), st); err != nil {
		return fuseFs.ToErrno(err)
	}
	return 0
}

func (fe *FsEntry) GetChildren() *map[string]TocEntry { return &fe.child }

func (fe *FsEntry) Children() *map[string]TocEntry { return &fe.child }

func (fe *FsEntry) RemoveChild(baseName string) {
	delete(fe.child, baseName)
}

func (fe *FsEntry) GetParent() *FsEntry {
	return (fe.parent).(*FsEntry)
}

func (fe *FsEntry) AtomicGetFileState() int8 {
	fe.StateMu.Lock()
	defer fe.StateMu.Unlock()
	s := fe.State
	return s
}

func (fe *FsEntry) AtomicSetFileState(from, to int8) bool {
	fe.StateMu.Lock()
	defer fe.StateMu.Unlock()
	if fe.State == from {
		fe.State = to
		return true
	}
	return false
}

// AtomicGetRealPath returns the absolute path in the real file system
// be aware that this method will lock StateMu
func (fe *FsEntry) AtomicGetRealPath() string {
	s := fe.AtomicGetFileState()
	switch s {
	case EnRwLayer:
		return path.Join(fe.fi.rwLayerPath, fe.Name)
	case EnRoLayer:
		layer := fe.GetSourceLayer() - 1
		if layer < 0 || layer > len(*fe.fi.layerLookupMap) {
			panic(common.ErrNoRoPath)
		}
		return path.Join((*fe.fi.layerLookupMap)[layer].GetAbsPath(), fe.Name)
	case EnEmpty:
		return ""
	default:
		panic(common.ErrLayerNotFound)
	}
}

// GetRwLayerPath returns the absolute path to the RW layer in the real file system
// this method will NOT lock StateMu
func (fe *FsEntry) GetRwLayerPath() string {
	return path.Join(fe.fi.rwLayerPath, fe.Name)
}

// GetRoLayerPath returns the absolute path to the RO layer in the real file system
// this method will NOT lock StateMu
func (fe *FsEntry) GetRoLayerPath() string {
	layer := fe.GetSourceLayer() - 1
	if layer < 0 || layer > len(*fe.fi.layerLookupMap) {
		panic(common.ErrNoRoPath)
	}
	return path.Join((*fe.fi.layerLookupMap)[layer].GetAbsPath(), fe.Name)
}

func (fe *FsEntry) GetFileMode() uint32 {
	return uint32(fe.Stat().Mode())
}

func (fe *FsEntry) promoteRegularFile() syscall.Errno {
	dest, err := Creat(fe.GetRwLayerPath(), fe.GetFileMode())

	if err != nil {
		return fuseFs.ToErrno(err)
	}

	src, err := syscall.Open(fe.GetRoLayerPath(), syscall.O_RDONLY, 0)
	defer syscall.Close(src)
	if err != nil {
		_ = syscall.Close(dest)
		return fuseFs.ToErrno(err)
	}

	var result syscall.Errno
	buffer := make([]byte, 1<<12)

	for {
		n, err := syscall.Read(src, buffer[:])
		if n == 0 {
			break
		}
		if err != nil {
			result = fuseFs.ToErrno(err)
			break
		}

		if _, err := syscall.Write(dest, buffer[:n]); err != nil {
			result = fuseFs.ToErrno(err)
			break
		}
	}

	if err := syscall.Close(dest); err != nil && result == 0 {
		result = fuseFs.ToErrno(err)
	}

	return result
}

func (fe *FsEntry) Promote() syscall.Errno {
	if fe.IsDir() {
		if err := syscall.Mkdir(fe.GetRwLayerPath(), fe.GetFileMode()); err != nil {
			if err == unix.EEXIST {
				if err = syscall.Chmod(fe.GetRwLayerPath(), fe.GetFileMode()); err != nil {
					return fuseFs.ToErrno(err)
				} else {
					return 0
				}
			}
			return fuseFs.ToErrno(err)
		}
	} else if fe.Type == "symlink" {
		if err := syscall.Symlink(fe.LinkName, fe.GetRwLayerPath()); err != nil {
			return fuseFs.ToErrno(err)
		}
	} else { // Regular file
		if err := fe.promoteRegularFile(); err != 0 {
			return err
		}
	}

	for xk, xv := range fe.Xattrs {
		if err := unix.Lsetxattr(fe.GetRwLayerPath(), xk, xv, 0); err != nil {
			return fuseFs.ToErrno(err)
		}
	}

	st := &syscall.Stat_t{}
	_ = fe.GetAttrFromFs(st)
	fe.stable.Ino = st.Ino
	fe.stable.Mode = FileMode(st.Mode)
	return 0
}

//---------------------------------------------------------------------------------------
// Entry
type Entry struct {
	*common.TraceableEntry

	parent TocEntry
	child  map[string]TocEntry

	State int8
	ready chan bool
}

func (en *Entry) AddChild(base string, entry TocEntry) {
	if en.child == nil {
		en.child = make(map[string]TocEntry)
	}
	en.child[base] = entry
}

func (en *Entry) GetInterface() TocEntry {
	return TocEntry(en)
}

func (en *Entry) GetEntry() *Entry {
	return en
}

func (en *Entry) String() string {
	return fmt.Sprintf("Entry [%d] - %s", en.State, en.Name)
}

// ---------------------------------------------------------------------
// Helper Methods

// modeOfEntry gets system's mode bits from TOCEntry
// From estargz
func modeOfEntry(e *Entry) uint32 {
	// Permission bits
	res := uint32(e.Stat().Mode() & os.ModePerm)

	// File type bits
	switch e.Stat().Mode() & os.ModeType {
	case os.ModeDevice:
		res |= syscall.S_IFBLK
	case os.ModeDevice | os.ModeCharDevice:
		res |= syscall.S_IFCHR
	case os.ModeDir:
		res |= syscall.S_IFDIR
	case os.ModeNamedPipe:
		res |= syscall.S_IFIFO
	case os.ModeSymlink:
		res |= syscall.S_IFLNK
	case os.ModeSocket:
		res |= syscall.S_IFSOCK
	default: // regular file.
		res |= syscall.S_IFREG
	}

	// SUID, SGID, Sticky bits
	// Stargz package doesn't provide these bits so let's calculate them manually
	// here. TOCEntry.Mode is a copy of tar.Header.Mode so we can understand the
	// mode using that package.
	// See also:
	// - https://github.com/google/crfs/blob/71d77da419c90be7b05d12e59945ac7a8c94a543/stargz/stargz.go#L706
	hm := (&tar.Header{Mode: e.Mode}).FileInfo().Mode()
	if hm&os.ModeSetuid != 0 {
		res |= syscall.S_ISUID
	}
	if hm&os.ModeSetgid != 0 {
		res |= syscall.S_ISGID
	}
	if hm&os.ModeSticky != 0 {
		res |= syscall.S_ISVTX
	}

	return res
}
