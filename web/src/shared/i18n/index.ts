import i18n, { changeLanguage } from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

// ─── Static imports: fallback language core only (zh-CN) ───
// Only `common` + `auth` for zh-CN are bundled. `resource` (the largest
// namespace) and every non-zh-CN core bundle are async chunks:
//   • `ensureInitialLanguage()` (main.tsx) loads the detected language's
//     common/auth before the first paint, so /login never flashes zh-CN.
//   • `ensureResourceNamespace()` (ResourceGate) loads `resource` when the
//     authenticated shell mounts. /login never pays for it in any language.
import commonZhCN from "./locales/zh-CN/common.json";
import authZhCN from "./locales/zh-CN/auth.json";

// ─── Lazy loaders: split by namespace ───
// /login renders before the app shell and only needs `common` + `auth`.
// `resource` (the largest namespace) is imported by ResourceGate once the
// authenticated shell mounts, so the login screen — and therefore the
// first-screen budget — never pays for it in any language.
type CoreBundle = { common: unknown; auth: unknown };

const coreLoaders: Record<string, () => Promise<CoreBundle>> = {
  en: () =>
    Promise.all([import("./locales/en/common.json"), import("./locales/en/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  "zh-TW": () =>
    Promise.all([import("./locales/zh-TW/common.json"), import("./locales/zh-TW/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  ja: () =>
    Promise.all([import("./locales/ja/common.json"), import("./locales/ja/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  ko: () =>
    Promise.all([import("./locales/ko/common.json"), import("./locales/ko/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  de: () =>
    Promise.all([import("./locales/de/common.json"), import("./locales/de/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  fr: () =>
    Promise.all([import("./locales/fr/common.json"), import("./locales/fr/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  es: () =>
    Promise.all([import("./locales/es/common.json"), import("./locales/es/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  pt: () =>
    Promise.all([import("./locales/pt/common.json"), import("./locales/pt/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  vi: () =>
    Promise.all([import("./locales/vi/common.json"), import("./locales/vi/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  pl: () =>
    Promise.all([import("./locales/pl/common.json"), import("./locales/pl/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  tr: () =>
    Promise.all([import("./locales/tr/common.json"), import("./locales/tr/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  uk: () =>
    Promise.all([import("./locales/uk/common.json"), import("./locales/uk/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
  ru: () =>
    Promise.all([import("./locales/ru/common.json"), import("./locales/ru/auth.json")]).then(
      ([common, auth]) => ({ common: common.default, auth: auth.default }),
    ),
};

type ShellBundle = { resource: unknown; help: unknown };

const shellLoaders: Record<string, () => Promise<ShellBundle>> = {
  "zh-CN": () =>
    Promise.all([
      import("./locales/zh-CN/resource.json"),
      import("./locales/zh-CN/help.json"),
    ]).then(([resource, help]) => ({ resource: resource.default, help: help.default })),
  en: () =>
    Promise.all([import("./locales/en/resource.json"), import("./locales/en/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  "zh-TW": () =>
    Promise.all([
      import("./locales/zh-TW/resource.json"),
      import("./locales/zh-TW/help.json"),
    ]).then(([resource, help]) => ({ resource: resource.default, help: help.default })),
  ja: () =>
    Promise.all([import("./locales/ja/resource.json"), import("./locales/ja/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  ko: () =>
    Promise.all([import("./locales/ko/resource.json"), import("./locales/ko/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  de: () =>
    Promise.all([import("./locales/de/resource.json"), import("./locales/de/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  fr: () =>
    Promise.all([import("./locales/fr/resource.json"), import("./locales/fr/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  es: () =>
    Promise.all([import("./locales/es/resource.json"), import("./locales/es/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  pt: () =>
    Promise.all([import("./locales/pt/resource.json"), import("./locales/pt/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  vi: () =>
    Promise.all([import("./locales/vi/resource.json"), import("./locales/vi/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  pl: () =>
    Promise.all([import("./locales/pl/resource.json"), import("./locales/pl/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  tr: () =>
    Promise.all([import("./locales/tr/resource.json"), import("./locales/tr/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  uk: () =>
    Promise.all([import("./locales/uk/resource.json"), import("./locales/uk/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
  ru: () =>
    Promise.all([import("./locales/ru/resource.json"), import("./locales/ru/help.json")]).then(
      ([resource, help]) => ({ resource: resource.default, help: help.default }),
    ),
};

/** Languages whose `common` + `auth` bundles are already registered. */
const coreLoaded = new Set<string>(["zh-CN"]);
/** Languages whose `resource` + `help` bundles are already registered. */
const resourceLoaded = new Set<string>();

async function addCore(lng: string): Promise<void> {
  if (coreLoaded.has(lng)) return;
  const loader = coreLoaders[lng];
  if (!loader) return;
  const core = await loader();
  i18n.addResourceBundle(lng, "common", core.common, true, true);
  i18n.addResourceBundle(lng, "auth", core.auth, true, true);
  coreLoaded.add(lng);
}

async function addResource(lng: string): Promise<void> {
  if (resourceLoaded.has(lng)) return;
  const loader = shellLoaders[lng];
  if (!loader) return;
  const bundle = await loader();
  i18n.addResourceBundle(lng, "resource", bundle.resource, true, true);
  i18n.addResourceBundle(lng, "help", bundle.help, true, true);
  resourceLoaded.add(lng);
}

/**
 * Load every namespace for a language (used by the language switcher).
 * Each namespace loader dedupes on its own set, so zh-CN's lazy `resource`
 * bundle is still fetched the first time it is needed.
 */
export async function loadLanguageResources(lng: string): Promise<void> {
  await Promise.all([addCore(lng), addResource(lng)]);
}

/**
 * Load the `resource` namespace for the current language.
 * Called by ResourceGate before the authenticated shell renders.
 */
export async function ensureResourceNamespace(lng: string): Promise<void> {
  await addResource(normalizeLanguage(lng));
}

// oxlint-disable-next-line import/no-named-as-default-member
i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      "zh-CN": { common: commonZhCN, auth: authZhCN },
    },
    fallbackLng: "zh-CN",
    ns: ["common", "auth", "resource", "help"],
    defaultNS: "common",
    saveMissing: true,
    missingKeyHandler: (_lngs, _ns, key) => {
      if (typeof console !== "undefined") {
        console.warn(`[i18n] missing key: "${key}"`);
      }
    },
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["localStorage", "navigator"],
      lookupLocalStorage: "preferredLanguage",
      caches: ["localStorage"],
    },
  });

/**
 * Map a browser/detector tag onto a bundled resource key.
 * zh-Hant variants → zh-TW, other zh variants → zh-CN (static fallback),
 * en-US → en, de-AT → de, …
 */
export function normalizeLanguage(lng: string): string {
  if (!lng) return "zh-CN";
  if (lng === "zh" || lng === "zh-CN") return "zh-CN";
  if (lng.startsWith("zh")) {
    return /TW|HK|Hant/i.test(lng) ? "zh-TW" : "zh-CN";
  }
  return lng.split("-")[0];
}

/**
 * Load the detected language before the first render.
 *
 * zh-CN is bundled statically; every other language (including en) is a
 * separate async chunk, so awaiting here avoids painting the zh-CN fallback
 * for a frame when the browser is English.
 */
export async function ensureInitialLanguage(): Promise<void> {
  const raw =
    i18n.language || (typeof navigator !== "undefined" ? navigator.language : "") || "zh-CN";
  const target = normalizeLanguage(raw);
  await addCore(target);
  if (target !== i18n.language) {
    await changeLanguage(target);
  }
}

export default i18n;
