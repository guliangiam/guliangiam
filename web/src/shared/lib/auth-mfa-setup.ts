import type { MfaPropsItem } from "@/shared/types/domain";

export type PendingMfaSetupContext = {
  organization: string;
  application: string;
  username: string;
  returnTo: string;
  challengeId: string;
  requiredMfa: string[];
  target?: string;
  countryCode?: string;
};

const PENDING_MFA_SETUP_STORAGE_KEY = "pendingMfaSetup";

function toSetupMfaType(mfaType?: string) {
  switch (mfaType) {
    case "totp":
      return "app";
    case "app":
    case "email":
    case "sms":
    case "radius":
    case "push":
      return mfaType;
    default:
      return "app";
  }
}

export function toVerificationMfaType(mfaType?: string) {
  switch (mfaType) {
    case "app":
      return "totp";
    default:
      return mfaType ?? "totp";
  }
}

export function buildMfaSetupUrl(values: {
  owner: string;
  name: string;
  returnTo: string;
  requiredMfa?: string[];
}) {
  const searchParams = new URLSearchParams({
    owner: values.owner,
    name: values.name,
    returnTo: values.returnTo,
  });

  const firstRequiredMfa = values.requiredMfa?.[0];
  if (firstRequiredMfa) {
    searchParams.set("mfaType", toSetupMfaType(firstRequiredMfa));
  }

  return `/mfa/setup?${searchParams.toString()}`;
}

export function buildCompletedMfaSetupLoginUrl(
  context: Pick<PendingMfaSetupContext, "organization" | "application" | "username" | "returnTo">,
) {
  const searchParams = new URLSearchParams({
    organization: context.organization,
    application: context.application,
    username: context.username,
    returnTo: context.returnTo,
    mfaSetup: "completed",
  });

  return `/login?${searchParams.toString()}`;
}

export function createMfaSetupResumeContext(context: PendingMfaSetupContext): {
  loginValues: {
    type: "password";
    signinMethod: "password";
    organization: string;
    username: string;
    password: string;
    application: string;
    challengeId: string;
  };
  mfaProps: MfaPropsItem[];
} {
  return {
    loginValues: {
      type: "password",
      signinMethod: "password",
      organization: context.organization,
      username: context.username,
      password: "",
      application: context.application,
      challengeId: context.challengeId,
    },
    mfaProps:
      context.requiredMfa.length === 0
        ? [{ mfaType: "totp", isPreferred: true }]
        : context.requiredMfa.map((item, index) => ({
            mfaType: toVerificationMfaType(item),
            isPreferred: index === 0,
          })),
  };
}

export function storePendingMfaSetup(context: PendingMfaSetupContext) {
  if (typeof window === "undefined") {
    return;
  }

  window.sessionStorage.setItem(PENDING_MFA_SETUP_STORAGE_KEY, JSON.stringify(context));
}

export function consumePendingMfaSetup(values?: {
  owner?: string;
  username?: string;
}): PendingMfaSetupContext | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = window.sessionStorage.getItem(PENDING_MFA_SETUP_STORAGE_KEY);
  if (!raw) {
    return null;
  }

  window.sessionStorage.removeItem(PENDING_MFA_SETUP_STORAGE_KEY);

  try {
    const parsed = JSON.parse(raw) as Partial<PendingMfaSetupContext>;
    if (
      typeof parsed.organization !== "string" ||
      typeof parsed.application !== "string" ||
      typeof parsed.username !== "string" ||
      typeof parsed.returnTo !== "string" ||
      typeof parsed.challengeId !== "string" ||
      !Array.isArray(parsed.requiredMfa)
    ) {
      return null;
    }

    if (
      (values?.owner && parsed.organization !== values.owner) ||
      (values?.username && parsed.username !== values.username)
    ) {
      return null;
    }

    return {
      organization: parsed.organization,
      application: parsed.application,
      username: parsed.username,
      returnTo: parsed.returnTo,
      challengeId: parsed.challengeId,
      requiredMfa: parsed.requiredMfa.filter((item): item is string => typeof item === "string"),
      target: typeof parsed.target === "string" ? parsed.target : undefined,
      countryCode: typeof parsed.countryCode === "string" ? parsed.countryCode : undefined,
    };
  } catch {
    return null;
  }
}
