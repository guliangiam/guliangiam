import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { UseMutationResult } from "@tanstack/react-query";

import { AccountFaceIdSection } from "./AccountFaceIdSection";
import type { FaceIdItem, UserDetail } from "@/shared/api/iam/user-types";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

const mockGetIamFaceIdProviders = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/face-id-provider", () => ({
  getIamFaceIdProviders: (...args: unknown[]) => mockGetIamFaceIdProviders(...args),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

const baseUser = { id: "user-1", organizationId: "org-1", isVerified: false } as UserDetail;

const faceIds: FaceIdItem[] = [
  { id: "face-1", imageUrl: "", createdAt: "2024-01-01T00:00:00Z", provider: "aliyun" },
];

function makeRegisterMutation(mutate: (params: { file: File; providerId?: string }) => void) {
  return { mutate, isPending: false } as unknown as UseMutationResult<
    FaceIdItem,
    Error,
    { file: File; providerId?: string }
  >;
}

function makeRevokeMutation(mutate: () => void) {
  return { mutate, isPending: false } as unknown as UseMutationResult<void, Error, void>;
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("AccountFaceIdSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetIamFaceIdProviders.mockResolvedValue([]);
  });

  describe("real-name gate", () => {
    it("shows guidance instead of register entry when not verified", () => {
      render(
        <AccountFaceIdSection
          user={baseUser}
          faceIds={[]}
          registerFaceIdMutation={makeRegisterMutation(vi.fn<(p: { file: File }) => void>())}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
        />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByText("user.faceIdRealNameRequired")).toBeInTheDocument();
      expect(screen.queryByText("user.faceIdRegisterNew")).not.toBeInTheDocument();
      // 未实名不读取 provider 列表
      expect(mockGetIamFaceIdProviders).not.toHaveBeenCalled();
    });

    it("invokes onGoToRealName from the guidance action", () => {
      const onGoToRealName = vi.fn<() => void>();
      render(
        <AccountFaceIdSection
          user={baseUser}
          faceIds={[]}
          registerFaceIdMutation={makeRegisterMutation(vi.fn<(p: { file: File }) => void>())}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
          onGoToRealName={onGoToRealName}
        />,
        { wrapper: createWrapper() },
      );
      fireEvent.click(screen.getByRole("button", { name: "user.faceIdGoToRealName" }));
      expect(onGoToRealName).toHaveBeenCalled();
    });
  });

  describe("verified user", () => {
    const verifiedUser = { ...baseUser, isVerified: true } as UserDetail;

    it("renders existing face templates", async () => {
      render(
        <AccountFaceIdSection
          user={verifiedUser}
          faceIds={faceIds}
          registerFaceIdMutation={makeRegisterMutation(vi.fn<(p: { file: File }) => void>())}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
        />,
        { wrapper: createWrapper() },
      );
      await waitFor(() => {
        expect(screen.getByText("face-1")).toBeInTheDocument();
      });
    });

    it("keeps the register entry when the provider list is empty", async () => {
      mockGetIamFaceIdProviders.mockResolvedValue([]);
      render(
        <AccountFaceIdSection
          user={verifiedUser}
          faceIds={[]}
          registerFaceIdMutation={makeRegisterMutation(vi.fn<(p: { file: File }) => void>())}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
        />,
        { wrapper: createWrapper() },
      );
      await waitFor(() => {
        expect(screen.getAllByText("user.faceIdRegisterNew").length).toBeGreaterThan(0);
      });
    });

    it("keeps the register entry when the provider list query fails (403 tolerance)", async () => {
      mockGetIamFaceIdProviders.mockRejectedValue(new Error("forbidden"));
      render(
        <AccountFaceIdSection
          user={verifiedUser}
          faceIds={[]}
          registerFaceIdMutation={makeRegisterMutation(vi.fn<(p: { file: File }) => void>())}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
        />,
        { wrapper: createWrapper() },
      );
      await waitFor(() => {
        expect(screen.getAllByText("user.faceIdRegisterNew").length).toBeGreaterThan(0);
      });
    });

    it("registers the selected image without a provider when none is chosen", async () => {
      const mutate = vi.fn<(p: { file: File; providerId?: string }) => void>();
      render(
        <AccountFaceIdSection
          user={verifiedUser}
          faceIds={[]}
          registerFaceIdMutation={makeRegisterMutation(mutate)}
          revokeFaceIdMutation={makeRevokeMutation(vi.fn<() => void>())}
        />,
        { wrapper: createWrapper() },
      );

      fireEvent.click(screen.getAllByRole("button", { name: /user\.faceIdRegisterNew$/ })[0]);

      await waitFor(() => {
        expect(screen.getByText("user.faceIdRegisterTitle")).toBeInTheDocument();
      });
      // 未选图片时确认按钮禁用
      expect(screen.getByRole("button", { name: "user.faceIdRegister" })).toBeDisabled();

      const file = new File(["bytes"], "face.png", { type: "image/png" });
      const input = document.querySelector('input[type="file"]');
      expect(input).toBeTruthy();
      fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByRole("button", { name: "user.faceIdRegister" })).toBeEnabled();
      });
      fireEvent.click(screen.getByRole("button", { name: "user.faceIdRegister" }));

      expect(mutate).toHaveBeenCalledWith(
        { file, providerId: undefined },
        expect.objectContaining({ onSuccess: expect.any(Function) }),
      );
    });
  });
});
