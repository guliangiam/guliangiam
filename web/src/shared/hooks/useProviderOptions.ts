import { useQuery } from "@tanstack/react-query";
import { getProviders } from "@/shared/api/iam/provider-extended";

/**
 * Shared provider option query. User identity pickers and application binding
 * pickers both need the providers of one organization; keeping the query key
 * and org scoping in one place prevents cross-tenant options from leaking into
 * one of the pages.
 */
export function useProviderOptions(
  organizationId: string | undefined,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: ["providers", "options", organizationId ?? "all"],
    queryFn: () => getProviders(organizationId),
    enabled: options?.enabled ?? true,
    staleTime: 60_000,
  });
}
