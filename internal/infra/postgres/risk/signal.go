package risk

import (
	"context"
	"errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

const queryGetSignal = `
SELECT id, organization_id, user_id, signal_type, signal_value, ip::text, created_at
FROM guliangiam.risk_signals
WHERE id = $1 AND organization_id = $2
`

// GetSignal implements domain.SignalReader. Both predicates are mandatory so
// a signal from another tenant is reported as not found, never as visible.
func (r Repository) GetSignal(ctx context.Context, id, organizationID string) (*domain.Signal, error) {
	var signal domain.Signal
	err := r.db.QueryRow(ctx, queryGetSignal, id, organizationID).Scan(
		&signal.ID, &signal.OrganizationID, &signal.UserID, &signal.SignalType,
		&signal.SignalValue, &signal.IP, &signal.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NewNotFound("risk signal not found")
	}
	if err != nil {
		return nil, err
	}
	return &signal, nil
}

var _ domain.SignalReader = Repository{}
