package ldaps

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

func toListItem(l *domain.Ldap) LdapListItem {
	return LdapListItem{
		ID: l.ID, OrganizationID: l.OrganizationID, Name: l.Name, DisplayName: l.DisplayName,
		Host: l.Host, Port: l.Port, BaseDN: l.BaseDN, IsEnabled: l.IsEnabled,
		LastSyncAt: l.LastSyncAt, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

func toDetail(l *domain.Ldap) LdapDetail {
	return LdapDetail{
		LdapListItem:        toListItem(l),
		ServerName:          l.ServerName,
		Admin:               l.Admin,
		Type:                l.Type,
		AttributeName:       l.AttributeName,
		PasswordType:        l.PasswordType,
		Filter:              l.Filter,
		FilterFields:        l.FilterFields,
		EnableSsl:           l.EnableSsl,
		AllowSelfSignedCert: l.AllowSelfSignedCert,
		EnableGroups:        l.EnableGroups,
		DefaultGroups:       l.DefaultGroups,
		CustomAttributes:    l.CustomAttributes,
		SyncInterval:        l.SyncInterval,
		Revision:            l.Revision,
	}
}
