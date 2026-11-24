package payments

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ─── RefundsRepo ────────────────────────────────────────────────────────

type RefundsRepository struct{ db shared.DBTX }

func NewRefundsRepository(db shared.DBTX) RefundsRepository {
	return RefundsRepository{db: db}
}

var _ domain.RefundsRepo = RefundsRepository{}

func (r RefundsRepository) scanRefund(sc interface{ Scan(...interface{}) error }) (*domain.Refund, error) {
	var ref domain.Refund
	err := sc.Scan(
		&ref.ID, &ref.OrganizationID, &ref.PaymentID, &ref.OutRefundNo,
		&ref.Channel, &ref.Amount, &ref.Currency, &ref.Reason,
		&ref.State, &ref.ChannelRefundNo, &ref.ErrorCode, &ref.ErrorMessage,
		&ref.RequestedAt, &ref.CompletedAt, nil, nil, nil,
	)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r RefundsRepository) Create(ctx context.Context, ref *domain.Refund) error {
	var errCode, errMsg *string
	if ref.ErrorCode != nil {
		s := *ref.ErrorCode
		errCode = &s
	}
	if ref.ErrorMessage != nil {
		s := *ref.ErrorMessage
		errMsg = &s
	}
	_, err := r.db.Exec(ctx, RefundInsert,
		ref.ID, ref.OrganizationID, ref.PaymentID, ref.OutRefundNo,
		ref.Channel, ref.Amount, ref.Currency, ref.Reason, ref.State,
		ref.ChannelRefundNo, errCode, errMsg,
		ref.RequestedAt, ref.CompletedAt, ref.CreatedAt, ref.UpdatedAt, ref.Revision,
	)
	return err
}

func (r RefundsRepository) GetByID(ctx context.Context, id string) (*domain.Refund, error) {
	ref, err := r.scanRefund(r.db.QueryRow(ctx, RefundGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Refund, error) {
	ref, err := r.scanRefund(r.db.QueryRow(ctx, RefundGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) GetByOutRefundNo(ctx context.Context, channel, outRefundNo string) (*domain.Refund, error) {
	ref, err := r.scanRefund(r.db.QueryRow(ctx, RefundGetByOutRefundNo, channel, outRefundNo))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found by channel refund no")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) Update(ctx context.Context, ref *domain.Refund) error {
	_, err := r.db.Exec(ctx, RefundUpdate,
		ref.ChannelRefundNo, ref.State,
		ref.ErrorCode, ref.ErrorMessage, ref.CompletedAt,
		ref.UpdatedAt, ref.ID,
	)
	return err
}

func (r RefundsRepository) ListByPayment(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	return r.list(ctx, RefundListByPayment, paymentID)
}

func (r RefundsRepository) ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*domain.Refund, error) {
	return r.list(ctx, RefundListByPaymentForOrganization, paymentID, organizationID)
}

func (r RefundsRepository) list(ctx context.Context, query string, args ...any) ([]*domain.Refund, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.Refund
	for rows.Next() {
		ref, err := r.scanRefund(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, ref)
	}
	return items, rows.Err()
}

func (r RefundsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Refund, int64, error) {
	// 空壳占位。
	return nil, 0, nil
}

// LockPaymentForRefund 用 SELECT FOR UPDATE 锁住 payment 行，并派生已退/在途金额。
func (r RefundsRepository) LockPaymentForRefund(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
	var p domain.LockedPayment
	err := r.db.QueryRow(ctx, RefundLockPayment, paymentID).Scan(
		&p.RefundedAmount, &p.RefundingAmount, &p.Price, &p.Status,
		&p.Provider, &p.OutTradeNo, &p.Currency,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found or not settled")
		}
		return nil, err
	}
	return &p, nil
}

// ─── AttemptsRepo — 空壳占位 ──────────────────────────────────────────

type AttemptsRepository struct{ db shared.DBTX }

func NewAttemptsRepository(db shared.DBTX) AttemptsRepository { return AttemptsRepository{db: db} }

var _ domain.AttemptsRepo = AttemptsRepository{}

// scanPayAttempt scans a pay_attempts row into domain.PayAttempt.
// SELECT 列顺序: id, organization_id, payment_id, refund_id, channel,
// attempt_type, attempt_no, channel_order_no, channel_request_id,
// state, amount, currency, error_code, error_message, latency_ms,
// occurred_at, raw_response (jsonb).
func scanPayAttempt(sc interface{ Scan(...interface{}) error }) (*domain.PayAttempt, error) {
	var a domain.PayAttempt
	var rawResponseBytes []byte
	var stateStr string
	err := sc.Scan(
		&a.ID, &a.OrganizationID, &a.PaymentID, &a.RefundID, &a.Channel,
		&a.AttemptType, &a.AttemptNo, &a.ChannelOrderNo, &a.ChannelRequestID,
		&stateStr, &a.Amount, &a.Currency, &a.ErrorCode, &a.ErrorMessage, &a.LatencyMs,
		&a.OccurredAt, &rawResponseBytes,
	)
	if err != nil {
		return nil, err
	}
	a.State = domain.AttemptState(stateStr)
	if len(rawResponseBytes) > 0 && string(rawResponseBytes) != jsonNullLiteral {
		if err := json.Unmarshal(rawResponseBytes, &a.RawResponse); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal raw_response")
		}
	}
	if a.RawResponse == nil {
		a.RawResponse = map[string]any{}
	}
	return &a, nil
}

func (r AttemptsRepository) GetByID(ctx context.Context, id string) (*domain.PayAttempt, error) {
	a, err := scanPayAttempt(r.db.QueryRow(ctx, AttemptGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("attempt not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AttemptsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.PayAttempt, error) {
	a, err := scanPayAttempt(r.db.QueryRow(ctx, AttemptGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("attempt not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AttemptsRepository) ListByPayment(ctx context.Context, paymentID string) ([]*domain.PayAttempt, error) {
	return r.list(ctx, AttemptListByPayment, paymentID)
}

func (r AttemptsRepository) ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*domain.PayAttempt, error) {
	return r.list(ctx, AttemptListByPaymentForOrganization, paymentID, organizationID)
}

func (r AttemptsRepository) list(ctx context.Context, query string, args ...any) ([]*domain.PayAttempt, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.PayAttempt
	for rows.Next() {
		a, err := scanPayAttempt(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r AttemptsRepository) ListByRefund(ctx context.Context, refundID string) ([]*domain.PayAttempt, error) {
	rows, err := r.db.Query(ctx, AttemptListByRefund, refundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.PayAttempt
	for rows.Next() {
		a, err := scanPayAttempt(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r AttemptsRepository) Create(ctx context.Context, a *domain.PayAttempt) error {
	var rawBytes []byte
	if len(a.RawResponse) > 0 {
		var err error
		rawBytes, err = json.Marshal(a.RawResponse)
		if err != nil {
			return apperrors.WrapInternalf(err, "marshal raw_response")
		}
	}
	_, err := r.db.Exec(ctx, AttemptInsert,
		a.ID, a.OrganizationID, a.PaymentID, a.RefundID, a.Channel,
		a.AttemptType, a.AttemptNo, a.ChannelOrderNo, a.ChannelRequestID,
		string(a.State), a.Amount, a.Currency, a.ErrorCode, a.ErrorMessage, a.LatencyMs,
		a.OccurredAt, rawBytes,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert attempt")
	}
	return nil
}
