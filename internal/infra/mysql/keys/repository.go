package keys

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 keys 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id + organization_id 查 Key；跨租户按不存在处理。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Key, error) {
	var k domain.Key
	var privateKey sql.NullString
	var expiresAt sql.NullTime
	err := r.db.QueryRowContext(ctx, queryGetKeyByID, id, organizationID).Scan(
		&k.ID, &k.OrganizationID, &k.Name, &k.DisplayName, &k.KeyType, &k.PublicKey,
		&privateKey, &k.Algorithm, &expiresAt, &k.IsEnabled, &k.CreatedAt, &k.UpdatedAt, &k.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("key not found")
		}
		return nil, err
	}
	if privateKey.Valid {
		opened, err := openKeyPrivateKey(k.ID, privateKey.String)
		if err != nil {
			return nil, err
		}
		k.PrivateKey = opened
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		k.ExpiresAt = &t
	}
	return &k, nil
}

// List 分页列出 Key。MySQL 版需要手动展开参数重复占位符。
// countArgs: org_id×2 + keyword×3 + key_type×2 + is_enabled×2 = 9
// listArgs: 同上 + limit + offset = 11
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Key, int64, error) {
	var keyTypeArg interface{}
	if q.KeyType != nil {
		keyTypeArg = *q.KeyType
	} // 否则传 nil
	var enabledArg interface{}
	if q.IsEnabled != nil {
		enabledArg = *q.IsEnabled
	} // 否则传 nil

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		keyTypeArg, keyTypeArg,
		enabledArg, enabledArg,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountKeys, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListKeys, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Key, 0)
	for rows.Next() {
		var k domain.Key
		var privateKey sql.NullString
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&k.ID, &k.OrganizationID, &k.Name, &k.DisplayName, &k.KeyType, &k.PublicKey,
			&privateKey, &k.Algorithm, &expiresAt, &k.IsEnabled, &k.CreatedAt, &k.UpdatedAt, &k.Revision,
		); err != nil {
			return nil, 0, err
		}
		if privateKey.Valid {
			opened, err := openKeyPrivateKey(k.ID, privateKey.String)
			if err != nil {
				return nil, 0, err
			}
			k.PrivateKey = opened
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			k.ExpiresAt = &t
		}
		items = append(items, &k)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 Key。private_key nullable → sql.NullString；密文由 seal 生成。
func (r Repository) Create(ctx context.Context, k *domain.Key) error {
	privateKey, err := keyPrivateKeyForWrite(k)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertKey,
		k.ID, k.OrganizationID, k.Name, k.DisplayName, k.KeyType, k.PublicKey,
		privateKey, k.Algorithm, k.ExpiresAt, k.IsEnabled, k.CreatedAt, k.UpdatedAt, k.Revision,
	)
	return err
}

// Update 更新 Key。PG 版没有 RETURNING，MySQL 直接 ExecContext。
func (r Repository) Update(ctx context.Context, k *domain.Key) error {
	privateKey, err := keyPrivateKeyForWrite(k)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryUpdateKey,
		k.DisplayName, k.PublicKey, privateKey, k.Algorithm, k.ExpiresAt,
		k.IsEnabled, k.UpdatedAt, k.Revision, k.ID, k.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("key not found")
	}
	return nil
}

// Delete 按 id + organization_id 删除 Key；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteKey, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("key not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
