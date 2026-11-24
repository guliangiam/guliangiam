import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  enableUserMfaSetup,
  getUser,
  getUserOptions,
  initiateUserMfaSetup,
  verifyUserMfaSetup,
} from "@/shared/api/iam/user";
import { getOrganizationIdentityBySlug } from "@/shared/api/iam/organization";
import {
  buildCompletedMfaSetupLoginUrl,
  consumePendingMfaSetup,
  storePendingMfaSetup,
} from "@/shared/lib/auth-mfa-setup";
import { normalizeIsoCountryCode, toE164 } from "@/shared/ui/country-data";
import type { MfaPropsItem } from "@/shared/types/domain";
import { queryKeys } from "@/shared/api/query-keys";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

export type SetupFormValues = {
  owner: string;
  name: string;
  mfaType: string;
  passcode?: string;
  dest?: string;
  countryCode?: string;
  rememberInHours?: number;
};

export type InitiatedState = {
  secret?: string;
  url?: string;
  recoveryCodes?: string[];
  countryCode?: string;
};

export const MFA_TYPE_VALUES = ["app", "email", "sms", "radius", "push"] as const;

export function useMfaSetupPageController() {
  const { message } = App.useApp();
  const { t } = useTranslation("auth");
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm<SetupFormValues>();
  const [initiated, setInitiated] = useState<InitiatedState | null>(null);
  const [verifiedRecoveryCodes, setVerifiedRecoveryCodes] = useState<string[]>([]);
  const [showFullSecret, setShowFullSecret] = useState(false);
  const [showFullInitiatedSecret, setShowFullInitiatedSecret] = useState(false);

  const owner = searchParams.get("owner") ?? "";
  const name = searchParams.get("name") ?? "";
  const mfaTypeFromQuery = searchParams.get("mfaType") ?? "app";
  const returnTo = searchParams.get("returnTo") ?? "";
  const mfaType = Form.useWatch("mfaType", form) ?? mfaTypeFromQuery;

  const organizationIdentityQuery = useQuery({
    queryKey: ["organization", "auth", "identity", owner],
    queryFn: () => getOrganizationIdentityBySlug(owner),
    enabled: owner !== "",
  });

  const userIdentityQuery = useQuery({
    queryKey: ["user", "auth", "identity", organizationIdentityQuery.data?.id ?? "", name],
    queryFn: async () => {
      // 服务端 keyword 过滤精确定位，避免组织用户超过 50 条时找不到目标。
      const users = await getUserOptions(organizationIdentityQuery.data!.id, name);
      return users.find((user) => user.username === name) ?? null;
    },
    enabled:
      name !== "" &&
      organizationIdentityQuery.data !== null &&
      organizationIdentityQuery.data !== undefined,
  });

  const detailQuery = useQuery({
    queryKey: ["user", "auth", userIdentityQuery.data?.id ?? ""],
    queryFn: () => getUser(userIdentityQuery.data!.id, organizationIdentityQuery.data!.id),
    enabled: userIdentityQuery.data !== null && userIdentityQuery.data !== undefined,
  });

  const currentMfa = useMemo<MfaPropsItem | null>(() => {
    if (!detailQuery.data) return null;
    return (detailQuery.data.mfaSetupSummary.find((item) => item.mfaType === mfaType) ??
      null) as MfaPropsItem | null;
  }, [detailQuery.data, mfaType]);

  useEffect(() => {
    form.setFieldsValue({
      owner,
      name,
      mfaType: mfaTypeFromQuery,
      // The form stores ISO alpha-2; legacy dial values (`+86`/`86`) stay empty
      // so the admin re-selects a country rather than being guessed one.
      countryCode:
        normalizeIsoCountryCode(
          typeof detailQuery.data?.profile.countryCode === "string"
            ? detailQuery.data.profile.countryCode
            : "",
        ) ||
        normalizeIsoCountryCode(currentMfa?.countryCode) ||
        "",
      rememberInHours: Number(currentMfa?.mfaRememberInHours ?? 0),
    });
  }, [
    detailQuery.data?.profile.countryCode,
    currentMfa?.countryCode,
    currentMfa?.mfaRememberInHours,
    form,
    mfaTypeFromQuery,
    name,
    owner,
  ]);

  const resetSetupState = () => {
    setInitiated(null);
    setVerifiedRecoveryCodes([]);
  };

  const initiateMutation = useMutation({
    mutationFn: async (values: SetupFormValues) => {
      if (!userIdentityQuery.data) throw new Error(tr("mfa.userNotResolved"));
      return initiateUserMfaSetup(
        userIdentityQuery.data.id,
        { mfaType: values.mfaType },
        organizationIdentityQuery.data?.id,
      );
    },
    onSuccess: (result) => {
      setInitiated({
        secret: result.secret ?? "",
        url: result.url ?? "",
        recoveryCodes: result.recoveryCodes ?? [],
        countryCode:
          normalizeIsoCountryCode(result.countryCode) ||
          normalizeIsoCountryCode(form.getFieldValue("countryCode")),
      });
      setVerifiedRecoveryCodes([]);
      message.success(tr("mfa.initiated"));
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<SetupFormValues>(form), tr);
      }
      message.error(localizedError(error, tr, "mfa.initFailed"));
    },
  });

  const smsDestination = (values: SetupFormValues) => {
    const countryCode = normalizeIsoCountryCode(values.countryCode);
    const dest =
      values.mfaType === "sms" ? toE164(countryCode, values.dest ?? "") : (values.dest ?? "");
    if (values.mfaType === "sms" && !dest) {
      throw new Error(tc("form.invalidPhone"));
    }
    return { countryCode, dest };
  };

  const verifyMutation = useMutation({
    mutationFn: async (values: SetupFormValues) => {
      if (!userIdentityQuery.data) throw new Error(tr("mfa.userNotResolved"));
      const { countryCode, dest } = smsDestination(values);
      return verifyUserMfaSetup(
        userIdentityQuery.data.id,
        {
          mfaType: values.mfaType,
          passcode: values.passcode ?? "",
          secret: initiated?.secret ?? "",
          dest,
          countryCode,
        },
        organizationIdentityQuery.data?.id,
      );
    },
    onSuccess: () => {
      setVerifiedRecoveryCodes(initiated?.recoveryCodes ?? []);
      message.success(tr("mfa.verifySuccess"));
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<SetupFormValues>(form), tr);
      }
      message.error(localizedError(error, tr, "mfa.verifyFailed"));
    },
  });

  const enableMutation = useMutation({
    mutationFn: async (values: SetupFormValues) => {
      if (!userIdentityQuery.data) throw new Error(tr("mfa.userNotResolved"));
      const { countryCode, dest } = smsDestination(values);
      return enableUserMfaSetup(
        userIdentityQuery.data.id,
        {
          mfaType: values.mfaType,
          secret: initiated?.secret ?? "",
          recoveryCodes: verifiedRecoveryCodes,
          passcode: values.passcode ?? "",
          dest,
          countryCode,
        },
        organizationIdentityQuery.data?.id,
      );
    },
    onSuccess: async (_, values) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.session.all });
      await queryClient.invalidateQueries({
        queryKey: ["user", "auth", userIdentityQuery.data?.id ?? ""],
      });
      await queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
      message.success(tr("mfa.enabled"));

      const pendingSetup = consumePendingMfaSetup({ owner, username: name });
      if (pendingSetup) {
        storePendingMfaSetup({
          ...pendingSetup,
          target: values.dest ?? pendingSetup.target ?? "",
          countryCode:
            normalizeIsoCountryCode(values.countryCode) || pendingSetup.countryCode || "",
        });
        navigate(buildCompletedMfaSetupLoginUrl(pendingSetup), { replace: true });
        return;
      }
      if (returnTo && returnTo.startsWith("/") && !returnTo.startsWith("//")) {
        navigate(returnTo, { replace: true });
        return;
      }
      navigate(owner && name ? `/users/${owner}/${name}` : "/", { replace: true });
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<SetupFormValues>(form), tr);
      }
      message.error(localizedError(error, tr, "mfa.enableFailed"));
    },
  });

  const MFA_TYPE_OPTIONS = MFA_TYPE_VALUES.map((value) => {
    const keyMap: Record<string, string> = {
      app: "mfaSetup.typeAuthenticatorApp",
      email: "mfaSetup.typeEmail",
      sms: "mfaSetup.typeSms",
      radius: "mfaSetup.typeRadius",
      push: "mfaSetup.typePush",
    };
    return { label: t(keyMap[value]), value };
  });

  const currentStep = verifiedRecoveryCodes.length > 0 ? 2 : initiated ? 1 : 0;
  const showDestField =
    mfaType === "email" || mfaType === "sms" || mfaType === "radius" || mfaType === "push";
  const showCountryCodeField = mfaType === "sms";
  const showSecretInfo = mfaType === "app" || Boolean(initiated?.secret) || Boolean(initiated?.url);

  return {
    owner,
    name,
    returnTo,
    t,
    tc,
    tr,
    navigate,
    form,
    // State
    initiated,
    setInitiated,
    verifiedRecoveryCodes,
    setVerifiedRecoveryCodes,
    showFullSecret,
    setShowFullSecret,
    showFullInitiatedSecret,
    setShowFullInitiatedSecret,
    // Queries
    organizationIdentityQuery,
    userIdentityQuery,
    detailQuery,
    currentMfa,
    // Mutations
    initiateMutation,
    verifyMutation,
    enableMutation,
    // Options + derived
    mfaType,
    currentStep,
    showDestField,
    showCountryCodeField,
    showSecretInfo,
    MFA_TYPE_OPTIONS,
    resetSetupState,
  };
}
