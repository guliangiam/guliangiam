package notifications

type ListNotificationsRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	Status         string `json:"status"`
	Keyword        string `json:"keyword"`
}

type CreateNotificationRequest struct {
	OrganizationID string         `json:"organizationId"`
	ProviderID     string         `json:"providerId"`
	Recipient      string         `json:"recipient"`
	Subject        string         `json:"subject"`
	Content        string         `json:"content"`
	TemplateID     string         `json:"templateId"`
	Variables      map[string]any `json:"variables"`
}

type MarkAllReadRequest struct {
	OrganizationID string `json:"organizationId"`
}
