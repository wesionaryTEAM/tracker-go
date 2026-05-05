//go:build !windows && !darwin

package tracker

import "syscall"

// osVersion returns the OS kernel release string on Linux and macOS.
// Returns "" if syscall.Uname fails.
func osVersion() string {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return ""
	}
	b := make([]byte, 0, len(uname.Release))
	for _, v := range uname.Release {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}
