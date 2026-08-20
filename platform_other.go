//go:build !windows

package main

// attachConsole is only needed on Windows GUI-subsystem builds.
func attachConsole() {}
