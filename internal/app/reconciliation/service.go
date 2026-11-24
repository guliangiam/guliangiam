// Package reconciliation exposes persisted SCIM outbound reconciliation
// reports to the management surface.
package reconciliation

import (
	"context"
	"encoding/json"
	"time"

	scimdomain "github.com/guliangiam/guliangiam/internal/domain/scim"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Report is the management DTO for one persisted reconciliation run.
type Report struct {
	ID           string    `json:"id"`
	ConnectorID  string    `json:"connectorId"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt"`
	UserReport   jsonRaw   `json:"userReport"`
	GroupReport  jsonRaw   `json:"groupReport"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// jsonRaw 必须是 json.RawMessage：裸 []byte 会被 encoding/json 序列化为
// base64 字符串，报告对象到前端就破裂（BE-C-19）。domain 侧的 []byte 是
// 未命名类型，可直接赋值给 json.RawMessage，无需改声明层。
type jsonRaw = json.RawMessage

// ListQuery selects one page of reports for a tenant.
type ListQuery struct {
	OrganizationID string
	Page           int
	PageSize       int
}

// PageMeta mirrors the OpenAPI PageMeta component that the declared
// PaginatedResponse envelope requires.
type PageMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// ListResponse is the paginated report response.
type ListResponse struct {
	Items []Report `json:"items"`
	Page  PageMeta `json:"page"`
}

// Service is the read side of the reconciliation report store.
type Service interface {
	List(ctx context.Context, query ListQuery) (*ListResponse, error)
}

type service struct {
	reports scimdomain.ReconciliationReportRepository
}

// NewService constructs the report query service. A nil repository keeps the
// service available but every read fails closed with unavailable.
func NewService(reports scimdomain.ReconciliationReportRepository) Service {
	return &service{reports: reports}
}

func (s *service) List(ctx context.Context, query ListQuery) (*ListResponse, error) {
	if s == nil || s.reports == nil {
		return nil, apperrors.NewUnavailable("reconciliation reports are not available on this backend")
	}
	if query.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organization is required")
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	reports, total, err := s.reports.ListByOrganization(ctx, query.OrganizationID, pageSize, platformpagination.Offset(page, pageSize))
	if err != nil {
		return nil, err
	}
	items := make([]Report, 0, len(reports))
	for _, report := range reports {
		items = append(items, Report{
			ID:           report.ID,
			ConnectorID:  report.ConnectorID,
			Status:       report.Status,
			StartedAt:    report.StartedAt,
			FinishedAt:   report.FinishedAt,
			UserReport:   report.UserReport,
			GroupReport:  report.GroupReport,
			ErrorMessage: report.ErrorMessage,
			CreatedAt:    report.CreatedAt,
		})
	}
	return &ListResponse{
		Items: items,
		Page:  PageMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}
