//go:build !windows

package main

import "os/exec"

func hideEngineConsole(cmd *exec.Cmd) {}
