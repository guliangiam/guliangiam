import { beforeEach, describe, expect, it, vi } from "vitest";
import { useEffect } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Form } from "antd";
import type { FormInstance } from "antd";
import { MemoryRouter } from "react-router-dom";
import { createTestWrapper } from "@/test-utils/render";
import type { IamLdapItem } from "@/shared/api/iam/ldap";
import { DirectoryTab } from "./DirectoryTab";

const { mockGetIamLdaps } = vi.hoisted(() => ({
  mockGetIamLdaps: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/api/iam/ldap", () => ({
  getIamLdaps: (...args: unknown[]) => mockGetIamLdaps(...args),
}));

const initialValues = {
  ldapAttributes: ["uid", "mail"],
  kerberosRealm: "EXAMPLE.COM",
  kerberosKdcHost: "kdc.example.com",
  kerberosServiceName: "HTTP",
};

let formInstance: FormInstance | undefined;

function Harness({ onFinish }: { onFinish: (values: Record<string, unknown>) => void }) {
  const [form] = Form.useForm();
  useEffect(() => {
    formInstance = form;
  }, [form]);
  return (
    <Form form={form} initialValues={initialValues} onFinish={onFinish}>
      <DirectoryTab organizationId="org-1" />
    </Form>
  );
}

function renderDirectory(
  onFinish: (values: Record<string, unknown>) => void = vi.fn<
    (values: Record<string, unknown>) => void
  >(),
) {
  return render(
    <MemoryRouter>
      <Harness onFinish={onFinish} />
    </MemoryRouter>,
    { wrapper: createTestWrapper() },
  );
}

function ldapServer(overrides: Partial<IamLdapItem> = {}): IamLdapItem {
  return {
    id: "ldap-1",
    organizationId: "org-1",
    name: "corp",
    displayName: "Corp LDAP",
    host: "ldap.example.com",
    port: 389,
    isEnabled: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  formInstance = undefined;
  mockGetIamLdaps.mockResolvedValue([]);
});

describe("DirectoryTab", () => {
  it("echoes LDAP attributes and Kerberos fields", () => {
    renderDirectory();

    expect(screen.getByText("uid")).toBeInTheDocument();
    expect(screen.getByText("mail")).toBeInTheDocument();
    expect((screen.getByLabelText("organization.kerberosRealm") as HTMLInputElement).value).toBe(
      "EXAMPLE.COM",
    );
    expect((screen.getByLabelText("organization.kerberosKdcHost") as HTMLInputElement).value).toBe(
      "kdc.example.com",
    );
    expect(
      (screen.getByLabelText("organization.kerberosServiceName") as HTMLInputElement).value,
    ).toBe("HTTP");
  });

  it("submits LDAP and Kerberos values under the expected field names", async () => {
    const onFinish = vi.fn<(values: Record<string, unknown>) => void>();
    renderDirectory(onFinish);

    await userEvent.type(
      screen.getByLabelText("organization.kerberosRealm") as HTMLInputElement,
      ".LOCAL",
    );
    act(() => {
      formInstance!.submit();
    });

    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish.mock.calls[0]![0]).toMatchObject({
      ldapAttributes: ["uid", "mail"],
      kerberosRealm: "EXAMPLE.COM.LOCAL",
      kerberosKdcHost: "kdc.example.com",
      kerberosServiceName: "HTTP",
    });
  });

  it("loads the organization's LDAP servers and renders them", async () => {
    mockGetIamLdaps.mockResolvedValue([ldapServer()]);
    renderDirectory();

    expect(await screen.findByText("Corp LDAP")).toBeInTheDocument();
    expect(screen.getByText("ldap.example.com:389")).toBeInTheDocument();
    expect(mockGetIamLdaps).toHaveBeenCalledWith({ owner: "org-1" });
  });

  it("renders the no-data state when the LDAP list is empty", async () => {
    renderDirectory();

    expect(await screen.findByText("noData")).toBeInTheDocument();
    expect(mockGetIamLdaps).toHaveBeenCalledWith({ owner: "org-1" });
  });

  it("shows a loading state instead of the no-data placeholder while the LDAP list is still loading", async () => {
    let resolveList: (value: IamLdapItem[]) => void = () => {};
    mockGetIamLdaps.mockReturnValue(
      new Promise<IamLdapItem[]>((resolve) => {
        resolveList = resolve;
      }),
    );
    const { container } = renderDirectory();

    expect(container.querySelector(".ant-skeleton")).toBeInTheDocument();
    expect(screen.queryByText("noData")).not.toBeInTheDocument();
    expect(mockGetIamLdaps).toHaveBeenCalledWith({ owner: "org-1" });

    await act(async () => {
      resolveList([]);
    });
    await waitFor(() => expect(screen.getByText("noData")).toBeInTheDocument());
    expect(container.querySelector(".ant-skeleton")).not.toBeInTheDocument();
  });
});
