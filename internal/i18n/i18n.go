// Package i18n holds every user-visible text of the GUI in one place.
//
// The tool speaks plain English everywhere. Tone: calm, no jargon. The
// people reading these dialogs are archivists and historians, not IT staff.
// Every text about interruptions makes the same point first: nothing is lost.
package i18n

import (
	"fmt"
	"strings"
)

// T is the text catalogue. It stays a struct so a translation could return
// one day without touching the call sites.
type T struct{}

func (t T) AppTitle() string { return "GAMI Hashing Tool" }

func (t T) WelcomeText(version string) string {
	return "Welcome! This program creates a checksum list of your files for handover to GAMI.\n\n" +
		"You cannot break anything:\n" +
		"•  Your files are only read. Nothing is changed, moved or deleted.\n" +
		"•  The program does not use the internet. Nothing leaves your computer.\n" +
		"•  You can stop at any time and continue later.\n\n" +
		"There are three steps: choose a folder, choose where to save, start.\n\n" +
		"(Version " + version + ")"
}

func (t T) Next() string   { return "Next" }
func (t T) Cancel() string { return "Cancel" }
func (t T) Start() string  { return "Start" }

// Buttons of the resume prompts.
func (t T) ResumeBtn() string  { return "Continue" }
func (t T) NewRunBtn() string  { return "Start a new run" }
func (t T) RestartBtn() string { return "Start over" }
func (t T) QuitBtn() string    { return "Quit" }

// ResumeLastRun is shown right at startup when the previous run was
// interrupted. One click continues it; no folder or file picking needed.
func (t T) ResumeLastRun(root, output, rows string) string {
	return "Your last run was interrupted. That is fine, nothing was lost.\n\n" +
		"Folder: " + root + "\n" +
		"Already recorded: " + rows + " files\n" +
		"Result file: " + output + "\n\n" +
		"Choose \"Continue\" and the program picks up exactly where it stopped."
}

func (t T) PickRootTitle() string {
	return "Choose the folder to record"
}
func (t T) PickOutputTitle() string {
	return "Choose where to save the result file"
}
func (t T) DefaultOutputName(rootBase, date string) string {
	return "checksums_" + sanitizeFileName(rootBase) + "_" + date + ".csv"
}

func (t T) OutputInsideRoot() string {
	return "The result file must not be inside the folder that is being recorded. That way the folder is guaranteed to stay untouched.\n\n" +
		"Please choose a different location, for example the desktop."
}

// ResumeFound is shown when the user manually picked a result file that
// belongs to an interrupted run. The system's own save dialog may already
// have asked about replacing the file, so the first line calms that down.
func (t T) ResumeFound(rowCount string) string {
	return "This file belongs to an interrupted run. " + rowCount + " files are already recorded and will be kept.\n\n" +
		"\"Continue\" picks up where it stopped.\n" +
		"\"Start over\" clears the list and records everything again."
}

func (t T) ResumeDifferentRoot(oldRoot string) string {
	return "This result file belongs to an interrupted run. Back then the path was:\n\n" + oldRoot + "\n\n" +
		"If this is the same folder, for example an external drive that now has a different drive letter, choose \"Continue\".\n\n" +
		"If it is a different folder, choose \"Start over\". The file will then be created fresh."
}
func (t T) OverwriteBtn() string { return "Overwrite" }

func (t T) OverwriteExisting() string {
	return "The chosen file already exists and will be overwritten. Continue?"
}

func (t T) ConfirmStart(root, output string) string {
	return "Ready to start.\n\n" +
		"Folder: " + root + "\n" +
		"Result file: " + output + "\n\n" +
		"Depending on the amount of data this can take several hours. You can keep using the computer normally. If it goes to sleep, the run simply continues afterwards. Please keep external drives connected until it finishes.\n\n" +
		"You can stop at any time with \"Cancel\" in the progress window. Your progress is kept."
}

func (t T) ProgressTitle() string { return t.AppTitle() }
func (t T) Scanning(files, size string) string {
	if files == "" {
		return "Counting files …"
	}
	return "Counting files … " + files + " files, " + size
}
func (t T) HashProgress(filesDone, filesTotal, doneSize, totalSize, eta string) string {
	line := filesDone + " / " + filesTotal + " files  ·  " + doneSize + " / " + totalSize
	if eta != "" {
		line += "  ·  time left " + eta
	}
	return line
}

func (t T) CanceledText(output string) string {
	return "The run was stopped. That is fine: your progress is saved, nothing was lost.\n\n" +
		"The next time you start the program, it will ask right away whether you want to continue. One click on \"Continue\" is all it takes."
}

func (t T) DoneText(files, size, output string, resumed int64, resumedStr string) string {
	msg := "Done. " + files + " files (" + size + ") were recorded.\n\n" +
		"Result file:\n" + output
	if resumed > 0 {
		msg += "\n\nOf these, " + resumedStr + " files had already been recorded in the previous run."
	}
	msg += "\n\nPlease send the result file to GAMI as agreed."
	return msg
}

func (t T) DoneWithErrors(failed, errorLog string) string {
	return "\n\nNote: " + failed + " files could not be read and are missing from the list. Details are in the error log:\n" + errorLog + "\nJust send the log along."
}

func (t T) FatalError(err error) string {
	return "The program could not continue:\n\n" + err.Error()
}

func (t T) ZenityMissing() string {
	return "The graphical dialogs require the 'zenity' program, which was not found on this system.\n" +
		"Install it (e.g. 'sudo dnf install zenity' or 'sudo apt install zenity') or use the command line:\n" +
		"  gami-hash -root FOLDER -output FILE.csv"
}

func (t T) OpenFolderBtn() string { return "Open folder" }
func (t T) CloseBtn() string      { return "Close" }

// Sizes and numbers ---------------------------------------------------------

// FormatSize renders a byte count with decimal units.
func (t T) FormatSize(b int64) string {
	const (
		kb = 1000.0
		mb = kb * 1000
		gb = mb * 1000
		tb = gb * 1000
	)
	f := float64(b)
	var v float64
	var unit string
	switch {
	case f >= tb:
		v, unit = f/tb, "TB"
	case f >= gb:
		v, unit = f/gb, "GB"
	case f >= mb:
		v, unit = f/mb, "MB"
	case f >= kb:
		v, unit = f/kb, "kB"
	default:
		return fmt.Sprintf("%d bytes", b)
	}
	return fmt.Sprintf("%.1f %s", v, unit)
}

// FormatInt renders an integer with thousands separators (1,234,567).
func (t T) FormatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	var out []string
	for len(s) > 3 {
		out = append([]string{s[len(s)-3:]}, out...)
		s = s[:len(s)-3]
	}
	out = append([]string{s}, out...)
	return strings.Join(out, ",")
}

// FormatETA renders a duration in rough, friendly units.
func (t T) FormatETA(seconds float64) string {
	switch {
	case seconds < 60:
		return "under 1 minute"
	case seconds < 3600:
		return fmt.Sprintf("~%d min", int(seconds/60)+1)
	default:
		h := int(seconds / 3600)
		m := int(seconds/60) % 60
		if m > 0 {
			return fmt.Sprintf("~%d h %d min", h, m)
		}
		return fmt.Sprintf("~%d h", h)
	}
}

func sanitizeFileName(s string) string {
	// Keep the default output filename safe on all systems.
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", " ", "_")
	out := repl.Replace(s)
	if out == "" {
		out = "folder"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
