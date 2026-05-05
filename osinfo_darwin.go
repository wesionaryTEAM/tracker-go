//go:build darwin

package tracker

import (
	"syscall"
	"unsafe"
)

// osVersion returns the OS kernel release string on macOS.
// Returns "" if the syscall fails.
func osVersion() string {
	type utsname struct {
		sysname  [256]byte
		nodename [256]byte
		release  [256]byte
		version  [256]byte
		machine  [256]byte
	}

	var un utsname
	_, _, err := syscall.RawSyscall(405, uintptr(unsafe.Pointer(&un)), 0, 0) // 405 is SYS_UNAME on Darwin
	if err != 0 {
		return ""
	}

	b := make([]byte, 0, len(un.release))
	for _, v := range un.release {
		if v == 0 {
			break
		}
		b = append(b, v)
	}
	return string(b)
}
