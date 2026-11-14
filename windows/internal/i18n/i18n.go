// Package i18n picks the UI language (English or Turkish) from the system and
// translates the app's Go-side strings. The English text is the lookup key, so
// an untranslated string simply stays English.
//
// TOLLGATE_LANG=tr|en overrides the detected language (useful in development).
package i18n

import (
	"fmt"
	"os"
	"strings"
)

const (
	English = "en"
	Turkish = "tr"
)

var lang = detect()

// Lang is the active language code: "en" or "tr".
func Lang() string { return lang }

// T translates s into the active language.
func T(s string) string {
	if lang == Turkish {
		if v, ok := turkish[s]; ok {
			return v
		}
	}
	return s
}

// F translates a fmt format string, then formats it.
func F(format string, args ...any) string { return fmt.Sprintf(T(format), args...) }

func detect() string {
	if v := os.Getenv("TOLLGATE_LANG"); v != "" {
		return normalize(v)
	}
	return normalize(systemLanguage())
}

// normalize maps a locale name ("tr-TR", "tr_TR.UTF-8", "en-US") to a supported code.
func normalize(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	if l == "tr" || strings.HasPrefix(l, "tr-") || strings.HasPrefix(l, "tr_") || strings.HasPrefix(l, "tr.") {
		return Turkish
	}
	return English
}
