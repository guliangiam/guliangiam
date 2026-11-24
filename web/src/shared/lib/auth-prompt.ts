import type { PromptUser } from "@/shared/api/iam/user";
import type { ApplicationItem, UserDetailItem } from "@/shared/types/domain";

type ProviderConfigItem = Exclude<NonNullable<ApplicationItem["providers"]>[number], string>;
type SignupItemConfig = NonNullable<ApplicationItem["signupItems"]>[number];

function isProviderVisible(providerItem: ProviderConfigItem) {
  if (!providerItem.provider) {
    return false;
  }

  return (
    ["OAuth", "SAML", "Web3"].includes(providerItem.provider.category ?? "") &&
    providerItem.provider.type !== "WeChatMiniProgram"
  );
}

export function isProviderPrompted(providerItem: ProviderConfigItem) {
  return isProviderVisible(providerItem) && providerItem.prompted === true;
}

export function isSignupItemPrompted(signupItem: SignupItemConfig) {
  return signupItem.visible === true && signupItem.prompted === true;
}

export function getPromptedProviderItems(application: ApplicationItem | null) {
  return (application?.providers ?? [])
    .filter((item): item is ProviderConfigItem => typeof item === "object" && item !== null)
    .filter(isProviderPrompted);
}

export function getPromptedSignupItems(application: ApplicationItem | null) {
  return (application?.signupItems ?? []).filter(isSignupItemPrompted);
}

export function isAffiliationPrompted(application: ApplicationItem | null) {
  return getPromptedSignupItems(application).some((item) => item.name === "Affiliation");
}

export function getAffiliationPromptConfig(application: ApplicationItem | null) {
  const affiliationUrl = application?.affiliationUrl;
  if (typeof affiliationUrl !== "string" || affiliationUrl === "") {
    return {
      addressUrl: "",
      optionUrl: "",
      hasStructuredOptions: false,
    };
  }

  const [addressUrl = "", optionUrl = ""] = affiliationUrl.split("|");
  return {
    addressUrl,
    optionUrl,
    hasStructuredOptions: addressUrl !== "" && optionUrl !== "",
  };
}

export function hasPromptPage(application: ApplicationItem | null) {
  if (!application) {
    return false;
  }

  if (getPromptedProviderItems(application).length > 0) {
    return true;
  }

  if (getPromptedSignupItems(application).some((item) => item.name === "Country/Region")) {
    return true;
  }

  return isAffiliationPrompted(application);
}

function isAffiliationAnswered(user: UserDetailItem | null, application: ApplicationItem | null) {
  if (!isAffiliationPrompted(application)) {
    return true;
  }

  return Boolean(user?.affiliation);
}

function isProviderAnswered(user: UserDetailItem | null, providerItem: ProviderConfigItem) {
  const providerType = providerItem.provider?.type?.toLowerCase();
  if (!providerType || !user) {
    return false;
  }

  const linkedValue = (user as Record<string, unknown>)[providerType];
  return typeof linkedValue === "string" && linkedValue !== "";
}

function isSignupItemAnswered(user: UserDetailItem | null, signupItem: SignupItemConfig) {
  if (!user) {
    return false;
  }

  if (signupItem.name !== "Country/Region") {
    return true;
  }

  return typeof user.region === "string" && user.region !== "";
}

function getStringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

export function toPromptDraftUserFrom(owner: string, user: PromptUser | null) {
  if (!user) {
    return null;
  }

  const linkedProviderFields = Object.fromEntries(
    Object.entries(user.linkedProviders).map(([providerType, value]) => [providerType, value]),
  );

  return {
    owner,
    name: user.username,
    countryCode: getStringValue(user.profile.countryCode),
    region: getStringValue(user.profile.region),
    ...linkedProviderFields,
  } satisfies Partial<UserDetailItem> as UserDetailItem;
}

export function isPromptAnswered(user: UserDetailItem | null, application: ApplicationItem | null) {
  if (!application) {
    return true;
  }

  if (!isAffiliationAnswered(user, application)) {
    return false;
  }

  for (const providerItem of getPromptedProviderItems(application)) {
    if (!isProviderAnswered(user, providerItem)) {
      return false;
    }
  }

  for (const signupItem of getPromptedSignupItems(application)) {
    if (!isSignupItemAnswered(user, signupItem)) {
      return false;
    }
  }

  return true;
}

export function buildPromptUpdate(values: {
  application: ApplicationItem | null;
  user: UserDetailItem | null;
  fallbackOwner?: string;
  includeRegion: boolean;
}) {
  const { application, user, fallbackOwner, includeRegion } = values;
  if (!application || !user) {
    return null;
  }

  const columns: string[] = [];
  const payload: Record<string, unknown> = {};

  if (isAffiliationPrompted(application)) {
    columns.push("address", "affiliation", "score");
    payload.address = user.address;
    payload.affiliation = user.affiliation;
    payload.score = user.score;
  }

  if (includeRegion) {
    columns.push("region");
    payload.region = user.region;
  }

  if (columns.length === 0) {
    return null;
  }

  return {
    owner: user.owner || fallbackOwner || "",
    name: user.name,
    payload,
    columns,
  };
}

export function buildPromptRedirectPath(values: {
  applicationName: string;
  redirectUri: string;
  code: string;
  state: string;
}) {
  const query = new URLSearchParams({
    redirectUri: values.redirectUri,
    code: values.code,
    state: values.state,
  });
  return `/prompt/${encodeURIComponent(values.applicationName)}?${query.toString()}`;
}
