import { afterEach, describe, expect, it, vi } from "vitest";
import { handleMutationError } from "./useMutationError";
import { HttpError } from "@/shared/api/http";

const messageError = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("antd", () => ({
  message: {
    error: (...args: unknown[]) => messageError(...args),
  },
}));

afterEach(() => {
  messageError.mockReset();
});

describe("handleMutationError", () => {
  it("stays silent on 401 (the redirect is handled in http.ts)", () => {
    handleMutationError(new HttpError(401, undefined, "unauthorized"));
    expect(messageError).not.toHaveBeenCalled();
  });

  it("names the offending field for a rejected pageSize", () => {
    const error = new HttpError(422, undefined, "pageSize must be an integer between 1 and 100");
    error.code = "unprocessable_entity";
    error.details = { field: "pageSize", value: "200", max: 100 };

    handleMutationError(error);

    expect(messageError).toHaveBeenCalledTimes(1);
    expect(messageError.mock.calls[0][0]).toContain("pageSize");
    expect(messageError.mock.calls[0][0]).toContain("max 100");
  });

  it("lists the allowed values for an unknown sort field", () => {
    const error = new HttpError(422, undefined, "sortField must be one of: createdAt, username");
    error.code = "unprocessable_entity";
    error.details = { field: "sortField", value: "created_at", allowed: ["createdAt", "username"] };

    handleMutationError(error);

    const shown = String(messageError.mock.calls[0][0]);
    expect(shown).toContain("created_at");
    expect(shown).toContain("createdAt, username");
  });

  it("falls back to the server message when 422 carries no details", () => {
    const error = new HttpError(422, undefined, "Request rejected by a business rule");
    error.code = "unprocessable_entity";

    handleMutationError(error);

    expect(messageError).toHaveBeenCalledWith("Request rejected by a business rule");
  });

  it("uses the fallback message for an unclassified error", () => {
    handleMutationError(new Error(""), "Save failed");
    expect(messageError).toHaveBeenCalledWith("Save failed");
  });
});
