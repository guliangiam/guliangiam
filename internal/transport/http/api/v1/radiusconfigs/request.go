package radiusconfigs

import appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"

type ListRadiusConfigsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
	AuthProtocol   string
}

type CreateRadiusConfigRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	SharedSecret   string `json:"sharedSecret,omitempty"`
	AuthProtocol   string `json:"authProtocol"`
	IsEnabled      *bool  `json:"isEnabled,omitempty"`
}

type PatchRadiusConfigRequest struct {
	Name         *string `json:"name,omitempty"`
	Host         *string `json:"host,omitempty"`
	Port         *int    `json:"port,omitempty"`
	SharedSecret *string `json:"sharedSecret,omitempty"`
	AuthProtocol *string `json:"authProtocol,omitempty"`
	IsEnabled    *bool   `json:"isEnabled,omitempty"`
	Revision     int64   `json:"revision,omitempty"`
}

func (r CreateRadiusConfigRequest) toCommand() appradiusconfigs.CreateRadiusConfigCommand {
	return appradiusconfigs.CreateRadiusConfigCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		Host:           r.Host,
		Port:           r.Port,
		SharedSecret:   r.SharedSecret,
		AuthProtocol:   r.AuthProtocol,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchRadiusConfigRequest) toCommand(id string) appradiusconfigs.PatchRadiusConfigCommand {
	return appradiusconfigs.PatchRadiusConfigCommand{
		ID:           id,
		Name:         r.Name,
		Host:         r.Host,
		Port:         r.Port,
		SharedSecret: r.SharedSecret,
		AuthProtocol: r.AuthProtocol,
		IsEnabled:    r.IsEnabled,
		Revision:     r.Revision,
	}
}
