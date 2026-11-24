import type { Page } from "@playwright/test";
import { LoginPage } from "./pom/login-page";

/**
 * 测试凭据配置
 *
 * 在 CI 环境中可通过环境变量覆盖默认值。
 */
export const TEST_CREDENTIALS = {
  username: process.env.E2E_USERNAME ?? "admin",
  password: process.env.E2E_PASSWORD ?? "admin123",
};

/**
 * 是否启用需要后端的 E2E 测试
 */
export const shouldRunE2E = process.env.ENABLE_E2E === "true";

/**
 * 条件性 describe：仅在 ENABLE_E2E=true 时执行
 */
export function describeIfE2E(name: string, fn: () => void) {
  if (shouldRunE2E) {
    // eslint-disable-next-line no-restricted-imports -- dynamic describe
    const { test } = require("@playwright/test");
    test.describe(name, fn);
  } else {
    const { test } = require("@playwright/test");
    test.describe.skip(name, fn);
  }
}

/**
 * 登录辅助函数：使用 Page Object 模式完成登录
 */
export async function loginAsAdmin(page: Page) {
  const loginPage = new LoginPage(page);
  await loginPage.loginAndWaitForDashboard(TEST_CREDENTIALS.username, TEST_CREDENTIALS.password);
}

/**
 * 登录辅助函数：使用指定凭据登录
 */
export async function loginAs(page: Page, username: string, password: string) {
  const loginPage = new LoginPage(page);
  await loginPage.loginAndWaitForDashboard(username, password);
}
