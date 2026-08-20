// Package ui implements the minimal graphical wizard: pick a folder, pick an
// output location, confirm, watch a progress bar, read the done message.
// It uses the operating system's native dialogs (Win32 on Windows, zenity on
// Linux, osascript on macOS) — no embedded UI toolkit, no admin rights.
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ncruces/zenity"

	"github.com/PJET2000/gami-hash/internal/engine"
	"github.com/PJET2000/gami-hash/internal/i18n"
)

// Run executes the wizard and returns a process exit code.
// today is the current date (YYYY-MM-DD) used for the default file name.
func Run(t i18n.T, workers int, today string) int {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && !zenity.IsAvailable() {
		fmt.Println(t.ZenityMissing())
		return 1
	}

	// Step 0: what this program does (and a chance to bail out).
	err := zenity.Question(t.WelcomeText(engine.Version),
		zenity.Title(t.AppTitle()),
		zenity.OKLabel(t.Next()),
		zenity.CancelLabel(t.Cancel()))
	if err != nil {
		return 0
	}

	// Step 1: folder to record.
	root, err := zenity.SelectFile(zenity.Directory(), zenity.Title(t.PickRootTitle()))
	if err != nil || root == "" {
		return 0
	}
	root = filepath.Clean(root)

	// Step 2: output location (with resume/overwrite handling).
	opts, ok := pickOutput(t, root, today)
	if !ok {
		return 0
	}
	opts.Workers = workers

	// Step 3: explicit start.
	err = zenity.Question(t.ConfirmStart(opts.Root, opts.Output),
		zenity.Title(t.AppTitle()),
		zenity.OKLabel(t.Start()),
		zenity.CancelLabel(t.Cancel()))
	if err != nil {
		return 0
	}

	// Step 4: progress.
	res, runErr := runWithProgress(t, opts)
	if runErr != nil {
		zenity.Error(t.FatalError(runErr), zenity.Title(t.AppTitle()))
		return 1
	}

	// Step 5: done message.
	if res.Canceled {
		zenity.Info(t.CanceledText(opts.Output), zenity.Title(t.AppTitle()))
		return 0
	}
	msg := t.DoneText(
		t.FormatInt(res.FilesHashed+res.FilesResumed),
		t.FormatSize(res.BytesTotal),
		res.Output,
		res.FilesResumed,
		t.FormatInt(res.FilesResumed))
	if res.FilesFailed > 0 {
		msg += t.DoneWithErrors(t.FormatInt(res.FilesFailed), engine.ErrorLogPath(res.Output))
	}
	// "Open folder" saves the inevitable "where is my file now?" question.
	err = zenity.Question(msg,
		zenity.Title(t.AppTitle()),
		zenity.OKLabel(t.OpenFolderBtn()),
		zenity.CancelLabel(t.CloseBtn()))
	if err == nil {
		revealInFileManager(res.Output)
	}
	return 0
}

// revealInFileManager opens the system file manager showing the result file.
// Purely local (no network); best effort — a failure is silently ignored.
func revealInFileManager(path string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("explorer", "/select,"+path).Start()
	case "darwin":
		exec.Command("open", "-R", path).Start()
	default:
		exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

// pickOutput loops until the user has chosen a valid output path and decided
// about resuming or overwriting. ok=false means the user cancelled.
func pickOutput(t i18n.T, root, today string) (engine.Options, bool) {
	defaultName := t.DefaultOutputName(filepath.Base(root), today)
	// Suggest the desktop: non-technical users must be able to find the
	// result again afterwards. (Never inside the scanned folder.)
	defaultPath := filepath.Join(defaultSaveDir(root), defaultName)

	for {
		output, err := zenity.SelectFileSave(
			zenity.Title(t.PickOutputTitle()),
			zenity.Filename(defaultPath),
			zenity.FileFilter{Name: "CSV", Patterns: []string{"*.csv"}, CaseFold: true})
		if err != nil || output == "" {
			return engine.Options{}, false
		}
		if filepath.Ext(output) == "" {
			output += ".csv"
		}
		output = filepath.Clean(output)

		// Hard rule: never write into the scanned tree.
		if within(output, root) {
			zenity.Warning(t.OutputInsideRoot(), zenity.Title(t.AppTitle()))
			continue
		}

		opts := engine.Options{Root: root, Output: output}
		st := engine.CheckResume(output)
		switch {
		case st.Resumable && engine.SameRoot(st.Root, root):
			err := zenity.Question(t.ResumeFound(t.FormatInt(st.RowCount), today),
				zenity.Title(t.AppTitle()),
				zenity.OKLabel(t.ResumeBtn()),
				zenity.ExtraButton(t.RestartBtn()),
				zenity.CancelLabel(t.Cancel()))
			switch {
			case err == nil:
				opts.Fresh = false
			case errors.Is(err, zenity.ErrExtraButton):
				opts.Fresh = true
			default:
				continue // back to the save dialog
			}
			return opts, true

		case st.Resumable:
			// Interrupted run recorded under a different root path. Often
			// this is the *same* folder — external drives change drive
			// letters between sessions — so offer to continue.
			err := zenity.Question(t.ResumeDifferentRoot(st.Root),
				zenity.Title(t.AppTitle()),
				zenity.OKLabel(t.ResumeBtn()),
				zenity.ExtraButton(t.RestartBtn()),
				zenity.CancelLabel(t.Cancel()))
			switch {
			case err == nil:
				opts.AllowRootMismatch = true
			case errors.Is(err, zenity.ErrExtraButton):
				opts.Fresh = true
			default:
				continue
			}
			return opts, true

		case exists(output):
			err := zenity.Question(t.OverwriteExisting(),
				zenity.Title(t.AppTitle()),
				zenity.OKLabel(t.OverwriteBtn()),
				zenity.CancelLabel(t.Cancel()))
			if err != nil {
				continue
			}
			opts.Fresh = true
			return opts, true

		default:
			opts.Fresh = true
			return opts, true
		}
	}
}

// runWithProgress runs the engine while driving the progress dialog.
// The dialog's cancel button stops the run gracefully (progress is kept).
func runWithProgress(t i18n.T, opts engine.Options) (engine.Result, error) {
	dlg, err := zenity.Progress(
		zenity.Title(t.ProgressTitle()),
		zenity.MaxValue(1000))
	if err != nil {
		return engine.Result{}, fmt.Errorf("could not open progress window: %w", err)
	}
	defer dlg.Close()
	dlg.Text(t.Scanning("", ""))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var latest engine.Progress
	progressFn := func(p engine.Progress) {
		mu.Lock()
		latest = p
		mu.Unlock()
	}

	type outcome struct {
		res engine.Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := engine.Run(ctx, opts, progressFn)
		done <- outcome{res, err}
	}()

	eta := NewETA()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-dlg.Done(): // user pressed cancel (or closed the window)
			cancel()
			out := <-done
			return out.res, out.err

		case out := <-done:
			if out.err == nil && !out.res.Canceled {
				dlg.Value(999)
				dlg.Complete()
				// Let the bar visibly reach 100% before the window closes.
				time.Sleep(300 * time.Millisecond)
			}
			return out.res, out.err

		case <-tick.C:
			mu.Lock()
			p := latest
			mu.Unlock()
			if p.Phase == engine.PhaseScan {
				files, size := "", ""
				if p.FilesTotal > 0 {
					files, size = t.FormatInt(p.FilesTotal), t.FormatSize(p.BytesTotal)
				}
				dlg.Text(t.Scanning(files, size))
				continue
			}
			val := 0
			if p.BytesTotal > 0 {
				val = int(float64(p.BytesDone) / float64(p.BytesTotal) * 1000)
			} else if p.FilesTotal > 0 {
				val = int(float64(p.FilesDone) / float64(p.FilesTotal) * 1000)
			}
			if val > 999 {
				val = 999 // 100% only when truly complete
			}
			dlg.Value(val)
			etaStr := ""
			if secs, ok := eta.Update(p.BytesDone, p.BytesTotal); ok {
				etaStr = t.FormatETA(secs)
			}
			dlg.Text(t.HashProgress(
				t.FormatInt(min64(p.FilesDone, p.FilesTotal)),
				t.FormatInt(p.FilesTotal),
				t.FormatSize(min64(p.BytesDone, p.BytesTotal)),
				t.FormatSize(p.BytesTotal),
				etaStr))
		}
	}
}

func within(path, dir string) bool {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		return engine.SameRoot(path, dir) ||
			len(path) > len(dir) && engine.SameRoot(path[:len(dir)], dir) && path[len(dir)] == filepath.Separator
	}
	return path == dir || (len(path) > len(dir) && path[:len(dir)] == dir && path[len(dir)] == filepath.Separator)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// defaultSaveDir picks where the save dialog starts: the user's desktop if it
// can be found, then documents, then the home folder, and as a last resort
// the parent of the scanned folder.
func defaultSaveDir(root string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Dir(root)
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		if d := xdgUserDir(home, "XDG_DESKTOP_DIR"); d != "" {
			return d
		}
	}
	for _, name := range []string{"Desktop", "Schreibtisch", "Documents", "Dokumente"} {
		d := filepath.Join(home, name)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return home
}

// xdgUserDir reads one entry from ~/.config/user-dirs.dirs (Linux desktops
// localize folder names, e.g. "Schreibtisch" on German systems).
func xdgUserDir(home, key string) string {
	data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+"=") {
			continue
		}
		val := strings.Trim(strings.TrimPrefix(line, key+"="), `"`)
		val = strings.ReplaceAll(val, "$HOME", home)
		if fi, err := os.Stat(val); err == nil && fi.IsDir() {
			return val
		}
	}
	return ""
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
