package authorizationcodes

import (
	"context"
	"time"
)

type Repository interface {
	GetByCode(ctx context.Context, code string) (*AuthorizationCode, error)
	Create(ctx context.Context, authorizationCode *AuthorizationCode) error
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
}
