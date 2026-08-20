//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// attachConsole re-attaches stdout/stderr to the parent console. The Windows
// binary is built with -H=windowsgui (double-click opens no console window),
// which detaches standard streams; when run from cmd/PowerShell with
// arguments we attach back so CLI output is visible.
func attachConsole() {
	const attachParentProcess = ^uintptr(0)
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if r, _, _ := kernel32.NewProc("AttachConsole").Call(attachParentProcess); r == 0 {
		return // no parent console (e.g. double-click with args): stay silent
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
}
