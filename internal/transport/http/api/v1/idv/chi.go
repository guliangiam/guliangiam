package idv

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes wires the idv endpoints.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// GET /api/v1/idv/verifications
	router.Get("/api/v1/idv/verifications", func(w http.ResponseWriter, r *http.Request) {
		orgID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, total, err := handler.List(r.Context(), ListQuery{
			OrganizationID: orgID,
			UserID:         queryStringPtr(r, "userId"),
			Provider:       queryStringPtr(r, "provider"),
			Status:         queryStringPtr(r, "status"),
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(resp, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// GET /api/v1/idv/verifications/{id}
	router.Get("/api/v1/idv/verifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		orgID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.GetByID(r.Context(), chi.URLParam(r, "id"), orgID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	// DELETE /api/v1/idv/verifications/{id}
	router.Delete("/api/v1/idv/verifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		orgID, err := trustedOrgID(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.Delete(r.Context(), chi.URLParam(r, "id"), orgID); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// RegisterCallbackChiRoutes wires the public identity-provider callback. It is
// deliberately on the public face: a third-party provider cannot hold a
// session, so authentication is the webhook signature rather than a bearer
// token. The raw body is read once and handed to the verifier untouched.
func RegisterCallbackChiRoutes(router chi.Router, handler Handler) {
	router.Post("/api/v1/idv/callback/{provider}", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCallbackBodyBytes)
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				route.WriteError(w, r, apperrors.NewRequestEntityTooLargef("callback body exceeds %d bytes", maxCallbackBodyBytes))
				return
			}
			route.WriteError(w, r, apperrors.NewInvalidArgument("callback body is unreadable"))
			return
		}
		if len(payload) == 0 {
			route.WriteError(w, r, apperrors.NewInvalidArgument("callback body is required"))
			return
		}

		headers := make(map[string]string, len(r.Header))
		for key := range r.Header {
			headers[key] = r.Header.Get(key)
		}

		if _, err := handler.HandleCallback(r.Context(), chi.URLParam(r, "provider"), payload, headers); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("accepted", "callback accepted"))
	})
}

// trustedOrgID 解析并校验租户：以认证会话的作用域为准（平台管理员可显式指定
// 目标租户），且必须是 UUID —— 仓储把该值直接写进 uuid 列，slug/空串会让
// PostgreSQL 抛 22P02（invalid input syntax for type uuid）并以 500 暴露。
// 参数缺失或形态非法都属于客户端错误，统一返回 400。
func trustedOrgID(r *http.Request) (string, error) {
	orgID := auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		return "", apperrors.NewInvalidArgument("organizationId is required")
	}
	if _, err := uuid.Parse(orgID); err != nil {
		return "", apperrors.NewInvalidArgument("organizationId must be a UUID")
	}
	return orgID, nil
}

func queryStringPtr(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}
