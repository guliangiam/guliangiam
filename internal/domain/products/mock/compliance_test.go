package mock

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/products"
)

func TestMockRepositoryImplementsInterface(t *testing.T) {
	var _ domain.Repository = NewMockRepository(nil)
}
