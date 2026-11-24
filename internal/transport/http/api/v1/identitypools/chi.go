package identitypools

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	appw "github.com/guliangiam/guliangiam/internal/app/workload"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func RegisterChiRoutes(router chi.Router, handler Handler) {
	// 租户以认证作用域为准；query 的 organizationId 仅对平台管理员的显式
	// 跨租户操作生效。app 层对空组织返回 400，不会落到仓储的 "空=全租户" 分支。
	orgID := func(r *http.Request) string {
		return authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	}

	router.Get("/api/v1/identity-pools", func(w http.ResponseWriter, r *http.Request) {
		items, total, err := handler.ListPools(r.Context(), appw.ListPoolsQuery{OrganizationID: orgID(r), Page: route.QueryInt(r, "page", 1), PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize)})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})
	router.Post("/api/v1/identity-pools", func(w http.ResponseWriter, r *http.Request) {
		var cmd appw.CreatePoolCommand
		if err := route.DecodeJSON(r, &cmd); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.CreatePool(r.Context(), orgID(r), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteCreated(w, "/api/v1/identity-pools/"+resp.ID, resp)
	})
	router.Get("/api/v1/identity-pools/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetPool(r.Context(), orgID(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})

	router.Get("/api/v1/identity-pools/{poolId}/federation-trusts", func(w http.ResponseWriter, r *http.Request) {
		items, total, err := handler.ListTrusts(r.Context(), appw.ListTrustsQuery{OrganizationID: orgID(r), PoolID: chi.URLParam(r, "poolId"), Page: route.QueryInt(r, "page", 1), PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize)})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})
	router.Post("/api/v1/identity-pools/{poolId}/federation-trusts", func(w http.ResponseWriter, r *http.Request) {
		var cmd appw.CreateTrustCommand
		if err := route.DecodeJSON(r, &cmd); err != nil {
			route.WriteError(w, r, err)
			return
		}
		cmd.PoolID = chi.URLParam(r, "poolId")
		resp, err := handler.CreateTrust(r.Context(), orgID(r), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteCreated(w, "/api/v1/identity-pools/"+chi.URLParam(r, "poolId")+"/federation-trusts/"+resp.ID, resp)
	})
	router.Post("/api/v1/identity-pools/{poolId}/federation-trusts:match-subject", func(w http.ResponseWriter, r *http.Request) {
		var request MatchSubjectRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.PreviewSubjectMatches(request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Get("/api/v1/identity-pools/{poolId}/federation-trusts/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetTrust(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Patch("/api/v1/identity-pools/{poolId}/federation-trusts/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var request UpdateTrustStatusRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.SetTrustStatus(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Delete("/api/v1/identity-pools/{poolId}/federation-trusts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := handler.DeleteTrust(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})

	router.Get("/api/v1/identity-pools/{poolId}/service-accounts", func(w http.ResponseWriter, r *http.Request) {
		items, total, err := handler.ListAccounts(r.Context(), appw.ListAccountsQuery{OrganizationID: orgID(r), PoolID: chi.URLParam(r, "poolId"), Page: route.QueryInt(r, "page", 1), PageSize: route.QueryInt(r, "pageSize", route.DefaultPageSize)})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})
	router.Post("/api/v1/identity-pools/{poolId}/service-accounts", func(w http.ResponseWriter, r *http.Request) {
		var cmd appw.CreateAccountCommand
		if err := route.DecodeJSON(r, &cmd); err != nil {
			route.WriteError(w, r, err)
			return
		}
		cmd.PoolID = chi.URLParam(r, "poolId")
		resp, err := handler.CreateAccount(r.Context(), orgID(r), cmd)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteCreated(w, "/api/v1/identity-pools/"+chi.URLParam(r, "poolId")+"/service-accounts/"+resp.ID, resp)
	})
	router.Get("/api/v1/identity-pools/{poolId}/service-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetAccount(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Patch("/api/v1/identity-pools/{poolId}/service-accounts/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var request UpdateAccountStatusRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.SetAccountStatus(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Get("/api/v1/identity-pools/{poolId}/service-accounts/{id}/delete-precheck", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler.GetAccountDeletePrecheck(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Post("/api/v1/identity-pools/{poolId}/service-accounts/{id}/rotate-secret", func(w http.ResponseWriter, r *http.Request) {
		var request RotateAccountSecretRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		resp, err := handler.RotateAccountSecret(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, resp)
	})
	router.Delete("/api/v1/identity-pools/{poolId}/service-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var request DeleteAccountRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteAccount(r.Context(), orgID(r), chi.URLParam(r, "poolId"), chi.URLParam(r, "id"), request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewActionResponse("deleted", "resource deleted"))
	})
}
