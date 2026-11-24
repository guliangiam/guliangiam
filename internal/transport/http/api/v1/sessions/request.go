package sessions

import appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"

type ListSessionsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	Keyword        string
	ActiveOnly     *bool
}

type GetSessionRequest struct {
	ID             string
	OrganizationID string
}

type PatchSessionRequest struct {
	SessionState *string `json:"sessionState"`
	Revision     int64   `json:"revision"`
}

func (r PatchSessionRequest) toCommand(id string) appsessions.UpdateSessionCommand {
	return appsessions.UpdateSessionCommand{
		ID:           id,
		SessionState: r.SessionState,
		Revision:     r.Revision,
	}
}

func (r ListSessionsRequest) toQuery() appsessions.ListSessionsQuery {
	return appsessions.ListSessionsQuery{
		Page:           r.Page,
		PageSize:       r.PageSize,
		OrganizationID: r.OrganizationID,
		UserID:         r.UserID,
		ApplicationID:  r.ApplicationID,
		State:          r.State,
		Keyword:        r.Keyword,
		ActiveOnly:     r.ActiveOnly,
	}
}
