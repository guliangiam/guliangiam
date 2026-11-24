package risk

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	apprisk "github.com/guliangiam/guliangiam/internal/app/risk"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/risk/signals", func(w http.ResponseWriter, r *http.Request) {
		page, pageSize := pageFrom(r)
		resp, total, err := handler.ListSignals(r.Context(), ListSignalsQuery{
			OrganizationID: orgFrom(r), UserID: queryStringPtr(r, "userId"),
			SignalType: queryStringPtr(r, "signalType"), Page: page, PageSize: pageSize,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, page, pageSize, total))
	})

	router.Get("/api/v1/risk/decisions", func(w http.ResponseWriter, r *http.Request) {
		page, pageSize := pageFrom(r)
		resp, total, err := handler.ListDecisions(r.Context(), ListDecisionsQuery{
			OrganizationID: orgFrom(r), UserID: queryStringPtr(r, "userId"),
			Decision: queryStringPtr(r, "decision"), Page: page, PageSize: pageSize,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, page, pageSize, total))
	})

	router.Post("/api/v1/risk/decisions/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		var req ReviewDecisionRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.ReviewDecision(r.Context(), orgFrom(r), chi.URLParam(r, "id"), req, commandMetadata(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	router.Post("/api/v1/risk/decisions/{id}/appeal", func(w http.ResponseWriter, r *http.Request) {
		var req SubmitAppealRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.SubmitAppeal(r.Context(), orgFrom(r), chi.URLParam(r, "id"), req, commandMetadata(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	router.Post("/api/v1/risk/decisions/{id}/appeal/resolution", func(w http.ResponseWriter, r *http.Request) {
		var req ResolveAppealRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.ResolveAppeal(r.Context(), orgFrom(r), chi.URLParam(r, "id"), req, commandMetadata(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	router.Get("/api/v1/risk/session-governance", func(w http.ResponseWriter, r *http.Request) {
		page, pageSize := pageFrom(r)
		resp, total, err := handler.ListGovernance(r.Context(), ListGovernanceQuery{
			OrganizationID: orgFrom(r), Action: queryStringPtr(r, "action"), Page: page, PageSize: pageSize,
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, page, pageSize, total))
	})

	router.Post("/api/v1/risk/session-governance/{id}/transition", func(w http.ResponseWriter, r *http.Request) {
		var req TransitionGovernanceRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.TransitionGovernance(r.Context(), orgFrom(r), chi.URLParam(r, "id"), req, commandMetadata(r))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, resp)
	})

	router.Get("/api/v1/risk/audit-integrity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		resp, err := handler.VerifyAuditIntegrity(r.Context(), orgFrom(r), "")
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Get("/api/v1/risk/signals/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.Service.GetSignal(r.Context(), orgFrom(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Post("/api/v1/risk/signals", func(w http.ResponseWriter, r *http.Request) {
		var req ReportRiskSignalRequest
		if err := route.DecodeJSON(r, &req); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.Service.ReportSignal(r.Context(), apprisk.ReportSignalCommand{
			OrganizationID: orgFrom(r), UserID: req.UserID, SignalType: req.SignalType,
			SignalValue: req.SignalValue, IP: req.IP, Metadata: commandMetadata(r),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteCreated(w, "/api/v1/risk/signals/"+resp.ID, resp)
	})
	router.Get("/api/v1/risk/decisions/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetDecision(r.Context(), orgFrom(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Get("/api/v1/risk/session-governance/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetGovernance(r.Context(), orgFrom(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
}

func orgFrom(r *http.Request) string {
	return tenantctx.TenantScopeFromContext(r.Context()).OrganizationID
}

// ReportRiskSignalRequest deliberately omits organizationId: the tenant is
// resolved from the session scope, and DecodeJSON rejects unknown fields.
type ReportRiskSignalRequest struct {
	UserID      *string         `json:"userId"`
	SignalType  string          `json:"signalType"`
	SignalValue json.RawMessage `json:"signalValue"`
	IP          *string         `json:"ip"`
}

func commandMetadata(r *http.Request) apprisk.CommandMetadata {
	actorID := ""
	if session, ok := authtransport.SessionContextFromContext(r.Context()); ok {
		actorID = session.UserID
	}
	metadata := route.RequestMetadataFrom(r.Context())
	return apprisk.CommandMetadata{
		ActorID: actorID, IPAddress: util.ClientIP(r), UserAgent: r.UserAgent(),
		Surface: string(metadata.Surface), RequestID: metadata.RequestID, TraceID: metadata.TraceID,
	}
}

func pageFrom(r *http.Request) (int, int) {
	return route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize)
}

func queryStringPtr(r *http.Request, key string) *string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return nil
	}
	return &value
}
