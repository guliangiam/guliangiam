package agents

import (
	"context"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type agentRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Type           string
	Host           string
	Port           int
	Url            string
	Token          string
	ApplicationID  string
	IsEnabled      bool
	LastPingAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

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

// agentEncryptedMapper guards the DB-058 confidential column: token is stored
// as AES-256-GCM ciphertext.
var agentEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	agentTable       = "guliangiam.agents"
	agentColumnToken = "token"
)

func agentAAD(agentID string) crypto.AADContext {
	return crypto.AADContext{Table: agentTable, Column: agentColumnToken, PKValue: agentID}
}

// agentTokenForWrite returns the value to persist for the confidential column
// without mutating the domain object.
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

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Agent, error) {
	var row agentRow
	var applicationID *string
	err := r.db.QueryRow(ctx, queryGetAgentByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
		&row.Host, &row.Port, &row.Url, &row.Token, &applicationID,
		&row.IsEnabled, &row.LastPingAt,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	row.ApplicationID = stringOrEmpty(applicationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("agent not found")
		}
		return nil, err
	}

	return toAgent(row)
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Agent, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListAgents, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Agent, 0)
	for rows.Next() {
		var row agentRow
		var applicationID *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Type,
			&row.Host, &row.Port, &row.Url, &row.Token, &applicationID,
			&row.IsEnabled, &row.LastPingAt,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		row.ApplicationID = stringOrEmpty(applicationID)
		agent, err := toAgent(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, agent)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountAgents, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, agent *domain.Agent) error {
	token, err := agentTokenForWrite(agent)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertAgent,
		agent.ID, agent.OrganizationID, agent.Name, agent.DisplayName, agent.Type,
		agent.Host, agent.Port, agent.Url, token, nullableUUID(agent.ApplicationID),
		agent.IsEnabled, agent.LastPingAt,
		agent.CreatedAt, agent.UpdatedAt, agent.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, agent *domain.Agent) error {
	token, err := agentTokenForWrite(agent)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateAgent,
		agent.DisplayName, agent.Host, agent.Port,
		agent.Url, token, nullableUUID(agent.ApplicationID),
		agent.IsEnabled, agent.LastPingAt,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		agent.UpdatedAt, agent.Revision-1, agent.ID, agent.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		// 实体已在 app 层按租户读取：ErrNoRows 只代表 revision 冲突。
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("agent revision conflict")
		}
		return err
	}
	agent.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteAgent, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("agent not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：application_id 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
