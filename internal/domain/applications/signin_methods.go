package applications

import "strings"

// Canonical signin method names stored in app_signin_config.signin_methods.
// They are the single source of truth for which authentication channels an
// application offers. The former enable_* boolean columns are derived views and
// are no longer persisted.
const (
	SigninMethodPassword         = "Password"
	SigninMethodVerificationCode = "Verification code"
	SigninMethodWebAuthn         = "WebAuthn"
	SigninMethodLDAP             = "LDAP"
	SigninMethodFaceID           = "Face ID"
	SigninMethodDeviceLogin      = "Device login"
	SigninMethodWeChat           = "WeChat"
	SigninMethodDingTalk         = "DingTalk"
	SigninMethodLark             = "Lark"
	SigninMethodCAS              = "CAS"
)

// HasSigninMethod reports whether the application enables name. Matching is
// case-insensitive on trimmed values so console/API casing cannot drift.
func (a Application) HasSigninMethod(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, method := range a.SigninMethods {
		if strings.ToLower(strings.TrimSpace(method)) == target {
			return true
		}
	}
	return false
}

func (a Application) PasswordEnabled() bool      { return a.HasSigninMethod(SigninMethodPassword) }
func (a Application) PasswordLoginEnabled() bool { return a.HasSigninMethod(SigninMethodPassword) }
func (a Application) CodeSigninEnabled() bool    { return a.HasSigninMethod(SigninMethodVerificationCode) }
func (a Application) WebAuthnEnabled() bool      { return a.HasSigninMethod(SigninMethodWebAuthn) }
func (a Application) WeChatEnabled() bool        { return a.HasSigninMethod(SigninMethodWeChat) }
func (a Application) DingTalkEnabled() bool      { return a.HasSigninMethod(SigninMethodDingTalk) }
func (a Application) LarkEnabled() bool          { return a.HasSigninMethod(SigninMethodLark) }
func (a Application) CASEnabled() bool           { return a.HasSigninMethod(SigninMethodCAS) }
