import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 组织列表页 Page Object
 *
 * 封装组织管理列表的定位器和交互逻辑。
 * 路由: /organizations
 */
export class OrganizationsPage extends BasePage {
  // ── 页面元素 ──
  readonly searchInput: Locator;
  readonly newOrgButton: Locator;
  readonly orgTable: Locator;

  constructor(page: Page) {
    super(page);
    this.searchInput = page
      .locator(
        'input[placeholder*="search" i], input[placeholder*="搜索"], .ant-input-search input',
      )
      .first();
    this.newOrgButton = page.getByRole("button", { name: /new\s*organization|新建组织/i }).first();
    this.orgTable = page.locator(".ant-table").first();
  }

  // ── 导航 ──

  async gotoOrganizations() {
    await this.goto("/organizations");
  }

  // ── 搜索 ──

  async searchOrganization(query: string) {
    await this.searchInput.fill(query);
    await this.searchInput.press("Enter");
  }

  // ── 列表操作 ──

  async clickNewOrganization() {
    await this.newOrgButton.click();
  }

  async getTableRowByName(name: string): Promise<Locator> {
    return this.page.locator(".ant-table-row").filter({ hasText: name }).first();
  }

  async clickEditOrganization(name: string) {
    const row = await this.getTableRowByName(name);
    const editLink = row.getByRole("link", { name: /edit|编辑/i }).first();
    await editLink.click();
  }

  async clickDeleteOrganization(name: string) {
    const row = await this.getTableRowByName(name);
    const deleteBtn = row.getByRole("button", { name: /delete|删除/i }).first();
    await deleteBtn.click();
  }

  async confirmDelete() {
    const confirmBtn = this.page.locator(".ant-popconfirm .ant-btn-primary").first();
    await confirmBtn.click();
  }

  // ── 验证 ──

  async expectOrganizationsPageVisible() {
    await this.orgTable.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectOrganizationInTable(name: string) {
    const row = await this.getTableRowByName(name);
    await row.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectOrganizationNotInTable(name: string) {
    const row = await this.getTableRowByName(name);
    await row.waitFor({ state: "hidden", timeout: 10_000 });
  }
}
