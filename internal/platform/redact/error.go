package redact

import (
	stderrors "errors"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrorCategory names the class of an error without rendering its message.
//
// Use it where the error text may embed row values (PostgreSQL unique /
// check-constraint detail carries the conflicting columns and values), user
// input or an upstream response body. The platform error code survives, so
// "conflict" / "unprocessable_entity" / "internal" remains searchable, while the
// message and its cause chain are never written.
func ErrorCategory(err error) string {
	if err == nil {
		return ""
	}
	var appErr *apperrors.Error
	if stderrors.As(err, &appErr) && appErr != nil && appErr.Code != "" {
		return string(appErr.Code)
	}
	return Bound(fmt.Sprintf("%T", err))
}
