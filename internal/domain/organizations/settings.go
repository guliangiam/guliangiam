package organizations

type AccountItem struct {
	Name       string `json:"name"`
	Visible    bool   `json:"visible"`
	ViewRule   string `json:"viewRule"`
	ModifyRule string `json:"modifyRule"`
	Regex      string `json:"regex"`
	Tab        string `json:"tab"`
}

type MfaItem struct {
	Name string `json:"name"`
	Rule string `json:"rule,omitempty"`
}

type OrganizationSettings struct {
	OrganizationID       string
	Languages            []string
	DefaultApplicationID string
	AccountItems         []AccountItem
	NavPreferences       map[string]any
	FeatureFlags         map[string]any
	CountryCodes         []string
	UserTypes            []string
	Tags                 []string
	DcrPolicy            string
	// Phase 2: Extended settings fields
	MfaItems           []MfaItem
	AccountMenu        string
	MfaRememberInHours int
}
