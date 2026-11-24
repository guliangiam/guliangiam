package kerberos

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/service"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

// ctxCredentialsKey is the context key used by gokrb5 to store verified credentials.
// This mirrors the unexported constant in spnego/http.go:
//
//	ctxCredentials = "github.com/jcmturner/gokrb5/v8/ctxCredentials"
//
// Must be a plain string (not a custom type) to match gokrb5's context.Value lookup.
const ctxCredentialsKey = "github.com/jcmturner/gokrb5/v8/ctxCredentials" //nolint:gosec // G101: context key, not a credential

// Authenticator handles Kerberos/SPNEGO authentication using gokrb5
// for cryptographically verified token parsing and AP-REQ decryption.
type Authenticator struct {
	mu            sync.RWMutex
	realm         string
	keytabPath    string
	serviceName   string // HTTP service principal name (e.g., "HTTP")
	kt            *keytab.Keytab
	spnegoService *spnego.SPNEGO
}

// NewAuthenticator creates a new Kerberos authenticator.
// realm: Kerberos realm (e.g., "EXAMPLE.COM")
// keytabPath: Path to the keytab file
// serviceName: Service principal name (e.g., "HTTP")
func NewAuthenticator(realm, keytabPath, serviceName string) *Authenticator {
	a := &Authenticator{
		realm:       realm,
		keytabPath:  keytabPath,
		serviceName: serviceName,
	}
	// Attempt to load keytab at creation time.
	// If it fails, the authenticator operates in fallback mode
	// until ReloadKeytab is called successfully.
	if err := a.loadKeytab(); err != nil {
		slog.Warn("kerberos: keytab not loaded at init, will use fallback mode",
			"keytab_path", keytabPath,
			"error", err,
		)
	}
	return a
}

// loadKeytab loads (or reloads) the keytab file and rebuilds service settings.
func (a *Authenticator) loadKeytab() error {
	if a.keytabPath == "" {
		return apperrors.NewNotFound("kerberos: keytab path is empty")
	}

	kt, err := keytab.Load(a.keytabPath)
	if err != nil {
		return apperrors.WrapInternalf(err, a.keytabPath, "kerberos: failed to load keytab %s")
	}

	spn := a.serviceName
	if spn == "" {
		spn = "HTTP"
	}

	svc := spnego.SPNEGOService(kt, service.SName(spn))

	a.mu.Lock()
	a.kt = kt
	a.spnegoService = svc
	a.mu.Unlock()

	slog.Info("kerberos: keytab loaded successfully",
		"keytab_path", a.keytabPath,
		"service_name", spn,
	)
	return nil
}

// ReloadKeytab reloads the keytab from disk. Use this when the keytab
// file has been updated (e.g., key rotation) without restarting the service.
func (a *Authenticator) ReloadKeytab() error {
	return a.loadKeytab()
}

// IsReady returns true if the keytab is loaded and the authenticator
// can perform cryptographic verification of SPNEGO tokens.
func (a *Authenticator) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.kt != nil && a.spnegoService != nil
}

// ValidatedPrincipal represents a cryptographically verified Kerberos principal.
type ValidatedPrincipal struct {
	Username    string // Client username (e.g., "alice")
	Realm       string // Client realm (e.g., "EXAMPLE.COM")
	DisplayName string // Full principal name (e.g., "alice@EXAMPLE.COM")
}

// ValidateSPNEGO validates a SPNEGO token from the Authorization header
// using gokrb5 for cryptographic verification.
//
// This performs the full validation chain:
//  1. Base64-decode the token
//  2. Parse the SPNEGO ASN.1 structure
//  3. Decrypt the AP-REQ ticket using the service key from keytab
//  4. Validate the authenticator (checksum, timestamp, replay)
//  5. Extract the client principal from verified credentials
//
// Returns the validated principal if the token is cryptographically valid.
func (a *Authenticator) ValidateSPNEGO(authHeader string) (*ValidatedPrincipal, error) {
	// Remove "Negotiate " prefix if present
	tokenStr := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(tokenStr), "negotiate ") {
		tokenStr = strings.TrimSpace(tokenStr[10:])
	}

	if tokenStr == "" {
		return nil, apperrors.NewNotFound("kerberos: empty SPNEGO token")
	}

	// Base64-decode the token
	tokenBytes, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "kerberos: base64 decode SPNEGO token")
	}

	a.mu.RLock()
	svc := a.spnegoService
	a.mu.RUnlock()

	if svc == nil {
		return nil, apperrors.NewInvalidArgument("kerberos: keytab not loaded, cannot verify SPNEGO token")
	}

	// Parse the SPNEGO ASN.1 DER structure
	var st spnego.SPNEGOToken
	if err := st.Unmarshal(tokenBytes); err != nil {
		return nil, apperrors.WrapInternalf(err, "kerberos: failed to parse SPNEGO token")
	}

	// Verify the token cryptographically using the keytab.
	// AcceptSecContext:
	//   - Extracts the KRB5Token from the SPNEGO wrapper
	//   - Decrypts the ticket using the service key from keytab
	//   - Validates the authenticator (checksum, timestamp, replay check)
	//   - Establishes the security context with client credentials
	ok, ctx, status := svc.AcceptSecContext(&st)
	if !ok {
		return nil, apperrors.NewUnauthorizedf("kerberos: SPNEGO verification failed: %s", status.Message)
	}

	// Extract the client credentials from the verified context
	principal, err := extractPrincipalFromContext(ctx)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "kerberos: failed to extract principal")
	}

	return principal, nil
}

// extractPrincipalFromContext extracts the validated client principal from the
// gokrb5 SPNEGO context. After successful AcceptSecContext, gokrb5 stores the
// verified *credentials.Credentials in the context under the string key
// "github.com/jcmturner/gokrb5/v8/ctxCredentials" (matching spnego/http.go).
func extractPrincipalFromContext(ctx context.Context) (*ValidatedPrincipal, error) {
	if ctx == nil {
		return nil, apperrors.NewNotFound("no verification context available")
	}

	// gokrb5 stores credentials using a plain string context key.
	// We use the same string literal to ensure type-accurate lookup.
	credsVal := ctx.Value(ctxCredentialsKey)
	if credsVal == nil {
		return nil, apperrors.NewNotFound("no credentials in verification context")
	}

	creds, ok := credsVal.(*credentials.Credentials)
	if !ok {
		return nil, apperrors.NewUnauthorized("unexpected credentials type in verification context")
	}

	username := creds.UserName()
	realm := strings.ToUpper(creds.Realm())

	return &ValidatedPrincipal{
		Username:    username,
		Realm:       realm,
		DisplayName: fmt.Sprintf("%s@%s", username, realm),
	}, nil
}

// ParseAuthorizationHeader extracts the SPNEGO token from an HTTP Authorization header.
// Returns the raw token string (base64-encoded) and whether it's a Negotiate header.
func ParseAuthorizationHeader(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "negotiate") {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

// BuildNegotiateResponse builds a WWW-Authenticate: Negotiate header value.
func BuildNegotiateResponse(token string) string {
	if token == "" {
		return "Negotiate"
	}
	return "Negotiate " + token
}
