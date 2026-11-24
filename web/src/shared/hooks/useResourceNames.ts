import { useQuery } from "@tanstack/react-query";
import { getOrganizationOptions, type OrganizationOption } from "@/shared/api/iam/organization";
import { getApplicationOptions, type ApplicationOption } from "@/shared/api/iam/application";
import { getUserOptions } from "@/shared/api/iam/user";
import { getProvidersPaginated } from "@/shared/api/iam/provider-extended";
import { queryKeys } from "@/shared/api/query-keys";
import { useSessionStore } from "@/features/session/session-store";

/**
 * Resolve organization IDs/slugs to display names using TanStack Query cache.
 * Returns a Map that is keyed by *both* org UUID and org slug, so callers can
 * pass either (some list APIs return organizationId, some return organizationSlug).
 */
export function useOrganizationNames() {
  const account = useSessionStore((s) => s.account);
  const { data: orgOptions } = useQuery({
    queryKey: [...queryKeys.organizations.auth(), "options"],
    queryFn: getOrganizationOptions,
    enabled: account !== null,
    staleTime: 5 * 60_000,
  });

  const nameMap = new Map<string, string>();
  if (orgOptions) {
    for (const org of orgOptions as OrganizationOption[]) {
      const display = org.displayName || org.name || org.slug;
      // Key by UUID (backend detail APIs) AND slug (list APIs like sessions/tokens).
      nameMap.set(org.id, display);
      if (org.slug) nameMap.set(org.slug, display);
      if (org.name && org.name !== org.slug) nameMap.set(org.name, display);
    }
  }

  return { nameMap, isLoading: !orgOptions };
}

/**
 * Resolve application IDs/slugs to display names using TanStack Query cache.
 * Same dual-key approach as useOrganizationNames — keyed by both app UUID and slug.
 */
export function useApplicationNames(organizationId?: string) {
  const account = useSessionStore((s) => s.account);
  const { data: appOptions } = useQuery({
    queryKey: [...queryKeys.applications.auth(), "options", organizationId],
    queryFn: () => getApplicationOptions(organizationId),
    enabled: account !== null && !!organizationId,
    staleTime: 5 * 60_000,
  });

  const nameMap = new Map<string, string>();
  if (appOptions) {
    for (const app of appOptions as ApplicationOption[]) {
      const display = app.displayName || app.slug;
      nameMap.set(app.id, display);
      if (app.slug) nameMap.set(app.slug, display);
    }
  }

  return { nameMap, isLoading: !appOptions };
}

/**
 * Resolve user IDs/usernames to display names using TanStack Query cache.
 * Dual-keyed: both UUID (actorId in records) and username (some list APIs) map to the same display name.
 */
export function useUserNames(organizationId?: string) {
  const account = useSessionStore((s) => s.account);
  const { data: userOptions } = useQuery({
    queryKey: [...queryKeys.users.auth(), "options", organizationId],
    queryFn: () => getUserOptions(organizationId),
    enabled: account !== null,
    staleTime: 5 * 60_000,
  });

  const nameMap = new Map<string, string>();
  if (userOptions) {
    for (const u of userOptions) {
      const display = u.displayName || u.username;
      nameMap.set(u.id, display);
      if (u.username) nameMap.set(u.username, display);
    }
  }

  return { nameMap, isLoading: !userOptions };
}

/**
 * Resolve provider IDs to display names using TanStack Query cache.
 * Uses providers paginated list since no dedicated /options endpoint exists.
 */
export function useProviderNames() {
  const account = useSessionStore((s) => s.account);
  const { data } = useQuery({
    queryKey: ["providers", "name-map"],
    queryFn: () => getProvidersPaginated({ page: 1, pageSize: 100 }),
    enabled: account !== null,
    staleTime: 5 * 60_000,
  });

  const nameMap = new Map<string, string>();
  if (data?.items) {
    for (const p of data.items) {
      nameMap.set(p.id, p.displayName || p.name || p.category);
    }
  }

  return { nameMap, isLoading: !data };
}
