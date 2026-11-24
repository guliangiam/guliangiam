import { App } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProviderInlineCreateModal } from "./ProviderInlineCreateModal";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/features/session/session-store", () => ({
  useSessionStore: (selector: (state: { organization: { id: string } }) => unknown) =>
    selector({ organization: { id: "org-1" } }),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationOptions: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

vi.mock("@/shared/api/iam/provider-extended", () => ({
  createProvider: vi.fn<() => Promise<{ id: string }>>(async () => ({ id: "p-1" })),
}));

vi.mock("@/shared/lib/server-error", () => ({
  localizedError: (error: unknown) => String(error),
}));

function renderModal() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <App>
        <MemoryRouter>
          <ProviderInlineCreateModal open onClose={vi.fn<() => void>()} />
        </MemoryRouter>
      </App>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ProviderInlineCreateModal category options", () => {
  it("excludes Captcha from the inline create entry (unified API would 422)", async () => {
    renderModal();
    // Modal 内容渲染在 body portal 中；等待类别 Select 出现后展开下拉。
    await waitFor(() => {
      expect(screen.getByText("provider.newProvider")).toBeTruthy();
    });
    // 第 0 个是组织选择器，第 1 个是类别选择器。
    const selects = document.querySelectorAll(".ant-select");
    expect(selects.length).toBeGreaterThan(1);
    fireEvent.mouseDown(selects[1].querySelector(".ant-select-selector") ?? selects[1]);

    await waitFor(() => {
      expect(document.querySelector(".ant-select-item-option")).toBeTruthy();
    });
    const dropdownText = document.querySelector(".ant-select-dropdown")?.textContent ?? "";
    expect(dropdownText).not.toContain("Captcha");
    expect(dropdownText).toContain("OAuth");
    expect(dropdownText).toContain("SMS");
  });
});
