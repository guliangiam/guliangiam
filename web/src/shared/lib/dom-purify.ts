import DOMPurify from "dompurify";

// Configure DOMPurify with safe defaults for the IAM application.
// Allow common formatting tags but block dangerous elements.
const purifyConfig = {
  ALLOWED_TAGS: [
    "b",
    "i",
    "em",
    "strong",
    "a",
    "p",
    "br",
    "ul",
    "ol",
    "li",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "span",
    "div",
    "img",
    "table",
    "thead",
    "tbody",
    "tr",
    "th",
    "td",
    "blockquote",
    "code",
    "pre",
    "hr",
    "sub",
    "sup",
    "small",
  ],
  ALLOWED_ATTR: ["href", "src", "alt", "title", "class", "id", "target", "rel", "width", "height"],
  ALLOW_DATA_ATTR: false,
  FORBID_TAGS: ["script", "style", "iframe", "object", "embed", "form", "input"],
  FORBID_ATTR: ["onerror", "onload", "onclick", "onmouseover"],
};

/**
 * Sanitize HTML content to prevent XSS attacks.
 * Returns safe HTML string with dangerous elements/attributes removed.
 */
export function sanitizeHtml(dirty: string): string {
  if (!dirty) return "";
  return DOMPurify.sanitize(dirty, purifyConfig) as string;
}

// ---------------------------------------------------------------------------
// CSS sanitisation
// ---------------------------------------------------------------------------

/**
 * Properties permitted in tenant-supplied custom CSS.
 *
 * This is an allow-list, not a deny-list: anything missing here is dropped.
 * The set intentionally excludes properties that can trigger fetches or legacy
 * script execution (`behavior`, `-moz-binding`, `src`, `border-image` with
 * `url()` payloads is still filtered by the value check below).
 */
const ALLOWED_PROPERTIES: ReadonlySet<string> = new Set([
  "color",
  "background",
  "background-color",
  "background-image",
  "background-repeat",
  "background-position",
  "background-size",
  "background-attachment",
  "background-clip",
  "background-origin",
  "border",
  "border-top",
  "border-right",
  "border-bottom",
  "border-left",
  "border-color",
  "border-style",
  "border-width",
  "border-radius",
  // Chrome expands the `border-radius` shorthand into these longhands when
  // enumerating a declaration block; without them sanitisation silently drops
  // every tenant `border-radius` rule.
  "border-top-left-radius",
  "border-top-right-radius",
  "border-bottom-left-radius",
  "border-bottom-right-radius",
  "border-collapse",
  "border-spacing",
  "outline",
  "outline-color",
  "outline-style",
  "outline-width",
  "outline-offset",
  "margin",
  "margin-top",
  "margin-right",
  "margin-bottom",
  "margin-left",
  "padding",
  "padding-top",
  "padding-right",
  "padding-bottom",
  "padding-left",
  "display",
  "position",
  "top",
  "right",
  "bottom",
  "left",
  "inset",
  "z-index",
  "float",
  "clear",
  "overflow",
  "overflow-x",
  "overflow-y",
  "visibility",
  "opacity",
  "width",
  "height",
  "min-width",
  "min-height",
  "max-width",
  "max-height",
  "box-sizing",
  "box-shadow",
  "flex",
  "flex-direction",
  "flex-wrap",
  "flex-grow",
  "flex-shrink",
  "flex-basis",
  "align-items",
  "align-content",
  "align-self",
  "justify-content",
  "justify-items",
  "justify-self",
  "gap",
  "row-gap",
  "column-gap",
  "order",
  "grid",
  "grid-template",
  "grid-template-areas",
  "grid-template-columns",
  "grid-template-rows",
  "grid-area",
  "grid-column",
  "grid-row",
  "grid-auto-flow",
  "grid-auto-columns",
  "grid-auto-rows",
  "font",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "font-variant",
  "font-stretch",
  "line-height",
  "letter-spacing",
  "word-spacing",
  "text-align",
  "text-decoration",
  "text-decoration-color",
  "text-decoration-line",
  "text-decoration-style",
  "text-transform",
  "text-indent",
  "text-overflow",
  "text-shadow",
  "text-underline-offset",
  "white-space",
  "word-break",
  "overflow-wrap",
  "vertical-align",
  "list-style",
  "list-style-type",
  "list-style-position",
  "cursor",
  "transform",
  "transform-origin",
  "transition",
  "transition-property",
  "transition-duration",
  "transition-timing-function",
  "transition-delay",
  "animation",
  "animation-name",
  "animation-duration",
  "animation-timing-function",
  "animation-delay",
  "animation-iteration-count",
  "animation-direction",
  "animation-fill-mode",
  "animation-play-state",
  "content",
  "filter",
  "backdrop-filter",
  "mix-blend-mode",
  "object-fit",
  "object-position",
  "aspect-ratio",
  "user-select",
  "pointer-events",
  "resize",
  "appearance",
  "fill",
  "stroke",
  "stroke-width",
]);

/** Declarations whose *value* can fetch a URL or run legacy script. */
const DANGEROUS_VALUE = /url\s*\(|expression\s*\(|javascript\s*:|vbscript\s*:|-moz-binding/i;

/** Selector/media text must not be able to break out of the enclosing block. */
const DANGEROUS_SYNTAX = /[<>{}@\\]|url\s*\(|expression\s*\(/i;

function isAllowedProperty(property: string): boolean {
  const name = property.toLowerCase();
  // Custom properties are inert on their own; their value is still validated.
  if (name.startsWith("--")) return true;
  return ALLOWED_PROPERTIES.has(name);
}

function isSafeSelector(selector: string): boolean {
  return selector.trim() !== "" && !DANGEROUS_SYNTAX.test(selector);
}

/**
 * Serialise a style declaration block from the allow-listed properties only.
 * `CSSStyleDeclaration` normalises shorthand/longhand and colour values, so the
 * output is canonical CSS rather than the raw tenant string.
 */
function rebuildDeclarations(style: CSSStyleDeclaration): string {
  const parts: string[] = [];
  const seen = new Set<string>();
  for (let i = 0; i < style.length; i += 1) {
    const property = style.item(i);
    if (!isAllowedProperty(property)) continue;
    const value = style.getPropertyValue(property);
    if (!value || DANGEROUS_VALUE.test(value)) continue;
    const important = style.getPropertyPriority(property) ? " !important" : "";
    const declaration = `${property}: ${value}${important}`;
    if (seen.has(declaration)) continue;
    seen.add(declaration);
    parts.push(`${declaration};`);
  }
  return parts.length > 0 ? ` ${parts.join(" ")} ` : "";
}

const STYLE_RULE = 1;
const MEDIA_RULE = 4;
const KEYFRAMES_RULE = 7;
const KEYFRAME_RULE = 8;

/**
 * Prefix tenant selectors with the login-shell scope so application CSS cannot
 * reach outside the shell (antd portals, document body, other pages).
 * Root selectors (`:root`/`html`/`body`) map onto the scope element itself so
 * custom properties still inherit into the shell; selectors already carrying
 * the scope are left untouched to avoid double-prefixing.
 */
function scopeSelectorText(selectorText: string, scope: string): string {
  return selectorText
    .split(",")
    .map((part) => {
      const selector = part.trim();
      if (!selector) return "";
      if (
        selector === scope ||
        selector.startsWith(`${scope} `) ||
        selector.startsWith(`${scope}.`) ||
        selector.startsWith(`${scope}:`) ||
        selector.startsWith(`${scope}#`) ||
        selector.startsWith(`${scope}>`) ||
        selector.startsWith(`${scope}~`) ||
        selector.startsWith(`${scope}+`)
      ) {
        return selector;
      }
      const rootMatch = /^(:root|html|body)(?=$|[\s.:#>+~[])/.exec(selector);
      if (rootMatch) {
        const rest = selector.slice(rootMatch[1].length);
        return rest ? `${scope}${rest}` : scope;
      }
      return `${scope} ${selector}`;
    })
    .filter(Boolean)
    .join(", ");
}

function rebuildRule(rule: CSSRule, scope?: string): string {
  switch (rule.type) {
    case STYLE_RULE: {
      const styleRule = rule as CSSStyleRule;
      if (!isSafeSelector(styleRule.selectorText)) return "";
      const declarations = rebuildDeclarations(styleRule.style);
      if (!declarations.trim()) return "";
      const selectorText = scope
        ? scopeSelectorText(styleRule.selectorText, scope)
        : styleRule.selectorText.trim();
      if (!selectorText) return "";
      return `${selectorText} {${declarations}}`;
    }
    case MEDIA_RULE: {
      const mediaRule = rule as CSSMediaRule;
      const media = mediaRule.media.mediaText;
      if (DANGEROUS_SYNTAX.test(media)) return "";
      const inner = rebuildRuleList(mediaRule.cssRules, scope);
      if (!inner) return "";
      return `@media ${media.trim()} { ${inner} }`;
    }
    case KEYFRAMES_RULE: {
      const keyframes = rule as CSSKeyframesRule;
      const name = keyframes.name;
      if (!/^[A-Za-z_-][A-Za-z0-9_-]*$/.test(name)) return "";
      const frames: string[] = [];
      for (let i = 0; i < keyframes.cssRules.length; i += 1) {
        const frame = keyframes.cssRules[i] as CSSKeyframeRule;
        if (frame.type !== KEYFRAME_RULE) continue;
        const keyText = frame.keyText;
        if (!/^(from|to|\d+(\.\d+)?%)(\s*,\s*(from|to|\d+(\.\d+)?%))*$/i.test(keyText.trim())) {
          continue;
        }
        const declarations = rebuildDeclarations(frame.style);
        if (!declarations.trim()) continue;
        frames.push(`${keyText.trim()} {${declarations}}`);
      }
      if (frames.length === 0) return "";
      return `@keyframes ${name} { ${frames.join(" ")} }`;
    }
    default:
      // @import, @charset, @font-face, @supports, @page, … are not needed by
      // tenant branding CSS and are dropped wholesale.
      return "";
  }
}

function rebuildRuleList(rules: CSSRuleList, scope?: string): string {
  const out: string[] = [];
  for (let i = 0; i < rules.length; i += 1) {
    const rebuilt = rebuildRule(rules[i], scope);
    if (rebuilt) out.push(rebuilt);
  }
  return out.join("\n");
}

type CSSStyleSheetCtor = typeof CSSStyleSheet;

let cachedSheetCtor: CSSStyleSheetCtor | null | undefined;

/** Feature-detect constructable stylesheets, re-checking if the global changes. */
function getSheetCtor(): CSSStyleSheetCtor | null {
  const candidate =
    typeof globalThis !== "undefined"
      ? (globalThis as { CSSStyleSheet?: CSSStyleSheetCtor }).CSSStyleSheet
      : undefined;
  if (cachedSheetCtor !== undefined && cachedSheetCtor === candidate) {
    return cachedSheetCtor;
  }
  cachedSheetCtor = null;
  if (!candidate) return null;
  try {
    const probe = new candidate();
    if (typeof probe.replaceSync !== "function") return null;
    probe.replaceSync("a{color:red}");
    if (probe.cssRules.length !== 1) return null;
    cachedSheetCtor = candidate;
  } catch {
    return null;
  }
  return cachedSheetCtor;
}

/**
 * Regex fallback for environments without constructable stylesheets.
 * Still strips HTML breakout characters and CSS escapes so it cannot be used to
 * close the enclosing `<style>` element or rebuild a blocked token.
 */
function sanitizeCssFallback(css: string): string {
  return css
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/[<>]/g, "")
    .replace(/\\/g, "")
    .replace(/@import[^;]*;?/gi, "")
    .replace(/@charset[^;]*;?/gi, "")
    .replace(/expression\s*\(/gi, "")
    .replace(/url\s*\(/gi, "")
    .replace(/javascript\s*:/gi, "")
    .replace(/vbscript\s*:/gi, "")
    .replace(/behavior\s*:/gi, "")
    .replace(/-moz-binding\s*:/gi, "");
}

/**
 * Sanitize CSS content to prevent CSS-based attacks.
 *
 * Custom CSS is injected through `<style dangerouslySetInnerHTML>`, so the
 * browser parses it as HTML first: `</style><script>…</script>` would escape the
 * style element entirely. CSS escapes (`u\72l(`) can also rebuild blocked
 * tokens after a naive regex pass.
 *
 * Strategy (SEC-A7): parse into an isolated constructable stylesheet, then
 * **rebuild** the output from an allow-list of rule types, selectors and
 * properties — never pass tenant text through verbatim. Unknown at-rules,
 * unsafe selectors and any declaration outside the allow-list are dropped.
 * Without CSSOM support we fall back to the hardened regex pass.
 *
 * `scopeSelector` (optional) additionally prefixes every rebuilt selector with
 * a platform-supplied scope (e.g. `.login-content`) so tenant CSS is confined
 * to that subtree. Scoping needs the CSSOM rebuild; the regex fallback cannot
 * safely rewrite selectors and returns unscoped (but still sanitised) CSS.
 */
export function sanitizeCss(dirty: string, scopeSelector?: string): string {
  if (!dirty) return "";
  // Strip HTML breakout characters and CSS escapes *before* parsing. Escapes
  // are the only way to rebuild a blocked token (`u\72l(`), and `<`/`>` are the
  // only way to leave the <style> element.
  const preprocessed = dirty.replace(/[<>]/g, "").replace(/\\/g, "");

  const SheetCtor = getSheetCtor();
  if (!SheetCtor) return sanitizeCssFallback(preprocessed);

  try {
    const sheet = new SheetCtor();
    sheet.replaceSync(preprocessed);
    return rebuildRuleList(sheet.cssRules, scopeSelector);
  } catch {
    // Unparseable input (browser rejected the stylesheet) → nothing safe to emit.
    return sanitizeCssFallback(preprocessed);
  }
}
