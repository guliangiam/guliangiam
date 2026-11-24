package payments

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── RefundsRepo ────────────────────────────────────────────────────────

type RefundsRepository struct{ db mysqlshared.DBTX }

func NewRefundsRepository(db mysqlshared.DBTX) RefundsRepository {
	return RefundsRepository{db: db}
}

var _ domain.RefundsRepo = RefundsRepository{}

func (r RefundsRepository) Create(ctx context.Context, ref *domain.Refund) error {
	var channelRefundNo sql.NullString
	if ref.ChannelRefundNo != "" {
		channelRefundNo = sql.NullString{String: ref.ChannelRefundNo, Valid: true}
	}
	var errorCode sql.NullString
	if ref.ErrorCode != nil {
		errorCode = sql.NullString{String: *ref.ErrorCode, Valid: true}
	}
	var errorMessage sql.NullString
	if ref.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *ref.ErrorMessage, Valid: true}
	}
	var completedAt sql.NullTime
	if ref.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *ref.CompletedAt, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, refundInsert,
		ref.ID, ref.OrganizationID, ref.PaymentID, ref.OutRefundNo,
		ref.Channel, ref.Amount, ref.Currency, ref.Reason, ref.State,
		channelRefundNo, errorCode, errorMessage,
		ref.RequestedAt, completedAt, ref.CreatedAt, ref.UpdatedAt, ref.Revision,
	)
	return err
}

func (r RefundsRepository) GetByID(ctx context.Context, id string) (*domain.Refund, error) {
	ref, err := scanRefund(r.db.QueryRowContext(ctx, refundGetByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Refund, error) {
	ref, err := scanRefund(r.db.QueryRowContext(ctx, refundGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) GetByOutRefundNo(ctx context.Context, channel, outRefundNo string) (*domain.Refund, error) {
	ref, err := scanRefund(r.db.QueryRowContext(ctx, refundGetByOutRefundNo, channel, outRefundNo))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("refund not found by channel refund no")
		}
		return nil, err
	}
	return ref, nil
}

func (r RefundsRepository) Update(ctx context.Context, ref *domain.Refund) error {
	var channelRefundNo sql.NullString
	if ref.ChannelRefundNo != "" {
		channelRefundNo = sql.NullString{String: ref.ChannelRefundNo, Valid: true}
	}
	var errorCode sql.NullString
	if ref.ErrorCode != nil {
		errorCode = sql.NullString{String: *ref.ErrorCode, Valid: true}
	}
	var errorMessage sql.NullString
	if ref.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *ref.ErrorMessage, Valid: true}
	}
	var completedAt sql.NullTime
	if ref.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *ref.CompletedAt, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, refundUpdate,
		channelRefundNo, ref.State, errorCode, errorMessage, completedAt,
		ref.UpdatedAt, ref.ID,
	)
	return err
}

func (r RefundsRepository) ListByPayment(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	return r.list(ctx, refundListByPayment, paymentID)
}

func (r RefundsRepository) ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*domain.Refund, error) {
	return r.list(ctx, refundListByPaymentForOrganization, paymentID, organizationID)
}

func (r RefundsRepository) list(ctx context.Context, query string, args ...any) ([]*domain.Refund, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.Refund
	for rows.Next() {
		ref, err := scanRefund(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, ref)
	}
	return items, rows.Err()
}

// LockPaymentForRefund 用 SELECT ... FOR UPDATE 锁住 payment 行，并派生已退/在途金额。
func (r RefundsRepository) LockPaymentForRefund(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
	var p domain.LockedPayment
	err := r.db.QueryRowContext(ctx, refundLockPayment, paymentID).Scan(
		&p.RefundedAmount, &p.RefundingAmount, &p.Price, &p.Status,
		&p.Provider, &p.OutTradeNo, &p.Currency,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("payment not found or not settled")
		}
		return nil, err
	}
	return &p, nil
}

func (r RefundsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Refund, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, refundCount, organizationID, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, refundList, organizationID, organizationID, mysqlshared.ClampPageSize(limit), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []*domain.Refund
	for rows.Next() {
		ref, err := scanRefund(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, ref)
	}
	return items, total, rows.Err()
}

// ─── AttemptsRepo ──────────────────────────────────────────────────────

type AttemptsRepository struct{ db mysqlshared.DBTX }

func NewAttemptsRepository(db mysqlshared.DBTX) AttemptsRepository { return AttemptsRepository{db: db} }

var _ domain.AttemptsRepo = AttemptsRepository{}

func (r AttemptsRepository) GetByID(ctx context.Context, id string) (*domain.PayAttempt, error) {
	a, err := scanPayAttempt(r.db.QueryRowContext(ctx, attemptGetByID, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("attempt not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AttemptsRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.PayAttempt, error) {
	a, err := scanPayAttempt(r.db.QueryRowContext(ctx, attemptGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("attempt not found")
		}
		return nil, err
	}
	return a, nil
}

func (r AttemptsRepository) ListByPayment(ctx context.Context, paymentID string) ([]*domain.PayAttempt, error) {
	return r.list(ctx, attemptListByPayment, paymentID)
}

func (r AttemptsRepository) ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*domain.PayAttempt, error) {
	return r.list(ctx, attemptListByPaymentForOrganization, paymentID, organizationID)
}

func (r AttemptsRepository) list(ctx context.Context, query string, args ...any) ([]*domain.PayAttempt, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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
	rows, err := r.db.QueryContext(ctx, attemptListByRefund, refundID)
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
	var refundID sql.NullString
	if a.RefundID != nil {
		refundID = sql.NullString{String: *a.RefundID, Valid: true}
	}
	var channelOrderNo sql.NullString
	if a.ChannelOrderNo != nil {
		channelOrderNo = sql.NullString{String: *a.ChannelOrderNo, Valid: true}
	}
	var channelRequestID sql.NullString
	if a.ChannelRequestID != nil {
		channelRequestID = sql.NullString{String: *a.ChannelRequestID, Valid: true}
	}
	var amount sql.NullFloat64
	if a.Amount != nil {
		amount = sql.NullFloat64{Float64: *a.Amount, Valid: true}
	}
	var currency sql.NullString
	if a.Currency != nil {
		currency = sql.NullString{String: *a.Currency, Valid: true}
	}
	var errorCode sql.NullString
	if a.ErrorCode != nil {
		errorCode = sql.NullString{String: *a.ErrorCode, Valid: true}
	}
	var errorMessage sql.NullString
	if a.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *a.ErrorMessage, Valid: true}
	}
	var latencyMs sql.NullInt64
	if a.LatencyMs != nil {
		latencyMs = sql.NullInt64{Int64: int64(*a.LatencyMs), Valid: true}
	}
	var rawResponseStr sql.NullString
	if len(a.RawResponse) > 0 {
		b, err := json.Marshal(a.RawResponse)
		if err != nil {
			return apperrors.WrapInternalf(err, "marshal raw_response")
		}
		rawResponseStr = sql.NullString{String: string(b), Valid: true}
	}
	_, err := r.db.ExecContext(ctx, attemptInsert,
		a.ID, a.OrganizationID, a.PaymentID, refundID, a.Channel,
		a.AttemptType, a.AttemptNo, channelOrderNo, channelRequestID,
		string(a.State), amount, currency, errorCode, errorMessage, latencyMs,
		a.OccurredAt, rawResponseStr,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert attempt")
	}
	return nil
}
