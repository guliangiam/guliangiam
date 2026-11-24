package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const registryConfigType = "_provider_registry_v1"

type secretMetadata struct {
	Fingerprint string `json:"fingerprint"`
	Version     int64  `json:"version"`
	KeyVersion  int    `json:"keyVersion"`
}

type registryMetadata struct {
	Family   string                    `json:"family"`
	Kind     string                    `json:"kind"`
	Variant  string                    `json:"variant"`
	Status   string                    `json:"status"`
	DetailID string                    `json:"detailId"`
	Secrets  map[string]secretMetadata `json:"secrets,omitempty"`
}

type registryConfigRepository interface {
	GetConfigByType(ctx context.Context, providerID, configType string) (*domain.ProviderConfig, error)
	ListConfigsByType(ctx context.Context, providerIDs []string, configType string) ([]domain.ProviderConfig, error)
}

type secretStatusRotationRepository interface {
	RotateSecretStatus(ctx context.Context, current domain.ProviderSecretStatus, previousStatus string, previousExpiresAt time.Time) error
}

func metadataFromRegistration(registration domain.Registration, detailID string) registryMetadata {
	return registryMetadata{
		Family:   string(registration.Family),
		Kind:     string(registration.Kind),
		Variant:  string(registration.Variant),
		Status:   string(registration.Status),
		DetailID: detailID,
		Secrets:  map[string]secretMetadata{},
	}
}

func (m registryMetadata) config(providerID string, now time.Time) (*domain.ProviderConfig, error) {
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal provider registry metadata")
	}
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, apperrors.WrapInternalf(err, "decode provider registry metadata")
	}
	return &domain.ProviderConfig{
		ProviderID: providerID,
		ConfigType: registryConfigType,
		Config:     values,
		UpdatedAt:  now,
	}, nil
}

func decodeRegistryMetadata(config *domain.ProviderConfig) (registryMetadata, error) {
	if config == nil {
		return registryMetadata{}, apperrors.NewNotFound("provider registry metadata not found")
	}
	payload, err := json.Marshal(config.Config)
	if err != nil {
		return registryMetadata{}, apperrors.WrapInternalf(err, "marshal provider registry config")
	}
	var metadata registryMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return registryMetadata{}, apperrors.WrapInternalf(err, "decode provider registry config")
	}
	if !domain.Family(metadata.Family).Valid() || metadata.Kind == "" || !domain.Status(metadata.Status).Valid() {
		return registryMetadata{}, apperrors.NewInternal("invalid provider registry metadata")
	}
	if metadata.Secrets == nil {
		metadata.Secrets = map[string]secretMetadata{}
	}
	return metadata, nil
}

func getRegistryMetadata(ctx context.Context, repo domain.Repository, providerID string) (registryMetadata, error) {
	registryRepo, ok := repo.(registryConfigRepository)
	if !ok {
		return registryMetadata{}, apperrors.NewNotFound("provider registry metadata not found")
	}
	config, err := registryRepo.GetConfigByType(ctx, providerID, registryConfigType)
	if err != nil {
		return registryMetadata{}, err
	}
	return decodeRegistryMetadata(config)
}

func listRegistryMetadata(ctx context.Context, repo domain.Repository, providerIDs []string) (map[string]registryMetadata, error) {
	result := make(map[string]registryMetadata, len(providerIDs))
	if len(providerIDs) == 0 {
		return result, nil
	}
	registryRepo, ok := repo.(registryConfigRepository)
	if !ok {
		return result, nil
	}
	configs, err := registryRepo.ListConfigsByType(ctx, providerIDs, registryConfigType)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		metadata, err := decodeRegistryMetadata(&configs[i])
		if err != nil {
			return nil, fmt.Errorf("provider %s registry metadata: %w", configs[i].ProviderID, err)
		}
		result[configs[i].ProviderID] = metadata
	}
	return result, nil
}

func isNotFound(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}
