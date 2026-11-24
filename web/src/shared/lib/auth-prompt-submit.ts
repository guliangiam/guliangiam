import { updateUserProfile } from "@/shared/api/iam/user";
import { submitPromptUpdate } from "@/shared/lib/auth-prompt-data";
import { loadStoredSigninUrl, resolvePromptRedirectUrl } from "@/shared/lib/auth-signin-url";
import type { ApplicationItem, UserDetailItem } from "@/shared/types/domain";
import { t } from "i18next";

function getStringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

export function getPromptUserSignature(user: UserDetailItem | null) {
  return JSON.stringify(user ?? null);
}

export function shouldApplyIncomingPromptUser(values: {
  incomingPromptUser: UserDetailItem | null;
  draftUser: UserDetailItem | null;
  lastAppliedSignature: string | null;
}) {
  if (!values.incomingPromptUser) {
    return false;
  }

  const incomingSignature = getPromptUserSignature(values.incomingPromptUser);
  const currentDraftSignature = getPromptUserSignature(values.draftUser);

  return (
    values.draftUser === null ||
    incomingSignature !== values.lastAppliedSignature ||
    currentDraftSignature === values.lastAppliedSignature
  );
}

export async function submitPromptResponse(values: {
  application: ApplicationItem;
  draftUser: UserDetailItem;
  fallbackOwner?: string;
  shouldUseRegionSubmit: boolean;
  regionUserId?: string | null;
  shouldPromptRegion: boolean;
  search: string;
  navigate: (to: string, options?: { replace?: boolean }) => void;
  showSuccess: (message: string) => void;
  updateUserProfileImpl?: typeof updateUserProfile;
  submitPromptUpdateImpl?: typeof submitPromptUpdate;
  loadStoredSigninUrlImpl?: typeof loadStoredSigninUrl;
  resolvePromptRedirectUrlImpl?: typeof resolvePromptRedirectUrl;
  windowObject?: Window;
}) {
  const updateUserProfileImpl = values.updateUserProfileImpl ?? updateUserProfile;

  if (values.shouldUseRegionSubmit && values.regionUserId) {
    await updateUserProfileImpl(values.regionUserId, {
      region: getStringValue(values.draftUser.region),
    });
  }

  values.showSuccess(t("prompt.profileSaved"));

  const loadStoredSigninUrlImpl = values.loadStoredSigninUrlImpl ?? loadStoredSigninUrl;
  const resolvePromptRedirectUrlImpl =
    values.resolvePromptRedirectUrlImpl ?? resolvePromptRedirectUrl;
  const redirectUrl = resolvePromptRedirectUrlImpl({
    search: values.search,
    storedSigninUrl: loadStoredSigninUrlImpl(),
  });

  if (redirectUrl) {
    const windowObject = values.windowObject ?? window;
    windowObject.location.assign(redirectUrl);
    return;
  }

  values.navigate(
    `/login?organization=${encodeURIComponent(values.application.organization || "admin")}&application=${encodeURIComponent(values.application.name)}`,
    {
      replace: true,
    },
  );
}
