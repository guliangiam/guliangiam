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
  MCP_PROXY,
  MCP_SERVERS,
  MCP_SERVERS_ONLINE,
  MCP_SERVER_ACCESS_TOKEN,
  MCP_SERVER_BY_ID,
  MCP_SERVER_SYNC_INTRANET,
  MCP_SERVER_SYNC_TOOLS,
} from "@/shared/api/paths";

// ─────────────────────────────────────────────
// MCP Servers
// ─────────────────────────────────────────────
export type IamMcpServerItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  url: string;
  application: string;
  isEnabled: boolean;
  createdAt: string;
  updatedAt: string;
};
export type IamMcpServerDetail = IamMcpServerItem & {
  // The token is write-only: the detail response only reports whether it is set
  // and a masked display value. Never prefill a token input from this response.
  tokenSet: boolean;
  tokenMasked?: string;
  tools: { name: string; description: string; isAllowed: boolean }[];
  revision: number;
};
export type IamCreateMcpServerPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  url?: string;
  token?: string;
  application?: string;
  isEnabled?: boolean;
};
export type IamPatchMcpServerPayload = {
  displayName?: string;
  url?: string;
  token?: string;
  application?: string;
  isEnabled?: boolean;
  revision: number;
};

export const getIamMcpServers = (params?: IamListParams) =>
  iamList<IamMcpServerItem>(MCP_SERVERS, params);
export const getIamMcpServersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamMcpServerItem>(MCP_SERVERS, params);
export const getIamMcpServer = (id: string) =>
  iamGet<IamMcpServerDetail>(`${MCP_SERVER_BY_ID(id)}`);
export const createIamMcpServer = (p: IamCreateMcpServerPayload) => iamCreate(MCP_SERVERS, p);
export const patchIamMcpServer = (id: string, p: IamPatchMcpServerPayload) =>
  iamPatch(`${MCP_SERVER_BY_ID(id)}`, p);
export const deleteIamMcpServer = (id: string) => iamDelete(`${MCP_SERVER_BY_ID(id)}`);

// ─────────────────────────────────────────────
// MCP Server Advanced Features
// ─────────────────────────────────────────────

// Mirrors `appmcpservers.OnlineServer`. The registry entry carries only these
// four fields — listing `displayName` or `version` here would promise fields the
// server never sends, and the card would render them as blank.
export type OnlineMcpServer = {
  name: string;
  description: string;
  url: string;
  category: string;
};

// GET, not POST: the handler registers `router.Get(MCP_SERVERS_ONLINE)`
// and reads `registryUrl` from the query string. A POST to this path matched no
// route, so the server store page could never load.
export const getOnlineMcpServers = (registryUrl?: string) => {
  const query = registryUrl ? `?registryUrl=${encodeURIComponent(registryUrl)}` : "";
  return iamGet<OnlineMcpServer[]>(`${MCP_SERVERS_ONLINE}${query}`);
};

export type SyncIntranetPayload = {
  cidrs: string[];
  ports: number[];
};

export const syncIntranetServers = (payload: SyncIntranetPayload) =>
  iamPostRaw(MCP_SERVER_SYNC_INTRANET, payload);

export const syncMcpTools = (id: string) => iamPostRaw(`${MCP_SERVER_SYNC_TOOLS(id)}`);

export type McpAccessTokenResponse = {
  accessToken: string;
  expiresIn: number;
};

export const getMcpAccessToken = (id: string) =>
  iamGet<McpAccessTokenResponse>(`${MCP_SERVER_ACCESS_TOKEN(id)}`);

export type McpProxyPayload = {
  toolName: string;
  arguments: Record<string, unknown>;
};

export const proxyMcpRequest = (orgSlug: string, serverName: string, payload: McpProxyPayload) =>
  iamPostRaw(`${MCP_PROXY(orgSlug, serverName)}`, payload);
