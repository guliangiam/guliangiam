package openapi

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var openapiYAML []byte

var (
	openapiJSONOnce sync.Once
	openapiJSONData []byte
	openapiJSONErr  error
)

func ServerYAML() []byte {
	return append([]byte(nil), openapiYAML...)
}

func ServerJSON() ([]byte, error) {
	openapiJSONOnce.Do(func() {
		var document any
		if err := yaml.Unmarshal(openapiYAML, &document); err != nil {
			openapiJSONErr = err
			return
		}

		openapiJSONData, openapiJSONErr = json.MarshalIndent(document, "", "  ")
	})

	if openapiJSONErr != nil {
		return nil, openapiJSONErr
	}

	return append([]byte(nil), openapiJSONData...), nil
}

func ServerYAMLETag() string {
	sum := sha256.Sum256(openapiYAML)
	return fmt.Sprintf("\"%x\"", sum)
}

func ServerJSONETag() (string, error) {
	data, err := ServerJSON()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return fmt.Sprintf("\"%x\"", sum), nil
}
