import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 应用列表页 Page Object
 *
 * 封装应用管理列表的定位器和交互逻辑。
 * 路由: /applications
 */
export class ApplicationsPage extends BasePage {
  // ── 页面元素 ──
  readonly searchInput: Locator;
  readonly newAppButton: Locator;
  readonly appTable: Locator;

  constructor(page: Page) {
    super(page);
    this.searchInput = page
      .locator(
        'input[placeholder*="search" i], input[placeholder*="搜索"], input[placeholder*="application" i], .ant-input-search input',
      )
      .first();
    // 「New Application」现在是单个可聚焦按钮（LinkButton 用 useNavigate 跳转），
    // 不再是 <a> 里套 <button> 的嵌套交互控件。
    this.newAppButton = page.getByRole("button", { name: /new\s*application|新建应用/i }).first();
    this.appTable = page.locator(".ant-table").first();
  }

  // ── 导航 ──

  async gotoApplications() {
    await this.goto("/applications");
  }

  // ── 搜索 ──

  async searchApplication(query: string) {
    await this.searchInput.fill(query);
    await this.searchInput.press("Enter");
  }

  // ── 列表操作 ──

  async clickNewApplication() {
    await this.newAppButton.click();
  }

  async getTableRowByName(name: string): Promise<Locator> {
    return this.page.locator(".ant-table-row").filter({ hasText: name }).first();
  }

  async clickDetailApplication(name: string) {
    const row = await this.getTableRowByName(name);
    const detailLink = row.getByRole("link", { name: /detail|详情/i }).first();
    await detailLink.click();
  }

  async clickDeleteApplication(name: string) {
    const row = await this.getTableRowByName(name);
    const deleteBtn = row.getByRole("button", { name: /delete|删除/i }).first();
    await deleteBtn.click();
  }

  async confirmDelete() {
    const confirmBtn = this.page.locator(".ant-popconfirm .ant-btn-primary").first();
    await confirmBtn.click();
  }

  // ── 验证 ──

  async expectApplicationsPageVisible() {
    await this.appTable.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectApplicationInTable(name: string) {
    const row = await this.getTableRowByName(name);
    await row.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectApplicationNotInTable(name: string) {
    const row = await this.getTableRowByName(name);
    await row.waitFor({ state: "hidden", timeout: 10_000 });
  }
}
