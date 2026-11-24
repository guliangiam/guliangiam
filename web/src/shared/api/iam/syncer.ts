import {
  iamList,
  iamGet,
  iamCreate,
  iamPatch,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import type { SyncerColumnMapping } from "@/shared/ui/SyncerColumnMappingEditor";
import { SYNCERS, SYNCER_BY_ID, SYNCER_RUN, SYNCER_TEST_CONNECTION } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Syncers
// ─────────────────────────────────────────────
// Field names mirror the Go request structs (`CreateSyncerRequest` /
// `PatchSyncerRequest`) and `SyncerDetail`: the transport decodes with
// DisallowUnknownFields, so a renamed key is a 400, not a silently ignored
// field. Labels stay user-oriented (database user / database) and are mapped to
// these wire names by the pages.
export type IamSyncerItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  port: number;
  isEnabled: boolean;
  syncInterval: number;
  lastSyncAt?: string | null;
  createdAt: string;
  updatedAt: string;
};

/**
 * `columnMapping` is stored as a JSON *string* on the wire (`columnMapping` is
 * a text/jsonb column in Go), not as an array. Use the two helpers below to
 * cross the form <-> wire boundary.
 */
export type IamSyncerDetail = IamSyncerItem & {
  databaseType?: string;
  database?: string;
  tableName?: string;
  sslMode?: string;
  sshType?: string;
  sshHost?: string;
  sshPort?: number;
  sshUsername?: string;
  sshCertId?: string;
  fieldMapping?: string;
  columnMapping?: string;
  isReadOnly: boolean;
  affinity?: string;
  affiliationTable?: string;
  avatarBaseUrl?: string;
  errorText?: string;
  revision: number;
};

export type IamCreateSyncerPayload = {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  databaseType?: string;
  host: string;
  port?: number;
  database?: string;
  tableName?: string;
  username?: string;
  password?: string;
  sslMode?: string;
  sshType?: string;
  sshHost?: string;
  sshPort?: number;
  sshUsername?: string;
  sshPassword?: string;
  sshCertId?: string;
  isEnabled?: boolean;
  syncInterval?: number;
  fieldMapping?: string;
  columnMapping?: string;
  isReadOnly?: boolean;
  affinity?: string;
  affiliationTable?: string;
  avatarBaseUrl?: string;
  errorText?: string;
};

export type IamPatchSyncerPayload = {
  displayName?: string;
  host?: string;
  port?: number;
  database?: string;
  tableName?: string;
  username?: string;
  password?: string;
  isEnabled?: boolean;
  syncInterval?: number;
  fieldMapping?: string;
  databaseType?: string;
  sslMode?: string;
  sshType?: string;
  sshHost?: string;
  sshPort?: number;
  sshUsername?: string;
  sshPassword?: string;
  sshCertId?: string;
  columnMapping?: string;
  isReadOnly?: boolean;
  affinity?: string;
  affiliationTable?: string;
  avatarBaseUrl?: string;
  errorText?: string;
  revision: number;
};

/** Parse the wire JSON string into the column-mapping rows the editor renders. */
export function parseSyncerColumnMapping(raw: string | null | undefined): SyncerColumnMapping[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as SyncerColumnMapping[]) : [];
  } catch {
    return [];
  }
}

/** Encode editor rows as the JSON string the Go request struct expects. */
export function serializeSyncerColumnMapping(mappings: SyncerColumnMapping[] | undefined): string {
  return JSON.stringify(mappings ?? []);
}

export const getIamSyncers = (params?: IamListParams) => iamList<IamSyncerItem>(SYNCERS, params);
export const getIamSyncersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamSyncerItem>(SYNCERS, params);
export const getIamSyncer = (id: string) => iamGet<IamSyncerDetail>(`${SYNCER_BY_ID(id)}`);
export const createIamSyncer = (p: IamCreateSyncerPayload) => iamCreate(SYNCERS, p);
export const patchIamSyncer = (id: string, p: IamPatchSyncerPayload) =>
  iamPatch(`${SYNCER_BY_ID(id)}`, p);
export const deleteIamSyncer = (id: string) => iamDelete(`${SYNCER_BY_ID(id)}`);
export const runIamSyncer = (id: string) => iamPostRaw(`${SYNCER_RUN(id)}`);
export const testIamSyncerConnection = (id: string) => iamPostRaw(`${SYNCER_TEST_CONNECTION(id)}`);
