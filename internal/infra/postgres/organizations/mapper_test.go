package organizations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDerefString(t *testing.T) {
	t.Run("nil_pointer", func(t *testing.T) {
		assert.Equal(t, "", derefString(nil))
	})

	t.Run("non_nil_pointer", func(t *testing.T) {
		val := "hello"
		assert.Equal(t, "hello", derefString(&val))
	})

	t.Run("empty_string_pointer", func(t *testing.T) {
		val := ""
		assert.Equal(t, "", derefString(&val))
	})
}

func TestToOrganization(t *testing.T) {
	now := time.Now()

	t.Run("basic_mapping", func(t *testing.T) {
		website := "https://example.com"
		logo := "logo.png"
		row := organizationRow{
			ID:          "org-001",
			Slug:        "test-org",
			DisplayName: "Test Organization",
			WebsiteURL:  &website,
			LogoURL:     &logo,
			Status:      "active",
			InitScore:   100,
			Currency:    "USD",
			CreatedAt:   now,
			UpdatedAt:   now,
			Revision:    1,
		}

		org := toOrganization(row)

		assert.Equal(t, "org-001", org.ID)
		assert.Equal(t, "test-org", org.Slug)
		assert.Equal(t, "Test Organization", org.DisplayName)
		assert.Equal(t, "https://example.com", org.WebsiteURL)
		assert.Equal(t, "logo.png", org.LogoURL)
		assert.Equal(t, "active", org.Status)
		assert.Equal(t, 100, org.InitScore)
		assert.Equal(t, "USD", org.Currency)
		assert.Equal(t, int64(1), org.Revision)
	})

	t.Run("nil_optional_fields", func(t *testing.T) {
		row := organizationRow{
			ID:          "org-002",
			Slug:        "minimal",
			DisplayName: "Minimal Org",
			WebsiteURL:  nil,
			LogoURL:     nil,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		org := toOrganization(row)

		assert.Equal(t, "", org.WebsiteURL)
		assert.Equal(t, "", org.LogoURL)
		assert.Equal(t, "", org.LogoDarkURL)
		assert.Equal(t, "", org.FaviconURL)
	})

	t.Run("boolean_flags", func(t *testing.T) {
		row := organizationRow{
			ID:                 "org-003",
			Slug:               "flags",
			DisplayName:        "Flags Org",
			DisableSignin:      true,
			DisableSignup:      true,
			EnableSoftDeletion: true,
			EnableDeposit:      false,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		org := toOrganization(row)

		assert.True(t, org.DisableSignin)
		assert.True(t, org.DisableSignup)
		assert.True(t, org.EnableSoftDeletion)
		assert.False(t, org.EnableDeposit)
	})

	t.Run("kerberos_fields", func(t *testing.T) {
		row := organizationRow{
			ID:                  "org-004",
			Slug:                "kerb",
			DisplayName:         "Kerb Org",
			KerberosRealm:       "EXAMPLE.COM",
			KerberosKdcHost:     "kdc.example.com",
			KerberosKeytab:      "/etc/krb5.keytab",
			KerberosServiceName: "HTTP",
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		org := toOrganization(row)

		assert.Equal(t, "EXAMPLE.COM", org.KerberosRealm)
		assert.Equal(t, "kdc.example.com", org.KerberosKdcHost)
		assert.Equal(t, "/etc/krb5.keytab", org.KerberosKeytab)
		assert.Equal(t, "HTTP", org.KerberosServiceName)
	})
}
