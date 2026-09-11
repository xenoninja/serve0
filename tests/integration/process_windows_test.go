package integration_test

import (
	"os"
	"os/exec"
	"syscall"
)

func preparePreviewProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func interruptPreviewProcess(process *os.Process) error {
	// Windows cannot implement Process.Signal(os.Interrupt). A console break
	// targets the child's process group and is delivered as os.Interrupt by Go.
	generate := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	ok, _, err := generate.Call(syscall.CTRL_BREAK_EVENT, uintptr(process.Pid))
	if ok == 0 {
		return err
	}
	return nil
}
