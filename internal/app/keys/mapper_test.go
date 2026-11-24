package keys

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/stretchr/testify/assert"
)

func TestToListItem_Key(t *testing.T) {
	now := time.Now()
	k := &domain.Key{
		ID:             "key1",
		OrganizationID: "org1",
		Name:           "signing-key",
		DisplayName:    "Signing Key",
		KeyType:        "RSA",
		Algorithm:      "RS256",
		ExpiresAt:      &now,
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	item := toListItem(k)
	assert.Equal(t, "key1", item.ID)
	assert.Equal(t, "org1", item.OrganizationID)
	assert.Equal(t, "signing-key", item.Name)
	assert.Equal(t, "Signing Key", item.DisplayName)
	assert.Equal(t, "RSA", item.KeyType)
	assert.Equal(t, "RS256", item.Algorithm)
	assert.True(t, item.IsEnabled)
}

func TestToDetail_Key(t *testing.T) {
	now := time.Now()
	k := &domain.Key{
		ID:             "key1",
		OrganizationID: "org1",
		Name:           "signing-key",
		DisplayName:    "Signing Key",
		KeyType:        "RSA",
		Algorithm:      "RS256",
		ExpiresAt:      &now,
		IsEnabled:      true,
		PublicKey:      "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----",
		Revision:       3,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	detail := toDetail(k)
	assert.Equal(t, "key1", detail.ID)
	assert.Equal(t, "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----", detail.PublicKey)
	assert.Equal(t, int64(3), detail.Revision)
}
