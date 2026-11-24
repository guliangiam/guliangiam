import { render, screen } from "@testing-library/react";
import { Form, Input } from "antd";
import { describe, expect, it, vi } from "vitest";

import { CaptchaProviderFields } from "./CaptchaProviderFields";
import { assembleCaptchaConfig, CAPTCHA_CONFIG_FIELDS } from "./captcha-constants";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

// 组件在 Form 内通过 useWatch 读取 providerType；真实表单里 providerType 是
// 已注册字段，测试用隐藏 Form.Item 注册同名字段后断言 config 字段集合。
function Wrapper({ providerType }: { providerType: string }) {
  return (
    <Form initialValues={{ providerType }}>
      <Form.Item name="providerType" hidden>
        <Input />
      </Form.Item>
      <CaptchaProviderFields />
    </Form>
  );
}

describe("CaptchaProviderFields conditional config fields", () => {
  it("renders the CAM credential pair for TencentCaptcha", async () => {
    render(<Wrapper providerType="TencentCaptcha" />);
    expect(await screen.findByText("captcha.camSecretId")).toBeTruthy();
    expect(screen.getByText("captcha.camSecretKey")).toBeTruthy();
    expect(screen.queryByText("captcha.sceneId")).toBeNull();
  });

  it("renders secretId for NetEaseYidun and appName for BaiduCaptcha", async () => {
    const { unmount } = render(<Wrapper providerType="NetEaseYidun" />);
    expect(await screen.findByText("captcha.secretId")).toBeTruthy();
    unmount();

    render(<Wrapper providerType="BaiduCaptcha" />);
    expect(await screen.findByText("captcha.appName")).toBeTruthy();
  });

  it("renders project/score/action for reCAPTCHAEnterprise and sceneId for AliyunCaptcha", async () => {
    const { unmount } = render(<Wrapper providerType="reCAPTCHAEnterprise" />);
    expect(await screen.findByText("captcha.projectId")).toBeTruthy();
    expect(screen.getByText("captcha.scoreThreshold")).toBeTruthy();
    expect(screen.getByText("captcha.action")).toBeTruthy();
    unmount();

    render(<Wrapper providerType="AliyunCaptcha" />);
    expect(await screen.findByText("captcha.sceneId")).toBeTruthy();
  });

  it("renders no config fields for GeeTest and Default", async () => {
    render(<Wrapper providerType="GeeTest" />);
    expect(await screen.findByText("captcha.captchaId")).toBeTruthy();
    expect(
      screen.queryByText(/captcha\.(sceneId|appName|projectId|secretId|camSecret)/),
    ).toBeNull();
  });
});

describe("assembleCaptchaConfig", () => {
  it("keeps only non-empty values for the selected kind", () => {
    expect(
      assembleCaptchaConfig("tencent_captcha", {
        captchaCamSecretId: " AKID123 ",
        captchaCamSecretKey: "",
        captchaSceneId: "irrelevant",
      }),
    ).toEqual({ camSecretId: "AKID123" });
  });

  it("keeps finite numbers and drops NaN/undefined", () => {
    expect(
      assembleCaptchaConfig("recaptcha_v3", {
        captchaScoreThreshold: 0.7,
        captchaAction: undefined,
      }),
    ).toEqual({ scoreThreshold: 0.7 });
  });

  it("returns an empty object for kinds without config fields", () => {
    expect(CAPTCHA_CONFIG_FIELDS.geetest).toBeUndefined();
    expect(assembleCaptchaConfig("geetest", { captchaSceneId: "x" })).toEqual({});
  });
});
