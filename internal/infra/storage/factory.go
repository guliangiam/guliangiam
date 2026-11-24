package storage

import (
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Factory creates and manages storage providers by type.
type Factory struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewFactory creates a new storage provider factory.
func NewFactory() *Factory {
	return &Factory{
		providers: make(map[string]Provider),
	}
}

// Register registers a storage provider under the given type name.
// If a provider with the same type already exists, it will be replaced.
func (f *Factory) Register(providerType string, p Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[providerType] = p
}

// GetProvider returns the storage provider registered under the given type name.
// Returns an error if no provider is registered with that type.
func (f *Factory) GetProvider(providerType string) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.providers[providerType]
	if !ok {
		return nil, apperrors.NewNotFoundf("storage factory: provider not found for type %q", providerType)
	}
	return p, nil
}

// Verify checks that a provider with the given type is registered and returns it.
// This is a convenience method that combines GetProvider with existence checking.
func (f *Factory) Verify(providerType string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.providers[providerType]
	if !ok {
		return apperrors.NewUnavailablef("storage factory: provider not registered for type %q", providerType)
	}
	return nil
}

// Types returns all registered provider type names.
func (f *Factory) Types() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	types := make([]string, 0, len(f.providers))
	for t := range f.providers {
		types = append(types, t)
	}
	return types
}
