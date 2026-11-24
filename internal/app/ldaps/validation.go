package ldaps

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// validateLdapHost rejects a blank directory host: without it every connection
// test and sync would dial an empty address.
func validateLdapHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return apperrors.NewInvalidArgument("host is required")
	}
	return nil
}

// validateLdapPort enforces the TCP port range. port=0 used to be persisted
// and only failed later at dial time.
func validateLdapPort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "port must be between 1 and 65535", map[string]any{
			"field": "port", "value": port, "min": 1, "max": 65535,
		})
	}
	return nil
}
