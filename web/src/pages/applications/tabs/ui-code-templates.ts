/** 「载入默认模板」基线：来自真实登录页 LoginShellLayout 实际消费的
 * .login-content / .gl-panel / .login-form 结构。
 * app_form_config 的 HTML/CSS 列为 TEXT（V17 容量迁移），无 255 字符限制。
 * 阴影/圆角规则统一用 `.login-content .gl-panel`（特异度 0,2,0），
 * 否则会被 Ant Design 的 `.ant-card:not(.ant-card-bordered)` 覆盖。
 * HTML 模板不得含内联 style：sanitizeHtml 会剥掉 style 属性。 */
export type UiCodeFieldKey =
  | "headerHtml"
  | "footerHtml"
  | "formSideHtml"
  | "formCss"
  | "formCssMobile"
  | "customCss";

export const UI_CODE_TEMPLATES: Record<UiCodeFieldKey, string> = {
  formCss: `.login-content .gl-panel {
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(15, 42, 37, 0.08);
}
.login-form { max-width: 420px; }
`,
  formCssMobile: `.login-content { padding: 16px; }
.login-content .gl-panel { box-shadow: none; }
.login-form { max-width: 100%; }
`,
  customCss: `.login-content .gl-panel .ant-card-body { padding: 24px; }
`,
  headerHtml: `<header class="login-custom-header-inner">
  <span class="brand">My Brand</span>
</header>
`,
  footerHtml: `<footer class="login-custom-footer-inner">
  <span>&copy; My Company</span>
</footer>
`,
  formSideHtml: `<div class="login-side">
  <h2>Welcome back</h2>
  <p>Sign in to continue.</p>
</div>
`,
};

/** 「恢复默认」按钮写入的页脚（无内联样式，sanitize 后仍完整生效）。 */
export const DEFAULT_FOOTER_HTML =
  '<footer class="login-custom-footer-inner"><span>Powered by GuliangIAM</span></footer>';
