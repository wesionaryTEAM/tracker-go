//go:build !windows

package tracker

import (
	"os/exec"
	"strings"
)

// osVersion returns the OS kernel release string via `uname -r`.
// Returns "" if the command fails.
func osVersion() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
