/**
 * B2B 组织域名的共享常量与工具函数。
 *
 * 独立域名页（`pages/org-domains`）与组织详情页内嵌域名列表都要做同样的
 * 域名归一化、DNS TXT 主机拼接与状态着色，因此下沉到 `shared/lib`，
 * 避免页面之间互相 import 控制器。
 */
import type { DomainStatus } from "@/shared/api/iam/b2b";

export const STATUS_COLOR: Record<DomainStatus, string> = {
  pending: "gold",
  verified: "green",
  failed: "red",
};

export const DOMAIN_REGEX =
  /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/;

export function normalizeDomain(domain: string): string {
  return domain.trim().toLowerCase().replace(/\.$/, "");
}

export function dnsTxtHost(domain: string): string {
  return `_guliangiam-verification.${domain}`;
}
