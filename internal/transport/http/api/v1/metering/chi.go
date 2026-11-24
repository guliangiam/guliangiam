package metering

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	appmetering "github.com/guliangiam/guliangiam/internal/app/metering"
	"github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// RegisterChiRoutes mounts the three metering endpoints on the given router.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// 租户以认证会话的作用域为准；显式 organizationId 仅对平台管理员的
	// 跨租户操作生效（与 rebac / users 等处一致），普通用户传入的值被忽略。
	orgID := func(r *http.Request) string {
		return auth.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	}

	// GET /api/v1/metering/events
	router.Get("/api/v1/metering/events", func(w http.ResponseWriter, r *http.Request) {
		from, err := parseISO(r.URL.Query().Get("from"))
		if err != nil {
			route.WriteError(w, r, errors.NewInvalidArgument("from must be ISO-8601"))
			return
		}
		to, err := parseISO(r.URL.Query().Get("to"))
		if err != nil {
			route.WriteError(w, r, errors.NewInvalidArgument("to must be ISO-8601"))
			return
		}

		evType := qPtr(r, "eventType")
		subj := qPtr(r, "subject")
		items, total, err := handler.ListEvents(r.Context(), appmetering.ListEventsQuery{
			OrganizationID: orgID(r),
			EventType:      evType,
			Subject:        subj,
			From:           from,
			To:             to,
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// POST /api/v1/metering/events:ingest
	router.Post("/api/v1/metering/events/ingest", func(w http.ResponseWriter, r *http.Request) {
		var reqs []IngestEventRequest
		if err := route.DecodeJSON(r, &reqs); err != nil {
			route.WriteError(w, r, err)
			return
		}
		inserted, duplicates, err := handler.Ingest(r.Context(), orgID(r), reqs)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusAccepted, map[string]any{
			"inserted":   inserted,
			"duplicates": duplicates,
		})
	})

	// GET /api/v1/metering/aggregates
	router.Get("/api/v1/metering/aggregates", func(w http.ResponseWriter, r *http.Request) {
		var periodPtr *appmetering.Period
		if p := r.URL.Query().Get("period"); p != "" {
			pp := appmetering.Period(p)
			periodPtr = &pp
		}
		subj := qPtr(r, "subject")
		metric := qPtr(r, "metric")
		items, total, err := handler.ListAggregates(r.Context(), appmetering.ListAggregatesQuery{
			OrganizationID: orgID(r),
			Period:         periodPtr,
			Subject:        subj,
			Metric:         metric,
			Page:           route.QueryInt(r, "page", 1),
			PageSize:       route.QueryInt(r, "pageSize", route.DefaultPageSize),
		})
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, presenter.NewListResponse(items, route.QueryInt(r, "page", 1), route.QueryInt(r, "pageSize", route.DefaultPageSize), total))
	})

	// Detail endpoints read through the metering app service; the id is always
	// combined with the trusted organization id inside the repository predicate.
	router.Get("/api/v1/metering/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		event, err := handler.Service.GetEvent(r.Context(), orgID(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, event)
	})
	router.Get("/api/v1/metering/aggregates/{id}", func(w http.ResponseWriter, r *http.Request) {
		aggregate, err := handler.Service.GetAggregate(r.Context(), orgID(r), chi.URLParam(r, "id"))
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, aggregate)
	})

	// POST /api/v1/metering/aggregates/refresh — admin-triggered single-cell re-roll.
	// Useful when aggregate worker missed a window or operators need to back-fill
	// on demand. The RefreshAggregateService is idempotent (ON CONFLICT DO UPDATE).
	router.Post("/api/v1/metering/aggregates/refresh", func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			route.WriteError(w, r, errors.NewInvalidArgument("period query param is required"))
			return
		}
		q := r.URL.Query()
		subject := q.Get("subject")
		metric := q.Get("metric")
		if metric == "" {
			route.WriteError(w, r, errors.NewInvalidArgument("metric query param is required"))
			return
		}

		agg, err := handler.RefreshAggregate(r.Context(), orgID(r), appmetering.Period(period), subject, metric)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, agg)
	})
}

func qPtr(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

func parseISO(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
