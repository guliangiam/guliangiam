package mock

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

func TestMockRepositoryImplementsInterface(t *testing.T) {
	var _ domain.Repository = NewMockRepository(nil)
}
