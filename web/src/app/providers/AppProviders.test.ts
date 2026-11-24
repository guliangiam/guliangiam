import { describe, expect, it, vi } from "vitest";
import type { MutationOptions } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { HttpError } from "@/shared/api/http";
import { createAppQueryClient, mutationErrorText } from "./AppProviders";

const translate = ((key: string) => key) as TFunction;

function failingMutation(
  error: unknown,
  extra: Partial<MutationOptions<unknown, Error, void>> = {},
): MutationOptions<unknown, Error, void> {
  return {
    mutationFn: async () => {
      throw error;
    },
    ...extra,
  };
}

async function execute(
  client: ReturnType<typeof createAppQueryClient>,
  error: unknown,
  extra = {},
) {
  const mutation = client.getMutationCache().build(client, failingMutation(error, extra));
  await mutation.execute(undefined).catch(() => undefined);
}

describe("global mutation error text", () => {
  it("maps status codes to localized keys and never renders backend prose", () => {
    expect(mutationErrorText(new HttpError(503, undefined, "backend prose"), translate)).toBe(
      "common:serverError.unavailable",
    );
    expect(mutationErrorText(new HttpError(409, undefined, "backend prose"), translate)).toBe(
      "common:serverError.conflict",
    );
    expect(mutationErrorText(new Error("backend prose"), translate)).toBe("common:operationFailed");
  });
});

describe("global mutation error policy", () => {
  it("notifies for a mutation with no page-level error handler", async () => {
    const notify = vi.fn<(error: unknown) => void>();
    const client = createAppQueryClient({ notifyMutationError: notify });

    await execute(client, new HttpError(503, undefined, "backend prose"));

    expect(notify).toHaveBeenCalledTimes(1);
  });

  it("stays silent when the mutation declares its own onError", async () => {
    const notify = vi.fn<(error: unknown) => void>();
    const pageError = vi.fn<(error: unknown) => void>();
    const client = createAppQueryClient({ notifyMutationError: notify });

    await execute(client, new Error("boom"), { onError: pageError });

    expect(pageError).toHaveBeenCalledTimes(1);
    expect(notify).not.toHaveBeenCalled();
  });

  it("stays silent when an inline consumer opts out through mutation meta", async () => {
    const notify = vi.fn<(error: unknown) => void>();
    const client = createAppQueryClient({ notifyMutationError: notify });

    await execute(client, new Error("boom"), { meta: { suppressGlobalError: true } });

    expect(notify).not.toHaveBeenCalled();
  });

  it("stays silent on 401 because http.ts owns the redirect", async () => {
    const notify = vi.fn<(error: unknown) => void>();
    const client = createAppQueryClient({ notifyMutationError: notify });

    await execute(client, new HttpError(401, undefined, "unauthorized"));

    expect(notify).not.toHaveBeenCalled();
  });
});
