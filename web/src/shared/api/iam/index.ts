export type { IamListParams, IamListResponse } from "./client";
export {
  iamList,
  iamListWithPagination,
  iamGet,
  iamCreate,
  iamPatch,
  iamUpdate,
  iamDelete,
  iamPostRaw,
  buildQueryString,
} from "./client";

export * from "./schemas";
export * from "./cert";
export * from "./resource";
export * from "./model";
export * from "./adapter";
export * from "./enforcer";
export * from "./syncer";
export * from "./migration";
export * from "./webhook";
export * from "./dashboard";
export * from "./ldap";
export * from "./agent";
export * from "./product";
export * from "./order";
export * from "./payment";
export * from "./plan";
export * from "./pricing";
export * from "./subscription";
export * from "./ticket";
export * from "./form";
export * from "./mcp-server";
export * from "./coupon";
export * from "./entry";
export * from "./radius-accounting";
export * from "./captcha-provider";
export * from "./notification-provider";
export * from "./rule";
export * from "./face-id-provider";
export * from "./radius-config";
export * from "./transaction";
export * from "./key";
export * from "./webhook-event";

// Domain modules
export * from "./auth";
export * from "./auth-signup";
// auth-web3 intentionally omitted — Web3 providers removed (backend + frontend)
// See CLAUDE.md: "Web3 category (MetaMask, Web3-Onboard) was removed"
export * from "./auth-webauthn";
export * from "./auth-face-id";
export * from "./auth-mfa-push";
export * from "./user";
export * from "./organization";
export * from "./application";
export * from "./provider-extended";
export * from "./role";
export * from "./permission";
export * from "./group";
export * from "./session";
export * from "./token-extended";
export * from "./invitation";
export * from "./gateway";
export * from "./record";
export * from "./notification";
export * from "./verification";
export * from "./device";
