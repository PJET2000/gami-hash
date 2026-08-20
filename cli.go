package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/PJET2000/gami-hash/internal/engine"
	"github.com/PJET2000/gami-hash/internal/i18n"
	"github.com/PJET2000/gami-hash/internal/ui"
)

// Exit codes of the command-line mode.
const (
	exitOK        = 0   // completed, every file hashed
	exitFatal     = 1   // could not run (bad arguments, output not writable, ...)
	exitFileError = 2   // completed, but some files could not be read (see error log)
	exitCanceled  = 130 // interrupted (Ctrl-C); progress saved, run again to resume
)

func runCLI(args []string) int {
	fs := flag.NewFlagSet("gami-hash", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	root := fs.String("root", "", "folder to scan (required)")
	output := fs.String("output", "", "CSV file to write (required, must be outside -root)")
	workers := fs.Int("workers", workersFromEnv(), "parallel hashing workers (1-2 for spinning disks, more for SSDs)")
	fresh := fs.Bool("fresh", false, "start over instead of resuming an interrupted run")
	quiet := fs.Bool("quiet", false, "no progress output")
	version := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `gami-hash %s — hashes every file in a folder and writes one CSV manifest.

Usage:
  gami-hash -root FOLDER -output FILE.csv [-workers N] [-fresh] [-quiet]

Without arguments a graphical wizard starts instead.

The scanned folder is only ever read. Output columns:
  %s
An interrupted run (Ctrl-C, crash, power loss) is resumed automatically when
called again with the same -root and -output.

Exit codes: 0 done · 1 fatal error · 2 done but some files unreadable · 130 interrupted

Flags:
`, engine.Version, "relative_path,filename,size_bytes,sha256,mtime_utc")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitFatal
	}
	if *version {
		fmt.Println("gami-hash", engine.Version)
		return exitOK
	}
	if *root == "" || *output == "" {
		fs.Usage()
		return exitFatal
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	t := i18n.T{}
	eta := ui.NewETA()
	var lastLine int
	printProgress := func(p engine.Progress) {
		if *quiet {
			return
		}
		var line string
		if p.Phase == engine.PhaseScan {
			line = fmt.Sprintf("counting files … %s files, %s", t.FormatInt(p.FilesTotal), t.FormatSize(p.BytesTotal))
		} else {
			pct := 0.0
			if p.BytesTotal > 0 {
				pct = float64(p.BytesDone) / float64(p.BytesTotal) * 100
			}
			if pct > 100 {
				pct = 100
			}
			etaStr := "--"
			if secs, ok := eta.Update(p.BytesDone, p.BytesTotal); ok {
				etaStr = t.FormatETA(secs)
			}
			line = fmt.Sprintf("%5.1f%% · %s / %s files · %s / %s · ETA %s",
				pct, t.FormatInt(p.FilesDone), t.FormatInt(p.FilesTotal),
				t.FormatSize(p.BytesDone), t.FormatSize(p.BytesTotal), etaStr)
		}
		// Overwrite the previous line (pad so leftovers are erased).
		if pad := lastLine - len(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lastLine = len(line)
		fmt.Fprintf(os.Stderr, "\r%s", line)
	}

	// Throttle terminal updates to 2/s.
	last := time.Time{}
	progressFn := func(p engine.Progress) {
		if time.Since(last) < 500*time.Millisecond {
			return
		}
		last = time.Now()
		printProgress(p)
	}

	res, err := engine.Run(ctx, engine.Options{
		Root:    *root,
		Output:  *output,
		Workers: *workers,
		Fresh:   *fresh,
	}, progressFn)
	if !*quiet {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitFatal
	}

	if res.Canceled {
		fmt.Fprintf(os.Stderr, "interrupted — progress saved (%s files hashed this run).\n", t.FormatInt(res.FilesHashed))
		fmt.Fprintf(os.Stderr, "run the same command again to resume.\n")
		return exitCanceled
	}

	fmt.Fprintf(os.Stderr, "done: %s files (%s) recorded in %s\n",
		t.FormatInt(res.FilesHashed+res.FilesResumed), t.FormatSize(res.BytesTotal), res.Output)
	if res.FilesResumed > 0 {
		fmt.Fprintf(os.Stderr, "      %s of these were already recorded by a previous run\n", t.FormatInt(res.FilesResumed))
	}
	if res.Skipped > 0 {
		fmt.Fprintf(os.Stderr, "      %s non-regular entries skipped (symlinks etc.), see %s\n",
			t.FormatInt(res.Skipped), engine.ErrorLogPath(res.Output))
	}
	if res.FilesFailed > 0 {
		fmt.Fprintf(os.Stderr, "warning: %s files could not be read, see %s\n",
			t.FormatInt(res.FilesFailed), engine.ErrorLogPath(res.Output))
		return exitFileError
	}
	return exitOK
}
