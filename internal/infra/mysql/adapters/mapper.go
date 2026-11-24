package adapters

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
)

// rowToDomain 把 MySQL 表扫描结果转成 domain.Adapter。
// 若加密启用，password 和 db_password 列先解密再赋值。
func rowToDomain(row adapterRow) (*domain.Adapter, error) {
	a := &domain.Adapter{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		Type: row.Type, Server: row.Server, Database: row.Database,
		TableName: row.TableName, Username: row.Username, Password: row.Password,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		Owner: row.Owner, UseSameDb: row.UseSameDb, DatabaseType: row.DatabaseType,
		Host: row.Host, Port: row.Port, DbUser: row.DbUser, DbPassword: row.DbPassword,
		DatabaseName: row.DatabaseName, Policy: row.Policy, IsEnabled: row.IsEnabled,
	}
	if !adapterEncryptedMapper.IsEnabled() {
		return a, nil
	}
	password, err := adapterEncryptedMapper.DecryptColumn(adapterAAD(a.ID, adapterColumnPassword), a.Password)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt adapter password")
	}
	dbPassword, err := adapterEncryptedMapper.DecryptColumn(adapterAAD(a.ID, adapterColumnDbPassword), a.DbPassword)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt adapter db_password")
	}
	a.Password = password
	a.DbPassword = dbPassword
	return a, nil
}

// adapterEncryptedMapper 代理 platform/crypto.GlobalFieldEncryptor()，
// 负责 password 和 db_password 列的 AES-256-GCM 加解密（DB-058 机密列）。
var adapterEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	adapterTable            = "guliangiam.adapters"
	adapterColumnPassword   = "password"
	adapterColumnDbPassword = "db_password"
)

func adapterAAD(adapterID, column string) crypto.AADContext {
	return crypto.AADContext{Table: adapterTable, Column: column, PKValue: adapterID}
}

// adapterSecretsForWrite 返回持久化机密列所需的值，不改 domain 对象。
func adapterSecretsForWrite(a *domain.Adapter) (password, dbPassword string, err error) {
	password, dbPassword = a.Password, a.DbPassword
	if !adapterEncryptedMapper.IsEnabled() {
		return password, dbPassword, nil
	}
	password, err = adapterEncryptedMapper.EncryptColumn(adapterAAD(a.ID, adapterColumnPassword), a.Password)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt adapter password")
	}
	dbPassword, err = adapterEncryptedMapper.EncryptColumn(adapterAAD(a.ID, adapterColumnDbPassword), a.DbPassword)
	if err != nil {
		return "", "", apperrors.WrapInternalf(err, "encrypt adapter db_password")
	}
	return password, dbPassword, nil
}
