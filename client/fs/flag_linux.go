/*
   file created by Junlin Chen in 2022

*/

package fs

import "syscall"

const (
	UnmountFlag = syscall.MNT_FORCE | syscall.MNT_DETACH
)
