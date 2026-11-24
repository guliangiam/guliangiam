package auth

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// CAS handler provides CAS 1.0/2.0/3.0 protocol endpoints.

// casLogoutRedirectReport is the `redirect` query value that asks CAS logout for
// its adjudicated result as a JSON body instead of a 302.
const casLogoutRedirectReport = "report"

// CasLoginRedirect redirects to the CAS IdP login page.
func (h Handler) CasLoginRedirect(w http.ResponseWriter, r *http.Request) {
	serviceURL := r.URL.Query().Get("service")
	if serviceURL == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("service parameter is required"))
		return
	}

	// Redirect to the configured CAS IdP login URL
	casLoginURL := r.URL.Query().Get("casLoginUrl")
	if casLoginURL == "" {
		casLoginURL = "/api/v1/auth/login"
	}

	redirectURL := fmt.Sprintf("%s?service=%s", casLoginURL, url.QueryEscape(serviceURL))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// CasServiceValidate validates a CAS Service Ticket (CAS 2.0/3.0).
//
// CAS 3.0 §4.2.2: when the caller supplies `pgtUrl`, CAS binds a PGTIOU
// to the validation, persists the PGT + PGTIOU pair on the same row,
// and asynchronously POSTs the PGTIOU to `pgtUrl`. The proxy caller then
// exchanges the PGTIOU for the real PGT via /proxy.
func (h Handler) CasServiceValidate(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	ticket := r.URL.Query().Get("ticket")
	pgtURL := r.URL.Query().Get("pgtUrl")

	if service == "" || ticket == "" {
		writeCasErrorResponse(w, "INVALID_REQUEST", "service and ticket parameters are required")
		return
	}

	// CAS ticket format gate-keep (CAS 2.0/3.0 §3.1):
	// serviceValidate accepts ST- (service ticket), PT- (proxy ticket).
	// Reject anything else early — no need to hit the app layer.
	if !strings.HasPrefix(ticket, "ST-") && !strings.HasPrefix(ticket, "PT-") {
		writeCasErrorResponse(w, "INVALID_TICKET", "invalid ticket format (expected ST- or PT- prefix)")
		return
	}

	// Fail-close: handler must have a wired Service to validate against.
	// A nil Service means the transport layer was misconfigured at bootstrap.
	if h.Service == nil {
		writeCasErrorResponse(w, "INTERNAL_ERROR", "authorization service not configured") // naming-allow: CAS protocol standard code
		return
	}

	ctx := r.Context()
	result, err := h.validateCasTicketWithPgt(ctx, ticket, service, pgtURL)
	if err != nil {
		writeCasErrorResponse(w, "INVALID_TICKET", err.Error())
		return
	}

	writeCasSuccessResponse(w, result, false)
}

// CasProxyValidate validates a CAS Proxy Ticket (CAS 2.0/3.0).
func (h Handler) CasProxyValidate(w http.ResponseWriter, r *http.Request) {
	h.CasServiceValidate(w, r)
}

// CasProxy implements CAS 3.0 /proxy — exchange a PGTIOU (obtained via
// pgtUrl callback) for a PGT, then optionally issue a proxy ticket.
//
// Flow:
//
//	?pgtIou=<PGTIOU>                       → returns <proxyGrantingTicket> (the real PGT)
//	?pgtIou=<PGTIOU>&targetService=<url>   → additionally issues a proxy ticket
//	                                         and includes it in <cas:proxyTicket>
func (h Handler) CasProxy(w http.ResponseWriter, r *http.Request) {
	// CAS 3.0 /proxy endpoint is not supported by this IAM — the IdP only issues
	// service tickets, not proxy tickets or proxy granting tickets. CasProxyValidate
	// (the /proxyValidate variant) still works because it delegates to CasServiceValidate.
	writeCasErrorResponse(w, "UNAUTHORIZED_SERVICE", "proxy tickets are not supported")
}

// CasLogout handles CAS single logout (设计报告 28.10.5 FE-014).
//
// The `service` parameter is echoed back only when the application service has
// matched it against the organization's registered redirect URIs. When no
// session cookie is present there is no organization and therefore no
// whitelist, so the redirect is refused rather than passed through: echoing the
// caller's value is precisely the open redirect this endpoint must not have.
//
// `redirect=report` asks for the machine-readable form of the same decision
// instead of the 302. The console cannot read a Location header of a redirect it
// is not allowed to follow, and 42.1.2 forbids it from re-deriving the verdict,
// so the validated target has to arrive in the body. The protocol surface keeps
// the browser-facing 302 as its default so no CAS client has to change.
func (h Handler) CasLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	report := route.QueryOptionalString(r, "redirect")
	reportOnly := report != nil && *report == casLogoutRedirectReport

	sessionID := ""
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		sessionID = strings.TrimSpace(cookie.Value)
	}

	result := &appauth.CasLogoutResult{}
	switch {
	case sessionID == "" || h.Service == nil:
		result.RedirectRejected = r.URL.Query().Get("service") != ""
	default:
		meta := route.RequestMetadataFrom(ctx)
		computed, err := h.Service.CasLogout(ctx, appauth.CasLogoutCommand{
			SessionID: sessionID,
			IssuerURL: platformconfig.IssuerURL(r),
			Service:   r.URL.Query().Get("service"),
			IPAddress: util.ClientIP(r),
			UserAgent: r.UserAgent(),
			Surface:   string(meta.Surface),
			RequestID: meta.RequestID,
			TraceID:   meta.TraceID,
		})
		if err != nil {
			ClearSessionCookie(w)
			route.WriteError(w, r, err)
			return
		}
		result = computed
	}

	ClearSessionCookie(w)

	if result.RedirectURI != "" && !reportOnly {
		http.Redirect(w, r, result.RedirectURI, http.StatusFound)
		return
	}

	// A settled logout is a success even when the redirect was refused: 28.10.3
	// requires the entry point to stay idempotent, so the outcome is reported in
	// the body instead of as an error status the caller would have to interpret.
	// redirectUri is the single adjudicated target: a caller renders it or
	// navigates to it verbatim, and never re-uses the raw `service` value.
	route.WriteJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"revokedSessions":  result.RevokedSessions,
		"redirectRejected": result.RedirectRejected,
		"redirectUri":      result.RedirectURI,
	})
}

// validateCasTicket validates a CAS ticket and returns user info.
func (h Handler) validateCasTicket(ctx context.Context, ticket string, service string) (*casAuthResult, error) {
	// The ticket format is: ST-{sessionID}
	if !strings.HasPrefix(ticket, "ST-") {
		return nil, apperrors.NewInvalidArgument("invalid ticket format")
	}

	sessionID := strings.TrimPrefix(ticket, "ST-")

	// We need to resolve the session to get user info.
	// For CAS, we extract the session context from the session store.
	// This requires looking up the session by ID.
	sessionCtx, ok := h.resolveSessionByID(ctx, sessionID)
	if !ok {
		return nil, apperrors.NewNotFound("ticket validation failed: session not found")
	}

	return &casAuthResult{
		User: sessionCtx.Username,
		Org:  sessionCtx.OrganizationSlug,
	}, nil
}

// resolveSessionByID attempts to resolve a session context from a session ID.
// This is a simplified implementation — production use should validate against the session store.
func (h Handler) resolveSessionByID(ctx context.Context, sessionID string) (*sessionContext, bool) {
	// Use the existing session middleware's resolution logic
	// For now, return a basic context — full implementation would query the session store
	return nil, false
}

type sessionContext struct {
	Username         string
	OrganizationSlug string
}

type casAuthResult struct {
	User   string `xml:"user"`
	Org    string `xml:"organization"`
	PgtIou string `xml:"-"`
}

// validateCasTicketWithPgt validates a CAS ticket via the application service
// (not the stub resolveSessionByID), and optionally processes the pgtUrl chain.
func (h Handler) validateCasTicketWithPgt(ctx context.Context, ticket, service, pgtURL string) (*casAuthResult, error) {
	// Forward to the app-level CAS validation which consumes the ST atomically
	// and handles pgtUrl → PGTIOU binding.
	_ = ticket
	_ = service
	_ = pgtURL
	return h.appValidateCasTicket(ctx, ticket, service, pgtURL)
}

func (h Handler) appValidateCasTicket(ctx context.Context, ticket, service, pgtURL string) (*casAuthResult, error) {
	orgSlug := "" // CAS serviceValidate doesn't carry org slug directly — app layer resolves via ticket owner
	result, err := h.Service.CASServiceValidate(ctx, appauth.CASServiceValidateCommand{
		OrganizationSlug: orgSlug,
		Ticket:           ticket,
		Service:          service,
		PgtURL:           pgtURL,
	})
	if err != nil {
		return nil, err
	}
	return &casAuthResult{
		User:   result.Username,
		PgtIou: result.PgtIou,
	}, nil
}

type casXMLProxySuccessResponse struct {
	XMLName             xml.Name `xml:"http://www.yale.edu/tp/cas serviceResponse"`
	XMLNS               string   `xml:"xmlns:cas,attr"`
	ProxyGrantingTicket string   `xml:"cas:proxyGrantingTicket,omitempty"`
	ProxyTicket         string   `xml:"cas:proxyTicket,omitempty"`
}

// writeCasSuccessResponse writes a CAS 2.0 XML success response.
func writeCasSuccessResponse(w http.ResponseWriter, result *casAuthResult, includeAttributes bool) {
	resp := casXMLServiceResponse{
		AuthSuccess: &casXMLAuthSuccess{
			User:   result.User,
			PgtIou: result.PgtIou,
		},
	}

	w.Header().Set("Content-Type", "application/xml;charset=UTF-8")
	output, _ := xml.Marshal(resp)
	w.Write(output)
}

// writeCasErrorResponse writes a CAS 2.0 XML error response.
func writeCasErrorResponse(w http.ResponseWriter, code string, message string) {
	resp := casXMLServiceResponse{
		AuthFailure: &casXMLAuthFailure{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/xml;charset=UTF-8")
	w.WriteHeader(http.StatusUnauthorized)
	output, _ := xml.Marshal(resp)
	w.Write(output)
}

// CAS XML response types
type casXMLServiceResponse struct {
	XMLName     xml.Name           `xml:"http://www.yale.edu/tp/cas serviceResponse"`
	AuthSuccess *casXMLAuthSuccess `xml:"authenticationSuccess,omitempty"`
	AuthFailure *casXMLAuthFailure `xml:"authenticationFailure,omitempty"`
}

type casXMLAuthSuccess struct {
	User   string `xml:"user"`
	PgtIou string `xml:"proxyGrantingTicket,omitempty"`
}

type casXMLAuthFailure struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}
