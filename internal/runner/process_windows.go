//go:build windows

package runner

import (
	"os"
	"os/exec"
	"strconv"
)

func configureProcessGroup(_ *exec.Cmd) {}

func killProcessGroup(process *os.Process) {
	if process != nil {
		_ = exec.Command("taskkill", "/pid", strconv.Itoa(process.Pid), "/t", "/f").Run()
	}
}
