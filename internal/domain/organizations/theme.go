package organizations

type OrganizationThemeSettings struct {
	OrganizationID string
	ThemeName      string
	Tokens         map[string]any
	CustomCSS      string
	Assets         map[string]any
}
