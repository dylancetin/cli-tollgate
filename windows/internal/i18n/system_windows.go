//go:build windows

package i18n

import "golang.org/x/sys/windows"

// systemLanguage is the user's Windows display language (for example "tr-TR").
func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
