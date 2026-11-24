// Command scim-reconcile performs one manual SCIM outbound reconciliation run:
// it fetches the downstream Users/Groups, diffs them against an expected-state
// snapshot, persists the drift report, and exits non-zero on drift. With -list
// it pages through previously persisted reports instead.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	scimdomain "github.com/guliangiam/guliangiam/internal/domain/scim"
	postgresscim "github.com/guliangiam/guliangiam/internal/infra/postgres/scim"
	scimoutbound "github.com/guliangiam/guliangiam/internal/outbound/scim"
)

type expectedState struct {
	Users  []scimoutbound.UserRecord  `json:"users"`
	Groups []scimoutbound.GroupRecord `json:"groups"`
}

type reportView struct {
	ID           string          `json:"id"`
	ConnectorID  string          `json:"connectorId"`
	Status       string          `json:"status"`
	StartedAt    time.Time       `json:"startedAt"`
	FinishedAt   time.Time       `json:"finishedAt"`
	UserReport   json.RawMessage `json:"userReport"`
	GroupReport  json.RawMessage `json:"groupReport"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

func main() {
	os.Exit(runReconcile())
}

func runReconcile() int {
	var (
		dsn         = flag.String("dsn", "", "PostgreSQL DSN (default POSTGRES_DSN/DB_DSN)")
		orgID       = flag.String("org", "", "organization id (required)")
		connectorID = flag.String("connector", "", "connector identifier recorded in the report")
		baseURL     = flag.String("base-url", "", "SCIM base URL")
		token       = flag.String("token", os.Getenv("SCIM_BEARER_TOKEN"), "SCIM bearer token (default SCIM_BEARER_TOKEN)")
		expected    = flag.String("expected", "", "expected-state JSON file (users/groups)")
		pageSize    = flag.Int("page-size", 100, "downstream page size")
		timeout     = flag.Duration("timeout", 60*time.Second, "overall timeout")
		allowLocal  = flag.Bool("allow-private-targets", false, "allow private/loopback SCIM targets (default false; SSRF-safe)")
		dryRun      = flag.Bool("dry-run", false, "do not persist the report")
		listMode    = flag.Bool("list", false, "list persisted reconciliation reports instead of running a reconciliation")
		listPage    = flag.Int("list-page", 1, "page number for -list (1-based)")
		listSize    = flag.Int("list-size", 20, "page size for -list")
	)
	flag.Parse()

	if *orgID == "" {
		flag.Usage()
		return fail("org is required")
	}
	if *dsn == "" {
		*dsn = os.Getenv("POSTGRES_DSN")
	}
	if *dsn == "" {
		*dsn = os.Getenv("DB_DSN")
	}
	if *dsn == "" {
		return fail("dsn is required (POSTGRES_DSN/DB_DSN)")
	}

	if *listMode {
		return listReports(*dsn, *orgID, *listPage, *listSize, *timeout)
	}

	if *connectorID == "" || *baseURL == "" || *expected == "" {
		flag.Usage()
		return fail("connector, base-url and expected are required (or use -list)")
	}
	if *allowLocal {
		_ = os.Setenv("SCIM_ALLOW_PRIVATE_TARGETS", "true")
	}

	var state expectedState
	raw, err := os.ReadFile(*expected)
	if err != nil {
		return fail("read expected state: " + err.Error())
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return fail("decode expected state: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	startedAt := time.Now().UTC()
	client := scimoutbound.NewClient(*baseURL, *token, int(timeout.Milliseconds()), 0)
	userReport, groupReport, reconcileErr := scimoutbound.FetchAndReconcile(ctx, client, state.Users, state.Groups, *pageSize)
	finishedAt := time.Now().UTC()

	report := &scimdomain.ReconciliationReport{
		ID:             uuid.NewString(),
		OrganizationID: *orgID,
		ConnectorID:    *connectorID,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		Status:         "completed",
		CreatedAt:      finishedAt,
	}
	if reconcileErr != nil {
		report.Status = "failed"
		report.ErrorMessage = truncate(reconcileErr.Error(), 2000)
	} else {
		report.UserReport, _ = json.Marshal(userReport)
		report.GroupReport, _ = json.Marshal(groupReport)
	}

	if !*dryRun {
		pool, err := pgxpool.New(ctx, *dsn)
		if err != nil {
			return fail("connect postgres: " + err.Error())
		}
		defer pool.Close()
		repository := postgresscim.NewReconciliationReportRepository(pool)
		if err := repository.Create(ctx, report); err != nil {
			return fail("persist reconciliation report: " + err.Error())
		}
	}

	if reconcileErr != nil {
		fmt.Fprintf(os.Stderr, "SCIM reconciliation failed: %v\n", reconcileErr)
		return 2
	}

	summary := map[string]any{
		"reportId":       report.ID,
		"organizationId": report.OrganizationID,
		"connectorId":    report.ConnectorID,
		"status":         report.Status,
		"userReport":     userReport,
		"groupReport":    groupReport,
		"userDrift":      userReport.HasDrift(),
		"groupDrift":     groupReport.HasDrift(),
		"persisted":      !*dryRun,
		"startedAt":      report.StartedAt,
		"finishedAt":     report.FinishedAt,
	}
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(encoded))

	if userReport.HasDrift() || groupReport.HasDrift() {
		return 1
	}
	return 0
}

func listReports(dsn, orgID string, page, pageSize int, timeout time.Duration) int {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fail("connect postgres: " + err.Error())
	}
	defer pool.Close()
	repository := postgresscim.NewReconciliationReportRepository(pool)
	reports, total, err := repository.ListByOrganization(ctx, orgID, pageSize, (page-1)*pageSize)
	if err != nil {
		return fail("list reconciliation reports: " + err.Error())
	}
	views := make([]reportView, 0, len(reports))
	for _, report := range reports {
		views = append(views, reportView{
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
	encoded, _ := json.MarshalIndent(map[string]any{
		"organizationId": orgID,
		"page":           page,
		"pageSize":       pageSize,
		"total":          total,
		"reports":        views,
	}, "", "  ")
	fmt.Println(string(encoded))
	return 0
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func fail(message string) int {
	fmt.Fprintln(os.Stderr, "scim-reconcile: "+message)
	return 2
}
