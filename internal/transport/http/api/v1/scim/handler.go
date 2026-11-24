package scim

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Handler handles SCIM v2 protocol endpoints.
type Handler struct {
	ScimService appscim.Service
}

// NewHandler creates a new SCIM handler with the required service dependencies.
func NewHandler(scimService appscim.Service) *Handler {
	return &Handler{
		ScimService: scimService,
	}
}

// --- User Endpoints ---

// ListUsers handles GET /scim/v2/Users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required (trusted tenant scope)")
		return
	}

	startIndex, count, ok := parseScimListParams(w, r)
	if !ok {
		return
	}

	resp, err := h.ScimService.ListUsers(r.Context(), orgID, appscim.ScimListQuery{
		StartIndex: startIndex,
		Count:      count,
		Filter:     r.URL.Query().Get("filter"),
		SortBy:     r.URL.Query().Get("sortBy"),
		SortOrder:  r.URL.Query().Get("sortOrder"),
	})
	if err != nil {
		writeSCIMListError(w, err)
		return
	}

	// Convert generic ScimListResponse to interface-based response for transport
	resources := make([]interface{}, 0, len(resp.Resources))
	for _, u := range resp.Resources {
		resources = append(resources, u)
	}

	writeSCIMJSON(w, http.StatusOK, ScimListResponse{
		TotalResults: resp.TotalResults,
		StartIndex:   resp.StartIndex,
		ItemsPerPage: resp.ItemsPerPage,
		Schemas:      resp.Schemas,
		Resources:    resources,
	})
}

// GetUser handles GET /scim/v2/Users/{id}
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing user id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	user, err := h.ScimService.GetUser(r.Context(), orgID, id)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, user)
}

// CreateUser handles POST /scim/v2/Users
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required (trusted tenant scope)")
		return
	}

	var req appscim.ScimUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.ScimService.CreateUser(r.Context(), orgID, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusCreated, user)
}

// UpdateUser handles PUT /scim/v2/Users/{id}
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing user id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	var req appscim.ScimUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.ScimService.ReplaceUser(r.Context(), orgID, id, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, user)
}

// PatchUser handles PATCH /scim/v2/Users/{id}
func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing user id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	var req appscim.ScimPatchOp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.ScimService.PatchUser(r.Context(), orgID, id, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /scim/v2/Users/{id}
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing user id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	if err := h.ScimService.DeleteUser(r.Context(), orgID, id); err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Group Endpoints ---

// ListGroups handles GET /scim/v2/Groups
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required (trusted tenant scope)")
		return
	}

	startIndex, count, ok := parseScimListParams(w, r)
	if !ok {
		return
	}

	resp, err := h.ScimService.ListGroups(r.Context(), orgID, appscim.ScimListQuery{
		StartIndex: startIndex,
		Count:      count,
		Filter:     r.URL.Query().Get("filter"),
		SortBy:     r.URL.Query().Get("sortBy"),
		SortOrder:  r.URL.Query().Get("sortOrder"),
	})
	if err != nil {
		writeSCIMListError(w, err)
		return
	}

	resources := make([]interface{}, 0, len(resp.Resources))
	for _, g := range resp.Resources {
		resources = append(resources, g)
	}

	writeSCIMJSON(w, http.StatusOK, ScimListResponse{
		TotalResults: resp.TotalResults,
		StartIndex:   resp.StartIndex,
		ItemsPerPage: resp.ItemsPerPage,
		Schemas:      resp.Schemas,
		Resources:    resources,
	})
}

// GetGroup handles GET /scim/v2/Groups/{id}
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing group id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	group, err := h.ScimService.GetGroup(r.Context(), orgID, id)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, group)
}

// CreateGroup handles POST /scim/v2/Groups
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required (trusted tenant scope)")
		return
	}

	var req appscim.ScimGroup
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.ScimService.CreateGroup(r.Context(), orgID, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusCreated, group)
}

// UpdateGroup handles PUT /scim/v2/Groups/{id}
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing group id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	var req appscim.ScimGroup
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.ScimService.ReplaceGroup(r.Context(), orgID, id, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, group)
}

// PatchGroup handles PATCH /scim/v2/Groups/{id}
func (h *Handler) PatchGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing group id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	var req appscim.ScimPatchOp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSCIMError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.ScimService.PatchGroup(r.Context(), orgID, id, req)
	if err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	writeSCIMJSON(w, http.StatusOK, group)
}

// DeleteGroup handles DELETE /scim/v2/Groups/{id}
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeSCIMError(w, http.StatusBadRequest, "missing group id")
		return
	}

	orgID := resolveOrgID(r)
	if orgID == "" {
		writeSCIMError(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	if err := h.ScimService.DeleteGroup(r.Context(), orgID, id); err != nil {
		writeSCIMError(w, scimStatusCodeFromError(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

// ScimListResponse is the transport-layer SCIM list response with interface{} Resources.
type ScimListResponse struct {
	TotalResults int           `json:"totalResults"`
	StartIndex   int           `json:"startIndex"`
	ItemsPerPage int           `json:"itemsPerPage"`
	Schemas      []string      `json:"schemas"`
	Resources    []interface{} `json:"Resources,omitempty"`
}

// ScimError is the transport-layer SCIM error response.
type ScimError struct {
	Schemas  []string `json:"schemas"`
	Detail   string   `json:"detail"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
}

func writeSCIMJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	// The canonical RFC 7644 writer lives in transport/http/route, next to the
	// OAuth one, so the protocol face has a single error-format source
	// (手册 4.1.3 落地清单第 3 项).
	route.WriteSCIMError(w, status, detail)
}

// writeSCIMListError 透传 app 层错误 Meta 中的 scimType（如 count 超限的
// tooMany），未携带时回落到状态码映射。
func writeSCIMListError(w http.ResponseWriter, err error) {
	scimType := ""
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		if v, ok := appErr.Meta["scimType"].(string); ok {
			scimType = v
		}
	}
	route.WriteSCIMErrorWithType(w, scimStatusCodeFromError(err), err.Error(), scimType)
}

// parseScimListParams parses startIndex/count strictly (BE-E-07): a
// non-numeric or negative value is a 400 client error, not a silent default.
// An absent parameter keeps zero so the service applies protocol defaults.
// The accessors stay literal so the OpenAPI generator and check-api-params
// can see which parameters this face reads.
func parseScimListParams(w http.ResponseWriter, r *http.Request) (startIndex, count int, ok bool) {
	for _, p := range []struct {
		name string
		dst  *int
		raw  string
	}{
		{"startIndex", &startIndex, r.URL.Query().Get("startIndex")},
		{"count", &count, r.URL.Query().Get("count")},
	} {
		if p.raw == "" {
			continue
		}
		v, err := strconv.Atoi(p.raw)
		if err != nil || v < 0 {
			writeSCIMError(w, http.StatusBadRequest, p.name+" must be a non-negative integer")
			return 0, 0, false
		}
		*p.dst = v
	}
	return startIndex, count, true
}

// resolveOrgID returns the tenant scope injected by BearerTokenAuth after the
// bearer token was validated. It only accepts that trusted scope: query params
// and headers are caller-controlled and are deliberately ignored here, so the
// organization used for validation and the one used for execution are the same
// value (GAP-011/014). No scope yields "", which the handlers map to 400.
func resolveOrgID(r *http.Request) string {
	return tenantctx.TenantScopeFromContext(r.Context()).OrganizationID
}

// Error classification helpers using structured error codes.
func isBadRequest(err error) bool {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.CodeInvalidArgument
	}
	return false
}

func isConflict(err error) bool {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.CodeConflict || appErr.Code == apperrors.CodeAlreadyExists
	}
	return false
}

func isNotFound(err error) bool {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.CodeNotFound
	}
	return false
}

// scimStatusCodeFromError maps a platform/errors.Error to the corresponding HTTP
// status code using the structured error code, falling back to 500 for unknown errors.
func scimStatusCodeFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}
