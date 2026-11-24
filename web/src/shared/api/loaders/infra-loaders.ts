import { createDetailLoader, createListLoader } from "@/shared/api/loaders/factory";
import { requestDocumentJson } from "@/shared/api/http";

import {
  AGENTS,
  AGENT_BY_ID,
  CERTS,
  CERT_BY_ID,
  LDAPS,
  LDAP_BY_ID,
  RESOURCES,
  RESOURCE_BY_ID,
  RULES,
  RULE_BY_ID,
  SYNCERS,
  SYNCER_BY_ID,
  WEBHOOKS,
  WEBHOOK_BY_ID,
} from "@/shared/api/paths";

export const resourcesListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${RESOURCES}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const resourceDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${RESOURCE_BY_ID(id)}`),
});
export const certsListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${CERTS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const certDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${CERT_BY_ID(id)}`),
});
export const agentsListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${AGENTS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const agentDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${AGENT_BY_ID(id)}`),
});
export const syncersListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${SYNCERS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const syncerDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${SYNCER_BY_ID(id)}`),
});
export const webhooksListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${WEBHOOKS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const webhookDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${WEBHOOK_BY_ID(id)}`),
});
export const ldapsListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${LDAPS}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const ldapDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${LDAP_BY_ID(id)}`),
});
export const rulesListLoader = createListLoader({
  fetch: async (_ctx, p) =>
    requestDocumentJson(
      `${RULES}?keyword=${p.keyword ?? ""}&page=${p.page ?? 1}&pageSize=${p.pageSize ?? 10}`,
    ),
});
export const ruleDetailLoader = createDetailLoader({
  fetch: async (_ctx, id) => requestDocumentJson(`${RULE_BY_ID(id)}`),
});
