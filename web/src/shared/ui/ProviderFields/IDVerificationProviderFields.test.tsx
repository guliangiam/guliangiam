import { render, screen } from "@testing-library/react";
import { Form, Input } from "antd";
import { describe, expect, it, vi } from "vitest";

import { IDVerificationProviderFields } from "./IDVerificationProviderFields";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

// providerType 通过隐藏 Form.Item 注册，组件用 useWatch 从 Form 上下文读取。
function Wrapper({ providerType }: { providerType: string }) {
  return (
    <Form initialValues={{ providerType }}>
      <Form.Item name="providerType" hidden>
        <Input />
      </Form.Item>
      <IDVerificationProviderFields />
    </Form>
  );
}

describe("IDVerificationProviderFields per-kind credential mapping", () => {
  it("renders the AccessKey pair (admin/password) for AliyunIdv, never appId", async () => {
    render(<Wrapper providerType="AliyunIdv" />);
    expect(await screen.findByText("accessKeyId")).toBeTruthy();
    expect(screen.getByText("accessKeySecret")).toBeTruthy();
    expect(screen.getByText("idvSceneId")).toBeTruthy();
    // 回归：误导性的 appId 控件曾导致 AccessKeyId 填错列 → 实名 503。
    expect(screen.queryByText("appId")).toBeNull();
    expect(screen.queryByText("clientId")).toBeNull();
  });

  it("renders the AccessKey pair for TencentIdv", async () => {
    render(<Wrapper providerType="TencentIdv" />);
    expect(await screen.findByText("accessKeyId")).toBeTruthy();
    expect(screen.getByText("accessKeySecret")).toBeTruthy();
    expect(screen.getByText("idvSceneId")).toBeTruthy();
  });

  it("renders clientId/clientSecret for Jumio and Veriff", async () => {
    const { unmount } = render(<Wrapper providerType="Jumio" />);
    expect(await screen.findByText("clientId")).toBeTruthy();
    expect(screen.getByText("clientSecret")).toBeTruthy();
    expect(screen.getByText("clientSecret2")).toBeTruthy();
    expect(screen.queryByText("accessKeyId")).toBeNull();
    unmount();

    render(<Wrapper providerType="Veriff" />);
    expect(await screen.findByText("clientId")).toBeTruthy();
    expect(screen.getByText("clientSecret")).toBeTruthy();
  });

  it("renders a single token + scene for Onfido and Persona", async () => {
    const { unmount } = render(<Wrapper providerType="Onfido" />);
    expect(await screen.findByText("clientId")).toBeTruthy();
    expect(screen.getByText("idvSceneId")).toBeTruthy();
    expect(screen.queryByText("clientSecret")).toBeNull();
    unmount();

    render(<Wrapper providerType="Persona" />);
    expect(await screen.findByText("clientId")).toBeTruthy();
    expect(screen.getByText("idvSceneId")).toBeTruthy();
  });
});
