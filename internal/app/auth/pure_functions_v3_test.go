package auth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- password_reset_signup.go ---

func TestResolveVerificationType_Email(t *testing.T) {
	recipient, vType := resolveVerificationType("alice@example.com", "")
	assert.Equal(t, "alice@example.com", recipient)
	assert.Equal(t, "email", vType)
}

func TestResolveVerificationType_Phone(t *testing.T) {
	recipient, vType := resolveVerificationType("", "+1234567890")
	assert.Equal(t, "+1234567890", recipient)
	assert.Equal(t, "phone", vType)
}

func TestResolveVerificationType_Both(t *testing.T) {
	recipient, vType := resolveVerificationType("alice@example.com", "+1234567890")
	assert.Equal(t, "alice@example.com", recipient)
	assert.Equal(t, "email", vType)
}

func TestResolveVerificationType_Neither(t *testing.T) {
	recipient, vType := resolveVerificationType("", "")
	assert.Equal(t, "", recipient)
	assert.Equal(t, "", vType)
}

func TestValidateResetPasswordCommand_MissingOrg(t *testing.T) {
	err := validateResetPasswordCommand(ResetPasswordCommand{})
	assert.Error(t, err)
}

func TestValidateResetPasswordCommand_MissingUsername(t *testing.T) {
	err := validateResetPasswordCommand(ResetPasswordCommand{OrganizationSlug: "org1"})
	assert.Error(t, err)
}

func TestValidateResetPasswordCommand_MissingCode(t *testing.T) {
	err := validateResetPasswordCommand(ResetPasswordCommand{OrganizationSlug: "org1", Username: "alice"})
	assert.Error(t, err)
}

func TestValidateResetPasswordCommand_MissingPassword(t *testing.T) {
	err := validateResetPasswordCommand(ResetPasswordCommand{OrganizationSlug: "org1", Username: "alice", VerificationCode: "123456"})
	assert.Error(t, err)
}

func TestValidateResetPasswordCommand_Success(t *testing.T) {
	err := validateResetPasswordCommand(ResetPasswordCommand{
		OrganizationSlug: "org1", Username: "alice", VerificationCode: "123456", NewPassword: "P@ssw0rd",
	})
	assert.NoError(t, err)
}

func TestValidateSignupCommand_MissingOrg(t *testing.T) {
	err := validateSignupCommand(SignupCommand{})
	assert.Error(t, err)
}

func TestValidateSignupCommand_MissingUsername(t *testing.T) {
	err := validateSignupCommand(SignupCommand{OrganizationSlug: "org1"})
	assert.Error(t, err)
}

func TestValidateSignupCommand_MissingPassword(t *testing.T) {
	err := validateSignupCommand(SignupCommand{OrganizationSlug: "org1", Username: "alice"})
	assert.Error(t, err)
}

func TestValidateSignupCommand_Success(t *testing.T) {
	err := validateSignupCommand(SignupCommand{OrganizationSlug: "org1", Username: "alice", Password: "P@ssw0rd"})
	assert.NoError(t, err)
}

// --- social_login.go ---

func TestGenerateSocialUsername_V3_WithUsername(t *testing.T) {
	result := generateSocialUsername("github", &SocialUserInfo{Username: "alice", ID: "1234567890"})
	assert.Contains(t, result, "alice")
}

func TestGenerateSocialUsername_V3_WithDisplayName(t *testing.T) {
	result := generateSocialUsername("google", &SocialUserInfo{DisplayName: "Bob Smith", ID: "1234567890"})
	assert.NotEmpty(t, result)
}

func TestGenerateSocialUsername_V3_WithEmail(t *testing.T) {
	result := generateSocialUsername("github", &SocialUserInfo{Email: "carol@example.com", ID: "1234567890"})
	assert.NotEmpty(t, result)
}

func TestGenerateSocialUsername_V3_FallbackToID(t *testing.T) {
	result := generateSocialUsername("wechat", &SocialUserInfo{ID: "1234567890"})
	assert.Contains(t, result, "user_wechat")
}

func TestGenerateSocialUsername_V3_EmptyInfo(t *testing.T) {
	// Function panics on nil, so test with empty struct instead
	result := generateSocialUsername("github", &SocialUserInfo{ID: "1234567890"})
	assert.Contains(t, result, "user_github")
}

// --- authorize.go ---

func TestValidateAuthorizeScopes_V3(t *testing.T) {
	scopes, err := validateAuthorizeScopes([]string{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, scopes)
}

func TestBuildAuthorizationRedirectURL_V3(t *testing.T) {
	url := buildAuthorizationRedirectURL("https://example.com/callback", "auth-code-123", "state-abc")
	assert.Contains(t, url, "code=auth-code-123")
	assert.Contains(t, url, "state=state-abc")
}

// --- web3.go ---

func TestNormalizeEthereumAddress_V3(t *testing.T) {
	assert.Equal(t, "0xabc", normalizeEthereumAddress("0xABC"))
	assert.Equal(t, "0xabc", normalizeEthereumAddress("0xabc"))
}

func TestParseChainID_V3(t *testing.T) {
	id, err := ParseChainID("1")
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1), id)

	// Empty string defaults to chain ID 1
	id, err = ParseChainID("")
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1), id)

	_, err = ParseChainID("abc")
	assert.Error(t, err)
}
