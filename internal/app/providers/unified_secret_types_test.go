package providers

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
)

func TestDefaultSecretType_AllFamilies(t *testing.T) {
	cases := []struct {
		family domain.Family
		want   string
	}{
		{domain.FamilyIdentity, secretTypeClientSecret},
		{domain.FamilyAI, secretTypeAPIKey},
		{domain.FamilyPayment, secretTypeConfig},
		{domain.FamilyNotification, secretTypeAccessToken},
		{domain.Family("unknown"), secretTypeClientSecret},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, defaultSecretType(c.family), "family=%s", c.family)
	}
}
