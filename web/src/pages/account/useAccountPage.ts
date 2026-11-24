import { useState } from "react";
import { App, Form } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getAuthAccountContext, socialAuthorize } from "@/shared/api/iam/auth";
import { getOrganizationSettings } from "@/shared/api/iam/organization";
import {
  getOwnProfile,
  updateOwnProfile,
  verifyOwnIdentification,
  registerOwnFaceId,
  revokeOwnFaceId,
} from "@/shared/api/iam/auth-selfprofile";
import {
  initiateOwnMfaSetup,
  verifyOwnMfaSetup,
  enableOwnMfaSetup,
  replaceOwnMfaMethods,
  updateOwnPreferredMfa,
  listOwnConsents,
  deleteOwnIdentity,
} from "@/shared/api/iam/auth-selfservice";
import {
  getProviders,
  getSessions,
  getCurrentSession,
  revokeSession,
  revokeOtherSessions,
} from "@/shared/api/iam";
import type { UpdateUserProfilePayload } from "@/shared/api/iam/user-types";
import { queryKeys } from "@/shared/api/query-keys";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
  serverErrorMessage,
} from "@/shared/lib/server-error";
import { buildIdvReturnUrl, CloudAuthMetaError, collectMetaInfo } from "@/shared/lib/cloudauth-sdk";
import { idvPendingStorageKey, useIdvPendingPolling } from "@/shared/hooks/useIdvPendingPolling";

export function useAccountPage() {
  const { message } = App.useApp();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const queryClient = useQueryClient();
  const [profileForm] = Form.useForm();
  const [realNameForm] = Form.useForm();
  // SDK 注入 + metaInfo 采集是发起实名认证的前置阶段，按钮据此改文案。
  const [idvCollecting, setIdvCollecting] = useState(false);

  const accountQuery = useQuery({
    queryKey: ["auth", "account"],
    queryFn: () => getAuthAccountContext(),
  });

  const account = accountQuery.data;
  const userId = account?.account?.id;

  const { data: user } = useQuery({
    queryKey: ["user", userId],
    queryFn: () => getOwnProfile(),
    enabled: !!userId,
  });

  const { data: providers } = useQuery({
    queryKey: queryKeys.providers.all,
    queryFn: () => getProviders(),
  });

  // 组织级账户项规则（accountItems）：读取失败（如普通用户无 organizations:read）时
  // 保持空数组，useAccountItems 对未定义字段默认全部可见可编辑（fail-open）。
  const orgId = account?.organization?.id;
  const { data: orgSettings } = useQuery({
    queryKey: ["organization-settings", orgId],
    queryFn: () => getOrganizationSettings(orgId!),
    enabled: !!orgId,
    retry: false,
  });
  const accountItems = orgSettings?.accountItems ?? [];

  const sessionsQuery = useQuery({
    queryKey: ["sessions", "iam"],
    queryFn: () => getSessions(),
  });

  const currentSessionQuery = useQuery({
    queryKey: ["session", "current"],
    queryFn: () => getCurrentSession(),
  });

  const mfaMethods = user?.mfaMethods ?? [];
  const preferredMfa = user?.preferredMfaType;
  const identities = user?.identities ?? [];
  const faceIds = user?.faceIds ?? [];

  const consentsQuery = useQuery({
    queryKey: ["auth", "consents"],
    queryFn: () => listOwnConsents(),
  });

  // After the hosted provider flow redirects back, resume the pending
  // verification: the shared hook polls the read endpoint (which refreshes from
  // the provider) until it reaches a terminal state, then reloads the user so
  // isVerified reflects the provider's decision.
  useIdvPendingPolling({ userId, organizationId: user?.organizationId });

  const updateProfileMutation = useMutation({
    mutationFn: async (values: UpdateUserProfilePayload) => {
      if (!userId || !user) throw new Error(t("account.userNotLoaded"));
      // PATCH /auth/profile 为部分合并语义：只提交本段表单字段，服务端保留其余
      // 字段，无需客户端全量合并。email/phone 不在自助面开放（须走 /auth/account
      // 验证码绑定），表单也不编辑它们，故不会被提交。
      return updateOwnProfile(values);
    },
    onSuccess: () => {
      message.success(t("account.profileUpdated"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      if (isHttpError(error)) {
        // 同一 mutation 服务于 profileForm 与 realNameForm 两张表单；
        // 字段名互不重叠，解析器只会标红各自已注册的字段。
        applyServerFieldErrors(profileForm, error, buildServerFieldResolver(profileForm), t);
        applyServerFieldErrors(realNameForm, error, buildServerFieldResolver(realNameForm), t);
      }
      message.error(localizedError(error, t, "account.updateFailed"));
    },
  });

  const verifyIdMutation = useMutation({
    mutationFn: async () => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      // 后端对 metaInfo / returnUrl 双 fail-closed（缺失即 400）：先采集 SDK 的真实
      // metaInfo 再发请求，采集失败在本地拦截，不会退回空 body。
      setIdvCollecting(true);
      try {
        const metaInfo = await collectMetaInfo();
        return await verifyOwnIdentification({
          metaInfo,
          returnUrl: buildIdvReturnUrl("/account"),
        });
      } finally {
        setIdvCollecting(false);
      }
    },
    onSuccess: (ticket) => {
      const redirectUrl = ticket?.data?.redirectUrl;
      if (redirectUrl) {
        const verificationId = ticket?.data?.verificationId;
        if (verificationId && userId) {
          window.localStorage.setItem(idvPendingStorageKey(userId), verificationId);
        }
        // 自服务是本人操作：直接跳转厂商托管页，手机 H5 可原地完成刷脸。
        message.info(tc("verifyIdRedirecting"));
        window.location.assign(redirectUrl);
        return;
      }
      if (ticket && ticket.status !== "verified") {
        message.success(tc("verifyIdPending"));
      } else {
        message.success(tc("verifyIdSuccess"));
      }
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      // SDK 侧失败（加载超时/全局缺失）不是后端拒绝，通用「验证失败」会诱导用户重试。
      if (error instanceof CloudAuthMetaError) {
        message.error(tc("verifyIdMetaFailed"));
        return;
      }
      message.error(serverErrorMessage(error, tc, "verifyIdFailed"));
    },
  });

  const mfaSetupMutation = useMutation({
    mutationFn: async (mfaType: string) => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      return initiateOwnMfaSetup({ mfaType });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "account.mfaSetupFailed"));
    },
  });

  const mfaVerifyMutation = useMutation({
    mutationFn: async (params: { mfaType: string; passcode: string; secret?: string }) => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      return verifyOwnMfaSetup({
        mfaType: params.mfaType,
        passcode: params.passcode,
        secret: params.secret,
      });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "account.mfaVerifyFailed"));
    },
  });

  const mfaEnableMutation = useMutation({
    mutationFn: async (params: {
      mfaType: string;
      secret?: string;
      passcode: string;
      recoveryCodes: string[];
    }) => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      return enableOwnMfaSetup({
        mfaType: params.mfaType,
        secret: params.secret,
        passcode: params.passcode,
        recoveryCodes: params.recoveryCodes,
      });
    },
    onSuccess: () => {
      message.success(t("account.mfaEnabled"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "account.mfaEnableFailed"));
    },
  });

  const mfaDisableMutation = useMutation({
    mutationFn: async (methodType: string) => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      const remaining = mfaMethods
        .filter((m) => m.methodType !== methodType)
        .map((m) => ({
          methodType: m.methodType,
          enabled: m.enabled,
          displayName: m.displayName,
        }));
      return replaceOwnMfaMethods(user?.revision ?? 0, remaining);
    },
    onSuccess: () => {
      message.success(t("account.mfaDisabled"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
  });

  const setPreferredMfaMutation = useMutation({
    mutationFn: async (mfaType: string) => {
      if (!userId) throw new Error(t("account.userNotLoaded"));
      return updateOwnPreferredMfa({ mfaType });
    },
    onSuccess: () => {
      message.success(t("account.mfaPreferredSet"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
  });

  // ─────────────────────────────────────────────
  // Self-service FaceID enroll / revoke（会话绑定，无 userId/orgId 参数）
  // ─────────────────────────────────────────────

  const registerFaceIdMutation = useMutation({
    mutationFn: (params: { file: File; providerId?: string }) =>
      registerOwnFaceId({
        file: params.file,
        providerId: params.providerId,
        deviceInfo: typeof navigator === "undefined" ? undefined : navigator.userAgent,
      }),
    onSuccess: () => {
      message.success(t("user.faceIdRegistered"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "user.faceIdRegisterFailed"));
    },
  });

  const revokeFaceIdMutation = useMutation({
    mutationFn: () => revokeOwnFaceId(),
    onSuccess: () => {
      message.success(t("user.faceIdRevoked"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "user.faceIdRevokeFailed"));
    },
  });

  // ─────────────────────────────────────────────
  // Self-service third-party identity unlink / link
  // ─────────────────────────────────────────────

  const deleteIdentityMutation = useMutation({
    mutationFn: (identityId: string) => deleteOwnIdentity(identityId),
    onSuccess: () => {
      message.success(t("account.identityUnlinked"));
      void queryClient.invalidateQueries({ queryKey: ["user", userId] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "account.identityUnlinkFailed"));
    },
  });

  // 绑定走托管认证重定向：organizationSlug 必须是组织 slug（account.organization.name），
  // state 携带 link:{userId}:{providerId} 供回调收尾时识别绑定意图。
  const linkIdentity = async (providerId: string) => {
    try {
      const result = await socialAuthorize({
        providerId,
        organizationSlug: account?.organization?.name,
        state: `link:${userId}:${providerId}`,
      });
      window.location.href = result.redirectUrl;
    } catch (error) {
      message.error(localizedError(error, t, "account.linkIdentityFailed"));
    }
  };

  const revokeSessionMutation = useMutation({
    mutationFn: (sessionId: string) => revokeSession(sessionId),
    onSuccess: () => {
      message.success(t("account.sessionRevoked"));
      void queryClient.invalidateQueries({ queryKey: ["sessions", "iam"] });
    },
  });

  const revokeOthersMutation = useMutation({
    mutationFn: () => revokeOtherSessions(),
    onSuccess: (data) => {
      message.success(t("account.sessionsRevoked", { count: data?.revokedCount ?? 0 }));
      void queryClient.invalidateQueries({ queryKey: ["sessions", "iam"] });
    },
  });

  return {
    accountQuery,
    account,
    userId,
    user,
    accountItems,
    providers,
    sessionsQuery,
    currentSessionQuery,
    mfaMethods,
    preferredMfa,
    identities,
    faceIds,
    consentsQuery,
    profileForm,
    realNameForm,
    idvCollecting,
    updateProfileMutation,
    verifyIdMutation,
    mfaSetupMutation,
    mfaVerifyMutation,
    mfaEnableMutation,
    mfaDisableMutation,
    setPreferredMfaMutation,
    registerFaceIdMutation,
    revokeFaceIdMutation,
    deleteIdentityMutation,
    linkIdentity,
    revokeSessionMutation,
    revokeOthersMutation,
  };
}
