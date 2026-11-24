package tokens

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 access_tokens + refresh_tokens 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// tokenRow 是 access_tokens 表扫描目标（19 列）。
// Scopes/Audience PG text[] → MySQL TEXT (JSON 数组字符串)
// Metadata PG jsonb → MySQL TEXT (JSON 字符串)
type tokenRow struct {
	ID                    string
	UserID                string
	Username              string
	OrganizationID        string
	OrganizationSlug      string
	ApplicationID         string
	ApplicationSlug       string
	TokenPrefix           string
	TokenHash             string
	DeviceSecretHash      string
	DeviceSecretExpiresIn int
	Scopes                string // JSON 数组 → []string
	Audience              string // JSON 数组 → []string
	Metadata              string // JSON → map[string]any
	ExpiresAt             time.Time
	RevokedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}

// refreshTokenRow 是 refresh_tokens 表扫描目标（17 列）。
type refreshTokenRow struct {
	ID               string
	AccessTokenID    string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	RefreshTokenHash string
	FamilyID         string
	Scopes           string // JSON 数组 → []string
	Audience         string // JSON 数组 → []string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}

// ── Helpers ──────────────────────────────────────────────────────

func marshalJSONObject(value map[string]any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

func marshalStringSlice(value []string) ([]byte, error) {
	if value == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(value)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// activeOnlyArgs 根据 *bool 生成对应的 3 个参数值。
// PG 版：($4 IS NULL OR ($4 = true AND ...) OR ($4 = false AND ...))
// MySQL 版同样结构需要传 3 次 activeOnly 值（nil / true / false）。
func activeOnlyArgs(activeOnly *bool) []any {
	v := any(nil)
	if activeOnly != nil {
		v = *activeOnly
	}
	return []any{v, v, v}
}

// listKeywordArgs 返回 keyword × 6（1 CASE WHEN + 5 LIKE CONCAT 各占 ?）。
func listKeywordArgs(keyword string) []any {
	return []any{keyword, keyword, keyword, keyword, keyword, keyword}
}

// ── SELECT ──────────────────────────────────────────────────────

// GetByID 按 access_token id 查。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Token, error) {
	return r.queryToken(ctx, queryGetTokenByID, id)
}

// GetByIDForOrganization 只在 token 所属用户属于 organizationID 时返回；
// 跨租户与不存在同为 not found。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Token, error) {
	return r.queryToken(ctx, queryGetTokenByIDForOrganization, id, organizationID)
}

// GetByTokenHash 按 token_hash 查 access token。
func (r Repository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Token, error) {
	return r.queryToken(ctx, queryGetTokenByHash, tokenHash)
}

// GetByDeviceSecretHash 按 device_secret_hash 查 access token。
func (r Repository) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*domain.Token, error) {
	return r.queryToken(ctx, queryGetTokenByDeviceSecretHash, deviceSecretHash)
}

// queryToken 通用 access token 查询执行器。
func (r Repository) queryToken(ctx context.Context, query string, args ...any) (*domain.Token, error) {
	var row tokenRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
		&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
		&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
		&row.Scopes, &row.Audience, &row.Metadata,
		&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("token not found")
		}
		return nil, err
	}
	return toToken(row)
}

// GetRefreshByTokenHash 按 refresh token_hash 查。FOR UPDATE 加锁。
func (r Repository) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var row refreshTokenRow
	err := r.db.QueryRowContext(ctx, queryGetRefreshTokenByHash, tokenHash).Scan(
		&row.ID,
		&row.AccessTokenID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.RefreshTokenHash,
		&row.FamilyID,
		&row.Scopes,
		&row.Audience,
		&row.ExpiresAt,
		&row.RevokedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("refresh token not found")
		}
		return nil, err
	}
	return toRefreshToken(row)
}

// ── List ────────────────────────────────────────────────────────

// List 参数占位符：
//
//	org_id×2(2), user_id×2(2), app_id×2(2), active_only×3(3), keyword×6(6), limit(1), offset(1)
//	总计 17；Count 去掉 limit/offset = 15
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Token, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	listArgs := make([]any, 0, 17)
	listArgs = append(listArgs,
		q.OrganizationID, q.OrganizationID,
		q.UserID, q.UserID,
		q.ApplicationID, q.ApplicationID,
	)
	listArgs = append(listArgs, activeOnlyArgs(q.ActiveOnly)...) // +3
	listArgs = append(listArgs, listKeywordArgs(q.Keyword)...)   // +6
	listArgs = append(listArgs, limit, offset)                   // +2 → 17

	rows, err := r.db.QueryContext(ctx, queryListTokens, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Token, 0)
	for rows.Next() {
		var row tokenRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
			&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
			&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
			&row.Scopes, &row.Audience, &row.Metadata,
			&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		token, err := toToken(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode token")
		}
		items = append(items, token)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	countArgs := make([]any, 0, 15)
	countArgs = append(countArgs,
		q.OrganizationID, q.OrganizationID,
		q.UserID, q.UserID,
		q.ApplicationID, q.ApplicationID,
	)
	countArgs = append(countArgs, activeOnlyArgs(q.ActiveOnly)...) // +3
	countArgs = append(countArgs, listKeywordArgs(q.Keyword)...)   // +6 → 15

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountTokens, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ── Create ──────────────────────────────────────────────────────

func (r Repository) Create(ctx context.Context, token *domain.Token) error {
	metadataJSON, err := marshalJSONObject(token.Metadata)
	if err != nil {
		return apperrors.NewInternal("encode token metadata")
	}
	scopesJSON, err := marshalStringSlice(token.Scopes)
	if err != nil {
		return err
	}
	audienceJSON, err := marshalStringSlice(token.Audience)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertToken,
		token.ID, token.UserID, token.ApplicationID, token.TokenHash, token.TokenPrefix,
		nullableString(token.DeviceSecretHash), token.DeviceSecretExpiresIn,
		string(scopesJSON), string(audienceJSON), string(metadataJSON),
		token.ExpiresAt, token.RevokedAt, token.CreatedAt, token.UpdatedAt, token.Revision,
	)
	return err
}

// UpdateDeviceSecret 把 Native SSO 设备密钥哈希写入已存在的 access token 行。
func (r Repository) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	if tokenID == "" || deviceSecretHash == "" {
		return apperrors.NewInternal("update device secret: token id and hash are required")
	}
	result, err := r.db.ExecContext(ctx, queryUpdateTokenDeviceSecret, deviceSecretHash, expiresIn, tokenID)
	if err != nil {
		return apperrors.WrapInternal(err, "update device secret")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.WrapInternal(err, "update device secret rows affected")
	}
	if affected == 0 {
		return apperrors.NewNotFound("token not found")
	}
	return nil
}

func (r Repository) CreateRefresh(ctx context.Context, token *domain.RefreshToken) error {
	scopesJSON, err := marshalStringSlice(token.Scopes)
	if err != nil {
		return err
	}
	audienceJSON, err := marshalStringSlice(token.Audience)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertRefreshToken,
		token.ID, nullableString(token.AccessTokenID), token.UserID, token.ApplicationID,
		token.RefreshTokenHash, "", nullableString(token.FamilyID),
		string(scopesJSON), string(audienceJSON),
		token.ExpiresAt, token.RevokedAt, token.CreatedAt, token.UpdatedAt, token.Revision,
	)
	return err
}

// ── Revoke ──────────────────────────────────────────────────────

// Revoke 撤销 access token。零 affected rows 有两种情况：
//   - token 不存在：先 GetByID → not found
//   - token 已被撤销：检查 RevokedAt 不为 nil → 直接返回（幂等）
func (r Repository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, queryRevokeToken, revokedAt, revokedAt, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		token, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
		if token.RevokedAt != nil {
			return nil // 已撤销 → 幂等成功
		}
	}

	return nil
}

// RevokeRefresh 撤销 refresh token。零 affected rows 表示已撤销 → 报告 ErrRefreshAlreadyRevoked。
func (r Repository) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, queryRevokeRefreshToken, revokedAt, revokedAt, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrRefreshAlreadyRevoked
	}
	return nil
}

// RevokeByQuery 按过滤条件批量撤销 access token。
// 占位符：revoked_at×2(2), org_id×2(2), user_id×2(2), app_id×2(2), token_prefix×2(2), expires_at(1) = 11
func (r Repository) RevokeByQuery(ctx context.Context, q domain.RevokeQuery, revokedAt time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryRevokeTokensByQuery,
		revokedAt, revokedAt, // SET 的 COALESCE 和 updated_at
		q.OrganizationID, q.OrganizationID,
		q.UserID, q.UserID,
		q.ApplicationID, q.ApplicationID,
		q.TokenPrefix, q.TokenPrefix,
		revokedAt, // WHERE expires_at > ?
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// RevokeByFamilyID 撤销同一 family 的 refresh tokens + 关联 access tokens。
// MySQL 改写为两步 UPDATE。返回 refresh token 撤销数。
func (r Repository) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	// 1. revoke refresh tokens → affected = refresh count
	result, err := r.db.ExecContext(ctx, queryRevokeRefreshByFamily,
		revokedAt, revokedAt, familyID,
	)
	if err != nil {
		return 0, err
	}
	refreshCount, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	// 2. revoke 关联 access tokens（UPDATE JOIN）
	_, err = r.db.ExecContext(ctx, queryRevokeAccessByFamilyJoin,
		familyID, revokedAt, revokedAt,
	)
	if err != nil {
		return refreshCount, err // refresh 已经撤销，access 失败也要报告
	}

	return refreshCount, nil
}

// RevokeRefreshByUser 撤销用户所有 refresh tokens + 关联 access tokens。
// 返回 refresh token 撤销数。
func (r Repository) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, queryRevokeRefreshByUser,
		revokedAt, revokedAt, userID,
	)
	if err != nil {
		return 0, err
	}
	refreshCount, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	// revoke 关联 access tokens
	_, err = r.db.ExecContext(ctx, queryRevokeAccessByUserJoin,
		userID, revokedAt, revokedAt,
	)
	if err != nil {
		return refreshCount, err
	}

	return refreshCount, nil
}

// RevokeByAuthorizationCodeID 撤销被泄露 authorization code 产生的 tokens。
// MySQL 改写：先 revoke access tokens（metadata 匹配），再 revoke 关联 refresh tokens。
// 返回 access + refresh 撤销总数。
func (r Repository) RevokeByAuthorizationCodeID(ctx context.Context, codeID string, revokedAt time.Time) (int64, error) {
	// 1. revoke access tokens where metadata.authorizationCodeId = ?
	accessResult, err := r.db.ExecContext(ctx, queryRevokeAccessByCodeID,
		revokedAt, revokedAt, codeID,
	)
	if err != nil {
		return 0, err
	}
	accessCount, err := accessResult.RowsAffected()
	if err != nil {
		return 0, err
	}

	// 2. revoke refresh tokens where access_token_id IN (刚才 revoke 的 access tokens)
	//    使用子查询 + JSON 匹配确保只选刚才被撤销的
	refreshResult, err := r.db.ExecContext(ctx, queryRevokeRefreshByAccessToken,
		revokedAt, revokedAt, codeID, revokedAt,
	)
	if err != nil {
		return accessCount, err
	}
	refreshCount, err := refreshResult.RowsAffected()
	if err != nil {
		return accessCount, err
	}

	return accessCount + refreshCount, nil
}

// Delete 按 id 删除 access token。
func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteToken, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("token not found")
	}
	return nil
}

// ── mappers ──────────────────────────────────────────────────────

func toToken(row tokenRow) (*domain.Token, error) {
	token := &domain.Token{
		ID:                    row.ID,
		UserID:                row.UserID,
		Username:              row.Username,
		OrganizationID:        row.OrganizationID,
		OrganizationSlug:      row.OrganizationSlug,
		ApplicationID:         row.ApplicationID,
		ApplicationSlug:       row.ApplicationSlug,
		TokenPrefix:           row.TokenPrefix,
		TokenHash:             row.TokenHash,
		DeviceSecretHash:      row.DeviceSecretHash,
		DeviceSecretExpiresIn: row.DeviceSecretExpiresIn,
		ExpiresAt:             row.ExpiresAt,
		RevokedAt:             row.RevokedAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		Revision:              row.Revision,
	}

	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &token.Metadata); err != nil {
			return nil, apperrors.NewInternal("decode token metadata")
		}
	}
	if token.Metadata == nil {
		token.Metadata = map[string]any{}
	}

	if row.Scopes != "" {
		_ = json.Unmarshal([]byte(row.Scopes), &token.Scopes)
	}
	if token.Scopes == nil {
		token.Scopes = []string{}
	}

	if row.Audience != "" {
		_ = json.Unmarshal([]byte(row.Audience), &token.Audience)
	}
	if token.Audience == nil {
		token.Audience = []string{}
	}

	return token, nil
}

func toRefreshToken(row refreshTokenRow) (*domain.RefreshToken, error) {
	rt := &domain.RefreshToken{
		ID:               row.ID,
		AccessTokenID:    row.AccessTokenID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		RefreshTokenHash: row.RefreshTokenHash,
		FamilyID:         row.FamilyID,
		ExpiresAt:        row.ExpiresAt,
		RevokedAt:        row.RevokedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
	}

	if row.Scopes != "" {
		_ = json.Unmarshal([]byte(row.Scopes), &rt.Scopes)
	}
	if rt.Scopes == nil {
		rt.Scopes = []string{}
	}

	if row.Audience != "" {
		_ = json.Unmarshal([]byte(row.Audience), &rt.Audience)
	}
	if rt.Audience == nil {
		rt.Audience = []string{}
	}

	return rt, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
