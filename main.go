// gami-hash walks a folder tree, hashes every file (SHA-256) and writes one
// CSV manifest. Double-click (no arguments) starts the graphical wizard;
// any argument switches to command-line mode.
//
// The program is strictly read-only towards the scanned folder, needs no
// installation and no admin rights, and contains no networking code at all.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/PJET2000/gami-hash/internal/engine"
	"github.com/PJET2000/gami-hash/internal/i18n"
	"github.com/PJET2000/gami-hash/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		attachConsole() // no-op except on Windows GUI-subsystem builds
		os.Exit(runCLI(os.Args[1:]))
	}
	t := i18n.T{Lang: i18n.Detect(osUILanguage())}
	os.Exit(ui.Run(t, workersFromEnv(), time.Now().Format("2006-01-02")))
}

// workersFromEnv allows a manual override of the hashing parallelism without
// adding anything to the GUI: GAMI_HASH_WORKERS=4 gami-hash
// (more workers help on SSDs; on spinning disks the default of 2 is safer).
func workersFromEnv() int {
	if v := os.Getenv("GAMI_HASH_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 64 {
			return n
		}
		fmt.Fprintln(os.Stderr, "ignoring invalid GAMI_HASH_WORKERS value:", v)
	}
	return engine.DefaultWorkers
}
