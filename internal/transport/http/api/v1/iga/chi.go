package iga

import "github.com/go-chi/chi/v5"

func RegisterChiRoutes(r chi.Router, h Handler) {
	r.Get("/api/v1/iga/access-campaigns", h.ListCampaigns)
	r.Post("/api/v1/iga/access-campaigns", h.CreateCampaign)
	r.Get("/api/v1/iga/access-campaigns/{id}", h.GetCampaign)
	r.Patch("/api/v1/iga/access-campaigns/{id}/status", h.UpdateCampaignStatus)
	r.Delete("/api/v1/iga/access-campaigns/{id}", h.DeleteCampaign)
	r.Get("/api/v1/iga/sod-rules", h.ListSODRules)
	r.Post("/api/v1/iga/sod-rules", h.CreateSODRule)
	r.Get("/api/v1/iga/sod-rules/{id}", h.GetSODRule)
	r.Patch("/api/v1/iga/sod-rules/{id}", h.UpdateSODRule)
	r.Delete("/api/v1/iga/sod-rules/{id}", h.DeleteSODRule)
	r.Get("/api/v1/iga/access-lifecycle", h.ListLifecycle)
	r.Post("/api/v1/iga/access-lifecycle", h.CreateLifecycle)
	r.Get("/api/v1/iga/access-lifecycle/{id}", h.GetLifecycle)
	r.Patch("/api/v1/iga/access-lifecycle/{id}/status", h.UpdateLifecycleStatus)
	r.Get("/api/v1/iga/access-requests", h.ListAccessRequests)
	r.Post("/api/v1/iga/access-requests", h.CreateAccessRequest)
	r.Get("/api/v1/iga/access-requests/{id}", h.GetAccessRequest)
	r.Post("/api/v1/iga/access-requests/{id}/submit", h.SubmitAccessRequest)
	r.Post("/api/v1/iga/access-requests/{id}/approve", h.ApproveAccessRequest)
	r.Post("/api/v1/iga/access-requests/{id}/reject", h.RejectAccessRequest)
	r.Post("/api/v1/iga/access-requests/{id}/withdraw", h.WithdrawAccessRequest)
	r.Post("/api/v1/iga/access-requests/{id}/provision", h.ProvisionAccessRequest)
	r.Get("/api/v1/iga/privileged-sessions", h.ListPrivilegedSessions)
	r.Post("/api/v1/iga/privileged-sessions", h.StartPrivilegedSession)
}
