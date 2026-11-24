import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { FIXTURES_FILE } from "./fixtures";
import { shouldRunE2E } from "./helpers";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const AUTH_STATE = path.resolve(HERE, ".auth/admin.json");
if (fs.existsSync(AUTH_STATE)) test.use({ storageState: AUTH_STATE });

/**
 * 从真实会话解析动态路由夹具，避免 route-sweep 写死 UUID 后与库内对象漂移。
 * 只读会话，不创建业务对象：种子 built-in 组织/用户已覆盖大多数详情页；
 * 其余实体详情允许「对象不存在」错误面（route-sweep 已显式接受该分支）。
 */
test("prepare deterministic fixtures", async ({ request }) => {
  if (!shouldRunE2E) test.skip(true, "ENABLE_E2E not set; skipping fixtures setup");

  const response = await request.get("/api/v1/auth/account");
  expect(response.ok(), `fixtures 读取会话失败 status=${response.status()}`).toBeTruthy();
  const body = (await response.json()) as {
    organization?: { id?: string; name?: string; slug?: string };
    account?: { id?: string; name?: string; displayName?: string };
  };

  const org = body.organization ?? {};
  const account = body.account ?? {};
  const username = account.name ?? process.env.E2E_USERNAME ?? "admin";

  let userId = account.id ?? "";
  if (org.id) {
    const users = await request.get(`/api/v1/users?organizationId=${org.id}&pageSize=1`);
    if (users.ok()) {
      const payload = (await users.json()) as { items?: Array<{ id?: string }> };
      userId = payload.items?.[0]?.id ?? userId;
    }
  }
  userId = userId || org.id || "";

  const fixtures: Record<string, string> = {
    organizationId: org.id ?? "",
    orgId: org.id ?? "",
    organizationName: org.name ?? org.slug ?? "built-in",
    orgName: org.name ?? org.slug ?? "built-in",
    organizationSlug: org.slug ?? org.name ?? "built-in",
    owner: org.name ?? org.slug ?? "built-in",
    userId,
    username,
    userName: username,
    id: userId || org.id || "",
    webhookId: userId || org.id || "",
    applicationName: "app-built-in",
    appName: "app-built-in",
    applicationSlug: "app-built-in",
  };
  for (const [key, value] of Object.entries(fixtures)) {
    if (!value) throw new Error(`fixtures 缺少 ${key}`);
  }

  fs.mkdirSync(path.dirname(FIXTURES_FILE), { recursive: true });
  fs.writeFileSync(FIXTURES_FILE, `${JSON.stringify(fixtures, null, 2)}\n`, "utf8");
});
