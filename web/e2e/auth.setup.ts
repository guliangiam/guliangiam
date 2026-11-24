import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as setup, expect } from "@playwright/test";
import { TEST_CREDENTIALS, shouldRunE2E } from "./helpers";
import { LoginPage } from "./pom/login-page";

// 复用登录态：route-sweep 的 130+ 页面不应各自登录一次——既慢，也会撞上
// 认证面 per-IP 限流（默认 10 次/分钟）。这里登录一次并落盘 storageState。
const HERE = path.dirname(fileURLToPath(import.meta.url));
export const AUTH_STATE_FILE = path.resolve(HERE, ".auth/admin.json");

setup("authenticate as admin", async ({ page }) => {
  if (!shouldRunE2E) {
    setup.skip(true, "ENABLE_E2E not set; skipping auth setup");
  }

  const loginPage = new LoginPage(page);
  await loginPage.gotoLogin();
  await loginPage.fillUsername(TEST_CREDENTIALS.username);
  await loginPage.fillPassword(TEST_CREDENTIALS.password);
  await loginPage.clickLogin();
  await expect(page).toHaveURL(/^.*\/$/, { timeout: 15_000 });
  await page.locator(".gl-header").first().waitFor({ state: "visible", timeout: 15_000 });

  fs.mkdirSync(path.dirname(AUTH_STATE_FILE), { recursive: true });
  await page.context().storageState({ path: AUTH_STATE_FILE });
});
