package auth

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// sessionSidPrefix makes a session identifier recognizable in a token dump or a
// support ticket without making it guessable.
const sessionSidPrefix = "gl_sid_"

// newSessionSid mints the OIDC session identifier carried by both the ID token
// and the logout token for one session (设计报告 14.12.3).
//
// It is a dedicated value rather than `sessions.id` on purpose. The primary key
// is an internal handle: it is the session cookie's value, it is embedded in CAS
// tickets, and it is exposed through the session-management API. A relying party
// that received it as `sid` would hold a value that identifies and potentially
// controls our session, and revoking one session's `sid` in isolation would mean
// revoking the row the whole server looks up. 256 bits from crypto/rand gives a
// value that carries no relationship to any of that.
func newSessionSid() (string, error) {
	return platformcrypto.NewRandomSecret(sessionSidPrefix, platformcrypto.SecretEntropyBytes)
}

// sessionSidFor resolves an internal session id (the value in the session cookie
// and in a CAS ticket) to that session's OIDC identifier. It reports "" when the
// session is gone, so a caller omits the claim instead of publishing a stale
// handle.
//
// The unit of work is a parameter because WithinTransaction does not swap the
// context: reading through Service.UOW inside a transaction would use a second
// connection and miss the transaction's own writes.
func sessionSidFor(ctx context.Context, tx appuow.UnitOfWork, sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	session, err := tx.Sessions().GetByID(ctx, sessionID)
	if err != nil || session == nil {
		return ""
	}
	return session.Sid
}
