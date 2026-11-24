package records

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	metaKeyAllowed     = "allowed"
	metaKeyField       = "field"
	metaKeyValue       = "value"
	metaKeyMax         = "max"
	auditResultFailure = "failure"
	auditResultSuccess = "success"
)

// AuditLogQuery 是审计中心分类日志查询。
type AuditLogQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Category       string
	Result         string
	ApplicationID  string
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
}

// LoginLogQuery 是登录日志查询。
type LoginLogQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ApplicationID  string
	Result         string
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
}

// LoginLogDTO 是登录日志视图行。
type LoginLogDTO struct {
	ID             string    `json:"id"`
	Result         string    `json:"result"`
	Username       string    `json:"username"`
	OrganizationID string    `json:"organizationId"`
	ApplicationID  string    `json:"applicationId"`
	IP             string    `json:"ip"`
	UserAgent      string    `json:"userAgent"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (s Service) auditReader() (domain.AuditLogReader, error) {
	reader, ok := s.UOW.Records().(domain.AuditLogReader)
	if !ok {
		return nil, apperrors.New(apperrors.CodeInternal, "audit log query is not supported by the current storage driver")
	}
	return reader, nil
}

// ListAuditLogs 按分类（login/operation/error）返回审计记录。
func (s Service) ListAuditLogs(ctx context.Context, query AuditLogQuery) ([]*RecordDTO, int64, error) {
	reader, err := s.auditReader()
	if err != nil {
		return nil, 0, err
	}
	category, ok := normalizeCategory(query.Category)
	if !ok {
		return nil, 0, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "category must be one of login, operation, error", map[string]any{
			metaKeyField: "category", metaKeyValue: query.Category, metaKeyAllowed: []string{"login", "operation", "error"},
		})
	}
	result, ok := normalizeResult(query.Result)
	if !ok {
		return nil, 0, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "result must be one of success, failure", map[string]any{
			metaKeyField: "result", metaKeyValue: query.Result, metaKeyAllowed: []string{auditResultSuccess, auditResultFailure},
		})
	}
	items, total, err := reader.ListAuditLogs(ctx, domain.AuditLogQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		Category:       category,
		Result:         result,
		ApplicationID:  query.ApplicationID,
		Keyword:        query.Keyword,
		StartTime:      query.StartTime,
		EndTime:        query.EndTime,
	})
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*RecordDTO, len(items))
	for i, item := range items {
		dtos[i] = toRecordDTO(item)
	}
	return dtos, total, nil
}

// ListLoginLogs 合并成功登录（sessions）与失败登录（records）。
func (s Service) ListLoginLogs(ctx context.Context, query LoginLogQuery) ([]*LoginLogDTO, int64, error) {
	reader, err := s.auditReader()
	if err != nil {
		return nil, 0, err
	}
	result, ok := normalizeResult(query.Result)
	if !ok {
		return nil, 0, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "result must be one of success, failure", map[string]any{
			metaKeyField: "result", metaKeyValue: query.Result, metaKeyAllowed: []string{auditResultSuccess, auditResultFailure},
		})
	}
	items, total, err := reader.ListLoginLogs(ctx, domain.LoginLogQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		ApplicationID:  query.ApplicationID,
		Result:         result,
		Keyword:        query.Keyword,
		StartTime:      query.StartTime,
		EndTime:        query.EndTime,
	})
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]*LoginLogDTO, len(items))
	for i, item := range items {
		dtos[i] = &LoginLogDTO{
			ID:             item.ID,
			Result:         string(item.Result),
			Username:       item.Username,
			OrganizationID: item.OrganizationID,
			ApplicationID:  item.ApplicationID,
			IP:             item.IP,
			UserAgent:      item.UserAgent,
			Reason:         item.Reason,
			CreatedAt:      item.CreatedAt,
		}
	}
	return dtos, total, nil
}

func normalizeCategory(value string) (domain.AuditLogCategory, bool) {
	switch value {
	case "", "login":
		return domain.AuditLogCategory(value), true
	case "operation", "error":
		return domain.AuditLogCategory(value), true
	default:
		return "", false
	}
}

func normalizeResult(value string) (domain.AuditLogResult, bool) {
	switch value {
	case "", auditResultSuccess, auditResultFailure:
		return domain.AuditLogResult(value), true
	default:
		return "", false
	}
}
