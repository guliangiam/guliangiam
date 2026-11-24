package privacy

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
)

// DsarView is a transport-neutral DSAR projection (transport must not import
// the domain package; check-layers Rule 2).
type DsarView struct {
	ID              string
	OrganizationID  string
	SubjectType     string
	SubjectID       string
	RequestType     string
	State           string
	SLADueAt        time.Time
	ClosedAt        *time.Time
	CoolingUntil    *time.Time
	RejectionReason *string
	CreatedBy       *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Revision        int64
}

func ToDsarView(request *domain.DsarRequest) DsarView {
	return DsarView{
		ID: request.ID, OrganizationID: request.OrganizationID,
		SubjectType: request.SubjectType, SubjectID: request.SubjectID,
		RequestType: request.RequestType, State: request.State,
		SLADueAt: request.SLADueAt, ClosedAt: request.ClosedAt, CoolingUntil: request.CoolingUntil,
		RejectionReason: request.RejectionReason, CreatedBy: request.CreatedBy,
		CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt, Revision: request.Revision,
	}
}

// EvidenceDownload is the transport-ready evidence package.
type EvidenceDownload struct {
	Lines       []byte
	Checksum    string
	RecordCount int
	Region      string
}

func (s Service) ExportDownload(ctx context.Context, organizationID, id, actorID string) (*EvidenceDownload, error) {
	pkg, err := s.Export(ctx, organizationID, id, actorID)
	if err != nil {
		return nil, err
	}
	lines, err := pkg.JSONLines()
	if err != nil {
		return nil, err
	}
	return &EvidenceDownload{
		Lines: lines, Checksum: pkg.Checksum, RecordCount: pkg.LineCount, Region: pkg.Region,
	}, nil
}

// EraseScopeView is the transport-neutral erasure adjudication result.
type EraseTargetView struct {
	Category string `json:"category"`
	TargetID string `json:"targetId"`
}

type RetentionItemView struct {
	Category   string `json:"category"`
	LegalBasis string `json:"legalBasis"`
	Blocks     bool   `json:"blocks"`
}

type EraseScopeView struct {
	Erase  []EraseTargetView   `json:"erase"`
	Retain []RetentionItemView `json:"retain"`
}

func ToEraseScopeView(scope *domain.EraseScope) EraseScopeView {
	view := EraseScopeView{
		Erase:  make([]EraseTargetView, 0, len(scope.Erase)),
		Retain: make([]RetentionItemView, 0, len(scope.Retain)),
	}
	for _, item := range scope.Erase {
		view.Erase = append(view.Erase, EraseTargetView{Category: item.Category, TargetID: item.TargetID})
	}
	for _, item := range scope.Retain {
		view.Retain = append(view.Retain, RetentionItemView{Category: item.Category, LegalBasis: item.LegalBasis, Blocks: item.Blocks})
	}
	return view
}

// ConsentView is the transport-safe projection of a consent_records row.
type ConsentView struct {
	ID             string
	OrganizationID string
	SubjectType    string
	SubjectID      string
	Purpose        string
	Version        int
	GrantedAt      time.Time
	WithdrawnAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ToConsentView projects a domain consent record (no secrets involved).
func ToConsentView(record *domain.ConsentRecord) ConsentView {
	return ConsentView{
		ID:             record.ID,
		OrganizationID: record.OrganizationID,
		SubjectType:    record.SubjectType,
		SubjectID:      record.SubjectID,
		Purpose:        record.Purpose,
		Version:        record.Version,
		GrantedAt:      record.GrantedAt,
		WithdrawnAt:    record.WithdrawnAt,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		Revision:       record.Revision,
	}
}
