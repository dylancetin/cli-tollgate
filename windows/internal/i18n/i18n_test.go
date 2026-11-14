package i18n

import (
	"regexp"
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"tr-TR": Turkish, "tr_TR.UTF-8": Turkish, "TR": Turkish, "tr": Turkish,
		"en-US": English, "": English, "C": English, "de-DE": English, "trk": English,
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

var verb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

// A translation must keep the same fmt verbs in the same order, or
// Sprintf would print garbage like %!s(MISSING).
func TestTurkishKeepsVerbs(t *testing.T) {
	for en, tr := range turkish {
		if tr == "" {
			t.Errorf("%q has an empty translation", en)
		}
		if a, b := verb.FindAllString(en, -1), verb.FindAllString(tr, -1); !slices.Equal(a, b) {
			t.Errorf("%q → %q: verbs %v vs %v", en, tr, a, b)
		}
	}
}

func TestTranslate(t *testing.T) {
	saved := lang
	defer func() { lang = saved }()

	lang = Turkish
	if got := F("Running on %s", "127.0.0.1:8317"); got != "127.0.0.1:8317 üzerinde çalışıyor" {
		t.Errorf("F = %q", got)
	}
	if got := T("not in the table"); got != "not in the table" {
		t.Errorf("untranslated string changed: %q", got)
	}
	lang = English
	if got := T("Stopped"); got != "Stopped" {
		t.Errorf("English T = %q", got)
	}
}
