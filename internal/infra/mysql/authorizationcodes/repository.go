package authorizationcodes

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// codePrefixLength 是明文 code 保留的前 N 位，仅用于排障，永远不参与匹配。
const codePrefixLength = 8

// Repository 是 MySQL 8.0+ 实现的 authorization_codes 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// authorizationCodeRow 是 SELECT 的扫描目标。
// Scopes 在 MySQL 里是 TEXT（JSON 数组字符串），读回时需反序列化。
type authorizationCodeRow struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	RedirectURI      string
	Scopes           string // JSON 数组字符串 → []string
	State            string
	Nonce            string
	CodeChallenge    string
	ChallengeMethod  string
	Resource         string
	Sid              string
	AuthTime         time.Time
	ExpiresAt        time.Time
	UsedAt           *time.Time
	CreatedAt        time.Time
}

// GetByCode 通过授权码明文的 SHA-256 hash 查找。明文永远不进 WHERE 子句，
// 所以数据库 dump 和慢查询日志都不会泄露可用的授权码（手册 9.5）。
func (r Repository) GetByCode(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	var row authorizationCodeRow
	err := r.db.QueryRowContext(ctx, queryGetAuthorizationCodeByCode,
		platformcrypto.HashTokenValue(code),
	).Scan(
		&row.ID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.RedirectURI,
		&row.Scopes,
		&row.State,
		&row.Nonce,
		&row.CodeChallenge,
		&row.ChallengeMethod,
		&row.Resource,
		&row.Sid,
		&row.AuthTime,
		&row.ExpiresAt,
		&row.UsedAt,
		&row.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("authorization code not found")
		}
		return nil, err
	}
	item, err := toAuthorizationCode(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode authorization code")
	}
	return item, nil
}

// Create 插入新授权码。Scopes 存 JSON 数组字符串。
func (r Repository) Create(ctx context.Context, authorizationCode *domain.AuthorizationCode) error {
	scopesJSON, err := json.Marshal(authorizationCode.Scopes)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertAuthorizationCode,
		authorizationCode.ID,
		platformcrypto.HashTokenValue(authorizationCode.Code),
		codePrefix(authorizationCode.Code),
		authorizationCode.UserID,
		authorizationCode.ApplicationID,
		authorizationCode.RedirectURI,
		string(scopesJSON),
		authorizationCode.State,
		authorizationCode.Sid,
		authorizationCode.Nonce,
		authorizationCode.CodeChallenge,
		authorizationCode.ChallengeMethod,
		authorizationCode.Resource,
		authorizationCode.AuthTime,
		authorizationCode.ExpiresAt,
		authorizationCode.CreatedAt,
	)
	return err
}

// MarkUsed 一次性消费授权码。零 affected rows 表示已被并发消费或行已消失，
// 一律报告冲突，让调用方不会把重放误认为成功。
func (r Repository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, queryMarkAuthorizationCodeUsed, usedAt, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("authorization code already used")
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────

func codePrefix(code string) string {
	if len(code) <= codePrefixLength {
		return code
	}
	return code[:codePrefixLength]
}

// toAuthorizationCode 把 DB 扫描行（Scopes 是 JSON 字符串）转成 domain 对象。
// Code 字段故意留空：明文只存在于创建时跳转响应中，永远不入库。
func toAuthorizationCode(row authorizationCodeRow) (*domain.AuthorizationCode, error) {
	var scopes []string
	if row.Scopes != "" {
		if err := json.Unmarshal([]byte(row.Scopes), &scopes); err != nil {
			return nil, err
		}
	} else {
		scopes = []string{}
	}
	return &domain.AuthorizationCode{
		ID:               row.ID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		RedirectURI:      row.RedirectURI,
		Scopes:           scopes,
		State:            row.State,
		Nonce:            row.Nonce,
		CodeChallenge:    row.CodeChallenge,
		ChallengeMethod:  row.ChallengeMethod,
		Resource:         row.Resource,
		Sid:              row.Sid,
		AuthTime:         row.AuthTime,
		ExpiresAt:        row.ExpiresAt,
		UsedAt:           row.UsedAt,
		CreatedAt:        row.CreatedAt,
	}, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
