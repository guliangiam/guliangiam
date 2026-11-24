package tokens

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestToken_Validate_TableDriven(t *testing.T) {
	now := time.Now().Add(time.Hour)
	tests := []struct {
		name    string
		token   Token
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid token",
			token: Token{
				UserID: "user-1", TokenHash: "hash123",
				ApplicationID: "app-1", ExpiresAt: now,
				Scopes: []string{"read", "write"},
			},
			wantErr: false,
		},
		{
			name: "missing userID",
			token: Token{
				TokenHash: "h", ApplicationID: "a", ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing tokenHash",
			token: Token{
				UserID: "u", ApplicationID: "a", ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing applicationID",
			token: Token{
				UserID: "u", TokenHash: "h", ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing expiresAt",
			token: Token{
				UserID: "u", TokenHash: "h", ApplicationID: "a",
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "empty scope string",
			token: Token{
				UserID: "u", TokenHash: "h", ApplicationID: "a",
				ExpiresAt: now, Scopes: []string{""},
			},
			wantErr: true,
			errCode: apperrors.CodeNotFound,
		},
		{
			name: "valid token with no scopes",
			token: Token{
				UserID: "u", TokenHash: "h", ApplicationID: "a",
				ExpiresAt: now, Scopes: []string{},
			},
			wantErr: false,
		},
		{
			name: "valid token with nil scopes",
			token: Token{
				UserID: "u", TokenHash: "h", ApplicationID: "a",
				ExpiresAt: now, Scopes: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				appErr, ok := err.(*apperrors.Error)
				if !ok {
					t.Fatalf("error type = %T, want *apperrors.Error", err)
				}
				if appErr.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", appErr.Code, tt.errCode)
				}
			}
		})
	}
}

func TestToken_IsExpired_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		token       Token
		now         time.Time
		wantExpired bool
	}{
		{
			name:        "expired - past",
			token:       Token{ExpiresAt: now.Add(-time.Hour)},
			now:         now,
			wantExpired: true,
		},
		{
			name:        "not expired - future",
			token:       Token{ExpiresAt: now.Add(time.Hour)},
			now:         now,
			wantExpired: false,
		},
		{
			name:        "not expired - exact time",
			token:       Token{ExpiresAt: now},
			now:         now,
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsExpired(tt.now)
			if got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestToken_IsRevoked_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		token       Token
		wantRevoked bool
	}{
		{
			name:        "not revoked - nil RevokedAt",
			token:       Token{RevokedAt: nil},
			wantRevoked: false,
		},
		{
			name:        "revoked - RevokedAt set",
			token:       Token{RevokedAt: &now},
			wantRevoked: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsRevoked()
			if got != tt.wantRevoked {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.wantRevoked)
			}
		})
	}
}

func TestToken_Fields(t *testing.T) {
	now := time.Now()
	token := Token{
		ID:               "token-1",
		UserID:           "user-1",
		Username:         "alice",
		OrganizationID:   "org-1",
		OrganizationSlug: "acme",
		ApplicationID:    "app-1",
		ApplicationSlug:  "myapp",
		TokenPrefix:      "glt_",
		TokenHash:        "hash123",
		Scopes:           []string{"openid", "profile"},
		Audience:         []string{"app-1"},
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}
	if token.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", token.UserID, "user-1")
	}
	if len(token.Scopes) != 2 {
		t.Errorf("len(Scopes) = %d, want 2", len(token.Scopes))
	}
	if len(token.Audience) != 1 {
		t.Errorf("len(Audience) = %d, want 1", len(token.Audience))
	}
}

func TestToken_ValidGrantTypes(t *testing.T) {
	validTypes := []string{
		"authorization_code",
		"client_credentials",
		"refresh_token",
		"password",
		"urn:ietf:params:oauth:grant-type:device_code",
		"urn:ietf:params:oauth:grant-type:jwt-bearer",
	}
	for _, gt := range validTypes {
		t.Run(gt, func(t *testing.T) {
			if !validGrantTypes[gt] {
				t.Errorf("grant type %q should be valid", gt)
			}
		})
	}
}

func TestToken_InvalidGrantTypes(t *testing.T) {
	invalidTypes := []string{"implicit", "custom", ""}
	for _, gt := range invalidTypes {
		t.Run(gt, func(t *testing.T) {
			if validGrantTypes[gt] {
				t.Errorf("grant type %q should be invalid", gt)
			}
		})
	}
}
