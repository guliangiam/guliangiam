package privacy

import (
	"context"
	"time"

	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
)

// Handler 是 DSAR 管理面的 HTTP 适配层；租户来自请求上下文（可信来源），
// 请求体不接受 organizationId。
type Handler struct {
	Service appprivacy.Service
	Consent appprivacy.ConsentService
}

type CreateDsarRequest struct {
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	RequestType string `json:"requestType"`
}

type TransitionDsarRequest struct {
	Target   string `json:"target"`
	Reason   string `json:"reason,omitempty"`
	Revision int64  `json:"revision,omitempty"`
}

type DsarResponse struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	SubjectType     string     `json:"subjectType"`
	SubjectID       string     `json:"subjectId"`
	RequestType     string     `json:"requestType"`
	State           string     `json:"state"`
	SLADueAt        time.Time  `json:"slaDueAt"`
	ClosedAt        *time.Time `json:"closedAt,omitempty"`
	CoolingUntil    *time.Time `json:"coolingUntil,omitempty"`
	RejectionReason *string    `json:"rejectionReason,omitempty"`
	CreatedBy       *string    `json:"createdBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Revision        int64      `json:"revision"`
}

func toResponse(request appprivacy.DsarView) DsarResponse {
	return DsarResponse{
		ID:              request.ID,
		OrganizationID:  request.OrganizationID,
		SubjectType:     request.SubjectType,
		SubjectID:       request.SubjectID,
		RequestType:     request.RequestType,
		State:           request.State,
		SLADueAt:        request.SLADueAt,
		ClosedAt:        request.ClosedAt,
		CoolingUntil:    request.CoolingUntil,
		RejectionReason: request.RejectionReason,
		CreatedBy:       request.CreatedBy,
		CreatedAt:       request.CreatedAt,
		UpdatedAt:       request.UpdatedAt,
		Revision:        request.Revision,
	}
}

func (h Handler) Create(ctx context.Context, organizationID, actorID string, request CreateDsarRequest) (*DsarResponse, error) {
	created, err := h.Service.Create(ctx, appprivacy.CreateDsarRequestCommand{
		OrganizationID: organizationID,
		SubjectType:    request.SubjectType,
		SubjectID:      request.SubjectID,
		RequestType:    request.RequestType,
		ActorID:        actorID,
	})
	if err != nil {
		return nil, err
	}
	response := toResponse(appprivacy.ToDsarView(created))
	return &response, nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*DsarResponse, error) {
	request, err := h.Service.Get(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	response := toResponse(appprivacy.ToDsarView(request))
	return &response, nil
}

func (h Handler) List(ctx context.Context, organizationID string, limit, offset int) ([]DsarResponse, int64, error) {
	requests, total, err := h.Service.List(ctx, organizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]DsarResponse, 0, len(requests))
	for _, request := range requests {
		responses = append(responses, toResponse(appprivacy.ToDsarView(request)))
	}
	return responses, total, nil
}

func (h Handler) ListOverdue(ctx context.Context, organizationID string, limit int) ([]DsarResponse, error) {
	requests, err := h.Service.ListOverdue(ctx, h.Service.Clock.Now(), limit)
	if err != nil {
		return nil, err
	}
	responses := make([]DsarResponse, 0, len(requests))
	for _, request := range requests {
		if request.OrganizationID != organizationID {
			continue
		}
		responses = append(responses, toResponse(appprivacy.ToDsarView(request)))
	}
	return responses, nil
}

func (h Handler) Transition(ctx context.Context, organizationID, id, actorID string, request TransitionDsarRequest) (*DsarResponse, error) {
	updated, err := h.Service.Transition(ctx, appprivacy.TransitionDsarRequestCommand{
		OrganizationID: organizationID,
		ID:             id,
		Target:         request.Target,
		Reason:         request.Reason,
		ActorID:        actorID,
		Revision:       request.Revision,
	})
	if err != nil {
		return nil, err
	}
	response := toResponse(appprivacy.ToDsarView(updated))
	return &response, nil
}

// Export builds the evidence package for a fulfilled access/portability DSAR
// and returns its deterministic JSON Lines representation (design/privacy §4).
func (h Handler) Export(ctx context.Context, organizationID, id, actorID string) ([]byte, *appprivacy.EvidenceDownload, error) {
	download, err := h.Service.ExportDownload(ctx, organizationID, id, actorID)
	if err != nil {
		return nil, nil, err
	}
	return download.Lines, download, nil
}

type EraseTargetRequest struct {
	Category string `json:"category"`
	TargetID string `json:"targetId"`
}

type RetentionItemRequest struct {
	Category   string `json:"category"`
	LegalBasis string `json:"legalBasis"`
	Blocks     bool   `json:"blocks"`
}

type ReviewErasureRequest struct {
	Targets    []EraseTargetRequest   `json:"targets"`
	Retentions []RetentionItemRequest `json:"retentions"`
}

// ReviewErasure runs the compliance two-track adjudication for an erase DSAR.
func (h Handler) ReviewErasure(ctx context.Context, organizationID, id, actorID string, request ReviewErasureRequest) (*appprivacy.EraseScopeView, error) {
	targets := make([]appprivacy.ErasureTarget, 0, len(request.Targets))
	for _, target := range request.Targets {
		targets = append(targets, appprivacy.ErasureTarget{Category: target.Category, TargetID: target.TargetID})
	}
	retentions := make([]appprivacy.RetentionRule, 0, len(request.Retentions))
	for _, retention := range request.Retentions {
		retentions = append(retentions, appprivacy.RetentionRule{
			Category: retention.Category, LegalBasis: retention.LegalBasis, Blocks: retention.Blocks,
		})
	}
	scope, err := h.Service.ReviewErasure(ctx, appprivacy.ReviewErasureCommand{
		OrganizationID: organizationID,
		ID:             id,
		ActorID:        actorID,
		Targets:        targets,
		Retentions:     retentions,
	})
	if err != nil {
		return nil, err
	}
	view := appprivacy.ToEraseScopeView(scope)
	return &view, nil
}

type ConsentRecordResponse struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	SubjectType    string     `json:"subjectType"`
	SubjectID      string     `json:"subjectId"`
	Purpose        string     `json:"purpose"`
	Version        int        `json:"version"`
	GrantedAt      time.Time  `json:"grantedAt"`
	WithdrawnAt    *time.Time `json:"withdrawnAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

type GrantConsentRequest struct {
	SubjectType string `json:"subjectType,omitempty"`
	SubjectID   string `json:"subjectId"`
	Purpose     string `json:"purpose"`
}

type WithdrawConsentRequest struct {
	SubjectType string `json:"subjectType,omitempty"`
	SubjectID   string `json:"subjectId"`
}

func toConsentResponse(view appprivacy.ConsentView) ConsentRecordResponse {
	return ConsentRecordResponse{
		ID:             view.ID,
		OrganizationID: view.OrganizationID,
		SubjectType:    view.SubjectType,
		SubjectID:      view.SubjectID,
		Purpose:        view.Purpose,
		Version:        view.Version,
		GrantedAt:      view.GrantedAt,
		WithdrawnAt:    view.WithdrawnAt,
		CreatedAt:      view.CreatedAt,
		UpdatedAt:      view.UpdatedAt,
		Revision:       view.Revision,
	}
}

func (h Handler) ListConsents(ctx context.Context, organizationID string, query appprivacy.ListConsentsQuery) ([]ConsentRecordResponse, error) {
	query.OrganizationID = organizationID
	records, err := h.Consent.List(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]ConsentRecordResponse, 0, len(records))
	for _, record := range records {
		out = append(out, toConsentResponse(appprivacy.ToConsentView(record)))
	}
	return out, nil
}

func (h Handler) GrantConsent(ctx context.Context, organizationID, actorID string, request GrantConsentRequest) (*ConsentRecordResponse, error) {
	record, err := h.Consent.Grant(ctx, appprivacy.GrantConsentCommand{
		OrganizationID: organizationID,
		ActorID:        actorID,
		SubjectType:    request.SubjectType,
		SubjectID:      request.SubjectID,
		Purpose:        request.Purpose,
	})
	if err != nil {
		return nil, err
	}
	response := toConsentResponse(appprivacy.ToConsentView(record))
	return &response, nil
}

func (h Handler) WithdrawConsent(ctx context.Context, organizationID, actorID, purpose string, request WithdrawConsentRequest) (*ConsentRecordResponse, error) {
	record, err := h.Consent.Withdraw(ctx, appprivacy.WithdrawConsentCommand{
		OrganizationID: organizationID,
		ActorID:        actorID,
		SubjectType:    request.SubjectType,
		SubjectID:      request.SubjectID,
		Purpose:        purpose,
	})
	if err != nil {
		return nil, err
	}
	response := toConsentResponse(appprivacy.ToConsentView(record))
	return &response, nil
}
