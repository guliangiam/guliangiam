import { beforeEach, describe, expect, it } from "vitest";
import {
  SECTION_NAV_STORAGE_KEY,
  resolveSectionNavScheme,
  useSectionNavStore,
} from "./section-nav-store";
import { DEFAULT_SECTION_NAV_SCHEME, isSectionNavScheme } from "./types";

describe("section-nav-store", () => {
  beforeEach(() => {
    localStorage.clear();
    useSectionNavStore.setState({ scheme: null, updatedAt: null });
  });

  it("treats the system default as the fallback scheme", () => {
    expect(DEFAULT_SECTION_NAV_SCHEME).toBe("rail-right");
    expect(resolveSectionNavScheme({ scheme: null })).toBe(DEFAULT_SECTION_NAV_SCHEME);
    expect(resolveSectionNavScheme({ scheme: "rail-left" })).toBe("rail-left");
  });

  it("accepts only the closed set of schemes", () => {
    expect(isSectionNavScheme("rail-right")).toBe(true);
    expect(isSectionNavScheme("rail-left")).toBe(true);
    expect(isSectionNavScheme("rail-top")).toBe(true);
    expect(isSectionNavScheme("right")).toBe(false);
    expect(isSectionNavScheme(null)).toBe(false);
    expect(isSectionNavScheme(undefined)).toBe(false);
  });

  it("persists a chosen scheme with an updatedAt stamp", () => {
    useSectionNavStore.getState().setScheme("rail-top");

    const state = useSectionNavStore.getState();
    expect(state.scheme).toBe("rail-top");
    expect(typeof state.updatedAt).toBe("string");

    const raw = localStorage.getItem(SECTION_NAV_STORAGE_KEY);
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw ?? "{}").state.scheme).toBe("rail-top");
  });

  it("clears the local preference when reset", () => {
    useSectionNavStore.getState().setScheme("rail-left");
    useSectionNavStore.getState().resetScheme();

    const state = useSectionNavStore.getState();
    expect(state.scheme).toBeNull();
    expect(state.updatedAt).toBeNull();
    expect(resolveSectionNavScheme(state)).toBe(DEFAULT_SECTION_NAV_SCHEME);
  });

  it("falls back to the system default when the persisted value is invalid", async () => {
    localStorage.setItem(
      SECTION_NAV_STORAGE_KEY,
      JSON.stringify({ state: { scheme: "not-a-scheme", updatedAt: 42 }, version: 0 }),
    );

    await useSectionNavStore.persist.rehydrate();

    const state = useSectionNavStore.getState();
    expect(state.scheme).toBeNull();
    expect(state.updatedAt).toBeNull();
    expect(resolveSectionNavScheme(state)).toBe(DEFAULT_SECTION_NAV_SCHEME);
  });
});
