import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 应用详情/编辑页 Page Object
 *
 * 封装应用配置表单的定位器和交互逻辑。
 * 路由: /applications/new, /applications/:applicationId
 */
export class ApplicationDetailPage extends BasePage {
  // ── 表单元素 ──
  readonly nameInput: Locator;
  readonly displayNameInput: Locator;
  readonly redirectUrisInput: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  constructor(page: Page) {
    super(page);
    this.nameInput = page.locator('input[name="name"], #name').first();
    this.displayNameInput = page.locator('input[name="displayName"], #displayName').first();
    this.redirectUrisInput = page
      .locator('input[name="redirectUris"], #redirectUris, textarea[name="redirectUris"]')
      .first();
    this.saveButton = page.getByRole("button", { name: /save|submit|保存|提交/i }).first();
    this.cancelButton = page.getByRole("button", { name: /cancel|取消/i }).first();
  }

  // ── 导航 ──

  async gotoCreateApplication() {
    await this.goto("/applications/new");
  }

  async gotoApplicationDetail(applicationId: string) {
    await this.goto(`/applications/${applicationId}`);
  }

  // ── 表单操作 ──

  async fillName(name: string) {
    await this.nameInput.fill(name);
  }

  async fillDisplayName(displayName: string) {
    await this.displayNameInput.fill(displayName);
  }

  async fillRedirectUris(uris: string) {
    await this.redirectUrisInput.fill(uris);
  }

  async clickSave() {
    await this.saveButton.click();
  }

  async clickCancel() {
    await this.cancelButton.click();
  }

  // ── 验证 ──

  async expectFormVisible() {
    await this.nameInput.waitFor({ state: "visible" });
  }
}
