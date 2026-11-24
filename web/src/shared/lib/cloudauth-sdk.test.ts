import { afterEach, describe, expect, it, vi } from "vitest";

type SdkModule = typeof import("./cloudauth-sdk");

// 模块级单例（sdkPromise）必须每个用例重置，否则上一个用例的失败态会污染下一个。
async function freshImport(): Promise<SdkModule> {
  vi.resetModules();
  return import("./cloudauth-sdk");
}

function injectedScripts(): HTMLScriptElement[] {
  return Array.from(document.querySelectorAll<HTMLScriptElement>("script[data-cloudauth-sdk]"));
}

function fireLoad(script: HTMLScriptElement) {
  script.dispatchEvent(new Event("load"));
}

function fireError(script: HTMLScriptElement) {
  script.dispatchEvent(new Event("error"));
}

afterEach(() => {
  delete (window as Window & { getMetaInfo?: unknown }).getMetaInfo;
  injectedScripts().forEach((s) => s.remove());
  vi.useRealTimers();
});

describe("cloudauth-sdk", () => {
  it("resolves metaInfo once the SDK script loads and the global appears", async () => {
    const sdk = await freshImport();
    const collecting = sdk.collectMetaInfo();
    const script = injectedScripts()[0];
    expect(script).toBeDefined();
    window.getMetaInfo = () => '{"deviceType":"web"}';
    fireLoad(script);
    await expect(collecting).resolves.toBe('{"deviceType":"web"}');
  });

  it("rejects with reason=load when the script fails to load", async () => {
    const sdk = await freshImport();
    const collecting = sdk.collectMetaInfo();
    fireError(injectedScripts()[0]);
    await expect(collecting).rejects.toMatchObject({ name: "CloudAuthMetaError", reason: "load" });
  });

  it("rejects with reason=timeout when the script never settles", async () => {
    vi.useFakeTimers();
    const sdk = await freshImport();
    const collecting = sdk.collectMetaInfo();
    const failure = collecting.catch((error: unknown) => error);
    vi.advanceTimersByTime(9000);
    await expect(failure).resolves.toMatchObject({ reason: "timeout" });
  });

  it("rejects with reason=missing when the script loads but exposes no getMetaInfo", async () => {
    const sdk = await freshImport();
    const collecting = sdk.collectMetaInfo();
    fireLoad(injectedScripts()[0]);
    await expect(collecting).rejects.toMatchObject({ reason: "missing" });
  });

  it("rejects with reason=invalid for blank or non-JSON metaInfo", async () => {
    const sdk = await freshImport();
    window.getMetaInfo = () => "";
    await expect(sdk.collectMetaInfo()).rejects.toMatchObject({ reason: "invalid" });

    const sdk2 = await freshImport();
    window.getMetaInfo = () => "not-json";
    await expect(sdk2.collectMetaInfo()).rejects.toMatchObject({ reason: "invalid" });
  });

  it("skips injection entirely when getMetaInfo is already present", async () => {
    const sdk = await freshImport();
    window.getMetaInfo = () => '{"ok":true}';
    await expect(sdk.collectMetaInfo()).resolves.toBe('{"ok":true}');
    expect(injectedScripts()).toHaveLength(0);
  });

  it("reuses one injected script for concurrent calls", async () => {
    const sdk = await freshImport();
    const first = sdk.collectMetaInfo();
    const second = sdk.collectMetaInfo();
    expect(injectedScripts()).toHaveLength(1);
    window.getMetaInfo = () => '{"ok":true}';
    fireLoad(injectedScripts()[0]);
    await Promise.all([first, second]);
  });

  it("builds returnUrl as origin+path without query or hash", async () => {
    const sdk = await freshImport();
    expect(sdk.buildIdvReturnUrl("/users/u-1")).toBe(`${window.location.origin}/users/u-1`);
  });
});
