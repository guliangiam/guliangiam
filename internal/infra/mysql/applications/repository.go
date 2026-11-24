package applications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 applications 存储层。
// 与 internal/infra/postgres/applications.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// rowsAffected 从 sql.Result 安全提取影响行数，替代 PG 的 commandTag.RowsAffected()。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// beginner 是能开启 MySQL 事务的东西（*sql.DB，实现 mysqlshared.Beginner）。
// *sql.Tx 不实现 Beginner（嵌套事务用 savepoint），所以 withinTransaction 会在 tx 上直接执行。
type beginner interface {
	mysqlshared.DBTX
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// withinTransaction 在 *sql.DB 上开启真实事务，否则直接执行。
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx mysqlshared.DBTX) error) error {
	if b, ok := r.db.(beginner); ok {
		tx, err := b.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if err := fn(ctx, tx); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// r.db 不支持 Begin（e.g. 已经是 *sql.Tx），直接在 DBTX 上执行。
	return fn(ctx, r.db)
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanApplicationRow 扫描 guliangiam.applications 表的 SELECT *。
// 列顺序必须与 sql.go 中所有 SELECT 查询严格一致。
func scanApplicationRow(s scannable) (applicationRow, error) {
	var row applicationRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Slug, &row.DisplayName, &row.Category,
		&row.AppType, &row.Description, &row.HomepageURL, &row.Domain, &row.Status,
		&row.ClientID, &row.RedirectURIs, &row.GrantTypes, &row.ClientSecretHash, &row.TokenEndpointAuthMethod,
		&row.CustomScopes, &row.DisableSignin, &row.Tags, &row.EnableExclusiveSignin, &row.EnableSigninSession,
		&row.Logo, &row.EnableSamlCompress,
		&row.SignupItems, &row.TokenFormat, &row.ExpireInHours, &row.RefreshTokenExpireInHours, &row.TokenSigningMethod,
		&row.ProviderItems, &row.SamlReplyUrl, &row.Certificate, &row.CustomCss, &row.CustomHeader,
		&row.SigninUrl, &row.LogoutUrl, &row.LogoutRedirectUrl, &row.IsPortal, &row.IsVisible,
		&row.PromptUrl, &row.SamlMetadata,
		// Phase 2 extended fields
		&row.IsShared, &row.Title, &row.Favicon, &row.MenuOrder, &row.MenuMode,
		&row.EnableSignup, &row.EnableGuestSignin, &row.EnableAutoSignin, &row.EnableLinkWithEmail, &row.CookieExpireInHours,
		&row.SignupUrl, &row.ForgetUrl, &row.AffiliationUrl, &row.TokenFields, &row.TokenAttributes,
		&row.ForcedRedirectOrigin, &row.CodeExpire, &row.SamlNameIdFormat, &row.SamlNameIdContent, &row.SamlSignRequest,
		&row.SamlMetadataUrl, &row.SamlAttributes, &row.EnableSamlC14n10, &row.EnableSamlPostBinding, &row.DisableSamlAttributes,
		&row.EnableSamlAssertionSignature, &row.UseEmailAsSamlNameId, &row.ConsentMode, &row.DpopRequired, &row.PkceRequired,
		&row.IpWhitelist, &row.IpBlacklist, &row.FailedSigninLimit, &row.FailedSigninFrozenTime, &row.CodeResendTimeout,
		&row.CorsAllowedOrigins, &row.EnableBetaOidc, &row.DefaultGroup, &row.OrgChoiceMode, &row.FormCss,
		&row.FormCssMobile, &row.FormSideHtml, &row.FormBackgroundUrl, &row.FormBackgroundUrlMobile, &row.FormOffset,
		&row.FormPosition, &row.HeaderHtml, &row.FooterHtml, &row.SignupHtml, &row.SigninHtml,
		&row.TermsOfUse, &row.PrivacyPolicy, &row.BackchannelLogoutUri, &row.FrontchannelLogoutUri, &row.FrontchannelLogoutSessionRequired, &row.OtherDomains, &row.UpstreamHost,
		&row.SslMode, &row.SslCert, &row.ClientCert, &row.SamlHashAlgorithm, &row.SamlC14nPrefix, &row.PageHtml, &row.SigninMethods, &row.SigninItems, &row.CustomJs,
		// Audit columns
		&row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.CreatedBy, &row.UpdatedBy,
		&row.Revision,
	)
	return row, err
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	return r.getByIDQuery(ctx, queryGetApplicationByID, id)
}

// GetByIDForOrganization is the management-route lookup: the tenant predicate is
// in SQL, so an id from another organization behaves exactly like a missing one.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Application, error) {
	return r.getByIDQuery(ctx, queryGetApplicationByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Application, error) {
	row, err := scanApplicationRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("application not found")
		}
		return nil, err
	}

	application, err := toApplication(row)
	if err != nil {
		return nil, err
	}
	bindings, err := r.listProviderBindings(ctx, application.ID)
	if err != nil {
		return nil, err
	}
	application.ProviderBindings = bindings
	return &application, nil
}

func (r Repository) GetByClientID(ctx context.Context, clientID string) (*domain.Application, error) {
	row, err := scanApplicationRow(r.db.QueryRowContext(ctx, queryGetApplicationByClientID, clientID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("application not found")
		}
		return nil, err
	}
	application, err := toApplication(row)
	if err != nil {
		return nil, err
	}
	bindings, err := r.listProviderBindings(ctx, application.ID)
	if err != nil {
		return nil, err
	}
	application.ProviderBindings = bindings
	return &application, nil
}

func (r Repository) GetByOrganizationAndSlug(ctx context.Context, organizationID, slug string) (*domain.Application, error) {
	row, err := scanApplicationRow(r.db.QueryRowContext(ctx, queryGetApplicationByOrganizationAndSlug, organizationID, slug))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("application not found")
		}
		return nil, err
	}
	application, err := toApplication(row)
	if err != nil {
		return nil, err
	}
	bindings, err := r.listProviderBindings(ctx, application.ID)
	if err != nil {
		return nil, err
	}
	application.ProviderBindings = bindings
	return &application, nil
}

// List 参数：orgChk, org, kwChk, kw1, kw2, kw3, stChk, st, catChk, cat, limit, offset = 12
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	rows, err := r.db.QueryContext(ctx, queryListApplications,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		query.Category, query.Category,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Application, 0)
	for rows.Next() {
		row, err := scanApplicationRow(rows)
		if err != nil {
			return nil, 0, err
		}
		application, err := toApplication(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, &application)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// 批量拉取 provider bindings 避免 N+1
	if len(items) > 0 {
		ids := make([]string, len(items))
		for i, app := range items {
			ids[i] = app.ID
		}
		bindingsMap, err := r.batchListProviderBindings(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		for _, app := range items {
			app.ProviderBindings = bindingsMap[app.ID]
		}
	}

	// Count 参数同 List 去掉 limit/offset = 10
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountApplications,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		query.Category, query.Category,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, application *domain.Application) error {
	tags, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.Tags))
	if err != nil {
		return err
	}
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryInsertApplication,
			application.ID, application.OrganizationID, application.Slug, application.DisplayName,
			application.Category, application.AppType, application.Description, application.HomepageURL,
			application.Domain, application.Status, application.ClientID, tags, application.Logo,
			application.IsPortal, application.IsVisible, application.IsShared, application.Title, application.Favicon,
			application.MenuOrder, application.MenuMode, application.CreatedAt, application.UpdatedAt, application.Revision,
		); err != nil {
			return err
		}
		return upsertApplicationConfigs(ctx, tx, application)
	})
}

// Delete 软删除 application，同时原子清理 provider_bindings 和 consents。
// 先执行带租户谓词的软删除：0 行即 NotFound，跨租户 id 不会触达子表清理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		res, err := tx.ExecContext(ctx, queryDeleteApplication, id, organizationID)
		n, err := rowsAffected(res, err)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperrors.NewNotFound("application not found")
		}
		if _, err := tx.ExecContext(ctx, queryDeleteApplicationProviderBindings, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, queryDeleteApplicationConsents, id); err != nil {
			return err
		}
		return nil
	})
}

// Update 在同一事务中更新主表与配置子表，全部成功后才推进内存中的 revision。
func (r Repository) Update(ctx context.Context, application *domain.Application) error {
	tags, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.Tags))
	if err != nil {
		return err
	}
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		res, err := tx.ExecContext(ctx, queryUpdateApplication,
			application.Slug, application.DisplayName, application.Category, application.AppType,
			application.Description, application.HomepageURL, application.Domain, application.Status,
			application.ClientID, tags, application.Logo,
			application.IsPortal, application.IsVisible, application.IsShared, application.Title, application.Favicon,
			application.MenuOrder, application.MenuMode, application.UpdatedAt,
			application.ID, application.Revision-1, application.OrganizationID,
		)
		n, err := rowsAffected(res, err)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperrors.NewConflict("application revision conflict")
		}

		if err := upsertApplicationConfigs(ctx, tx, application); err != nil {
			return err
		}

		var newRevision int64
		if err := tx.QueryRowContext(ctx, querySelectRevisionUpdate, application.ID, application.OrganizationID).Scan(&newRevision); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperrors.NewNotFound("application not found")
			}
			return err
		}
		application.Revision = newRevision
		return nil
	})
}

func upsertApplicationConfigs(ctx context.Context, tx mysqlshared.DBTX, application *domain.Application) error {
	redirectURIs, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.RedirectURIs))
	if err != nil {
		return err
	}
	grantTypes, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.GrantTypes))
	if err != nil {
		return err
	}
	customScopes, err := mysqlshared.MarshalJSON(application.CustomScopes)
	if err != nil {
		return err
	}
	tokenAttributes, err := mysqlshared.MarshalJSON(application.TokenAttributes)
	if err != nil {
		return err
	}
	tokenFields := application.TokenFields
	if tokenFields == "" {
		tokenFields = "[]"
	}
	if _, err := tx.ExecContext(ctx, queryUpsertOAuthConfig,
		application.ID, redirectURIs, grantTypes, application.EffectiveTokenEndpointAuthMethod(), application.ClientSecretHash,
		customScopes, application.TokenFormat, application.ExpireInHours, application.RefreshTokenExpireInHours,
		application.TokenSigningMethod, tokenFields, tokenAttributes, application.DpopRequired, application.PkceRequired,
		application.CookieExpireInHours, application.EnableBetaOidc, application.ConsentMode,
		application.BackchannelLogoutUri, application.FrontchannelLogoutUri, application.FrontchannelLogoutSessionRequired,
	); err != nil {
		return err
	}

	signinMethods, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.SigninMethods))
	if err != nil {
		return err
	}
	signinItems, err := mysqlshared.MarshalJSON(application.SigninItems)
	if err != nil {
		return err
	}
	corsAllowedOrigins, err := mysqlshared.MarshalJSON(splitNonEmpty(application.CorsAllowedOrigins, ","))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertSigninConfig,
		application.ID, application.DisableSignin, application.EnableExclusiveSignin, application.EnableSigninSession,
		application.EnableSignup, application.EnableGuestSignin,
		application.EnableAutoSignin, application.EnableLinkWithEmail,
		application.SigninUrl, application.LogoutUrl, application.LogoutRedirectUrl, application.PromptUrl,
		application.SignupUrl, application.ForgetUrl, application.AffiliationUrl, signinMethods, signinItems,
		application.FailedSigninLimit, application.FailedSigninFrozenTime, application.CodeResendTimeout, application.CodeExpire,
		application.DefaultGroup, application.OrgChoiceMode, application.IpWhitelist, application.IpBlacklist, corsAllowedOrigins,
	); err != nil {
		return err
	}

	samlAttributes, err := mysqlshared.MarshalJSON(application.SamlAttributes)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertSAMLConfig,
		application.ID, application.SamlReplyUrl, application.Certificate, application.SamlMetadata, application.SamlMetadataUrl,
		application.EnableSamlCompress, application.SamlNameIdFormat, application.SamlNameIdContent, application.SamlSignRequest,
		samlAttributes, application.EnableSamlC14n10, application.EnableSamlPostBinding, application.DisableSamlAttributes,
		application.EnableSamlAssertionSignature, application.UseEmailAsSamlNameId, application.SamlHashAlgorithm, application.SamlC14nPrefix,
	); err != nil {
		return err
	}

	signupItems, err := mysqlshared.MarshalJSON(application.SignupItems)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertFormConfig,
		application.ID, signupItems, application.CustomCss, application.CustomHeader, application.FormCss, application.FormCssMobile,
		application.FormSideHtml, application.FormBackgroundUrl, application.FormBackgroundUrlMobile, application.FormOffset,
		application.FormPosition, application.HeaderHtml, application.FooterHtml, application.SignupHtml, application.SigninHtml,
		application.PageHtml, application.CustomJs, application.TermsOfUse, application.PrivacyPolicy,
	); err != nil {
		return err
	}

	providerItems, err := mysqlshared.MarshalJSON(application.ProviderItems)
	if err != nil {
		return err
	}
	otherDomains, err := mysqlshared.MarshalJSON(stringSliceOrEmpty(application.OtherDomains))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, queryUpsertNetworkConfig,
		application.ID, providerItems, application.ForcedRedirectOrigin, otherDomains,
		application.UpstreamHost, application.SslMode, application.SslCert, application.ClientCert,
	)
	return err
}

func stringSliceOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func splitNonEmpty(value, separator string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, separator)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// ReplaceProviderBindings 原子替换一个 application 的所有 provider bindings。
func (r Repository) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []domain.ApplicationProviderBinding) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryDeleteApplicationProviderBindings, applicationID); err != nil {
			return err
		}

		for _, binding := range bindings {
			if _, err := tx.ExecContext(ctx, queryInsertApplicationProviderBinding, applicationID, binding.ProviderID); err != nil {
				return err
			}
		}

		return nil
	})
}

var _ domain.Repository = Repository{}

// HasDeletedWithOrgAndSlug 检查是否存在软删除的 org+slug 组合。
func (r Repository) HasDeletedWithOrgAndSlug(ctx context.Context, organizationID, slug string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, queryHasDeletedWithOrgAndSlug, organizationID, slug).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// SetRegistrationAccessToken 动态客户端 registration_access_token 哈希窄写。
func (r Repository) SetRegistrationAccessToken(ctx context.Context, applicationID, tokenHash string) error {
	res, err := r.db.ExecContext(ctx, querySetRegistrationAccessToken, tokenHash, applicationID)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("application not found")
	}
	return nil
}

// GetRegistrationAccessTokenHash 返回动态客户端存储的 registration_access_token 哈希。
func (r Repository) GetRegistrationAccessTokenHash(ctx context.Context, clientID string) (string, error) {
	var hash *string
	err := r.db.QueryRowContext(ctx, queryGetRegistrationAccessTokenHash, clientID).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", apperrors.NewNotFound("client not found")
		}
		return "", err
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

// listProviderBindings 查询单个 application 的 provider bindings。
func (r Repository) listProviderBindings(ctx context.Context, applicationID string) ([]domain.ApplicationProviderBinding, error) {
	rows, err := r.db.QueryContext(ctx, queryGetApplicationProviderBindingsByApplicationID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]domain.ApplicationProviderBinding, 0)
	for rows.Next() {
		var row applicationProviderBindingRow
		if err := rows.Scan(&row.ApplicationID, &row.ProviderID); err != nil {
			return nil, err
		}
		bindings = append(bindings, domain.ApplicationProviderBinding(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return bindings, nil
}

// batchListProviderBindings 动态展开 IN 子句占位符（MySQL database/sql 不支持 IN (?) + slice）。
func (r Repository) batchListProviderBindings(ctx context.Context, applicationIDs []string) (map[string][]domain.ApplicationProviderBinding, error) {
	if len(applicationIDs) == 0 {
		return make(map[string][]domain.ApplicationProviderBinding), nil
	}

	placeholders := make([]string, len(applicationIDs))
	args := make([]any, len(applicationIDs))
	for i, id := range applicationIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(queryBatchGetApplicationProviderBindingsByApplicationIDs, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]domain.ApplicationProviderBinding, len(applicationIDs))
	for rows.Next() {
		var row applicationProviderBindingRow
		if err := rows.Scan(&row.ApplicationID, &row.ProviderID); err != nil {
			return nil, err
		}
		result[row.ApplicationID] = append(result[row.ApplicationID], domain.ApplicationProviderBinding(row))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return result, nil
}
