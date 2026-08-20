package i18n

import "testing"

func TestFormatSize(t *testing.T) {
	x := T{}
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{999, "999 bytes"},
		{1000, "1.0 kB"},
		{1536000, "1.5 MB"},
		{4_000_000_000_000, "4.0 TB"},
		{993_000_000_000, "993.0 GB"},
	}
	for _, c := range cases {
		if got := x.FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}

func TestFormatInt(t *testing.T) {
	x := T{}
	if got := x.FormatInt(1234567); got != "1,234,567" {
		t.Errorf("got %q", got)
	}
	if got := x.FormatInt(999); got != "999" {
		t.Errorf("small: %q", got)
	}
	if got := x.FormatInt(0); got != "0" {
		t.Errorf("zero: %q", got)
	}
}

func TestFormatETA(t *testing.T) {
	x := T{}
	if got := x.FormatETA(30); got != "under 1 minute" {
		t.Errorf("%q", got)
	}
	if got := x.FormatETA(90); got != "~2 min" {
		t.Errorf("%q", got)
	}
	if got := x.FormatETA(2*3600 + 15*60); got != "~2 h 15 min" {
		t.Errorf("%q", got)
	}
	if got := x.FormatETA(3600); got != "~1 h" {
		t.Errorf("%q", got)
	}
}

func TestDefaultOutputName(t *testing.T) {
	x := T{}
	got := x.DefaultOutputName("Accessions 2024/Fonds:A", "2026-08-20")
	want := "checksums_Accessions_2024-Fonds-A_2026-08-20.csv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNoEmDashesInTexts(t *testing.T) {
	x := T{}
	all := []string{
		x.WelcomeText("v1"), x.ResumeLastRun("r", "o", "5"), x.OutputInsideRoot(),
		x.ResumeFound("5"), x.ResumeDifferentRoot("old"), x.OverwriteExisting(),
		x.ConfirmStart("r", "o"), x.CanceledText("o"),
		x.DoneText("5", "1 GB", "o", 2, "2"), x.DoneWithErrors("1", "log"),
	}
	for _, s := range all {
		if idx := indexOf(s, "—"); idx >= 0 {
			t.Errorf("em dash in dialog text: %q", s[:idx+3])
		}
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
