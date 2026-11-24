package privacy

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/privacy/exports/{id}", func(w http.ResponseWriter, r *http.Request) {
		lines, pkg, err := handler.Export(r.Context(), orgFrom(r), chi.URLParam(r, "id"), actorFrom(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=privacy-export-%s.jsonl", chi.URLParam(r, "id")))
		w.Header().Set("X-Checksum-Sha256", pkg.Checksum)
		w.Header().Set("X-Record-Count", fmt.Sprintf("%d", pkg.RecordCount))
		w.Header().Set("X-Region", pkg.Region)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(lines)
	})

	router.Get("/api/v1/privacy/dsar", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageFrom(r)
		responses, total, err := handler.List(r.Context(), orgFrom(r), limit, offset)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(responses, limit, offset, total))
	})

	router.Post("/api/v1/privacy/dsar", func(w http.ResponseWriter, r *http.Request) {
		var request CreateDsarRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Create(r.Context(), orgFrom(r), actorFrom(r), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Get("/api/v1/privacy/consents", func(w http.ResponseWriter, r *http.Request) {
		query := appprivacy.ListConsentsQuery{
			SubjectType: r.URL.Query().Get("subjectType"),
			SubjectID:   r.URL.Query().Get("subjectId"),
			Purpose:     r.URL.Query().Get("purpose"),
			Limit:       route.QueryInt(r, "limit", 100),
		}
		if query.SubjectType == "" {
			query.SubjectType = "user"
		}
		responses, err := handler.ListConsents(r.Context(), orgFrom(r), query)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(responses, query.Limit, 0, int64(len(responses))))
	})

	router.Post("/api/v1/privacy/consents", func(w http.ResponseWriter, r *http.Request) {
		var request GrantConsentRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.GrantConsent(r.Context(), orgFrom(r), actorFrom(r), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Post("/api/v1/privacy/consents/{purpose}/withdraw", func(w http.ResponseWriter, r *http.Request) {
		var request WithdrawConsentRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.WithdrawConsent(r.Context(), orgFrom(r), actorFrom(r), chi.URLParam(r, "purpose"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/api/v1/privacy/dsar/overdue", func(w http.ResponseWriter, r *http.Request) {
		responses, err := handler.ListOverdue(r.Context(), orgFrom(r), route.QueryInt(r, "limit", 100))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, responses)
	})

	router.Get("/api/v1/privacy/dsar/{id}", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.Get(r.Context(), orgFrom(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/privacy/dsar/{id}/erase-review", func(w http.ResponseWriter, r *http.Request) {
		var request ReviewErasureRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		scope, err := handler.ReviewErasure(r.Context(), orgFrom(r), chi.URLParam(r, "id"), actorFrom(r), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, scope)
	})

	router.Post("/api/v1/privacy/dsar/{id}/transition", func(w http.ResponseWriter, r *http.Request) {
		var request TransitionDsarRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.Transition(r.Context(), orgFrom(r), chi.URLParam(r, "id"), actorFrom(r), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})
}

func orgFrom(r *http.Request) string {
	return tenantctx.TenantScopeFromContext(r.Context()).OrganizationID
}

func actorFrom(r *http.Request) string {
	if session, ok := authtransport.SessionContextFromContext(r.Context()); ok {
		return session.UserID
	}
	return ""
}

func pageFrom(r *http.Request) (int, int) {
	page := route.QueryInt(r, "page", 1)
	pageSize := route.QueryInt(r, "pageSize", route.DefaultPageSize)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	return pageSize, (page - 1) * pageSize
}
