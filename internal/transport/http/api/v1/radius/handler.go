package radius

import (
	"context"
	"strings"
	"sync"

	"github.com/guliangiam/guliangiam/cred"
	appradius "github.com/guliangiam/guliangiam/internal/app/radius"
	"github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ServerStatus represents the current state of the RADIUS server.
type ServerStatus struct {
	Running   bool   `json:"running"`
	Addr      string `json:"addr,omitempty"`
	SecretSet bool   `json:"secretSet"`
}

// Handler manages the RADIUS authentication server lifecycle.
type Handler struct {
	UOW       uow.UnitOfWork
	RadiusSvc *appradius.Service

	// AccountingHandler persists RADIUS accounting packets (BE-B-13). It is
	// injected by the composition root; nil keeps the server's ack-only
	// default. The signature mirrors infra/radius.AccountingHandler (a type
	// alias), so the structural assertion below binds without importing infra.
	AccountingHandler func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64)

	mu         sync.Mutex
	server     appradius.Server
	status     ServerStatus
	lifeCancel context.CancelFunc
}

// NewHandler creates a new RADIUS handler with the given unit of work and radius service.
func NewHandler(uow uow.UnitOfWork, radiusSvc *appradius.Service) *Handler {
	return &Handler{
		UOW:       uow,
		RadiusSvc: radiusSvc,
		status: ServerStatus{
			Running: false,
		},
	}
}

// StartServer starts the RADIUS authentication server. The request context is
// intentionally NOT used as the server lifecycle (BE-B-13); the server runs
// until StopServer.
func (h *Handler) StartServer(_ context.Context, addr, secret string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.status.Running {
		return ErrServerAlreadyRunning()
	}

	if strings.TrimSpace(addr) == "" {
		addr = ":1812"
	}
	if strings.TrimSpace(secret) == "" {
		return ErrSecretRequired()
	}

	server := h.RadiusSvc.NewServer(addr, secret)

	// Wire up the authentication handler to validate against the guliangiam user store.
	server.SetAuthHandler(func(ctx context.Context, username, password string) (bool, error) {
		return h.authenticateUser(ctx, username, password)
	})

	// BE-B-13: wire the accounting handler so accounting packets are persisted
	// instead of being silently acked. The domain port only exposes
	// SetAuthHandler/Start/Stop, so the accounting setter is bound through a
	// structural assertion (infra/radius.AccountingHandler is a type alias).
	if h.AccountingHandler != nil {
		if acctSetter, ok := server.(interface {
			SetAcctHandler(func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64))
		}); ok {
			acctSetter.SetAcctHandler(h.AccountingHandler)
		}
	}

	// BE-B-13: the server lifecycle must not be bound to the HTTP request that
	// started it — r.Context() is cancelled as soon as the response completes.
	// Use a detached context cancelled by StopServer (or process shutdown).
	lifeCtx, lifeCancel := context.WithCancel(context.Background())
	if err := server.Start(lifeCtx); err != nil {
		lifeCancel()
		return err
	}

	h.server = server
	h.lifeCancel = lifeCancel
	h.status = ServerStatus{
		Running:   true,
		Addr:      addr,
		SecretSet: true,
	}

	return nil
}

// StopServer stops the RADIUS authentication server.
func (h *Handler) StopServer() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.status.Running || h.server == nil {
		return ErrServerNotRunning()
	}

	if err := h.server.Stop(); err != nil {
		return err
	}
	if h.lifeCancel != nil {
		h.lifeCancel()
		h.lifeCancel = nil
	}

	h.server = nil
	h.status = ServerStatus{
		Running: false,
	}

	return nil
}

// GetStatus returns the current status of the RADIUS server.
func (h *Handler) GetStatus() ServerStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status
}

// authenticateUser validates a username/password combination against the guliangiam user store.
func (h *Handler) authenticateUser(ctx context.Context, username, password string) (bool, error) {
	// The RADIUS username may be in the format "orgSlug/username" or just "username".
	var orgSlug, cleanUsername string
	if parts := strings.SplitN(username, "/", 2); len(parts) == 2 {
		orgSlug = parts[0]
		cleanUsername = parts[1]
	} else {
		cleanUsername = username
	}

	// If no organization is provided, reject: a cross-tenant username search
	// would be both ambiguous and a tenant-enumeration oracle.
	if orgSlug == "" {
		return false, nil
	}

	// BE-B-5: RADIUS packets arrive on a bare UDP context with no tenant GUC.
	// organizations/users are FORCE RLS, so a scope-less read fails closed and
	// every authentication was rejected. Resolve the slug under a controlled
	// System scope (the slug comes from the packet, but the lookup only maps it
	// to a tenant id — no tenant data is returned to the caller), then bind the
	// resolved tenant for the credential read.
	var organizationID string
	err := h.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx uow.UnitOfWork) error {
		org, gerr := tx.Organizations().GetBySlug(ctx, orgSlug)
		if gerr != nil {
			return gerr
		}
		organizationID = org.ID
		return nil
	})
	if err != nil || organizationID == "" {
		return false, nil
	}

	userCtx := tenantctx.WithTenantScope(ctx, organizationID, "", tenantctx.TenantFilterStrict)
	user, err := h.UOW.Users().GetByUsername(userCtx, organizationID, cleanUsername)
	if err != nil {
		// Don't reveal whether the user exists.
		return false, nil
	}

	// RADIUS authenticates against the local hash only. A user without a local
	// password (LDAP/social-only) or a provider marker such as "ldap" has no
	// verifier and must fail closed.
	algorithm, ok := user.Credentials.PasswordVerificationAlgorithm()
	if !ok {
		return false, nil
	}

	credManager := cred.GetCredManager(algorithm)
	if credManager == nil {
		return false, nil
	}

	return credManager.IsPasswordCorrect(password, user.Credentials.PasswordHash, ""), nil
}

// --- Errors ---

// The RADIUS control-plane errors are constructors, not shared sentinels.
// route.WriteError runs apperrors.WithStack, which stamps the stack onto the
// *apperrors.Error **in place** — a package-level value would be mutated by
// every request that hit it, which is a data race under -race. They used to be
// values of a package-local type carrying only a message, so the management face
// rendered every one of them as 500 `internal` instead of the intended
// 409/400. Asking the application namespace gives the right status for free.
func ErrServerAlreadyRunning() error {
	return apperrors.NewConflict("RADIUS server is already running")
}

func ErrServerNotRunning() error {
	return apperrors.NewConflict("RADIUS server is not running")
}

func ErrSecretRequired() error {
	return apperrors.NewInvalidArgument("shared secret is required")
}
