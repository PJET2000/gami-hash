package i18n

import "testing"

func TestDetect(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	cases := []struct {
		override, lcAll, lang string
		want                  Lang
	}{
		{"de-DE", "", "", DE},
		{"de-AT", "", "", DE},
		{"en-US", "", "", EN},
		{"", "de_DE.UTF-8", "", DE},
		{"", "", "de_DE.utf8", DE},
		{"", "", "fr_FR.UTF-8", EN},
		{"", "", "", EN},
		{"", "C", "de_DE.UTF-8", EN}, // LC_ALL=C wins over LANG
	}
	for _, c := range cases {
		t.Setenv("LC_ALL", c.lcAll)
		t.Setenv("LANG", c.lang)
		if got := Detect(c.override); got != c.want {
			t.Errorf("Detect(%q) with LC_ALL=%q LANG=%q = %q, want %q", c.override, c.lcAll, c.lang, got, c.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	de := T{Lang: DE}
	en := T{Lang: EN}
	cases := []struct {
		bytes  int64
		de, en string
	}{
		{0, "0 Byte", "0 bytes"},
		{999, "999 Byte", "999 bytes"},
		{1000, "1,0 kB", "1.0 kB"},
		{1536000, "1,5 MB", "1.5 MB"},
		{4_000_000_000_000, "4,0 TB", "4.0 TB"},
		{993_000_000_000, "993,0 GB", "993.0 GB"},
	}
	for _, c := range cases {
		if got := de.FormatSize(c.bytes); got != c.de {
			t.Errorf("de FormatSize(%d) = %q, want %q", c.bytes, got, c.de)
		}
		if got := en.FormatSize(c.bytes); got != c.en {
			t.Errorf("en FormatSize(%d) = %q, want %q", c.bytes, got, c.en)
		}
	}
}

func TestFormatInt(t *testing.T) {
	de := T{Lang: DE}
	en := T{Lang: EN}
	if got := de.FormatInt(1234567); got != "1.234.567" {
		t.Errorf("de: %q", got)
	}
	if got := en.FormatInt(1234567); got != "1,234,567" {
		t.Errorf("en: %q", got)
	}
	if got := de.FormatInt(999); got != "999" {
		t.Errorf("de small: %q", got)
	}
	if got := de.FormatInt(0); got != "0" {
		t.Errorf("zero: %q", got)
	}
}

func TestFormatETA(t *testing.T) {
	de := T{Lang: DE}
	if got := de.FormatETA(30); got != "unter 1 Minute" {
		t.Errorf("%q", got)
	}
	if got := de.FormatETA(90); got != "~2 Min." {
		t.Errorf("%q", got)
	}
	if got := de.FormatETA(2*3600 + 15*60); got != "~2 Std. 15 Min." {
		t.Errorf("%q", got)
	}
	en := T{Lang: EN}
	if got := en.FormatETA(3600); got != "~1 h" {
		t.Errorf("%q", got)
	}
}

func TestDefaultOutputName(t *testing.T) {
	de := T{Lang: DE}
	got := de.DefaultOutputName("Akzessionslaufwerk 2024/Bestand:A", "2026-08-20")
	want := "pruefsummen_Akzessionslaufwerk_2024-Bestand-A_2026-08-20.csv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
