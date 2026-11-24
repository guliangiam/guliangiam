import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import {
  CERTS,
  CERT_BY_ID,
  CERT_CHECK_DOMAIN_EXPIRATION,
  CERT_UPDATE_DOMAIN_EXPIRE,
} from "@/shared/api/paths";
import { captureError } from "@/shared/lib/sentry";

// ─────────────────────────────────────────────
// Certs
// ─────────────────────────────────────────────
// Mirrors `appcerts.CertListItem`. The console used to declare `issuer`,
// `algorithm`, `isEnabled` and `expiresAt` — none of which the server sends. A
// column bound to a field the server never sends renders blank, or worse, a
// fallback that reads as real data (`isEnabled` undefined rendered every
// certificate as "disabled").
export type IamCertItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  scope: string;
  type: string;
  cryptoAlgorithm: string;
  bitSize: number;
  expireInYears: number;
  expireTime?: string;
  provider?: string;
  createdAt: string;
  updatedAt: string;
  revision: number;
};
export type IamCertDetail = IamCertItem & {
  owner?: string;
  certType?: string;
  expireAt?: string;
  certificate: string;
  privateKey: string;
  authorityPublicKey: string;
  authorityRootPublicKey: string;
  acmeIssuer: string;
  acmeEndpoint: string;
  account: string;
  accessKey: string;
  accessSecret: string;
  domainExpireTime: string;
};
export type IamCreateCertPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  cryptoAlgorithm: string;
  certificate: string;
  privateKey: string;
  scope?: string;
  bitSize?: number;
  expireInYears?: number;
  authorityPublicKey?: string;
  authorityRootPublicKey?: string;
  acmeIssuer?: string;
  acmeEndpoint?: string;
  provider?: string;
  account?: string;
  accessKey?: string;
  accessSecret?: string;
};
export type IamPatchCertPayload = {
  type?: string;
  cryptoAlgorithm?: string;
  displayName?: string;
  certificate?: string;
  privateKey?: string;
  scope?: string;
  bitSize?: number;
  expireInYears?: number;
  authorityPublicKey?: string;
  authorityRootPublicKey?: string;
  acmeIssuer?: string;
  acmeEndpoint?: string;
  provider?: string;
  account?: string;
  accessKey?: string;
  accessSecret?: string;
  revision: number;
};

export const getIamCerts = (params?: IamListParams) => iamList<IamCertItem>(CERTS, params);
export const getIamCertsPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamCertItem>(CERTS, params);
export const getIamCert = (id: string) => iamGet<IamCertDetail>(`${CERT_BY_ID(id)}`);
export const createIamCert = (p: IamCreateCertPayload) => iamCreate(CERTS, p);
export const patchIamCert = (id: string, p: IamPatchCertPayload) =>
  iamPatch(`${CERT_BY_ID(id)}`, p);
export const deleteIamCert = (id: string) => iamDelete(`${CERT_BY_ID(id)}`);

/** Check and update domain expiration time for an SSL certificate. */
export const checkDomainExpire = (id: string, domainExpireTime: string) =>
  iamPostRaw(`${CERT_UPDATE_DOMAIN_EXPIRE(id)}`, { domainExpireTime });

/** Check domain expiration for a certificate by parsing its NotAfter. */
export const checkDomainExpiration = (id: string) =>
  iamPostRaw(`${CERT_CHECK_DOMAIN_EXPIRATION(id)}`);

export async function getCertOptions(): Promise<{ label: string; value: string }[]> {
  try {
    // Must stay <= platformpagination.MaxPageSize (100); a larger pageSize is
    // rejected with 422 rather than truncated.
    const certs = await getIamCerts({ pageSize: 100 });
    return (certs ?? []).map((c) => ({
      label: c.displayName || c.name,
      value: c.id,
    }));
  } catch (error) {
    captureError(error, { context: "getCertOptions" });
    return [];
  }
}
