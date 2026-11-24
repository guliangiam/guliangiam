import { requestDocumentJson } from "@/shared/api/http";
import {
  IDENTITY_POOLS,
  IDENTITY_POOL_BY_ID,
  IDENTITY_POOL_FEDERATION_TRUSTS,
  IDENTITY_POOL_FEDERATION_TRUST_BY_ID,
  IDENTITY_POOL_FEDERATION_TRUST_MATCH_SUBJECT,
  IDENTITY_POOL_FEDERATION_TRUST_STATUS,
  IDENTITY_POOL_SERVICE_ACCOUNTS,
  IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID,
  IDENTITY_POOL_SERVICE_ACCOUNT_DELETE_PRECHECK,
  IDENTITY_POOL_SERVICE_ACCOUNT_ROTATE_SECRET,
  IDENTITY_POOL_SERVICE_ACCOUNT_STATUS,
} from "@/shared/api/paths";
import type { IamListResponse } from "./client";

export type WorkloadResourceStatus = "active" | "disabled" | "deleted";
export type WorkloadMutableStatus = Exclude<WorkloadResourceStatus, "deleted">;

export type IdentityPoolItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type FederationTrustItem = {
  id: string;
  organizationId: string;
  poolId: string;
  name: string;
  issuer: string;
  jwksUri: string;
  audience: string[];
  subjects: string[];
  status: WorkloadResourceStatus;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type ServiceAccountItem = {
  id: string;
  organizationId: string;
  poolId: string;
  name: string;
  description: string;
  clientId: string;
  clientSecretPlain?: string;
  scopes: string[];
  accessTokenTtlSeconds: number;
  lastRotatedAt: string | null;
  status: WorkloadResourceStatus;
  secretSet: boolean;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type SubjectMatch = {
  subject: string;
  allowed: boolean;
};

export type ServiceAccountDeletePrecheck = {
  id: string;
  organizationId: string;
  poolId: string;
  activeTokenCount: number;
  scopes: string[];
  status: WorkloadResourceStatus;
  canDelete: boolean;
  revision: number;
};

export type CreateIdentityPoolRequest = {
  name: string;
  displayName?: string;
  description?: string;
};

export type CreateFederationTrustRequest = {
  name: string;
  issuer: string;
  jwksUri: string;
  audience: string[];
  subjects: string[];
};

export type UpdateFederationTrustStatusRequest = {
  status: WorkloadMutableStatus;
};

export type MatchSubjectRequest = {
  patterns: string[];
  samples: string[];
};

export type CreateServiceAccountRequest = {
  name: string;
  description?: string;
  scopes: string[];
  accessTokenTtlSeconds: number;
};

export type UpdateServiceAccountStatusRequest = {
  status: WorkloadMutableStatus;
  revision: number;
};

export type ServiceAccountCredentials = {
  clientId: string;
  clientSecretPlain: string;
};

function withOrganization(path: string, organizationId: string, list = false): string {
  // Inline the list paging params instead of spreading a shared constant:
  // the static API-parameter gate reads object spreads as literal query keys
  // and reported a phantom "LIST_QUERY" parameter that no handler reads.
  const query = new URLSearchParams(
    list ? { organizationId, page: "1", pageSize: "100" } : { organizationId },
  );
  return `${path}?${query.toString()}`;
}

async function requestRequired<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await requestDocumentJson<T>(path, init);
  if (response === undefined) {
    throw new Error("The server returned an empty JSON response");
  }
  return response;
}

export function listIdentityPools(organizationId: string) {
  return requestRequired<IamListResponse<IdentityPoolItem>>(
    withOrganization(IDENTITY_POOLS, organizationId, true),
  );
}

export function getIdentityPool(organizationId: string, id: string) {
  return requestRequired<IdentityPoolItem>(
    withOrganization(IDENTITY_POOL_BY_ID(id), organizationId),
  );
}

export function createIdentityPool(organizationId: string, input: CreateIdentityPoolRequest) {
  return requestRequired<IdentityPoolItem>(withOrganization(IDENTITY_POOLS, organizationId), {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listFederationTrusts(organizationId: string, poolId: string) {
  return requestRequired<IamListResponse<FederationTrustItem>>(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUSTS(poolId), organizationId, true),
  );
}

export function getFederationTrust(organizationId: string, poolId: string, id: string) {
  return requestRequired<FederationTrustItem>(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUST_BY_ID(poolId, id), organizationId),
  );
}

export function createFederationTrust(
  organizationId: string,
  poolId: string,
  input: CreateFederationTrustRequest,
) {
  return requestRequired<FederationTrustItem>(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUSTS(poolId), organizationId),
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function setFederationTrustStatus(
  organizationId: string,
  poolId: string,
  id: string,
  input: UpdateFederationTrustStatusRequest,
) {
  return requestRequired<FederationTrustItem>(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUST_STATUS(poolId, id), organizationId),
    { method: "PATCH", body: JSON.stringify(input) },
  );
}

export async function deleteFederationTrust(organizationId: string, poolId: string, id: string) {
  await requestDocumentJson(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUST_BY_ID(poolId, id), organizationId),
    { method: "DELETE" },
  );
}

export function matchFederationTrustSubject(
  organizationId: string,
  poolId: string,
  input: MatchSubjectRequest,
) {
  return requestRequired<SubjectMatch[]>(
    withOrganization(IDENTITY_POOL_FEDERATION_TRUST_MATCH_SUBJECT(poolId), organizationId),
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function listServiceAccounts(organizationId: string, poolId: string) {
  return requestRequired<IamListResponse<ServiceAccountItem>>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNTS(poolId), organizationId, true),
  );
}

export function getServiceAccount(organizationId: string, poolId: string, id: string) {
  return requestRequired<ServiceAccountItem>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID(poolId, id), organizationId),
  );
}

export function createServiceAccount(
  organizationId: string,
  poolId: string,
  input: CreateServiceAccountRequest,
) {
  return requestRequired<ServiceAccountItem>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNTS(poolId), organizationId),
    { method: "POST", body: JSON.stringify(input) },
  );
}

export function setServiceAccountStatus(
  organizationId: string,
  poolId: string,
  id: string,
  input: UpdateServiceAccountStatusRequest,
) {
  return requestRequired<ServiceAccountItem>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNT_STATUS(poolId, id), organizationId),
    { method: "PATCH", body: JSON.stringify(input) },
  );
}

export function rotateServiceAccountSecret(
  organizationId: string,
  poolId: string,
  id: string,
  revision: number,
) {
  return requestRequired<ServiceAccountItem>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNT_ROTATE_SECRET(poolId, id), organizationId),
    { method: "POST", body: JSON.stringify({ revision }) },
  );
}

export function getServiceAccountDeletePrecheck(
  organizationId: string,
  poolId: string,
  id: string,
) {
  return requestRequired<ServiceAccountDeletePrecheck>(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNT_DELETE_PRECHECK(poolId, id), organizationId),
  );
}

export async function deleteServiceAccount(
  organizationId: string,
  poolId: string,
  id: string,
  revision: number,
) {
  await requestDocumentJson(
    withOrganization(IDENTITY_POOL_SERVICE_ACCOUNT_BY_ID(poolId, id), organizationId),
    {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ revision }),
    },
  );
}
