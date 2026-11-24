package consents

import "context"

type Repository interface {
	GetByUserAndApplication(ctx context.Context, userID string, applicationID string) (*Consent, error)
	CountActiveByApplication(ctx context.Context, applicationID string) (int64, error)
	Upsert(ctx context.Context, consent *Consent) error
	Revoke(ctx context.Context, userID string, applicationID string) error
}
