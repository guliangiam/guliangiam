import type { Page, Locator } from "@playwright/test";

/**
 * Base Page Object — 所有 POM 的公共基类
 *
 * 封装了通用的导航、等待和元素定位逻辑，
 * 子类通过继承获得一致的基础能力。
 */
export class BasePage {
  readonly page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  // ── 导航 ──

  async goto(path: string) {
    await this.page.goto(path);
  }

  // ── 等待 ──

  async waitForUrl(pattern: RegExp | string, timeout = 10_000) {
    await this.page.waitForURL(pattern, { timeout });
  }

  async waitForSelector(selector: string, timeout = 10_000) {
    return this.page.waitForSelector(selector, { timeout });
  }

  // ── 通用元素定位 ──

  getBreadcrumb(): Locator {
    return this.page.locator(".ant-breadcrumb");
  }

  getSidebar(): Locator {
    return this.page.locator(".ant-layout-sider, .gl-sidebar").first();
  }

  getSidebarMenu(): Locator {
    return this.page.locator(".ant-menu");
  }

  // ── 通用操作 ──

  async clickMenuByLabel(label: string | RegExp) {
    const menuItem = this.page.locator(".ant-menu-item").filter({ hasText: label }).first();
    await menuItem.click();
  }

  async clickSubMenuByLabel(label: string | RegExp) {
    const submenu = this.page.locator(".ant-menu-submenu-title").filter({ hasText: label }).first();
    await submenu.click();
  }

  async clickBreadcrumbLink(text: string) {
    await this.page.locator(".ant-breadcrumb-link").filter({ hasText: text }).first().click();
  }

  // ── 通知/消息 ──

  async waitForSuccessMessage(timeout = 5_000): Promise<Locator> {
    const msg = this.page.locator(".ant-message-success").first();
    await msg.waitFor({ state: "visible", timeout });
    return msg;
  }

  async waitForErrorMessage(timeout = 5_000): Promise<Locator> {
    const msg = this.page.locator(".ant-message-error").first();
    await msg.waitFor({ state: "visible", timeout });
    return msg;
  }
}
