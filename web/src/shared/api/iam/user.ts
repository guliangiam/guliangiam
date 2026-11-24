import { z } from "zod";
import { HttpError, requestDocumentJson, requestBlob } from "@/shared/api/http";
import {
  AUTH_EXIT_IMPERSONATE,
  ORGANIZATION_BY_SLUG,
  USERS,
  USER_BY_ID,
  USER_CONSENTS,
  USER_CONSENT_BY_ID,
  USER_COUNT,
  USER_CREDENTIALS,
  USER_EXPORT,
  USER_GROUP_BY_NAME,
  USER_GROUP_MEMBERSHIPS,
  USER_IDENTITIES,
  USER_IMPERSONATE,
  USER_IMPORT,
  USER_MASKED_CONTACT,
  USER_MFA_METHODS,
  USER_MFA_SETUP_ENABLE,
  USER_MFA_SETUP_INITIATE,
  USER_MFA_SETUP_VERIFY,
  USER_ORDERS,
  USER_PAYMENTS,
  USER_PREFERRED_MFA,
  USER_PROFILE,
  USER_COLLECTIONS,
  USER_CONTACT_VERIFICATION_SEND,
  USER_CONTACT_VERIFICATION_VERIFY,
  USER_EFFECTIVE_PERMISSIONS,
  USER_IDENTITY_DOCUMENT_RECOGNIZE,
  USER_FACEIDS,
  USER_CUSTOM_ATTRIBUTES,
  USER_REMOVE_FROM_GROUP,
  USER_ROLE_BINDINGS,
  USER_ROLE_BINDING_BY_ROLE,
  USER_SET_PASSWORD,
  USER_TRANSACTIONS,
  USER_TRUSTED_DEVICES,
  USER_TRUSTED_DEVICE_BY_ID,
  USER_UNLINK,
  USER_UPLOAD,
  USER_UPLOAD_PREVIEW,
  USER_VERIFY_IDENTIFICATION,
  USER_WEBAUTHN_CREDENTIALS,
  USER_WEBAUTHN_CREDENTIAL_BY_ID,
  USER_WEBAUTHN_CREDENTIAL_REVOKE,
  USER_WEBAUTHN_CREDENTIAL_DEACTIVATE,
  USER_WEBAUTHN_CREDENTIAL_RESTORE,
  USER_WEBAUTHN_CREDENTIAL_RENAME,
  USER_WEBAUTHN_CREDENTIAL_REGISTRATION_BEGIN,
  USER_WEBAUTHN_CREDENTIAL_REGISTRATION_FINISH,
} from "@/shared/api/paths";
import { captureError } from "@/shared/lib/sentry";
import type { OrganizationIdentity } from "./schemas";
import {
  userListItemSchema,
  userListResponseSchema,
  userDetailSchema,
  createUserResultSchema,
  updatedUserCredentialsResultSchema,
  userMfaSetupInitiateResultSchema,
  userEffectivePermissionsSchema,
  type PromptUser,
  type UserDetail,
  type CreateUserPayload,
  type CreateUserResult,
  type FaceIdItem,
  type PatchUserPayload,
  type UpdateUserProfilePayload,
  type UpdateUserCredentialsPayload,
  type ReplaceUserIdentityItem,
  type ReplaceUserMfaMethodItem,
  type UpdateUserPreferredMfaPayload,
  type InitiateUserMfaSetupPayload,
  type VerifyUserMfaSetupPayload,
  type EnableUserMfaSetupPayload,
  type UserMfaSetupInitiateResult,
  type UpdatedUserCredentialsResult,
  type UserListParams,
  type WebAuthnCredentialItem,
  type AdminWebAuthnRegistrationBeginResult,
  type WebAuthnRegistrationFinishPayload,
  type UserTransactionItem,
  type UserConsentItem,
  type ImportUserItem,
  type BulkImportResult,
  type PreviewResult,
  type UserOrderItem,
  type UserPaymentItem,
  type UserIdentity,
  type UserMfaMethod,
  type UserRoleBinding,
  type UserGroupMembership,
  type IdentityVerificationTicket,
  type VerifyIdentificationPayload,
  type UserCollectionsPayload,
  type ContactVerificationChannel,
  type ContactVerificationTicket,
  type IdentityDocumentType,
  type IdentityDocumentFields,
  type CustomAttributeDefinition,
  type UserEffectivePermission,
} from "./user-types";

// Re-export all types for backward compatibility
export type {
  UserOption,
  PromptUser,
  UserDetail,
  CreateUserPayload,
  CreateUserResult,
  PatchUserPayload,
  UpdateUserProfilePayload,
  UpdateUserCredentialsPayload,
  ReplaceUserIdentityItem,
  ReplaceUserMfaMethodItem,
  UpdateUserPreferredMfaPayload,
  InitiateUserMfaSetupPayload,
  VerifyUserMfaSetupPayload,
  EnableUserMfaSetupPayload,
  UserMfaSetupInitiateResult,
  UpdatedUserCredentialsResult,
  UserListParams,
  UserListItem,
  WebAuthnCredentialItem,
  WebAuthnCredentialStatus,
  AdminWebAuthnRegistrationBeginResult,
  WebAuthnRegistrationFinishPayload,
  RenameWebAuthnCredentialPayload,
  UserTransactionItem,
  UserConsentItem,
  ImportUserItem,
  BulkImportResult,
  PreviewResult,
  UserOrderItem,
  UserPaymentItem,
  AddressItem,
  ManagedAccountItem,
  FaceIdItem,
  ProductInfoItem,
  MfaItemEntry,
  MfaAccountItem,
  ConsentRecordItem,
  UserIdentity,
  UserMfaMethod,
  UserRoleBinding,
  UserGroupMembership,
  UserCollectionsPayload,
  ContactVerificationChannel,
  ContactVerificationTicket,
  IdentityDocumentType,
  IdentityDocumentFields,
  CustomAttributeDefinition,
  UserEffectivePermission,
} from "./user-types";

// ─────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────

function toUserItem(item: z.infer<typeof userListItemSchema>) {
  // Keep the stored user tag: status/userType/isAdmin/isForbidden already have
  // dedicated columns, so the list must not overwrite it with a derived label.
  return { ...item };
}

export function toUserDetail(item: z.infer<typeof userDetailSchema>): UserDetail {
  return {
    id: item.id,
    organizationId: item.organizationId,
    username: item.username,
    displayName: item.displayName,
    avatarUrl: item.avatarUrl,
    email: item.email,
    phone: item.phone,
    emailVerified:
      typeof item.profile.emailVerified === "boolean" ? item.profile.emailVerified : undefined,
    phoneVerified:
      typeof item.profile.phoneVerified === "boolean" ? item.profile.phoneVerified : undefined,
    userType: item.userType,
    status: item.status,
    isAdmin: item.isAdmin,
    isForbidden: item.isForbidden,
    isDeleted: item.isDeleted,
    isVerified: item.isVerified,
    tag: item.tag,
    score: item.score,
    karma: item.karma,
    ranking: item.ranking,
    balance: item.balance,
    lastSigninAt: item.lastSigninAt ?? null,
    createdAt: item.createdAt,
    externalId: item.externalId,
    signupApplicationId: item.signupApplicationId,
    registerType: item.registerType,
    registerSource: item.registerSource,
    preferredMfaType: item.preferredMfaType,
    recoveryCodes: item.recoveryCodes,
    mfaSetupSummary: item.mfaSetupSummary,
    profile: item.profile,
    credentialSummary: item.credentialSummary,
    identities: (item.identities ?? []) as UserIdentity[],
    mfaMethods: (item.mfaMethods ?? []) as UserMfaMethod[],
    webauthnCredentialCount: item.webauthnCredentialCount,
    roleBindings: (item.roleBindings ?? []) as UserRoleBinding[],
    groupMemberships: (item.groupMemberships ?? []) as UserGroupMembership[],
    consentSummary: item.consentSummary,
    balanceCredit: item.balanceCredit,
    balanceCurrency: item.balanceCurrency,
    ipWhitelist: item.ipWhitelist,
    addresses: item.addresses,
    managedAccounts: item.managedAccounts,
    faceIds: item.faceIds,
    cart: item.cart,
    mfaItems: item.mfaItems,
    mfaAccounts: item.mfaAccounts,
    applicationScopes: item.applicationScopes,
    custom1: item.custom1,
    custom2: item.custom2,
    custom3: item.custom3,
    custom4: item.custom4,
    custom5: item.custom5,
    custom6: item.custom6,
    custom7: item.custom7,
    custom8: item.custom8,
    custom9: item.custom9,
    custom10: item.custom10,
    revision: item.revision,
  };
}

// ─────────────────────────────────────────────
// API Functions
// ─────────────────────────────────────────────

export async function getUsers(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }

  const response = await requestDocumentJson(`${USERS}?${query.toString()}`);
  return userListResponseSchema.parse(response).items.map(toUserItem);
}

export async function getUsersPaginated(params?: UserListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  if (params?.status) query.set("status", params.status);
  if (params?.userType) query.set("userType", params.userType);
  if (params?.sortField) query.set("sortField", params.sortField);
  if (params?.sortOrder) query.set("sortOrder", params.sortOrder);
  if (params?.searchField) query.set("searchField", params.searchField);
  if (params?.searchValue) query.set("searchValue", params.searchValue);

  const response = await requestDocumentJson(`${USERS}?${query.toString()}`);
  const parsed = userListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getUserOptions(organizationId?: string, search?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) {
    query.set("organizationId", organizationId);
  }
  if (search) {
    query.set("keyword", search);
  }

  const response = await requestDocumentJson(`${USERS}?${query.toString()}`);
  return userListResponseSchema.parse(response).items.map((item) => ({
    id: item.id,
    username: item.username,
    displayName: item.displayName,
  }));
}

export async function getUserByOrganizationSlugAndUsername(
  organizationSlug: string,
  username: string,
): Promise<UserDetail | null> {
  const organization = await getOrganizationIdentityBySlug(organizationSlug);
  if (!organization) {
    return null;
  }

  // 用服务端 keyword 过滤精确定位：组织用户超过 50 条时首页翻页会找不到
  // 目标用户，导致详情页误报「不存在」。
  const users = await getUserOptions(organization.id, username);
  const matched = users.find((item) => item.username === username);
  if (!matched) {
    return null;
  }

  // 详情 GET 必须显式携带目标组织：平台管理员会话组织 ≠ 用户所属组织时，
  // 不带 query 的请求会落到会话组织导致 404（admin 打不开其他租户用户
  // 详情页的根因）。租户用户请求的 org 与会话组织一致，行为不变。
  return getUser(matched.id, organization.id);
}

export async function getPromptUserByOrganizationSlugAndUsername(
  organizationSlug: string,
  username: string,
  getProvidersFn: (organizationId?: string) => Promise<Array<{ id?: string; type?: string }>>,
): Promise<PromptUser | null> {
  const organization = await getOrganizationIdentityBySlug(organizationSlug);
  if (!organization) {
    return null;
  }

  const [users, providers] = await Promise.all([
    getUserOptions(organization.id, username),
    getProvidersFn(organization.id),
  ]);
  const matched = users.find((item) => item.username === username);
  if (!matched) {
    return null;
  }

  // 同 getUserByOrganizationSlugAndUsername：详情 GET 必须显式携带目标组织。
  const user = await getUser(matched.id, organization.id);
  const providerTypeById = new Map(
    providers.map((provider) => [provider.id ?? "", (provider.type ?? "").toLowerCase()]),
  );
  const linkedProviders: Record<string, string> = {};

  for (const identity of user.identities) {
    const providerId = identity.providerId ?? "";
    const providerType = providerTypeById.get(providerId) ?? "";
    if (!providerType) {
      continue;
    }

    const providerAccount =
      identity.providerAccount && identity.providerAccount !== ""
        ? identity.providerAccount
        : identity.subject
          ? identity.subject
          : "";

    if (providerAccount !== "" && linkedProviders[providerType] === undefined) {
      linkedProviders[providerType] = providerAccount;
    }
  }

  return {
    id: user.id,
    username: user.username,
    profile: user.profile,
    linkedProviders,
  };
}

/**
 * 详情级端点在服务端强制可信租户：平台管理员会话组织可能与目标用户所属
 * 组织不同，必须把目标组织显式作为 query 传入，否则请求会落到会话组织
 * 导致 404。租户用户可不传。
 */
function withOrgQuery(base: string, organizationId?: string): string {
  if (!organizationId) return base;
  const separator = base.includes("?") ? "&" : "?";
  return `${base}${separator}organizationId=${encodeURIComponent(organizationId)}`;
}

export async function getUser(id: string, organizationId?: string): Promise<UserDetail> {
  // 详情 GET 带 `private, max-age=30`：保存后 Query 失效重取会命中旧响应，
  // 页面拿到旧 revision 导致再次保存 409；编辑器必须绕开 HTTP 缓存。
  const response = await requestDocumentJson(withOrgQuery(USER_BY_ID(id), organizationId), {
    cache: "no-store",
  });
  return toUserDetail(userDetailSchema.parse(response));
}

export async function createUser(payload: CreateUserPayload): Promise<CreateUserResult> {
  const response = await requestDocumentJson(`${USERS}`, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
  return createUserResultSchema.parse(response);
}

export async function updateUser(id: string, payload: PatchUserPayload, organizationId?: string) {
  await requestDocumentJson(withOrgQuery(USER_BY_ID(id), organizationId), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function deleteUser(id: string, organizationId?: string) {
  await requestDocumentJson(withOrgQuery(USER_BY_ID(id), organizationId), {
    method: "DELETE",
  });
}

export async function updateUserProfile(
  id: string,
  payload: UpdateUserProfilePayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_PROFILE(id), organizationId), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

/**
 * Replaces the JSONB collections (addresses, managed accounts, MFA entries,
 * cart, consent scopes) of one user. Secrets must be submitted empty unless the
 * operator edits them; the backend keeps the stored value for empty secrets.
 */
export async function updateUserCollections(
  id: string,
  payload: UserCollectionsPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_COLLECTIONS(id), organizationId), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

/** Sends a verification code to the user's stored email or phone. */
export async function sendUserContactVerification(
  id: string,
  channel: ContactVerificationChannel,
  organizationId?: string,
): Promise<ContactVerificationTicket> {
  const response = await requestDocumentJson<ContactVerificationTicket>(
    withOrgQuery(USER_CONTACT_VERIFICATION_SEND(id), organizationId),
    {
      method: "POST",
      body: JSON.stringify({ channel }),
      headers: {
        "Content-Type": "application/json",
      },
    },
  );
  return response as ContactVerificationTicket;
}

/** Consumes the code and flips the verified flag of the channel. */
export async function verifyUserContactVerification(
  id: string,
  channel: ContactVerificationChannel,
  code: string,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_CONTACT_VERIFICATION_VERIFY(id), organizationId), {
    method: "POST",
    body: JSON.stringify({ channel, code }),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

/** Lists tenant custom attribute definitions (key/value fallback is allowed). */
export async function listCustomAttributeDefinitions(
  organizationId?: string,
): Promise<CustomAttributeDefinition[]> {
  const response = await requestDocumentJson<{ items?: CustomAttributeDefinition[] }>(
    withOrgQuery(USER_CUSTOM_ATTRIBUTES, organizationId),
  );
  return response?.items ?? [];
}

/**
 * Expands the user's role bindings into concrete resource/action grants.
 * Read-only; the server scopes the user id to the trusted tenant.
 */
export async function getUserEffectivePermissions(
  id: string,
  organizationId?: string,
): Promise<UserEffectivePermission[]> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_EFFECTIVE_PERMISSIONS(id), organizationId),
  );
  return userEffectivePermissionsSchema.parse(response).items;
}

/**
 * Uploads one document image and returns the provider's OCR fields. Nothing is
 * persisted: the caller copies the fields into the form and saves normally.
 */
export async function recognizeUserIdentityDocument(
  id: string,
  params: {
    documentType: IdentityDocumentType;
    file: File;
    side?: "front" | "back";
  },
  organizationId?: string,
): Promise<IdentityDocumentFields> {
  const formData = new FormData();
  formData.append("documentType", params.documentType);
  if (params.side) formData.append("side", params.side);
  formData.append("file", params.file, params.file.name);

  const response = await requestDocumentJson<IdentityDocumentFields>(
    withOrgQuery(USER_IDENTITY_DOCUMENT_RECOGNIZE(id), organizationId),
    {
      method: "POST",
      body: formData,
    },
  );
  return response as IdentityDocumentFields;
}

/**
 * Uploads a captured face image; the server runs the provider's liveness check
 * and only then stores the image and appends the face template to the user.
 * Returns the persisted FaceId entry.
 */
export async function registerUserFaceId(
  id: string,
  params: { file: File; providerId?: string; deviceInfo?: string },
  organizationId?: string,
): Promise<FaceIdItem> {
  const formData = new FormData();
  if (params.providerId) formData.append("providerId", params.providerId);
  if (params.deviceInfo) formData.append("deviceInfo", params.deviceInfo);
  formData.append("file", params.file, params.file.name);

  const response = await requestDocumentJson<FaceIdItem>(
    withOrgQuery(USER_FACEIDS(id), organizationId),
    {
      method: "POST",
      body: formData,
    },
  );
  return response as FaceIdItem;
}

/**
 * Revokes the user's active face template: blocks face-id login and clears the
 * legacy face_ids marker in one audited server transaction. Returns 204.
 */
export async function revokeUserFaceId(id: string, organizationId?: string): Promise<void> {
  await requestDocumentJson<unknown>(withOrgQuery(USER_FACEIDS(id), organizationId), {
    method: "DELETE",
  });
}

export async function updateUserCredentials(
  id: string,
  payload: UpdateUserCredentialsPayload,
  organizationId?: string,
): Promise<UpdatedUserCredentialsResult> {
  const response = await requestDocumentJson(withOrgQuery(USER_CREDENTIALS(id), organizationId), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });

  return updatedUserCredentialsResultSchema.parse(response);
}

export async function replaceUserIdentities(
  id: string,
  items: ReplaceUserIdentityItem[],
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_IDENTITIES(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ items }),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function replaceUserMfaMethods(
  id: string,
  revision: number,
  items: ReplaceUserMfaMethodItem[],
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_MFA_METHODS(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ revision, items }),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function updateUserPreferredMfa(
  id: string,
  payload: UpdateUserPreferredMfaPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_PREFERRED_MFA(id), organizationId), {
    method: "PUT",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function initiateUserMfaSetup(
  id: string,
  payload: InitiateUserMfaSetupPayload,
  organizationId?: string,
): Promise<UserMfaSetupInitiateResult> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_MFA_SETUP_INITIATE(id), organizationId),
    {
      method: "POST",
      body: JSON.stringify(payload),
      headers: {
        "Content-Type": "application/json",
      },
    },
  );

  return userMfaSetupInitiateResultSchema.parse(response);
}

export async function verifyUserMfaSetup(
  id: string,
  payload: VerifyUserMfaSetupPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_MFA_SETUP_VERIFY(id), organizationId), {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export async function enableUserMfaSetup(
  id: string,
  payload: EnableUserMfaSetupPayload,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_MFA_SETUP_ENABLE(id), organizationId), {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

// ─────────────────────────────────────────────
// Internal helpers (shared with other modules)
// ─────────────────────────────────────────────

async function getOrganizationIdentityBySlug(slug: string): Promise<OrganizationIdentity | null> {
  try {
    const response = await requestDocumentJson(ORGANIZATION_BY_SLUG(slug));
    const matched = z.object({ id: z.string(), slug: z.string() }).parse(response);
    return { id: matched.id, slug: matched.slug };
  } catch (error) {
    captureError(error, { slug, fn: "getOrganizationIdentityBySlug" });
    // 404 = 组织不存在（返回 null 走 not-found 分支）；其余错误向上抛，
    // 避免把 403/5xx/网络故障误判成「组织不存在」。
    if (error instanceof HttpError && error.status === 404) return null;
    throw error;
  }
}

// ─────────────────────────────────────────────
// Sub-resource API functions
// ─────────────────────────────────────────────

/** Admin resets a user's password (users:password:reset). Reason is audit-only. */
export async function setUserPassword(
  id: string,
  password: string,
  reason?: string,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_SET_PASSWORD(id), organizationId), {
    method: "POST",
    body: JSON.stringify(reason ? { password, reason } : { password }),
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Start a real identity verification session. The backend persists a pending
 * record and returns the third-party provider redirect URL; approval only
 * happens after the provider confirms the result. The payload carries the
 * SDK-collected metaInfo/returnUrl the provider requires (fail-closed on the
 * server when blank).
 */
export async function verifyIdentification(
  userId: string,
  organizationId?: string,
  payload?: VerifyIdentificationPayload,
): Promise<IdentityVerificationTicket | undefined> {
  return requestDocumentJson<IdentityVerificationTicket>(
    withOrgQuery(USER_VERIFY_IDENTIFICATION(userId), organizationId),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      ...(payload ? { body: JSON.stringify(payload) } : {}),
    },
  );
}

// Backend Contract (users/webauthn_admin.go): `status` is one of
// active / revoked / deactivated. `credentialId` is the base64url path key —
// the UUID `id` is display-only and must never be sent in the URL.
const webAuthnCredentialSchema = z.object({
  id: z.string(),
  credentialId: z.string(),
  name: z.string(),
  attestationType: z.string(),
  transport: z.array(z.string()),
  signCount: z.number(),
  aaguid: z.string(),
  status: z.enum(["active", "revoked", "deactivated"]),
  createdAt: z.string(),
  updatedAt: z.string(),
});

// 平台管理员会话组织可能不同于目标用户组织；这些端点后端一律经
// trustedOrgID(query organizationId) 裁决，缺参会回退会话租户导致跨租户 404，
// 因此与 profile/sessions 一致显式携带目标组织。

/** List WebAuthn credentials for a user. */
export async function listWebAuthnCredentials(
  userId: string,
  organizationId?: string,
): Promise<WebAuthnCredentialItem[]> {
  // The endpoint answers with Cache-Control: private, max-age=30; React Query
  // invalidation must not be satisfied by that stale browser cache after a
  // credential write, so this read opts out of HTTP caching.
  const response = await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIALS(userId), organizationId),
    {
      cache: "no-store",
    },
  );
  return z.array(webAuthnCredentialSchema).parse(response);
}

/** Get one WebAuthn credential (detail drawer). */
export async function getWebAuthnCredential(
  userId: string,
  credentialId: string,
  organizationId?: string,
): Promise<WebAuthnCredentialItem> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId), organizationId),
    {
      cache: "no-store",
    },
  );
  return webAuthnCredentialSchema.parse(response);
}

/** Rename a credential; the backend returns the updated record. */
export async function renameWebAuthnCredential(
  userId: string,
  credentialId: string,
  name: string,
  organizationId?: string,
): Promise<WebAuthnCredentialItem> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_RENAME(userId, credentialId), organizationId),
    {
      method: "POST",
      body: JSON.stringify({ name }),
      headers: { "Content-Type": "application/json" },
    },
  );
  return webAuthnCredentialSchema.parse(response);
}

/** Delete a WebAuthn credential for a user. */
export async function deleteWebAuthnCredential(
  userId: string,
  credentialId: string,
  organizationId?: string,
) {
  await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_BY_ID(userId, credentialId), organizationId),
    {
      method: "DELETE",
    },
  );
}

/** Revoke a WebAuthn credential (compromised or lost device). */
export async function revokeWebAuthnCredential(
  userId: string,
  credentialId: string,
  organizationId?: string,
) {
  await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_REVOKE(userId, credentialId), organizationId),
    {
      method: "POST",
    },
  );
}

/** Deactivate a WebAuthn credential temporarily. */
export async function deactivateWebAuthnCredential(
  userId: string,
  credentialId: string,
  organizationId?: string,
) {
  await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_DEACTIVATE(userId, credentialId), organizationId),
    {
      method: "POST",
    },
  );
}

/** Restore a deactivated WebAuthn credential. */
export async function restoreWebAuthnCredential(
  userId: string,
  credentialId: string,
  organizationId?: string,
) {
  await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_RESTORE(userId, credentialId), organizationId),
    {
      method: "POST",
    },
  );
}

/** Begin admin on-behalf registration: issue a one-time challenge. */
export async function beginWebAuthnCredentialRegistration(
  userId: string,
  name?: string,
  organizationId?: string,
): Promise<AdminWebAuthnRegistrationBeginResult> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_REGISTRATION_BEGIN(userId), organizationId),
    {
      method: "POST",
      body: JSON.stringify({ name: name ?? "" }),
      headers: { "Content-Type": "application/json" },
    },
  );
  return z
    .object({
      challenge: z.string(),
      challengeId: z.string(),
      rpId: z.string(),
      rpName: z.string(),
      userId: z.string(),
      userDisplayName: z.string(),
      timeout: z.number(),
      userVerification: z.enum(["required", "preferred"]),
    })
    .parse(response);
}

/** Finish admin on-behalf registration; the server derives credential material. */
export async function finishWebAuthnCredentialRegistration(
  userId: string,
  payload: WebAuthnRegistrationFinishPayload,
  organizationId?: string,
): Promise<WebAuthnCredentialItem> {
  const response = await requestDocumentJson(
    withOrgQuery(USER_WEBAUTHN_CREDENTIAL_REGISTRATION_FINISH(userId), organizationId),
    {
      method: "POST",
      body: JSON.stringify(payload),
      headers: { "Content-Type": "application/json" },
    },
  );
  return webAuthnCredentialSchema.parse(response);
}

/** List transactions for a user. */
export async function listUserTransactions(
  userId: string,
  page = 1,
  pageSize = 20,
  organizationId?: string,
): Promise<{ items: UserTransactionItem[]; total: number }> {
  const query = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${USER_TRANSACTIONS(userId)}?${query.toString()}`);
  const parsed = z
    .object({
      items: z.array(
        z.object({
          id: z.string(),
          transactionType: z.string(),
          amount: z.number(),
          currency: z.string(),
          status: z.string(),
          providerType: z.string().optional(),
          createdAt: z.string(),
        }),
      ),
      page: z.object({ total: z.number() }),
    })
    .parse(response);
  return { items: parsed.items, total: parsed.page.total };
}

/** List consents for a user. */
export async function listUserConsents(
  userId: string,
  organizationId?: string,
): Promise<UserConsentItem[]> {
  const response = await requestDocumentJson(withOrgQuery(USER_CONSENTS(userId), organizationId));
  return z
    .array(
      z.object({
        id: z.string(),
        applicationId: z.string(),
        applicationSlug: z.string().optional(),
        grantedScopes: z.array(z.string()),
        createdAt: z.string(),
        revokedAt: z.string().optional(),
      }),
    )
    .parse(response);
}

/** Revoke a consent for a user. */
export async function revokeConsent(
  userId: string,
  consentId: string,
  organizationId?: string,
): Promise<void> {
  await requestDocumentJson(withOrgQuery(USER_CONSENT_BY_ID(userId, consentId), organizationId), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// Bulk Import / Export
// ─────────────────────────────────────────────

/** Import users in bulk via JSON array. */
export async function importUsers(users: ImportUserItem[]): Promise<BulkImportResult> {
  const response = await requestDocumentJson(USER_IMPORT, {
    method: "POST",
    body: JSON.stringify({ users }),
    headers: { "Content-Type": "application/json" },
  });
  return z
    .object({
      total: z.number(),
      succeeded: z.number(),
      failed: z.number(),
      errors: z.array(z.string()).optional(),
    })
    .parse(response);
}

/** Import users from an Excel file (.xlsx). */
export async function importUsersFromExcel(
  file: File,
  organizationId?: string,
): Promise<BulkImportResult> {
  const formData = new FormData();
  formData.append("file", file);
  const query = organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : "";
  const response = await requestDocumentJson(`${USER_UPLOAD}${query}`, {
    method: "POST",
    body: formData,
  });
  return z
    .object({
      total: z.number(),
      succeeded: z.number(),
      failed: z.number(),
      errors: z.array(z.string()).optional(),
    })
    .parse(response);
}

/** Upload users via Excel file (full import). */
export async function uploadUsers(file: File, organizationId?: string): Promise<BulkImportResult> {
  return importUsersFromExcel(file, organizationId);
}

/** Preview users from Excel file without importing. */
export async function previewUsers(file: File, organizationId?: string): Promise<PreviewResult> {
  const formData = new FormData();
  formData.append("file", file);
  const query = organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : "";
  const response = await requestDocumentJson(`${USER_UPLOAD_PREVIEW}${query}`, {
    method: "POST",
    body: formData,
  });
  return z
    .object({
      totalRows: z.number(),
      columns: z.array(z.string()),
      preview: z.array(z.record(z.string(), z.unknown())),
    })
    .parse(response);
}

/** Export users to Excel (.xlsx) and trigger download. */
export async function exportUsersToExcel(organizationId?: string): Promise<void> {
  const query = new URLSearchParams();
  if (organizationId) query.set("organizationId", organizationId);

  const url = `${USER_EXPORT}${query.toString() ? `?${query.toString()}` : ""}`;

  const response = await requestBlob(url);

  const blob = await response.blob();
  const disposition = response.headers.get("content-disposition") ?? "";
  const filenameMatch = disposition.match(/filename="?([^";\n]+)"?/);
  const filename =
    filenameMatch?.[1] ?? `users_export_${new Date().toISOString().slice(0, 10)}.xlsx`;

  const anchor = document.createElement("a");
  anchor.href = URL.createObjectURL(blob);
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(anchor.href);
}

/** Get total user count, optionally filtered by organization. */
export async function getUserCount(organizationId?: string): Promise<number> {
  const query = new URLSearchParams();
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(
    `${USER_COUNT}${query.toString() ? `?${query.toString()}` : ""}`,
  );
  return z.object({ count: z.number() }).parse(response).count;
}

/** Remove a user from a group. */
export async function removeUserFromGroup(userId: string, groupId: string) {
  await requestDocumentJson(USER_REMOVE_FROM_GROUP(userId), {
    method: "POST",
    body: JSON.stringify({ groupId }),
    headers: { "Content-Type": "application/json" },
  });
}

/** Replace all role bindings for a user. */
export async function replaceUserRoleBindings(
  id: string,
  items: Array<{ roleId: string; roleName?: string }>,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_ROLE_BINDINGS(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ items }),
    headers: { "Content-Type": "application/json" },
  });
}

/** Replace all group memberships for a user. */
export async function replaceUserGroupMemberships(
  id: string,
  items: Array<{ groupId: string; groupName?: string }>,
  organizationId?: string,
) {
  await requestDocumentJson(withOrgQuery(USER_GROUP_MEMBERSHIPS(id), organizationId), {
    method: "PUT",
    body: JSON.stringify({ items }),
    headers: { "Content-Type": "application/json" },
  });
}

/** Delete a specific role binding for a user. */
export async function deleteUserRoleBinding(userId: string, roleId: string) {
  await requestDocumentJson(USER_ROLE_BINDING_BY_ROLE(userId, roleId), { method: "DELETE" });
}

/** Delete a specific group membership for a user. */
export async function deleteUserGroupMembership(userId: string, groupId: string) {
  await requestDocumentJson(USER_GROUP_BY_NAME(userId, groupId), { method: "DELETE" });
}

/** Get masked contact info for a user (for password reset flow). */
export async function getMaskedContact(
  userId: string,
): Promise<{ email?: string; phone?: string }> {
  const response = await requestDocumentJson(USER_MASKED_CONTACT(userId));
  return response as { email?: string; phone?: string };
}

/** Unlink an identity from a user. */
export async function unlinkIdentity(userId: string, providerId: string, identityType: string) {
  await requestDocumentJson(USER_UNLINK(userId), {
    method: "POST",
    body: JSON.stringify({ providerId, identityType }),
    headers: { "Content-Type": "application/json" },
  });
}

/** Impersonate a user (admin-only). */
export async function impersonateUser(userId: string): Promise<{ token: string }> {
  const response = await requestDocumentJson(USER_IMPERSONATE(userId), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  return z.object({ token: z.string() }).parse(response);
}

/**
 * Exit the current impersonation session via the auth face.
 *
 * 不能用 users 面的 /users/{id}/exit-impersonate：imp 优先后请求身份是被冒充
 * 用户（非管理员），users:write 会 403。auth 面按会话自身身份撤销 imp token。
 */
export async function exitImpersonationSession() {
  await requestDocumentJson(AUTH_EXIT_IMPERSONATE, { method: "POST" });
}

// ─────────────────────────────────────────────
// User-Scoped Orders & Payments
// ─────────────────────────────────────────────

/** List orders for a specific user. */
export async function listUserOrders(
  userId: string,
  organizationId?: string,
  page = 1,
  pageSize = 20,
): Promise<{ items: UserOrderItem[]; page: number; pageSize: number; total: number }> {
  const query = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${USER_ORDERS(userId)}?${query.toString()}`);
  const parsed = z
    .object({
      items: z.array(
        z.object({
          id: z.string(),
          organizationId: z.string(),
          name: z.string(),
          displayName: z.string(),
          user: z.string().optional(),
          price: z.number(),
          currency: z.string().optional(),
          state: z.string(),
          createdAt: z.string(),
          updatedAt: z.string(),
        }),
      ),
      page: z.object({ page: z.number(), pageSize: z.number(), total: z.number() }),
    })
    .parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

/** List payments for a specific user. */
export async function listUserPayments(
  userId: string,
  organizationId?: string,
  page = 1,
  pageSize = 20,
): Promise<{ items: UserPaymentItem[]; page: number; pageSize: number; total: number }> {
  const query = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${USER_PAYMENTS(userId)}?${query.toString()}`);
  const parsed = z
    .object({
      items: z.array(
        z.object({
          id: z.string(),
          organizationId: z.string(),
          name: z.string(),
          displayName: z.string(),
          provider: z.string().optional(),
          type: z.string().optional(),
          state: z.string(),
          price: z.number(),
          currency: z.string().optional(),
          createdAt: z.string(),
          updatedAt: z.string(),
        }),
      ),
      page: z.object({ page: z.number(), pageSize: z.number(), total: z.number() }),
    })
    .parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

/** 管理端受信设备视图（GET /users/{id}/trusted-devices）。 */
export type TrustedDeviceItem = {
  id: string;
  userId: string;
  deviceCredentialId: string;
  status: "active" | "revoked" | "expired";
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
};

export async function listUserTrustedDevices(
  userId: string,
  organizationId?: string,
): Promise<TrustedDeviceItem[]> {
  return (
    (await requestDocumentJson<TrustedDeviceItem[]>(
      withOrgQuery(USER_TRUSTED_DEVICES(userId), organizationId),
    )) ?? []
  );
}

export async function revokeUserTrustedDevice(
  userId: string,
  deviceId: string,
  organizationId?: string,
): Promise<void> {
  await requestDocumentJson<unknown>(
    withOrgQuery(USER_TRUSTED_DEVICE_BY_ID(userId, deviceId), organizationId),
    {
      method: "DELETE",
    },
  );
}
