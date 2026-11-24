package mcpservers

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// mcpServerEncryptedMapper seals token (A-6.2). The column keeps its
// historical VARCHAR(255) definition and now stores a v2 envelope bound to
// (table, column, row id) instead of the plaintext bearer token.
var mcpServerEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	mcpServerTable          = "guliangiam.mcp_servers"
	mcpServerColumnToken    = "token"
	mcpServerTokenMaxLength = 255
)

func mcpServerAAD(serverID string) crypto.AADContext {
	return crypto.AADContext{Table: mcpServerTable, Column: mcpServerColumnToken, PKValue: serverID}
}

// scanMcpServer scans a full mcp_server row into a domain McpServer and opens
// the stored token.
func scanMcpServer(sc interface{ Scan(...interface{}) error }) (*domain.McpServer, error) {
	var s domain.McpServer
	var tools []byte
	var storedToken string
	err := sc.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.DisplayName, &s.URL, &storedToken, &s.Application, &tools, &s.IsEnabled, &s.CreatedAt, &s.UpdatedAt, &s.Revision)
	if err != nil {
		return nil, err
	}
	token, err := openMcpServerToken(s.ID, s.OrganizationID, storedToken)
	if err != nil {
		return nil, err
	}
	s.Token = token
	if len(tools) > 0 && string(tools) != "null" {
		if err := json.Unmarshal(tools, &s.Tools); err != nil {
			return nil, apperrors.WrapInternalf(err, "unmarshal tools")
		}
	}
	if s.Tools == nil {
		s.Tools = []domain.Tool{}
	}
	return &s, nil
}

// openMcpServerToken decrypts a sealed token. A stored value that is not an
// envelope is a pre-A-6.2 plaintext row: it is returned for compatibility but
// an operator WARN tells them to run the backfill. The WARN carries only row
// identifiers, never the token or the ciphertext.
func openMcpServerToken(serverID, organizationID, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}

	plaintext, err := mcpServerEncryptedMapper.DecryptEnvelopeColumn(mcpServerAAD(serverID), stored)
	if err == nil {
		return plaintext, nil
	}
	if !errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.WrapInternalf(err, "decrypt mcp server token")
	}

	// A v0 direct-KEK token is ciphertext, not plaintext. Open it so the row
	// keeps working, but still ask the operator to re-seal it.
	if legacy, legacyErr := mcpServerEncryptedMapper.DecryptColumn(mcpServerAAD(serverID), stored); legacyErr == nil {
		slog.Warn("mcp server token is a legacy v0 envelope; run `migrate encrypt-credential-secrets`",
			"server_id", serverID, "organization_id", organizationID)
		return legacy, nil
	}

	slog.Warn("mcp server token is stored in plaintext; run `migrate encrypt-credential-secrets`",
		"server_id", serverID, "organization_id", organizationID)
	return stored, nil
}

// sealMcpServerToken returns the value to persist without mutating the domain
// object. An empty token stays empty ("not configured").
func sealMcpServerToken(server *domain.McpServer) (string, error) {
	if server.Token == "" {
		return "", nil
	}
	sealed, err := mcpServerEncryptedMapper.EncryptColumn(mcpServerAAD(server.ID), server.Token)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt mcp server token")
	}
	// The column is still VARCHAR(255). Refuse an oversized envelope instead of
	// letting the database silently fail the write; no value is logged.
	if len(sealed) > mcpServerTokenMaxLength {
		return "", apperrors.NewInternalf(
			"encrypted mcp server token is %d characters, exceeding the %d-character column",
			len(sealed), mcpServerTokenMaxLength)
	}
	return sealed, nil
}
