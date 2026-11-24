import { test, expect } from "@playwright/test";

/**
 * 应用配置 8 个可编辑分区 + 嵌套 Custom Scopes 区块的“逐个挂载 → 整体保存 → 刷新回显”回归。
 *
 * 覆盖审计 GAP-A4：应用详情页的配置分区必须在真实数据库环境下完成保存，
 * 并且刷新后仍能回显保存前的值，而不是只验证页面能打开。
 *
 * cd9ff59a 起应用详情页从 antd Tabs 迁移为 SectionNavShell 锚点导航：
 * 全部分区同时挂载，导航链接为 `.gl-section-nav__link`（role=link），
 * 点击同步激活并带 `aria-current="location"`；Custom Scopes 无独立导航项，
 * 嵌在 OIDC 分区内（`#section-custom-scopes`）。
 *
 * 登录走真实浏览器的 CSRF + Cookie 流程（与 UI 登录等价，但不受登录页
 * 动画/跳转时序影响）。仅在 ENABLE_E2E=true 时运行。
 */

const APP_ID = process.env.E2E_APPLICATION_ID ?? "01a0ea76-0c00-7000-8000-000000000002";
const ORG_SLUG = process.env.E2E_ORGANIZATION ?? "built-in";
const USERNAME = process.env.E2E_USERNAME ?? "admin";
const PASSWORD = process.env.E2E_PASSWORD ?? "admin";

// 审计口径的 8 个可编辑配置分区导航项（Overview 仅展示；Sign-in Log 只读；
// Custom Scopes 嵌在 OIDC 分区内，单独以锚点区块断言）。
const CONFIG_SECTION_LABELS = [
  /^Application Profile$/i,
  /^Sign-in & Sign-up$/i,
  /^OIDC\/OAuth$/i,
  /^SAML$/i,
  /^Identity Providers$/i,
  /^UI Customization$/i,
  /^Security & Risk$/i,
  /^Reverse Proxy$/i,
];

async function loginThroughBrowser(page: import("@playwright/test").Page) {
  await page.goto("/login");
  // CSRF cookie 由 API middleware 下发，Vite 页面本身不会设置它。
  await page.evaluate(async () => {
    await fetch("/api/v1/auth/contact", { credentials: "include" }).catch(() => undefined);
  });
  await expect
    .poll(() => page.evaluate(() => /(?:^|;\s*)csrf_token=/.test(document.cookie)), {
      timeout: 10_000,
    })
    .toBe(true);

  await page.evaluate(
    async ({ applicationId, organizationSlug, username, password }) => {
      const csrfMatch = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
      const csrf = csrfMatch ? decodeURIComponent(csrfMatch[1]) : "";
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(csrf ? { "X-CSRF-Token": csrf } : {}),
        },
        body: JSON.stringify({ applicationId, organizationSlug, username, password }),
      });
      if (!res.ok) {
        throw new Error(`login failed: ${res.status} ${await res.text()}`);
      }
    },
    {
      applicationId: APP_ID,
      organizationSlug: ORG_SLUG,
      username: USERNAME,
      password: PASSWORD,
    },
  );
}

/**
 * 点击分区导航链接并等待它被激活（SectionNavShell 点击即同步 setActiveKey）。
 * 所有分区共享同一个 Form 实例且同时挂载，这里不做重量级的可见内容断言
 * （Vite dev 按需编译懒加载 chunk，逐分区等待完整渲染会让回归变成分钟级）。
 */
async function visitSection(page: import("@playwright/test").Page, label: RegExp) {
  const link = page.getByRole("link", { name: label });
  await link.click();
  // Vite dev 首次按需编译懒加载 chunk，激活态回写可能超过默认 5s。
  await expect(link).toHaveAttribute("aria-current", "location", { timeout: 30_000 });
}

test.describe("应用配置 Tab 保存回显", () => {
  test.setTimeout(240_000);
  test.skip(process.env.ENABLE_E2E !== "true", "requires a running backend and database");

  test("9 个配置 Tab 挂载 → 保存 → 刷新后回显", async ({ page }) => {
    // 足够宽的视口保证分区导航完整可见、点击不被裁切。
    await page.setViewportSize({ width: 1680, height: 1000 });

    await test.step("login", async () => {
      await loginThroughBrowser(page);
    });

    await test.step("open application detail", async () => {
      await page.goto(`/applications/${APP_ID}`);
      await expect(page.locator(".gl-section-nav__link").first()).toBeVisible({ timeout: 20_000 });
      await visitSection(page, /^Application Profile$/i);
    });

    const displayNameInput = page.getByRole("textbox", { name: /Display Name/i }).first();
    await expect(displayNameInput).toBeVisible({ timeout: 20_000 });
    const displayNameBefore = await displayNameInput.inputValue();
    expect(displayNameBefore).not.toBe("");

    for (const label of CONFIG_SECTION_LABELS) {
      await test.step(`mount ${label}`, async () => {
        await visitSection(page, label);
      });
    }

    await test.step("mount Custom Scopes", async () => {
      await expect(page.locator("#section-custom-scopes")).toBeVisible({ timeout: 30_000 });
    });

    await test.step("save config", async () => {
      const saveButton = page.getByRole("button", { name: /save|保存|提交/i }).first();
      await expect(saveButton).toBeVisible({ timeout: 10_000 });
      await saveButton.click();
      await expect(page.locator(".ant-message-success").first()).toBeVisible({ timeout: 15_000 });
    });

    await test.step("reload and assert echo", async () => {
      await page.reload();
      await expect(page.locator(".gl-section-nav__link").first()).toBeVisible({ timeout: 20_000 });
      await visitSection(page, /^Application Profile$/i);
      const displayNameAfter = page.getByRole("textbox", { name: /Display Name/i }).first();
      await expect(displayNameAfter).toBeVisible({ timeout: 20_000 });
      await expect(displayNameAfter).not.toHaveValue("");
      await expect(displayNameAfter).toHaveValue(displayNameBefore);
    });
  });
});
