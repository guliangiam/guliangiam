import { act, renderHook } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { isValidElement } from "react";
import { MemoryRouter } from "react-router-dom";
import type { UploadProps } from "antd";
import { useSessionStore } from "@/features/session/session-store";
import { queryKeys } from "@/shared/api/query-keys";
import { createTestQueryClient, createTestWrapper } from "@/test-utils/render";
import { useApplicationsPageController } from "./useApplicationsPageController";

const {
  mockImportApplicationConfig,
  mockGetApplicationsPaginated,
  mockDeleteApplication,
  mockCreateApplication,
  mockGetApplication,
  mockExportApplicationConfig,
  mockDownloadApplicationExport,
  mockMessage,
} = vi.hoisted(() => ({
  mockImportApplicationConfig: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplicationsPaginated: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDeleteApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockCreateApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetApplication: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockExportApplicationConfig: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDownloadApplicationExport: vi.fn<(...args: unknown[]) => void>(),
  mockMessage: {
    success: vi.fn<(...args: unknown[]) => void>(),
    error: vi.fn<(...args: unknown[]) => void>(),
    warning: vi.fn<(...args: unknown[]) => void>(),
    info: vi.fn<(...args: unknown[]) => void>(),
  },
}));

vi.mock("@/shared/api/iam/application", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/iam/application")>();
  return {
    ...actual,
    getApplicationsPaginated: (...args: unknown[]) => mockGetApplicationsPaginated(...args),
    deleteApplication: (...args: unknown[]) => mockDeleteApplication(...args),
    createApplication: (...args: unknown[]) => mockCreateApplication(...args),
    getApplication: (...args: unknown[]) => mockGetApplication(...args),
    importApplicationConfig: (...args: unknown[]) => mockImportApplicationConfig(...args),
    exportApplicationConfig: (...args: unknown[]) => mockExportApplicationConfig(...args),
  };
});

vi.mock("./application-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./application-helpers")>();
  return {
    ...actual,
    downloadApplicationExport: (...args: unknown[]) => mockDownloadApplicationExport(...args),
  };
});

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map<string, string>() }),
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

type UploadElement = ReactElement<UploadProps>;

function findImportUpload(node: ReactNode): UploadElement | null {
  if (!isValidElement(node)) return null;
  const props = node.props as UploadProps & { children?: ReactNode };
  if (props.accept === ".json" && typeof props.beforeUpload === "function") {
    return node as UploadElement;
  }
  if (!props.children) return null;
  const children = Array.isArray(props.children) ? props.children : [props.children];
  for (const child of children) {
    const found = findImportUpload(child);
    if (found) return found;
  }
  return null;
}

function renderController() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const Wrapper = createTestWrapper({ queryClient });
  const view = renderHook(() => useApplicationsPageController(), {
    wrapper: ({ children }) => (
      <MemoryRouter>
        <Wrapper>{children}</Wrapper>
      </MemoryRouter>
    ),
  });
  return { ...view, invalidateSpy };
}

function jsonFile(payload: unknown, name = "application.json") {
  return new File([JSON.stringify(payload)], name, { type: "application/json" });
}

describe("useApplicationsPageController import", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useSessionStore.setState({
      account: {
        owner: "built-in",
        name: "admin",
        isAdmin: true,
        organization: { id: "org-1", name: "built-in", displayName: "Built-in" },
      },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
    });
  });

  it("imports the JSON into the session organization and invalidates the application list", async () => {
    mockImportApplicationConfig.mockResolvedValue({ id: "app-1", status: "active" });
    const { result, invalidateSpy } = renderController();

    const upload = findImportUpload(result.current.toolbar);
    expect(upload).not.toBeNull();
    const payload = { export: { tokenEndpointAuthMethod: "none", slug: "exported-app" } };
    const file = jsonFile(payload);

    await act(async () => {
      await upload!.props.beforeUpload!(file, [file]);
    });

    expect(mockImportApplicationConfig).toHaveBeenCalledTimes(1);
    expect(mockImportApplicationConfig.mock.calls[0][0]).toBe("org-1");
    expect(mockImportApplicationConfig.mock.calls[0][1]).toEqual(payload);
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: queryKeys.applications.paginated(),
    });
    expect(mockMessage.success).toHaveBeenCalledWith("application.imported");
    expect(mockMessage.error).not.toHaveBeenCalled();
  });

  it("warns about secret rotation after a client_secret_basic import", async () => {
    mockImportApplicationConfig.mockResolvedValue({ id: "app-2", status: "active" });
    const { result } = renderController();

    const upload = findImportUpload(result.current.toolbar);
    const payload = { export: { tokenEndpointAuthMethod: "client_secret_basic" } };
    const file = jsonFile(payload);

    await act(async () => {
      await upload!.props.beforeUpload!(file, [file]);
    });

    expect(mockImportApplicationConfig).toHaveBeenCalledWith("org-1", payload);
    expect(mockMessage.warning).toHaveBeenCalledWith("application.importSecretNotIncluded");
    expect(mockMessage.success).not.toHaveBeenCalled();
  });

  it("blocks imports whose client material is not part of the export", async () => {
    const { result, invalidateSpy } = renderController();

    const upload = findImportUpload(result.current.toolbar);
    const file = jsonFile({ export: { tokenEndpointAuthMethod: "private_key_jwt" } });

    await act(async () => {
      await upload!.props.beforeUpload!(file, [file]);
    });

    expect(mockImportApplicationConfig).not.toHaveBeenCalled();
    expect(invalidateSpy).not.toHaveBeenCalled();
    expect(mockMessage.error).toHaveBeenCalledWith("application.importRequiresClientMaterial");
    expect(mockMessage.success).not.toHaveBeenCalled();
  });

  it("reports the backend error and does not pretend the import succeeded", async () => {
    mockImportApplicationConfig.mockRejectedValue(new Error("import failed at backend"));
    const { result, invalidateSpy } = renderController();

    const upload = findImportUpload(result.current.toolbar);
    const file = jsonFile({ export: { tokenEndpointAuthMethod: "none" } });

    await expect(upload!.props.beforeUpload!(file, [file])).rejects.toThrow(
      "import failed at backend",
    );

    expect(mockMessage.error).toHaveBeenCalledWith("import failed at backend");
    expect(mockMessage.success).not.toHaveBeenCalled();
    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});

type OrgSelectElement = ReactElement<{
  value?: string;
  allowClear?: boolean;
  onChange: (value: string) => void;
}>;

function findOrgSelect(node: ReactNode): OrgSelectElement | null {
  if (!isValidElement(node)) return null;
  const props = node.props as OrgSelectElement["props"] & { children?: ReactNode };
  if (props.allowClear === true && typeof props.onChange === "function") {
    return node as OrgSelectElement;
  }
  if (!props.children) return null;
  const children = Array.isArray(props.children) ? props.children : [props.children];
  for (const child of children) {
    const found = findOrgSelect(child);
    if (found) return found;
  }
  return null;
}

describe("useApplicationsPageController organization filter", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockGetApplicationsPaginated.mockResolvedValue({ items: [], total: 0 });
    useSessionStore.setState({
      account: {
        owner: "built-in",
        name: "admin",
        isAdmin: true,
        organization: { id: "org-1", name: "built-in", displayName: "Built-in" },
      },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
    });
  });

  it("resets the page to 1 when the organization filter changes", async () => {
    const { result } = renderController();

    act(() => {
      result.current.crud.serverPagination.onChange(3, result.current.crud.pagination.pageSize);
    });
    expect(result.current.crud.serverPagination.page).toBe(3);

    const orgSelect = findOrgSelect(result.current.toolbar);
    expect(orgSelect).not.toBeNull();
    await act(async () => {
      orgSelect!.props.onChange("org-2");
    });

    expect(result.current.crud.serverPagination.page).toBe(1);
  });

  it("falls back to the new session organization after an org switch elsewhere", async () => {
    const { result } = renderController();

    const orgSelect = findOrgSelect(result.current.toolbar);
    await act(async () => {
      orgSelect!.props.onChange("org-2");
    });
    expect(findOrgSelect(result.current.toolbar)!.props.value).toBe("org-2");

    act(() => {
      useSessionStore.setState({
        organization: {
          id: "org-3",
          name: "acme",
          displayName: "Acme",
          navItems: [],
          userNavItems: [],
        },
      });
    });

    // 旧选择绑定在 org-1 上下文，会话切到 org-3 后必须失效回退，而不是继续查 org-2。
    expect(findOrgSelect(result.current.toolbar)!.props.value).toBe("org-3");
    expect(result.current.crud.serverPagination.page).toBe(1);
  });
});

type ColumnLike = {
  key?: string;
  render?: (value: unknown, record: unknown, index: number) => ReactNode;
};

type RowActionsElement = ReactElement<{
  onExport: (record: unknown) => void;
  onDelete: (record: unknown) => Promise<unknown>;
}>;

function renderRowActions(result: { columns: unknown; crud: unknown }): RowActionsElement {
  const columns = result.columns as (list: unknown) => ColumnLike[];
  const actions = columns(result.crud).find((column) => column.key === "actions");
  expect(actions?.render).toBeTypeOf("function");
  const record = { id: "app-1", name: "portal", displayName: "Portal", organizationId: "org-1" };
  return actions!.render!(null, record, 0) as RowActionsElement;
}

describe("useApplicationsPageController row actions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    mockGetApplicationsPaginated.mockResolvedValue({ items: [], total: 0 });
    useSessionStore.setState({
      account: {
        owner: "built-in",
        name: "admin",
        isAdmin: true,
        organization: { id: "org-1", name: "built-in", displayName: "Built-in" },
      },
      organization: {
        id: "org-1",
        name: "built-in",
        displayName: "Built-in",
        navItems: [],
        userNavItems: [],
      },
    });
  });

  it("exports with the design-contract filename {name}.json", async () => {
    const blob = new Blob(["{}"], { type: "application/json" });
    mockExportApplicationConfig.mockResolvedValue(blob);
    const { result } = renderController();

    const row = renderRowActions(result.current);
    await act(async () => {
      row.props.onExport({
        id: "app-1",
        name: "portal",
        displayName: "Portal",
        organizationId: "org-1",
      });
    });

    expect(mockExportApplicationConfig).toHaveBeenCalledWith("app-1", "org-1");
    expect(mockDownloadApplicationExport).toHaveBeenCalledWith(blob, "portal");
    expect(mockMessage.error).not.toHaveBeenCalled();
  });

  it("reports export failures instead of downloading a broken file", async () => {
    mockExportApplicationConfig.mockRejectedValue(new Error("export denied"));
    const { result } = renderController();

    const row = renderRowActions(result.current);
    await act(async () => {
      row.props.onExport({ id: "app-1", name: "portal", organizationId: "org-1" });
    });

    expect(mockDownloadApplicationExport).not.toHaveBeenCalled();
    expect(mockMessage.error).toHaveBeenCalledTimes(1);
  });

  it("routes row deletion through the list delete mutation with the trusted org key", async () => {
    mockDeleteApplication.mockResolvedValue(undefined);
    const { result, invalidateSpy } = renderController();

    const row = renderRowActions(result.current);
    await act(async () => {
      await row.props.onDelete({ id: "app-1", name: "portal", organizationId: "org-1" });
    });

    expect(mockDeleteApplication).toHaveBeenCalledWith("app-1", "org-1");
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.applications.paginated() });
  });
});
