package ldaps

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

// ldapEncryptedMapper 负责 bind_password 列的 AES-256-GCM 加解密（DB-058 机密列）。
var ldapEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	ldapTable              = "guliangiam.ldaps"
	ldapColumnBindPassword = "bind_password"
)

func ldapAAD(ldapID string) crypto.AADContext {
	return crypto.AADContext{Table: ldapTable, Column: ldapColumnBindPassword, PKValue: ldapID}
}

// toLdap 把 MySQL 表扫描结果转成 domain.Ldap。JSON 列反序列化 + 密码解密。
func toLdap(row ldapRow) (*domain.Ldap, error) {
	l := &domain.Ldap{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		Name:                row.Name,
		DisplayName:         row.DisplayName,
		ServerName:          row.ServerName,
		Host:                row.Host,
		Port:                row.Port,
		Admin:               row.Admin,
		BindDN:              row.BindDN,
		BindPassword:        row.BindPassword,
		BaseDN:              row.BaseDN,
		Type:                row.Type,
		AttributeName:       row.AttributeName,
		PasswordType:        row.PasswordType,
		Filter:              row.Filter,
		EnableSsl:           row.EnableSsl,
		AllowSelfSignedCert: row.AllowSelfSignedCert,
		EnableGroups:        row.EnableGroups,
		IsEnabled:           row.IsEnabled,
		SyncInterval:        row.SyncInterval,
		LastSyncAt:          row.LastSyncAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
	}
	if len(row.FilterFields) > 0 {
		if err := mysqlshared.UnmarshalJSON(row.FilterFields, &l.FilterFields); err != nil {
			return nil, err
		}
	}
	if l.FilterFields == nil {
		l.FilterFields = []string{}
	}
	if len(row.DefaultGroups) > 0 {
		if err := mysqlshared.UnmarshalJSON(row.DefaultGroups, &l.DefaultGroups); err != nil {
			return nil, err
		}
	}
	if l.DefaultGroups == nil {
		l.DefaultGroups = []string{}
	}
	if err := mysqlshared.UnmarshalJSON(row.CustomAttributes, &l.CustomAttributes); err != nil {
		return nil, err
	}
	if l.CustomAttributes == nil {
		l.CustomAttributes = map[string]string{}
	}
	if err := decryptLdapSecrets(l); err != nil {
		return nil, err
	}
	return l, nil
}

// encryptLdapSecrets 加密 bind_password（写路径）。
func encryptLdapSecrets(l *domain.Ldap) error {
	if !ldapEncryptedMapper.IsEnabled() {
		return nil
	}
	enc, err := ldapEncryptedMapper.EncryptColumn(ldapAAD(l.ID), l.BindPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "encrypt bind_password")
	}
	l.BindPassword = enc
	return nil
}

// decryptLdapSecrets 解密 bind_password（读路径）。
func decryptLdapSecrets(l *domain.Ldap) error {
	if !ldapEncryptedMapper.IsEnabled() {
		return nil
	}
	dec, err := ldapEncryptedMapper.DecryptColumn(ldapAAD(l.ID), l.BindPassword)
	if err != nil {
		return apperrors.WrapInternalf(err, "decrypt bind_password")
	}
	l.BindPassword = dec
	return nil
}
