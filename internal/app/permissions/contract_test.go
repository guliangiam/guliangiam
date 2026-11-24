package permissions

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/infra/casbin"
)

func TestCasbinManagerImplementsCasbinPolicyManager(t *testing.T) {
	var _ domain.CasbinPolicyManager = (*casbin.Manager)(nil)
}
