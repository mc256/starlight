/*
   file created by Junlin Chen in 2022

*/

package receive

import (
	"github.com/hanwen/go-fuse/v2/fuse"
	"syscall"
)

func (r *ReferencedFile) SetBlockSize(out *fuse.Attr) {
	out.Blocks = out.Size / uint64(4096)
	if out.Size%uint64(4096) > 0 {
		out.Blocks++
	}
}

func Creat(path string, perm uint32) (int, error) {
	return syscall.Open(path, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC, perm)
}

func FileMode(m uint16) uint32 {
	return uint32(m)
}
