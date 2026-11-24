package auth

import (
	"context"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These tests pin 设计报告 14.24 (RFC 8693) as implemented for. The
// headline defect was that any authenticated client could present another
// client's subject token and receive a token for that user — a horizontal
// privilege escalation that required nothing but knowing one token value.

const (
	exchangeCallerClientID = "caller-client"
	exchangeCallerSecret   = "caller-secret"
	exchangeOtherClientID  = "other-client"
	exchangeSubjectToken   = "gl_pat_subject-token-value"
)

func exchangeTestService(t *testing.T) (Service, *testTokenRepository) {
	t.Helper()

	secretHash, err := platformcrypto.HashClientSecret(exchangeCallerSecret)
	if err != nil {
		t.Fatalf("hash client secret: %v", err)
	}

	caller := &applicationdomain.Application{
		ID:               "app-caller",
		OrganizationID:   "org-1",
		Slug:             "caller",
		ClientID:         exchangeCallerClientID,
		ClientSecretHash: secretHash,
		GrantTypes:       []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
	}
	other := &applicationdomain.Application{
		ID:             "app-other",
		OrganizationID: "org-1",
		Slug:           "other",
		ClientID:       exchangeOtherClientID,
	}

	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	tokenRepo := &testTokenRepository{
		tokensByHash: map[string]*tokendomain.Token{
			platformcrypto.HashTokenValue(exchangeSubjectToken): {
				ID:              "token-subject",
				UserID:          "user-1",
				Username:        "jane",
				OrganizationID:  "org-1",
				ApplicationID:   "app-other",
				ApplicationSlug: "other",
				Scopes:          []string{"openid", "profile"},
				Audience:        []string{"https://api.example.com"},
				ExpiresAt:       now.Add(time.Hour),
			},
		},
	}

	uow := serviceTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			appRepo: &loginTestAppRepository{
				apps:       map[string]*applicationdomain.Application{caller.ID: caller, other.ID: other},
				byClientID: map[string]*applicationdomain.Application{caller.ClientID: caller, other.ClientID: other},
			},
		},
		tokenRepo: tokenRepo,
	}
	return Service{Clock: fixedAuthClock{}, UOW: uow, UUIDGenerator: fakeUUIDGenerator{fixedID: "generated"}}, tokenRepo
}

func exchangeCommand(overrides func(*GetTokenExchangeCommand)) GetTokenExchangeCommand {
	command := GetTokenExchangeCommand{
		GrantType:        tokenExchangeGrantType,
		SubjectToken:     exchangeSubjectToken,
		SubjectTokenType: SubjectTokenTypeAccessToken,
		ClientID:         exchangeCallerClientID,
		ClientSecret:     exchangeCallerSecret,
	}
	if overrides != nil {
		overrides(&command)
	}
	return command
}

// TestTokenExchange_RejectsForeignSubjectToken is the core of: the
// caller may only exchange a subject token issued to itself.
func TestTokenExchange_RejectsForeignSubjectToken(t *testing.T) {
	service, _ := exchangeTestService(t)

	_, err := service.GetTokenExchangeToken(context.Background(), exchangeCommand(nil))
	if err == nil {
		t.Fatal("expected a foreign subject token to be refused")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("error = %v, want forbidden", err)
	}
}

func TestTokenExchange_RequiresClientAuthentication(t *testing.T) {
	service, _ := exchangeTestService(t)

	cases := map[string]func(*GetTokenExchangeCommand){
		"missing client_id":       func(c *GetTokenExchangeCommand) { c.ClientID = "" },
		"missing secret":          func(c *GetTokenExchangeCommand) { c.ClientSecret = "" },
		"wrong secret":            func(c *GetTokenExchangeCommand) { c.ClientSecret = "nope" },
		"unknown client_id":       func(c *GetTokenExchangeCommand) { c.ClientID = "no-such-client" },
		"secret on public client": func(c *GetTokenExchangeCommand) { c.ClientSecret = "extra" },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			command := exchangeCommand(mutate)
			if name == "secret on public client" {
				// Point at the confidential caller so "public" does not apply.
				command.ClientID = exchangeCallerClientID
			}
			_, err := service.GetTokenExchangeToken(context.Background(), command)
			if err == nil {
				t.Fatal("expected the exchange to be refused")
			}
			appErr, ok := err.(*apperrors.Error)
			if !ok || appErr.Code != apperrors.CodeUnauthorized {
				t.Fatalf("error = %v, want unauthorized", err)
			}
		})
	}
}

func TestTokenExchange_RejectsUnvalidatedActorToken(t *testing.T) {
	service, _ := exchangeTestService(t)

	// The caller owns the subject token, so the failure must come from the actor.
	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.ActorToken = "gl_pat_not-a-real-token"
		c.ActorTokenType = SubjectTokenTypeAccessToken
	})

	_, err := service.GetTokenExchangeToken(context.Background(), command)
	if err == nil {
		t.Fatal("expected an invalid actor token to be refused")
	}
}

func TestTokenExchange_RejectsActorTokenWithoutType(t *testing.T) {
	service, _ := exchangeTestService(t)

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.ActorToken = exchangeSubjectToken
		c.ActorTokenType = ""
	})

	if _, err := service.GetTokenExchangeToken(context.Background(), command); err == nil {
		t.Fatal("expected a missing actor_token_type to be refused")
	}
}

// TestTokenExchange_IssuedTokenTypeIsTheActualType pins RFC 8693 §2.2.1: the
// response reports what was issued, not what was asked for. Echoing the request
// told a client asking for a JWT that it had received one.
func TestTokenExchange_IssuedTokenTypeIsTheActualType(t *testing.T) {
	service, _ := exchangeTestService(t)

	// Ask for a JWT while pointing the subject token at the caller so the
	// ownership check passes.
	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.RequestedTokenType = SubjectTokenTypeJWT
	})
	command.SubjectToken = "gl_pat_own-token"

	// Give the caller its own subject token.
	service.UOW.(serviceTestUnitOfWork).tokenRepo.tokensByHash[platformcrypto.HashTokenValue("gl_pat_own-token")] = &tokendomain.Token{
		ID:              "token-own",
		UserID:          "user-1",
		Username:        "jane",
		OrganizationID:  "org-1",
		ApplicationID:   "app-caller",
		ApplicationSlug: "caller",
		Scopes:          []string{"openid"},
		ExpiresAt:       time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC),
	}

	response, err := service.GetTokenExchangeToken(context.Background(), command)
	if err != nil {
		t.Fatalf("GetTokenExchangeToken() error = %v", err)
	}

	if response.IssuedTokenType != SubjectTokenTypeAccessToken {
		t.Fatalf("issued_token_type = %q, want the actual type %q",
			response.IssuedTokenType, SubjectTokenTypeAccessToken)
	}
}

func TestTokenExchange_RejectsUnsupportedRequestedTokenType(t *testing.T) {
	service, _ := exchangeTestService(t)

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.RequestedTokenType = "urn:ietf:params:oauth:token-type:refresh_token"
	})

	if _, err := service.GetTokenExchangeToken(context.Background(), command); err == nil {
		t.Fatal("expected an unsupported requested_token_type to be refused")
	}
}

func TestTokenExchange_RejectsResourceAndAudienceTogether(t *testing.T) {
	service, _ := exchangeTestService(t)

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.Resource = "https://api.example.com"
		c.Audience = "https://other.example.com"
	})

	if _, err := service.GetTokenExchangeToken(context.Background(), command); err == nil {
		t.Fatal("expected resource+audience to be refused as ambiguous")
	}
}

// TestTokenExchange_InvalidTargetCarriesProtocolCode covers RFC 8707 §2: an
// out-of-scope resource is `invalid_target`, a protocol code with no application
// counterpart.
func TestTokenExchange_InvalidTargetCarriesProtocolCode(t *testing.T) {
	service, _ := exchangeTestService(t)

	cases := map[string]string{
		"resource not permitted": "https://someone-elses-api.example.com",
	}

	for name, resource := range cases {
		t.Run(name, func(t *testing.T) {
			command := exchangeCommand(func(c *GetTokenExchangeCommand) {
				c.Resource = resource
				c.SubjectToken = "gl_pat_own-token"
			})
			service.UOW.(serviceTestUnitOfWork).tokenRepo.tokensByHash[platformcrypto.HashTokenValue("gl_pat_own-token")] = &tokendomain.Token{
				ID:              "token-own",
				UserID:          "user-1",
				Username:        "jane",
				OrganizationID:  "org-1",
				ApplicationID:   "app-caller",
				ApplicationSlug: "caller",
				Scopes:          []string{"openid"},
				ExpiresAt:       time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC),
			}

			_, err := service.GetTokenExchangeToken(context.Background(), command)
			if err == nil {
				t.Fatal("expected the resource to be refused")
			}
			protocolErr, ok := err.(apperrors.ProtocolCodeError)
			if !ok {
				t.Fatalf("error %T does not carry a protocol code: %v", err, err)
			}
			if protocolErr.ProtocolCode() != "invalid_target" {
				t.Fatalf("protocol code = %q, want invalid_target", protocolErr.ProtocolCode())
			}
		})
	}
}

func TestTokenExchange_RejectsWrongGrantType(t *testing.T) {
	service, _ := exchangeTestService(t)

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.GrantType = "authorization_code"
	})

	if _, err := service.GetTokenExchangeToken(context.Background(), command); err == nil {
		t.Fatal("expected a wrong grant_type to be refused")
	}
}

func TestActDepth_CountsNesting(t *testing.T) {
	flat := map[string]any{"client_id": "a"}
	nested := map[string]any{"client_id": "a", "act": map[string]any{"client_id": "b"}}
	deep := map[string]any{"client_id": "a", "act": map[string]any{
		"client_id": "b",
		"act":       map[string]any{"client_id": "c"},
	}}

	if got := actDepth(flat); got != 1 {
		t.Errorf("flat depth = %d, want 1", got)
	}
	if got := actDepth(nested); got != 2 {
		t.Errorf("nested depth = %d, want 2", got)
	}
	if got := actDepth(deep); got != 3 {
		t.Errorf("deep depth = %d, want 3", got)
	}
	if got := actDepth(nil); got != 0 {
		t.Errorf("nil depth = %d, want 0", got)
	}
}

// TestActChainMetadata_RespectsMaxDepth is the cap that keeps a delegation chain
// from growing without bound.
func TestActChainMetadata_RespectsMaxDepth(t *testing.T) {
	parent := map[string]any{"client_id": "a", "act": map[string]any{
		"client_id": "b",
		"act":       map[string]any{"client_id": "c"},
	}}
	actor := &tokenExchangeActor{Subject: "user-1", ClientID: "d", Parent: parent}

	act := actor.actChainMetadata()
	if act["client_id"] != "d" {
		t.Fatalf("client_id = %v, want d", act["client_id"])
	}
	// parent depth is already 3, so it must not be attached (would make depth 4).
	if _, present := act["act"]; present {
		t.Fatalf("expected the parent chain to be dropped at the depth cap: %v", act)
	}
}

func TestActChainMetadata_AttachesParentBelowCap(t *testing.T) {
	parent := map[string]any{"client_id": "a"}
	actor := &tokenExchangeActor{Subject: "user-1", ClientID: "b", Parent: parent}

	act := actor.actChainMetadata()
	nested, ok := act["act"].(map[string]any)
	if !ok {
		t.Fatalf("expected the parent chain to be attached: %v", act)
	}
	if nested["client_id"] != "a" {
		t.Fatalf("nested client_id = %v, want a", nested["client_id"])
	}
}

func TestActor_ActChainMetadataNilReceiver(t *testing.T) {
	var actor *tokenExchangeActor
	if got := actor.actChainMetadata(); got != nil {
		t.Fatalf("nil actor produced %v, want nil", got)
	}
}
