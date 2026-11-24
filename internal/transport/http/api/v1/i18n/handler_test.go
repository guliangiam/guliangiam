package i18n

import (
	"testing"

	platformi18n "github.com/guliangiam/guliangiam/internal/platform/i18n"
)

// TestSupportedLanguagesMirrorTranslator guards the drift this batch fixed: the
// API must list every language the translator ships, with real display names.
func TestSupportedLanguagesMirrorTranslator(t *testing.T) {
	languages := supportedLanguages()
	if len(languages) != len(platformi18n.SupportedLanguages) {
		t.Fatalf("supportedLanguages() = %d entries, platformi18n has %d",
			len(languages), len(platformi18n.SupportedLanguages))
	}

	seen := make(map[string]bool, len(languages))
	for _, language := range languages {
		if language.Name == "" || language.NativeName == "" {
			t.Errorf("%s has an empty display name", language.Code)
		}
		if language.Name == language.Code {
			t.Errorf("%s has no metadata entry (fell back to the raw code)", language.Code)
		}
		seen[language.Code] = true
	}
	for _, code := range platformi18n.SupportedLanguages {
		if !seen[code] {
			t.Errorf("translator language %s is missing from the API list", code)
		}
	}
	if !seen["zh-TW"] || !seen["ru"] {
		t.Errorf("expected zh-TW and ru in the API list, got %+v", languages)
	}
}
