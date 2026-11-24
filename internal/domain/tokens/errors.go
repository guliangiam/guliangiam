package tokens

import apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

// ErrRefreshAlreadyRevoked is returned by Repository.RevokeRefresh when the
// refresh token was already revoked, i.e. this call lost the race to another
// request that rotated the same token pair.
//
// It lives in the domain layer (not in the PostgreSQL adapter) so the app layer
// can match on it without importing infrastructure. Rotation must treat it as
// token reuse and refuse to mint a second token pair: otherwise a client that
// replays a refresh token concurrently ends up with two live sessions and the
// family-wide reuse detection never fires.
var ErrRefreshAlreadyRevoked = apperrors.NewUnauthorized("token reuse detected: refresh token has already been rotated")
