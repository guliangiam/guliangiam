package users

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// CustomAttributeResponse is the management projection of one definition.
type CustomAttributeResponse struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	DataType    string   `json:"dataType"`
	Required    bool     `json:"required"`
	Options     []string `json:"options"`
	Writable    bool     `json:"writable"`
	Visibility  string   `json:"visibility"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Revision    int64    `json:"revision"`
}

type CustomAttributeListResponse struct {
	Items []CustomAttributeResponse `json:"items"`
}

type CreateCustomAttributeRequest struct {
	OrganizationID string   `json:"organizationId,omitempty"`
	Key            string   `json:"key"`
	DataType       string   `json:"dataType"`
	Required       bool     `json:"required"`
	Options        []string `json:"options,omitempty"`
	Writable       *bool    `json:"writable,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	Description    string   `json:"description,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
}

type UpdateCustomAttributeRequest struct {
	OrganizationID string   `json:"organizationId,omitempty"`
	DataType       string   `json:"dataType"`
	Required       bool     `json:"required"`
	Options        []string `json:"options,omitempty"`
	Writable       *bool    `json:"writable,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	Description    string   `json:"description,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
}

func toCustomAttributeResponse(definition appusers.CustomAttributeView) CustomAttributeResponse {
	return CustomAttributeResponse{
		ID: definition.ID, Key: definition.Key, DataType: definition.DataType,
		Required: definition.Required, Options: definition.Options, Writable: definition.Writable,
		Visibility: definition.Visibility, Description: definition.Description,
		Enabled: definition.Enabled, Revision: definition.Revision,
	}
}

func (h Handler) ListCustomAttributes(ctx context.Context, organizationID string, includeDisabled bool) (CustomAttributeListResponse, error) {
	definitions, err := h.Service.ListCustomAttributes(ctx, organizationID, includeDisabled)
	if err != nil {
		return CustomAttributeListResponse{}, err
	}
	items := make([]CustomAttributeResponse, 0, len(definitions))
	for _, definition := range definitions {
		items = append(items, toCustomAttributeResponse(appusers.ToCustomAttributeView(definition)))
	}
	return CustomAttributeListResponse{Items: items}, nil
}

func (h Handler) CreateCustomAttribute(ctx context.Context, request CreateCustomAttributeRequest) (CustomAttributeResponse, error) {
	definition, err := h.Service.CreateCustomAttribute(ctx, appusers.CreateCustomAttributeCommand{
		OrganizationID: request.OrganizationID, ActorID: operatorID(ctx), Key: request.Key, DataType: request.DataType,
		Required: request.Required, Options: request.Options, Writable: request.Writable,
		Visibility: request.Visibility, Description: request.Description, Enabled: request.Enabled,
	})
	if err != nil {
		return CustomAttributeResponse{}, err
	}
	return toCustomAttributeResponse(appusers.ToCustomAttributeView(definition)), nil
}

func (h Handler) UpdateCustomAttribute(ctx context.Context, key string, request UpdateCustomAttributeRequest) (CustomAttributeResponse, error) {
	definition, err := h.Service.UpdateCustomAttribute(ctx, appusers.UpdateCustomAttributeCommand{
		OrganizationID: request.OrganizationID, ActorID: operatorID(ctx), Key: key, DataType: request.DataType,
		Required: request.Required, Options: request.Options, Writable: request.Writable,
		Visibility: request.Visibility, Description: request.Description, Enabled: request.Enabled,
	})
	if err != nil {
		return CustomAttributeResponse{}, err
	}
	return toCustomAttributeResponse(appusers.ToCustomAttributeView(definition)), nil
}

func (h Handler) DeleteCustomAttribute(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteCustomAttribute(ctx, organizationID, id, operatorID(ctx))
}

func registerCustomAttributeRoutes(router chi.Router, handler Handler) {
	router.Get("/api/v1/users/custom-attributes", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		includeDisabled := r.URL.Query().Get("includeDisabled") == "true"
		response, err := handler.ListCustomAttributes(r.Context(), organizationID, includeDisabled)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Post("/api/v1/users/custom-attributes", func(w http.ResponseWriter, r *http.Request) {
		var request CreateCustomAttributeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID
		response, err := handler.CreateCustomAttribute(r.Context(), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusCreated, response)
	})

	router.Put("/api/v1/users/custom-attributes/{key}", func(w http.ResponseWriter, r *http.Request) {
		var request UpdateCustomAttributeRequest
		if err := route.DecodeJSON(r, &request); err != nil {
			route.WriteError(w, r, err)
			return
		}
		organizationID, err := trustedOrgID(r, request.OrganizationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		request.OrganizationID = organizationID
		response, err := handler.UpdateCustomAttribute(r.Context(), chi.URLParam(r, "key"), request)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Delete("/api/v1/users/custom-attributes/{id}", func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := trustedOrgID(r, r.URL.Query().Get("organizationId"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		if err := handler.DeleteCustomAttribute(r.Context(), organizationID, chi.URLParam(r, "id")); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
