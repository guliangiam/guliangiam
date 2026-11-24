import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import { ENTRIES, ENTRY_BY_ID, ENTRY_TRANSCRIPT } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Entries (Audit Log / OTLP Ingestion)
// ─────────────────────────────────────────────
export type IamEntryItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  application: string;
  clientIp: string;
  createdAt: string;
};
export type IamEntryDetail = IamEntryItem & {
  provider: string;
  userAgent: string;
  updatedAt: string;
  revision: number;
};
export type IamCreateEntryPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  provider?: string;
  application?: string;
  type?: string;
  clientIp?: string;
  userAgent?: string;
  message?: string;
};
export type IamPatchEntryPayload = {
  displayName?: string;
  revision: number;
};

export const getIamEntries = (params?: IamListParams) => iamList<IamEntryItem>(ENTRIES, params);
export const getIamEntriesPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamEntryItem>(ENTRIES, params);
export const getIamEntry = (id: string) => iamGet<IamEntryDetail>(`${ENTRY_BY_ID(id)}`);
export const createIamEntry = (p: IamCreateEntryPayload) => iamCreate(ENTRIES, p);
export const patchIamEntry = (id: string, p: IamPatchEntryPayload) =>
  iamPatch(`${ENTRY_BY_ID(id)}`, p);
export const deleteIamEntry = (id: string) => iamDelete(`${ENTRY_BY_ID(id)}`);

// ─────────────────────────────────────────────
// Entry transcript (read-only, append-order)
// ─────────────────────────────────────────────
/**
 * One fact in an entry's message sequence. `body` is the record as served: every
 * sensitive value inside it has already been replaced on the server (42.3.13 条 2),
 * so the console must not attempt a second redaction pass.
 */
export type IamEntryTranscriptMessage = {
  index: number;
  kind: string;
  scope?: string;
  name?: string;
  timestamp: string;
  body?: Record<string, unknown>;
  bodyBytes: number;
  /** True when the record exceeded the per-message bound and only this marker is served. */
  bodyOmitted?: boolean;
};

export type IamEntryTranscriptPage = {
  messages: IamEntryTranscriptMessage[];
  /** Opaque. Empty on the last page; the console must never construct one. */
  nextCursor?: string;
  /** Field paths the server replaced, so "absent" is distinguishable from "redacted". */
  redactedFields?: string[];
};

export type IamEntryTranscriptParams = {
  cursor?: string;
  limit?: number;
};

function buildTranscriptQueryString(params?: IamEntryTranscriptParams): string {
  const parts: string[] = [];
  if (params?.cursor) parts.push(`cursor=${encodeURIComponent(params.cursor)}`);
  if (params?.limit !== undefined) parts.push(`limit=${params.limit}`);
  return parts.length > 0 ? `?${parts.join("&")}` : "";
}

/**
 * Read one page of an entry's message sequence, in the order the messages were
 * produced. Read-only by contract: there is no write or delete counterpart, and
 * no endpoint that returns the un-redacted record (42.3.13 条 4、6).
 */
export const getIamEntryTranscript = (id: string, params?: IamEntryTranscriptParams) => {
  const qs = buildTranscriptQueryString(params);
  return iamGet<IamEntryTranscriptPage>(`${ENTRY_TRANSCRIPT(id)}${qs}`);
};
