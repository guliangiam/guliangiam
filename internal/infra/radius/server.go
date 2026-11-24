package radius

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/vendors/microsoft"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// UserAuthenticator is a function type that authenticates a RADIUS user.
// It receives the username and password, and returns true if authentication succeeds.
type UserAuthenticator func(ctx context.Context, username, password string) (bool, error)

// AccountingHandler is a function type that handles RADIUS accounting requests.
// It is a type ALIAS (not a defined type) so the transport layer can wire a
// plain func via a structural interface assertion without importing infra
// (BE-B-13; layering gate forbids transport → infra).
type AccountingHandler = func(ctx context.Context, username string, sessionID string, eventType string, bytesIn, bytesOut int64)

// Server is a RADIUS authentication and accounting server.
type Server struct {
	addr     string
	secret   string
	authFunc func(ctx context.Context, username, password string) (bool, error)
	acctFunc AccountingHandler
	server   *radius.PacketServer
	mu       sync.Mutex
	done     chan struct{}
	stopOnce sync.Once
}

// NewServer creates a new RADIUS server.
func NewServer(addr, secret string) *Server {
	return &Server{
		addr:   addr,
		secret: secret,
		done:   make(chan struct{}),
	}
}

// SetAuthHandler sets the authentication handler function.
func (s *Server) SetAuthHandler(authFunc func(ctx context.Context, username, password string) (bool, error)) {
	s.authFunc = authFunc
}

// SetAcctHandler sets the accounting handler function.
func (s *Server) SetAcctHandler(acctFunc AccountingHandler) {
	s.acctFunc = acctFunc
}

// Start starts the RADIUS server. The passed context is the server LIFECYCLE
// context (BE-B-13): when it is cancelled the server shuts down. Callers must
// pass a detached long-lived context, never a per-request context.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	authHandler := s.buildAuthHandler()
	acctHandler := s.buildAcctHandler()

	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {
		switch r.Code {
		case radius.CodeAccessRequest:
			authHandler(w, r)
		case radius.CodeAccountingRequest:
			acctHandler(w, r)
		case radius.CodeAccessAccept, radius.CodeAccessReject, radius.CodeAccountingResponse,
			radius.CodeAccessChallenge, radius.CodeStatusServer, radius.CodeStatusClient,
			radius.CodeDisconnectRequest, radius.CodeDisconnectACK, radius.CodeDisconnectNAK,
			radius.CodeCoARequest, radius.CodeCoAACK, radius.CodeCoANAK, radius.CodeReserved:
			// 非请求类报文：服务器不回包，静默丢弃。
		}
	})

	secretSource := radius.StaticSecretSource([]byte(s.secret))

	s.server = &radius.PacketServer{
		Addr:         s.addr,
		SecretSource: secretSource,
		Handler:      handler,
	}

	recover.SafeGo("radius-server", func() {
		if err := s.server.ListenAndServe(); err != nil {
			slog.Error("radius server: ListenAndServe exited with error", "error", err)
		}
	})

	// Lifecycle watcher: honour the caller's lifecycle context. Stop() closes
	// s.done so the watcher also exits on an explicit stop (no goroutine leak).
	recover.SafeGo("radius-server-lifecycle", func() {
		select {
		case <-ctx.Done():
			if err := s.Stop(); err != nil {
				slog.Error("radius server: lifecycle shutdown failed", "error", err)
			}
		case <-s.done:
		}
	})

	return nil
}

// Stop stops the RADIUS server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopOnce.Do(func() { close(s.done) })
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// buildAuthHandler constructs the RADIUS authentication handler.
// It supports PAP (plaintext), CHAP, and MS-CHAPv2 authentication methods.
func (s *Server) buildAuthHandler() radius.HandlerFunc {
	if s.authFunc == nil {
		return s.defaultAuthHandler
	}

	return func(w radius.ResponseWriter, r *radius.Request) {
		username := rfc2865.UserName_GetString(r.Packet)
		if username == "" {
			_ = w.Write(r.Response(radius.CodeAccessReject))
			return
		}

		// Detect authentication method and extract password
		password, authMethod, err := s.extractPassword(r)
		if err != nil {
			slog.WarnContext(r.Context(), "radius auth: failed to extract password", "username", username, "error", err)
			_ = w.Write(r.Response(radius.CodeAccessReject))
			return
		}

		// Authenticate the user
		authenticated, err := s.authFunc(r.Context(), username, password)
		if err != nil {
			slog.WarnContext(r.Context(), "radius auth: error authenticating user", "username", username, "error", err)
			_ = w.Write(r.Response(radius.CodeAccessReject))
			return
		}

		if !authenticated {
			slog.InfoContext(r.Context(), "radius auth: authentication failed", "username", username, "method", authMethod)
			_ = w.Write(r.Response(radius.CodeAccessReject))
			return
		}

		// Authentication successful
		resp := r.Response(radius.CodeAccessAccept)
		_ = w.Write(resp)
	}
}

// extractPassword extracts the password from the RADIUS request based on the authentication method.
// Returns the plaintext password and the detected method name.
func (s *Server) extractPassword(r *radius.Request) (passwordOut, identityOut string, err error) {
	// Try PAP (User-Password attribute - decrypted by layeh.com/radius)
	password := rfc2865.UserPassword_GetString(r.Packet)
	if password != "" {
		return password, "PAP", nil
	}

	// Try CHAP (CHAP-Password attribute)
	chapPassword := rfc2865.CHAPPassword_Get(r.Packet)
	if len(chapPassword) > 0 {
		// CHAP requires the CHAP-Challenge and the shared secret for verification.
		// For now, we return the challenge as the "password" for verification against stored CHAP hash.
		// In a full implementation, the authenticator function would verify the CHAP response.
		chapChallenge := rfc2865.CHAPChallenge_Get(r.Packet)
		_ = chapChallenge
		return fmt.Sprintf("%x", chapPassword), "CHAP", nil
	}

	// Try MS-CHAPv2 (MS-CHAP-Response attribute from Microsoft vendor)
	msChapResponse := microsoft.MSCHAPResponse_Get(r.Packet)
	if len(msChapResponse) > 0 {
		// MS-CHAPv2 verification is complex and requires NT password hashes.
		// Return the response bytes for the authenticator to verify.
		return fmt.Sprintf("%x", msChapResponse), "MS-CHAPv2", nil
	}

	return "", "", apperrors.NewNotFound("no supported authentication method found")
}

// buildAcctHandler constructs the RADIUS accounting handler.
func (s *Server) buildAcctHandler() radius.HandlerFunc {
	if s.acctFunc == nil {
		return s.defaultAcctHandler
	}

	return func(w radius.ResponseWriter, r *radius.Request) {
		username := rfc2865.UserName_GetString(r.Packet)
		sessionID := rfc2866.AcctSessionID_GetString(r.Packet)
		statusType := rfc2866.AcctStatusType_Get(r.Packet)
		bytesIn := rfc2866.AcctInputOctets_Get(r.Packet)
		bytesOut := rfc2866.AcctOutputOctets_Get(r.Packet)

		eventType := fmt.Sprintf("Unknown(%d)", statusType)
		switch statusType {
		case rfc2866.AcctStatusType_Value_Start:
			eventType = "Start"
		case rfc2866.AcctStatusType_Value_Stop:
			eventType = "Stop"
		case rfc2866.AcctStatusType_Value_InterimUpdate:
			eventType = "InterimUpdate"
		case rfc2866.AcctStatusType_Value_AccountingOn,
			rfc2866.AcctStatusType_Value_AccountingOff,
			rfc2866.AcctStatusType_Value_Failed:
			// 这些状态不承载会话事件，沿用兜底词汇以免改变审计事件取值。
			eventType = fmt.Sprintf("Unknown(%d)", statusType)
		}

		if s.acctFunc != nil {
			s.acctFunc(r.Context(), username, sessionID, eventType, int64(bytesIn), int64(bytesOut))
		}

		_ = w.Write(r.Response(radius.CodeAccountingResponse))
	}
}

func (s *Server) defaultAuthHandler(w radius.ResponseWriter, r *radius.Request) {
	// Default: reject all. Override with SetAuthHandler for real authentication.
	_ = w.Write(r.Response(radius.CodeAccessReject))
}

func (s *Server) defaultAcctHandler(w radius.ResponseWriter, r *radius.Request) {
	_ = w.Write(r.Response(radius.CodeAccountingResponse))
}
