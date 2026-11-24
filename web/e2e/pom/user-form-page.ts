import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 用户创建/编辑页 Page Object
 *
 * 封装用户表单的定位器和交互逻辑。
 * 路由: /users/new, /users/:orgName/:username
 */
export class UserFormPage extends BasePage {
  // ── 表单元素 ──
  readonly usernameInput: Locator;
  readonly displayNameInput: Locator;
  readonly emailInput: Locator;
  readonly phoneInput: Locator;
  readonly organizationSelect: Locator;
  readonly userTypeSelect: Locator;
  readonly statusSelect: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  constructor(page: Page) {
    super(page);
    this.usernameInput = page.locator('input[name="username"], #username').first();
    this.displayNameInput = page.locator('input[name="displayName"], #displayName').first();
    this.emailInput = page.locator('input[name="email"], #email, input[type="email"]').first();
    this.phoneInput = page.locator('input[name="phone"], #phone').first();
    this.organizationSelect = page
      .locator(".ant-select")
      .filter({ hasText: /organization/i })
      .first();
    this.userTypeSelect = page.locator('input[name="userType"], #userType').first();
    this.statusSelect = page.locator('input[name="status"], #status').first();
    this.saveButton = page.getByRole("button", { name: /save|submit|保存|提交/i }).first();
    this.cancelButton = page.getByRole("button", { name: /cancel|取消/i }).first();
  }

  // ── 导航 ──

  async gotoCreateUser() {
    await this.goto("/users/new");
  }

  // ── 表单操作 ──

  async fillUsername(username: string) {
    await this.usernameInput.fill(username);
  }

  async fillDisplayName(displayName: string) {
    await this.displayNameInput.fill(displayName);
  }

  async fillEmail(email: string) {
    await this.emailInput.fill(email);
  }

  async fillPhone(phone: string) {
    await this.phoneInput.fill(phone);
  }

  async selectOrganization(orgName: string) {
    await this.organizationSelect.click();
    const option = this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: orgName })
      .first();
    await option.click();
  }

  async selectUserType(userType: string) {
    // 找到 userType 对应的 Select 组件
    const formItem = this.page
      .locator(".ant-form-item")
      .filter({ hasText: /user\s*type|用户类型/i })
      .first();
    const select = formItem.locator(".ant-select").first();
    await select.click();
    const option = this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: new RegExp(userType, "i") })
      .first();
    await option.click();
  }

  async selectStatus(status: string) {
    const formItem = this.page
      .locator(".ant-form-item")
      .filter({ hasText: /status|状态/i })
      .first();
    const select = formItem.locator(".ant-select").first();
    await select.click();
    const option = this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: new RegExp(status, "i") })
      .first();
    await option.click();
  }

  async clickSave() {
    await this.saveButton.click();
  }

  async clickCancel() {
    await this.cancelButton.click();
  }

  /**
   * 创建用户完整流程
   */
  async createUser(params: {
    username: string;
    displayName: string;
    email?: string;
    phone?: string;
  }) {
    await this.gotoCreateUser();
    await this.fillUsername(params.username);
    await this.fillDisplayName(params.displayName);
    if (params.email) await this.fillEmail(params.email);
    if (params.phone) await this.fillPhone(params.phone);
    await this.clickSave();
  }

  // ── 验证 ──

  async expectFormVisible() {
    await this.usernameInput.waitFor({ state: "visible" });
  }
}
