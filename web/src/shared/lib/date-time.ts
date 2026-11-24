/** 后端时间统一展示：空值、非法值与 Go 零值时间（0001-01-01）一律显示 "-"。 */
export function formatDateTime(value?: string | null): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime()) || parsed.getFullYear() <= 1) return "-";
  return parsed.toLocaleString();
}
