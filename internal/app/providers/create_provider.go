package providers

import "context"

func (s Service) CreateProvider(ctx context.Context, command CreateProviderCommand) (*Provider, error) {
	return s.unified().Create(ctx, command)
}
