package tokens

import (
	"testing"
	"time"
)

func TestToken_Validate(t *testing.T) {
	now := time.Now().Add(time.Hour)
	validToken := Token{
		UserID:        "user-1",
		TokenHash:     "hash123",
		ApplicationID: "app-1",
		ExpiresAt:     now,
		Scopes:        []string{"read", "write"},
	}

	if err := validToken.Validate(); err != nil {
		t.Errorf("valid token should pass, got error: %v", err)
	}
}

func TestToken_Validate_MissingFields(t *testing.T) {
	now := time.Now().Add(time.Hour)
	tests := []struct {
		name    string
		token   Token
		wantErr bool
	}{
		{
			"missing userID",
			Token{TokenHash: "h", ApplicationID: "a", ExpiresAt: now},
			true,
		},
		{
			"missing tokenHash",
			Token{UserID: "u", ApplicationID: "a", ExpiresAt: now},
			true,
		},
		{
			"missing applicationID",
			Token{UserID: "u", TokenHash: "h", ExpiresAt: now},
			true,
		},
		{
			"missing expiresAt",
			Token{UserID: "u", TokenHash: "h", ApplicationID: "a"},
			true,
		},
		{
			"empty scope",
			Token{UserID: "u", TokenHash: "h", ApplicationID: "a", ExpiresAt: now, Scopes: []string{""}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToken_IsExpired(t *testing.T) {
	now := time.Now()
	token := Token{ExpiresAt: now.Add(-time.Hour)}
	if !token.IsExpired(now) {
		t.Error("token should be expired")
	}

	token.ExpiresAt = now.Add(time.Hour)
	if token.IsExpired(now) {
		t.Error("token should not be expired")
	}
}

func TestToken_IsRevoked(t *testing.T) {
	token := Token{}
	if token.IsRevoked() {
		t.Error("token should not be revoked initially")
	}

	now := time.Now()
	token.RevokedAt = &now
	if !token.IsRevoked() {
		t.Error("token should be revoked after setting RevokedAt")
	}
}
