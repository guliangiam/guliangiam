package i18n

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformi18n "github.com/guliangiam/guliangiam/internal/platform/i18n"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Handler provides HTTP endpoints for i18n management.
type Handler struct {
	translator *platformi18n.Translator
}

// NewHandler creates a new i18n Handler.
func NewHandler(translator *platformi18n.Translator) *Handler {
	return &Handler{translator: translator}
}

// Language represents a supported language with metadata.
type Language struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"nativeName"`
}

// languageMetadata holds display names only. The supported SET comes from
// platformi18n.SupportedLanguages; a code without metadata falls back to the
// raw code so a language added to the translator can never silently disappear
// from the API (the drift that previously shipped 12 of 14 languages).
var languageMetadata = map[string]Language{
	"zh-CN": {Code: "zh-CN", Name: "Chinese (Simplified)", NativeName: "简体中文"},
	"zh-TW": {Code: "zh-TW", Name: "Chinese (Traditional)", NativeName: "繁體中文"},
	"en":    {Code: "en", Name: "English", NativeName: "English"},
	"ru":    {Code: "ru", Name: "Russian", NativeName: "Русский"},
	"ja":    {Code: "ja", Name: "Japanese", NativeName: "日本語"},
	"ko":    {Code: "ko", Name: "Korean", NativeName: "한국어"},
	"de":    {Code: "de", Name: "German", NativeName: "Deutsch"},
	"fr":    {Code: "fr", Name: "French", NativeName: "Français"},
	"es":    {Code: "es", Name: "Spanish", NativeName: "Español"},
	"pt":    {Code: "pt", Name: "Portuguese", NativeName: "Português"},
	"vi":    {Code: "vi", Name: "Vietnamese", NativeName: "Tiếng Việt"},
	"pl":    {Code: "pl", Name: "Polish", NativeName: "Polski"},
	"tr":    {Code: "tr", Name: "Turkish", NativeName: "Türkçe"},
	"uk":    {Code: "uk", Name: "Ukrainian", NativeName: "Українська"},
}

// supportedLanguages renders platformi18n.SupportedLanguages in order.
func supportedLanguages() []Language {
	languages := make([]Language, 0, len(platformi18n.SupportedLanguages))
	for _, code := range platformi18n.SupportedLanguages {
		meta, ok := languageMetadata[code]
		if !ok {
			meta = Language{Code: code, Name: code, NativeName: code}
		}
		languages = append(languages, meta)
	}
	return languages
}

// ListLanguages returns all supported languages.
// GET /api/v1/i18n/languages
func (h *Handler) ListLanguages(w http.ResponseWriter, r *http.Request) {
	languages := supportedLanguages()
	route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(languages, 1, len(languages), int64(len(languages))))
}

// GetTranslations returns all translations for a given language.
// GET /api/v1/i18n/translations/{language}
func (h *Handler) GetTranslations(w http.ResponseWriter, r *http.Request) {
	language := chi.URLParam(r, "language")
	if language == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("language parameter is required"))
		return
	}

	if !h.translator.HasLanguage(language) {
		route.WriteError(w, r, apperrors.NewNotFound("language not found"))
		return
	}

	translations := h.translator.GetTranslations(language)
	route.WriteJSON(w, http.StatusOK, translations)
}

// UpdateTranslation updates or creates a single translation key.
// PUT /api/v1/i18n/translations/{language}
func (h *Handler) UpdateTranslation(w http.ResponseWriter, r *http.Request) {
	language := chi.URLParam(r, "language")
	if language == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("language parameter is required"))
		return
	}

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// A body over the bootstrap MaxBytesReader wall must be the registered
		// 413, not the generic invalid-JSON 400 this handler used to return.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			route.WriteError(w, r, err)
			return
		}
		route.WriteError(w, r, apperrors.NewInvalidArgument("invalid JSON body"))
		return
	}

	for key, value := range body {
		h.translator.UpdateTranslation(language, key, value)
	}

	route.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"msg":    "translations updated",
	})
}

// Translate returns the translation for a single key.
// GET /api/v1/i18n/translate?language=en&key=auth:login_failed
func (h *Handler) Translate(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	key := r.URL.Query().Get("key")

	if language == "" || key == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("language and key parameters are required"))
		return
	}

	translation := h.translator.Translate(language, key)
	route.WriteJSON(w, http.StatusOK, map[string]string{
		"language":    language,
		"key":         key,
		"translation": translation,
	})
}
