package entries

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntry_Creation(t *testing.T) {
	now := time.Now()
	e := Entry{
		ID:             "entry-001",
		OrganizationID: "org-001",
		Name:           "user-login",
		DisplayName:    "User Login",
		Provider:       "password",
		Application:    "app-001",
		Type:           "signin",
		ClientIP:       "192.168.1.1",
		UserAgent:      "Mozilla/5.0",
		Message:        "login successful",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, "entry-001", e.ID)
	assert.Equal(t, "signin", e.Type)
	assert.Equal(t, "192.168.1.1", e.ClientIP)
}

func TestEntry_Types(t *testing.T) {
	commonTypes := []string{"signin", "signup", "signout", "password_change", "mfa_setup"}

	for _, entryType := range commonTypes {
		t.Run("type_"+entryType, func(t *testing.T) {
			e := Entry{Type: entryType}
			assert.Equal(t, entryType, e.Type)
		})
	}
}
