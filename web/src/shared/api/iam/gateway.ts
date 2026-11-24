import { z } from "zod";
import { requestDocumentJson } from "@/shared/api/http";
import {
  GATEWAY_SERVERS,
  GATEWAY_SERVER_BY_ID,
  GATEWAY_SITES,
  GATEWAY_SITES_GLOBAL,
  GATEWAY_SITE_BY_ID,
  GATEWAY_SITE_CERT_STATUS,
  GATEWAY_SITE_CHECK_HEALTH,
  GATEWAY_SITE_HEALTH,
  GATEWAY_SITE_PROVISION_CERT,
  GATEWAY_SITE_RENEW_CERT,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Types — Gateway Servers
// ─────────────────────────────────────────────

export type GatewayServerDetail = z.infer<typeof gatewayServerDetailSchema>;
export type CreateGatewayServerPayload = {
  organizationId: string;
  name: string;
  endpoint: string;
  protocol: string;
  status: string;
  metadata?: Record<string, unknown>;
};
export type UpdateGatewayServerPayload = {
  name?: string;
  endpoint?: string;
  protocol?: string;
  status?: string;
  metadata?: Record<string, unknown>;
  revision: number;
};

// ─────────────────────────────────────────────
// Types — Gateway Sites
// ─────────────────────────────────────────────

export type GatewaySiteDetail = z.infer<typeof gatewaySiteDetailSchema>;
export type CreateGatewaySitePayload = {
  organizationId: string;
  name: string;
  host: string;
  status: string;
  metadata?: Record<string, unknown>;
};
export type UpdateGatewaySitePayload = {
  name?: string;
  host?: string;
  status?: string;
  sslMode?: string;
  certId?: string;
  forceHttps?: boolean;
  enableHealthCheck?: boolean;
  healthCheckInterval?: number;
  enableAlert?: boolean;
  alertInterval?: number;
  alertTryTimes?: number;
  alertProviderIds?: string[];
  metadata?: Record<string, unknown>;
  revision: number;
};

// ─────────────────────────────────────────────
// Zod Schemas — Gateway Servers
// ─────────────────────────────────────────────

const gatewayServerListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  endpoint: z.string(),
  protocol: z.string(),
  status: z.string(),
  metadata: z.record(z.string(), z.any()).optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
  revision: z.number(),
});

const gatewayServerListResponseSchema = z.object({
  items: z.array(gatewayServerListItemSchema),
  page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
});

const gatewayServerDetailSchema = gatewayServerListItemSchema;

// ─────────────────────────────────────────────
// Zod Schemas — Gateway Sites
// ─────────────────────────────────────────────

const gatewaySiteListItemSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  name: z.string(),
  host: z.string(),
  status: z.string(),
  sslMode: z.string().optional().default(""),
  certId: z.string().optional().default(""),
  forceHttps: z.boolean().optional().default(false),
  enableHealthCheck: z.boolean().optional().default(false),
  healthCheckInterval: z.number().optional().default(0),
  lastHealthStatus: z.string().optional().default(""),
  lastHealthAt: z.string().optional().nullable(),
  enableAlert: z.boolean().optional().default(false),
  alertInterval: z.number().optional().default(0),
  alertTryTimes: z.number().optional().default(0),
  alertProviderIds: z.array(z.string()).optional(),
  metadata: z.record(z.string(), z.any()).optional().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
  revision: z.number(),
});

const gatewaySiteListResponseSchema = z.object({
  items: z.array(gatewaySiteListItemSchema),
  page: z.object({ total: z.number(), page: z.number(), pageSize: z.number() }),
});

const gatewaySiteDetailSchema = gatewaySiteListItemSchema;

// ─────────────────────────────────────────────
// API Functions — Gateway Servers
// ─────────────────────────────────────────────

export async function getGatewayServers(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${GATEWAY_SERVERS}?${query.toString()}`);
  return gatewayServerListResponseSchema.parse(response).items;
}

export type GatewayServerListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  sortBy?: string;
  sortOrder?: string;
};

export async function getGatewayServersPaginated(params?: GatewayServerListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  const response = await requestDocumentJson(`${GATEWAY_SERVERS}?${query.toString()}`);
  const parsed = gatewayServerListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getGatewayServer(id: string): Promise<GatewayServerDetail> {
  const response = await requestDocumentJson(GATEWAY_SERVER_BY_ID(id));
  return gatewayServerDetailSchema.parse(response);
}

export async function createGatewayServer(payload: CreateGatewayServerPayload) {
  await requestDocumentJson(GATEWAY_SERVERS, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updateGatewayServer(id: string, payload: UpdateGatewayServerPayload) {
  await requestDocumentJson(GATEWAY_SERVER_BY_ID(id), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function deleteGatewayServer(id: string) {
  await requestDocumentJson(GATEWAY_SERVER_BY_ID(id), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// API Functions — Gateway Sites
// ─────────────────────────────────────────────

export async function getGatewaySites(organizationId?: string) {
  const query = new URLSearchParams({ page: "1", pageSize: "50" });
  if (organizationId) query.set("organizationId", organizationId);
  const response = await requestDocumentJson(`${GATEWAY_SITES}?${query.toString()}`);
  return gatewaySiteListResponseSchema.parse(response).items;
}

export type GatewaySiteListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  organizationId?: string;
  sortBy?: string;
  sortOrder?: string;
};

export async function getGatewaySitesPaginated(params?: GatewaySiteListParams) {
  const query = new URLSearchParams({
    page: String(params?.page ?? 1),
    pageSize: String(params?.pageSize ?? 50),
  });
  if (params?.search) query.set("keyword", params.search);
  if (params?.organizationId) query.set("organizationId", params.organizationId);
  const response = await requestDocumentJson(`${GATEWAY_SITES}?${query.toString()}`);
  const parsed = gatewaySiteListResponseSchema.parse(response);
  return {
    items: parsed.items,
    page: parsed.page.page,
    pageSize: parsed.page.pageSize,
    total: parsed.page.total,
  };
}

export async function getGatewaySite(id: string): Promise<GatewaySiteDetail> {
  const response = await requestDocumentJson(GATEWAY_SITE_BY_ID(id));
  return gatewaySiteDetailSchema.parse(response);
}

export async function createGatewaySite(payload: CreateGatewaySitePayload) {
  await requestDocumentJson(GATEWAY_SITES, {
    method: "POST",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function updateGatewaySite(id: string, payload: UpdateGatewaySitePayload) {
  await requestDocumentJson(GATEWAY_SITE_BY_ID(id), {
    method: "PATCH",
    body: JSON.stringify(payload),
    headers: { "Content-Type": "application/json" },
  });
}

export async function deleteGatewaySite(id: string) {
  await requestDocumentJson(GATEWAY_SITE_BY_ID(id), {
    method: "DELETE",
  });
}

// ─────────────────────────────────────────────
// API Functions — Site SSL Certificates
// ─────────────────────────────────────────────

export type CertStatusResult = {
  status: string;
  issuer: string;
  expireTime: string | null;
  daysRemaining: number;
};

export const provisionSiteCert = (id: string) =>
  requestDocumentJson(GATEWAY_SITE_PROVISION_CERT(id), {
    method: "POST",
  });

export const renewSiteCert = (id: string) =>
  requestDocumentJson(GATEWAY_SITE_RENEW_CERT(id), {
    method: "POST",
  });

export const getSiteCertStatus = (id: string): Promise<CertStatusResult | undefined> =>
  requestDocumentJson<CertStatusResult>(GATEWAY_SITE_CERT_STATUS(id));

// ─────────────────────────────────────────────
// API Functions — Site Health Monitoring
// ─────────────────────────────────────────────

export type HealthCheckResult = {
  status: string;
  statusCode: number;
  latencyMs: number;
  checkedAt: string;
  error: string;
};

export const checkSiteHealth = (id: string): Promise<HealthCheckResult | undefined> =>
  requestDocumentJson<HealthCheckResult>(GATEWAY_SITE_CHECK_HEALTH(id), { method: "POST" });

export const getSiteHealth = (id: string): Promise<HealthCheckResult | undefined> =>
  requestDocumentJson<HealthCheckResult>(GATEWAY_SITE_HEALTH(id));

// ─────────────────────────────────────────────
// API Functions — Global Sites
// ─────────────────────────────────────────────

export type GlobalSiteItem = {
  id: string;
  organizationId: string;
  name: string;
  host: string;
  status: string;
  lastHealthStatus: string;
  sslMode: string;
};

export const getGlobalSites = (): Promise<GlobalSiteItem[] | undefined> =>
  requestDocumentJson<GlobalSiteItem[]>(GATEWAY_SITES_GLOBAL);
