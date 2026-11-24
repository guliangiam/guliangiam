package mysql

import (
	"encoding/json"
	"errors"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// jsonMarshal/jsonUnmarshal 是 mysql 包内部轻量的 JSON 工具，避免引用
// postgres/shared 里的 marshalRelationDefs。双基线各自维护一份。
func jsonMarshal(v any) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "json marshal")
	}
	return b, nil
}

func jsonUnmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return errors.New("empty json payload")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return apperrors.WrapInternal(err, "json unmarshal")
	}
	return nil
}

func marshalRelationDefs(defs map[string]any) ([]byte, error) {
	return jsonMarshal(defs)
}
