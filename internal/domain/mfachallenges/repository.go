package mfachallenges

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Challenge, error)
	Create(ctx context.Context, challenge *Challenge) error
	Update(ctx context.Context, challenge *Challenge) error
	Delete(ctx context.Context, id string) error
}
