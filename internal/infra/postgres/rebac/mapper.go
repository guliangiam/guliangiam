package rebac

import (
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func marshalRelationDefs(defs map[string]any) ([]byte, error) {
	if len(defs) == 0 {
		return []byte("{}"), nil
	}
	return shared.MarshalJSON(defs)
}
