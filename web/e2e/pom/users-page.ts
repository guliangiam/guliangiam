import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 用户列表页 Page Object
 *
 * 封装用户管理列表的定位器和交互逻辑。
 * 路由: /users
 */
export class UsersPage extends BasePage {
  // ── 页面元素 ──
  readonly searchInput: Locator;
  readonly newUserButton: Locator;
  readonly userTable: Locator;
  readonly importButton: Locator;
  readonly exportButton: Locator;

  constructor(page: Page) {
    super(page);
    this.searchInput = page
      .locator(
        'input[placeholder*="search" i], input[placeholder*="搜索"], .ant-input-search input, input[placeholder*="name" i]',
      )
      .first();
    this.newUserButton = page.getByRole("button", { name: /new\s*user|新建用户/i }).first();
    this.userTable = page.locator(".ant-table").first();
    this.importButton = page.getByRole("button", { name: /import|导入/i }).first();
    this.exportButton = page.getByRole("button", { name: /export|导出/i }).first();
  }

  // ── 导航 ──

  async gotoUsers() {
    await this.goto("/users");
  }

  // ── 搜索 ──

  async searchUser(query: string) {
    await this.searchInput.fill(query);
    await this.searchInput.press("Enter");
  }

  async clearSearch() {
    await this.searchInput.clear();
    await this.searchInput.press("Enter");
  }

  // ── 列表操作 ──

  async clickNewUser() {
    await this.newUserButton.click();
  }

  async getTableRowByUsername(username: string): Promise<Locator> {
    return this.page.locator(".ant-table-row").filter({ hasText: username }).first();
  }

  async clickEditUser(username: string) {
    const row = await this.getTableRowByUsername(username);
    const editBtn = row.locator('button:has(.anticon-edit), [aria-label*="edit" i]').first();
    await editBtn.click();
  }

  async clickDeleteUser(username: string) {
    const row = await this.getTableRowByUsername(username);
    const deleteBtn = row.locator('button:has(.anticon-delete), [aria-label*="delete" i]').first();
    await deleteBtn.click();
  }

  async confirmDelete() {
    const confirmBtn = this.page.locator(".ant-popconfirm .ant-btn-primary").first();
    await confirmBtn.click();
  }

  // ── 验证 ──

  async expectUsersPageVisible() {
    await this.userTable.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectUserInTable(username: string) {
    const row = await this.getTableRowByUsername(username);
    await row.waitFor({ state: "visible", timeout: 10_000 });
  }

  async expectUserNotInTable(username: string) {
    const row = await this.getTableRowByUsername(username);
    await row.waitFor({ state: "hidden", timeout: 10_000 });
  }
}
