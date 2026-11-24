import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * AppShell 布局 Page Object
 *
 * 封装主布局（侧边栏、顶栏、面包屑）的定位器和交互逻辑。
 */
export class AppShellPage extends BasePage {
  // ── 布局元素 ──
  readonly sidebar: Locator;
  readonly header: Locator;
  readonly breadcrumb: Locator;
  readonly logoutButton: Locator;
  readonly userMenuButton: Locator;
  readonly sidebarToggle: Locator;
  readonly themeButton: Locator;
  readonly languageButton: Locator;
  readonly orgSelect: Locator;

  constructor(page: Page) {
    super(page);
    this.sidebar = page.locator(".ant-layout-sider, .gl-sidebar").first();
    this.header = page.locator(".gl-header").first();
    this.breadcrumb = page.locator(".ant-breadcrumb").first();
    // 退出登录在昵称下拉菜单内（AppShell 的 Dropdown），不是顶栏独立按钮。
    this.userMenuButton = page.locator(".gl-header__user").first();
    this.logoutButton = page.getByRole("menuitem", { name: /logout|登出|退出/i }).first();
    this.sidebarToggle = page.locator(".gl-sidebar__toggle-btn").first();
    this.themeButton = page.getByRole("button", { name: /theme|主题/i }).first();
    this.languageButton = page.locator("button:has(.anticon-global)").first();
    this.orgSelect = page.locator(".gl-header__meta .ant-select").first();
  }

  // ── 侧边栏操作 ──

  async clickSidebarMenu(label: string | RegExp) {
    const menuItem = this.page.locator(".ant-menu-item").filter({ hasText: label }).first();
    await menuItem.click();
  }

  /**
   * 在指定分组内点击菜单项。
   *
   * 顶层 Home 分组和 Identity 分组都可能有同名项（例如 "Applications"），
   * 全局 `filter({hasText})` 会命中第一个（/apps），与调用方想要的管理页
   * （/applications）不同。按分组限定可消除歧义。
   */
  async clickSidebarMenuInGroup(group: string | RegExp, label: string | RegExp) {
    const submenu = this.page
      .locator(".ant-menu-submenu")
      .filter({ has: this.page.locator(".ant-menu-submenu-title", { hasText: group }) })
      .first();
    await submenu.locator(".ant-menu-item").filter({ hasText: label }).first().click();
  }

  async expandSidebarGroup(label: string | RegExp) {
    const submenu = this.page.locator(".ant-menu-submenu-title").filter({ hasText: label }).first();
    await submenu.click();
  }

  async toggleSidebar() {
    await this.sidebarToggle.click();
  }

  // ── 面包屑操作 ──

  async getBreadcrumbTexts(): Promise<string[]> {
    const items = this.page.locator(".ant-breadcrumb-link");
    const count = await items.count();
    const texts: string[] = [];
    for (let i = 0; i < count; i++) {
      texts.push(await items.nth(i).innerText());
    }
    return texts;
  }

  // ── 登出操作 ──

  async logout() {
    // 先展开昵称下拉菜单，再点菜单项；菜单项文案随 i18n 变化，用正则匹配。
    await this.userMenuButton.click();
    await this.logoutButton.click();
    // 等登录表单真正出现再返回：navigate("/login") 与 onSettled 的会话清理
    // 是同一批异步动作，紧接着 page.goto() 会和它竞争，导致偶发落在 "/"。
    await this.page
      .locator('input[id="username"], input[name="username"]')
      .first()
      .waitFor({ state: "visible", timeout: 10_000 });
  }

  // ── 验证 ──

  async expectSidebarVisible() {
    await this.sidebar.waitFor({ state: "visible" });
  }

  async expectHeaderVisible() {
    await this.header.waitFor({ state: "visible" });
  }

  async expectBreadcrumbVisible() {
    await this.breadcrumb.waitFor({ state: "visible" });
  }
}
