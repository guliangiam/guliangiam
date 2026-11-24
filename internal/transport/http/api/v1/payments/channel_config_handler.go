package payments

import (
	"context"
	"encoding/json"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// ChannelConfigHandler 是渠道配置版本化管理的 HTTP handler (design doc G6 §P2).
// 依赖 apppayments.ChannelConfigService — 不直接 import domain — Clean Arch Rule 2.
type ChannelConfigHandler struct {
	Svc apppayments.ChannelConfigService
}

func NewChannelConfigHandler(svc apppayments.ChannelConfigService) ChannelConfigHandler {
	return ChannelConfigHandler{Svc: svc}
}

// ─── Request / Response ──────────────────────────────────────────────

type ListChannelConfigsRequest struct {
	OrganizationID string
	Page           int
	PageSize       int
}

type CreateChannelConfigRequest struct {
	ID             string `json:"id,omitempty"` // 可选；缺省由服务端生成 UUID
	OrganizationID string `json:"organizationId"`
	Channel        string `json:"channel"`
	Environment    string `json:"environment"` // sandbox / production
	// Config is the plaintext JSON object. The server canonicalizes it, derives
	// configSet from its top-level keys and seals it before persistence. There
	// is deliberately no configSealed field: strict decoding rejects old clients
	// instead of accepting client-supplied ciphertext.
	Config json.RawMessage `json:"config"`
}

type ChannelConfigTransitionRequest struct {
	Status string `json:"status"`
}

type ChannelConfigView struct {
	ID                string   `json:"id"`
	OrganizationID    string   `json:"organizationId"`
	Channel           string   `json:"channel"`
	Environment       string   `json:"environment"`
	ConfigFingerprint string   `json:"configFingerprint"`
	ConfigSet         []string `json:"configSet,omitempty"`
	VersionNo         int      `json:"versionNo"`
	Status            string   `json:"status"`
	TestPassedAt      *string  `json:"testPassedAt,omitempty"`
	ActivatedAt       *string  `json:"activatedAt,omitempty"`
	DisabledAt        *string  `json:"disabledAt,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

type ListChannelConfigsResponse = presenter.ListResponse[ChannelConfigView]

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02T15:04:05Z")
	return &s
}

// ─── Handlers ────────────────────────────────────────────────────────

func (h ChannelConfigHandler) List(ctx context.Context, req ListChannelConfigsRequest) (ListChannelConfigsResponse, error) {
	cfgs, err := h.Svc.List(ctx, req.OrganizationID)
	if err != nil {
		return ListChannelConfigsResponse{}, err
	}
	r := make([]ChannelConfigView, 0, len(cfgs))
	for _, c := range cfgs {
		r = append(r, ChannelConfigView{
			ID:                c.ID,
			OrganizationID:    c.OrganizationID,
			Channel:           c.Channel,
			Environment:       c.Environment,
			ConfigFingerprint: c.ConfigFingerprint,
			ConfigSet:         c.ConfigSet,
			VersionNo:         c.VersionNo,
			Status:            c.Status,
			TestPassedAt:      formatTimePtr(c.TestPassedAt),
			ActivatedAt:       formatTimePtr(c.ActivatedAt),
			DisabledAt:        formatTimePtr(c.DisabledAt),
			CreatedAt:         c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			UpdatedAt:         c.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return paginateList(r, req.Page, req.PageSize), nil
}

func (h ChannelConfigHandler) Create(ctx context.Context, req CreateChannelConfigRequest) (ChannelConfigView, error) {
	if req.OrganizationID == "" || req.Channel == "" || len(req.Config) == 0 {
		return ChannelConfigView{}, errInvalidArgument("organizationId, channel and config are required")
	}
	cfg, err := h.Svc.Create(ctx, apppayments.CreateChannelConfigInput{
		ID:             req.ID,
		OrganizationID: req.OrganizationID,
		Channel:        req.Channel,
		Environment:    req.Environment,
		Config:         string(req.Config),
	})
	if err != nil {
		return ChannelConfigView{}, err
	}
	return ChannelConfigView{
		ID:                cfg.ID,
		OrganizationID:    cfg.OrganizationID,
		Channel:           cfg.Channel,
		Environment:       cfg.Environment,
		ConfigFingerprint: cfg.ConfigFingerprint,
		ConfigSet:         cfg.ConfigSet,
		VersionNo:         cfg.VersionNo,
		Status:            cfg.Status,
		TestPassedAt:      formatTimePtr(cfg.TestPassedAt),
		ActivatedAt:       formatTimePtr(cfg.ActivatedAt),
		DisabledAt:        formatTimePtr(cfg.DisabledAt),
		CreatedAt:         cfg.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         cfg.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (h ChannelConfigHandler) Get(ctx context.Context, organizationID, id string) (ChannelConfigView, error) {
	cfg, err := h.Svc.GetByID(ctx, organizationID, id)
	if err != nil {
		return ChannelConfigView{}, err
	}
	return ChannelConfigView{
		ID:                cfg.ID,
		OrganizationID:    cfg.OrganizationID,
		Channel:           cfg.Channel,
		Environment:       cfg.Environment,
		ConfigFingerprint: cfg.ConfigFingerprint,
		ConfigSet:         cfg.ConfigSet,
		VersionNo:         cfg.VersionNo,
		Status:            cfg.Status,
		TestPassedAt:      formatTimePtr(cfg.TestPassedAt),
		ActivatedAt:       formatTimePtr(cfg.ActivatedAt),
		DisabledAt:        formatTimePtr(cfg.DisabledAt),
		CreatedAt:         cfg.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         cfg.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (h ChannelConfigHandler) Transition(ctx context.Context, organizationID, id string, newStatus string) (ChannelConfigView, error) {
	if newStatus == "" {
		return ChannelConfigView{}, errInvalidArgument("status is required")
	}
	cfg, err := h.Svc.Transition(ctx, organizationID, id, newStatus)
	if err != nil {
		return ChannelConfigView{}, err
	}
	return ChannelConfigView{
		ID:                cfg.ID,
		OrganizationID:    cfg.OrganizationID,
		Channel:           cfg.Channel,
		Environment:       cfg.Environment,
		ConfigFingerprint: cfg.ConfigFingerprint,
		ConfigSet:         cfg.ConfigSet,
		VersionNo:         cfg.VersionNo,
		Status:            cfg.Status,
		TestPassedAt:      formatTimePtr(cfg.TestPassedAt),
		ActivatedAt:       formatTimePtr(cfg.ActivatedAt),
		DisabledAt:        formatTimePtr(cfg.DisabledAt),
		CreatedAt:         cfg.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         cfg.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}
