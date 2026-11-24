package i18n

import "github.com/guliangiam/guliangiam/internal/transport/http/route"

func Routes() []route.Definition {
	return []route.Definition{
		{
			Method:      "GET",
			Pattern:     "/api/v1/i18n/languages",
			OperationID: "ListLanguages",
			Summary:     "List supported languages",
			Permission:  "i18n:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/i18n/translations/{language}",
			OperationID: "GetTranslations",
			Summary:     "Get translations for a language",
			Permission:  "i18n:read",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "PUT",
			Pattern:     "/api/v1/i18n/translations/{language}",
			OperationID: "UpdateTranslations",
			Summary:     "Update translations for a language",
			Permission:  "i18n:write",
			Surface:     route.SurfaceManagement,
		},
		{
			Method:      "GET",
			Pattern:     "/api/v1/i18n/translate",
			OperationID: "TranslateText",
			Summary:     "Translate text using AI provider",
			Permission:  "i18n:write",
			Surface:     route.SurfaceManagement,
		},
	}
}
