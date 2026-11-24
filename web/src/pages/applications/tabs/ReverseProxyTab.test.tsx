import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestWrapper } from "@/test-utils/render";
import { ReverseProxyTab } from "./ReverseProxyTab";
import type { ApplicationConfigFormValues } from "./types";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: unknown) =>
      typeof options === "object" && options !== null && "row" in options
        ? `${key} (${String((options as { row: unknown }).row)})`
        : key,
  }),
}));

function renderTab(initialValues: Partial<ApplicationConfigFormValues> = {}) {
  const onFinish = vi.fn<(values: ApplicationConfigFormValues) => void>();
  const handle: { form: FormInstance<ApplicationConfigFormValues> | null } = { form: null };

  function Harness() {
    const [form] = Form.useForm<ApplicationConfigFormValues>();
    handle.form = form;

    return (
      <Form form={form} initialValues={initialValues} onFinish={onFinish}>
        <ReverseProxyTab form={form} />
      </Form>
    );
  }

  render(<Harness />, { wrapper: createTestWrapper() });

  return { form: handle.form as FormInstance<ApplicationConfigFormValues>, onFinish };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ReverseProxyTab echo", () => {
  it("prefills the protocol, upstream host, certificate and domain rows", () => {
    const { form } = renderTab({
      sslMode: "HTTPS Only",
      upstreamHost: "internal-server:8080",
      sslCert: "cert-1",
      otherDomains: ["https://a.example", "https://b.example"],
    });

    expect(screen.getByText("application.sslModeOption.httpsOnly")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("internal-server:8080")).toHaveValue("internal-server:8080");
    expect(screen.getByPlaceholderText("application.placeholder.sslCert")).toHaveValue("cert-1");

    const domainInputs = screen.getAllByPlaceholderText("https://example.com/callback");
    expect(domainInputs).toHaveLength(2);
    expect(domainInputs[0]).toHaveValue("https://a.example");
    expect(domainInputs[1]).toHaveValue("https://b.example");
    expect(form.getFieldValue("otherDomains")).toEqual(["https://a.example", "https://b.example"]);
  });

  it("renders a stored legacy sslMode through the legacy option instead of the raw value", () => {
    renderTab({ sslMode: "Auto" });

    expect(screen.getByText("application.sslModeOption.legacy")).toBeInTheDocument();
    expect(screen.queryByText("Auto")).not.toBeInTheDocument();
  });

  it("offers exactly the four target protocol values", async () => {
    const user = userEvent.setup();
    renderTab({ sslMode: "" });

    await user.click(screen.getByRole("combobox"));
    await screen.findByText("application.sslModeOption.httpsOnly");

    for (const key of ["none", "http", "httpsAndHttp", "httpsOnly"]) {
      expect(screen.getAllByText(`application.sslModeOption.${key}`).length).toBeGreaterThan(0);
    }
    expect(screen.queryByText("application.sslModeOption.auto")).not.toBeInTheDocument();
    expect(screen.queryByText("application.sslModeOption.enable")).not.toBeInTheDocument();
    expect(screen.queryByText("application.sslModeOption.disable")).not.toBeInTheDocument();
  });
});

describe("ReverseProxyTab submit", () => {
  it("submits sslMode, upstreamHost and sslCert independently", async () => {
    const { form, onFinish } = renderTab({
      sslMode: "HTTPS Only",
      upstreamHost: "internal-server:8080",
      sslCert: "cert-1",
    });

    act(() => form.submit());

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    const values = onFinish.mock.calls[0][0];

    // These three fields are independent single-line values, so each one reaches
    // the payload exactly as entered.
    expect(values.sslMode).toBe("HTTPS Only");
    expect(values.upstreamHost).toBe("internal-server:8080");
    expect(values.sslCert).toBe("cert-1");
  });

  it("submits otherDomains as a string array after adding a row", async () => {
    const { form, onFinish } = renderTab({ otherDomains: ["https://a.example"] });
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: /Add URI/ }));
    await user.type(
      screen.getAllByPlaceholderText("https://example.com/callback")[1],
      "https://c.example",
    );

    expect(form.getFieldValue("otherDomains")).toEqual(["https://a.example", "https://c.example"]);

    act(() => form.submit());

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish.mock.calls[0][0].otherDomains).toEqual([
      "https://a.example",
      "https://c.example",
    ]);
  });
});

describe("ReverseProxyTab validation", () => {
  it("blocks an upstream host that carries a scheme", async () => {
    const { form, onFinish } = renderTab({ upstreamHost: "http://internal-server:8080" });

    act(() => form.submit());

    await waitFor(() =>
      expect(screen.getByText("application.validation.upstreamHostInvalid")).toBeInTheDocument(),
    );
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("blocks an upstream host whose port is outside 1-65535", async () => {
    const { form, onFinish } = renderTab({ upstreamHost: "internal-server:70000" });

    act(() => form.submit());

    await waitFor(() =>
      expect(screen.getByText("application.validation.upstreamHostInvalid")).toBeInTheDocument(),
    );
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("blocks an sslCert that is not a plain certificate identifier", async () => {
    const { form, onFinish } = renderTab({ sslCert: "证书-001" });

    act(() => form.submit());

    await waitFor(() =>
      expect(screen.getByText("application.validation.sslCertInvalid")).toBeInTheDocument(),
    );
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("blocks an invalid otherDomains row and names the row", async () => {
    const { form, onFinish } = renderTab({
      otherDomains: ["https://a.example", "not-a-valid-url"],
    });

    act(() => form.submit());

    // The row number is part of the message so the operator knows which entry
    // has to change.
    await waitFor(() =>
      expect(screen.getByText("application.validation.otherDomainInvalid (2)")).toBeInTheDocument(),
    );
    expect(onFinish).not.toHaveBeenCalled();
  });

  it("submits legal reverse proxy values", async () => {
    const { form, onFinish } = renderTab({
      upstreamHost: "[::1]:8443",
      sslCert: "certs/acme-2026.pem",
      otherDomains: ["https://a.example", "http://b.example:8080/cb", ""],
    });

    act(() => form.submit());

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    const values = onFinish.mock.calls[0][0];
    expect(values.upstreamHost).toBe("[::1]:8443");
    expect(values.sslCert).toBe("certs/acme-2026.pem");
    expect(values.otherDomains).toEqual(["https://a.example", "http://b.example:8080/cb", ""]);
  });
});

describe("ReverseProxyTab cross-field hints", () => {
  it("warns when HTTPS termination has no certificate but does not block the save", async () => {
    const { form, onFinish } = renderTab({
      sslMode: "HTTPS Only",
      upstreamHost: "internal-server:8080",
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(
        screen.getByText("application.validation.sslCertRequiredForHttps"),
      ).toBeInTheDocument(),
    );
    // Warning-only: the server decides whether the rule applies to this row.
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
  });

  it("warns when a certificate is set on a protocol that does not use HTTPS", async () => {
    const { form } = renderTab({
      sslMode: "HTTP",
      upstreamHost: "internal-server:8080",
      sslCert: "cert-1",
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(
        screen.getByText("application.validation.sslCertRequiresHttpsMode"),
      ).toBeInTheDocument(),
    );
  });

  it("warns when SSL is configured without an upstream host", async () => {
    const { form } = renderTab({
      sslMode: "HTTPS Only",
      sslCert: "cert-1",
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(
        screen.getByText("application.validation.upstreamHostRequiredForProxy"),
      ).toBeInTheDocument(),
    );
  });

  it("warns when the upstream host points back at the application domain", async () => {
    const { form } = renderTab({
      domain: "app.example",
      sslMode: "HTTPS Only",
      sslCert: "cert-1",
      upstreamHost: "app.example:8080",
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(screen.getByText("application.validation.upstreamHostSelfLoop")).toBeInTheDocument(),
    );
  });

  it("warns when otherDomains repeats the application domain", async () => {
    const { form } = renderTab({
      domain: "app.example",
      otherDomains: ["https://app.example"],
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(
        screen.getByText("application.validation.otherDomainEqualsDomain (1)"),
      ).toBeInTheDocument(),
    );
  });

  it("warns when two otherDomains rows resolve to the same host", async () => {
    const { form } = renderTab({
      otherDomains: ["https://a.example", "http://A.example/cb"],
    });

    act(() => form.submit());

    await waitFor(() =>
      expect(
        screen.getByText("application.validation.otherDomainDuplicate (2)"),
      ).toBeInTheDocument(),
    );
  });
});
