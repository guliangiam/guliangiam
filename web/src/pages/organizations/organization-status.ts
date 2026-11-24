/**
 * 组织状态兼容层。
 *
 * 设计闭集为 `active / suspended`（docs/design/iam/organizations.md）。
 * 历史数据可能存有 `normal`（可用）、`disabled` / `archived`（停用）等旧值；
 * 读取与展示统一归一化到闭集，保存时按新值写回，避免旧记录被当成异常值。
 */
const LEGACY_STATUS_ALIASES: Record<string, string> = {
  normal: "active",
  disabled: "suspended",
  archived: "suspended",
};

export function normalizeOrganizationStatus(status?: string | null): string {
  if (!status) return "";
  return LEGACY_STATUS_ALIASES[status] ?? status;
}
