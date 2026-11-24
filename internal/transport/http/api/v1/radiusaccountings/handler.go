package radiusaccountings

import (
	"context"
	"time"

	appradiusaccountings "github.com/guliangiam/guliangiam/internal/app/radiusaccountings"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appradiusaccountings.Service
}

func NewHandler(s appradiusaccountings.Service) Handler {
	return Handler{Service: s}
}

type ListRadiusAccountingsResponse = presenter.ListResponse[appradiusaccountings.RadiusAccountingListItem]
type RadiusAccountingResponse = appradiusaccountings.RadiusAccountingDetail

type ListRadiusAccountingsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Username       string
}

type CreateRadiusAccountingRequest struct {
	OrganizationID     string    `json:"organizationId"`
	Name               string    `json:"name"`
	Username           string    `json:"username,omitempty"`
	ServiceType        int64     `json:"serviceType,omitempty"`
	NasID              string    `json:"nasId,omitempty"`
	NasIPAddr          string    `json:"nasIpAddr,omitempty"`
	NasPortID          string    `json:"nasPortId,omitempty"`
	NasPortType        int64     `json:"nasPortType,omitempty"`
	NasPort            int64     `json:"nasPort,omitempty"`
	FramedIPAddr       string    `json:"framedIpAddr,omitempty"`
	FramedIPNetmask    string    `json:"framedIpNetmask,omitempty"`
	AcctSessionID      string    `json:"acctSessionId,omitempty"`
	AcctSessionTime    int64     `json:"acctSessionTime,omitempty"`
	AcctInputTotal     int64     `json:"acctInputTotal,omitempty"`
	AcctOutputTotal    int64     `json:"acctOutputTotal,omitempty"`
	AcctInputPackets   int64     `json:"acctInputPackets,omitempty"`
	AcctOutputPackets  int64     `json:"acctOutputPackets,omitempty"`
	AcctTerminateCause int64     `json:"acctTerminateCause,omitempty"`
	AcctStartTime      time.Time `json:"acctStartTime"`
}

type PatchRadiusAccountingRequest struct {
	AcctSessionTime    *int64     `json:"acctSessionTime,omitempty"`
	AcctInputTotal     *int64     `json:"acctInputTotal,omitempty"`
	AcctOutputTotal    *int64     `json:"acctOutputTotal,omitempty"`
	AcctInputPackets   *int64     `json:"acctInputPackets,omitempty"`
	AcctOutputPackets  *int64     `json:"acctOutputPackets,omitempty"`
	AcctTerminateCause *int64     `json:"acctTerminateCause,omitempty"`
	AcctStopTime       *time.Time `json:"acctStopTime,omitempty"`
	Revision           int64      `json:"revision,omitempty"`
}

func (r CreateRadiusAccountingRequest) toCommand() appradiusaccountings.CreateRadiusAccountingCommand {
	return appradiusaccountings.CreateRadiusAccountingCommand{
		OrganizationID: r.OrganizationID, Name: r.Name, Username: r.Username,
		ServiceType: r.ServiceType, NasID: r.NasID, NasIPAddr: r.NasIPAddr,
		NasPortID: r.NasPortID, NasPortType: r.NasPortType, NasPort: r.NasPort,
		FramedIPAddr: r.FramedIPAddr, FramedIPNetmask: r.FramedIPNetmask,
		AcctSessionID: r.AcctSessionID, AcctSessionTime: r.AcctSessionTime,
		AcctInputTotal: r.AcctInputTotal, AcctOutputTotal: r.AcctOutputTotal,
		AcctInputPackets: r.AcctInputPackets, AcctOutputPackets: r.AcctOutputPackets,
		AcctTerminateCause: r.AcctTerminateCause, AcctStartTime: r.AcctStartTime,
	}
}

func (r PatchRadiusAccountingRequest) toCommand(id string) appradiusaccountings.PatchRadiusAccountingCommand {
	return appradiusaccountings.PatchRadiusAccountingCommand{
		ID: id, AcctSessionTime: r.AcctSessionTime, AcctInputTotal: r.AcctInputTotal,
		AcctOutputTotal: r.AcctOutputTotal, AcctInputPackets: r.AcctInputPackets,
		AcctOutputPackets: r.AcctOutputPackets, AcctTerminateCause: r.AcctTerminateCause,
		AcctStopTime: r.AcctStopTime, Revision: r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListRadiusAccountingsRequest) (ListRadiusAccountingsResponse, error) {
	items, total, err := h.Service.ListRadiusAccountings(ctx, appradiusaccountings.ListRadiusAccountingsQuery{Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID, Keyword: request.Keyword, Username: request.Username})
	if err != nil {
		return ListRadiusAccountingsResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*RadiusAccountingResponse, error) {
	response, err := h.Service.GetRadiusAccounting(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := RadiusAccountingResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateRadiusAccountingRequest) error {
	return h.Service.CreateRadiusAccounting(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchRadiusAccountingRequest) error {
	return h.Service.PatchRadiusAccounting(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteRadiusAccounting(ctx, organizationID, id)
}
