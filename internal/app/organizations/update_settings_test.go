package organizations

import (
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/stretchr/testify/assert"
)

func TestAccountItemDTO_ToDomain(t *testing.T) {
	dto := AccountItemDTO{
		Name: "displayName", Visible: true, ViewRule: "all",
		ModifyRule: "admin", Regex: ".*", Tab: "basic",
	}
	result := dto.toDomain()
	assert.Equal(t, orgdomain.AccountItem{
		Name: "displayName", Visible: true, ViewRule: "all",
		ModifyRule: "admin", Regex: ".*", Tab: "basic",
	}, result)
}

func TestAccountItemDTO_ToDomain_Empty(t *testing.T) {
	dto := AccountItemDTO{}
	result := dto.toDomain()
	assert.Equal(t, orgdomain.AccountItem{}, result)
}

func TestMfaItemDTO_ToDomain(t *testing.T) {
	dto := MfaItemDTO{Name: "totp", Rule: "required"}
	result := dto.toDomain()
	assert.Equal(t, orgdomain.MfaItem{Name: "totp", Rule: "required"}, result)
}

func TestMfaItemDTO_ToDomain_Empty(t *testing.T) {
	dto := MfaItemDTO{}
	result := dto.toDomain()
	assert.Equal(t, orgdomain.MfaItem{}, result)
}
