package applications

import (
	"context"
	"errors"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	tenantctx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// beginner is implemented by both pgx.Pool and pgx.Tx (which uses savepoints).
// This allows withinTransaction to work whether r.db is a pool connection or
// an already-active transaction.
type beginner interface {
	shared.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// withinTransaction executes fn inside a database transaction. If r.db implements
// beginner (i.e. is a pgx.Pool or pgx.Tx), a new transaction or savepoint is
// opened. Otherwise fn is called directly (backward compatibility for test mocks
// that only implement DBTX).
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx shared.DBTX) error) error {
	if b, ok := r.db.(beginner); ok {
		tx, err := b.Begin(ctx)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback(ctx)
			}
		}()

		if err := fn(ctx, tx); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// r.db does not support Begin (e.g. test mock); execute directly.
	return fn(ctx, r.db)
}

// scannable is satisfied by both pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

// scanApplicationRow scans ALL columns from the full-detail SELECT (GetByID/GetByClientID).
// Column order must match queryGetApplicationByID in sql_detail.go.
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

// scanApplicationListRow scans the 29-column projection returned by queryListApplications.
// List query is intentionally slim (apps main table + 2 config tables), so we scan only
// the columns that exist and zero-value the rest via applicationRow's default init.
func scanApplicationListRow(s scannable) (applicationRow, error) {
	var row applicationRow
	err := s.Scan(
		// apps main table (24 columns)
		&row.ID, &row.OrganizationID, &row.Slug, &row.DisplayName,
		&row.Category, &row.AppType, &row.Description, &row.HomepageURL,
		&row.Domain, &row.Status, &row.ClientID, &row.Tags, &row.Logo,
		&row.IsPortal, &row.IsVisible, &row.IsShared, &row.Title, &row.Favicon,
		&row.MenuOrder, &row.MenuMode,
		&row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.Revision,
		// oauth config (3 columns)
		&row.TokenFormat, &row.GrantTypes, &row.TokenEndpointAuthMethod,
		// signin config (3 columns)
		&row.DisableSignin, &row.EnableSignup,
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
	row, err := scanApplicationRow(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
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
	row, err := scanApplicationRow(r.db.QueryRow(ctx, queryGetApplicationByClientID, clientID))
	if err != nil {
		if err == pgx.ErrNoRows {
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
	row, err := scanApplicationRow(r.db.QueryRow(ctx, queryGetApplicationByOrganizationAndSlug, organizationID, slug))
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListApplications, query.OrganizationID, query.Keyword, query.Status, query.Category, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Application, 0)
	for rows.Next() {
		row, err := scanApplicationListRow(rows)
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

	// Batch-fetch provider bindings for all applications in a single query
	// to avoid the N+1 query problem.
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

	var total int64
	if err := r.db.QueryRow(ctx, queryCountApplications, query.OrganizationID, query.Keyword, query.Status, query.Category).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, application *domain.Application) error {
	if err := r.createApplication(ctx, application); err != nil {
		return mapCreateApplicationError(err)
	}
	return nil
}

// mapCreateApplicationError 把 INSERT 的唯一约束冲突收敛为 409。
//
// 应用层 GetByOrganizationAndSlug 预检与 INSERT 不是原子操作：两个并发同名
// 创建请求可以同时通过预检，落败的一方命中 uq_applications_org_slug
// (SQLSTATE 23505)。不映射时原始 *pgconn.PgError 会一路冒泡到
// route.WriteError，被渲染成不透明 500，而获胜方的记录已经提交——表现为
// “写成功、界面报服务暂时不可用”。
func mapCreateApplicationError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerror.CodeUniqueViolation {
		return apperrors.Wrap(apperrors.CodeAlreadyExists, err, "application slug or client id already exists")
	}
	return err
}

func (r Repository) createApplication(ctx context.Context, application *domain.Application) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		// 1. 序列化 JSON 字段
		customScopes, err := shared.MarshalJSON(application.CustomScopes)
		if err != nil {
			return err
		}
		signupItems, err := shared.MarshalJSON(application.SignupItems)
		if err != nil {
			return err
		}
		providerItems, err := shared.MarshalJSON(application.ProviderItems)
		if err != nil {
			return err
		}
		tokenAttributes, err := shared.MarshalJSON(application.TokenAttributes)
		if err != nil {
			return err
		}
		samlAttributes, err := shared.MarshalJSON(application.SamlAttributes)
		if err != nil {
			return err
		}
		signinMethodNames := application.SigninMethods
		if signinMethodNames == nil {
			signinMethodNames = []string{}
		}
		signinItemsJSON, err := shared.MarshalJSON(application.SigninItems)
		if err != nil {
			return err
		}
		corsOrigins := splitNonEmpty(application.CorsAllowedOrigins, ",")
		if corsOrigins == nil {
			corsOrigins = []string{}
		}
		redirectURIs := application.RedirectURIs
		if redirectURIs == nil {
			redirectURIs = []string{}
		}
		grantTypes := application.GrantTypes
		if grantTypes == nil {
			grantTypes = []string{}
		}
		otherDomains := application.OtherDomains
		if otherDomains == nil {
			otherDomains = []string{}
		}

		// PG jsonb column token_fields requires valid JSON text; Go zero-value
		// empty string '' would cause 'invalid input syntax for type json'.
		tokenFieldsJSON := application.TokenFields
		if tokenFieldsJSON == "" {
			tokenFieldsJSON = "[]"
		}

		tags := application.Tags
		if tags == nil {
			tags = []string{}
		}

		// 2. INSERT 核心表
		if _, err := tx.Exec(ctx, queryInsertApplicationCore,
			application.ID, application.OrganizationID, application.Slug, application.DisplayName,
			application.Category, application.AppType, application.Description, application.HomepageURL,
			application.Domain, application.Status, application.ClientID, tags, application.Logo,
			application.IsPortal, application.IsVisible, application.IsShared, application.Title, application.Favicon,
			application.MenuOrder, application.MenuMode, application.CreatedAt, application.UpdatedAt, application.Revision,
		); err != nil {
			return err
		}

		// 3. INSERT/OFF CONFLICT 5 个子表
		// OAuth config
		if _, err := tx.Exec(ctx, queryInsertOAuthConfig,
			application.ID, redirectURIs, grantTypes,
			application.EffectiveTokenEndpointAuthMethod(), application.ClientSecretHash,
			nil, customScopes, application.TokenFormat, application.ExpireInHours,
			application.RefreshTokenExpireInHours, application.TokenSigningMethod,
			tokenFieldsJSON, tokenAttributes,
			application.DpopRequired, application.PkceRequired, application.CookieExpireInHours,
			application.EnableBetaOidc, application.ConsentMode, application.BackchannelLogoutUri, application.FrontchannelLogoutUri, application.FrontchannelLogoutSessionRequired,
		); err != nil {
			return err
		}

		// Signin config
		if _, err := tx.Exec(ctx, queryInsertSigninConfig,
			application.ID, application.DisableSignin, application.EnableExclusiveSignin,
			application.EnableSigninSession, application.EnableSignup, application.EnableGuestSignin,
			application.EnableAutoSignin, application.EnableLinkWithEmail,
			application.SigninUrl, application.LogoutUrl, application.LogoutRedirectUrl,
			application.PromptUrl, application.SignupUrl, application.ForgetUrl, application.AffiliationUrl,
			signinMethodNames, signinItemsJSON, application.FailedSigninLimit, application.FailedSigninFrozenTime,
			application.CodeResendTimeout, application.CodeExpire, application.DefaultGroup, application.OrgChoiceMode,
			application.IpWhitelist, application.IpBlacklist, corsOrigins,
		); err != nil {
			return err
		}

		// SAML config
		if _, err := tx.Exec(ctx, queryInsertSamlConfig,
			application.ID, application.SamlReplyUrl, application.Certificate,
			application.SamlMetadata, application.SamlMetadataUrl, application.EnableSamlCompress,
			application.SamlNameIdFormat, application.SamlNameIdContent, application.SamlSignRequest,
			samlAttributes, application.EnableSamlC14n10, application.EnableSamlPostBinding,
			application.DisableSamlAttributes, application.EnableSamlAssertionSignature,
			application.UseEmailAsSamlNameId, application.SamlHashAlgorithm, application.SamlC14nPrefix,
		); err != nil {
			return err
		}

		// Form config
		if _, err := tx.Exec(ctx, queryInsertFormConfig,
			application.ID, signupItems, application.CustomCss, application.CustomHeader,
			application.FormCss, application.FormCssMobile, application.FormSideHtml,
			application.FormBackgroundUrl, application.FormBackgroundUrlMobile,
			application.FormOffset, application.FormPosition,
			application.HeaderHtml, application.FooterHtml, application.SignupHtml,
			application.SigninHtml, application.PageHtml, application.CustomJs,
			application.TermsOfUse, application.PrivacyPolicy,
		); err != nil {
			return err
		}

		// Network config
		if _, err := tx.Exec(ctx, queryInsertNetworkConfig,
			application.ID, providerItems, application.ForcedRedirectOrigin,
			otherDomains, application.UpstreamHost,
			application.SslMode, application.SslCert, application.ClientCert,
		); err != nil {
			return err
		}

		return nil
	})
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		// 仅跨租户（platform_admin / system）场景需要 LOCAL 级 RLS 提升。
		// 普通成员必须保持 RLS 隔离，避免在 Casbin 判定之后再绕过数据库层租户边界。
		scope := tenantctx.TenantScopeFromContext(ctx)
		if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
			if _, err := tx.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`); err != nil {
				return err
			}
		}

		// 先执行带租户谓词的主删除：0 行即 NotFound，跨租户 id 不会触达子表清理。
		result, err := tx.Exec(ctx, queryDeleteApplication, id, organizationID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return apperrors.NewNotFound("application not found")
		}
		// 归属已确认，再清理子表（FK 为 ON DELETE CASCADE，这里保留幂等显式清理）。
		if _, err := tx.Exec(ctx, queryDeleteApplicationProviderBindings, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queryDeleteApplicationConsents, id); err != nil {
			return err
		}
		return nil
	})
}

func (r Repository) Update(ctx context.Context, application *domain.Application) error {
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		customScopes, err := shared.MarshalJSON(application.CustomScopes)
		if err != nil {
			return err
		}
		signupItems, err := shared.MarshalJSON(application.SignupItems)
		if err != nil {
			return err
		}
		providerItems, err := shared.MarshalJSON(application.ProviderItems)
		if err != nil {
			return err
		}
		tokenAttributes, err := shared.MarshalJSON(application.TokenAttributes)
		if err != nil {
			return err
		}
		samlAttributes, err := shared.MarshalJSON(application.SamlAttributes)
		if err != nil {
			return err
		}
		signinItemsJSON, err := shared.MarshalJSON(application.SigninItems)
		if err != nil {
			return err
		}
		signinMethodNames := application.SigninMethods
		corsOrigins := splitNonEmpty(application.CorsAllowedOrigins, ",")

		tokenFieldsJSON := application.TokenFields
		if tokenFieldsJSON == "" {
			tokenFieldsJSON = "[]"
		}

		// nil-slice safety for NOT NULL PG columns — pgx serializes nil as SQL NULL
		tags := application.Tags
		if tags == nil {
			tags = []string{}
		}
		// PATCH path: 关联表 NOT NULL 列的 nil 也必须 coerce 到空 slice
		redirectURIs := application.RedirectURIs
		if redirectURIs == nil {
			redirectURIs = []string{}
		}
		grantTypes := application.GrantTypes
		if grantTypes == nil {
			grantTypes = []string{}
		}
		// app_signin_config.signin_methods (text[]) 也需要
		if signinMethodNames == nil {
			signinMethodNames = []string{}
		}

		var newRevision int64
		err = tx.QueryRow(ctx, queryUpdateApplicationCore,
			application.Slug, application.DisplayName, application.Category, application.AppType,
			application.Description, application.HomepageURL, application.Domain, application.Status,
			application.ClientID, tags, application.Logo,
			application.IsPortal, application.IsVisible, application.IsShared, application.Title, application.Favicon,
			application.MenuOrder, application.MenuMode, application.ID, application.OrganizationID,
		).Scan(&newRevision)
		if err != nil {
			if err == pgx.ErrNoRows {
				return apperrors.NewConflict("application revision conflict")
			}
			return err
		}
		application.Revision = newRevision

		if _, err := tx.Exec(ctx, queryInsertOAuthConfig,
			application.ID, redirectURIs, grantTypes,
			application.EffectiveTokenEndpointAuthMethod(), application.ClientSecretHash,
			nil, customScopes, application.TokenFormat, application.ExpireInHours,
			application.RefreshTokenExpireInHours, application.TokenSigningMethod,
			tokenFieldsJSON, tokenAttributes,
			application.DpopRequired, application.PkceRequired, application.CookieExpireInHours,
			application.EnableBetaOidc, application.ConsentMode, application.BackchannelLogoutUri, application.FrontchannelLogoutUri, application.FrontchannelLogoutSessionRequired,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, queryInsertSigninConfig,
			application.ID, application.DisableSignin, application.EnableExclusiveSignin,
			application.EnableSigninSession, application.EnableSignup, application.EnableGuestSignin,
			application.EnableAutoSignin, application.EnableLinkWithEmail,
			application.SigninUrl, application.LogoutUrl, application.LogoutRedirectUrl,
			application.PromptUrl, application.SignupUrl, application.ForgetUrl, application.AffiliationUrl,
			signinMethodNames, signinItemsJSON, application.FailedSigninLimit, application.FailedSigninFrozenTime,
			application.CodeResendTimeout, application.CodeExpire, application.DefaultGroup, application.OrgChoiceMode,
			application.IpWhitelist, application.IpBlacklist, corsOrigins,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, queryInsertSamlConfig,
			application.ID, application.SamlReplyUrl, application.Certificate,
			application.SamlMetadata, application.SamlMetadataUrl, application.EnableSamlCompress,
			application.SamlNameIdFormat, application.SamlNameIdContent, application.SamlSignRequest,
			samlAttributes, application.EnableSamlC14n10, application.EnableSamlPostBinding,
			application.DisableSamlAttributes, application.EnableSamlAssertionSignature,
			application.UseEmailAsSamlNameId, application.SamlHashAlgorithm, application.SamlC14nPrefix,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, queryInsertFormConfig,
			application.ID, signupItems, application.CustomCss, application.CustomHeader,
			application.FormCss, application.FormCssMobile, application.FormSideHtml,
			application.FormBackgroundUrl, application.FormBackgroundUrlMobile,
			application.FormOffset, application.FormPosition,
			application.HeaderHtml, application.FooterHtml, application.SignupHtml,
			application.SigninHtml, application.PageHtml, application.CustomJs,
			application.TermsOfUse, application.PrivacyPolicy,
		); err != nil {
			return err
		}

		otherDomains := application.OtherDomains
		if otherDomains == nil {
			otherDomains = []string{}
		}
		if _, err := tx.Exec(ctx, queryInsertNetworkConfig,
			application.ID, providerItems, application.ForcedRedirectOrigin,
			otherDomains, application.UpstreamHost,
			application.SslMode, application.SslCert, application.ClientCert,
		); err != nil {
			return err
		}

		return nil
	})
}

func (r Repository) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []domain.ApplicationProviderBinding) error {
	// DELETE + INSERT must be atomic to prevent data loss under concurrent access.
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryDeleteApplicationProviderBindings, applicationID); err != nil {
			return err
		}

		for _, binding := range bindings {
			if _, err := tx.Exec(ctx, queryInsertApplicationProviderBinding, applicationID, binding.ProviderID); err != nil {
				return err
			}
		}

		return nil
	})
}

var _ domain.Repository = Repository{}

// HasDeletedWithOrgAndSlug checks whether a soft-deleted application with the
// given organization+slug exists.
func (r Repository) HasDeletedWithOrgAndSlug(ctx context.Context, organizationID, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM guliangiam.applications WHERE organization_id = $1::uuid AND slug = $2 AND deleted_at IS NOT NULL)`,
		organizationID, slug,
	).Scan(&exists)
	return exists, err
}

// SetRegistrationAccessToken stores the hash of a dynamic client's
// registration_access_token. The narrow UPDATE leaves every other column —
// including the 110+ that the wide statements maintain — untouched.
func (r Repository) SetRegistrationAccessToken(ctx context.Context, applicationID, tokenHash string) error {
	result, err := r.db.Exec(ctx, querySetRegistrationAccessToken, applicationID, tokenHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("application not found")
	}
	return nil
}

// GetRegistrationAccessTokenHash returns the stored hash for a client. An empty
// result means the client was not dynamically registered (or predates the
// column), and the caller must treat that as "no credential to verify against"
// rather than as a match.
func (r Repository) GetRegistrationAccessTokenHash(ctx context.Context, clientID string) (string, error) {
	var hash *string
	err := r.db.QueryRow(ctx, queryGetRegistrationAccessTokenHash, clientID).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NewNotFound("client not found")
		}
		return "", err
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

func (r Repository) listProviderBindings(ctx context.Context, applicationID string) ([]domain.ApplicationProviderBinding, error) {
	rows, err := r.db.Query(ctx, queryGetApplicationProviderBindingsByApplicationID, applicationID)
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

// batchListProviderBindings fetches provider bindings for multiple applications
// in a single query using WHERE application_id = ANY($1::uuid[]), eliminating
// the N+1 query problem when listing applications.
func (r Repository) batchListProviderBindings(ctx context.Context, applicationIDs []string) (map[string][]domain.ApplicationProviderBinding, error) {
	if len(applicationIDs) == 0 {
		return make(map[string][]domain.ApplicationProviderBinding), nil
	}

	rows, err := r.db.Query(ctx, queryBatchGetApplicationProviderBindingsByApplicationIDs, applicationIDs)
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

func splitNonEmpty(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// maxInt returns value when positive, otherwise fallback. Kept package-local
// (and renamed from max) so it does not shadow the predeclared max builtin.
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
