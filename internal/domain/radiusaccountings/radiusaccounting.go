package radiusaccountings

import "time"

type RadiusAccounting struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	Name               string     `json:"name"`
	Username           string     `json:"username,omitempty"`
	ServiceType        int64      `json:"serviceType"`
	NasID              string     `json:"nasId,omitempty"`
	NasIPAddr          string     `json:"nasIpAddr,omitempty"`
	NasPortID          string     `json:"nasPortId,omitempty"`
	NasPortType        int64      `json:"nasPortType"`
	NasPort            int64      `json:"nasPort"`
	FramedIPAddr       string     `json:"framedIpAddr,omitempty"`
	FramedIPNetmask    string     `json:"framedIpNetmask,omitempty"`
	AcctSessionID      string     `json:"acctSessionId,omitempty"`
	AcctSessionTime    int64      `json:"acctSessionTime"`
	AcctInputTotal     int64      `json:"acctInputTotal"`
	AcctOutputTotal    int64      `json:"acctOutputTotal"`
	AcctInputPackets   int64      `json:"acctInputPackets"`
	AcctOutputPackets  int64      `json:"acctOutputPackets"`
	AcctTerminateCause int64      `json:"acctTerminateCause"`
	AcctStartTime      time.Time  `json:"acctStartTime"`
	AcctStopTime       *time.Time `json:"acctStopTime,omitempty"`
	LastUpdate         time.Time  `json:"lastUpdate"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	Revision           int64      `json:"revision"`
}
