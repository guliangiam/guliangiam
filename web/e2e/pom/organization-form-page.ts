import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 组织创建/编辑页 Page Object
 *
 * 封装组织表单的定位器和交互逻辑。
 * 路由: /organizations/new, /organizations/:orgName
 */
export class OrganizationFormPage extends BasePage {
  // ── 表单元素 ──
  readonly slugInput: Locator;
  readonly displayNameInput: Locator;
  readonly websiteUrlInput: Locator;
  readonly logoUrlInput: Locator;
  readonly statusSelect: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  constructor(page: Page) {
    super(page);
    this.slugInput = page.locator('input[name="slug"], #slug').first();
    this.displayNameInput = page.locator('input[name="displayName"], #displayName').first();
    this.websiteUrlInput = page.locator('input[name="websiteUrl"], #websiteUrl').first();
    this.logoUrlInput = page.locator('input[name="logoUrl"], #logoUrl').first();
    this.statusSelect = page
      .locator(".ant-form-item")
      .filter({ hasText: /status|状态/i })
      .locator(".ant-select")
      .first();
    this.saveButton = page.getByRole("button", { name: /save|submit|保存|提交/i }).first();
    this.cancelButton = page.getByRole("button", { name: /cancel|取消/i }).first();
  }

  // ── 导航 ──

  async gotoCreateOrganization() {
    await this.goto("/organizations/new");
  }

  // ── 表单操作 ──

  async fillSlug(slug: string) {
    await this.slugInput.fill(slug);
  }

  async fillDisplayName(displayName: string) {
    await this.displayNameInput.fill(displayName);
  }

  async fillWebsiteUrl(url: string) {
    await this.websiteUrlInput.fill(url);
  }

  async fillLogoUrl(url: string) {
    await this.logoUrlInput.fill(url);
  }

  async selectStatus(status: string) {
    await this.statusSelect.click();
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
   * 创建组织完整流程
   */
  async createOrganization(params: { slug: string; displayName: string; websiteUrl?: string }) {
    await this.gotoCreateOrganization();
    await this.fillSlug(params.slug);
    await this.fillDisplayName(params.displayName);
    if (params.websiteUrl) await this.fillWebsiteUrl(params.websiteUrl);
    await this.clickSave();
  }

  // ── 验证 ──

  async expectFormVisible() {
    await this.slugInput.waitFor({ state: "visible" });
  }
}
