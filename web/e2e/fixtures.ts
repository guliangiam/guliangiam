import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const FIXTURES_FILE = path.resolve(HERE, ".auth/fixtures.json");

/**
 * 动态路由夹具的唯一来源：`fixtures.setup.ts` 从真实会话（/api/v1/auth/account）
 * 解析出的组织/用户标识，落盘到 `.auth/fixtures.json`。这里保留种子 built-in
 * 的后备值，避免夹具文件缺失时把动态路由静默跳过。
 */
const FALLBACK: Record<string, string> = {
  id: "01a0ea76-0c00-7000-8000-000000000003",
  userId: "01a0ea76-0c00-7000-8000-000000000003",
  organizationId: "01a0ea76-0c00-7000-8000-000000000001",
  orgId: "01a0ea76-0c00-7000-8000-000000000001",
  organizationName: "built-in",
  orgName: "built-in",
  organizationSlug: "built-in",
  owner: "built-in",
  applicationName: "app-built-in",
  appName: "app-built-in",
  applicationSlug: "app-built-in",
  username: "admin",
  userName: "admin",
  webhookId: "01a0ea76-0c00-7000-8000-000000000003",
};

export function loadFixtureIds(): Record<string, string> {
  try {
    const raw = fs.readFileSync(FIXTURES_FILE, "utf8");
    const parsed = JSON.parse(raw) as Record<string, string>;
    return { ...FALLBACK, ...parsed };
  } catch {
    return { ...FALLBACK };
  }
}

/** 把路由模板中的 `:param` 替换为真实对象标识。 */
export function materialize(route: string, ids: Record<string, string> = loadFixtureIds()): string {
  return route.replace(/:([A-Za-z0-9_]+)/g, (_, param: string) => ids[param] ?? ids.id);
}
