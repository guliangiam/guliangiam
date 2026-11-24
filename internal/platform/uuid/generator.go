package uuid

import (
	"strings"

	guuid "github.com/google/uuid"
)

type Generator interface {
	NewString() (string, error)
}

type V7Generator struct{}

func (V7Generator) NewString() (string, error) {
	id, err := guuid.NewV7()
	if err != nil {
		return "", err
	}

	return id.String(), nil
}

// IsValid reports whether value is a syntactically valid UUID.
//
// Public endpoints that accept an organization identifier have to tell a UUID
// from a slug, because `guliangiam.resolve_org_id` only guards WHERE clauses —
// inserts cast straight to `uuid`.
func IsValid(value string) bool {
	_, err := guuid.Parse(strings.TrimSpace(value))
	return err == nil
}
