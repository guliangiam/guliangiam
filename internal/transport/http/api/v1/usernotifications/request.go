package usernotifications

type ListRequest struct {
	Category string `json:"category"`
	IsRead   *bool  `json:"isRead"`
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type ReplyRequest struct {
	Content string `json:"content"`
}

type BatchDeleteRequest struct {
	IDs []string `json:"ids"`
}

type PreferencesRequest struct {
	Preferences []PreferenceItemRequest `json:"preferences"`
}

type PreferenceItemRequest struct {
	Category string `json:"category"`
	Channel  string `json:"channel"`
	Enabled  bool   `json:"enabled"`
}

type ChannelBindingRequest struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"externalId"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
