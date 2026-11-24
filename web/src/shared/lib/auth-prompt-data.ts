import { loadApplicationDetail } from "@/shared/lib/auth-application-loader";
import { getAuthAccountContext } from "@/shared/api/iam/auth";
import { buildPromptUpdate, getAffiliationPromptConfig } from "@/shared/lib/auth-prompt";
import { requestDocumentJson } from "@/shared/api/http";
import type { ApplicationItem, UserDetailItem } from "@/shared/types/domain";

import { USER_BY_ID } from "@/shared/api/paths";

async function getAddressOptions(url: string) {
  return (await requestDocumentJson<unknown[]>(url)) ?? [];
}

async function getAffiliationOptions(url: string, code: string) {
  return (
    (await requestDocumentJson<Array<{ id?: number; name?: string }>>(
      `${url}/${encodeURIComponent(code)}`,
    )) ?? []
  );
}

async function updateUserDetail(
  owner: string,
  name: string,
  payload: unknown,
  _columns?: string[],
) {
  // Use v1 user patch endpoint. The "owner/name" format is resolved to find the user ID.
  // For now, we rely on the account context to get the user ID directly.
  // The columns parameter is not supported in v1 API (all provided fields are updated).
  const account = await getAuthAccountContext();
  const userId =
    (account as Record<string, unknown>)?.userId ??
    (account.account as Record<string, unknown>)?.id ??
    `${owner}/${name}`;

  await requestDocumentJson(USER_BY_ID(String(userId)), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: {
      "Content-Type": "application/json",
    },
  });
}

export type PromptAddressOption = {
  label?: string;
  value?: string;
  children?: PromptAddressOption[];
};

export type PromptAffiliationOption = {
  id?: number;
  name?: string;
};

export async function loadPromptSession() {
  const ctx = await getAuthAccountContext();
  return {
    account: {
      owner: ctx.account.owner,
      name: ctx.account.name,
      displayName: ctx.account.displayName ?? undefined,
      email: undefined,
      phone: undefined,
      avatar: ctx.account.avatar ?? undefined,
    },
    organization: ctx.organization
      ? {
          name: ctx.organization.name ?? "",
          displayName: ctx.organization.displayName ?? undefined,
        }
      : null,
  };
}

export async function loadPromptApplication(applicationName: string) {
  return loadApplicationDetail(applicationName);
}

export async function loadPromptAddressOptions(
  application: ApplicationItem | null,
): Promise<PromptAddressOption[]> {
  const config = getAffiliationPromptConfig(application);
  if (config.addressUrl === "") {
    return [];
  }

  const options = await getAddressOptions(config.addressUrl);
  return Array.isArray(options) ? (options as PromptAddressOption[]) : [];
}

export async function loadPromptAffiliationOptions(values: {
  application: ApplicationItem | null;
  addressCode?: string | null;
}): Promise<PromptAffiliationOption[]> {
  const { application, addressCode } = values;
  const config = getAffiliationPromptConfig(application);
  if (config.optionUrl === "" || !addressCode) {
    return [];
  }

  const options = await getAffiliationOptions(config.optionUrl, String(addressCode));
  return Array.isArray(options) ? options : [];
}

export async function submitPromptUpdate(values: {
  application: ApplicationItem | null;
  user: UserDetailItem | null;
  fallbackOwner?: string;
  includeRegion: boolean;
}) {
  const update = buildPromptUpdate(values);
  if (!update) {
    return false;
  }

  await updateUserDetail(update.owner, update.name, update.payload, update.columns);
  return true;
}
