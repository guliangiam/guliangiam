import { iamList as _iamList, iamGet as _iamGet, iamUpdate as _iamUpdate } from "./client";
import { I18N_LANGUAGES, I18N_TRANSLATE, I18N_TRANSLATIONS } from "@/shared/api/paths";

// Re-export iamList, iamGet, iamUpdate from client for convenience
export const iamList = _iamList;
export const iamGet = _iamGet;
export const iamUpdate = _iamUpdate;

// Get list of supported languages
export async function iamListLanguages<T>(): Promise<T[]> {
  return iamList<T>(I18N_LANGUAGES);
}

// Get translations for a language
export async function iamGetTranslations<T>(language: string): Promise<T> {
  return iamGet<T>(I18N_TRANSLATIONS(language));
}

// Update translations for a language
export async function iamUpdateTranslations(
  language: string,
  translations: Record<string, string>,
): Promise<void> {
  await iamUpdate(I18N_TRANSLATIONS(language), translations);
}

// Translate a single key
export async function iamTranslate(
  language: string,
  key: string,
): Promise<{ language: string; key: string; translation: string }> {
  return iamGet(
    `${I18N_TRANSLATE}?language=${encodeURIComponent(language)}&key=${encodeURIComponent(key)}`,
  );
}
