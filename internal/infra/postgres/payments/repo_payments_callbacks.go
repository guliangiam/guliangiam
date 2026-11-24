package payments

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ─── Payments 表扩展查询 ────────────────────────────────────────────────
// scanPayment (mapper.go) already covers the full QueryPaymentColumns projection,
// including the settlement columns these lookups need.

func (r Repository) GetByChannelOrderNo(ctx context.Context, channel, orderNo string) (*domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, QueryGetByChannelOrderNo, channel, orderNo))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found by channel order")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) GetByOutTradeNo(ctx context.Context, channel, outTradeNo string) (*domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, QueryGetByOutTradeNo, channel, outTradeNo))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found by out trade no")
		}
		return nil, err
	}
	return p, nil
}

// ─── CallbackReceiptsRepo ────────────────────────────────────────────────

type CallbackReceiptsRepository struct{ db shared.DBTX }

func NewCallbackReceiptsRepository(db shared.DBTX) CallbackReceiptsRepository {
	return CallbackReceiptsRepository{db: db}
}

var _ domain.CallbackReceiptsRepo = CallbackReceiptsRepository{}

func (r CallbackReceiptsRepository) Create(ctx context.Context, rec *domain.CallbackReceipt) error {
	var headers, rawEvent *[]byte
	if rec.Headers != nil {
		b, err := json.Marshal(rec.Headers)
		if err != nil {
			return err
		}
		headers = &b
	}
	if rec.RawEvent != nil {
		b, err := json.Marshal(rec.RawEvent)
		if err != nil {
			return err
		}
		rawEvent = &b
	}
	var eventKey, outTradeNo *string
	if rec.EventKey != nil {
		s := *rec.EventKey
		eventKey = &s
	}
	if rec.OutTradeNo != nil {
		s := *rec.OutTradeNo
		outTradeNo = &s
	}
	_, err := r.db.Exec(ctx, ReceiptInsert,
		rec.ID, rec.OrganizationID, rec.Channel, rec.PayloadSHA256,
		rec.Payload, headers, rec.ReceivedAt, rec.ClientIP,
		rec.ProcessState, eventKey, outTradeNo, rawEvent, rec.ErrorMessage,
		rec.CreatedAt, rec.UpdatedAt, rec.Revision,
	)
	return err
}

func (r CallbackReceiptsRepository) scan(sc interface{ Scan(...interface{}) error }) (*domain.CallbackReceipt, error) {
	var rec domain.CallbackReceipt
	var payload []byte
	var headersBytes, rawEventBytes *[]byte
	var eventKey, outTradeNo *string
	if err := sc.Scan(
		&rec.ID, &rec.OrganizationID, &rec.Channel, &rec.PayloadSHA256,
		&payload, &headersBytes, &rec.ReceivedAt, &rec.ClientIP,
		&rec.ProcessState, &eventKey, &outTradeNo, &rawEventBytes,
		&rec.ErrorMessage, &rec.CreatedAt, &rec.UpdatedAt, &rec.Revision,
	); err != nil {
		return nil, err
	}
	rec.Payload = payload
	if eventKey != nil {
		s := *eventKey
		rec.EventKey = &s
	}
	if outTradeNo != nil {
		s := *outTradeNo
		rec.OutTradeNo = &s
	}
	if headersBytes != nil {
		_ = json.Unmarshal(*headersBytes, &rec.Headers)
	}
	if rawEventBytes != nil {
		_ = json.Unmarshal(*rawEventBytes, &rec.RawEvent)
	}
	return &rec, nil
}

func (r CallbackReceiptsRepository) GetByPayloadHash(ctx context.Context, channel, sha256 string) (*domain.CallbackReceipt, error) {
	rec, err := r.scan(r.db.QueryRow(ctx, ReceiptGetByPayloadSHA256, channel, sha256))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("callback receipt not found by payload hash")
		}
		return nil, err
	}
	return rec, nil
}

func (r CallbackReceiptsRepository) GetByEventKey(ctx context.Context, channel, eventKey string) (*domain.CallbackReceipt, error) {
	rec, err := r.scan(r.db.QueryRow(ctx, ReceiptGetByEventKey, channel, eventKey))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("callback receipt not found by event key")
		}
		return nil, err
	}
	return rec, nil
}

func (r CallbackReceiptsRepository) Update(ctx context.Context, rec *domain.CallbackReceipt) error {
	var rawEventBytes *[]byte
	if rec.RawEvent != nil {
		b, _ := json.Marshal(rec.RawEvent)
		rawEventBytes = &b
	}
	var eventKey, outTradeNo *string
	if rec.EventKey != nil {
		s := *rec.EventKey
		eventKey = &s
	}
	if rec.OutTradeNo != nil {
		s := *rec.OutTradeNo
		outTradeNo = &s
	}
	_, err := r.db.Exec(ctx, ReceiptUpdate,
		rec.ProcessState, eventKey, outTradeNo, rawEventBytes,
		rec.ErrorMessage, rec.UpdatedAt, rec.ID,
	)
	return err
}

func (r CallbackReceiptsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.CallbackReceipt, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, ReceiptCount, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, ReceiptList, organizationID, shared.ClampPageSize(limit), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.CallbackReceipt, 0, shared.ClampPageSize(limit))
	for rows.Next() {
		rec, err := r.scan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, rec)
	}
	return items, total, rows.Err()
}
