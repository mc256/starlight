/*
   file created by Junlin Chen in 2022

*/

package receive

import (
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func (r *ReferencedFile) SetBlockSize(out *fuse.Attr) {
	out.Blksize = uint32(r.ChunkSize)
	if r.ChunkSize == 0 {
		out.Blksize = 4096
	}
	out.Blocks = out.Size / uint64(out.Blksize)
	if out.Size%uint64(out.Blksize) > 0 {
		out.Blocks++
	}

	out.Padding = 0
}

func Creat(path string, perm uint32) (int, error) {
	return syscall.Creat(path, perm)
}

func FileMode(m uint32) uint32 {
	return m
}
