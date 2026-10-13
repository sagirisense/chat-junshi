//go:build !windows

package codexclient

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
