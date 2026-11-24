package keys

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
)

func toListItem(k *domain.Key) KeyListItem {
	return KeyListItem{
		ID: k.ID, OrganizationID: k.OrganizationID, Name: k.Name,
		DisplayName: k.DisplayName, KeyType: k.KeyType, Algorithm: k.Algorithm,
		ExpiresAt: k.ExpiresAt, IsEnabled: k.IsEnabled,
		CreatedAt: k.CreatedAt, UpdatedAt: k.UpdatedAt,
	}
}

func toDetail(k *domain.Key) KeyDetail {
	return KeyDetail{
		KeyListItem: toListItem(k),
		PublicKey:   k.PublicKey,
		Revision:    k.Revision,
	}
}
