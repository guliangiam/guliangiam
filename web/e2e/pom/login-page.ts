import type { Page, Locator } from "@playwright/test";
import { BasePage } from "./base";

/**
 * 登录页 Page Object
 *
 * 封装登录表单的定位器和交互逻辑。
 * 登录页路由: /login
 */
export class LoginPage extends BasePage {
  // ── 表单元素 ──
  readonly usernameInput: Locator;
  readonly passwordInput: Locator;
  readonly loginButton: Locator;
  readonly signupLink: Locator;
  readonly forgotPasswordLink: Locator;

  constructor(page: Page) {
    super(page);
    this.usernameInput = page.locator('input[id="username"], input[name="username"]').first();
    this.passwordInput = page.locator('input[type="password"]').first();
    this.loginButton = page.getByRole("button", { name: /sign\s*in|登录/i }).first();
    this.signupLink = page.getByRole("link", { name: /sign\s*up|注册/i }).first();
    this.forgotPasswordLink = page
      .getByRole("link", { name: /forgot\s*password|忘记密码/i })
      .first();
  }

  // ── 导航 ──

  async gotoLogin() {
    await this.goto("/login");
  }

  // ── 表单操作 ──

  // 登录壳异步拉取应用定制后会重渲染表单，可能清空刚 fill 的受控值，
  // 立即点击登录会间歇提交空用户名（E2E-03）。等值落定，被清空则重填。
  private async fillStable(input: Locator, value: string) {
    for (let attempt = 0; attempt < 5; attempt++) {
      await input.fill(value);
      await this.page.waitForTimeout(200);
      if ((await input.inputValue()) === value) return;
    }
    throw new Error(`login input value did not stabilize (${value.length} chars)`);
  }

  async fillUsername(username: string) {
    await this.fillStable(this.usernameInput, username);
  }

  async fillPassword(password: string) {
    await this.fillStable(this.passwordInput, password);
  }

  async clickLogin() {
    await this.loginButton.click();
  }

  /**
   * 完整登录流程：填写用户名密码 → 点击登录
   */
  async loginAs(username: string, password: string) {
    await this.gotoLogin();
    await this.fillUsername(username);
    await this.fillPassword(password);
    await this.clickLogin();
  }

  /**
   * 登录并等待控制台外壳挂载完成。
   *
   * URL 变成 "/" 只说明 navigate 已发生；此时 AppShell 仍在拉取会话，主区域
   * 还是 Skeleton，侧边栏菜单尚未渲染。后续用例若立即统计/点击菜单会命中
   * 「元素不存在」的假失败，因此这里等到 header 真正可见再返回。
   */
  async loginAndWaitForDashboard(username: string, password: string) {
    await this.loginAs(username, password);
    await this.waitForUrl(/^.*\/$/, 10_000);
    await this.page.locator(".gl-header").first().waitFor({ state: "visible", timeout: 15_000 });
  }

  // ── 验证 ──

  async expectLoginFormVisible() {
    await this.usernameInput.waitFor({ state: "visible" });
    await this.passwordInput.waitFor({ state: "visible" });
  }

  async expectOnLoginPage() {
    await this.page.waitForURL(/\/login/);
  }
}
