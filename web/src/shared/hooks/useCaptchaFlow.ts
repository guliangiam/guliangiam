import { useCallback, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  getCaptcha,
  getCaptchaStatus,
  getCaptchaClientSecret,
  type CaptchaState,
} from "@/shared/lib/auth-captcha-shim";
import { isPasswordMethod, type LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { ApplicationItem } from "@/shared/types/domain";

type ApplicationProviderListItem = NonNullable<ApplicationItem["providers"]>[number];
type ApplicationProviderItem = Extract<ApplicationProviderListItem, { provider?: unknown }>;

// Re-exported from the shim so the captcha payload has exactly one definition.
export { getCaptchaClientSecret, type CaptchaState };

type UseCaptchaFlowValues = {
  application: ApplicationItem | null;
  loginMethod: LoginMethodKey;
  watchedOrganization: string;
  watchedUsername: string;
  watchedApplication: string;
  resetCaptchaToken: () => void;
};

function isProviderConfigItem(
  providerItem: ApplicationProviderListItem,
): providerItem is ApplicationProviderItem {
  return typeof providerItem === "object" && providerItem !== null && "provider" in providerItem;
}

function getCaptchaProviderItems(application: ApplicationItem | null) {
  return (application?.providers ?? [])
    .filter(isProviderConfigItem)
    .filter((providerItem) => providerItem.provider?.category === "Captcha");
}

export function getCaptchaRule(application: ApplicationItem | null) {
  const captchaProviderItems = getCaptchaProviderItems(application);
  if (captchaProviderItems.some((providerItem) => providerItem.rule === "Always")) {
    return "Always" as const;
  }
  if (captchaProviderItems.some((providerItem) => providerItem.rule === "Dynamic")) {
    return "Dynamic" as const;
  }
  if (captchaProviderItems.some((providerItem) => providerItem.rule === "Internet-Only")) {
    return "Internet-Only" as const;
  }
  return "Never" as const;
}

export function useCaptchaFlow(values: UseCaptchaFlowValues) {
  const {
    application,
    loginMethod,
    resetCaptchaToken,
    watchedApplication,
    watchedOrganization,
    watchedUsername,
  } = values;
  const [captchaState, setCaptchaState] = useState<CaptchaState | null>(null);
  const [captchaReloadSeed, setCaptchaReloadSeed] = useState(0);

  const captchaRule = getCaptchaRule(application);

  const captchaStatusQuery = useQuery({
    queryKey: [
      "auth",
      "captcha-status",
      watchedOrganization,
      watchedUsername,
      watchedApplication,
      loginMethod,
      captchaRule,
    ],
    enabled:
      captchaRule !== "Never" &&
      isPasswordMethod(loginMethod) &&
      Boolean(watchedOrganization && watchedUsername && watchedApplication),
    queryFn: async () => getCaptchaStatus({ organizationId: watchedOrganization }),
  });

  // Derived during render: the rule and the server-side risk result fully
  // determine whether a captcha is required, so nothing needs to be mirrored
  // into state from an effect.
  const captchaProviderType = captchaStatusQuery.data?.providerType ?? undefined;
  const captchaSiteKey = captchaStatusQuery.data?.siteKey ?? undefined;
  const captchaRequired =
    captchaRule === "Always" ||
    ((captchaRule === "Dynamic" || captchaRule === "Internet-Only") &&
      captchaStatusQuery.data?.required === true);

  // The challenge document carries the id/image/type; the status document
  // carries the public site key a third-party widget renders with. Merge both
  // so the widget has everything it needs in one state object.
  const mergeStatus = useCallback(
    (captcha: Awaited<ReturnType<typeof getCaptcha>>) => ({
      ...captcha,
      type: captcha.type ?? captchaProviderType ?? null,
      clientId: captchaSiteKey ?? null,
    }),
    [captchaProviderType, captchaSiteKey],
  );

  useEffect(() => {
    async function loadCaptcha() {
      if (!captchaRequired || !application?.id) {
        setCaptchaState(null);
        setCaptchaReloadSeed(0);
        resetCaptchaToken();
        return;
      }

      const captcha = await getCaptcha({ organizationId: watchedOrganization });
      setCaptchaState(mergeStatus(captcha));
      setCaptchaReloadSeed((seed) => seed + 1);
      resetCaptchaToken();
    }

    void loadCaptcha();
  }, [application, captchaRequired, mergeStatus, resetCaptchaToken, watchedOrganization]);

  const reloadCaptcha = useCallback(async () => {
    if (!application?.id || !captchaRequired) {
      return;
    }

    const captcha = await getCaptcha({ organizationId: watchedOrganization });
    setCaptchaState(mergeStatus(captcha));
    setCaptchaReloadSeed((seed) => seed + 1);
    resetCaptchaToken();
  }, [application, captchaRequired, mergeStatus, resetCaptchaToken, watchedOrganization]);

  return {
    captchaRule,
    captchaState,
    captchaRequired,
    captchaReloadSeed,
    reloadCaptcha,
  };
}
