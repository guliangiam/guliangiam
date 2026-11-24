package agents

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
)

// agentEncryptedMapper 负责 token 列的 AES-256-GCM 加解密（DB-058 机密列）。
var agentEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	agentTable       = "guliangiam.agents"
	agentColumnToken = "token"
)

func agentAAD(agentID string) crypto.AADContext {
	return crypto.AADContext{Table: agentTable, Column: agentColumnToken, PKValue: agentID}
}

// toAgent 把 MySQL 表扫描结果转成 domain.Agent。Token 若加密则先解密。
func toAgent(row agentRow) (*domain.Agent, error) {
	agent := &domain.Agent{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Type:           row.Type,
		Host:           row.Host,
		Port:           row.Port,
		Url:            row.Url,
		Token:          row.Token,
		ApplicationID:  row.ApplicationID,
		IsEnabled:      row.IsEnabled,
		LastPingAt:     row.LastPingAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if !agentEncryptedMapper.IsEnabled() {
		return agent, nil
	}
	token, err := agentEncryptedMapper.DecryptColumn(agentAAD(agent.ID), agent.Token)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt agent token")
	}
	agent.Token = token
	return agent, nil
}

// agentTokenForWrite 返回持久化机密列所需的值，不改 domain 对象。
func agentTokenForWrite(agent *domain.Agent) (string, error) {
	if !agentEncryptedMapper.IsEnabled() {
		return agent.Token, nil
	}
	token, err := agentEncryptedMapper.EncryptColumn(agentAAD(agent.ID), agent.Token)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt agent token")
	}
	return token, nil
}
