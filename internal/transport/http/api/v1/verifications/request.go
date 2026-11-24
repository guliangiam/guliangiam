package verifications

type ListVerificationsRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Recipient      string `json:"recipient"`
}

type SendVerificationCodeRequest struct {
	OrganizationID string `json:"organizationId"`
	// ApplicationID scopes delivery to the provider bound to the application
	// currently signing in / registering, so a tenant can route one app's SMS
	// through a dedicated provider account.
	ApplicationID string `json:"applicationId,omitempty"`
	Type          string `json:"type"`
	Recipient     string `json:"recipient"`
	CaptchaToken  string `json:"captchaToken,omitempty"`
	CaptchaID     string `json:"captchaId,omitempty"`
	ClientIP      string `json:"-"` // set from HTTP request, not from JSON body
}

type VerifyCodeRequest struct {
	Type      string `json:"type"`
	Recipient string `json:"recipient"`
	Code      string `json:"code"`
	// OrganizationID optionally binds verification to the issuing tenant
	// (BE-A-20). The public FE verify modal does not send it; empty keeps the
	// legacy org-agnostic behaviour.
	OrganizationID string `json:"organizationId"`
	// ApplicationID is accepted for symmetry with the send request; verification
	// itself resolves the provider from the binding recorded at send time.
	ApplicationID string `json:"applicationId,omitempty"`
	ClientIP      string `json:"-"` // set from HTTP request, not from JSON body
}
