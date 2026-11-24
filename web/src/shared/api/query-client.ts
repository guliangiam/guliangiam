import type { QueryClient } from "@tanstack/react-query";

/**
 * Module-level QueryClient bridge for route loaders.
 *
 * The QueryClient is created in AppProviders (React component scope), but
 * route loaders run outside React. This bridge lets loaders access the
 * singleton client to prefetch data via `ensureQueryData`.
 *
 * AppProviders calls `setQueryClient(client)` after creating the client.
 * Route loaders call `getQueryClient()` to access it.
 */
let queryClient: QueryClient | null = null;

export function setQueryClient(client: QueryClient): void {
  queryClient = client;
}

export function getQueryClient(): QueryClient | null {
  return queryClient;
}
