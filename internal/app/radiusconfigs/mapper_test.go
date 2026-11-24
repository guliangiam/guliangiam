package radiusconfigs

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	now := time.Now()
	c := &domain.RadiusConfig{
		ID: "rc-1", OrganizationID: "org-1", Name: "main",
		Host: "radius.example.com", Port: 1812, AuthProtocol: "PAP",
		IsEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(c)
	assert.Equal(t, "rc-1", item.ID)
	assert.Equal(t, "radius.example.com", item.Host)
	assert.Equal(t, 1812, item.Port)
	assert.Equal(t, "PAP", item.AuthProtocol)
}

func TestToDetail(t *testing.T) {
	c := &domain.RadiusConfig{
		ID: "rc-1", OrganizationID: "org-1", Name: "main",
		SharedSecret: "secret123", Revision: 1,
	}
	detail := toDetail(c)
	assert.Equal(t, "rc-1", detail.ID)
	assert.True(t, detail.SharedSecretSet)
	assert.NotEmpty(t, detail.SharedSecretMasked)
	assert.NotEqual(t, "secret123", detail.SharedSecretMasked)
	assert.Equal(t, int64(1), detail.Revision)
}
