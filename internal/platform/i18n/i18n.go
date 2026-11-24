package i18n

import (
	"embed"
	"encoding/json"
	"sync"
)

// SupportedLanguages lists all languages that have translations.
var SupportedLanguages = []string{
	"zh-CN", "zh-TW", "en", "ru", "ja", "ko", "de", "fr", "es", "pt", "vi", "pl", "tr", "uk",
}

// IsSupportedLanguage reports whether code is one of the 14 languages backed by
// an embedded translation package. The language set is a code-level capability,
// not tenant reference data.
func IsSupportedLanguage(code string) bool {
	for _, supported := range SupportedLanguages {
		if code == supported {
			return true
		}
	}
	return false
}

//go:embed locales/*/data.json
var localeFS embed.FS

// Translator provides translation lookup by language and key.
type Translator struct {
	mu      sync.RWMutex
	langMap map[string]map[string]string // language -> key -> translation
}

// NewTranslator creates a Translator and preloads all embedded translations.
func NewTranslator() *Translator {
	t := &Translator{
		langMap: make(map[string]map[string]string),
	}
	for _, lang := range SupportedLanguages {
		t.loadLanguage(lang)
	}
	return t
}

// loadLanguage reads and parses the embedded JSON for a given language.
func (t *Translator) loadLanguage(lang string) {
	path := "locales/" + lang + "/data.json"
	data, err := localeFS.ReadFile(path)
	if err != nil {
		// Language file not found; skip silently
		return
	}

	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return
	}

	t.mu.Lock()
	t.langMap[lang] = translations
	t.mu.Unlock()
}

// Translate returns the translated string for the given language and key.
// Falls back to English if the key is not found in the requested language,
// then falls back to the key itself if not found in English either.
func (t *Translator) Translate(language, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try requested language
	if m, ok := t.langMap[language]; ok {
		if val, ok := m[key]; ok {
			return val
		}
	}

	// Fallback to English
	if language != "en" {
		if m, ok := t.langMap["en"]; ok {
			if val, ok := m[key]; ok {
				return val
			}
		}
	}

	// Return key as last resort
	return key
}

// TranslateWithArgs translates and performs simple {{.key}} substitution.
func (t *Translator) TranslateWithArgs(language, key string, args map[string]string) string {
	text := t.Translate(language, key)
	for k, v := range args {
		text = replaceAll(text, "{{."+k+"}}", v)
	}
	return text
}

// HasLanguage checks if translations are loaded for the given language.
func (t *Translator) HasLanguage(language string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.langMap[language]
	return ok
}

// GetTranslations returns all translations for a language.
func (t *Translator) GetTranslations(language string) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if m, ok := t.langMap[language]; ok {
		// Return a copy
		result := make(map[string]string, len(m))
		for k, v := range m {
			result[k] = v
		}
		return result
	}
	return map[string]string{}
}

// UpdateTranslation sets or updates a single translation key for a language.
func (t *Translator) UpdateTranslation(language, key, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.langMap[language]; !ok {
		t.langMap[language] = make(map[string]string)
	}
	t.langMap[language][key] = value
}

// replaceAll is a simple string replacement without regexp.
func replaceAll(s, old, replacement string) string {
	result := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return result + s
		}
		result += s[:i] + replacement
		s = s[i+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
