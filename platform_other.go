//go:build !windows

package main

// attachConsole is only needed on Windows GUI-subsystem builds.
func attachConsole() {}

// osUILanguage: on Linux/macOS the language comes from LANG/LC_* instead.
func osUILanguage() string { return "" }
