import { describe, expect, it } from "vitest";
import {
  CAPTCHA_PROVIDER_TYPES,
  NOTIFICATION_PROVIDER_TYPES,
  captchaProviderTypeOptions,
  notificationProviderTypeOptions,
  providerTypeLabel,
} from "./enums";

// These lists mirror the server's closed sets (手册 4.14.3). The console used to
// send display names (`Default`, `reCAPTCHA`, `Turnstile`, `DingTalk`, …) while
// the backend factories register lowercase keys, so not one value matched and
// every provider created here was unusable. The lists are asserted literally so
// a change on either side has to be deliberate.
describe("closed enum sets", () => {
  it("declares the captcha provider types the captcha factory registers", () => {
    expect(CAPTCHA_PROVIDER_TYPES).toEqual([
      "default",
      "recaptcha",
      "recaptcha_v3",
      "recaptcha_enterprise",
      "hcaptcha",
      "turnstile",
      "geetest",
      "aliyun_captcha",
      "tencent_captcha",
      "netease_yidun",
      "baidu_captcha",
      "go_captcha",
    ]);
  });

  it("declares the notification provider types the notification factory registers", () => {
    expect(NOTIFICATION_PROVIDER_TYPES).toEqual([
      "email",
      "sms",
      "dingtalk",
      "lark",
      "wecom",
      "wecom_app",
      "wechat_official",
      "slack",
      "discord",
      "rocketchat",
      "teams",
      "bark",
      "telegram",
      "pushover",
      "line",
      "matrix",
      "pushbullet",
      "viber",
      "webpush",
      "twitter",
      "reddit",
      "customhttp",
      "googlechat",
      "cucloud",
      "ntfy",
      "serverchan",
      "pagerduty",
      "opsgenie",
      "gotify",
    ]);
  });

  it("never offers a display name as a value", () => {
    // The exact spellings the console used to send. Any of them reappearing as a
    // value is the original defect, and the server answers 422 for it.
    const displayNames = ["Default", "reCAPTCHA", "hCaptcha", "Turnstile", "GeeTest", "Aliyun"];
    for (const name of displayNames) {
      expect(CAPTCHA_PROVIDER_TYPES as readonly string[]).not.toContain(name);
    }
    const notificationDisplayNames = [
      "DingTalk",
      "WeCom",
      "Lark",
      "Telegram",
      "Discord",
      "Slack",
      "Bark",
      "CustomHTTP",
    ];
    for (const name of notificationDisplayNames) {
      expect(NOTIFICATION_PROVIDER_TYPES as readonly string[]).not.toContain(name);
    }
  });

  it("offers the record-configured notifiers the factory builds per delivery", () => {
    // customhttp/googlechat/cucloud/ntfy/serverchan/pagerduty/opsgenie/gotify
    // take their endpoint and credentials from the provider record, so the
    // factory registers a builder for each delivery instead of a startup
    // instance. A record naming one can deliver.
    const recordConfigured = [
      "customhttp",
      "googlechat",
      "cucloud",
      "ntfy",
      "serverchan",
      "pagerduty",
      "opsgenie",
      "gotify",
    ];
    for (const name of recordConfigured) {
      expect(NOTIFICATION_PROVIDER_TYPES as readonly string[]).toContain(name);
    }
  });

  it("has no duplicate values", () => {
    expect(new Set(CAPTCHA_PROVIDER_TYPES).size).toBe(CAPTCHA_PROVIDER_TYPES.length);
    expect(new Set(NOTIFICATION_PROVIDER_TYPES).size).toBe(NOTIFICATION_PROVIDER_TYPES.length);
  });
});

describe("enum options", () => {
  it("produces one option per declared value, in the server's order", () => {
    const options = captchaProviderTypeOptions();
    expect(options.map((o) => o.value)).toEqual([...CAPTCHA_PROVIDER_TYPES]);
  });

  it("uses a readable caption where the raw value is not one", () => {
    const byValue = new Map(notificationProviderTypeOptions().map((o) => [o.value, o.label]));
    expect(byValue.get("rocketchat")).toBe("Rocket.Chat");
    expect(byValue.get("webpush")).toBe("Web Push");
    // A value with no caption falls back to the raw value rather than blank.
    expect(byValue.get("dingtalk")).toBe("DingTalk");
    expect(byValue.get("reddit")).toBe("reddit");
  });
});

describe("providerTypeLabel", () => {
  it("tolerates a value the server added after this build", () => {
    // The response side is open (手册 4.14.3 item 2): an unknown value must still
    // render, never blank out a cell or throw.
    expect(providerTypeLabel("brand-new-provider")).toBe("brand-new-provider");
    expect(providerTypeLabel("")).toBe("");
  });

  it("captions the values it knows", () => {
    expect(providerTypeLabel("recaptcha")).toBe("reCAPTCHA");
    expect(providerTypeLabel("recaptcha_v3")).toBe("reCAPTCHA v3");
    expect(providerTypeLabel("turnstile")).toBe("Cloudflare Turnstile");
    expect(providerTypeLabel("aliyun_captcha")).toBe("Aliyun Captcha");
  });
});
