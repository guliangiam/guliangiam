package scim

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/stretchr/testify/assert"
)

func TestPatchToEmails(t *testing.T) {
	got := patchToEmails([]interface{}{
		"not-a-map",
		map[string]interface{}{"value": "a@example.com", "type": "home", "primary": true},
		map[string]interface{}{},
	})
	assert.Len(t, got, 2)
	assert.Equal(t, "a@example.com", got[0].Value)
	assert.Equal(t, "home", got[0].Type)
	assert.True(t, got[0].Primary)
	assert.Equal(t, attrTypeWork, got[1].Type)
	assert.Empty(t, got[1].Value)

	direct := []domain.Email{{Value: "b@example.com", Type: "work"}}
	assert.Equal(t, direct, patchToEmails(direct))

	assert.Empty(t, patchToEmails("unsupported"))
}

func TestPatchToPhones(t *testing.T) {
	got := patchToPhones([]interface{}{
		"not-a-map",
		map[string]interface{}{"value": "+8613800000000", "type": "mobile", "primary": true},
		map[string]interface{}{},
	})
	assert.Len(t, got, 2)
	assert.Equal(t, "+8613800000000", got[0].Value)
	assert.Equal(t, "mobile", got[0].Type)
	assert.True(t, got[0].Primary)
	assert.Equal(t, attrTypeWork, got[1].Type)
	assert.Empty(t, got[1].Value)

	direct := []domain.PhoneNumber{{Value: "+8613900000000", Type: "work"}}
	assert.Equal(t, direct, patchToPhones(direct))

	assert.Empty(t, patchToPhones("unsupported"))
}
