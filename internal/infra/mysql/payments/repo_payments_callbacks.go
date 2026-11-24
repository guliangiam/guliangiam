package payments

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── Payments 表扩展查询 ────────────────────────────────────────────────

func (r Repository) GetByChannelOrderNo(ctx context.Context, channel, orderNo string) (*domain.Payment, error) {
	p, err := scanPaymentExtended(r.db.QueryRowContext(ctx, queryGetByChannelOrderNo, channel, orderNo))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found by channel order")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) GetByOutTradeNo(ctx context.Context, channel, outTradeNo string) (*domain.Payment, error) {
	p, err := scanPaymentExtended(r.db.QueryRowContext(ctx, queryGetByOutTradeNo, channel, outTradeNo))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found by out trade no")
		}
		return nil, err
	}
	return p, nil
}

// ─── CallbackReceiptsRepo ────────────────────────────────────────────────

type CallbackReceiptsRepository struct{ db mysqlshared.DBTX }

func NewCallbackReceiptsRepository(db mysqlshared.DBTX) CallbackReceiptsRepository {
	return CallbackReceiptsRepository{db: db}
}

var _ domain.CallbackReceiptsRepo = CallbackReceiptsRepository{}

func (r CallbackReceiptsRepository) Create(ctx context.Context, rec *domain.CallbackReceipt) error {
	var headersStr sql.NullString
	if rec.Headers != nil {
		b, err := json.Marshal(rec.Headers)
		if err != nil {
			return err
		}
		headersStr = sql.NullString{String: string(b), Valid: true}
	}
	var eventKey sql.NullString
	if rec.EventKey != nil {
		eventKey = sql.NullString{String: *rec.EventKey, Valid: true}
	}
	var outTradeNo sql.NullString
	if rec.OutTradeNo != nil {
		outTradeNo = sql.NullString{String: *rec.OutTradeNo, Valid: true}
	}
	var rawEventStr sql.NullString
	if rec.RawEvent != nil {
		b, err := json.Marshal(rec.RawEvent)
		if err != nil {
			return err
		}
		rawEventStr = sql.NullString{String: string(b), Valid: true}
	}
	var clientIP sql.NullString
	if rec.ClientIP != nil {
		clientIP = sql.NullString{String: *rec.ClientIP, Valid: true}
	}
	var errorMessage sql.NullString
	if rec.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *rec.ErrorMessage, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, receiptInsert,
		rec.ID, rec.OrganizationID, rec.Channel, rec.PayloadSHA256,
		rec.Payload, headersStr, rec.ReceivedAt, clientIP,
		rec.ProcessState, eventKey, outTradeNo, rawEventStr, errorMessage,
		rec.CreatedAt, rec.UpdatedAt, rec.Revision,
	)
	return err
}

func (r CallbackReceiptsRepository) GetByPayloadHash(ctx context.Context, channel, sha256 string) (*domain.CallbackReceipt, error) {
	rec, err := scanCallbackReceipt(r.db.QueryRowContext(ctx, receiptGetByPayloadSHA256, channel, sha256))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("callback receipt not found by payload hash")
		}
		return nil, err
	}
	return rec, nil
}

func (r CallbackReceiptsRepository) GetByEventKey(ctx context.Context, channel, eventKey string) (*domain.CallbackReceipt, error) {
	rec, err := scanCallbackReceipt(r.db.QueryRowContext(ctx, receiptGetByEventKey, channel, eventKey))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("callback receipt not found by event key")
		}
		return nil, err
	}
	return rec, nil
}

func (r CallbackReceiptsRepository) Update(ctx context.Context, rec *domain.CallbackReceipt) error {
	var rawEventStr sql.NullString
	if rec.RawEvent != nil {
		b, err := json.Marshal(rec.RawEvent)
		if err != nil {
			return err
		}
		rawEventStr = sql.NullString{String: string(b), Valid: true}
	}
	var eventKey sql.NullString
	if rec.EventKey != nil {
		eventKey = sql.NullString{String: *rec.EventKey, Valid: true}
	}
	var outTradeNo sql.NullString
	if rec.OutTradeNo != nil {
		outTradeNo = sql.NullString{String: *rec.OutTradeNo, Valid: true}
	}
	var errorMessage sql.NullString
	if rec.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *rec.ErrorMessage, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, receiptUpdate,
		rec.ProcessState, eventKey, outTradeNo, rawEventStr,
		errorMessage, rec.UpdatedAt, rec.ID,
	)
	return err
}

func (r CallbackReceiptsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.CallbackReceipt, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, receiptCount, organizationID, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, receiptList, organizationID, organizationID, mysqlshared.ClampPageSize(limit), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []*domain.CallbackReceipt
	for rows.Next() {
		rec, err := scanCallbackReceipt(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, rec)
	}
	return items, total, rows.Err()
}
