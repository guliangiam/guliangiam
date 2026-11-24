package transactions

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 transactions 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 Transaction（系统路径：退款/用户子资源）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return r.getByIDWithQuery(ctx, queryGetByID, id)
}

// GetByIDForOrganization 只在 Transaction 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Transaction, error) {
	return r.getByIDWithQuery(ctx, queryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDWithQuery(ctx context.Context, query string, args ...any) (*domain.Transaction, error) {
	t, err := scanTransaction(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("transaction not found")
		}
		return nil, err
	}
	return t, nil
}

// List 分页列出 Transaction。
// countArgs: org×2 + keyword×3 + user×2 + order×2 + status×2 + type×2 = 13
// listArgs: countArgs + 2 = 15
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Transaction, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	statusVal := ""
	if q.Status != nil {
		statusVal = *q.Status
	}
	typeVal := ""
	if q.TransactionType != nil {
		typeVal = *q.TransactionType
	}

	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.UserID, q.UserID,
		q.OrderID, q.OrderID,
		statusVal, statusVal,
		typeVal, typeVal,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountTransactions, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, queryListTransactions, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return items, total, nil
}

// Create 插入 Transaction。MySQL 版 nullable UUID 直接用 *string。
func (r Repository) Create(ctx context.Context, tx *domain.Transaction) error {
	var metadataStr string
	if len(tx.Metadata) > 0 {
		metadataStr = string(tx.Metadata)
	} else {
		metadataStr = `{}`
	}

	_, err := r.db.ExecContext(ctx, queryInsertTransaction,
		tx.ID, tx.OrganizationID,
		nilIfEmpty(tx.UserID), nilIfEmpty(tx.OrderID), nilIfEmpty(tx.PaymentID),
		tx.TransactionType, tx.Amount, tx.Currency, tx.Status, tx.ProviderType,
		nilIfEmpty(tx.ProviderID), metadataStr,
		tx.CreatedAt, tx.UpdatedAt, tx.Revision,
	)
	return err
}

// Update 更新 Transaction（带租户谓词）。
func (r Repository) Update(ctx context.Context, tx *domain.Transaction) error {
	var metadataStr string
	if len(tx.Metadata) > 0 {
		metadataStr = string(tx.Metadata)
	} else {
		metadataStr = `{}`
	}
	res, err := r.db.ExecContext(ctx, queryUpdateTransaction,
		tx.Status, metadataStr, tx.UpdatedAt,
		tx.ID, tx.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("transaction not found")
	}
	return nil
}

// Delete 只在 Transaction 属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteTransaction, id, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("transaction not found")
	}
	return nil
}

// scanTransaction 扫描一行到 domain.Transaction。
// MySQL nullable UUID/TEXT 用 sql.NullString，metadata JSON 用 string。
func scanTransaction(sc interface {
	Scan(...interface{}) error
}) (*domain.Transaction, error) {
	var t domain.Transaction
	var userID, orderID, paymentID, providerID sql.NullString
	var metadataStr string
	err := sc.Scan(
		&t.ID, &t.OrganizationID,
		&userID, &orderID, &paymentID,
		&t.TransactionType, &t.Amount, &t.Currency, &t.Status, &t.ProviderType,
		&providerID, &metadataStr,
		&t.CreatedAt, &t.UpdatedAt, &t.Revision,
	)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		t.UserID = userID.String
	}
	if orderID.Valid {
		t.OrderID = orderID.String
	}
	if paymentID.Valid {
		t.PaymentID = paymentID.String
	}
	if providerID.Valid {
		t.ProviderID = providerID.String
	}
	if metadataStr != "" && metadataStr != "null" && metadataStr != "{}" {
		t.Metadata = json.RawMessage(metadataStr)
	} else if metadataStr == "{}" {
		t.Metadata = json.RawMessage(`{}`)
	}
	return &t, nil
}

// nilIfEmpty 返回空字符串的 nil 指针或原值指针。
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// 编译期接口断言
var _ domain.Repository = Repository{}
