package outbox

import "errors"

// PermanentError marks a delivery failure that retrying cannot fix — a malformed
// payload, a contract violation, or an event type the handler does not accept.
// The worker sends such items straight to the dead-letter table instead of
// spending the remaining retry budget on a failure that will never succeed.
type PermanentError struct {
	err error
}

// Permanent wraps err so the worker classifies it as non-retryable.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{err: err}
}

func (e *PermanentError) Error() string { return e.err.Error() }
func (e *PermanentError) Unwrap() error { return e.err }

// IsPermanent reports whether err (or any error it wraps) is a PermanentError.
func IsPermanent(err error) bool {
	var target *PermanentError
	return errors.As(err, &target)
}
