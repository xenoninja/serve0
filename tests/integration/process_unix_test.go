//go:build !windows

package integration_test

import (
	"os"
	"os/exec"
)

func preparePreviewProcess(cmd *exec.Cmd) {}

func interruptPreviewProcess(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
