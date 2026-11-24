import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { createTestWrapper } from "@/test-utils/render";
import { SignupPreviewFrame } from "./SignupPreviewFrame";

function renderFrame(props: Partial<Parameters<typeof SignupPreviewFrame>[0]> = {}) {
  return render(<SignupPreviewFrame {...props} />, { wrapper: createTestWrapper() });
}

describe("SignupPreviewFrame shell parity", () => {
  // 回归：注册预览曾只渲染 gl-auth-panel + signupItems，背景/CSS/位置等壳定制
  // 完全不反映，与真实注册页（LoginShellLayout）漂移。现与登录预览同一壳渲染链。
  it("injects scoped customCss through the shared shell", () => {
    const { container } = renderFrame({ customCss: ".gl-panel { border-radius: 4px; }" });
    const styles = Array.from(container.querySelectorAll("style"));
    expect(styles.some((s) => s.textContent?.includes(".login-content .gl-panel"))).toBe(true);
  });

  it("reflects the desktop background image on the shell", () => {
    const { container } = renderFrame({ formBackgroundUrl: "https://cdn.example.com/bg.png" });
    const shell = container.querySelector<HTMLElement>(".login-content");
    expect(shell?.style.background).toContain("cdn.example.com");
  });

  it("uses the mobile background image at the mobile breakpoint", () => {
    const { container } = renderFrame({
      compactBreakpoint: "mobile",
      formBackgroundUrl: "https://cdn.example.com/desktop.png",
      formBackgroundUrlMobile: "https://cdn.example.com/mobile.png",
    });
    const shell = container.querySelector<HTMLElement>(".login-content");
    expect(shell?.style.background).toContain("mobile.png");
    expect(shell?.style.background).not.toContain("desktop.png");
  });

  it("renders the full-page signupHtml override instead of the shell", () => {
    const { container } = renderFrame({ signupHtml: "<div>custom signup page</div>" });
    expect(container.querySelector(".login-signin-html-page")?.textContent).toContain(
      "custom signup page",
    );
    expect(container.querySelector(".login-content")).toBeNull();
  });

  it("renders the signup form heading and submit action", () => {
    const { container } = renderFrame();
    expect(container.querySelector(".gl-auth-form__title")).toBeTruthy();
    // 预览帧嵌在详情页外层 Form 内：不得产生嵌套 <form>（浏览器会丢弃内层表单，
    // 提交按钮失效）；提交按钮为 type=button 经 form store 触发。
    expect(container.querySelector("form")).toBeNull();
    const submit = container.querySelector<HTMLButtonElement>("button.ant-btn-primary");
    expect(submit).toBeTruthy();
    expect(submit?.type).toBe("button");
  });

  // 回归：副标题曾把 <strong> React 元素传给返回字符串的 t()，org 插值被
  // 字符串化为 "[object Object]"。现经 Trans components 渲染加粗组织名。
  it("renders the org name bold in the subtitle, never [object Object]", () => {
    const { container } = renderFrame();
    const subtitle = container.querySelector(".gl-auth-form__subtitle");
    expect(subtitle?.textContent).not.toContain("[object Object]");
    expect(subtitle?.textContent).toContain("built-in");
    const strong = subtitle?.querySelector("strong");
    expect(strong?.textContent).toBe("built-in");
  });
});
