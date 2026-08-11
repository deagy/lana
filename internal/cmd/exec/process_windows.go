//go:build !unix

package exec

import "os/exec"

// Some platforms have no portable process-group equivalent available in the
// standard library. Kill the direct process and report no sandbox guarantee.
func configureProcessGroup(cmd *exec.Cmd) {}
func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
