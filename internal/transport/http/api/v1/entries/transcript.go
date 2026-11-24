package entries

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// Transcript handles GET /api/v1/entries/{id}/transcript: one page of an entry's
// message sequence, in the order the messages were produced.
//
// The response carries facts that were ingested from a running system, so it is
// declared uncacheable before anything else. It is the one read face that can
// contain live credentials, and a cached copy would outlive the redaction that
// produced it (42.3.13 条 5).
func (h Handler) Transcript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	meta := route.RequestMetadataFrom(r.Context())
	response, err := h.Service.ListEntryTranscript(r.Context(), appentries.TranscriptQuery{
		EntryID:        chi.URLParam(r, "id"),
		OrganizationID: authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId")),
		Cursor:         r.URL.Query().Get("cursor"),
		Limit:          route.QueryInt(r, "limit", 0),
		IPAddress:      util.ClientIP(r),
		UserAgent:      r.UserAgent(),
		Surface:        string(meta.Surface),
		RequestID:      meta.RequestID,
		TraceID:        meta.TraceID,
	})
	if err != nil {
		route.WriteError(w, r, err)
		return
	}

	route.WriteJSON(w, http.StatusOK, response)
}
