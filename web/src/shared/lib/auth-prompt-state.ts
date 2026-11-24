import { getPromptUserByOrganizationSlugAndUsername, type PromptUser } from "@/shared/api/iam/user";
import { getProviders } from "@/shared/api/iam/provider-extended";
import { toPromptDraftUserFrom } from "@/shared/lib/auth-prompt";
import type { ApplicationItem, UserDetailItem } from "@/shared/types/domain";

type LoadPromptUserStateParams = {
  application: ApplicationItem | null;
  owner: string;
  username: string;
  organizationSlug?: string | null;
};

export type PromptUserState = {
  user: PromptUser | null;
  promptUser: UserDetailItem | null;
};

export async function loadPromptUserStateDetails(
  params: LoadPromptUserStateParams,
): Promise<PromptUserState> {
  const { owner, username, organizationSlug } = params;

  const user = organizationSlug
    ? await getPromptUserByOrganizationSlugAndUsername(organizationSlug, username, getProviders)
    : null;
  const draftUser = toPromptDraftUserFrom(owner, user);

  return {
    user,
    promptUser: draftUser,
  };
}

export async function loadPromptUserState(
  params: LoadPromptUserStateParams,
): Promise<UserDetailItem | null> {
  const result = await loadPromptUserStateDetails(params);
  return result.promptUser;
}
