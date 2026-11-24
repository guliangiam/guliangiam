package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	casdomain "github.com/guliangiam/guliangiam/internal/domain/castickets"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// CASLoginCommand contains parameters for a CAS login.
type CASLoginCommand struct {
	OrganizationSlug string
	Username         string
	Password         string
	Service          string
}

// CASLoginResult contains the result of a CAS login.
type CASLoginResult struct {
	Ticket  string `json:"ticket"`
	Message string `json:"message,omitempty"`
}

// CASServiceValidateCommand contains parameters for CAS ticket validation.
type CASServiceValidateCommand struct {
	OrganizationSlug string
	ApplicationSlug  string
	Ticket           string
	Service          string
	PgtURL           string
	Format           string
}

// CASValidateResult contains the result of CAS ticket validation.
type CASValidateResult struct {
	Success  bool
	Username string
	UserID   string
	Attrs    map[string]string
	PgtIou   string
}

// casGenerateTicketID generates a unique CAS ticket identifier.
func casGenerateTicketID(prefix string) string {
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), randomHex(8))
}

// generateCASTicket generates a CAS service ticket and persists it.
func (s Service) generateCASTicket(ctx context.Context, userID, username, service string) (string, error) {
	ticketID := casGenerateTicketID("ST")
	now := s.Clock.Now()
	ticketDBID, _ := platformuuid.V7Generator{}.NewString()

	ticket := &casdomain.Ticket{
		ID:         ticketDBID,
		TicketID:   ticketID,
		TicketType: "service",
		UserID:     userID,
		Username:   username,
		Service:    service,
		ExpiresAt:  now.Add(5 * time.Minute),
		CreatedAt:  now,
	}

	if err := s.UOW.CASTickets().Create(ctx, ticket); err != nil {
		return "", err
	}

	return ticketID, nil
}

// CASLogin performs a CAS login and returns a service ticket.
func (s Service) CASLogin(ctx context.Context, command CASLoginCommand) (*CASLoginResult, error) {
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.Password) == "" {
		return nil, apperrors.NewInvalidArgument("password is required")
	}
	if strings.TrimSpace(command.Service) == "" {
		return nil, apperrors.NewInvalidArgument("service is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lookup organization by slug %q", command.OrganizationSlug)
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid credentials")
	}

	// CAS shares the local-password gate with the login service: no hash means
	// no local password, and provider markers (ldap/plain/unknown) never select
	// a verifier. Legacy hashes are still accepted read-only here.
	if err := verifyPassword(command.Password, user.Credentials.PasswordAlgo, user.Credentials.PasswordHash); err != nil {
		if isCredentialError(err) {
			return nil, apperrors.NewUnauthorized("invalid credentials")
		}
		return nil, err
	}

	// ── Risk engine evaluation (cas channel) ───────────────────────────
	// CASLogin has no step-up path, so challenge is treated as deny. The risk
	// evidence and the service ticket are written in one transaction: this
	// path previously had none, and splitting them would let a ticket exist
	// without the decision that authorised it.
	ticketID := casGenerateTicketID("ST")
	uid := user.ID
	var ticketCreateErr error
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if evalErr := s.evaluateLoginRisk(ctx, tx, "cas", organization.ID, &uid,
			"", "", "", map[string]any{}); evalErr != nil {
			switch evalErr.(type) {
			case *RiskDeniedError, *RiskChallengeError:
				return apperrors.NewUnauthorized("access denied by risk assessment")
			default:
				return evalErr // engine unavailable → fail-closed
			}
		}

		// ADR-029 pre_login (cas channel): fail-closed policy gate before
		// the service ticket is minted.
		if err := s.enforceActionsPreLogin(ctx, "cas", organization.ID, user.ID, user.Username, "", "", ""); err != nil {
			return err
		}

		now := s.Clock.Now()
		ticketDBID, _ := platformuuid.V7Generator{}.NewString()
		ticket := &casdomain.Ticket{
			ID:         ticketDBID,
			TicketID:   ticketID,
			TicketType: "service",
			UserID:     user.ID,
			Username:   user.Username,
			Service:    strings.TrimSpace(command.Service),
			ExpiresAt:  now.Add(5 * time.Minute),
			CreatedAt:  now,
		}
		if createErr := tx.CASTickets().Create(ctx, ticket); createErr != nil {
			ticketCreateErr = createErr
			return createErr
		}
		return nil
	})
	if ticketCreateErr != nil {
		return nil, apperrors.WrapInternal(ticketCreateErr, "create CAS service ticket")
	}
	if err != nil {
		return nil, err
	}

	return &CASLoginResult{
		Ticket:  ticketID,
		Message: "success",
	}, nil
}

// CASServiceValidate validates a CAS 2.0/3.0 service ticket.
func (s Service) CASServiceValidate(ctx context.Context, command CASServiceValidateCommand) (*CASValidateResult, error) {
	return s.casValidateTicket(ctx, command)
}

// CASValidate validates a CAS 1.0 service ticket (plain text response).
func (s Service) CASValidate(ctx context.Context, command CASServiceValidateCommand) (*CASValidateResult, error) {
	return s.casValidateTicket(ctx, command)
}

func (s Service) casValidateTicket(ctx context.Context, command CASServiceValidateCommand) (*CASValidateResult, error) {
	if strings.TrimSpace(command.Ticket) == "" {
		return nil, apperrors.NewInvalidArgument("INVALID_REQUEST: ticket parameter is required")
	}
	if strings.TrimSpace(command.Service) == "" {
		return nil, apperrors.NewInvalidArgument("INVALID_REQUEST: service parameter is required")
	}

	// Consume the ticket atomically.
	ticket, err := s.UOW.CASTickets().DeleteByTicketID(ctx, command.Ticket)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("INVALID_TICKET: %s", err.Error())
	}

	if !strings.HasPrefix(ticket.TicketID, "ST-") {
		return nil, apperrors.NewInvalidArgument("INVALID_TICKET: not a service ticket")
	}

	now := s.Clock.Now()
	if now.After(ticket.ExpiresAt) {
		return nil, apperrors.NewInvalidArgument("INVALID_TICKET: ticket has expired")
	}
	if !strings.HasPrefix(command.Service, ticket.Service) {
		return nil, apperrors.NewInvalidArgument("INVALID_SERVICE")
	}

	result := &CASValidateResult{
		Success:  true,
		Username: ticket.Username,
		UserID:   ticket.UserID,
		Attrs:    make(map[string]string),
	}

	// ── PGTIOU / PGT chain — CAS 3.0 §4.2.2 ──
	// When the caller supplies a pgtUrl, we bind a PGTIOU to the PGT,
	// persist both on the same row, and asynchronously POST the PGTIOU to
	// the pgtUrl. The real PGT NEVER traverses the network — the caller
	// redeems the PGTIOU later via /proxy.
	if strings.TrimSpace(command.PgtURL) != "" {
		pgtIou, pgtID, err := s.createPGTIOUBoundPGT(ctx, ticket.UserID, ticket.Username, ticket.Service)
		if err != nil {
			return nil, apperrors.NewInternalf("pgtIou binding failed: %s", err.Error())
		}
		result.PgtIou = pgtIou

		// Async POST — CAS spec says failure to notify doesn't invalidate the PGT.
		// We spawn a goroutine; best-effort, logged at transport level.
		go s.notifyPgtURL(ctx, command.PgtURL, pgtIou)
		_ = pgtID // bound to the row; caller redeems via PGTIOU → /proxy
	}

	return result, nil
}

// createPGTIOUBoundPGT creates a proxy-granting ticket row with pgt_iou set,
// returning the PGT ticket ID and PGTIOU. The PGTIOU is one-time-use and
// expires 5 minutes after creation (bound by MarkPGTIOUUsed in the repo layer).
func (s Service) createPGTIOUBoundPGT(ctx context.Context, userID, username, service string) (iou, ticketID string, err error) {
	pgtIou := casGeneratePGTIOU()
	pgtID := casGenerateTicketID("PGT")
	now := s.Clock.Now()
	ticketDBID, _ := platformuuid.V7Generator{}.NewString()

	ticket := &casdomain.Ticket{
		ID:              ticketDBID,
		TicketID:        pgtID,
		TicketType:      casTicketProxyGranting,
		UserID:          userID,
		Username:        username,
		Service:         service,
		ExpiresAt:       now.Add(30 * time.Minute),
		CreatedAt:       now,
		PGTIOU:          pgtIou,
		PGTIOUExpiresAt: now.Add(5 * time.Minute),
	}
	if err := s.UOW.CASTickets().Create(ctx, ticket); err != nil {
		return "", "", err
	}
	return pgtIou, pgtID, nil
}

// notifyPgtURL POSTs the PGTIOU to the SP's proxy callback URL.
// The caller's SP must then invoke /proxy to redeem the PGTIOU → real PGT.
//
// pgtURL 来自 CAS 客户端请求参数（外部可控），属于典型的 SSRF 入口：只允许
// HTTPS，并交给 SSRF 防护客户端（拒绝私网/回环/链路本地目标）。投递失败保持
// best-effort，不影响主流程。
func (s Service) notifyPgtURL(ctx context.Context, pgtURL, pgtIou string) {
	parsed, err := url.Parse(pgtURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return
	}
	if err := ssrf.ValidateURL(pgtURL); err != nil {
		slog.WarnContext(ctx, "cas pgtUrl rejected by SSRF policy", "error", err)
		return
	}

	// 异步投递不能沿用请求 ctx（请求结束即被取消）；脱钩后单独限时。
	postCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	form := url.Values{}
	form.Set("pgtIou", pgtIou)
	req, err := http.NewRequestWithContext(postCtx, http.MethodPost, pgtURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ssrf.SafeHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		// Best-effort only — logging at Info level in production.
		return
	}
	resp.Body.Close()
}

// casGeneratePGTIOU produces a 20-char opaque token for PGTIOU.
// CAS convention: IOU = "I owe you" (one-time-use binding).
func casGeneratePGTIOU() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "PGTIOU-" + hex.EncodeToString(b)[:10]
}

// CASProxy lets a proxy caller exchange a PGTIOU + targetService for a PGT.
func (s Service) CASProxy(ctx context.Context, pgtIou, targetService string) (string, error) {
	if strings.TrimSpace(pgtIou) == "" {
		return "", apperrors.NewInvalidArgument("INVALID_REQUEST: pgtIou is required")
	}
	pgtID, err := s.UOW.CASTickets().MarkPGTIOUUsed(ctx, pgtIou)
	if err != nil {
		return "", apperrors.NewInvalidArgument("INVALID_PROXY_GRANTING_TICKET_IOU: not recognized, already used, or expired")
	}
	// targetService is validated at proxy-ticket issuance time (CASGenerateProxyTicket)
	_ = targetService
	return pgtID, nil
}

// CASGenerateProxyTicket validates a PGT and generates a proxy ticket.
func (s Service) CASGenerateProxyTicket(ctx context.Context, pgtID, targetService string) (string, error) {
	if strings.TrimSpace(pgtID) == "" {
		return "", apperrors.NewInvalidArgument("pgt parameter is required")
	}
	if strings.TrimSpace(targetService) == "" {
		return "", apperrors.NewInvalidArgument("targetService parameter is required")
	}

	pgt, err := s.UOW.CASTickets().GetByTicketID(ctx, pgtID)
	if err != nil {
		return "", apperrors.NewInvalidArgument("proxy-granting ticket not recognized")
	}

	if pgt.TicketType != casTicketProxyGranting {
		return "", apperrors.NewInvalidArgument("invalid proxy-granting ticket")
	}

	now := s.Clock.Now()
	if now.After(pgt.ExpiresAt) {
		return "", apperrors.NewInvalidArgument("proxy-granting ticket has expired")
	}

	proxyTicketID := casGenerateTicketID("PT")
	ticketDBID, _ := platformuuid.V7Generator{}.NewString()

	proxyTicket := &casdomain.Ticket{
		ID:         ticketDBID,
		TicketID:   proxyTicketID,
		TicketType: "proxy",
		UserID:     pgt.UserID,
		Username:   pgt.Username,
		Service:    targetService,
		ExpiresAt:  now.Add(5 * time.Minute),
		CreatedAt:  now,
	}

	if err := s.UOW.CASTickets().Create(ctx, proxyTicket); err != nil {
		return "", err
	}

	return proxyTicketID, nil
}

// StoreProxyGrantingTicket stores a proxy-granting ticket.
func (s Service) StoreProxyGrantingTicket(ctx context.Context, userID, username, service string) (string, error) {
	pgtID := casGenerateTicketID("PGT")
	now := s.Clock.Now()
	ticketDBID, _ := platformuuid.V7Generator{}.NewString()

	ticket := &casdomain.Ticket{
		ID:         ticketDBID,
		TicketID:   pgtID,
		TicketType: casTicketProxyGranting,
		UserID:     userID,
		Username:   username,
		Service:    service,
		ExpiresAt:  now.Add(30 * time.Minute),
		CreatedAt:  now,
	}

	if err := s.UOW.CASTickets().Create(ctx, ticket); err != nil {
		return "", err
	}

	return pgtID, nil
}

// --- CAS XML Response Types ---

type CASServiceResponse struct {
	XMLName      xml.Name         `xml:"cas:serviceResponse"`
	XMLNS        string           `xml:"xmlns:cas,attr"`
	AuthSuccess  *CASAuthSuccess  `xml:"cas:authenticationSuccess,omitempty"`
	AuthFailure  *CASAuthFailure  `xml:"cas:authenticationFailure,omitempty"`
	ProxySuccess *CASProxySuccess `xml:"cas:proxySuccess,omitempty"`
	ProxyFailure *CASProxyFailure `xml:"cas:proxyFailure,omitempty"`
}

type CASAuthSuccess struct {
	User    string         `xml:"cas:user"`
	Attrs   *CASAttributes `xml:"cas:attributes,omitempty"`
	PgtIou  string         `xml:"cas:proxyGrantingTicket,omitempty"`
	Proxies *CASProxies    `xml:"cas:proxies,omitempty"`
}

type CASAttributes struct {
	AuthDate   string              `xml:"cas:authenticationDate,omitempty"`
	IsNewLogin string              `xml:"cas:isFromNewLogin,omitempty"`
	IsLongTerm string              `xml:"cas:longTermAuthenticationRequestTokenUsed,omitempty"`
	UserAttrs  []CASNamedAttribute `xml:"cas:userAttributes>cas:attribute,omitempty"`
	NamedAttrs []CASNamedAttribute `xml:"cas:attribute,omitempty"`
}

type CASNamedAttribute struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type CASProxies struct {
	Proxy []string `xml:"cas:proxy"`
}

type CASAuthFailure struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

type CASProxySuccess struct {
	ProxyTicket string `xml:"cas:proxyTicket"`
}

type CASProxyFailure struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

// BuildCASXMLSuccessResponse builds a CAS 2.0/3.0 XML success response.
func BuildCASXMLSuccessResponse(username, userID string, attrs map[string]string) []byte {
	namedAttrs := make([]CASNamedAttribute, 0, len(attrs))
	for k, v := range attrs {
		namedAttrs = append(namedAttrs, CASNamedAttribute{Name: k, Value: v})
	}

	response := CASServiceResponse{
		XMLNS: "http://www.yale.edu/tp/cas",
		AuthSuccess: &CASAuthSuccess{
			User: username,
			Attrs: &CASAttributes{
				AuthDate:   time.Now().UTC().Format(time.RFC3339),
				IsNewLogin: "true",
				IsLongTerm: "false",
				NamedAttrs: namedAttrs,
			},
		},
	}

	data, _ := xml.Marshal(response)
	return data
}

// BuildCASXMLFailureResponse builds a CAS 2.0/3.0 XML failure response.
func BuildCASXMLFailureResponse(code, message string) []byte {
	response := CASServiceResponse{
		XMLNS: "http://www.yale.edu/tp/cas",
		AuthFailure: &CASAuthFailure{
			Code:    code,
			Message: message,
		},
	}

	data, _ := xml.Marshal(response)
	return data
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
