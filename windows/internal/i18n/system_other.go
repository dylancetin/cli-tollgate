//go:build !windows

package i18n

import "os"

// systemLanguage follows the POSIX locale variables (development builds).
func systemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
