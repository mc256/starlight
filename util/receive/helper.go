/*
   file created by Junlin Chen in 2022

*/

package receive

import (
	"archive/tar"
	"os"
	"syscall"
)

// ---------------------------------------------------------------------
// Helper Methods

// modeOfEntry gets system's mode bits from TOCEntry
// From estargz
func modeOfEntry(e *ReferencedFile) uint32 {
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
