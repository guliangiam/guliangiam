package auth

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidSubjectTokenType(t *testing.T) {
	assert.True(t, isValidSubjectTokenType(SubjectTokenTypeAccessToken))
	assert.True(t, isValidSubjectTokenType(SubjectTokenTypeJWT))
	assert.True(t, isValidSubjectTokenType(SubjectTokenTypeIDToken))
	assert.False(t, isValidSubjectTokenType("invalid"))
	assert.False(t, isValidSubjectTokenType(""))
}

func TestIsScopeSubset(t *testing.T) {
	assert.True(t, isScopeSubset([]string{"openid"}, []string{"openid", "profile"}))
	assert.True(t, isScopeSubset([]string{"openid", "profile"}, []string{"openid", "profile"}))
	assert.True(t, isScopeSubset(nil, []string{"openid"}))
	assert.True(t, isScopeSubset([]string{}, []string{"openid"}))
	assert.False(t, isScopeSubset([]string{"openid", "admin"}, []string{"openid", "profile"}))
	assert.False(t, isScopeSubset([]string{"admin"}, nil))
}

func TestGenerateDeviceCode(t *testing.T) {
	code, err := generateDeviceCode()
	require.NoError(t, err)
	assert.NotEmpty(t, code)

	// Should be unique
	code2, err := generateDeviceCode()
	require.NoError(t, err)
	assert.NotEqual(t, code, code2)
}

func TestGenerateUserCode(t *testing.T) {
	code, err := generateUserCode()
	require.NoError(t, err)
	assert.NotEmpty(t, code)
	// Should contain a dash separator
	assert.Contains(t, code, "-")
	// Format: XXXX-XXXX
	assert.Equal(t, 9, len(code)) // 4 chars + dash + 4 chars
}

func TestMatchRedirectURI_EmptyRegisteredSetAdmitsNothing(t *testing.T) {
	// This used to be the opposite: the login-then-issue path had its own
	// allowlist helper that returned true for an empty registered set, which is
	// an open redirect (设计报告 14.6.4). The single matcher now lives in
	// domain/applications and is covered exhaustively by
	// redirect_uri_conformance_test.go; this case pins the regression.
	assert.False(t, applicationdomain.MatchRedirectURI("https://any.com/callback", nil))
	assert.False(t, applicationdomain.MatchRedirectURI("https://any.com/callback", []string{}))
	assert.False(t, applicationdomain.MatchRedirectURI("https://any.com/callback", []string{"", "  "}))
}

func TestToLoginChallengeResponse_Nil(t *testing.T) {
	result := toLoginChallengeResponse(nil)
	assert.Nil(t, result)
}

func TestDefaultVerifyLiveness(t *testing.T) {
	ok, confidence, algo, err := defaultVerifyLiveness(context.Background(), "", "", "", "", "", "")
	assert.Error(t, err) // Returns Unimplemented error
	assert.False(t, ok)
	assert.Equal(t, 0.0, confidence)
	assert.Equal(t, "", algo)
}

func TestDefaultCompareFace(t *testing.T) {
	ok, similarity, err := defaultCompareFace(context.Background(), "", "", "", "", "", "")
	assert.Error(t, err) // Returns Unimplemented error
	assert.False(t, ok)
	assert.Equal(t, 0.0, similarity)
}

func TestIsWorkloadNotFound_RecognizesPgxErrNoRows(t *testing.T) {
	// Regression: workload repositories use pgx (not database/sql),
	// so QueryRow.Scan returns pgx.ErrNoRows instead of sql.ErrNoRows.
	// The helper must recognize both; otherwise a "service account not found"
	// during password login gets misclassified as an internal 500.
	t.Run("pgx ErrNoRows is not found", func(t *testing.T) {
		if !isWorkloadNotFound(pgx.ErrNoRows) {
			t.Error("expected pgx.ErrNoRows to be recognized as 'not found'")
		}
	})
	t.Run("sql ErrNoRows is not found", func(t *testing.T) {
		if !isWorkloadNotFound(sql.ErrNoRows) {
			t.Error("expected sql.ErrNoRows to be recognized as 'not found'")
		}
	})
	t.Run("nil error is not found", func(t *testing.T) {
		if isWorkloadNotFound(nil) {
			t.Error("nil error should not be 'not found'")
		}
	})
}
