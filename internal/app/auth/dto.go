package auth

import "time"

type SessionAccount struct {
	// ID is the stable user primary key. Clients must use it for ownership/
	// requester comparisons; Name is the login identifier (username) and may
	// change or collide with other identifiers.
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	IsAdmin     bool   `json:"isAdmin"`
}

type SessionOrganization struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName,omitempty"`
	LogoUrl      string   `json:"logoUrl,omitempty"`
	FaviconUrl   string   `json:"faviconUrl,omitempty"`
	NavItems     []string `json:"navItems"`
	UserNavItems []string `json:"userNavItems"`
	// NavGroupOrder 是 navPreferences 中可选的菜单分组顺序（组 key 数组）；
	// 数组顺序即侧边栏分组顺序，未列出的分组按前端静态顺序排后。
	NavGroupOrder []string `json:"navGroupOrder,omitempty"`
}

type AccountContext struct {
	Account      SessionAccount      `json:"account"`
	Organization SessionOrganization `json:"organization"`
	AccessToken  string              `json:"accessToken,omitempty"`
	// PasswordPolicy 是组织策略叠加默认值后的生效快照，供前端展示与后端
	// 校验共用同一门槛，避免前端再硬编码长度。
	PasswordPolicy PasswordPolicyConfig `json:"passwordPolicy"`
	// SessionPolicy 是组织空闲超时快照，前端会话提示与后端过期判定同源。
	SessionPolicy SessionPolicyConfig `json:"sessionPolicy"`
	// NeedUpdatePassword 来自 user_credentials.need_update_password，登录与
	// 账户上下文共用同一来源；当前仅用于前端提示，不做后端硬拦截。
	NeedUpdatePassword bool `json:"needUpdatePassword"`
}

type LoginMfaMethod struct {
	ID          string `json:"id"`
	MethodType  string `json:"methodType"`
	DisplayName string `json:"displayName,omitempty"`
	Target      string `json:"target,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

type LoginResponse struct {
	Status              string                `json:"status"`
	Account             *SessionAccount       `json:"account,omitempty"`
	Organization        *SessionOrganization  `json:"organization,omitempty"`
	AccessToken         string                `json:"accessToken,omitempty"`
	PasswordPolicy      *PasswordPolicyConfig `json:"passwordPolicy,omitempty"`
	SessionPolicy       *SessionPolicyConfig  `json:"sessionPolicy,omitempty"`
	NeedUpdatePassword  bool                  `json:"needUpdatePassword"`
	DeviceInfo          map[string]any        `json:"deviceInfo,omitempty"`
	NextMfa             string                `json:"nextMfa,omitempty"`
	ChallengeID         string                `json:"challengeId,omitempty"`
	ChallengeState      string                `json:"challengeState,omitempty"`
	ChallengeExpiresAt  *time.Time            `json:"challengeExpiresAt,omitempty"`
	RequiredMfa         []string              `json:"requiredMfa,omitempty"`
	AvailableMfaMethods []LoginMfaMethod      `json:"availableMfaMethods,omitempty"`
	// HandleLoggedIn response fields
	Code         string `json:"code,omitempty"`
	RedirectTo   string `json:"redirectTo,omitempty"`
	SAMLResponse string `json:"samlResponse,omitempty"`
	RelayState   string `json:"relayState,omitempty"`
	CASTicket    string `json:"casTicket,omitempty"`
	IdToken      string `json:"idToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	// Password expiration notice
	PasswordExpired   bool   `json:"passwordExpired,omitempty"`
	PasswordExpiresAt string `json:"passwordExpiresAt,omitempty"`
	// Remember-me (F3): the plaintext is delivered once and rotated on every use.
	RememberMeToken     string     `json:"rememberMeToken,omitempty"`
	RememberMeExpiresAt *time.Time `json:"rememberMeExpiresAt,omitempty"`
}

type LoginChallengeResponse struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	MethodType string     `json:"methodType,omitempty"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}

type SessionIdentity struct {
	// ID is the internal session row's primary key — opaque value the client
	// sends back in the `__Host-session` cookie and the CAS ticket exchange
	// handler uses to resolve the live session.
	ID string
	// UserID is the application user's UUID — used by ScopeAuthorizer to check
	// platform_admins membership. Phase C-Step 1 added.
	UserID           string
	OrganizationID   string
	OrganizationSlug string
	Username         string
	AccessToken      string
	// Sid is the session's OIDC identifier. The authorize endpoint stamps it onto
	// the authorization code so the token endpoint can put it in the ID token
	// (设计报告 14.12.3).
	Sid string
	// AuthTime is when the session was established, i.e. when the end user
	// actually authenticated. It is the value OIDC's auth_time carries; the code
	// issue time is a different moment (设计报告 14.8).
	AuthTime     time.Time
	IsPrivileged bool
	LifecycleID  string
	TicketID     string
}

type ConsentDetail struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	Username         string     `json:"username"`
	OrganizationID   string     `json:"organizationId"`
	OrganizationSlug string     `json:"organizationSlug"`
	ApplicationID    string     `json:"applicationId"`
	ApplicationSlug  string     `json:"applicationSlug"`
	GrantedScopes    []string   `json:"grantedScopes"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	Revision         int64      `json:"revision"`
}

type ConsentResponse struct {
	Status  string        `json:"status"`
	Consent ConsentDetail `json:"consent"`
}

type ConsentEvaluationDetail struct {
	ApplicationID     string   `json:"applicationId"`
	ApplicationSlug   string   `json:"applicationSlug"`
	RequestedScopes   []string `json:"requestedScopes"`
	ConsentableScopes []string `json:"consentableScopes"`
	GrantedScopes     []string `json:"grantedScopes"`
	MissingScopes     []string `json:"missingScopes"`
	Required          bool     `json:"required"`
}

type ConsentEvaluationResponse struct {
	Status     string                  `json:"status"`
	Evaluation ConsentEvaluationDetail `json:"evaluation"`
}

type AuthorizationCodeDetail struct {
	Code        string     `json:"code,omitempty"`
	RedirectURI string     `json:"redirectUri,omitempty"`
	RedirectTo  string     `json:"redirectTo,omitempty"`
	State       string     `json:"state,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type AuthorizeResponse struct {
	Status     string                   `json:"status"`
	Code       *AuthorizationCodeDetail `json:"code,omitempty"`
	Evaluation *ConsentEvaluationDetail `json:"evaluation,omitempty"`
	// Implicit flow response fields (response_type=token or id_token)
	AccessToken string `json:"accessToken,omitempty"`
	IdToken     string `json:"idToken,omitempty"`
	TokenType   string `json:"tokenType,omitempty"`
	ExpiresIn   int64  `json:"expiresIn,omitempty"`
	State       string `json:"state,omitempty"`
	RedirectTo  string `json:"redirectTo,omitempty"`
}

type ExchangeAuthorizationCodeResponse struct {
	Status       string `json:"status"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	// IdToken is present only when the granted scopes contain `openid`, as OIDC
	// Core §3.1.3.3 requires for the authorization-code flow.
	IdToken   string `json:"idToken,omitempty"`
	TokenType string `json:"tokenType"`
	ExpiresIn int64  `json:"expiresIn"`
	Scope     string `json:"scope,omitempty"`
	// DeviceSecret is returned only when the granted scope contains device_sso
	// (Native SSO, RFC 8693 §4.1). It is a bearer credential for exchanging the
	// access token at the device secret endpoint, so it is emitted once here and
	// never included in token introspection or token list responses.
	DeviceSecret string `json:"deviceSecret,omitempty"`
}
