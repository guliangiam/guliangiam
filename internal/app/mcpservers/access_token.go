package mcpservers

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type AccessTokenResult struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	TokenType   string `json:"tokenType"`
}

func (s Service) GetAccessToken(ctx context.Context, organizationID, serverID, userID string) (*AccessTokenResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(serverID) == "" || strings.TrimSpace(userID) == "" {
		return nil, apperrors.NewInvalidArgument("serverId and userId are required")
	}

	// 先带可信 org 取 server：跨租户即 404。后续裁决只以 server 的
	// OrganizationID 为锚点，绝不把 query 里的 organizationId 当依据。
	srv, err := s.UOW.McpServers().GetByIDForOrganization(ctx, serverID, organizationID)
	if err != nil {
		return nil, s.auditAccessTokenDecision(ctx, organizationID, serverID, userID, "denied", err)
	}

	// userId 是调用方输入：目标用户必须属于同一可信租户。跨租户与不存在
	// 表现一致（404），避免用该接口枚举其他租户的用户。
	user, err := s.UOW.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, s.auditAccessTokenDecision(ctx, organizationID, serverID, userID, "denied", err)
	}
	if user == nil || user.OrganizationID != srv.OrganizationID {
		return nil, s.auditAccessTokenDecision(ctx, organizationID, serverID, userID, "denied",
			apperrors.NewNotFound("user not found"))
	}

	expiresIn := 300
	now := s.Clock.Now()
	exp := now.Add(time.Duration(expiresIn) * time.Second)

	jtiBytes := make([]byte, 16)
	_ = base64.RawURLEncoding.EncodeToString(jtiBytes)

	claims := map[string]any{
		"iss":       "guliangiam",
		"sub":       userID,
		"aud":       fmt.Sprintf("mcp-server:%s", serverID),
		"exp":       exp.Unix(),
		"iat":       now.Unix(),
		"scope":     "mcp:tools",
		"server_id": serverID,
	}

	token, err := signMCPJWT(claims, s.JWKS)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "failed to sign access token")
	}

	if err := s.auditAccessTokenDecision(ctx, srv.OrganizationID, serverID, userID, "issued", nil); err != nil {
		return nil, err
	}

	return &AccessTokenResult{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		TokenType:   "Bearer",
	}, nil
}

// auditAccessTokenDecision records the outcome (issued/denied) with the server
// id, the target user id and the result. Denials are audited in the caller's
// tenant only: a cross-tenant attempt must not write into the server's tenant.
// A failed denial audit is logged rather than promoted, so the 404 stands.
func (s Service) auditAccessTokenDecision(ctx context.Context, organizationID, serverID, userID, result string, cause error) error {
	auditErr := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			organizationID, "",
			"mcpservers.access_token", "mcpservers", serverID,
			"mcpservers", "mcpservers.access_token."+result,
			map[string]any{"serverId": serverID, "userId": userID, "result": result},
		)
	})
	if auditErr != nil {
		slog.WarnContext(ctx, "failed to audit MCP access token decision",
			"server_id", serverID, "result", result, "error", auditErr)
		if cause == nil {
			return apperrors.WrapInternal(auditErr, "failed to audit access token issuance")
		}
	}
	return cause
}

func signMCPJWT(claims map[string]any, jwksProvider domainauth.JWKSProvider) (string, error) {
	if jwksProvider == nil {
		return "", apperrors.NewInvalidArgument("JWKS provider not configured")
	}
	return jwksProvider.SignToken(claims, "JWT")
}
