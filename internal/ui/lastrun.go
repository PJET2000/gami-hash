package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/PJET2000/gami-hash/internal/engine"
)

// The GUI remembers an interrupted run in one small file in the user's
// config folder (~/.config/gami-hash on Linux, %AppData% on Windows,
// ~/Library/Application Support on macOS). On the next start the wizard can
// then offer to continue with a single click, without any folder or file
// picking. The file is removed when a run completes. It contains only the
// two paths the user chose, nothing else.
type lastRun struct {
	Root   string `json:"root"`
	Output string `json:"output"`
}

func lastRunPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gami-hash", "last-run.json")
}

// saveLastRun records the run about to start. Best effort: a failure here
// only costs the one-click resume convenience, never the run itself.
func saveLastRun(root, output string) {
	p := lastRunPath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	data, _ := json.MarshalIndent(lastRun{Root: root, Output: output}, "", "  ")
	os.WriteFile(p, data, 0o644)
}

func clearLastRun() {
	if p := lastRunPath(); p != "" {
		os.Remove(p)
	}
}

// resumableLastRun returns the recorded run if it is still genuinely
// resumable: the state file exists, the engine checkpoint agrees (same root,
// valid CSV) and the folder is reachable. Anything stale is cleaned up.
func resumableLastRun() (lastRun, engine.ResumeState, bool) {
	var lr lastRun
	p := lastRunPath()
	if p == "" {
		return lr, engine.ResumeState{}, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return lr, engine.ResumeState{}, false
	}
	if json.Unmarshal(data, &lr) != nil || lr.Root == "" || lr.Output == "" {
		clearLastRun()
		return lr, engine.ResumeState{}, false
	}
	st := engine.CheckResume(lr.Output)
	if !st.Resumable || !engine.SameRoot(st.Root, lr.Root) {
		clearLastRun()
		return lr, engine.ResumeState{}, false
	}
	if fi, err := os.Stat(lr.Root); err != nil || !fi.IsDir() {
		// Folder not reachable right now (drive unplugged?). Keep the state
		// file so the offer returns once the drive is back, but do not offer
		// a resume that would immediately fail.
		return lr, engine.ResumeState{}, false
	}
	return lr, st, true
}
