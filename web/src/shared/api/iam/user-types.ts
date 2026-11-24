// ─────────────────────────────────────────────
// User Types & Schemas
// ─────────────────────────────────────────────

import { z } from "zod";
import { pageMetaSchema, flexibleObjectSchema, flexibleObjectArraySchema } from "./schemas";

// ── Basic Types ──

/** Represents a linked identity provider entry for a user. */
export type UserIdentity = {
  id?: string;
  providerId: string;
  identityType: string;
  subject: string;
  providerAccount?: string;
  linkedAt?: string | null;
  profileSnapshot?: Record<string, unknown>;
};

/** Represents a MFA method entry for a user. */
export type UserMfaMethod = {
  methodType: string;
  enabled: boolean;
  verified: boolean;
  displayName?: string;
  lastUsedAt?: string | null;
};

/** Represents a role binding entry for a user. */
export type UserRoleBinding = {
  roleId: string;
  roleName?: string;
  id?: string;
  name?: string;
};

/** Represents a group membership entry for a user. */
export type UserGroupMembership = {
  groupId: string;
  groupName?: string;
  id?: string;
  name?: string;
};

export type UserOption = {
  id: string;
  username: string;
  displayName: string;
};

export type PromptUser = {
  id: string;
  username: string;
  profile: Record<string, unknown>;
  linkedProviders: Record<string, string>;
};

export type CreateUserPayload = {
  organizationId: string;
  username: string;
  externalId?: string;
  userType: string;
  status: string;
  isAdmin: boolean;
  isForbidden?: boolean;
  isDeleted?: boolean;
  needUpdatePassword?: boolean;
  ipWhitelist?: string;
  tag?: string;
  score?: number;
  karma?: number;
  ranking?: number;
  balance?: number;
  balanceCredit?: number;
  balanceCurrency?: string;
  signupApplicationId?: string;
  password?: string;
  profile: {
    displayName?: string;
    firstName?: string;
    lastName?: string;
    avatarUrl?: string;
    email?: string;
    phone?: string;
    countryCode?: string;
    region?: string;
    location?: string;
    gender?: string;
    birthday?: string;
    bio?: string;
    homepage?: string;
    education?: string;
    language?: string;
    realName?: string;
    idCardType?: string;
    idCard?: string;
    affiliation?: string;
    title?: string;
  };
  roleBindingIds?: string[];
  groupIds?: string[];
};

export type CreateUserResult = {
  id: string;
  status: string;
};

export type UserDetail = {
  id: string;
  organizationId: string;
  username: string;
  displayName: string;
  avatarUrl: string;
  email: string;
  phone: string;
  /** 只读验证标记（后端 profile.emailVerified / profile.phoneVerified 的顶层投影）。 */
  emailVerified?: boolean;
  phoneVerified?: boolean;
  userType: string;
  status: string;
  isAdmin: boolean;
  isForbidden: boolean;
  isDeleted: boolean;
  isVerified: boolean;
  tag: string;
  score: number;
  karma: number;
  ranking: number;
  balance: number;
  lastSigninAt?: string | null;
  createdAt: string;
  externalId: string;
  signupApplicationId: string;
  registerType: string;
  registerSource: string;
  preferredMfaType: string;
  recoveryCodes: string[];
  mfaSetupSummary: Record<string, unknown>[];
  profile: Record<string, unknown>;
  credentialSummary: Record<string, unknown>;
  identities: UserIdentity[];
  mfaMethods: UserMfaMethod[];
  webauthnCredentialCount: number;
  roleBindings: UserRoleBinding[];
  groupMemberships: UserGroupMembership[];
  consentSummary: Record<string, unknown>;
  balanceCredit: number;
  balanceCurrency: string;
  ipWhitelist: string;
  addresses: AddressItem[];
  managedAccounts: ManagedAccountItem[];
  faceIds: FaceIdItem[];
  cart: ProductInfoItem[];
  mfaItems: MfaItemEntry[];
  mfaAccounts: MfaAccountItem[];
  applicationScopes: ConsentRecordItem[];
  custom1: string;
  custom2: string;
  custom3: string;
  custom4: string;
  custom5: string;
  custom6: string;
  custom7: string;
  custom8: string;
  custom9: string;
  custom10: string;
  revision: number;
};

export type AddressItem = {
  id?: string;
  countryCode?: string;
  province?: string;
  city?: string;
  district?: string;
  postalCode?: string;
  street: string;
  details?: string;
  /** Legacy free-text region kept for backward compatibility with old rows. */
  region?: string;
  isDefault?: boolean;
};

/** Full replacement payload for `PATCH /users/{id}/collections`.
 * mfaItems / mfaAccounts are deliberately absent: the server derives them from
 * the real MFA factor state and ignores them on the write path. */
export type UserCollectionsPayload = {
  addresses: AddressItem[];
  managedAccounts: ManagedAccountItem[];
  faceIds: FaceIdItem[];
  cart: ProductInfoItem[];
  applicationScopes: ConsentRecordItem[];
};

export type ContactVerificationChannel = "email" | "phone";

export type ContactVerificationTicket = {
  channel: ContactVerificationChannel;
  recipientMasked: string;
  expiresAt: string;
};

export type IdentityDocumentType = "id_card" | "passport" | "driver_license";

/** Tenant-defined custom attribute. Definitions drive the profile editor
 * suggestions; unknown keys are still allowed (key-value fallback). */
export type CustomAttributeDefinition = {
  id: string;
  key: string;
  dataType: string;
  required: boolean;
  options: string[];
  writable: boolean;
  visibility: string;
  description?: string;
  enabled: boolean;
  revision: number;
};

export type IdentityDocumentFields = {
  fullName?: string;
  idNumber?: string;
  idType?: string;
  gender?: string;
  birthday?: string;
  nationality?: string;
  address?: string;
  issuingAuthority?: string;
  validFrom?: string;
  validTo?: string;
  confidence?: number;
};

export type ManagedAccountItem = {
  application: string;
  username: string;
  password?: string;
  passwordSet: boolean;
  passwordMasked: string;
  signinUrl: string;
};

export type FaceIdItem = {
  id: string;
  imageUrl: string;
  createdAt: string;
  /** 面部识别提供商（可选，由后端填充）。 */
  provider?: string;
};

export type ProductInfoItem = {
  productId: string;
  quantity: number;
  price: number;
};

export type MfaItemEntry = {
  mfaType: string;
  secret?: string;
  secretSet: boolean;
  secretMasked: string;
};

export type MfaAccountItem = {
  accountName: string;
  issuer: string;
  secretKey?: string;
  secretKeySet: boolean;
  secretKeyMasked: string;
  origin: string;
};

export type ConsentRecordItem = {
  application: string;
  scopes: string[];
  grantedAt: string;
};

export type PatchUserPayload = {
  externalId?: string;
  userType?: string;
  status?: string;
  isAdmin?: boolean;
  isForbidden?: boolean;
  isDeleted?: boolean;
  tag?: string;
  score?: number;
  karma?: number;
  ranking?: number;
  balance?: number;
  balanceCredit?: number;
  balanceCurrency?: string;
  ipWhitelist?: string;
  language?: string;
  title?: string;

  custom1?: string;
  custom2?: string;
  custom3?: string;
  custom4?: string;
  custom5?: string;
  custom6?: string;
  custom7?: string;
  custom8?: string;
  custom9?: string;
  custom10?: string;
  revision: number;
};

export type UpdateUserProfilePayload = {
  displayName?: string;
  firstName?: string;
  lastName?: string;
  avatarUrl?: string;
  email?: string;
  phone?: string;
  countryCode?: string;
  region?: string;
  location?: string;
  birthday?: string;
  gender?: string;
  bio?: string;
  homepage?: string;
  education?: string;
  language?: string;
  realName?: string;
  idCardType?: string;
  idCard?: string;
  affiliation?: string;
  title?: string;
  properties?: Record<string, unknown>;
};

export type UpdateUserCredentialsPayload = {
  password?: string;
  oldPassword?: string;
  rotateAccessSecret?: boolean;
  needUpdatePassword?: boolean;
};

export type ReplaceUserIdentityItem = {
  id?: string;
  providerId: string;
  identityType: string;
  subject: string;
  providerAccount?: string;
  profileSnapshot?: Record<string, unknown>;
};

export type UpdatedUserCredentialsResult = {
  status: "updated";
  accessKey?: string;
  accessSecret?: string;
};

export type ReplaceUserMfaMethodItem = {
  id?: string;
  methodType: string;
  enabled: boolean;
  displayName?: string;
};

export type UpdateUserPreferredMfaPayload = {
  mfaType: string;
  revision?: number;
};

export type InitiateUserMfaSetupPayload = {
  mfaType: string;
};

export type VerifyUserMfaSetupPayload = {
  mfaType: string;
  passcode: string;
  secret?: string;
  dest?: string;
  countryCode?: string;
};

export type EnableUserMfaSetupPayload = {
  mfaType: string;
  secret?: string;
  dest?: string;
  countryCode?: string;
  /** Proof of possession re-checked server-side at enable; required for TOTP. */
  passcode?: string;
  recoveryCodes: string[];
};

export type UserMfaSetupInitiateResult = {
  secret?: string;
  url?: string;
  recoveryCodes?: string[];
  countryCode?: string;
  mfaRememberInHours?: number;
};

export type UserListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  status?: string;
  userType?: string;
  sortField?: string;
  sortOrder?: "asc" | "desc";
  searchField?: string;
  searchValue?: string;
};

/** Credential lifecycle states returned by the backend (never `disabled`). */
export type WebAuthnCredentialStatus = "active" | "revoked" | "deactivated";

export type WebAuthnCredentialItem = {
  id: string;
  /** base64url authenticator credentialId — the only valid URL path parameter. */
  credentialId: string;
  name: string;
  attestationType: string;
  transport: string[];
  signCount: number;
  aaguid: string;
  status: WebAuthnCredentialStatus;
  createdAt: string;
  updatedAt: string;
};

/** PublicKeyCredentialCreationOptions material for admin on-behalf registration. */
export type AdminWebAuthnRegistrationBeginResult = {
  challenge: string;
  challengeId: string;
  rpId: string;
  rpName: string;
  userId: string;
  userDisplayName: string;
  timeout: number;
  /** 该用户已有活跃凭据的 base64url credentialId。 */
  excludeCredentials?: string[];
  /** 服务端下发的 UV 策略，注册入口不得写死。 */
  userVerification: "required" | "preferred";
};

/**
 * Finish payload for admin on-behalf registration.
 * publicKey / aaguid / signCount / authenticatorData are intentionally absent:
 * the server derives them from `attestationObject`.
 */
export type WebAuthnRegistrationFinishPayload = {
  challengeId: string;
  challenge: string;
  credentialId?: string;
  clientDataJSON: string;
  attestationObject: string;
  transports?: string[];
  name?: string;
};

export type RenameWebAuthnCredentialPayload = {
  name: string;
};

export type UserTransactionItem = {
  id: string;
  transactionType: string;
  amount: number;
  currency: string;
  status: string;
  providerType?: string;
  createdAt: string;
};

export type UserConsentItem = {
  id: string;
  applicationId: string;
  applicationSlug?: string;
  grantedScopes: string[];
  createdAt: string;
  revokedAt?: string;
};

export type ImportUserItem = {
  username: string;
  displayName?: string;
  email?: string;
  phone?: string;
  password?: string;
  organizationId?: string;
};

export type BulkImportResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors?: string[];
};

export type PreviewResult = {
  totalRows: number;
  columns: string[];
  preview: Record<string, unknown>[];
};

export type UserOrderItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  user?: string;
  price: number;
  currency?: string;
  state: string;
  createdAt: string;
  updatedAt: string;
};

export type UserPaymentItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  provider?: string;
  type?: string;
  state: string;
  price: number;
  currency?: string;
  createdAt: string;
  updatedAt: string;
};

// ── Zod Schemas ──

export const userListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  username: z.string(),
  displayName: z.string(),
  avatarUrl: z.string(),
  email: z.string(),
  phone: z.string(),
  userType: z.string(),
  status: z.string(),
  isAdmin: z.boolean(),
  isForbidden: z.boolean(),
  isDeleted: z.boolean().default(false),
  isVerified: z.boolean().default(false),
  tag: z.string().default(""),
  affiliation: z.string().default(""),
  realName: z.string().default(""),
  signupApplicationId: z.string().default(""),
  registerType: z.string().default(""),
  registerSource: z.string().default(""),
  score: z.number().default(0),
  karma: z.number().default(0),
  ranking: z.number().default(0),
  balance: z.number().default(0),
  balanceCredit: z.number().default(0),
  balanceCurrency: z.string().default(""),
  lastSigninAt: z.string().nullable().optional(),
  createdAt: z.string(),
  revision: z.number().optional().default(0),
});

export const userListResponseSchema = z.object({
  items: z.array(userListItemSchema),
  page: pageMetaSchema,
});

export const userDetailSchema = userListItemSchema.extend({
  externalId: z.string(),
  signupApplicationId: z.string().default(""),
  registerType: z.string().default(""),
  registerSource: z.string().default(""),
  preferredMfaType: z.string(),
  recoveryCodes: z.array(z.string()).catch([]),
  mfaSetupSummary: flexibleObjectArraySchema,
  profile: flexibleObjectSchema,
  credentialSummary: flexibleObjectSchema,
  identities: flexibleObjectArraySchema,
  mfaMethods: flexibleObjectArraySchema,
  webauthnCredentialCount: z.number(),
  roleBindings: flexibleObjectArraySchema,
  groupMemberships: flexibleObjectArraySchema,
  consentSummary: flexibleObjectSchema,
  balanceCredit: z.number().default(0),
  balanceCurrency: z.string().default("USD"),
  ipWhitelist: z.string().default(""),
  addresses: z
    .array(
      z.object({
        id: z.string().optional(),
        countryCode: z.string().optional(),
        province: z.string().optional(),
        city: z.string().optional(),
        district: z.string().optional(),
        postalCode: z.string().optional(),
        street: z.string().default(""),
        details: z.string().optional(),
        // Legacy free-text region; null for rows created by the hierarchy editor.
        region: z.string().optional(),
        isDefault: z.boolean().optional(),
      }),
    )
    .catch([]),
  managedAccounts: z
    .array(
      z.object({
        application: z.string(),
        username: z.string(),
        password: z.string().optional(),
        passwordSet: z.boolean().default(false),
        passwordMasked: z.string().default(""),
        signinUrl: z.string(),
      }),
    )
    .catch([]),
  faceIds: z
    .array(z.object({ id: z.string(), imageUrl: z.string(), createdAt: z.string() }))
    .catch([]),
  cart: z
    .array(z.object({ productId: z.string(), quantity: z.number(), price: z.number() }))
    .catch([]),
  mfaItems: z
    .array(
      z.object({
        mfaType: z.string(),
        secret: z.string().optional(),
        secretSet: z.boolean().default(false),
        secretMasked: z.string().default(""),
      }),
    )
    .catch([]),
  mfaAccounts: z
    .array(
      z.object({
        accountName: z.string(),
        issuer: z.string(),
        secretKey: z.string().optional(),
        secretKeySet: z.boolean().default(false),
        secretKeyMasked: z.string().default(""),
        origin: z.string(),
      }),
    )
    .catch([]),
  applicationScopes: z
    .array(
      z.object({ application: z.string(), scopes: z.array(z.string()), grantedAt: z.string() }),
    )
    .catch([]),
  custom1: z.string().default(""),
  custom2: z.string().default(""),
  custom3: z.string().default(""),
  custom4: z.string().default(""),
  custom5: z.string().default(""),
  custom6: z.string().default(""),
  custom7: z.string().default(""),
  custom8: z.string().default(""),
  custom9: z.string().default(""),
  custom10: z.string().default(""),
  revision: z.number(),
});

export const createUserResultSchema = z.object({
  id: z.string(),
  status: z.string(),
});

export const updatedUserCredentialsResultSchema = z.object({
  status: z.literal("updated"),
  accessKey: z.string().optional(),
  accessSecret: z.string().optional(),
});

export const userMfaSetupInitiateResultSchema = z.object({
  secret: z.string().optional(),
  url: z.string().optional(),
  recoveryCodes: z.array(z.string()).optional(),
  countryCode: z.string().optional(),
  mfaRememberInHours: z.number().optional(),
});

export type IdentityVerificationTicket = {
  status: string;
  msg: string;
  data?: {
    verificationId?: string;
    provider?: string;
    redirectUrl?: string;
    expiresAt?: string;
  };
};

// 发起实名认证的请求体：metaInfo 必须是 CloudAuth SDK 实时采集的设备指纹
// （后端对空值 fail-closed），returnUrl 为纯 origin+path 回跳地址。
export type VerifyIdentificationPayload = {
  metaInfo: string;
  returnUrl: string;
  country?: string;
  action?: string;
};

/** One resource/action grant expanded from a bound role's Casbin policies. */
export type UserEffectivePermission = {
  resource: string;
  action: string;
  role: string;
};

export const userEffectivePermissionsSchema = z.object({
  items: z.array(
    z.object({
      resource: z.string(),
      action: z.string(),
      role: z.string(),
    }),
  ),
});

export type UserListItem = z.infer<typeof userListItemSchema>;
