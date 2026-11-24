package i18n

import (
	"testing"
)

func TestNewTranslator(t *testing.T) {
	translator := NewTranslator()
	if translator == nil {
		t.Fatal("expected non-nil translator")
	}
}

func TestTranslate_English(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		key      string
		expected string
	}{
		{"auth:login_failed", "Login failed"},
		{"auth:unauthorized", "Unauthorized operation"},
		{"validation:required", "This field is required"},
		{"resource:created", "Resource created successfully"},
	}

	for _, tt := range tests {
		result := translator.Translate("en", tt.key)
		if result != tt.expected {
			t.Errorf("Translate(en, %q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}

func TestTranslate_Chinese(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		key      string
		expected string
	}{
		{"auth:login_failed", "登录失败"},
		{"auth:unauthorized", "未授权的操作"},
		{"validation:required", "此字段为必填项"},
		{"resource:created", "资源创建成功"},
	}

	for _, tt := range tests {
		result := translator.Translate("zh-CN", tt.key)
		if result != tt.expected {
			t.Errorf("Translate(zh-CN, %q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}

func TestTranslate_FallbackToEnglish(t *testing.T) {
	translator := NewTranslator()

	// Use a language that exists but doesn't have a specific key that only English has
	// All languages have the same keys, so test with a key that only exists in English
	translator.UpdateTranslation("en", "test:english_only_key", "English only")
	result := translator.Translate("ja", "test:english_only_key")
	// Should fall back to English
	if result != "English only" {
		t.Errorf("Translate(ja, test:english_only_key) = %q, want English fallback", result)
	}
}

func TestTranslate_UnknownKey(t *testing.T) {
	translator := NewTranslator()

	result := translator.Translate("en", "nonexistent:key")
	if result != "nonexistent:key" {
		t.Errorf("Translate(en, nonexistent:key) = %q, want key itself", result)
	}
}

func TestTranslate_UnknownLanguage(t *testing.T) {
	translator := NewTranslator()

	result := translator.Translate("xx-XX", "auth:login_failed")
	// Should fall back to English
	if result != "Login failed" {
		t.Errorf("Translate(xx-XX, auth:login_failed) = %q, want English fallback", result)
	}
}

func TestHasLanguage(t *testing.T) {
	translator := NewTranslator()

	if !translator.HasLanguage("en") {
		t.Error("expected HasLanguage(en) = true")
	}
	if !translator.HasLanguage("zh-CN") {
		t.Error("expected HasLanguage(zh-CN) = true")
	}
	if translator.HasLanguage("xx-XX") {
		t.Error("expected HasLanguage(xx-XX) = false")
	}
}

func TestGetTranslations(t *testing.T) {
	translator := NewTranslator()

	translations := translator.GetTranslations("en")
	if len(translations) == 0 {
		t.Error("expected non-empty translations for English")
	}

	// Verify a specific key exists
	if translations["auth:login_failed"] != "Login failed" {
		t.Errorf("translations[auth:login_failed] = %q, want 'Login failed'", translations["auth:login_failed"])
	}
}

func TestGetTranslations_UnknownLanguage(t *testing.T) {
	translator := NewTranslator()

	translations := translator.GetTranslations("xx-XX")
	if len(translations) != 0 {
		t.Error("expected empty translations for unknown language")
	}
}

func TestUpdateTranslation(t *testing.T) {
	translator := NewTranslator()

	// Update existing key
	translator.UpdateTranslation("en", "auth:login_failed", "New login failed message")
	result := translator.Translate("en", "auth:login_failed")
	if result != "New login failed message" {
		t.Errorf("Translate after update = %q, want 'New login failed message'", result)
	}

	// Add new key
	translator.UpdateTranslation("en", "custom:new_key", "Custom value")
	result = translator.Translate("en", "custom:new_key")
	if result != "Custom value" {
		t.Errorf("Translate new key = %q, want 'Custom value'", result)
	}

	// Add to new language
	translator.UpdateTranslation("xx-XX", "test:key", "Test value")
	result = translator.Translate("xx-XX", "test:key")
	if result != "Test value" {
		t.Errorf("Translate new language = %q, want 'Test value'", result)
	}
}

func TestTranslateWithArgs(t *testing.T) {
	translator := NewTranslator()

	result := translator.TranslateWithArgs("en", "validation:required", map[string]string{
		"field": "username",
	})
	// Since the English translation doesn't have {{.field}}, it should return the translation as-is
	if result != "This field is required" {
		t.Errorf("TranslateWithArgs = %q, want 'This field is required'", result)
	}
}

func TestAllLanguagesHaveAuthKeys(t *testing.T) {
	translator := NewTranslator()

	for _, lang := range SupportedLanguages {
		if !translator.HasLanguage(lang) {
			t.Errorf("language %q not loaded", lang)
			continue
		}
		// Check that auth:login_failed exists in all languages
		result := translator.Translate(lang, "auth:login_failed")
		if result == "auth:login_failed" {
			t.Errorf("language %q missing auth:login_failed translation", lang)
		}
	}
}
