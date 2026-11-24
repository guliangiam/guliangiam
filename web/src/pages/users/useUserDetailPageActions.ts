/**
 * UserDetailPage 的页面控制器 hook。
 *
 * 从 UserDetailPage.tsx 拆分而来，把「页面可编辑状态 + 8 个 mutation +
 * 全部提交/identity/role/group 处理器 + user → state 同步」收敛到一处，
 * 让页面组件退化为纯布局控制器（与 `useUserDetailPageQueries` 对称）。
 *
 * 所有分支、提示文案、失效 query key 与默认值均与原实现保持一致。
 */
import { useEffect, useState } from "react";
import { App } from "antd";
import type { FormInstance } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteUser,
  recognizeUserIdentityDocument,
  replaceUserGroupMemberships,
  replaceUserIdentities,
  replaceUserMfaMethods,
  replaceUserRoleBindings,
  updateUser,
  updateUserCollections,
  updateUserCredentials,
  updateUserPreferredMfa,
  updateUserProfile,
  verifyIdentification,
  type AddressItem,
  type ConsentRecordItem,
  type FaceIdItem,
  type IdentityDocumentType,
  type ManagedAccountItem,
  type ProductInfoItem,
  type UserCollectionsPayload,
  type UserDetail,
} from "@/shared/api/iam/user";
import { socialAuthorize } from "@/shared/api/iam/auth";
import { isHttpError } from "@/shared/api/http";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { buildIdvReturnUrl, CloudAuthMetaError, collectMetaInfo } from "@/shared/lib/cloudauth-sdk";
import { idvPendingStorageKey, useIdvPendingPolling } from "@/shared/hooks/useIdvPendingPolling";
import {
  buildCredentialsPayload,
  buildCustomProperties,
  buildIdentities,
  buildProfileFormValues,
  buildUserFormValues,
  buildUserSettingsPayload,
  buildUserUpdatePayload,
  hasCredentialChanges,
  hasUserSettingsChanges,
  hasUserUpdateChanges,
  validateCustomProperties,
} from "./user-detail-payload";
import { toGroupMembershipItems, toIdentityItems, toRoleBindingItems } from "./user-detail-helpers";
import type {
  CredentialsFormValues,
  GroupListItem,
  IdentityFormItem,
  NewIdentityValues,
  ProfileFormValues,
  RoleOption,
  UserDetailFormValues,
} from "./tabs/types";

type UseUserDetailPageActionsParams = {
  id: string | undefined;
  user: UserDetail | undefined;
  roleOptions: RoleOption[] | undefined;
  groupList: GroupListItem[] | undefined;
  /**
   * 详情页的纯路径（`/users/:organizationName/:userName`，不含 query/hash）。
   *
   * 阿里云托管页完成后按 returnUrl 回跳；官方解析不接受 query/hash，且带 `?tab=`
   * 会把导航状态写进厂商链接，因此由页面把路由参数拼成纯路径传进来。
   */
  returnPath: string;
  /**
   * 页面唯一的表单实例。
   *
   * 所有可编辑 Tab 共用这一个实例（Tab 内只渲染 Form.Item，不再各自包 <form>），
   * 这样页头的保存按钮、草稿自动保存与「保存后回显」都只作用于同一份值；
   * 同一时刻只有一个 Tab 挂载（Tabs destroyOnHidden），校验范围自然收敛到当前 Tab。
   */
  form: FormInstance<UserDetailFormValues>;
};

const EMPTY_NEW_IDENTITY: NewIdentityValues = {
  providerId: "",
  identityType: "",
  subject: "",
  providerAccount: "",
};

/** 页头保存可能触达的接口阶段；顺序即提交顺序（profile 自增 revision，必须排在 users 之后）。 */
type SaveStage = "users" | "profile" | "collections" | "credentials";

/** Normalized collection payload. Secrets are omitted so the backend keeps the
 * stored value; all other fields are sent as-is (full replacement). */
function buildCollectionsPayload(state: {
  addresses: AddressItem[];
  managedAccounts: ManagedAccountItem[];
  faceIds: FaceIdItem[];
  cart: ProductInfoItem[];
  applicationScopes: ConsentRecordItem[];
}): UserCollectionsPayload {
  return {
    addresses: state.addresses.map((item) => ({
      id: item.id,
      countryCode: item.countryCode,
      province: item.province,
      city: item.city,
      district: item.district,
      postalCode: item.postalCode,
      street: item.street,
      details: item.details,
      region: item.region,
      isDefault: item.isDefault,
    })),
    managedAccounts: state.managedAccounts.map((item) => ({
      application: item.application,
      username: item.username,
      signinUrl: item.signinUrl,
      passwordSet: item.passwordSet,
      passwordMasked: item.passwordMasked,
      ...(item.password ? { password: item.password } : {}),
    })),
    faceIds: state.faceIds,
    cart: state.cart,
    applicationScopes: state.applicationScopes,
  };
}

export function useUserDetailPageActions({
  id,
  user,
  roleOptions,
  groupList,
  returnPath,
  form,
}: UseUserDetailPageActionsParams) {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const reportMutationError = (error: unknown, fallbackKey: string) => {
    if (isHttpError(error) && error.status === 401) return;
    message.error(serverErrorMessage(error, t, fallbackKey));
  };

  // ResourceFormPage owns the 422 field mapping so every form shares one
  // resolver; this helper only keeps the mutation rejection intact.
  const submitWithFieldErrors = async (operation: () => Promise<unknown>) => {
    await operation();
  };

  // ─── Sub-form states ───────────────────────────────────────
  const [addresses, setAddresses] = useState<AddressItem[]>([]);
  const [managedAccounts, setManagedAccounts] = useState<ManagedAccountItem[]>([]);
  const [applicationScopes, setApplicationScopes] = useState<ConsentRecordItem[]>([]);
  const [cart, setCart] = useState<ProductInfoItem[]>([]);
  const [faceIds, setFaceIds] = useState<FaceIdItem[]>([]);
  const [identities, setIdentities] = useState<IdentityFormItem[]>([]);
  const [customProperties, setCustomProperties] = useState<
    NonNullable<ProfileFormValues["properties"]>
  >([]);
  const [syncedUser, setSyncedUser] = useState<typeof user>(undefined);

  // ─── Editable identity state ───────────────────────────────
  const [addIdentityVisible, setAddIdentityVisible] = useState(false);
  const [newIdentity, setNewIdentity] = useState<NewIdentityValues>(EMPTY_NEW_IDENTITY);

  // ─── Editable role/group state ─────────────────────────────
  const [addRoleVisible, setAddRoleVisible] = useState(false);
  const [selectedRoleId, setSelectedRoleId] = useState<string>("");
  const [roleSavePending, setRoleSavePending] = useState(false);
  const [addGroupVisible, setAddGroupVisible] = useState(false);
  const [selectedGroupId, setSelectedGroupId] = useState<string>("");
  const [groupSavePending, setGroupSavePending] = useState(false);

  // ─── Hosted IDV challenge state ────────────────────────────
  // 发起成功 = 拿到厂商托管页；`idvChallenge` 驱动轮询，`idvQrVisible` 只驱动弹窗：
  // 操作员关掉弹窗不等于放弃核验，轮询仍要把结果刷回 isVerified。
  const [idvChallenge, setIdvChallenge] = useState<
    { verificationId: string; redirectUrl: string } | undefined
  >(undefined);
  const [idvQrVisible, setIdvQrVisible] = useState(false);
  const [idvCollecting, setIdvCollecting] = useState(false);

  useIdvPendingPolling({
    userId: id,
    organizationId: user?.organizationId,
    verificationId: idvChallenge?.verificationId,
    onSettled: (status) => {
      setIdvChallenge(undefined);
      setIdvQrVisible(false);
      if (status === "approved") message.success(t("verifyIdSuccess"));
      else message.error(t("verifyIdFailed"));
    },
  });

  const closeIdvQr = () => setIdvQrVisible(false);

  // ─── Sync user data to forms (split into focused effects) ────

  // Sync every editable field from user data into the single page form.
  // Profile values are a subset of the user form values, so a shallow merge is
  // enough and keeps last-fetch data authoritative after each refetch.
  useEffect(() => {
    if (!user) return;
    form.setFieldsValue({ ...buildUserFormValues(user), ...buildProfileFormValues(user) });
  }, [user, form]);

  // Sync sub-resource arrays, custom properties and identities from user data.
  // These run during render (React's documented alternative to syncing external
  // data into state from an effect), so no extra render pass is triggered.
  if (user && user !== syncedUser) {
    setSyncedUser(user);
    setAddresses(user.addresses ?? []);
    setManagedAccounts(user.managedAccounts ?? []);
    setApplicationScopes(user.applicationScopes ?? []);
    setCart(user.cart ?? []);
    setFaceIds(user.faceIds ?? []);
    setCustomProperties(buildCustomProperties(user.profile));
    setIdentities(buildIdentities(user));
  }

  // ─── Mutations ─────────────────────────────────────────────

  // ─── 页面保存：跨 Tab 汇总 ────────────────────────────────
  //
  // 所有可编辑 Tab 共用同一个 Form 实例；Tab 因 destroyOnHidden 卸载时 antd 默认
  // preserve 保留字段值，因此整份 store 始终包含所有已访问 Tab 的改动。
  // 保存计划 = 当前 Tab 的主阶段（保持 basic/account 必发 PATCH+PUT 的原有语义）
  // + 其他阶段中确实有改动的部分，避免「在 Profile 保存只发 PUT /profile」把
  // Basic/凭据 的改动静默丢弃。users → profile → credentials 的顺序不可打乱：
  // PUT /profile 会自增 revision，先 PUT 会让 PATCH 的乐观锁校验 409。
  const stageLabelKey: Record<Exclude<SaveStage, "users">, string> = {
    profile: "resource:user.tabs.profile",
    collections: "resource:user.assetsSection",
    credentials: "resource:user.tabs.credentials",
  };

  const collectionsPayload = () =>
    buildCollectionsPayload({
      addresses,
      managedAccounts,
      faceIds,
      cart,
      applicationScopes,
    });

  const collectionsBaseline = user
    ? JSON.stringify(
        buildCollectionsPayload({
          addresses: user.addresses ?? [],
          managedAccounts: user.managedAccounts ?? [],
          faceIds: user.faceIds ?? [],
          cart: user.cart ?? [],
          applicationScopes: user.applicationScopes ?? [],
        }),
      )
    : "";

  const saveAllMutation = useMutation({
    mutationFn: async ({ plan, values }: { plan: SaveStage[]; values: UserDetailFormValues }) => {
      // 三个端点各自独立，无法共享事务。这里记录失败所在阶段：首段（users）失败
      // 直接抛出原始错误以保留 422 字段映射；后续段失败则重新拉取用户，让界面
      // 展示「已经落库的部分」，并用一条明确的提示说明保存只完成了一部分。
      let stage: SaveStage = "users";
      try {
        for (const current of plan) {
          stage = current;
          if (current === "users") {
            await updateUser(id!, buildUserUpdatePayload(values, user), user?.organizationId);
          } else if (current === "profile") {
            await updateUserProfile(
              id!,
              buildUserSettingsPayload(values, customProperties),
              user?.organizationId,
            );
          } else if (current === "collections") {
            await updateUserCollections(id!, collectionsPayload(), user?.organizationId);
          } else {
            await updateUserCredentials(id!, buildCredentialsPayload(values), user?.organizationId);
          }
        }
        return plan;
      } catch (error) {
        if (stage === "users") throw error;
        throw new Error(`${t("resource:user.saveUserFailed")}: ${t(stageLabelKey[stage])}`, {
          cause: error,
        });
      }
    },
    onError: () => {
      // 部分成功时也要让用户看到实际落库结果，而不是停留在旧表单值。
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
    onSuccess: (plan) => {
      const credentialsOnly = plan.length === 1 && plan[0] === "credentials";
      message.success(credentialsOnly ? t("resource:user.credentialsUpdated") : t("updateSuccess"));
      form.setFieldsValue({ rotateAccessSecret: false });
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });

  const verifyIdMutation = useMutation({
    mutationFn: async () => {
      // 后端对 metaInfo / returnUrl 双 fail-closed（缺失即 400），所以必须先采集
      // CloudAuth SDK 的 metaInfo 再发请求；采集失败在本地拦截，不发空 body。
      setIdvCollecting(true);
      try {
        const metaInfo = await collectMetaInfo();
        return await verifyIdentification(id!, user?.organizationId, {
          metaInfo,
          returnUrl: buildIdvReturnUrl(returnPath),
        });
      } finally {
        setIdvCollecting(false);
      }
    },
    onSuccess: (ticket) => {
      const redirectUrl = ticket?.data?.redirectUrl;
      if (redirectUrl) {
        const verificationId = ticket?.data?.verificationId;
        if (verificationId && id)
          window.localStorage.setItem(idvPendingStorageKey(id), verificationId);
        // 管理端是操作员的桌面：不 `location.assign` 把页面甩走，改弹二维码让本人手机扫码。
        setIdvChallenge(verificationId ? { verificationId, redirectUrl } : undefined);
        setIdvQrVisible(true);
        queryClient.invalidateQueries({ queryKey: ["user"] });
        return;
      }
      if (ticket && ticket.status !== "verified") {
        message.success(t("verifyIdPending"));
      } else {
        message.success(t("verifyIdSuccess"));
      }
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
    onError: (error) => {
      // SDK 侧失败（加载超时/全局缺失）不是后端拒绝，厂商文案会误导用户重试。
      if (error instanceof CloudAuthMetaError) {
        message.error(t("verifyIdMetaFailed"));
        return;
      }
      reportMutationError(error, "verifyIdFailed");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteUser(id!, user?.organizationId),
    onSuccess: () => {
      message.success(t("common:deleted"));
      queryClient.invalidateQueries({ queryKey: ["users"] });
      queryClient.invalidateQueries({ queryKey: ["user"] });
      navigate("/users");
    },
    onError: (error) => reportMutationError(error, "common:deleteFailed"),
  });

  const identityMutation = useMutation({
    mutationFn: (items: IdentityFormItem[]) => replaceUserIdentities(id!, toIdentityItems(items)),
    onSuccess: () => {
      message.success(t("resource:user.identitiesUpdated"));
      queryClient.invalidateQueries({ queryKey: ["user"] });
    },
    onError: (error) => {
      reportMutationError(error, "resource:user.updateIdentitiesFailed");
      // 失败后回拉服务端真值，避免表单停留在未落库的乐观状态。
      queryClient.invalidateQueries({ queryKey: ["user"] });
    },
  });

  const mfaMutation = useMutation({
    mutationFn: (items: { methodType: string; enabled: boolean; displayName?: string }[]) =>
      replaceUserMfaMethods(id!, user?.revision ?? 0, items, user?.organizationId),
    onSuccess: () => {
      message.success(t("updateSuccess"));
      queryClient.invalidateQueries({ queryKey: ["user"] });
    },
    onError: (error) => reportMutationError(error, "common:saveFailed"),
  });

  const preferredMfaMutation = useMutation({
    mutationFn: (mfaType: string) => updateUserPreferredMfa(id!, { mfaType }, user?.organizationId),
    onSuccess: () => {
      message.success(t("resource:user.preferredMfaUpdated"));
      queryClient.invalidateQueries({ queryKey: ["user"] });
    },
    onError: (error) => reportMutationError(error, "resource:user.setPreferredMfaFailed"),
  });

  // ─── Access Secret 轮换（独立按钮触发，不再走表单 Switch）────────────────
  // 轮换成功后端一次性返回新 key/secret，保存在本地 state 供凭证 Tab 展示；
  // 旧 secret 在 AccessSecretGracePeriod（30 分钟）内仍可通过 prev hash 校验。
  const [rotatedSecret, setRotatedSecret] = useState<{
    accessKey: string;
    accessSecret: string;
  } | null>(null);
  const rotateSecretMutation = useMutation({
    mutationFn: () =>
      updateUserCredentials(id!, { rotateAccessSecret: true }, user?.organizationId),
    onSuccess: (result) => {
      if (result?.accessKey && result?.accessSecret) {
        setRotatedSecret({ accessKey: result.accessKey, accessSecret: result.accessSecret });
      }
      message.success(t("resource:user.secretRotated", "Access secret rotated"));
      queryClient.invalidateQueries({ queryKey: ["user"] });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    },
    onError: (error) => reportMutationError(error, "resource:user.secretRotateFailed"),
  });
  const dismissRotatedSecret = () => setRotatedSecret(null);

  // ─── Identity document OCR ────────────────────────────────
  const recognizeDocumentMutation = useMutation({
    mutationFn: ({
      documentType,
      file,
      side,
    }: {
      documentType: IdentityDocumentType;
      file: File;
      side?: "front" | "back";
    }) => recognizeUserIdentityDocument(id!, { documentType, file, side }, user?.organizationId),
    onSuccess: (fields) => {
      // 需求 7：OCR 识别出的全部身份字段回填对应表单项；OCR 未返回值时保留原值。
      form.setFieldsValue({
        realName: fields.fullName ?? form.getFieldValue("realName"),
        idCardType: fields.idType ?? form.getFieldValue("idCardType"),
        idCard: fields.idNumber ?? form.getFieldValue("idCard"),
        gender: fields.gender ?? form.getFieldValue("gender"),
        birthday: fields.birthday ?? form.getFieldValue("birthday"),
        issuingAuthority: fields.issuingAuthority ?? form.getFieldValue("issuingAuthority"),
        idCardExpiry: fields.validTo ?? form.getFieldValue("idCardExpiry"),
      });
      message.success(t("resource:user.documentRecognized", "Document recognized"));
    },
    onError: (error) => reportMutationError(error, "resource:user.documentRecognizeFailed"),
  });

  // ─── Save handlers ─────────────────────────────────────────
  //
  // 页面共用一个 Form 实例；Tabs destroyOnHidden 只挂载当前 Tab，但 antd 默认
  // preserve 会把卸载字段的值留在 store 里，所以 `getFieldsValue(true)` 能读到
  // 所有已访问 Tab 的改动。PUT /profile 是全量替换（缺省字段会被清空），必须从
  // 整份 store 取值；payload 装配函数按白名单挑字段，不会把别的 Tab 字段带进
  // 不属于它的接口。
  const readAllValues = (): UserDetailFormValues =>
    form.getFieldsValue(true) as UserDetailFormValues;

  /** 当前 Tab 的主阶段固定执行，其余阶段只在整份 store 里确实有改动时补发。 */
  const resolveSavePlan = (always: SaveStage | null, values: UserDetailFormValues): SaveStage[] => {
    const usersDirty = hasUserUpdateChanges(buildUserUpdatePayload(values, user));
    const settingsDirty = hasUserSettingsChanges(values, customProperties, user);
    const credentialsDirty = hasCredentialChanges(values, user);
    const collectionsDirty = collectionsBaseline !== JSON.stringify(collectionsPayload());
    const plan: SaveStage[] = [];
    if (always === "users" || usersDirty) plan.push("users");
    if (always === "users" || always === "profile" || settingsDirty) plan.push("profile");
    if (collectionsDirty) plan.push("collections");
    if (always === "credentials" || credentialsDirty) plan.push("credentials");
    return plan;
  };

  const runPageSave = (always: SaveStage | null, submitted?: CredentialsFormValues) => {
    const values = readAllValues();
    if (submitted?.newPassword && submitted.newPassword !== submitted.confirmPassword) {
      message.error(t("resource:user.validation.passwordMismatch"));
      return Promise.resolve(false);
    }
    const plan = resolveSavePlan(always, values);
    if (plan.length === 0) {
      message.info(t("resource:user.noChanges", "No changes to save"));
      return Promise.resolve(true);
    }
    if (plan.includes("profile") && !ensureCustomPropertiesValid()) {
      return Promise.resolve(false);
    }
    return submitWithFieldErrors(() => saveAllMutation.mutateAsync({ plan, values }));
  };

  const handleReset = () => {
    if (!user) return;
    form.resetFields();
    form.setFieldsValue({ ...buildUserFormValues(user), ...buildProfileFormValues(user) });
    setCustomProperties(buildCustomProperties(user.profile));
    setAddresses(user.addresses ?? []);
    setManagedAccounts(user.managedAccounts ?? []);
    setApplicationScopes(user.applicationScopes ?? []);
    setCart(user.cart ?? []);
    setFaceIds(user.faceIds ?? []);
  };

  // 自定义属性不在 Form 内，validateFields 覆盖不到；保存前显式校验，
  // 避免空键/重复键/保留键被静默丢弃或互相覆盖。
  const ensureCustomPropertiesValid = () => {
    const errorKey = validateCustomProperties(customProperties);
    if (errorKey) {
      message.error(t(`resource:user.properties.${errorKey}`));
      return false;
    }
    return true;
  };

  const handleMainSave = () => runPageSave("users");
  // 非表单型分区（身份 / MFA / 会话 / 资产）的保存入口：只提交确实有改动的阶段。
  const handleSectionSave = () => runPageSave(null);
  const handleCredentialsSave = (values: CredentialsFormValues) =>
    runPageSave("credentials", values);

  // ─── Identity handlers ─────────────────────────────────────

  const openAddIdentity = () => setAddIdentityVisible(true);
  const closeAddIdentity = () => {
    setAddIdentityVisible(false);
    setNewIdentity(EMPTY_NEW_IDENTITY);
  };

  const handleAddIdentity = () => {
    if (!newIdentity.providerId || !newIdentity.identityType || !newIdentity.subject) {
      message.warning(t("resource:user.identityRequired"));
      return;
    }
    const updated = [
      ...identities,
      {
        ...newIdentity,
        linkedAt: new Date().toISOString(),
      },
    ];
    setIdentities(updated);
    identityMutation.mutate(updated);
    closeAddIdentity();
  };

  const handleDeleteIdentity = (index: number) => {
    const updated = identities.filter((_, i) => i !== index);
    setIdentities(updated);
    identityMutation.mutate(updated);
  };

  const handleLinkIdentity = async (providerId: string) => {
    try {
      const result = await socialAuthorize({
        providerId,
        organizationSlug: user?.organizationId ?? undefined,
        state: `link:${id}:${providerId}`,
      });
      window.location.href = result.redirectUrl;
    } catch {
      message.error(t("linkIdentityFailed"));
    }
  };

  // ─── Role/Group handlers ───────────────────────────────────

  const openAddRole = () => setAddRoleVisible(true);
  const closeAddRole = () => {
    setAddRoleVisible(false);
    setSelectedRoleId("");
  };

  const openAddGroup = () => setAddGroupVisible(true);
  const closeAddGroup = () => {
    setAddGroupVisible(false);
    setSelectedGroupId("");
  };

  const handleAddRole = async () => {
    if (!selectedRoleId || roleSavePending) return;
    const currentBindings = user?.roleBindings ?? [];
    const alreadyBound = currentBindings.some(
      (rb) => rb.roleId === selectedRoleId || rb.id === selectedRoleId,
    );
    if (alreadyBound) {
      message.warning(t("resource:user.roleAlreadyAssigned", "Role already assigned"));
      return;
    }
    const roleName =
      roleOptions?.find((r) => r.id === selectedRoleId)?.displayName ?? selectedRoleId;
    const updated = [...currentBindings, { roleId: selectedRoleId, roleName }];
    setRoleSavePending(true);
    try {
      await replaceUserRoleBindings(id!, toRoleBindingItems(updated), user?.organizationId);
      message.success(t("updateSuccess"));
      await queryClient.invalidateQueries({ queryKey: ["user"] });
      closeAddRole();
    } catch (error) {
      reportMutationError(error, "common:saveFailed");
    } finally {
      setRoleSavePending(false);
    }
  };

  const handleRemoveRole = (roleId: string) => {
    const updated = (user?.roleBindings ?? []).filter(
      (rb) => rb.roleId !== roleId && rb.id !== roleId,
    );
    replaceUserRoleBindings(id!, toRoleBindingItems(updated), user?.organizationId)
      .then(() => {
        message.success(t("updateSuccess"));
        queryClient.invalidateQueries({ queryKey: ["user"] });
        return undefined;
      })
      .catch((error) => reportMutationError(error, "common:saveFailed"));
  };

  const handleAddGroup = async () => {
    if (!selectedGroupId || groupSavePending) return;
    const currentMemberships = user?.groupMemberships ?? [];
    const alreadyMember = currentMemberships.some(
      (gm) => gm.groupId === selectedGroupId || gm.id === selectedGroupId,
    );
    if (alreadyMember) {
      message.warning(t("resource:user.alreadyGroupMember", "Already a member of this group"));
      return;
    }
    const groupName =
      groupList?.find((g) => g.id === selectedGroupId)?.displayName ?? selectedGroupId;
    const updated = [...currentMemberships, { groupId: selectedGroupId, groupName }];
    setGroupSavePending(true);
    try {
      await replaceUserGroupMemberships(id!, toGroupMembershipItems(updated), user?.organizationId);
      message.success(t("updateSuccess"));
      await queryClient.invalidateQueries({ queryKey: ["user"] });
      // 组详情页缓存 members：用户侧改组必须同时失效 ["groups"]/["group"]。
      await queryClient.invalidateQueries({ queryKey: ["groups"] });
      await queryClient.invalidateQueries({ queryKey: ["group"] });
      closeAddGroup();
    } catch (error) {
      reportMutationError(error, "common:saveFailed");
    } finally {
      setGroupSavePending(false);
    }
  };

  const handleRemoveGroup = (groupId: string) => {
    const updated = (user?.groupMemberships ?? []).filter(
      (gm) => gm.groupId !== groupId && gm.id !== groupId,
    );
    replaceUserGroupMemberships(id!, toGroupMembershipItems(updated), user?.organizationId)
      .then(() => {
        message.success(t("updateSuccess"));
        queryClient.invalidateQueries({ queryKey: ["user"] });
        // 组详情页缓存 members：用户侧改组必须同时失效 ["groups"]/["group"]。
        queryClient.invalidateQueries({ queryKey: ["groups"] });
        queryClient.invalidateQueries({ queryKey: ["group"] });
        return undefined;
      })
      .catch((error) => reportMutationError(error, "common:saveFailed"));
  };

  return {
    // single page form instance (shared with every editable tab)
    form,
    // sub-resource state
    addresses,
    setAddresses,
    managedAccounts,
    setManagedAccounts,
    applicationScopes,
    setApplicationScopes,
    cart,
    setCart,
    faceIds,
    setFaceIds,
    identities,
    customProperties,
    setCustomProperties,
    // identity modal
    addIdentityVisible,
    newIdentity,
    setNewIdentity,
    openAddIdentity,
    closeAddIdentity,
    // role/group modals
    addRoleVisible,
    selectedRoleId,
    setSelectedRoleId,
    roleSavePending,
    openAddRole,
    closeAddRole,
    addGroupVisible,
    selectedGroupId,
    setSelectedGroupId,
    groupSavePending,
    openAddGroup,
    closeAddGroup,
    // mutations
    saveAllMutation,
    verifyIdMutation,
    // hosted IDV challenge
    idvQrVisible,
    idvQrUrl: idvChallenge?.redirectUrl ?? "",
    idvCollecting,
    closeIdvQr,
    deleteMutation,
    mfaMutation,
    preferredMfaMutation,
    recognizeDocumentMutation,
    rotateSecretMutation,
    rotatedSecret,
    dismissRotatedSecret,
    // handlers
    handleReset,
    handleMainSave,
    handleSectionSave,
    handleCredentialsSave,
    handleAddIdentity,
    handleDeleteIdentity,
    handleLinkIdentity,
    handleAddRole,
    handleRemoveRole,
    handleAddGroup,
    handleRemoveGroup,
  };
}

export type UserDetailPageActions = ReturnType<typeof useUserDetailPageActions>;
