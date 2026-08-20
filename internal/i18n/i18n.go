// Package i18n holds the user-visible texts of the GUI in German and English.
// The language is picked from the operating system; German systems (the
// pilot institutions) see German, everything else sees English.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

type Lang string

const (
	DE Lang = "de"
	EN Lang = "en"
)

// Detect picks the UI language from the environment. On Windows the caller
// passes the result of the Win32 UI-language lookup via override (empty on
// other systems or if the lookup failed).
func Detect(override string) Lang {
	candidates := []string{override, os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG")}
	for _, c := range candidates {
		c = strings.ToLower(c)
		if strings.HasPrefix(c, "de") {
			return DE
		}
		if c != "" {
			return EN
		}
	}
	return EN
}

type T struct{ Lang Lang }

func (t T) s(de, en string) string {
	if t.Lang == DE {
		return de
	}
	return en
}

func (t T) AppTitle() string { return "GAMI Hashing Tool" }

func (t T) WelcomeText(version string) string {
	return t.s(
		"Willkommen! Dieses Programm erstellt eine Prüfsummen-Liste Ihrer Dateien für die Übergabe an GAMI.\n\n"+
			"Sie können dabei nichts kaputt machen:\n"+
			"•  Ihre Dateien werden nur gelesen — nie verändert, verschoben oder gelöscht.\n"+
			"•  Das Programm nutzt kein Internet. Es verlässt nichts Ihren Rechner.\n"+
			"•  Sie können jederzeit unterbrechen und später weitermachen.\n\n"+
			"Drei Schritte: Ordner auswählen → Speicherort für die Ergebnisdatei wählen → Start.\n\n"+
			"(Version "+version+")",
		"Welcome! This program creates a checksum list of your files for handover to GAMI.\n\n"+
			"You cannot break anything:\n"+
			"•  Your files are only read — never modified, moved or deleted.\n"+
			"•  The program does not use the internet. Nothing leaves your computer.\n"+
			"•  You can stop at any time and continue later.\n\n"+
			"Three steps: choose a folder → choose where to save the result → start.\n\n"+
			"(Version "+version+")")
}

func (t T) Next() string   { return t.s("Weiter", "Next") }
func (t T) Cancel() string { return t.s("Abbrechen", "Cancel") }
func (t T) Start() string  { return t.s("Starten", "Start") }

func (t T) PickRootTitle() string {
	return t.s("Zu erfassenden Ordner wählen", "Choose the folder to record")
}
func (t T) PickOutputTitle() string {
	return t.s("Speicherort für die Ergebnisdatei wählen", "Choose where to save the result file")
}
func (t T) DefaultOutputName(rootBase, date string) string {
	name := sanitizeFileName(rootBase)
	if t.Lang == DE {
		return "pruefsummen_" + name + "_" + date + ".csv"
	}
	return "checksums_" + name + "_" + date + ".csv"
}

func (t T) OutputInsideRoot() string {
	return t.s(
		"Die Ergebnisdatei darf nicht innerhalb des zu erfassenden Ordners gespeichert werden, damit dieser unverändert bleibt.\n\nBitte wählen Sie einen anderen Speicherort (z. B. den Schreibtisch).",
		"The result file must not be saved inside the folder being recorded, so that folder stays untouched.\n\nPlease choose a different location (for example the desktop).")
}

func (t T) ResumeFound(rowCount, date string) string {
	return t.s(
		"Für diese Ergebnisdatei wurde ein unterbrochener Durchlauf gefunden ("+rowCount+" Dateien bereits erfasst).\n\nMöchten Sie fortsetzen? Bereits erfasste Dateien werden dabei übersprungen.",
		"An interrupted run was found for this result file ("+rowCount+" files already recorded).\n\nDo you want to continue? Files already recorded will be skipped.")
}
func (t T) ResumeBtn() string  { return t.s("Fortsetzen", "Continue") }
func (t T) RestartBtn() string { return t.s("Von vorn beginnen", "Start over") }

func (t T) ResumeDifferentRoot(oldRoot string) string {
	return t.s(
		"Diese Ergebnisdatei gehört zu einem unterbrochenen Durchlauf, damals unter dem Pfad:\n\n"+oldRoot+"\n\n"+
			"Wenn das derselbe Ordner ist — zum Beispiel eine externe Festplatte, die jetzt einen anderen Laufwerksbuchstaben hat — wählen Sie „Fortsetzen“.\n\n"+
			"Wenn es ein anderer Ordner ist, wählen Sie „Von vorn beginnen“ (die Datei wird dann überschrieben).",
		"This result file belongs to an interrupted run, recorded back then under the path:\n\n"+oldRoot+"\n\n"+
			"If this is the same folder — for example an external drive that now has a different drive letter — choose “Continue”.\n\n"+
			"If it is a different folder, choose “Start over” (the file will then be overwritten).")
}
func (t T) OverwriteBtn() string { return t.s("Überschreiben", "Overwrite") }

func (t T) OverwriteExisting() string {
	return t.s(
		"Die gewählte Datei existiert bereits und wird überschrieben. Fortfahren?",
		"The chosen file already exists and will be overwritten. Continue?")
}

func (t T) ConfirmStart(root, output string) string {
	return t.s(
		"Bereit zum Start.\n\nZu erfassender Ordner:\n"+root+"\n\nErgebnisdatei:\n"+output+"\n\n"+
			"Je nach Datenmenge kann dies mehrere Stunden dauern. Währenddessen können Sie normal am Computer weiterarbeiten. "+
			"Falls der Rechner zwischendurch in den Ruhezustand geht, läuft die Erfassung danach einfach weiter. "+
			"Bitte lassen Sie externe Festplatten so lange angeschlossen.\n\n"+
			"Sie können jederzeit unterbrechen und später fortsetzen.",
		"Ready to start.\n\nFolder to record:\n"+root+"\n\nResult file:\n"+output+"\n\n"+
			"Depending on the amount of data this can take several hours. You can keep using the computer normally in the meantime. "+
			"If the computer goes to sleep, the run simply continues afterwards. "+
			"Please keep external drives connected until it finishes.\n\n"+
			"You can stop at any time and continue later.")
}

func (t T) ProgressTitle() string { return t.AppTitle() }
func (t T) Scanning(files, size string) string {
	if files == "" {
		return t.s("Zähle Dateien …", "Counting files …")
	}
	return t.s("Zähle Dateien … "+files+" Dateien, "+size,
		"Counting files … "+files+" files, "+size)
}
func (t T) HashProgress(filesDone, filesTotal, doneSize, totalSize, eta string) string {
	line := filesDone + " / " + filesTotal + " " + t.s("Dateien", "files") + "  ·  " + doneSize + " / " + totalSize
	if eta != "" {
		line += "  ·  " + t.s("Restzeit", "time left") + " " + eta
	}
	return line
}

func (t T) CanceledText(output string) string {
	return t.s(
		"Der Durchlauf wurde angehalten. Der bisherige Fortschritt ist gespeichert.\n\nUm fortzusetzen, starten Sie das Programm erneut und wählen Sie denselben Ordner und dieselbe Ergebnisdatei:\n"+output,
		"The run was stopped. Progress so far has been saved.\n\nTo continue, start the program again and choose the same folder and the same result file:\n"+output)
}

func (t T) DoneText(files, size, output string, resumed int64, resumedStr string) string {
	msg := t.s(
		"Fertig. "+files+" Dateien ("+size+") wurden erfasst.\n\nErgebnisdatei:\n"+output,
		"Done. "+files+" files ("+size+") were recorded.\n\nResult file:\n"+output)
	if resumed > 0 {
		msg += t.s("\n\nDavon waren "+resumedStr+" Dateien bereits aus dem vorherigen Durchlauf erfasst.",
			"\n\nOf these, "+resumedStr+" files had already been recorded in the previous run.")
	}
	msg += t.s("\n\nBitte übermitteln Sie die Ergebnisdatei wie mit GAMI vereinbart.",
		"\n\nPlease deliver the result file to GAMI as agreed.")
	return msg
}

func (t T) OpenFolderBtn() string { return t.s("Ordner öffnen", "Open folder") }
func (t T) CloseBtn() string      { return t.s("Schließen", "Close") }

func (t T) DoneWithErrors(failed, errorLog string) string {
	return t.s(
		"\n\nHinweis: "+failed+" Dateien konnten nicht gelesen werden und fehlen in der Liste. Einzelheiten stehen im Fehlerprotokoll:\n"+errorLog,
		"\n\nNote: "+failed+" files could not be read and are missing from the list. Details are in the error log:\n"+errorLog)
}

func (t T) FatalError(err error) string {
	return t.s("Das Programm konnte nicht fortfahren:\n\n", "The program could not continue:\n\n") + err.Error()
}

func (t T) ZenityMissing() string {
	return "The graphical dialogs require the 'zenity' program, which was not found on this system.\n" +
		"Install it (e.g. 'sudo dnf install zenity' or 'sudo apt install zenity') or use the command line:\n" +
		"  gami-hash -root FOLDER -output FILE.csv"
}

// Sizes and numbers ---------------------------------------------------------

// FormatSize renders a byte count with decimal units, localized decimals.
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
		return fmt.Sprintf("%d %s", b, t.s("Byte", "bytes"))
	}
	s := fmt.Sprintf("%.1f", v)
	if t.Lang == DE {
		s = strings.ReplaceAll(s, ".", ",")
	}
	return s + " " + unit
}

// FormatInt renders an integer with thousands separators (1.234.567 / 1,234,567).
func (t T) FormatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	sep := ","
	if t.Lang == DE {
		sep = "."
	}
	var out []string
	for len(s) > 3 {
		out = append([]string{s[len(s)-3:]}, out...)
		s = s[:len(s)-3]
	}
	out = append([]string{s}, out...)
	return strings.Join(out, sep)
}

// FormatETA renders a duration in rough, friendly units.
func (t T) FormatETA(seconds float64) string {
	switch {
	case seconds < 60:
		return t.s("unter 1 Minute", "under 1 minute")
	case seconds < 3600:
		m := int(seconds/60) + 1
		return fmt.Sprintf("~%d %s", m, t.s("Min.", "min"))
	default:
		h := int(seconds / 3600)
		m := int(seconds/60) % 60
		if m > 0 {
			return fmt.Sprintf("~%d %s %d %s", h, t.s("Std.", "h"), m, t.s("Min.", "min"))
		}
		return fmt.Sprintf("~%d %s", h, t.s("Std.", "h"))
	}
}

func sanitizeFileName(s string) string {
	// Keep the default output filename safe on all systems.
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", " ", "_")
	out := repl.Replace(s)
	if out == "" {
		out = "ordner"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
