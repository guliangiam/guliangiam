package mcpservers

import (
	"encoding/json"
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// mcpServerEncryptedMapper 负责 token 列的信封加解密（A-6.2）。列定义保持
// VARCHAR(255)，库内改为绑定 (table, column, 行 id) 的 v2 信封。
var mcpServerEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	mcpServerTable          = "guliangiam.mcp_servers"
	mcpServerColumnToken    = "token"
	mcpServerTokenMaxLength = 255
)

func mcpServerAAD(serverID string) crypto.AADContext {
	return crypto.AADContext{Table: mcpServerTable, Column: mcpServerColumnToken, PKValue: serverID}
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanMcpServerRow 扫描一行 McpServer，处理 tools JSON 反序列化与 token 解密。
func scanMcpServerRow(sc scannable) (*domain.McpServer, error) {
	var s domain.McpServer
	var toolsStr string
	var storedToken string
	err := sc.Scan(
		&s.ID, &s.OrganizationID, &s.Name, &s.DisplayName, &s.URL, &storedToken, &s.Application,
		&toolsStr, &s.IsEnabled, &s.CreatedAt, &s.UpdatedAt, &s.Revision,
	)
	if err != nil {
		return nil, err
	}
	token, err := openMcpServerToken(s.ID, s.OrganizationID, storedToken)
	if err != nil {
		return nil, err
	}
	s.Token = token
	if toolsStr != "" && toolsStr != "null" {
		if err := json.Unmarshal([]byte(toolsStr), &s.Tools); err != nil {
			return nil, apperrors.NewInternal("unmarshal tools")
		}
	}
	if s.Tools == nil {
		s.Tools = []domain.Tool{}
	}
	return &s, nil
}

// openMcpServerToken 解密信封。非信封值按 A-6.2 之前的存量明文兼容读取，
// 但必须写结构化 WARN；WARN 只包含行标识，不含明文或密文。
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

	// v0 是密文而非明文：先尝试 legacy 解密，成功后仍提示回填。
	if legacy, legacyErr := mcpServerEncryptedMapper.DecryptColumn(mcpServerAAD(serverID), stored); legacyErr == nil {
		slog.Warn("mcp server token is a legacy v0 envelope; run `migrate encrypt-credential-secrets`",
			"server_id", serverID, "organization_id", organizationID)
		return legacy, nil
	}

	slog.Warn("mcp server token is stored in plaintext; run `migrate encrypt-credential-secrets`",
		"server_id", serverID, "organization_id", organizationID)
	return stored, nil
}

// sealMcpServerToken 返回持久化值，不改 domain 对象；空值保持空值（未配置）。
func sealMcpServerToken(server *domain.McpServer) (string, error) {
	if server.Token == "" {
		return "", nil
	}
	sealed, err := mcpServerEncryptedMapper.EncryptColumn(mcpServerAAD(server.ID), server.Token)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt mcp server token")
	}
	if len(sealed) > mcpServerTokenMaxLength {
		return "", apperrors.NewInternalf(
			"encrypted mcp server token is %d characters, exceeding the %d-character column",
			len(sealed), mcpServerTokenMaxLength)
	}
	return sealed, nil
}
