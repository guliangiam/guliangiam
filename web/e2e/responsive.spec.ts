import { test, expect } from "@playwright/test";

/**
 * 布局响应式检查（宽度规范 + 视口矩阵）。
 *
 * 由 playwright.config.ts 的 viewport-390/768/1024/1440/1920 五档视口项目驱动，
 * 本文件只描述「在当前视口下应成立的布局不变量」，不再自带视口遍历：
 *   1. 文档不出现横向溢出（主显示区不错位）；
 *   2. 内容宽度档位封顶生效（列表 1600 / 表单详情 1200 / 阅读 960）；
 *   3. 表格横向滚动被限制在容器内，不把页面撑破；
 *   4. 表单控件不超出视口。
 *
 * 免登录可渲染的路径默认检查；需要会话的路径在未登录时被重定向到登录页，
 * 判为跳过而不是失败（与 a11y.spec.ts 同一口径）。提供完整环境并设置
 * ENABLE_E2E=true 后，同一批用例自动覆盖登录后的页面。
 *
 * 运行方式：
 *   npx playwright test e2e/responsive.spec.ts
 */

// 静态布局检查不需要录像；录像依赖 ffmpeg，缺失时会让整批用例在创建页面时就失败。
test.use({ video: "off" });

const LOGIN_PATH = "/login";

// 免登录可渲染路径 + 代表性应用路径（列表 / 表单 / 阅读三种宽度档各一）。
const ROUTES = [
  "/login",
  "/signup",
  "/forgot-password",
  "/device",
  "/users",
  "/users/new",
  "/settings",
  "/organizations",
];

// 宽度三档封顶（styles.css 的 .gl-content__inner 语义类）。
const WIDTH_CAPS: Array<[string, number]> = [
  [".gl-content__inner", 1600],
  [".gl-content__inner--form", 1200],
  [".gl-content__inner--reading", 960],
];

function redirectedToLogin(pathname: string, requested: string): boolean {
  return pathname !== requested && pathname.startsWith(LOGIN_PATH);
}

for (const route of ROUTES) {
  test(`布局响应式: ${route}`, async ({ page }) => {
    const response = await page.goto(route);
    expect(response?.status(), `${route} 应可访问（非 5xx）`).toBeLessThan(500);

    const requested = new URL(page.url()).pathname;
    test.skip(redirectedToLogin(requested, route), `${route} 需要登录会话（未提供后端环境）`);

    await page.waitForLoadState("networkidle");

    // 1. 文档无横向溢出：主显示区不错位、页面不被撑破。
    const overflow = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth: window.innerWidth,
    }));
    expect(
      overflow.scrollWidth,
      `${route} 在当前视口出现横向溢出（scrollWidth=${overflow.scrollWidth} > innerWidth=${overflow.innerWidth}）`,
    ).toBeLessThanOrEqual(overflow.innerWidth + 1);

    // 2. 内容宽度档位封顶。
    for (const [selector, cap] of WIDTH_CAPS) {
      const width = await page.evaluate((sel) => {
        const el = document.querySelector(sel);
        return el ? el.getBoundingClientRect().width : null;
      }, selector);
      if (width !== null) {
        expect(width, `${route} 的 ${selector} 超出宽度档 ${cap}px`).toBeLessThanOrEqual(cap + 1);
      }
    }

    // 3. 表格横向滚动被限制在容器内：可滚动时滚动容器必须裁剪内容。
    const tableProblems = await page.evaluate(() => {
      const problems: string[] = [];
      for (const container of document.querySelectorAll<HTMLElement>(".ant-table-container")) {
        const box = container.getBoundingClientRect();
        if (box.right > window.innerWidth + 1) {
          problems.push(`表格容器超出视口（right=${Math.round(box.right)}）`);
        }
        const scroller = container.querySelector<HTMLElement>(".ant-table-body");
        if (scroller && scroller.scrollWidth > scroller.clientWidth) {
          const overflowX = getComputedStyle(scroller).overflowX;
          if (overflowX === "visible") {
            problems.push("表格内容超宽但滚动容器 overflow-x=visible，无法横向滚动");
          }
        }
      }
      return problems;
    });
    expect(tableProblems, `${route} 表格布局异常：${tableProblems.join("；")}`).toEqual([]);

    // 4. 表单不超出视口。
    const formProblems = await page.evaluate(() => {
      const problems: string[] = [];
      for (const form of document.querySelectorAll<HTMLElement>(".ant-form")) {
        const box = form.getBoundingClientRect();
        if (box.width > 0 && box.right > window.innerWidth + 1) {
          problems.push(`表单超出视口（right=${Math.round(box.right)}）`);
        }
      }
      return problems;
    });
    expect(formProblems, `${route} 表单布局异常：${formProblems.join("；")}`).toEqual([]);
  });
}
