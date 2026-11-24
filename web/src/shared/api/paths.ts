/**
 * API 路径常量与生成器 — 唯一事实源
 *
 * 所有 iam API client 引用的 URL path 必须从此模块获取，
 * 禁止在 client 文件中直接硬编码 `/api/v1/...` 字符串。
 *
 * 命名约定：
 *   - 根路径常量：复数资源名（APPLICATIONS）或明确动作（AUTH_LOGIN）
 *   - 带动态段的路径：函数，参数名 = 段名（applicationById(id)）
 *
 * Query 字符串由各 client 自行构造（涉及各自参数逻辑），
 * paths.ts 只固定 URL path 本体（不含 `?` 及之后）。
 */

// ============================================================================
// Applications
// ============================================================================
export const APPLICATIONS = "/api/v1/applications";
export const APPLICATIONS_RESOLVE = "/api/v1/applications/resolve";
export const APPLICATION_BY_ID = (id: string) => `/api/v1/applications/${encodeURIComponent(id)}`;
export const APPLICATION_CLIENT_SECRET_ROTATE = (id: string) =>
  `/api/v1/applications/${encodeURIComponent(id)}/client-secret/rotate`;
export const APPLICATION_CLIENT_CERTIFICATES = (id: string) =>
  `/api/v1/applications/${encodeURIComponent(id)}/client-certificates`;
export const APPLICATION_DELETE_SUMMARY = (id: string) =>
  `/api/v1/applications/${encodeURIComponent(id)}/delete-summary`;
export const APPLICATION_EXPORT = (id: string) =>
  `/api/v1/applications/${encodeURIComponent(id)}/export`;
export const APPLICATION_PROVIDER_BINDINGS = (id: string) =>
  `/api/v1/applications/${encodeURIComponent(id)}/provider-bindings`;
export const APPLICATION_BY_CLIENT_ID = (clientId: string) =>
  `/api/v1/applications/by-client-id/${encodeURIComponent(clientId)}`;

// ============================================================================
// Auth
// ============================================================================
export const AUTH_ACCOUNT = "/api/v1/auth/account";
export const AUTH_AUTHORIZE = "/api/v1/auth/authorize";
export const AUTH_CAPTCHA = "/api/v1/auth/captcha";
export const AUTH_CAPTCHA_STATUS = "/api/v1/auth/captcha-status";
export const AUTH_CAPTCHA_VERIFY = "/api/v1/auth/captcha/verify";
export const AUTH_CAS_LOGOUT = "/api/v1/auth/cas/logout";
export const AUTH_EXIT_IMPERSONATE = "/api/v1/auth/exit-impersonate";
export const AUTH_LOGIN = "/api/v1/auth/login";
export const AUTH_LOGIN_LDAP = "/api/v1/auth/login/ldap";
export const AUTH_LOGOUT = "/api/v1/auth/logout";
export const AUTH_PASSWORD_CHANGE = "/api/v1/auth/password/change";
export const AUTH_SOCIAL_AUTHORIZE = "/api/v1/auth/social/authorize";
export const AUTH_SOCIAL_LOGIN = "/api/v1/auth/social/login";
export const AUTH_SAML_REDIRECT = (orgSlug: string, appSlug: string) =>
  `/api/v1/auth/saml/redirect/${encodeURIComponent(orgSlug)}/${encodeURIComponent(appSlug)}`;
export const AUTH_SAML_METADATA = (orgSlug: string, appId: string) =>
  `/api/v1/auth/saml/metadata/${encodeURIComponent(orgSlug)}/${encodeURIComponent(appId)}`;
export const AUTH_SAML_ACS = "/api/v1/auth/saml/acs";
export const AUTH_SAML_SP_ACS = (orgSlug: string, providerId: string) =>
  `/api/v1/auth/saml/acs/${encodeURIComponent(orgSlug)}/${encodeURIComponent(providerId)}`;
export const AUTH_SSO_LOGOUT = "/api/v1/auth/sso-logout";
export const AUTH_TOKENS_EXCHANGE = "/api/v1/auth/tokens/exchange";
export const AUTH_VERIFY_CODE = "/api/v1/auth/verify-code";
export const AUTH_WEBAUTHN_REGISTRATION_BEGIN = "/api/v1/auth/webauthn/registration/begin";
export const AUTH_WEBAUTHN_REGISTRATION_FINISH = "/api/v1/auth/webauthn/registration/finish";
export const AUTH_WEBAUTHN_AUTHENTICATION_BEGIN = "/api/v1/auth/webauthn/authentication/begin";
export const AUTH_WEBAUTHN_AUTHENTICATION_FINISH = "/api/v1/auth/webauthn/authentication/finish";
export const AUTH_WEBAUTHN_CREDENTIALS = "/api/v1/auth/webauthn/credentials";
export const AUTH_WEBAUTHN_CREDENTIAL_BY_ID = (id: string) =>
  `/api/v1/auth/webauthn/credentials/${encodeURIComponent(id)}`;
export const AUTH_MFA_SETUP_INITIATE = "/api/v1/auth/mfa-setup/initiate";
export const AUTH_MFA_SETUP_VERIFY = "/api/v1/auth/mfa-setup/verify";
export const AUTH_MFA_SETUP_ENABLE = "/api/v1/auth/mfa-setup/enable";
export const AUTH_MFA_METHODS = "/api/v1/auth/mfa-methods";
export const AUTH_PREFERRED_MFA = "/api/v1/auth/preferred-mfa";
export const AUTH_IDENTITY_BY_ID = (identityId: string) =>
  `/api/v1/auth/identities/${encodeURIComponent(identityId)}`;
export const AUTH_CONSENTS = "/api/v1/auth/consents";
export const AUTH_PROFILE = "/api/v1/auth/profile";
export const AUTH_VERIFY_IDENTIFICATION = "/api/v1/auth/verify-identification";
export const AUTH_FACEIDS = "/api/v1/auth/faceids";
export const AUTH_LOGIN_CHALLENGE = (challengeId: string) =>
  `/api/v1/auth/login-challenges/${encodeURIComponent(challengeId)}`;
export const AUTH_LOGIN_CHALLENGE_APPROVE = (challengeId: string) =>
  `/api/v1/auth/login-challenges/${encodeURIComponent(challengeId)}/approve`;
export const AUTH_LOGIN_CHALLENGE_REJECT = (challengeId: string) =>
  `/api/v1/auth/login-challenges/${encodeURIComponent(challengeId)}/reject`;

// ============================================================================
// Users + Users nested endpoints
// ============================================================================
export const USERS = "/api/v1/users";
export const USER_BY_ID = (id: string) => `/api/v1/users/${encodeURIComponent(id)}`;
export const USER_PROFILE = (id: string) => `/api/v1/users/${encodeURIComponent(id)}/profile`;
export const USER_COLLECTIONS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/collections`;
export const USER_EFFECTIVE_PERMISSIONS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/permissions`;
export const USER_CONTACT_VERIFICATION_SEND = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/contact-verification/send`;
export const USER_CONTACT_VERIFICATION_VERIFY = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/contact-verification/verify`;
export const USER_IDENTITY_DOCUMENT_RECOGNIZE = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/identity-documents/recognize`;
export const USER_FACEIDS = (id: string) => `/api/v1/users/${encodeURIComponent(id)}/faceids`;
export const USER_CUSTOM_ATTRIBUTES = "/api/v1/users/custom-attributes";
export const USER_CREDENTIALS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/credentials`;
export const USER_IDENTITIES = (id: string) => `/api/v1/users/${encodeURIComponent(id)}/identities`;
export const USER_MFA_METHODS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/mfa-methods`;
export const USER_PREFERRED_MFA = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/preferred-mfa`;
export const USER_MFA_SETUP_INITIATE = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/mfa-setup/initiate`;
export const USER_MFA_SETUP_VERIFY = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/mfa-setup/verify`;
export const USER_MFA_SETUP_ENABLE = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/mfa-setup/enable`;
export const USER_SET_PASSWORD = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/set-password`;
export const USER_REMOVE_FROM_GROUP = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/remove-from-group`;
export const USER_ROLE_BINDINGS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/role-bindings`;
export const USER_ROLE_BINDING_BY_ROLE = (userId: string, roleId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/role-bindings/${encodeURIComponent(roleId)}`;
export const USER_GROUP_MEMBERSHIPS = (id: string) =>
  `/api/v1/users/${encodeURIComponent(id)}/group-memberships`;
export const USER_GROUP_BY_NAME = (userId: string, groupId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/groups/${encodeURIComponent(groupId)}`;
export const USER_SESSIONS = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/sessions`;
export const USER_SESSION_BY_ID = (userId: string, sessionId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/sessions/${encodeURIComponent(sessionId)}`;
export const USER_CONSENTS = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/consents`;
export const USER_CONSENT_BY_ID = (userId: string, consentId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/consents/${encodeURIComponent(consentId)}`;
export const USER_TRANSACTIONS = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/transactions`;
export const USER_MASKED_CONTACT = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/masked-contact`;
export const USER_UNLINK = (userId: string) => `/api/v1/users/${encodeURIComponent(userId)}/unlink`;
export const USER_IMPERSONATE = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/impersonate`;
export const USER_ORDERS = (userId: string) => `/api/v1/users/${encodeURIComponent(userId)}/orders`;
export const USER_PAYMENTS = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/payments`;
export const USER_VERIFY_IDENTIFICATION = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/verify-identification`;
export const USER_WEBAUTHN_CREDENTIALS = (userId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/webauthn-credentials`;
export const USER_WEBAUTHN_CREDENTIAL_BY_ID = (userId: string, credentialId: string) =>
  `/api/v1/users/${encodeURIComponent(userId)}/webauthn-credentials/${encodeURIComponent(credentialId)}`;
export const USER_WEBAUTHN_CREDENTIAL_REVOKE = (userId: string, credentialId: string) =>
  `${USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId)}/revoke`;
export const USER_WEBAUTHN_CREDENTIAL_DEACTIVATE = (userId: string, credentialId: string) =>
  `${USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId)}/deactivate`;
export const USER_WEBAUTHN_CREDENTIAL_RESTORE = (userId: string, credentialId: string) =>
  `${USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId)}/restore`;
export const USER_WEBAUTHN_CREDENTIAL_RENAME = (userId: string, credentialId: string) =>
  `${USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId)}/rename`;
export const USER_WEBAUTHN_CREDENTIAL_REGISTRATION_BEGIN = (userId: string) =>
  `${USER_WEBAUTHN_CREDENTIALS(userId)}/registration/begin`;
export const USER_WEBAUTHN_CREDENTIAL_REGISTRATION_FINISH = (userId: string) =>
  `${USER_WEBAUTHN_CREDENTIALS(userId)}/registration/finish`;
export const USER_UPLOAD = "/api/v1/users/upload";
export const USER_UPLOAD_PREVIEW = "/api/v1/users/upload/preview";
export const USER_EXPORT = "/api/v1/users/export";
export const USER_COUNT = "/api/v1/users/count";

// ============================================================================
// B2B — Organization Domains
// ============================================================================
export const ORGANIZATION_DOMAINS = (orgId: string) =>
  `/api/v1/organizations/${encodeURIComponent(orgId)}/domains`;

// ============================================================================
// AI Gateway — Admin plane
// ============================================================================
export const AI_GATEWAY_BASE = "/api/v1/ai";

// ============================================================================
// Commerce — Payments Admin (channel configs / recon / attempts / refunds)
// ============================================================================
export const PAY_CHANNELS = "/api/v1/pay-channels";
export const PAY_CHANNEL_CONFIGS = "/api/v1/pay-channel-configs";
export const PAY_CHANNEL_CONFIG_BY_ID = (id: string) =>
  `/api/v1/pay-channel-configs/${encodeURIComponent(id)}`;
export const PAY_CHANNEL_CONFIG_TRANSITION = (id: string) =>
  `/api/v1/pay-channel-configs/${encodeURIComponent(id)}:transition`;
export const PAY_RECON = "/api/v1/pay-recon";
export const PAY_RECON_BATCHES = "/api/v1/pay-recon-batches";
export const PAY_RECON_BATCH_ITEMS = (batchId: string) =>
  `/api/v1/pay-recon-batches/${encodeURIComponent(batchId)}/items`;
export const PAY_RECON_ITEM_RESOLVE = (id: string) =>
  `/api/v1/pay-recon-items/${encodeURIComponent(id)}:resolve`;
export const PAY_RECON_ITEM_APPROVE = (id: string) =>
  `/api/v1/pay-recon-items/${encodeURIComponent(id)}:approve`;
export const PAY_ATTEMPT_BY_ID = (id: string) => `/api/v1/pay-attempts/${encodeURIComponent(id)}`;
export const PAYMENT_ATTEMPTS = (paymentId: string) =>
  `/api/v1/payments/${encodeURIComponent(paymentId)}/attempts`;
export const PAY_REFUND_BY_ID = (id: string) => `/api/v1/pay-refunds/${encodeURIComponent(id)}`;
export const PAYMENT_REFUNDS = (paymentId: string) =>
  `/api/v1/payments/${encodeURIComponent(paymentId)}/refunds`;
export const PAY_CALLBACK_RECEIPTS = "/api/v1/pay-callback-receipts";

// ============================================================================
// Organizations
// ============================================================================
export const ORGANIZATIONS = "/api/v1/organizations";
export const ORGANIZATIONS_NAMES = "/api/v1/organizations/names";
export const ORGANIZATION_BY_ID = (id: string) => `/api/v1/organizations/${encodeURIComponent(id)}`;
export const ORGANIZATION_BY_SLUG = (slug: string) =>
  `/api/v1/organizations/by-slug/${encodeURIComponent(slug)}`;
export const ORGANIZATION_SECURITY_POLICY = (id: string) =>
  `/api/v1/organizations/${encodeURIComponent(id)}/security-policy`;
export const ORGANIZATION_SETTINGS = (id: string) =>
  `/api/v1/organizations/${encodeURIComponent(id)}/settings`;
export const ORGANIZATION_THEME = (id: string) =>
  `/api/v1/organizations/${encodeURIComponent(id)}/theme`;
export const ORGANIZATION_APPLICATIONS = (orgId: string) =>
  `/api/v1/organizations/${encodeURIComponent(orgId)}/applications`;
export const ORGANIZATION_DEFAULT_APPLICATION = (orgId: string) =>
  `/api/v1/organizations/${encodeURIComponent(orgId)}/default-application`;
export const ORGANIZATION_DEPLOYMENT_REGIONS = "/api/v1/organizations/deployment-regions";
export const ORGANIZATION_CONFIGURATION = (id: string) =>
  `/api/v1/organizations/${encodeURIComponent(id)}/configuration`;

// ============================================================================
// Roles / Permissions / Groups
// ============================================================================
export const ROLES = "/api/v1/roles";
export const ROLE_BY_ID = (id: string) => `/api/v1/roles/${encodeURIComponent(id)}`;
export const ROLES_UPLOAD = "/api/v1/roles/upload";

export const PERMISSIONS = "/api/v1/permissions";
export const PERMISSIONS_BULK_IMPORT = "/api/v1/permissions/bulk-import";
export const PERMISSIONS_UPLOAD = "/api/v1/permissions/upload";
export const PERMISSION_BY_ID = (id: string) => `/api/v1/permissions/${encodeURIComponent(id)}`;
export const PERMISSION_ACTIONS = (id: string) =>
  `/api/v1/permissions/${encodeURIComponent(id)}/actions`;
export const PERMISSION_OBJECTS = (id: string) =>
  `/api/v1/permissions/${encodeURIComponent(id)}/objects`;
export const PERMISSION_POLICY = (id: string) =>
  `/api/v1/permissions/${encodeURIComponent(id)}/policy`;
export const PERMISSION_ROLES = (id: string) =>
  `/api/v1/permissions/${encodeURIComponent(id)}/roles`;

export const GROUPS = "/api/v1/groups";
export const GROUPS_BULK_IMPORT = "/api/v1/groups/bulk-import";
export const GROUPS_UPLOAD = "/api/v1/groups/upload";
export const GROUP_BY_ID = (id: string) => `/api/v1/groups/${encodeURIComponent(id)}`;

// ============================================================================
// Global Search
// ============================================================================
export const SEARCH = "/api/v1/search";

// ============================================================================
// Sessions / Tokens / Invitations
// ============================================================================
export const SESSIONS = "/api/v1/sessions";
export const SESSION_BY_ID = (id: string) => `/api/v1/sessions/${encodeURIComponent(id)}`;
export const SESSION_REVOKE = (sessionId: string) =>
  `/api/v1/sessions/${encodeURIComponent(sessionId)}/revoke`;
export const SESSIONS_CURRENT = "/api/v1/sessions/current";
export const SESSIONS_CURRENT_REVOKE_OTHERS = "/api/v1/sessions/current/revoke-others";
export const SESSIONS_IS_DUPLICATED = "/api/v1/sessions/is-duplicated";

export const TOKENS = "/api/v1/tokens";
export const TOKENS_REVOKE = "/api/v1/tokens/revoke";
export const TOKEN_BY_ID = (id: string) => `/api/v1/tokens/${encodeURIComponent(id)}`;
export const TOKEN_REVOKE_BY_ID = (id: string) => `/api/v1/tokens/${encodeURIComponent(id)}/revoke`;

export const INVITATIONS = "/api/v1/invitations";
export const INVITATIONS_VERIFY = "/api/v1/invitations/verify";
export const INVITATION_BY_ID = (id: string) => `/api/v1/invitations/${encodeURIComponent(id)}`;
export const INVITATION_RESEND = (id: string) =>
  `/api/v1/invitations/${encodeURIComponent(id)}/resend`;
export const INVITATION_REVOKE = (id: string) =>
  `/api/v1/invitations/${encodeURIComponent(id)}/revoke`;
export const INVITATION_SEND = (id: string) => `/api/v1/invitations/${encodeURIComponent(id)}/send`;

// ============================================================================
// Casbin domain (Enforcers / Adapters / Actions / Models)
// ============================================================================
export const ENFORCER_BY_ID = (id: string) => `/api/v1/enforcers/${id}`;
export const ENFORCER_ACTIONS = (id: string) => `/api/v1/enforcers/${id}/actions`;
export const ENFORCER_BATCH_ENFORCE = (id: string) => `/api/v1/enforcers/${id}/batch-enforce`;
export const ENFORCER_ENFORCE = (id: string) => `/api/v1/enforcers/${id}/enforce`;
export const ENFORCER_OBJECTS = (id: string) => `/api/v1/enforcers/${id}/objects`;
export const ENFORCER_POLICIES = (id: string) => `/api/v1/enforcers/${id}/policies`;
export const ENFORCER_RELOAD = (id: string) => `/api/v1/enforcers/${id}/reload`;
export const ENFORCER_ROLES = (id: string) => `/api/v1/enforcers/${id}/roles`;
export const ENFORCER_SUBJECTS = (id: string) => `/api/v1/enforcers/${id}/subjects`;
export const ENFORCER_IMPORT_POLICIES = (enforcerId: string) =>
  `/api/v1/enforcers/${enforcerId}/import-policies`;

export const ADAPTER_BY_ID = (id: string) => `/api/v1/adapters/${id}`;
export const ADAPTER_POLICIES = (id: string) => `/api/v1/adapters/${id}/policies`;

export const ACTION_BY_ID = (id: string) => `/api/v1/actions/${encodeURIComponent(id)}`;
export const ACTION_EXECUTION_LOGS = (actionId: string) =>
  `/api/v1/actions/${encodeURIComponent(actionId)}/execution-logs`;
export const ACTION_RUN = (actionId: string) =>
  `/api/v1/actions/${encodeURIComponent(actionId)}/run`;
export const ACTION_VERSIONS = (actionId: string) =>
  `/api/v1/actions/${encodeURIComponent(actionId)}/versions`;

export const MODEL_BY_ID = (id: string) => `/api/v1/models/${id}`;

// ============================================================================
// Gateway / Providers / LDAP / Syncers / Cert / Keys / Plans / Products
// ============================================================================
export const GATEWAY_SERVERS = "/api/v1/gateway/servers";
export const GATEWAY_SERVER_BY_ID = (id: string) =>
  `/api/v1/gateway/servers/${encodeURIComponent(id)}`;
export const GATEWAY_SITES = "/api/v1/gateway/sites";
export const GATEWAY_SITES_GLOBAL = "/api/v1/gateway/sites/global";
export const GATEWAY_SITE_BY_ID = (id: string) => `/api/v1/gateway/sites/${encodeURIComponent(id)}`;
export const GATEWAY_SITE_CERT_STATUS = (id: string) =>
  `/api/v1/gateway/sites/${encodeURIComponent(id)}/cert-status`;
export const GATEWAY_SITE_CHECK_HEALTH = (id: string) =>
  `/api/v1/gateway/sites/${encodeURIComponent(id)}/check-health`;
export const GATEWAY_SITE_HEALTH = (id: string) =>
  `/api/v1/gateway/sites/${encodeURIComponent(id)}/health`;
export const GATEWAY_SITE_PROVISION_CERT = (id: string) =>
  `/api/v1/gateway/sites/${encodeURIComponent(id)}/provision-cert`;
export const GATEWAY_SITE_RENEW_CERT = (id: string) =>
  `/api/v1/gateway/sites/${encodeURIComponent(id)}/renew-cert`;

export const PROVIDERS = "/api/v1/providers";
export const PROVIDER_BY_ID = (id: string) => `/api/v1/providers/${encodeURIComponent(id)}`;
export const PROVIDER_TEST_CONNECTION = (id: string) =>
  `/api/v1/providers/${encodeURIComponent(id)}/test-connection`;
export const PROVIDER_CONFIG = (id: string) => `/api/v1/providers/${encodeURIComponent(id)}/config`;

export const LDAP_BY_ID = (id: string) => `/api/v1/ldaps/${id}`;
export const LDAP_PREVIEW_USERS = (id: string) => `/api/v1/ldaps/${id}/preview-users`;
export const LDAP_SYNC = (id: string) => `/api/v1/ldaps/${id}/sync`;
export const LDAP_TEST_CONNECTION = (id: string) => `/api/v1/ldaps/${id}/test-connection`;

export const SYNCER_BY_ID = (id: string) => `/api/v1/syncers/${id}`;
export const SYNCER_RUN = (id: string) => `/api/v1/syncers/${id}/run`;
export const SYNCER_TEST_CONNECTION = (id: string) => `/api/v1/syncers/${id}/test-connection`;

export const CERT_BY_ID = (id: string) => `/api/v1/certs/${id}`;
export const CERT_CHECK_DOMAIN_EXPIRATION = (id: string) =>
  `/api/v1/certs/${encodeURIComponent(id)}/check-domain-expiration`;
export const CERT_UPDATE_DOMAIN_EXPIRE = (id: string) =>
  `/api/v1/certs/${encodeURIComponent(id)}/update-domain-expire`;

export const KEY_BY_ID = (id: string) => `/api/v1/keys/${id}`;

// Plans
export const PLANS = "/api/v1/plans";
export const PLAN_BY_ID = (id: string) => `/api/v1/plans/${id}`;

export const PRODUCT_BY_ID = (id: string) => `/api/v1/products/${id}`;

// Products
export const PRODUCTS = "/api/v1/products";
export const PRODUCT_BUY = (productId: string) => `/api/v1/products/${productId}/buy`;

// Pricings
export const PRICINGS = "/api/v1/pricings";
export const PRICING_BY_ID = (id: string) => `/api/v1/pricings/${id}`;

// ============================================================================
// Commerce (Orders / Payments / Coupons / Subscriptions / Tickets)
// ============================================================================
export const ORDERS_USER_ORDERS = "/api/v1/orders/user-orders";
export const ORDER_BY_ID = (id: string) => `/api/v1/orders/${id}`;
export const ORDER_BY_ENCODED_ID = (id: string) => `/api/v1/orders/${encodeURIComponent(id)}`;
export const ORDER_NOTIFY_PAID = (id: string) => `/api/v1/orders/${id}/notify-paid`;
export const ORDER_PAY_URL = (id: string) => `/api/v1/orders/${id}/pay-url`;
export const ORDER_PAY = (orderId: string) => `/api/v1/orders/${orderId}/pay`;
export const ORDER_CANCEL = (orderId: string) => `/api/v1/orders/${orderId}/cancel`;

export const PAYMENT_BY_ID = (id: string) => `/api/v1/payments/${id}`;
export const PAYMENT_BY_ENCODED_ID = (id: string) => `/api/v1/payments/${encodeURIComponent(id)}`;
export const PAYMENT_RESULT = (id: string) => `/api/v1/payments/${id}/result`;
export const PAYMENT_INVOICE = (id: string) => `/api/v1/payments/${encodeURIComponent(id)}/invoice`;

export const COUPON_BY_ID = (id: string) => `/api/v1/coupons/${encodeURIComponent(id)}`;
export const COUPONS_VALIDATE = "/api/v1/coupons/validate";
export const SUBSCRIPTION_BY_ID = (id: string) => `/api/v1/subscriptions/${id}`;

export const TICKET_BY_ID = (id: string) => `/api/v1/tickets/${id}`;
export const TICKET_MESSAGES = (ticketId: string) =>
  `/api/v1/tickets/${encodeURIComponent(ticketId)}/messages`;

export const TRANSACTION_BY_ID = (id: string) => `/api/v1/transactions/${id}`;

// ============================================================================
// Misc (Records / Notifications / Captcha / I18N / Rebac / MCP / Risk / IGA / Agent / Forms / Entries / Migration / Radius)
// ============================================================================
export const RECORDS = "/api/v1/records";
export const RECORDS_SEARCH = "/api/v1/records/search";
export const RECORDS_AUDIT_LOGS = "/api/v1/records/audit-logs";
export const RECORDS_LOGIN_LOGS = "/api/v1/records/login-logs";

export const VERIFICATIONS = "/api/v1/verifications";
export const VERIFICATION_BY_ID = (id: string) => `/api/v1/verifications/${encodeURIComponent(id)}`;

// ── IDV (Identity Verification) ──
export const IDV_VERIFICATIONS = "/api/v1/idv/verifications";
export const IDV_VERIFICATION_BY_ID = (id: string) =>
  `/api/v1/idv/verifications/${encodeURIComponent(id)}`;

export const I18N_TRANSLATE = "/api/v1/i18n/translate";
export const I18N_TRANSLATIONS = (language: string) => `/api/v1/i18n/translations/${language}`;

export const CAPTCHA_PROVIDER_BY_ID = (id: string) => `/api/v1/captcha-providers/${id}`;
export const NOTIFICATION_PROVIDER_BY_ID = (id: string) => `/api/v1/notification-providers/${id}`;
export const NOTIFICATION_MARK_READ = (id: string) => `/api/v1/notifications/${id}/mark-read`;
export const NOTIFICATION_BROADCAST = "/api/v1/notifications/broadcast";

// ── Personal notification inbox (/api/v1/me/**) ──
export const MY_NOTIFICATIONS = "/api/v1/me/notifications";
export const MY_NOTIFICATION_UNREAD_COUNT = "/api/v1/me/notifications/unread-count";
export const MY_NOTIFICATION_BY_ID = (id: string) =>
  `/api/v1/me/notifications/${encodeURIComponent(id)}`;
export const MY_NOTIFICATION_READ = (id: string) => `${MY_NOTIFICATION_BY_ID(id)}/read`;
export const MY_NOTIFICATION_UNREAD = (id: string) => `${MY_NOTIFICATION_BY_ID(id)}/unread`;
export const MY_NOTIFICATIONS_READ_ALL = "/api/v1/me/notifications/read-all";
export const MY_NOTIFICATIONS_BATCH_DELETE = "/api/v1/me/notifications/batch-delete";
export const MY_NOTIFICATION_REPLIES = (id: string) => `${MY_NOTIFICATION_BY_ID(id)}/replies`;
export const MY_NOTIFICATION_PREFERENCES = "/api/v1/me/notification-preferences";
export const MY_NOTIFICATION_CHANNEL_BINDINGS = "/api/v1/me/notification-channel-bindings";
export const MY_NOTIFICATION_CHANNEL_BINDING_BY_ID = (id: string) =>
  `/api/v1/me/notification-channel-bindings/${encodeURIComponent(id)}`;

export const AGENT_BY_ID = (id: string) => `/api/v1/agents/${id}`;
export const FORM_BY_ID = (id: string) => `/api/v1/forms/${id}`;
export const ENTRY_BY_ID = (id: string) => `/api/v1/entries/${id}`;
export const ENTRY_TRANSCRIPT = (id: string) => `/api/v1/entries/${id}/transcript`;
export const RESOURCE_BY_ID = (id: string) => `/api/v1/resources/${id}`;
export const RULE_BY_ID = (id: string) => `/api/v1/rules/${id}`;
export const WEBHOOK_EVENT_BY_ID = (id: string) => `/api/v1/webhook-events/${id}`;
export const WEBHOOK_EVENT_REPLAY = (eventId: string) => `/api/v1/webhook-events/${eventId}/replay`;
export const WEBHOOK_BY_ID = (id: string) => `/api/v1/webhooks/${id}`;
export const WEBHOOK_EVENTS = (webhookId: string) => `/api/v1/webhooks/${webhookId}/events`;

export const FACE_ID_PROVIDER_BY_ID = (id: string) => `/api/v1/face-id-providers/${id}`;

export const REBAC_NAMESPACE_BY_ID = (id: string) =>
  `/api/v1/rebac/namespaces/${encodeURIComponent(id)}`;
export const REBAC_NAMESPACE_TUPLES = (namespaceId: string) =>
  `/api/v1/rebac/namespaces/${encodeURIComponent(namespaceId)}/tuples`;
export const REBAC_NAMESPACE_CHECK = (namespaceId: string) =>
  `/api/v1/rebac/namespaces/${encodeURIComponent(namespaceId)}/check`;
export const REBAC_NAMESPACE_EXPAND = (namespaceId: string) =>
  `/api/v1/rebac/namespaces/${encodeURIComponent(namespaceId)}/expand`;
export const REBAC_TUPLE_BY_ID = (id: string) => `/api/v1/rebac/tuples/${encodeURIComponent(id)}`;

export const MCP_SERVER_BY_ID = (id: string) => `/api/v1/mcp-servers/${id}`;
export const MCP_SERVER_ACCESS_TOKEN = (id: string) =>
  `/api/v1/mcp-servers/${encodeURIComponent(id)}/access-token`;
export const MCP_SERVER_SYNC_TOOLS = (id: string) =>
  `/api/v1/mcp-servers/${encodeURIComponent(id)}/sync-tools`;
export const MCP_PROXY = (orgSlug: string, serverName: string) =>
  `/api/v1/mcp-servers/${encodeURIComponent(orgSlug)}/${encodeURIComponent(serverName)}/proxy`;
export const MCP_SERVERS_ONLINE = "/api/v1/mcp-servers/online";

export const IDENTITY_POOL_BY_ID = (id: string) =>
  `/api/v1/identity-pools/${encodeURIComponent(id)}`;
export const IDENTITY_POOL_FEDERATION_TRUSTS = (poolId: string) =>
  `/api/v1/identity-pools/${encodeURIComponent(poolId)}/federation-trusts`;
export const IDENTITY_POOL_FEDERATION_TRUST_MATCH_SUBJECT = (poolId: string) =>
  `${IDENTITY_POOL_FEDERATION_TRUSTS(poolId)}:match-subject`;
export const IDENTITY_POOL_FEDERATION_TRUST_BY_ID = (poolId: string, id: string) =>
  `${IDENTITY_POOL_FEDERATION_TRUSTS(poolId)}/${encodeURIComponent(id)}`;
export const IDENTITY_POOL_FEDERATION_TRUST_STATUS = (poolId: string, id: string) =>
  `${IDENTITY_POOL_FEDERATION_TRUST_BY_ID(poolId, id)}/status`;
export const IDENTITY_POOL_SERVICE_ACCOUNTS = (poolId: string) =>
  `/api/v1/identity-pools/${encodeURIComponent(poolId)}/service-accounts`;
export const IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID = (poolId: string, id: string) =>
  `${IDENTITY_POOL_SERVICE_ACCOUNTS(poolId)}/${encodeURIComponent(id)}`;
export const IDENTITY_POOL_SERVICE_ACCOUNT_STATUS = (poolId: string, id: string) =>
  `${IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID(poolId, id)}/status`;
export const IDENTITY_POOL_SERVICE_ACCOUNT_ROTATE_SECRET = (poolId: string, id: string) =>
  `${IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID(poolId, id)}/rotate-secret`;
export const IDENTITY_POOL_SERVICE_ACCOUNT_DELETE_PRECHECK = (poolId: string, id: string) =>
  `${IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID(poolId, id)}/delete-precheck`;

export const RISK_DECISION_REVIEW = (id: string) =>
  `/api/v1/risk/decisions/${encodeURIComponent(id)}/review`;

export const IGA_ACCESS_CAMPAIGN = (id: string) =>
  `/api/v1/iga/access-campaigns/${encodeURIComponent(id)}`;
export const IGA_SOD_RULE = (id: string) => `/api/v1/iga/sod-rules/${encodeURIComponent(id)}`;

export const RADIUS_CONFIG_BY_ID = (id: string) => `/api/v1/radius-configs/${id}`;
export const RADIUS_ACCOUNTING_BY_ID = (id: string) => `/api/v1/radius-accountings/${id}`;

// ============================================================================
// Batch: 一次性补齐所有遗漏的静态根路径常量 (2026-10-12)
// 来源：全 web/src 硬编码 /api/v1/ 路径审计（排除 .test.ts / generated / e2e）
// ============================================================================
export const ACTIONS = "/api/v1/actions";
export const ADAPTERS = "/api/v1/adapters";
export const AGENTS = "/api/v1/agents";
export const ANALYTICS_WEB_VITALS = "/api/v1/analytics/web-vitals";
export const APPLICATION_IMPORT = "/api/v1/applications/import";
export const APPLICATION_NAMES = "/api/v1/applications/names";
export const APPLICATION_USER_APPLICATION = "/api/v1/applications/user-application";
export const AUTH_CONSENTS_GRANT = "/api/v1/auth/consents/grant";
export const AUTH_DEVICE_AUTHORIZE = "/api/v1/auth/device/authorize";
export const AUTH_DEVICE_CANCEL = "/api/v1/auth/device/cancel";
export const AUTH_DEVICE_TOKEN = "/api/v1/auth/device/token";
export const AUTH_DEVICE_VERIFY = "/api/v1/auth/device/verify";
export const AUTH_FACEID = "/api/v1/auth/faceid";
export const AUTH_FACEID_COMPLETE = "/api/v1/auth/faceid/complete";
export const AUTH_FACEID_VERIFY = "/api/v1/auth/faceid/verify";
export const AUTH_MFA_PUSH_DISABLE = "/api/v1/auth/mfa/push/disable";
export const AUTH_MFA_PUSH_ENABLE = "/api/v1/auth/mfa/push/enable";
export const AUTH_MFA_PUSH_INITIATE = "/api/v1/auth/mfa/push/initiate";
export const AUTH_MFA_PUSH_VERIFY = "/api/v1/auth/mfa/push/verify";
export const AUTH_PASSWORD_CHECK = "/api/v1/auth/password/check";
export const AUTH_PASSWORD_RESET = "/api/v1/auth/password/reset";
export const AUTH_PERMISSIONS = "/api/v1/auth/permissions";
export const AUTH_QRCODE = "/api/v1/auth/qrcode";
export const AUTH_QRCODE_STATUS = (codeId: string) =>
  `/api/v1/auth/qrcode/${encodeURIComponent(codeId)}/status`;
export const AUTH_QRCODE_CONFIRM = "/api/v1/auth/qrcode/confirm";
export const AUTH_QRCODE_EXCHANGE = "/api/v1/auth/qrcode/exchange";
export const AUTH_QRCODE_SCAN = "/api/v1/auth/qrcode/scan";
export const AUTH_SIGNUP = "/api/v1/auth/signup";
export const AUTH_VERIFICATION_CODE_SEND = "/api/v1/auth/verification-code/send";
export const AUTH_VERIFICATION_CODE_VERIFY = "/api/v1/auth/verification-code/verify";
export const AUTH_WEB3 = "/api/v1/auth/web3";
export const AUTH_WEB3_VERIFY = "/api/v1/auth/web3/verify";
export const AUTH_WEBAUTHN_AUTH_BEGIN = "/api/v1/auth/webauthn/authentication/begin";
export const AUTH_WEBAUTHN_AUTH_FINISH = "/api/v1/auth/webauthn/authentication/finish";
export const AUTH_WEBAUTHN_REG_BEGIN = "/api/v1/auth/webauthn/registration/begin";
export const AUTH_WEBAUTHN_REG_FINISH = "/api/v1/auth/webauthn/registration/finish";
export const CAPTCHA_PROVIDERS = "/api/v1/captcha-providers";
export const CERTS = "/api/v1/certs";
export const COUPONS = "/api/v1/coupons";
export const DASHBOARD_ACTIVITY_HEATMAP = "/api/v1/dashboard/activity-heatmap";
export const DASHBOARD_AUTH = "/api/v1/dashboard/auth";
export const DASHBOARD_LOGIN_HEATMAP = "/api/v1/dashboard/login-heatmap";
export const DASHBOARD_MFA_COVERAGE = "/api/v1/dashboard/mfa-coverage";
export const DASHBOARD_ORG_USER_DISTRIBUTION = "/api/v1/dashboard/org-user-distribution";
export const DASHBOARD_PROVIDER_DISTRIBUTION = "/api/v1/dashboard/provider-distribution";
export const DASHBOARD_RESOURCES = "/api/v1/dashboard/resources";
export const DASHBOARD_STATS = "/api/v1/dashboard/stats";
export const DASHBOARD_TRENDS = "/api/v1/dashboard/trends";
export const DASHBOARD_USERS = "/api/v1/dashboard/users";
export const ENFORCERS = "/api/v1/enforcers";
export const ENFORCER_REFRESH_ENGINES = "/api/v1/enforcers/-/refresh-engines";
export const ENTRIES = "/api/v1/entries";
export const FACE_ID_PROVIDERS = "/api/v1/face-id-providers";
export const FORMS = "/api/v1/forms";
export const I18N_LANGUAGES = "/api/v1/i18n/languages";
export const IDENTITY_POOLS = "/api/v1/identity-pools";
export const IGA_ACCESS_CAMPAIGNS = "/api/v1/iga/access-campaigns";
export const IGA_ACCESS_LIFECYCLE = "/api/v1/iga/access-lifecycle";
export const IGA_ACCESS_REQUESTS = "/api/v1/iga/access-requests";
export const IGA_ACCESS_REQUEST_BY_ID = (id: string) =>
  `${IGA_ACCESS_REQUESTS}/${encodeURIComponent(id)}`;
export const IGA_ACCESS_REQUEST_SUBMIT = (id: string) => `${IGA_ACCESS_REQUEST_BY_ID(id)}/submit`;
export const IGA_ACCESS_REQUEST_APPROVE = (id: string) => `${IGA_ACCESS_REQUEST_BY_ID(id)}/approve`;
export const IGA_ACCESS_REQUEST_REJECT = (id: string) => `${IGA_ACCESS_REQUEST_BY_ID(id)}/reject`;
export const IGA_ACCESS_REQUEST_WITHDRAW = (id: string) =>
  `${IGA_ACCESS_REQUEST_BY_ID(id)}/withdraw`;
export const IGA_ACCESS_REQUEST_PROVISION = (id: string) =>
  `${IGA_ACCESS_REQUEST_BY_ID(id)}/provision`;
export const IGA_PRIVILEGED_SESSIONS = "/api/v1/iga/privileged-sessions";
export const IGA_SOD_RULES = "/api/v1/iga/sod-rules";
export const KEYS = "/api/v1/keys";
export const LDAPS = "/api/v1/ldaps";
export const MCP_SERVERS = "/api/v1/mcp-servers";
export const MCP_SERVER_SYNC_INTRANET = "/api/v1/mcp-servers/sync-intranet";
export const METERING_AGGREGATES = "/api/v1/metering/aggregates";
export const METERING_AGGREGATES_REFRESH = "/api/v1/metering/aggregates/refresh";
export const METERING_EVENTS = "/api/v1/metering/events";
export const METERING_EVENTS_INGEST = "/api/v1/metering/events/ingest";
export const MODELS = "/api/v1/models";
export const NOTIFICATION_PROVIDERS = "/api/v1/notification-providers";
export const NOTIFICATIONS = "/api/v1/notifications";
export const NOTIFICATIONS_MARK_ALL_READ = "/api/v1/notifications/mark-all-read";
export const ORDERS = "/api/v1/orders";
export const ORDER_PLACE = "/api/v1/orders/place";
export const PAYMENTS = "/api/v1/payments";
export const PERMISSIONS_BATCH_ENFORCE = "/api/v1/permissions/batch-enforce";
export const PERMISSIONS_ENFORCE = "/api/v1/permissions/enforce";
export const RADIUS_ACCOUNTINGS = "/api/v1/radius-accountings";
export const RADIUS_CONFIGS = "/api/v1/radius-configs";
export const REBAC_NAMESPACES = "/api/v1/rebac/namespaces";
export const RESOURCES = "/api/v1/resources";
export const RISK_DECISIONS = "/api/v1/risk/decisions";
export const RISK_SESSION_GOVERNANCE = "/api/v1/risk/session-governance";
export const RISK_SIGNALS = "/api/v1/risk/signals";
export const RULES = "/api/v1/rules";
export const RULE_VALIDATE = "/api/v1/rules/validate";
export const SUBSCRIPTIONS = "/api/v1/subscriptions";
export const SYNCERS = "/api/v1/syncers";
export const SYSTEM_HEALTH = "/api/v1/system/health";
export const SYSTEM_INFO = "/api/v1/system/info";
export const SYSTEM_METRICS = "/api/v1/system/metrics";
export const SYSTEM_VERSION = "/api/v1/system/version";
export const TICKETS = "/api/v1/tickets";
export const TRANSACTIONS = "/api/v1/transactions";
export const WEBHOOKS = "/api/v1/webhooks";
export const USER_IMPORT = "/api/v1/users/import";
export const USER_TRUSTED_DEVICES = (userId: string) => `/api/v1/users/${userId}/trusted-devices`;
export const USER_TRUSTED_DEVICE_BY_ID = (userId: string, deviceId: string) =>
  `${USER_TRUSTED_DEVICES(userId)}/${deviceId}`;
export const UPLOAD_RESOURCE = "/api/v1/upload-resource";
