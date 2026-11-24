package presenter

import (
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
)

// The canonical envelope lives in platform/http so middleware can emit the
// same shape without importing transport. The aliases keep the presenter API
// (and route.WriteError) unchanged while guaranteeing both writers serialize
// exactly the same fields.
type ErrorBody = httperr.ErrorBody

type ErrorResponse = httperr.ErrorResponse

const internalErrorMessage = httperr.InternalErrorMessage

func PresentError(err error, requestID, traceID string) ErrorResponse {
	return httperr.PresentError(err, requestID, traceID)
}
