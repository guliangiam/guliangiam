package route

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OAuthErrorCode is a protocol error code. The values come from RFC 6749 §5.2
// (token endpoint) and RFC 6750 §3 (bearer token), plus the RFC 9449 DPoP codes
// listed in 错误码注册表 §2. The type is a string so a standard that this project
// adopts later can pass its own code without touching the writer.
type OAuthErrorCode string

const (
	OAuthErrInvalidRequest          OAuthErrorCode = "invalid_request"
	OAuthErrInvalidClient           OAuthErrorCode = "invalid_client"
	OAuthErrInvalidGrant            OAuthErrorCode = "invalid_grant"
	OAuthErrUnauthorizedClient      OAuthErrorCode = "unauthorized_client"
	OAuthErrUnsupportedGrantType    OAuthErrorCode = "unsupported_grant_type"
	OAuthErrUnsupportedResponseType OAuthErrorCode = "unsupported_response_type"
	OAuthErrInvalidScope            OAuthErrorCode = "invalid_scope"
	OAuthErrInvalidRedirectURI      OAuthErrorCode = "invalid_redirect_uri"
	OAuthErrInvalidToken            OAuthErrorCode = "invalid_token"
	OAuthErrInsufficientScope       OAuthErrorCode = "insufficient_scope"
	OAuthErrInvalidDPoPProof        OAuthErrorCode = "invalid_dpop_proof"
	OAuthErrUseDPoPNonce            OAuthErrorCode = "use_dpop_nonce"
	OAuthErrServerError             OAuthErrorCode = "server_error"
	OAuthErrTemporarilyUnavailable  OAuthErrorCode = "temporarily_unavailable"

	// The codes below have no application-namespace counterpart, so they reach
	// the writer either through apperrors.ProtocolCodeError or through one of
	// these constants. They are listed here so 错误码注册表 §2 has a single code table
	// instead of the writer accepting unknown strings by construction.

	// RFC 6749 §4.1.2.1 (authorization endpoint) / RFC 8628 §3.5 (device grant).
	OAuthErrAccessDenied OAuthErrorCode = "access_denied"
	// RFC 8628 §3.5.
	OAuthErrAuthorizationPending OAuthErrorCode = "authorization_pending"
	OAuthErrSlowDown             OAuthErrorCode = "slow_down"
	OAuthErrExpiredToken         OAuthErrorCode = "expired_token"
	// RFC 8693 §2.2.2 / §5 (token exchange target).
	OAuthErrInvalidTarget OAuthErrorCode = "invalid_target"
	// RFC 9101 §5 (JAR request object).
	OAuthErrInvalidRequestObject OAuthErrorCode = "invalid_request_object"
	// RFC 7591 §3.2.2 / RFC 7592 §3.1 (dynamic client registration).
	OAuthErrInvalidClientMetadata    OAuthErrorCode = "invalid_client_metadata"
	OAuthErrInvalidSoftwareStatement OAuthErrorCode = "invalid_software_statement"
)

// ChallengeSchemeBearer and ChallengeSchemeDPoP are the two WWW-Authenticate
// schemes this server issues (RFC 6750 §3, RFC 9449 §5). Naming them keeps the
// scheme out of the call sites, where a typo would produce a challenge the
// client silently ignores.
const (
	ChallengeSchemeBearer = "Bearer"
	ChallengeSchemeDPoP   = "DPoP"
	// ChallengeSchemeBasic is used for token-endpoint client authentication
	// failures (RFC 6749 §5.2): the server MUST answer 401 with a challenge
	// matching the scheme the client attempted.
	ChallengeSchemeBasic = "Basic"
)

// scimErrorSchema is the RFC 7644 §3.12 error message schema URN.
const scimErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"

// oauthCodeByAppCode maps the application error namespace (错误码注册表 §1) onto
// the protocol namespace (错误码注册表 §2). The two namespaces never mix: a protocol
// endpoint must answer with an OAuth error code even when the failure came from
// a service that raised an application error.
var oauthCodeByAppCode = map[apperrors.Code]OAuthErrorCode{
	apperrors.CodeInvalidArgument:   OAuthErrInvalidRequest,
	apperrors.CodeNotFound:          OAuthErrInvalidRequest,
	apperrors.CodeConflict:          OAuthErrInvalidRequest,
	apperrors.CodeAlreadyExists:     OAuthErrInvalidRequest,
	apperrors.CodeUnauthorized:      OAuthErrInvalidToken,
	apperrors.CodeForbidden:         OAuthErrInsufficientScope,
	apperrors.CodePasswordExpired:   OAuthErrInvalidGrant,
	apperrors.CodeMfaRequired:       OAuthErrInvalidGrant,
	apperrors.CodeAccountLocked:     OAuthErrInvalidGrant,
	apperrors.CodeInvitationUsed:    OAuthErrInvalidGrant,
	apperrors.CodeInvitationExpired: OAuthErrInvalidGrant,
	apperrors.CodeGone:              OAuthErrInvalidGrant,
	apperrors.CodeTooManyRequests:   OAuthErrTemporarilyUnavailable,
	apperrors.CodeUnavailable:       OAuthErrTemporarilyUnavailable,
	apperrors.CodeInternal:          OAuthErrServerError,
	apperrors.CodeUnimplemented:     OAuthErrServerError,
}

// OAuthErrorFrom maps err onto the RFC 6749 error triple. The HTTP status is
// preserved from the application error so a 404 stays a 404; only the body
// shape and the code belong to the protocol namespace.
//
// An application-layer error that implements apperrors.ProtocolCodeError wins:
// some RFC codes (unsupported_response_type, invalid_target) have no application
// counterpart in 错误码注册表 §1, so the service layer names them directly.
func OAuthErrorFrom(err error) (statusOut int, code OAuthErrorCode, message string) {
	status := StatusCodeFromError(err)

	var protocolErr apperrors.ProtocolCodeError
	if errors.As(err, &protocolErr) {
		return status, OAuthErrorCode(protocolErr.ProtocolCode()), sanitizeProtocolDescription(protocolErr.Error())
	}

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return status, OAuthErrServerError, genericProtocolErrorMessage(status)
	}

	code, ok := oauthCodeByAppCode[appErr.Code]
	if !ok {
		code = OAuthErrInvalidRequest
	}
	if status >= http.StatusInternalServerError {
		// Never echo internal messages (错误码注册表 §1: 禁止回显堆栈、SQL 或内部路径).
		return status, code, genericProtocolErrorMessage(status)
	}
	return status, code, sanitizeProtocolDescription(appErr.Message)
}

func genericProtocolErrorMessage(status int) string {
	if status == http.StatusTooManyRequests {
		return "too many requests"
	}
	return "the authorization server encountered an unexpected condition"
}

// SafeProtocolDescription returns err's message when it is safe to show on the
// protocol face (a 4xx application error) and a generic sentence otherwise. A
// resource endpoint whose code is fixed by RFC 6750 §3.1 (UserInfo always
// answers `invalid_token`, never `insufficient_scope`) still needs this: the
// code is the caller's to fix, the description is not.
func SafeProtocolDescription(err error) string {
	if err == nil {
		return ""
	}
	if status := StatusCodeFromError(err); status >= http.StatusInternalServerError {
		return genericProtocolErrorMessage(status)
	}
	return sanitizeProtocolDescription(err.Error())
}

// singleLine keeps a value on one line. A CR/LF in a header value is rejected by
// the server, and in a body it lets an internal message smuggle a second line
// past a single-line parser.
func singleLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexAny(value, "\r\n"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	return value
}

// sanitizeProtocolDescription keeps error_description to a single line so the
// body stays machine readable and cannot smuggle multi-line internals.
func sanitizeProtocolDescription(message string) string {
	if message = singleLine(message); message == "" {
		return "invalid request"
	}
	return message
}

// ChallengeParam is one auth-param of a WWW-Authenticate challenge.
type ChallengeParam struct {
	Name  string
	Value string
}

// WriteAuthChallenge writes a WWW-Authenticate challenge (RFC 9110 §11.6.1):
// `scheme 1*SP #auth-param`. Empty values are omitted, so a caller never emits
// `error_description=""` (which reads as a present-but-empty explanation), and
// every value is escaped as a quoted-string (RFC 9110 §5.6.4) — one unescaped
// quote truncates the header at the first proxy and silently changes the
// challenge the client sees.
func WriteAuthChallenge(w http.ResponseWriter, scheme string, params ...ChallengeParam) {
	value := scheme
	for _, param := range params {
		rendered := singleLine(param.Value)
		if rendered == "" {
			continue
		}
		// The scheme is separated from the first auth-param by a single space;
		// the params themselves are a comma-separated list.
		separator := ", "
		if value == scheme {
			separator = " "
		}
		value += separator + param.Name + `="` + escapeAuthParam(rendered) + `"`
	}
	w.Header().Set("WWW-Authenticate", value)
}

func escapeAuthParam(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

// challengeParamError 是 RFC 6750 §3 challenge 的 error 参数名。
const challengeParamError = "error"

// WriteBearerChallenge writes the minimal RFC 6750 §3 challenge a 401 from a
// resource endpoint must carry: a code and no explanation. It has to run
// **before** the error body is written, because a header set after WriteHeader
// is silently dropped — which is how this challenge was lost on
// `/oauth2/userinfo`.
func WriteBearerChallenge(w http.ResponseWriter, code OAuthErrorCode) {
	WriteAuthChallenge(w, ChallengeSchemeBearer, ChallengeParam{Name: challengeParamError, Value: string(code)})
}

// WriteTokenChallenge writes the full challenge a 401 from a token or resource
// endpoint carries, including the optional `error_description` and `scope`
// auth-params (RFC 6750 §3).
func WriteTokenChallenge(w http.ResponseWriter, scheme string, code OAuthErrorCode, description, scope string) {
	WriteAuthChallenge(w, scheme,
		ChallengeParam{Name: challengeParamError, Value: string(code)},
		ChallengeParam{Name: "error_description", Value: description},
		ChallengeParam{Name: "scope", Value: scope},
	)
}

// WriteProtocolError serializes err as a protocol error. A 401 additionally
// carries the WWW-Authenticate challenge RFC 6750 §3 requires, which is how a
// resource server tells the client which scheme to retry with; the description
// travels with it so a client can log the cause without parsing the body.
func WriteProtocolError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, description := OAuthErrorFrom(err)
	if status == http.StatusUnauthorized {
		if code == OAuthErrInvalidClient {
			// RFC 6749 §5.2: client authentication failures at the token
			// endpoint answer with the challenge scheme the client attempted.
			// Basic has no error/error_description auth-params (RFC 7617), so
			// only realm is emitted; the body still carries the OAuth error.
			WriteAuthChallenge(w, ChallengeSchemeBasic, ChallengeParam{Name: "realm", Value: "guliangiam"})
		} else {
			WriteTokenChallenge(w, ChallengeSchemeBearer, code, description, "")
		}
	}
	if status == http.StatusTooManyRequests {
		// 错误码注册表 §1: too_many_requests must carry Retry-After.
		if w.Header().Get("Retry-After") == "" {
			w.Header().Set("Retry-After", "60")
		}
	}
	WriteOAuthError(w, status, code, description)
}

// WriteResourceError answers a protected-resource request with the RFC 6750 §3
// challenge and body. Unlike WriteProtocolError the code and status are the
// caller's, because a resource server collapses every failure onto the RFC 6750
// codes (`invalid_token` / `insufficient_scope`) no matter what the service
// raised; the description is sanitized so an internal message never reaches the
// client. The challenge is written before the body — a header set after
// WriteHeader is silently dropped.
func WriteResourceError(w http.ResponseWriter, err error, code OAuthErrorCode, status int) {
	description := SafeProtocolDescription(err)
	WriteTokenChallenge(w, ChallengeSchemeBearer, code, description, "")
	WriteOAuthError(w, status, code, description)
}

// WriteOAuthError writes an RFC 6749 §5.2 (or RFC 6750 §3) error body:
// {"error": "...", "error_description": "..."}. This writer is the only one the
// protocol face may use (手册 4.1.3 落地清单第 3 项).
func WriteOAuthError(w http.ResponseWriter, status int, code OAuthErrorCode, description string) {
	body := map[string]string{"error": string(code)}
	if description != "" {
		body["error_description"] = description
	}
	WriteJSON(w, status, body)
}

// WriteSCIMError writes an RFC 7644 §3.12 error body:
// {"schemas": [...], "detail": "...", "status": "400"}. SCIM uses its own
// envelope and its own media type, so the SCIM endpoints cannot share the OAuth
// writer either.
// scimTypeForStatus 把 HTTP 状态映射到 RFC 7644 §3.12 的 scimType。
// 未在规范中定义的组合（如 404/403）保持省略，避免客户端误判。
func scimTypeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalidValue"
	case http.StatusConflict:
		return "uniqueness"
	case http.StatusNotImplemented:
		return "notImplemented"
	default:
		return ""
	}
}

func WriteSCIMError(w http.ResponseWriter, status int, detail string) {
	WriteSCIMErrorWithType(w, status, detail, "")
}

// WriteSCIMErrorWithType 允许调用方显式指定 scimType（如 count 超限的
// tooMany）；留空时回落到 scimTypeForStatus 的状态码映射。
func WriteSCIMErrorWithType(w http.ResponseWriter, status int, detail, scimType string) {
	if detail == "" {
		detail = http.StatusText(status)
	}
	// RFC 7644 §3.1 requires the SCIM media type, so this writer cannot go
	// through WriteJSON (which forces application/json).
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	payload := map[string]any{
		"schemas": []string{scimErrorSchema},
		"detail":  detail,
		"status":  strconv.Itoa(status),
	}
	// RFC 7644 §3.12 的 scimType 用于客户端区分失败类别（DEF-30）。
	if scimType == "" {
		scimType = scimTypeForStatus(status)
	}
	if scimType != "" {
		payload["scimType"] = scimType
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("http route: scim error encode error", "error", err)
	}
}
