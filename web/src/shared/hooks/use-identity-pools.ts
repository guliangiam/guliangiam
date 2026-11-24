import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import {
  createFederationTrust,
  createIdentityPool,
  createServiceAccount,
  deleteFederationTrust,
  deleteServiceAccount,
  getFederationTrust,
  getIdentityPool,
  getServiceAccount,
  getServiceAccountDeletePrecheck,
  listFederationTrusts,
  listIdentityPools,
  listServiceAccounts,
  matchFederationTrustSubject,
  rotateServiceAccountSecret,
  setFederationTrustStatus,
  setServiceAccountStatus,
  type CreateFederationTrustRequest,
  type CreateIdentityPoolRequest,
  type CreateServiceAccountRequest,
  type MatchSubjectRequest,
  type WorkloadMutableStatus,
} from "@/shared/api/iam/identity-pools";

const POOL_STALE = 60_000;

function useOrganizationId() {
  return useSessionStore((state) => state.organization?.id ?? undefined);
}

function requireOrganization(organizationId: string | undefined): string {
  if (!organizationId) throw new Error("No organization is selected");
  return organizationId;
}

function requirePoolScope(organizationId: string | undefined, poolId: string | undefined) {
  const orgId = requireOrganization(organizationId);
  if (!poolId) throw new Error("No identity pool is selected");
  return { orgId, poolId };
}

export function useIdentityPools() {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"].lists(organizationId ?? ""),
    queryFn: () => listIdentityPools(requireOrganization(organizationId)),
    enabled: Boolean(organizationId),
    select: (data) => data.items,
    staleTime: POOL_STALE,
  });
}

export function useIdentityPool(id: string | undefined) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"].detail(organizationId ?? "", id ?? ""),
    queryFn: () => getIdentityPool(requireOrganization(organizationId), id ?? ""),
    enabled: Boolean(organizationId && id),
    staleTime: POOL_STALE,
  });
}

export function useCreateIdentityPool() {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateIdentityPoolRequest) =>
      createIdentityPool(requireOrganization(organizationId), input),
    onSuccess: () => {
      if (!organizationId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"].lists(organizationId),
      });
    },
  });
}

export function useFederationTrusts(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"]["federation-trusts"](organizationId ?? "", poolId ?? ""),
    queryFn: () => {
      const scope = requirePoolScope(organizationId, poolId);
      return listFederationTrusts(scope.orgId, scope.poolId);
    },
    enabled: Boolean(organizationId && poolId),
    select: (data) => data.items,
    staleTime: POOL_STALE,
  });
}

export function useFederationTrust(poolId: string | undefined, id: string | undefined) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"]["federation-trust"](
      organizationId ?? "",
      poolId ?? "",
      id ?? "",
    ),
    queryFn: () => {
      const scope = requirePoolScope(organizationId, poolId);
      return getFederationTrust(scope.orgId, scope.poolId, id ?? "");
    },
    enabled: Boolean(organizationId && poolId && id),
    staleTime: POOL_STALE,
  });
}

export function useCreateFederationTrust(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateFederationTrustRequest) => {
      const scope = requirePoolScope(organizationId, poolId);
      return createFederationTrust(scope.orgId, scope.poolId, input);
    },
    onSuccess: () => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["federation-trusts"](organizationId, poolId),
      });
    },
  });
}

export function useMatchFederationTrustSubject(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  return useMutation({
    mutationFn: (input: MatchSubjectRequest) => {
      const scope = requirePoolScope(organizationId, poolId);
      return matchFederationTrustSubject(scope.orgId, scope.poolId, input);
    },
  });
}

export function useSetFederationTrustStatus(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: WorkloadMutableStatus }) => {
      const scope = requirePoolScope(organizationId, poolId);
      return setFederationTrustStatus(scope.orgId, scope.poolId, id, { status });
    },
    onSuccess: (_result, { id }) => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["federation-trusts"](organizationId, poolId),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["federation-trust"](organizationId, poolId, id),
      });
    },
  });
}

export function useDeleteFederationTrust(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => {
      const scope = requirePoolScope(organizationId, poolId);
      return deleteFederationTrust(scope.orgId, scope.poolId, id);
    },
    onSuccess: (_result, id) => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["federation-trusts"](organizationId, poolId),
      });
      queryClient.removeQueries({
        queryKey: queryKeys["identity-pools"]["federation-trust"](organizationId, poolId, id),
      });
    },
  });
}

export function useServiceAccounts(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"]["service-accounts"](organizationId ?? "", poolId ?? ""),
    queryFn: () => {
      const scope = requirePoolScope(organizationId, poolId);
      return listServiceAccounts(scope.orgId, scope.poolId);
    },
    enabled: Boolean(organizationId && poolId),
    select: (data) => data.items,
    staleTime: POOL_STALE,
  });
}

export function useServiceAccount(poolId: string | undefined, id: string | undefined) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"]["service-account"](
      organizationId ?? "",
      poolId ?? "",
      id ?? "",
    ),
    queryFn: () => {
      const scope = requirePoolScope(organizationId, poolId);
      return getServiceAccount(scope.orgId, scope.poolId, id ?? "");
    },
    enabled: Boolean(organizationId && poolId && id),
    staleTime: POOL_STALE,
  });
}

export function useCreateServiceAccount(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateServiceAccountRequest) => {
      const scope = requirePoolScope(organizationId, poolId);
      return createServiceAccount(scope.orgId, scope.poolId, input);
    },
    onSuccess: () => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-accounts"](organizationId, poolId),
      });
    },
  });
}

export function useSetServiceAccountStatus(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      status,
      revision,
    }: {
      id: string;
      status: WorkloadMutableStatus;
      revision: number;
    }) => {
      const scope = requirePoolScope(organizationId, poolId);
      return setServiceAccountStatus(scope.orgId, scope.poolId, id, { status, revision });
    },
    onSuccess: (_result, { id }) => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-accounts"](organizationId, poolId),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-account"](organizationId, poolId, id),
      });
    },
  });
}

export function useRotateServiceAccountSecret(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) => {
      const scope = requirePoolScope(organizationId, poolId);
      return rotateServiceAccountSecret(scope.orgId, scope.poolId, id, revision);
    },
    onSuccess: (_result, { id }) => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-accounts"](organizationId, poolId),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-account"](organizationId, poolId, id),
      });
    },
  });
}

export function useServiceAccountDeletePrecheck(
  poolId: string | undefined,
  id: string | undefined,
  enabled = true,
) {
  const organizationId = useOrganizationId();
  return useQuery({
    queryKey: queryKeys["identity-pools"]["service-account-delete-precheck"](
      organizationId ?? "",
      poolId ?? "",
      id ?? "",
    ),
    queryFn: () => {
      const scope = requirePoolScope(organizationId, poolId);
      return getServiceAccountDeletePrecheck(scope.orgId, scope.poolId, id ?? "");
    },
    enabled: enabled && Boolean(organizationId && poolId && id),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnMount: "always",
  });
}

export function useDeleteServiceAccount(poolId: string | undefined) {
  const organizationId = useOrganizationId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, revision }: { id: string; revision: number }) => {
      const scope = requirePoolScope(organizationId, poolId);
      return deleteServiceAccount(scope.orgId, scope.poolId, id, revision);
    },
    onSuccess: (_result, { id }) => {
      if (!organizationId || !poolId) return;
      void queryClient.invalidateQueries({
        queryKey: queryKeys["identity-pools"]["service-accounts"](organizationId, poolId),
      });
      queryClient.removeQueries({
        queryKey: queryKeys["identity-pools"]["service-account"](organizationId, poolId, id),
      });
      queryClient.removeQueries({
        queryKey: queryKeys["identity-pools"]["service-account-delete-precheck"](
          organizationId,
          poolId,
          id,
        ),
      });
    },
  });
}
