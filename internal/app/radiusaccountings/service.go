package radiusaccountings

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.InfraUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.InfraUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListRadiusAccountingsQuery struct {
	Page, PageSize                    int
	OrganizationID, Keyword, Username string
}
type CreateRadiusAccountingCommand struct {
	OrganizationID, Name, Username                                                                                                               string
	ServiceType, NasPortType, NasPort, AcctSessionTime, AcctInputTotal, AcctOutputTotal, AcctInputPackets, AcctOutputPackets, AcctTerminateCause int64
	NasID, NasIPAddr, NasPortID, FramedIPAddr, FramedIPNetmask, AcctSessionID                                                                    string
	AcctStartTime                                                                                                                                time.Time
}
type PatchRadiusAccountingCommand struct {
	ID                                                                                                        string
	AcctSessionTime, AcctInputTotal, AcctOutputTotal, AcctInputPackets, AcctOutputPackets, AcctTerminateCause *int64
	AcctStopTime                                                                                              *time.Time
	Revision                                                                                                  int64
}

type RadiusAccountingListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	Username       string    `json:"username,omitempty"`
	AcctSessionID  string    `json:"acctSessionId,omitempty"`
	AcctStartTime  time.Time `json:"acctStartTime"`
	CreatedAt      time.Time `json:"createdAt"`
}
type RadiusAccountingDetail struct {
	RadiusAccountingListItem
	ServiceType        int64      `json:"serviceType"`
	NasID              string     `json:"nasId,omitempty"`
	NasIPAddr          string     `json:"nasIpAddr,omitempty"`
	AcctSessionTime    int64      `json:"acctSessionTime"`
	AcctInputTotal     int64      `json:"acctInputTotal"`
	AcctOutputTotal    int64      `json:"acctOutputTotal"`
	AcctTerminateCause int64      `json:"acctTerminateCause"`
	AcctStopTime       *time.Time `json:"acctStopTime,omitempty"`
	Revision           int64      `json:"revision"`
}

func toListItem(r *domain.RadiusAccounting) RadiusAccountingListItem {
	return RadiusAccountingListItem{ID: r.ID, OrganizationID: r.OrganizationID, Name: r.Name,
		Username: r.Username, AcctSessionID: r.AcctSessionID, AcctStartTime: r.AcctStartTime, CreatedAt: r.CreatedAt}
}
func toDetail(r *domain.RadiusAccounting) RadiusAccountingDetail {
	return RadiusAccountingDetail{RadiusAccountingListItem: toListItem(r), ServiceType: r.ServiceType,
		NasID: r.NasID, NasIPAddr: r.NasIPAddr, AcctSessionTime: r.AcctSessionTime,
		AcctInputTotal: r.AcctInputTotal, AcctOutputTotal: r.AcctOutputTotal,
		AcctTerminateCause: r.AcctTerminateCause, AcctStopTime: r.AcctStopTime, Revision: r.Revision}
}

func (s Service) ListRadiusAccountings(ctx context.Context, q ListRadiusAccountingsQuery) ([]RadiusAccountingListItem, int64, error) {
	items, total, err := s.UOW.RadiusAccountings().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, Username: q.Username})
	if err != nil {
		return nil, 0, err
	}
	r := make([]RadiusAccountingListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetRadiusAccounting(ctx context.Context, organizationID, id string) (*RadiusAccountingDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	ra, err := s.UOW.RadiusAccountings().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(ra)
	return &d, nil
}
func (s Service) CreateRadiusAccounting(ctx context.Context, c CreateRadiusAccountingCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	ra := &domain.RadiusAccounting{ID: id, OrganizationID: c.OrganizationID, Name: c.Name,
		Username: c.Username, ServiceType: c.ServiceType, NasID: c.NasID, NasIPAddr: c.NasIPAddr,
		NasPortID: c.NasPortID, NasPortType: c.NasPortType, NasPort: c.NasPort,
		FramedIPAddr: c.FramedIPAddr, FramedIPNetmask: c.FramedIPNetmask,
		AcctSessionID: c.AcctSessionID, AcctSessionTime: c.AcctSessionTime,
		AcctInputTotal: c.AcctInputTotal, AcctOutputTotal: c.AcctOutputTotal,
		AcctInputPackets: c.AcctInputPackets, AcctOutputPackets: c.AcctOutputPackets,
		AcctTerminateCause: c.AcctTerminateCause, AcctStartTime: c.AcctStartTime,
		LastUpdate: now, CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.RadiusAccountings().Create(ctx, ra); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			ra.OrganizationID, "",
			"radiusaccountings.create", "radiusaccountings", ra.ID,
			"radiusaccountings", "radiusaccountings.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchRadiusAccounting(ctx context.Context, organizationID string, c PatchRadiusAccountingCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		ra, err := tx.RadiusAccountings().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && ra.Revision != c.Revision {
			return apperrors.NewConflict("radius accounting revision mismatch")
		}
		if c.AcctSessionTime != nil {
			ra.AcctSessionTime = *c.AcctSessionTime
		}
		if c.AcctInputTotal != nil {
			ra.AcctInputTotal = *c.AcctInputTotal
		}
		if c.AcctOutputTotal != nil {
			ra.AcctOutputTotal = *c.AcctOutputTotal
		}
		if c.AcctInputPackets != nil {
			ra.AcctInputPackets = *c.AcctInputPackets
		}
		if c.AcctOutputPackets != nil {
			ra.AcctOutputPackets = *c.AcctOutputPackets
		}
		if c.AcctTerminateCause != nil {
			ra.AcctTerminateCause = *c.AcctTerminateCause
		}
		if c.AcctStopTime != nil {
			ra.AcctStopTime = c.AcctStopTime
		}
		ra.LastUpdate = s.Clock.Now()
		ra.UpdatedAt = s.Clock.Now()
		ra.Revision++
		if err := tx.RadiusAccountings().Update(ctx, ra); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			ra.OrganizationID, "",
			"radiusaccountings.update", "radiusaccountings", ra.ID,
			"radiusaccountings", "radiusaccountings.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteRadiusAccounting(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		ra, err := tx.RadiusAccountings().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.RadiusAccountings().Delete(ctx, id, ra.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			ra.OrganizationID, "",
			"radiusaccountings.delete", "radiusaccountings", id,
			"radiusaccountings", "radiusaccountings.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
