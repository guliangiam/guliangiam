package route

type Definition struct {
	Method      string
	Pattern     string
	OperationID string
	Summary     string
	Permission  string
	// Surface is the API face this route belongs to (手册 4.1.3). It is
	// declared explicitly and asserted against SurfaceFor(pattern) by the
	// surface contract gate, so a wrong value fails the build instead of
	// silently mis-tagging metrics, audit rows or OpenAPI tags.
	Surface Surface
	// SuccessStatus is the HTTP status the handler writes on success. Zero
	// means 200 OK, so existing declarations keep their current contract;
	// only non-default routes (201/202/204/3xx) declare a value. The
	// OpenAPI generator reads this field instead of guessing from the
	// operationId.
	SuccessStatus int
	// ResponseHeaders lists the headers set on the success response (for
	// example Location or Retry-After). Empty means none are documented.
	ResponseHeaders []string
	// ErrorResponses lists non-default error statuses the handler actually
	// returns (for example 409 already_exists or 422 unprocessable_entity).
	// Empty means none are documented beyond the generic 500, so existing
	// declarations keep their current contract. The OpenAPI generator emits
	// these in ascending order using the shared ErrorResponse schema.
	ErrorResponses []int
}
