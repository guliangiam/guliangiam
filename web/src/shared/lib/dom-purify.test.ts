import { afterEach, describe, expect, it, vi } from "vitest";
import { sanitizeCss, sanitizeHtml } from "./dom-purify";

describe("sanitizeCss", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("preserves harmless declarations", () => {
    const out = sanitizeCss("body { background: linear-gradient(#fff, #000); color: #333; }");
    expect(out).toContain("body {");
    expect(out).toContain("background:");
    expect(out).toContain("color:");
    expect(out).not.toContain("<");
    expect(out).not.toContain(">");
  });

  it("keeps @media wrappers used by the login shell layout", () => {
    const out = sanitizeCss("@media (min-width: 769px) { .x { color: red; } }");
    expect(out).toContain("@media");
    expect(out).toContain("(min-width: 769px)");
    expect(out).toContain(".x");
  });

  it("keeps @keyframes but drops frames with unsafe values", () => {
    const out = sanitizeCss(
      "@keyframes fade { from { opacity: 0 } to { opacity: 1; background: url(https://evil.example/x) } }",
    );
    expect(out).toContain("@keyframes fade");
    expect(out).toContain("opacity");
    expect(out).not.toContain("url(");
    expect(out).not.toContain("evil.example");
  });

  it("blocks HTML breakout from a style element", () => {
    const out = sanitizeCss("body{color:red}</style><script>alert(1)</script>");
    expect(out).not.toContain("<");
    expect(out).not.toContain(">");
    expect(out).not.toContain("<script");
    expect(out).not.toContain("alert");
  });

  it("drops url() values, including escaped reconstructions", () => {
    expect(sanitizeCss("a{background:url(https://evil.example/x)}")).not.toContain("url(");
    expect(sanitizeCss("a{background:u\\72l(https://evil.example/x)}")).not.toContain("url(");
    expect(sanitizeCss("a{background:u\\000072l(https://evil.example/x)}")).not.toContain("url(");
    expect(sanitizeCss("a{background:url/**/(https://evil.example/x)}")).not.toContain("url(");
    expect(sanitizeCss("a{background:url(https://evil.example/x)}")).not.toContain("evil.example");
  });

  it("drops @import rules entirely", () => {
    const out = sanitizeCss("@import url(https://evil.example/a.css); a { color: red; }");
    expect(out).not.toContain("@import");
    expect(out).not.toContain("evil.example");
    expect(out).toContain("a {");
  });

  it("drops declarations outside the property allow-list", () => {
    const out = sanitizeCss(
      "a { color: red; width: expression(alert(1)); behavior: url(x); -moz-binding: url(y); position: fixed; }",
    );
    expect(out).toContain("color:");
    expect(out).toContain("position:");
    expect(out).not.toContain("expression(");
    expect(out).not.toContain("behavior");
    expect(out).not.toContain("-moz-binding");
  });

  it("neutralises HTML-breakout selectors while keeping safe siblings", () => {
    const out = sanitizeCss("a<svg/onload=alert(1)> { color: red; } .ok { color: blue; }");
    // Angle brackets are unconditional HTML breakouts and must never survive.
    expect(out).not.toContain("<");
    expect(out).not.toContain(">");
    expect(out).toContain(".ok");
  });

  it("keeps allow-listed custom properties but strips unsafe values", () => {
    const out = sanitizeCss(":root { --brand: #123456; --leak: url(https://evil.example/x); }");
    expect(out).toContain("--brand");
    expect(out).not.toContain("url(");
    expect(out).not.toContain("--leak");
  });

  it("returns an empty string for CSS that yields no safe rules", () => {
    expect(
      sanitizeCss("@font-face { font-family: x; src: url(https://evil.example/f.woff) }"),
    ).toBe("");
  });

  it("falls back to the regex path when CSSOM is unavailable", () => {
    vi.stubGlobal("CSSStyleSheet", undefined);
    const out = sanitizeCss(
      "@import url(https://evil.example/a.css); body{background:url(https://evil.example/x)}</style>",
    );
    expect(out).not.toContain("url(");
    expect(out).not.toContain("@import");
    expect(out).not.toContain("<");
    expect(out).not.toContain(">");
  });
});

describe("sanitizeCss selector scoping", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const SCOPE = ".login-content";

  it("prefixes tenant selectors with the shell scope", () => {
    const out = sanitizeCss(".btn { color: red; }", SCOPE);
    expect(out).toContain(`${SCOPE} .btn {`);
  });

  it("scopes every selector in a comma list", () => {
    const out = sanitizeCss("a, .b { color: red; }", SCOPE);
    expect(out).toContain(`${SCOPE} a, ${SCOPE} .b {`);
  });

  it("maps root selectors onto the scope element itself", () => {
    const out = sanitizeCss(
      "body { color: red; } :root { --brand: #123456; } html.dark { color: blue; } body .x { color: green; }",
      SCOPE,
    );
    expect(out).toContain(`${SCOPE} {`);
    expect(out).toContain("--brand");
    expect(out).toContain(`${SCOPE}.dark {`);
    expect(out).toContain(`${SCOPE} .x {`);
    expect(out).not.toContain("body");
    expect(out).not.toContain(":root");
  });

  it("scopes rules nested inside @media", () => {
    const out = sanitizeCss("@media (max-width: 768px) { .x { color: red; } }", SCOPE);
    expect(out).toContain("@media (max-width: 768px)");
    expect(out).toContain(`${SCOPE} .x {`);
  });

  it("leaves keyframe selectors untouched", () => {
    const out = sanitizeCss("@keyframes fade { from { opacity: 0 } to { opacity: 1 } }", SCOPE);
    expect(out).toContain("@keyframes fade");
    expect(out).toContain("0% {");
    expect(out).not.toContain(`${SCOPE} 0%`);
    expect(out).not.toContain(`${SCOPE} from`);
  });

  it("does not double-prefix selectors already carrying the scope", () => {
    const out = sanitizeCss(
      `${SCOPE} .x { color: red; } ${SCOPE} { color: blue; } ${SCOPE}.compact { color: green; }`,
      SCOPE,
    );
    expect(out).not.toContain(`${SCOPE} ${SCOPE}`);
    expect(out).toContain(`${SCOPE} .x {`);
    expect(out).toContain(`${SCOPE}.compact {`);
  });

  it("keeps selectors global when no scope is requested", () => {
    const out = sanitizeCss("body { color: red; }");
    expect(out).toContain("body {");
    expect(out).not.toContain(SCOPE);
  });

  it("returns unscoped sanitised CSS on the regex fallback path", () => {
    vi.stubGlobal("CSSStyleSheet", undefined);
    const out = sanitizeCss("body { color: red; }", SCOPE);
    expect(out).toContain("body");
    expect(out).not.toContain(SCOPE);
  });
});

describe("sanitizeHtml", () => {
  it("drops script elements", () => {
    const out = sanitizeHtml("<p>hi</p><script>alert(1)</script>");
    expect(out).toContain("<p>hi</p>");
    expect(out).not.toContain("<script");
  });

  it("drops inline style attributes so CSS cannot exfiltrate data", () => {
    const out = sanitizeHtml(
      '<span style="background-image:url(https://evil.example/pixel)">hi</span>',
    );
    expect(out).toContain("hi");
    expect(out).not.toContain("style=");
    expect(out).not.toContain("evil.example");
  });
});
