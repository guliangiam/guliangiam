package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const defaultConsistencyInterval = 24 * time.Hour

type ConsistencyDifference struct {
	OrganizationID string `json:"organizationId"`
	ProviderID     string `json:"providerId"`
	DetailID       string `json:"detailId,omitempty"`
	Family         string `json:"family"`
	Code           string `json:"code"`
	Expected       string `json:"expected,omitempty"`
	Actual         string `json:"actual,omitempty"`
}

type ConsistencyReport struct {
	RunID             string                  `json:"runId"`
	CheckedAt         time.Time               `json:"checkedAt"`
	RegistrationCount int                     `json:"registrationCount"`
	DetailCount       int                     `json:"detailCount"`
	DifferenceCount   int                     `json:"differenceCount"`
	Digest            string                  `json:"digest"`
	Differences       []ConsistencyDifference `json:"-"`
}

type ConsistencyWorker struct {
	Repository domain.ConsistencyRepository
	UOW        appuow.TxRunner
	Clock      clock.Clock
	UUID       platformuuid.Generator
	Interval   time.Duration
	Logger     *slog.Logger

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

func NewConsistencyWorker(
	repository domain.ConsistencyRepository,
	uow appuow.TxRunner,
	ck clock.Clock,
	uuid platformuuid.Generator,
	interval time.Duration,
	logger *slog.Logger,
) *ConsistencyWorker {
	if interval <= 0 {
		interval = defaultConsistencyInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ConsistencyWorker{
		Repository: repository,
		UOW:        uow,
		Clock:      ck,
		UUID:       uuid,
		Interval:   interval,
		Logger:     logger.With("component", "provider-consistency-worker"),
	}
}

func (w *ConsistencyWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.mu.Unlock()

	recover.SafeGo("provider-consistency-worker", func() {
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.Logger.Warn("provider consistency worker stopped", "error", err)
		}
	})
	w.Logger.Info("provider consistency worker started", "interval", w.Interval.String())
}

func (w *ConsistencyWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.Logger.Info("provider consistency worker stopped")
}

func (w *ConsistencyWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil {
				w.Logger.Error("provider consistency run failed", "error", err)
			}
		}
	}
}

func (w *ConsistencyWorker) RunOnce(ctx context.Context) (ConsistencyReport, error) {
	if w.Repository == nil || w.UOW == nil || w.Clock == nil || w.UUID == nil {
		return ConsistencyReport{}, apperrors.NewInternal("provider consistency dependencies are required")
	}
	registrations, err := w.Repository.ListConsistencyRegistrations(ctx)
	if err != nil {
		return ConsistencyReport{}, err
	}
	details := make(map[domain.Family][]domain.DetailConsistency, 3)
	detailCount := 0
	for _, family := range []domain.Family{domain.FamilyAI, domain.FamilyPayment, domain.FamilyNotification} {
		items, err := w.Repository.ListConsistencyDetails(ctx, family)
		if err != nil {
			return ConsistencyReport{}, err
		}
		details[family] = items
		detailCount += len(items)
	}

	differences := evaluateConsistency(registrations, details)
	runID, err := w.UUID.NewString()
	if err != nil {
		return ConsistencyReport{}, err
	}
	report := ConsistencyReport{
		RunID:             runID,
		CheckedAt:         w.Clock.Now().UTC(),
		RegistrationCount: len(registrations),
		DetailCount:       detailCount,
		DifferenceCount:   len(differences),
		Digest:            consistencyDigest(len(registrations), detailCount, differences),
		Differences:       differences,
	}
	events, err := w.consistencyEvents(report)
	if err != nil {
		return ConsistencyReport{}, err
	}
	if err := w.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Outbox().EnqueueBatch(ctx, events)
	}); err != nil {
		return ConsistencyReport{}, err
	}
	if report.DifferenceCount > 0 {
		w.Logger.Warn("provider consistency differences reported",
			"run_id", report.RunID, "differences", report.DifferenceCount, "digest", report.Digest)
	} else {
		w.Logger.Info("provider consistency run completed cleanly",
			"run_id", report.RunID, "registrations", report.RegistrationCount,
			"details", report.DetailCount, "digest", report.Digest)
	}
	return report, nil
}

func consistencyDigest(registrationCount, detailCount int, differences []ConsistencyDifference) string {
	payload, _ := json.Marshal(struct {
		RegistrationCount int                     `json:"registrationCount"`
		DetailCount       int                     `json:"detailCount"`
		Differences       []ConsistencyDifference `json:"differences"`
	}{registrationCount, detailCount, differences})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func evaluateConsistency(
	registrations []domain.RegistrationConsistency,
	details map[domain.Family][]domain.DetailConsistency,
) []ConsistencyDifference {
	differences := make([]ConsistencyDifference, 0)
	byID := make(map[string]domain.RegistrationConsistency, len(registrations))
	for i := range registrations {
		registration := registrations[i]
		byID[registration.ID] = registration
		if !registration.Classified {
			differences = append(differences, registrationDifference(registration, "registration_unclassified", "classified", "unclassified"))
		}
		if registration.Status != "" {
			expectedEnabled := registration.Status == domain.StatusActive && registration.DeletedAt == nil
			if registration.Enabled != expectedEnabled {
				differences = append(differences, registrationDifference(registration, "registration_enabled_mismatch", routableState(expectedEnabled), routableState(registration.Enabled)))
			}
		}
		if registration.Family == domain.FamilyIdentity {
			continue
		}
		if registration.Status == "" {
			differences = append(differences, registrationDifference(registration, "registration_status_missing", "present", "missing"))
		}
		if registration.DetailID == "" {
			differences = append(differences, registrationDifference(registration, "detail_metadata_missing", "present", "missing"))
		}
	}

	seenDetails := make(map[string]map[string]bool)
	for _, family := range []domain.Family{domain.FamilyAI, domain.FamilyPayment, domain.FamilyNotification} {
		for i := range details[family] {
			detail := details[family][i]
			if detail.ProviderID != "" {
				if seenDetails[detail.ProviderID] == nil {
					seenDetails[detail.ProviderID] = make(map[string]bool)
				}
				seenDetails[detail.ProviderID][detail.ID] = true
			}
			registration, ok := byID[detail.ProviderID]
			if detail.ProviderID == "" || !ok {
				differences = append(differences, detailDifference(registration, detail, family, "registration_not_found", "present", "missing"))
				continue
			}
			if registration.OrganizationID != detail.OrganizationID {
				differences = append(differences, detailDifference(registration, detail, family, "organization_mismatch", registration.OrganizationID, detail.OrganizationID))
				continue
			}
			if registration.Family != family {
				differences = append(differences, detailDifference(registration, detail, family, "family_mismatch", string(registration.Family), string(family)))
				continue
			}
			if family != domain.FamilyPayment && registration.DetailID != "" && registration.DetailID != detail.ID {
				differences = append(differences, detailDifference(registration, detail, family, "detail_anchor_mismatch", registration.DetailID, detail.ID))
			}
			if family == domain.FamilyAI {
				differences = appendTextDifference(differences, registration, detail, family, "display_name_mismatch", registration.DisplayName, detail.DisplayName)
			}
			if family == domain.FamilyAI || family == domain.FamilyNotification {
				differences = appendTextDifference(differences, registration, detail, family, "name_mismatch", registration.Name, detail.Name)
			}
			if !strings.EqualFold(string(registration.Kind), detail.Kind) {
				differences = append(differences, detailDifference(registration, detail, family, "kind_mismatch", string(registration.Kind), detail.Kind))
			}
			if !strings.EqualFold(string(registration.Variant), detail.Variant) {
				differences = append(differences, detailDifference(registration, detail, family, "variant_mismatch", string(registration.Variant), detail.Variant))
			}

			registrationStatus := registration.Status
			if registrationStatus == "" {
				registrationStatus = domain.StatusDisabled
				if registration.Enabled {
					registrationStatus = domain.StatusActive
				}
			}
			currentDetail := family != domain.FamilyPayment || registration.DetailID == detail.ID
			if currentDetail {
				switch family {
				case domain.FamilyAI:
					expected := registrationStatus
					if expected == domain.StatusDraft {
						expected = domain.StatusTesting
					}
					if string(expected) != detail.Status {
						differences = append(differences, detailDifference(registration, detail, family, "status_mismatch", string(expected), detail.Status))
					}
					if (registration.DeletedAt == nil) != (detail.DeletedAt == nil) {
						differences = append(differences, detailDifference(registration, detail, family, "deleted_at_mismatch", deletionState(registration.DeletedAt != nil), deletionState(detail.DeletedAt != nil)))
					}
				case domain.FamilyPayment:
					if string(registrationStatus) != detail.Status {
						differences = append(differences, detailDifference(registration, detail, family, "status_mismatch", string(registrationStatus), detail.Status))
					}
					if (registration.DeletedAt == nil) != (detail.DeletedAt == nil) {
						differences = append(differences, detailDifference(registration, detail, family, "deleted_at_mismatch", deletionState(registration.DeletedAt != nil), deletionState(detail.DeletedAt != nil)))
					}
				case domain.FamilyNotification:
					expectedRoutable := registrationStatus == domain.StatusActive && registration.DeletedAt == nil
					if expectedRoutable != detail.Enabled {
						if registration.DeletedAt != nil && detail.Enabled {
							differences = append(differences, detailDifference(registration, detail, family, "deleted_detail_routable", "not_routable", "routable"))
						} else {
							differences = append(differences, detailDifference(registration, detail, family, "status_mismatch", routableState(expectedRoutable), routableState(detail.Enabled)))
						}
					}
				case domain.FamilyIdentity:
					// identity 一致性由注册侧独立核对（下方 registration 循环对 identity 直接跳过）。
				}
			}
		}
	}

	for i := range registrations {
		registration := registrations[i]
		if registration.Family == domain.FamilyIdentity || registration.DetailID == "" {
			continue
		}
		if !seenDetails[registration.ID][registration.DetailID] {
			differences = append(differences, registrationDifference(registration, "detail_not_found", registration.DetailID, "missing"))
		}
	}

	sort.Slice(differences, func(i, j int) bool {
		a, b := differences[i], differences[j]
		left := strings.Join([]string{a.OrganizationID, a.ProviderID, a.DetailID, a.Family, a.Code, a.Expected, a.Actual}, "\x00")
		right := strings.Join([]string{b.OrganizationID, b.ProviderID, b.DetailID, b.Family, b.Code, b.Expected, b.Actual}, "\x00")
		return left < right
	})
	return differences
}

func registrationDifference(registration domain.RegistrationConsistency, code, expected, actual string) ConsistencyDifference {
	return ConsistencyDifference{
		OrganizationID: registration.OrganizationID,
		ProviderID:     registration.ID,
		DetailID:       registration.DetailID,
		Family:         string(registration.Family),
		Code:           code,
		Expected:       expected,
		Actual:         actual,
	}
}

func detailDifference(registration domain.RegistrationConsistency, detail domain.DetailConsistency, family domain.Family, code, expected, actual string) ConsistencyDifference {
	difference := ConsistencyDifference{
		OrganizationID: detail.OrganizationID,
		ProviderID:     detail.ProviderID,
		DetailID:       detail.ID,
		Family:         string(family),
		Code:           code,
		Expected:       expected,
		Actual:         actual,
	}
	if registration.ID != "" && code != "organization_mismatch" {
		difference.OrganizationID = registration.OrganizationID
	}
	if code == "organization_mismatch" {
		difference.OrganizationID = ""
	}
	return difference
}

func appendTextDifference(differences []ConsistencyDifference, registration domain.RegistrationConsistency, detail domain.DetailConsistency, family domain.Family, code, expected, actual string) []ConsistencyDifference {
	if expected == actual {
		return differences
	}
	return append(differences, detailDifference(registration, detail, family, code, expected, actual))
}

func deletionState(deleted bool) string {
	if deleted {
		return "deleted"
	}
	return "not_deleted"
}

func routableState(routable bool) string {
	if routable {
		return "routable"
	}
	return "not_routable"
}
