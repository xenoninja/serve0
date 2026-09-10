//go:build !windows

package main_test

import (
	"os"
	"os/exec"
)

func preparePreviewProcess(cmd *exec.Cmd) {}

func interruptPreviewProcess(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
