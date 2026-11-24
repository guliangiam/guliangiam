import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { App } from "antd";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  getAffiliationPromptConfig,
  getPromptedProviderItems,
  getPromptedSignupItems,
  hasPromptPage,
  isAffiliationPrompted,
  isPromptAnswered,
} from "@/shared/lib/auth-prompt";
import {
  loadPromptAddressOptions,
  loadPromptAffiliationOptions,
  loadPromptApplication,
  loadPromptSession,
  type PromptAddressOption,
  type PromptAffiliationOption,
} from "@/shared/lib/auth-prompt-data";
import {
  getPromptUserSignature,
  shouldApplyIncomingPromptUser,
  submitPromptResponse,
} from "@/shared/lib/auth-prompt-submit";
import { loadPromptUserStateDetails } from "@/shared/lib/auth-prompt-state";
import { resolveAuthApplicationName } from "@/shared/lib/auth-route-context";
import { startDirectProviderSignin } from "@/shared/lib/auth-provider-signin";
import type { ApplicationItem, UserDetailItem } from "@/shared/types/domain";
import { localizedError } from "@/shared/lib/server-error";

type PromptedProviderItem = ReturnType<typeof getPromptedProviderItems>[number];

type PromptPageBaseState = {
  applicationName: string;
};

type PromptPageLoadingState = PromptPageBaseState & {
  status: "loading";
};

type PromptPageErrorState = PromptPageBaseState & {
  status: "session-error" | "application-error" | "user-error";
  errorTitle: string;
};

export type PromptPageReadyState = PromptPageBaseState & {
  status: "ready";
  application: ApplicationItem;
  draftUser: UserDetailItem;
  promptedProviderItems: PromptedProviderItem[];
  shouldPromptAffiliation: boolean;
  shouldPromptRegion: boolean;
  addressOptions: PromptAddressOption[];
  addressOptionsLoading: boolean;
  affiliationOptions: PromptAffiliationOption[];
  affiliationOptionsLoading: boolean;
  hasAffiliationOptionsError: boolean;
  shouldUseStructuredAffiliationInputs: boolean;
  canSubmit: boolean;
  isSubmitting: boolean;
  isLinkingProvider: string | null;
  handleLinkProvider: (providerItem: PromptedProviderItem) => Promise<void>;
  handleSubmit: () => Promise<void>;
  updateDraftUser: (fields: Partial<UserDetailItem>) => void;
};

export type PromptPageControllerState =
  | PromptPageLoadingState
  | PromptPageErrorState
  | PromptPageReadyState;

export function usePromptPageController(): PromptPageControllerState {
  const { message } = App.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const params = useParams<{ applicationName?: string }>();
  const applicationName = resolveAuthApplicationName({
    routeApplicationName: params.applicationName,
    search: location.search,
  });
  const [draftUser, setDraftUser] = useState<UserDetailItem | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isLinkingProvider, setIsLinkingProvider] = useState<string | null>(null);
  const lastAppliedPromptUserSignatureRef = useRef<string | null>(null);
  const { t } = useTranslation();

  const sessionQuery = useQuery({
    queryKey: ["auth", "prompt", "session"],
    queryFn: loadPromptSession,
  });

  const applicationQuery = useQuery({
    queryKey: ["auth", "prompt", "application", applicationName],
    enabled: Boolean(applicationName),
    queryFn: () => loadPromptApplication(applicationName),
  });

  const application = applicationQuery.data ?? null;
  const promptedProviderItems = useMemo(() => getPromptedProviderItems(application), [application]);
  const promptedSignupItems = useMemo(() => getPromptedSignupItems(application), [application]);
  const affiliationPromptConfig = useMemo(
    () => getAffiliationPromptConfig(application),
    [application],
  );
  const shouldPromptAffiliation = isAffiliationPrompted(application);
  const shouldPromptRegion = promptedSignupItems.some((item) => item.name === "Country/Region");
  const organizationSlug =
    sessionQuery.data?.organization?.name ?? sessionQuery.data?.account?.owner ?? "";

  const promptUserStateQuery = useQuery({
    queryKey: [
      "auth",
      "prompt",
      "state",
      applicationName,
      sessionQuery.data?.account?.owner,
      sessionQuery.data?.account?.name,
      organizationSlug,
    ],
    enabled: Boolean(
      application && sessionQuery.data?.account?.owner && sessionQuery.data?.account?.name,
    ),
    queryFn: () =>
      loadPromptUserStateDetails({
        application,
        owner: sessionQuery.data!.account.owner,
        username: sessionQuery.data!.account.name,
        organizationSlug,
      }),
  });

  useEffect(() => {
    const incomingPromptUser = promptUserStateQuery.data?.promptUser ?? null;
    if (!incomingPromptUser) {
      return;
    }

    if (
      !shouldApplyIncomingPromptUser({
        incomingPromptUser,
        draftUser,
        lastAppliedSignature: lastAppliedPromptUserSignatureRef.current,
      })
    ) {
      return;
    }

    setDraftUser(incomingPromptUser);
    lastAppliedPromptUserSignatureRef.current = getPromptUserSignature(incomingPromptUser);
  }, [draftUser, promptUserStateQuery.data?.promptUser]);

  const addressCode = Array.isArray(draftUser?.address)
    ? draftUser.address[draftUser.address.length - 1]
    : "";

  const addressOptionsQuery = useQuery<PromptAddressOption[]>({
    queryKey: ["auth", "prompt", "address-options", affiliationPromptConfig.addressUrl],
    enabled: shouldPromptAffiliation && affiliationPromptConfig.addressUrl !== "",
    queryFn: () => loadPromptAddressOptions(application),
  });

  const affiliationOptionsQuery = useQuery<PromptAffiliationOption[]>({
    queryKey: [
      "auth",
      "prompt",
      "affiliation-options",
      affiliationPromptConfig.optionUrl,
      addressCode,
    ],
    enabled:
      shouldPromptAffiliation && affiliationPromptConfig.optionUrl !== "" && addressCode !== "",
    queryFn: () => loadPromptAffiliationOptions({ application, addressCode }),
  });

  const addressOptions = addressOptionsQuery.data ?? [];
  const affiliationOptions = affiliationOptionsQuery.data ?? [];
  const hasAffiliationOptionsError =
    addressOptionsQuery.isError || (addressCode !== "" && affiliationOptionsQuery.isError);
  const shouldUseStructuredAffiliationInputs =
    affiliationPromptConfig.hasStructuredOptions && !hasAffiliationOptionsError;
  const canSubmit = isPromptAnswered(draftUser, application);
  const isUserLoading = promptUserStateQuery.isLoading;
  const shouldUseRegionSubmit = Boolean(shouldPromptRegion && promptUserStateQuery.data?.user?.id);

  function updateDraftUser(fields: Partial<UserDetailItem>) {
    setDraftUser((current) => {
      if (!current) {
        return current;
      }

      return {
        ...current,
        ...fields,
      };
    });
  }

  async function handleLinkProvider(providerItem: PromptedProviderItem) {
    if (!application || !providerItem.provider) {
      return;
    }

    setIsLinkingProvider(providerItem.provider.name ?? providerItem.provider.type ?? "link");
    try {
      await startDirectProviderSignin({
        application,
        provider: providerItem.provider,
        method: "link",
        unsupportedMessage: t("prompt.providerNotSupportedForLinking"),
        missingUrlMessage: t("prompt.providerMissingLinkingUrl"),
      });
    } catch (error) {
      message.error(localizedError(error, t, "prompt.providerLinkingFailed"));
      setIsLinkingProvider(null);
    }
  }

  async function handleSubmit() {
    if (!draftUser || !application) {
      return;
    }

    setIsSubmitting(true);
    try {
      await submitPromptResponse({
        application,
        draftUser,
        fallbackOwner: sessionQuery.data?.account.owner,
        shouldUseRegionSubmit,
        regionUserId: promptUserStateQuery.data?.user?.id,
        shouldPromptRegion,
        search: location.search,
        navigate,
        showSuccess: message.success,
      });
    } catch (error) {
      message.error(localizedError(error, t, "prompt.profileSaveFailed"));
    } finally {
      setIsSubmitting(false);
    }
  }

  if (sessionQuery.isLoading || applicationQuery.isLoading || isUserLoading) {
    return {
      applicationName,
      status: "loading",
    };
  }

  if (sessionQuery.isError || !sessionQuery.data?.account) {
    return {
      applicationName,
      status: "session-error",
      errorTitle: t("prompt.sessionInvalid"),
    };
  }

  if (applicationQuery.isError || !application || !hasPromptPage(application)) {
    return {
      applicationName,
      status: "application-error",
      errorTitle: t("prompt.applicationNotPromptable"),
    };
  }

  if (!draftUser) {
    return {
      applicationName,
      status: "user-error",
      errorTitle: t("prompt.userProfileNotLoaded"),
    };
  }

  return {
    applicationName,
    status: "ready",
    application,
    draftUser,
    promptedProviderItems,
    shouldPromptAffiliation,
    shouldPromptRegion,
    addressOptions,
    addressOptionsLoading: addressOptionsQuery.isLoading,
    affiliationOptions,
    affiliationOptionsLoading: affiliationOptionsQuery.isLoading,
    hasAffiliationOptionsError,
    shouldUseStructuredAffiliationInputs,
    canSubmit,
    isSubmitting,
    isLinkingProvider,
    handleLinkProvider,
    handleSubmit,
    updateDraftUser,
  };
}
