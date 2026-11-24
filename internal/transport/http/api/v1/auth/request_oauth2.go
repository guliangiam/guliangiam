package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type AuthorizeRequest struct {
	ClientID        string   `json:"clientId"`
	RedirectURI     string   `json:"redirectUri"`
	ResponseType    string   `json:"responseType,omitempty"`
	RequestedScopes []string `json:"requestedScopes,omitempty"`
	State           string   `json:"state,omitempty"`
	Nonce           string   `json:"nonce,omitempty"`
	CodeChallenge   string   `json:"codeChallenge,omitempty"`
	ChallengeMethod string   `json:"codeChallengeMethod,omitempty"`
	Resource        string   `json:"resource,omitempty"`
}

type ExchangeAuthorizationCodeRequest struct {
	GrantType    string `json:"grantType,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	RedirectURI  string `json:"redirectUri,omitempty"`
	CodeVerifier string `json:"codeVerifier,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Resource     string `json:"resource,omitempty"`

	ClientAssertion     string `json:"clientAssertion,omitempty"`
	ClientAssertionType string `json:"clientAssertionType,omitempty"`
	// ClientAuthMethod is derived by the decoder from the credentials actually
	// present (Authorization: Basic / body client_secret / assertion / mTLS),
	// never from client input.
	ClientAuthMethod string `json:"-"`
}

func (r ExchangeAuthorizationCodeRequest) Validate() error {
	if err := validateOptionalString("grantType", r.GrantType); err != nil {
		return err
	}
	if err := validateOptionalString("clientId", r.ClientID); err != nil {
		return err
	}
	if len(r.ClientSecret) > maxStringLength {
		return apperrors.NewInvalidArgument("clientSecret exceeds maximum length")
	}
	if err := validateOptionalString("code", r.Code); err != nil {
		return err
	}
	if err := validateOptionalString("redirectUri", r.RedirectURI); err != nil {
		return err
	}
	if len(r.CodeVerifier) > 128 {
		return apperrors.NewInvalidArgument("codeVerifier exceeds maximum length (128 chars)")
	}
	if len(r.CodeVerifier) < 43 && r.CodeVerifier != "" {
		return apperrors.NewInvalidArgument("codeVerifier too short (min 43 chars for PKCE)")
	}
	if len(r.ClientAssertion) > maxStringLength {
		return apperrors.NewInvalidArgument("clientAssertion exceeds maximum length")
	}
	if r.ClientAssertionType != "" && r.ClientAssertionType != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
		return apperrors.NewInvalidArgument("clientAssertionType must be urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	}
	if r.ClientAssertion != "" && r.ClientSecret != "" {
		return apperrors.NewInvalidArgument("a client assertion cannot be combined with a client secret")
	}
	if err := validateOptionalString("scope", r.Scope); err != nil {
		return err
	}
	if err := validateOptionalString("resource", r.Resource); err != nil {
		return err
	}
	return nil
}

func (r AuthorizeRequest) Validate() error {
	if err := validateOptionalString("clientId", r.ClientID); err != nil {
		return err
	}
	if err := validateOptionalString("redirectUri", r.RedirectURI); err != nil {
		return err
	}
	if err := validateOptionalString("responseType", r.ResponseType); err != nil {
		return err
	}
	if err := validateStringSlice("requestedScopes", r.RequestedScopes); err != nil {
		return err
	}
	if err := validateOptionalString("state", r.State); err != nil {
		return err
	}
	if err := validateOptionalString("nonce", r.Nonce); err != nil {
		return err
	}
	if err := validateOptionalString("codeChallenge", r.CodeChallenge); err != nil {
		return err
	}
	if err := validateOptionalString("codeChallengeMethod", r.ChallengeMethod); err != nil {
		return err
	}
	if err := validateOptionalString("resource", r.Resource); err != nil {
		return err
	}
	return nil
}

type GrantConsentRequest struct {
	ApplicationID string   `json:"applicationId"`
	GrantedScopes []string `json:"grantedScopes"`
}

type EvaluateConsentRequest struct {
	ApplicationID   string   `json:"applicationId"`
	RequestedScopes []string `json:"requestedScopes"`
}

type RevokeConsentRequest struct {
	ApplicationID string   `json:"applicationId"`
	GrantedScopes []string `json:"grantedScopes"`
}

type JwtBearerTokenRequest struct {
	GrantType           string `json:"grant_type"`
	ClientAssertionType string `json:"client_assertion_type"`
	ClientAssertion     string `json:"client_assertion"`
	Scope               string `json:"scope,omitempty"`
}

type TokenExchangeRequest struct {
	GrantType          string `json:"grant_type"`
	SubjectToken       string `json:"subject_token"`
	SubjectTokenType   string `json:"subject_token_type"`
	ActorToken         string `json:"actor_token,omitempty"`
	ActorTokenType     string `json:"actor_token_type,omitempty"`
	Resource           string `json:"resource,omitempty"`
	Audience           string `json:"audience,omitempty"`
	Scope              string `json:"scope,omitempty"`
	ClientID           string `json:"client_id,omitempty"`
	ClientSecret       string `json:"client_secret,omitempty"`
	RequestedTokenType string `json:"requested_token_type,omitempty"`
}

type TokenIntrospectionRequest struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type_hint,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
}

type BackchannelLogoutRequest struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId,omitempty"`
	ClientID  string `json:"clientId"`
}

type DPoPProofValidationRequest struct {
	DPoPHeader string `json:"dpopHeader"`
	Method     string `json:"method"`
	URL        string `json:"url"`
}

type ClientAuthRequest struct {
	ClientID            string `json:"clientId"`
	ClientSecret        string `json:"clientSecret,omitempty"`
	ClientAssertion     string `json:"clientAssertion,omitempty"`
	ClientAssertionType string `json:"clientAssertionType,omitempty"`
}

func (r AuthorizeRequest) toCommand(httpRequest *http.Request) appauth.AuthorizeCommand {
	actor := requestFromHTTPRequest(httpRequest)
	return appauth.AuthorizeCommand{
		OrganizationSlug: actor.OrganizationSlug,
		Username:         actor.Username,
		ClientID:         r.ClientID,
		RedirectURI:      r.RedirectURI,
		ResponseType:     r.ResponseType,
		RequestedScopes:  append([]string(nil), r.RequestedScopes...),
		State:            r.State,
		Nonce:            r.Nonce,
		CodeChallenge:    r.CodeChallenge,
		ChallengeMethod:  r.ChallengeMethod,
		Resource:         r.Resource,
		SessionID:        actor.Sid,
		AuthTime:         actor.AuthTime,
	}
}

func (r ExchangeAuthorizationCodeRequest) toCommand(httpRequest *http.Request) appauth.ExchangeAuthorizationCodeCommand {
	command := appauth.ExchangeAuthorizationCodeCommand{
		GrantType:           r.GrantType,
		ClientID:            r.ClientID,
		ClientSecret:        r.ClientSecret,
		Code:                r.Code,
		RedirectURI:         r.RedirectURI,
		CodeVerifier:        r.CodeVerifier,
		Resource:            r.Resource,
		ClientAuthMethod:    r.ClientAuthMethod,
		ClientAssertion:     r.ClientAssertion,
		ClientAssertionType: r.ClientAssertionType,
		// The DPoP proof's `htu` must be the URL the client POSTed to, so the
		// server derives the same absolute URL here rather than letting the
		// service compare against an empty string.
		DPoPURL:   platformconfig.AbsoluteRequestURL(httpRequest),
		IssuerURL: platformconfig.IssuerURL(httpRequest),
		// Same client certificate chain used by client_credentials: mTLS client
		// authentication is orthogonal to the grant type, so the
		// authorization-code exchange must see the certificate too.
		PeerCertificates: platformmtls.PeerCertificatesFromRequest(httpRequest),
	}
	// RFC 8705: when TLS termination exposes the client certificate, record its
	// x5t#S256 so the access token can be certificate-bound.
	if thumbprint, err := platformmtls.ThumbprintFromRequest(httpRequest); err == nil {
		command.ClientCertificateThumbprint = thumbprint
	}
	return command
}

func (r ExchangeAuthorizationCodeRequest) isClientCredentialsGrant() bool {
	return strings.TrimSpace(r.GrantType) == "client_credentials"
}

func (r ExchangeAuthorizationCodeRequest) isRefreshTokenGrant() bool {
	return strings.TrimSpace(r.GrantType) == "refresh_token"
}

func (r ExchangeAuthorizationCodeRequest) toRefreshTokenCommand(httpRequest *http.Request) appauth.ExchangeRefreshTokenCommand {
	return appauth.ExchangeRefreshTokenCommand{
		RefreshToken:        r.RefreshToken,
		ClientID:            r.ClientID,
		ClientSecret:        r.ClientSecret,
		Scope:               r.Scope,
		ClientAuthMethod:    r.ClientAuthMethod,
		ClientAssertion:     r.ClientAssertion,
		ClientAssertionType: r.ClientAssertionType,
		PeerCertificates:    platformmtls.PeerCertificatesFromRequest(httpRequest),
		TokenEndpointURL:    platformconfig.AbsoluteRequestURL(httpRequest),
	}
}

func (r ExchangeAuthorizationCodeRequest) toClientCredentialsCommand(httpRequest *http.Request) appauth.ExchangeClientCredentialsCommand {
	return appauth.ExchangeClientCredentialsCommand{
		ClientID:            r.ClientID,
		ClientSecret:        r.ClientSecret,
		Scope:               r.Scope,
		ClientAuthMethod:    r.ClientAuthMethod,
		ClientAssertion:     r.ClientAssertion,
		ClientAssertionType: r.ClientAssertionType,
		PeerCertificates:    platformmtls.PeerCertificatesFromRequest(httpRequest),
		// Same absolute URL used for DPoP `htu`; it is the `aud` a
		// private_key_jwt assertion must declare (RFC 7523 §3).
		TokenEndpointURL: platformconfig.AbsoluteRequestURL(httpRequest),
	}
}

func (r GrantConsentRequest) toCommand(httpRequest *http.Request) appauth.GrantConsentCommand {
	actor := requestFromHTTPRequest(httpRequest)
	return appauth.GrantConsentCommand{
		OrganizationSlug: actor.OrganizationSlug,
		Username:         actor.Username,
		ApplicationID:    r.ApplicationID,
		GrantedScopes:    append([]string(nil), r.GrantedScopes...),
	}
}

func (r EvaluateConsentRequest) toCommand(httpRequest *http.Request) appauth.EvaluateConsentCommand {
	actor := requestFromHTTPRequest(httpRequest)
	return appauth.EvaluateConsentCommand{
		OrganizationSlug: actor.OrganizationSlug,
		Username:         actor.Username,
		ApplicationID:    r.ApplicationID,
		RequestedScopes:  append([]string(nil), r.RequestedScopes...),
	}
}

func (r RevokeConsentRequest) toCommand(httpRequest *http.Request) appauth.RevokeConsentCommand {
	actor := requestFromHTTPRequest(httpRequest)
	return appauth.RevokeConsentCommand{
		OrganizationSlug: actor.OrganizationSlug,
		Username:         actor.Username,
		ApplicationID:    r.ApplicationID,
		GrantedScopes:    append([]string(nil), r.GrantedScopes...),
		CurrentSessionID: actor.SessionID,
	}
}

func (r JwtBearerTokenRequest) toCommand(httpRequest *http.Request) appauth.GetJwtBearerTokenCommand {
	return appauth.GetJwtBearerTokenCommand{
		ClientAssertionType: r.ClientAssertionType,
		ClientAssertion:     r.ClientAssertion,
		Scope:               r.Scope,
		TokenEndpointURL:    platformconfig.AbsoluteRequestURL(httpRequest),
	}
}

func (r TokenExchangeRequest) toCommand() appauth.GetTokenExchangeCommand {
	return appauth.GetTokenExchangeCommand{
		GrantType:          r.GrantType,
		SubjectToken:       r.SubjectToken,
		SubjectTokenType:   r.SubjectTokenType,
		ActorToken:         r.ActorToken,
		ActorTokenType:     r.ActorTokenType,
		Resource:           r.Resource,
		Audience:           r.Audience,
		Scope:              r.Scope,
		ClientID:           r.ClientID,
		ClientSecret:       r.ClientSecret,
		RequestedTokenType: r.RequestedTokenType,
	}
}

func (r TokenIntrospectionRequest) toCommand() appauth.IntrospectTokenCommand {
	return appauth.IntrospectTokenCommand{
		Token:     r.Token,
		TokenType: r.TokenType,
		ClientID:  r.ClientID,
	}
}

func (r BackchannelLogoutRequest) toCommand(httpRequest *http.Request) appauth.SendBackchannelLogoutCommand {
	session, _ := SessionContextFromContext(httpRequest.Context())
	return appauth.SendBackchannelLogoutCommand{
		UserID:         r.UserID,
		OrganizationID: session.OrganizationID,
		ActorID:        session.UserID,
		ActorType:      session.PrincipalType,
		SessionID:      r.SessionID,
		ClientID:       r.ClientID,
		IssuerURL:      platformconfig.IssuerURL(httpRequest),
	}
}

func (r DPoPProofValidationRequest) toCommand() appauth.ValidateDPoPProofCommand {
	return appauth.ValidateDPoPProofCommand{
		DPoPHeader: r.DPoPHeader,
		Method:     r.Method,
		URL:        r.URL,
	}
}

func (r ClientAuthRequest) toCommand(httpRequest *http.Request) appauth.AuthenticateClientCommand {
	return appauth.AuthenticateClientCommand{
		ClientID:            r.ClientID,
		ClientSecret:        r.ClientSecret,
		ClientAssertion:     r.ClientAssertion,
		ClientAssertionType: r.ClientAssertionType,
		PeerCertificates:    platformmtls.PeerCertificatesFromRequest(httpRequest),
		TokenEndpointURL:    platformconfig.AbsoluteRequestURL(httpRequest),
	}
}

// parseBasicClientCredentials decodes an Authorization: Basic header per
// RFC 6749 §2.3.1: each of client_id and client_secret is
// application/x-www-form-urlencoded before being joined with ":" and base64
// encoded. net/http's r.BasicAuth() only base64-decodes, so a client that
// follows the RFC (e.g. a secret containing "+" or "%") would otherwise fail.
func parseBasicClientCredentials(r *http.Request) (string, string, bool, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" || len(header) < 6 || !strings.EqualFold(header[:6], "basic ") {
		return "", "", false, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[6:]))
	if err != nil {
		return "", "", false, appauth.NewInvalidClientError("invalid Authorization header")
	}
	rawID, rawSecret, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", "", false, appauth.NewInvalidClientError("invalid Authorization header")
	}
	clientID, err := url.QueryUnescape(rawID)
	if err != nil {
		return "", "", false, appauth.NewInvalidClientError("invalid Authorization header")
	}
	clientSecret, err := url.QueryUnescape(rawSecret)
	if err != nil {
		return "", "", false, appauth.NewInvalidClientError("invalid Authorization header")
	}
	return clientID, clientSecret, true, nil
}

func decodeExchangeAuthorizationCodeRequest(r *http.Request) (ExchangeAuthorizationCodeRequest, error) {
	var request ExchangeAuthorizationCodeRequest
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "application/json") {
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return ExchangeAuthorizationCodeRequest{}, apperrors.NewInvalidArgument(err.Error())
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return ExchangeAuthorizationCodeRequest{}, apperrors.NewInvalidArgument(err.Error())
		}
		request = ExchangeAuthorizationCodeRequest{
			GrantType:           strings.TrimSpace(r.Form.Get("grant_type")),
			ClientID:            strings.TrimSpace(r.Form.Get("client_id")),
			ClientSecret:        strings.TrimSpace(r.Form.Get("client_secret")),
			Code:                strings.TrimSpace(r.Form.Get("code")),
			RefreshToken:        strings.TrimSpace(r.Form.Get("refresh_token")),
			RedirectURI:         strings.TrimSpace(r.Form.Get("redirect_uri")),
			CodeVerifier:        strings.TrimSpace(r.Form.Get("code_verifier")),
			Scope:               strings.TrimSpace(r.Form.Get("scope")),
			Resource:            strings.TrimSpace(r.Form.Get("resource")),
			ClientAssertion:     strings.TrimSpace(r.Form.Get("client_assertion")),
			ClientAssertionType: strings.TrimSpace(r.Form.Get("client_assertion_type")),
		}
	}

	// RFC 6749 §2.3: a client MUST NOT use more than one authentication method
	// per request. Detect Basic vs body before merging so a mixed request is
	// rejected instead of silently picking one.
	basicClientID, basicClientSecret, hasBasic, basicErr := parseBasicClientCredentials(r)
	if basicErr != nil {
		return ExchangeAuthorizationCodeRequest{}, basicErr
	}
	bodySecret := strings.TrimSpace(request.ClientSecret)
	if hasBasic && bodySecret != "" {
		return ExchangeAuthorizationCodeRequest{}, appauth.NewInvalidClientError("a client must not use more than one client authentication method")
	}
	if request.ClientAssertion != "" && (hasBasic || bodySecret != "") {
		return ExchangeAuthorizationCodeRequest{}, appauth.NewInvalidClientError("a client assertion cannot be combined with another client authentication method")
	}

	switch {
	case hasBasic:
		if bodyClientID := strings.TrimSpace(request.ClientID); bodyClientID != "" && bodyClientID != basicClientID {
			return ExchangeAuthorizationCodeRequest{}, appauth.NewInvalidClientError("client_id does not match the Authorization header")
		}
		request.ClientID = basicClientID
		request.ClientSecret = basicClientSecret
		request.ClientAuthMethod = "client_secret_basic"
	case bodySecret != "":
		request.ClientAuthMethod = "client_secret_post"
	case request.ClientAssertion != "":
		request.ClientAuthMethod = "private_key_jwt"
	}
	if request.isClientCredentialsGrant() {
		if err := request.Validate(); err != nil {
			return ExchangeAuthorizationCodeRequest{}, err
		}
	}

	return request, nil
}

// protocolClientAuthMethod derives the client authentication method from the
// credentials actually present, exactly like the token endpoint decoder. mTLS
// leaves the method empty; the registered method decides in the app layer.
func protocolClientAuthMethod(hasBasic bool, clientSecret, clientAssertion string) string {
	switch {
	case hasBasic:
		return "client_secret_basic"
	case strings.TrimSpace(clientSecret) != "":
		return "client_secret_post"
	case strings.TrimSpace(clientAssertion) != "":
		return "private_key_jwt"
	default:
		return ""
	}
}

func decodeTokenRevocationRequest(r *http.Request) appauth.RevokeTokenCommand {
	// RFC 7009 requires application/x-www-form-urlencoded,
	// but also accept JSON for convenience.
	if err := r.ParseForm(); err == nil {
		token := strings.TrimSpace(r.Form.Get("token"))
		tokenTypeHint := strings.TrimSpace(r.Form.Get("token_type_hint"))
		clientID := strings.TrimSpace(r.Form.Get("client_id"))
		clientSecret := strings.TrimSpace(r.Form.Get("client_secret"))
		clientAssertion := strings.TrimSpace(r.Form.Get("client_assertion"))
		clientAssertionType := strings.TrimSpace(r.Form.Get("client_assertion_type"))

		// HTTP Basic Auth takes precedence.
		hasBasic := false
		if basicClientID, basicClientSecret, ok := r.BasicAuth(); ok {
			hasBasic = true
			if clientID == "" {
				clientID = strings.TrimSpace(basicClientID)
			}
			if clientSecret == "" {
				clientSecret = strings.TrimSpace(basicClientSecret)
			}
		}

		return appauth.RevokeTokenCommand{
			Token:               token,
			TokenTypeHint:       tokenTypeHint,
			ClientID:            clientID,
			ClientSecret:        clientSecret,
			ClientAuthMethod:    protocolClientAuthMethod(hasBasic, clientSecret, clientAssertion),
			ClientAssertion:     clientAssertion,
			ClientAssertionType: clientAssertionType,
			PeerCertificates:    platformmtls.PeerCertificatesFromRequest(r),
			TokenEndpointURL:    platformconfig.AbsoluteRequestURL(r),
		}
	}

	// Fallback to JSON body.
	var req struct {
		Token         string `json:"token"`
		TokenTypeHint string `json:"token_type_hint"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
	}
	_ = route.DecodeJSON(r, &req)

	clientID := req.ClientID
	clientSecret := req.ClientSecret
	if basicClientID, basicClientSecret, ok := r.BasicAuth(); ok {
		if clientID == "" {
			clientID = strings.TrimSpace(basicClientID)
		}
		if clientSecret == "" {
			clientSecret = strings.TrimSpace(basicClientSecret)
		}
	}

	return appauth.RevokeTokenCommand{
		Token:         req.Token,
		TokenTypeHint: req.TokenTypeHint,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
	}
}

func decodeTokenIntrospectionRFCRequest(r *http.Request) appauth.IntrospectTokenRFCCommand {
	// RFC 7662 requires application/x-www-form-urlencoded.
	if err := r.ParseForm(); err == nil {
		token := strings.TrimSpace(r.Form.Get("token"))
		tokenTypeHint := strings.TrimSpace(r.Form.Get("token_type_hint"))
		clientID := strings.TrimSpace(r.Form.Get("client_id"))
		clientSecret := strings.TrimSpace(r.Form.Get("client_secret"))
		clientAssertion := strings.TrimSpace(r.Form.Get("client_assertion"))
		clientAssertionType := strings.TrimSpace(r.Form.Get("client_assertion_type"))

		hasBasic := false
		if basicClientID, basicClientSecret, ok := r.BasicAuth(); ok {
			hasBasic = true
			if clientID == "" {
				clientID = strings.TrimSpace(basicClientID)
			}
			if clientSecret == "" {
				clientSecret = strings.TrimSpace(basicClientSecret)
			}
		}

		return appauth.IntrospectTokenRFCCommand{
			Token:               token,
			TokenTypeHint:       tokenTypeHint,
			ClientID:            clientID,
			ClientSecret:        clientSecret,
			ClientAuthMethod:    protocolClientAuthMethod(hasBasic, clientSecret, clientAssertion),
			ClientAssertion:     clientAssertion,
			ClientAssertionType: clientAssertionType,
			PeerCertificates:    platformmtls.PeerCertificatesFromRequest(r),
			TokenEndpointURL:    platformconfig.AbsoluteRequestURL(r),
			IssuerURL:           platformconfig.IssuerURL(r),
		}
	}

	// Fallback to JSON body.
	var req struct {
		Token         string `json:"token"`
		TokenTypeHint string `json:"token_type_hint"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
	}
	_ = route.DecodeJSON(r, &req)

	clientID := req.ClientID
	clientSecret := req.ClientSecret
	if basicClientID, basicClientSecret, ok := r.BasicAuth(); ok {
		if clientID == "" {
			clientID = strings.TrimSpace(basicClientID)
		}
		if clientSecret == "" {
			clientSecret = strings.TrimSpace(basicClientSecret)
		}
	}

	return appauth.IntrospectTokenRFCCommand{
		Token:         req.Token,
		TokenTypeHint: req.TokenTypeHint,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		IssuerURL:     platformconfig.IssuerURL(r),
	}
}
