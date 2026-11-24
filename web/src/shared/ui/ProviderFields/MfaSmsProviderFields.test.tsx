import { render, screen } from "@testing-library/react";
import { Form, Input } from "antd";
import { describe, expect, it, vi } from "vitest";

import MfaProviderFields from "./MfaProviderFields";
import { SmsProviderFields } from "./SmsProviderFields";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

// MFA 组件在 Form 内通过 useWatch 读取 providerType（form 省略时走 context）。
function MfaWrapper({ providerType }: { providerType?: string }) {
  return (
    <Form initialValues={{ providerType }}>
      <Form.Item name="providerType" hidden>
        <Input />
      </Form.Item>
      <MfaProviderFields />
    </Form>
  );
}

describe("MfaProviderFields per-type layouts", () => {
  it("binds RADIUS to providerUrl + clientSecret, not the unconsumed host/port columns", async () => {
    render(<MfaWrapper providerType="RADIUS" />);
    expect(await screen.findByText("provider.mfa.radiusServer")).toBeTruthy();
    expect(screen.getByText("provider.mfa.sharedSecret")).toBeTruthy();
    expect(screen.getByPlaceholderText("radius://radius.example.com:1812")).toBeTruthy();
    // resolveRadiusProvider 不读 host/port/password 原生列，这些控件不得渲染。
    expect(screen.queryByText("host")).toBeNull();
    expect(screen.queryByText("port")).toBeNull();
  });

  it("accepts lowercase radius type values", async () => {
    render(<MfaWrapper providerType="radius" />);
    expect(await screen.findByText("provider.mfa.radiusServer")).toBeTruthy();
  });

  it("shows the built-in TOTP note without credential inputs", async () => {
    render(<MfaWrapper providerType="TOTP" />);
    expect(await screen.findByText("provider.mfa.totpHint")).toBeTruthy();
    expect(screen.queryByText("provider.mfa.radiusServer")).toBeNull();
  });

  it("points SMS and Email MFA at the organization's messaging providers", async () => {
    const { unmount } = render(<MfaWrapper providerType="SMS" />);
    expect(await screen.findByText("provider.mfa.smsHint")).toBeTruthy();
    unmount();
    render(<MfaWrapper providerType="Email" />);
    expect(await screen.findByText("provider.mfa.emailHint")).toBeTruthy();
  });

  it("points Push MFA at Notification-category providers", async () => {
    render(<MfaWrapper providerType="Push" />);
    expect(await screen.findByText("provider.mfa.pushHint")).toBeTruthy();
  });

  it("falls back to the TOTP note when no type is selected yet", async () => {
    render(<MfaWrapper />);
    expect(await screen.findByText("provider.mfa.totpHint")).toBeTruthy();
  });
});

function renderSms(subType: string) {
  return render(<SmsProviderFields value={{}} onChange={() => {}} subType={subType} />);
}

describe("SmsProviderFields vendor layouts (provider_builder fallback chains)", () => {
  it("renders Twilio Account SID / Auth Token / From labels", () => {
    renderSms("TwilioSMS");
    expect(screen.getByText("accountSid")).toBeTruthy();
    expect(screen.getByText("authToken")).toBeTruthy();
    expect(screen.getByText("fromNumber")).toBeTruthy();
    expect(screen.queryByText("templateCode")).toBeNull();
  });

  it("renders Twilio Verify Service SID instead of a sender", () => {
    renderSms("Twilio Verify");
    expect(screen.getByText("serviceSid")).toBeTruthy();
    expect(screen.queryByText("signName")).toBeNull();
    expect(screen.queryByText("fromNumber")).toBeNull();
  });

  it("renders Tencent SecretId / SDK App ID / templateId / region", () => {
    renderSms("TencentSMS");
    expect(screen.getByText("secretId")).toBeTruthy();
    expect(screen.getByText("sdkAppId")).toBeTruthy();
    expect(screen.getByText("templateId")).toBeTruthy();
    expect(screen.getByText("region")).toBeTruthy();
  });

  it("renders Huawei signature once (signature and senderId share the sender column)", () => {
    renderSms("HuaweiSMS");
    expect(screen.getByText("appKey")).toBeTruthy();
    expect(screen.getByText("appSecret")).toBeTruthy();
    expect(screen.getByText("signature")).toBeTruthy();
    // 两个控件不得写同一列：senderId 不单独渲染。
    expect(screen.queryByText("senderId")).toBeNull();
  });

  it("renders Baidu invokeId and SmsBao goodsId through the customHeader slot", () => {
    const { unmount } = renderSms("BaiduSMS");
    expect(screen.getByText("invokeId")).toBeTruthy();
    expect(screen.getByText("signId")).toBeTruthy();
    unmount();
    renderSms("SmsBao SMS");
    expect(screen.getByText("goodsId")).toBeTruthy();
    expect(screen.getByText("username")).toBeTruthy();
  });

  it("renders Azure ACS connection string and Infobip base URL", () => {
    const { unmount } = renderSms("AzureACSSMS");
    expect(screen.getByText("connectionString")).toBeTruthy();
    unmount();
    renderSms("Infobip");
    expect(screen.getByText("baseUrl")).toBeTruthy();
    expect(screen.getByText("senderName")).toBeTruthy();
  });

  it("renders no credential inputs for MockSMS", () => {
    const { container } = renderSms("MockSMS");
    expect(container.querySelectorAll("input").length).toBe(0);
  });

  it("keeps the generic default layout for unknown vendors", () => {
    renderSms("SomeNewVendor");
    expect(screen.getByText("accessKeyId")).toBeTruthy();
    expect(screen.getByText("accessKeySecret")).toBeTruthy();
    expect(screen.getByText("signName")).toBeTruthy();
    expect(screen.getByText("templateCode")).toBeTruthy();
  });
});
