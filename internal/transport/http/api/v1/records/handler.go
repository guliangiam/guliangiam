package records

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	apprecords "github.com/guliangiam/guliangiam/internal/app/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

type Handler struct {
	Service apprecords.Service
}

func (h Handler) Create(r *http.Request, request CreateRecordRequest) (*apprecords.RecordDTO, error) {
	// surface / request_id / trace_id are stamped by the request metadata
	// middleware from the route (手册 4.1.3, 12.5); they are deliberately not
	// part of the request body so a client cannot forge its API face.
	//
	// The actor and its origin are audit evidence too: they come from the
	// verified session and the observed request, never from CreateRecordRequest
	// (that body is accepted only for wire compatibility).
	meta := route.RequestMetadataFrom(r.Context())
	actorID := ""
	if session, ok := authtransport.SessionContextFromContext(r.Context()); ok {
		actorID = session.UserID
	}
	return h.Service.CreateRecord(r.Context(), apprecords.CreateRecordCommand{
		OrganizationID: request.OrganizationID,
		ActorID:        actorID,
		Action:         request.Action,
		ResourceType:   request.ResourceType,
		ResourceID:     request.ResourceID,
		IP:             util.ClientIP(r),
		UserAgent:      r.UserAgent(),
		Surface:        string(meta.Surface),
		RequestID:      meta.RequestID,
		TraceID:        meta.TraceID,
		Metadata:       request.Metadata,
	})
}

func (h Handler) List(ctx context.Context, request ListRecordsRequest) (ListRecordsResponse, error) {
	items, total, err := h.Service.ListRecords(ctx, apprecords.ListRecordsQuery(request))
	if err != nil {
		return ListRecordsResponse{}, err
	}

	// Echo the values the repository actually applies; a request that relied on
	// the defaults must not be reported back as page/pageSize 0.
	page, pageSize := platformpagination.ClampPage(request.Page), platformpagination.ClampPageSize(request.PageSize)
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// ListAuditLogs 返回按业务分类（登录/操作/错误）过滤的审计记录。
func (h Handler) ListAuditLogs(ctx context.Context, request AuditLogListRequest) (ListRecordsResponse, error) {
	items, total, err := h.Service.ListAuditLogs(ctx, apprecords.AuditLogQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		Category:       request.Category,
		Result:         request.Result,
		ApplicationID:  request.ApplicationID,
		Keyword:        request.Keyword,
		StartTime:      request.StartTime,
		EndTime:        request.EndTime,
	})
	if err != nil {
		return ListRecordsResponse{}, err
	}
	page, pageSize := platformpagination.ClampPage(request.Page), platformpagination.ClampPageSize(request.PageSize)
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// ListLoginLogs 返回合并后的登录日志（成功会话 + 失败审计记录）。
func (h Handler) ListLoginLogs(ctx context.Context, request AuditLogListRequest) (ListLoginLogsResponse, error) {
	items, total, err := h.Service.ListLoginLogs(ctx, apprecords.LoginLogQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		ApplicationID:  request.ApplicationID,
		Result:         request.Result,
		Keyword:        request.Keyword,
		StartTime:      request.StartTime,
		EndTime:        request.EndTime,
	})
	if err != nil {
		return ListLoginLogsResponse{}, err
	}
	page, pageSize := platformpagination.ClampPage(request.Page), platformpagination.ClampPageSize(request.PageSize)
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

func (h Handler) Search(ctx context.Context, request SearchRecordsRequest) (ListRecordsResponse, error) {
	items, total, err := h.Service.SearchRecords(ctx, apprecords.SearchRecordsCriteria(request))
	if err != nil {
		return ListRecordsResponse{}, err
	}

	// Search carries pagination in the body, where 0 means "omitted" rather than
	// "zero rows" (platformpagination.ValidateBodyPageParams). The repository
	// serves 1/DefaultPageSize for those, so the response has to say so.
	page, pageSize := platformpagination.ClampPage(request.Page), platformpagination.ClampPageSize(request.PageSize)
	return presenter.NewListResponse(items, page, pageSize, total), nil
}

const maxExportRecords int64 = 10_000

// Export returns a bounded, checksummed audit evidence bundle and records the
// export itself as an audit event (AC-SEC-005).
func (h Handler) Export(ctx context.Context, r *http.Request, request SearchRecordsRequest) (ExportRecordsResponse, error) {
	criteria := apprecords.SearchRecordsCriteria(request)
	criteria.Page = 1
	criteria.PageSize = 100
	items, total, err := h.Service.SearchRecords(ctx, criteria)
	if err != nil {
		return ExportRecordsResponse{}, err
	}
	if total > maxExportRecords {
		return ExportRecordsResponse{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"export exceeds the maximum record count",
			map[string]any{"field": "page", "max": maxExportRecords, "total": total})
	}
	records := make([]*apprecords.RecordDTO, 0, total)
	records = append(records, items...)
	for int64(len(records)) < total {
		criteria.Page++
		page, pageTotal, err := h.Service.SearchRecords(ctx, criteria)
		if err != nil {
			return ExportRecordsResponse{}, err
		}
		if len(page) == 0 || pageTotal != total {
			break
		}
		records = append(records, page...)
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return ExportRecordsResponse{}, apperrors.WrapInternal(err, "marshal audit export")
	}
	sum := sha256.Sum256(payload)
	manifest := ExportManifest{
		Count:       int64(len(records)),
		SHA256:      hex.EncodeToString(sum[:]),
		GeneratedAt: time.Now().UTC(),
		Filters: map[string]any{
			"actorId": request.ActorID, "action": request.Action,
			"resourceType": request.ResourceType, "resourceId": request.ResourceID,
			"requestId": request.RequestID, "keyword": request.Keyword,
			"startTime": request.StartTime, "endTime": request.EndTime,
		},
	}
	// The export is itself auditable; a client cannot ask for evidence without
	// leaving a trail. The checksum proves the bytes returned to the caller.
	meta := route.RequestMetadataFrom(ctx)
	actorID := ""
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		actorID = session.UserID
	}
	if _, err := h.Service.CreateRecord(ctx, apprecords.CreateRecordCommand{
		OrganizationID: request.OrganizationID,
		ActorID:        actorID,
		Action:         "records.exported",
		ResourceType:   "records",
		ResourceID:     manifest.SHA256,
		IP:             util.ClientIP(r),
		UserAgent:      r.UserAgent(),
		Surface:        string(meta.Surface),
		RequestID:      meta.RequestID,
		TraceID:        meta.TraceID,
		Metadata:       map[string]any{"count": manifest.Count, "sha256": manifest.SHA256},
	}); err != nil {
		return ExportRecordsResponse{}, err
	}
	return ExportRecordsResponse{Manifest: manifest, Records: records}, nil
}
