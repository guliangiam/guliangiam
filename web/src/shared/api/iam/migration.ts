import {
  iamList,
  iamGet,
  iamCreate,
  iamDelete,
  iamPostRaw,
  type IamListParams,
  iamListWithPagination,
} from "./client";
import type { SyncerColumnMapping } from "@/shared/ui/SyncerColumnMappingEditor";
import { SYNCERS, SYNCER_BY_ID, SYNCER_RUN, SYNCER_TEST_CONNECTION } from "@/shared/api/paths";

// ─────────────────────────────────────────────
// Migration (syncer-based data import)
// ─────────────────────────────────────────────
export type IamMigrationSyncerItem = {
  id: string;
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  isEnabled: boolean;
  lastSyncAt: string | null;
  createdAt: string;
  updatedAt: string;
  revision: number;
};

export type IamMigrationSyncerDetail = IamMigrationSyncerItem & {
  port: number;
  user: string;
  password: string;
  databaseType: string;
  databaseName: string;
  tableName: string;
  syncInterval: number;
  isReadOnly: boolean;
  affinity: string;
  errorText: string;
  columnMapping: SyncerColumnMapping[];
  sslMode: string;
  sshType: string;
  sshHost: string;
  sshPort: number;
  sshUser: string;
  sshPassword: string;
  cert: string;
  affiliationTable: string;
  avatarBaseUrl: string;
};

export type IamMigrationResult = {
  userSync?: {
    totalUsers: number;
    addedUsers: number;
    updatedUsers: number;
    failedUsers: number;
    errors: string[];
  };
  groupSync?: {
    totalGroups: number;
    addedGroups: number;
    updatedGroups: number;
    failedGroups: number;
    errors: string[];
  };
};

export const getIamMigrationSyncers = (params?: IamListParams) =>
  iamList<IamMigrationSyncerItem>(SYNCERS, params);

export const getIamMigrationSyncersPaginated = (params?: IamListParams) =>
  iamListWithPagination<IamMigrationSyncerItem>(SYNCERS, params);

export const getIamMigrationSyncer = (id: string) =>
  iamGet<IamMigrationSyncerDetail>(`${SYNCER_BY_ID(id)}`);

export const createIamMigrationSyncer = (p: {
  organizationId: string;
  name: string;
  displayName: string;
  type: string;
  host: string;
  port?: number;
  user?: string;
  password?: string;
  databaseName?: string;
  tableName?: string;
  syncInterval?: number;
  isEnabled?: boolean;
  columnMapping?: SyncerColumnMapping[];
}) => iamCreate(SYNCERS, p);

export const runIamMigrationSyncer = (id: string) => iamPostRaw(`${SYNCER_RUN(id)}`);

export const testIamMigrationSyncerConnection = (id: string) =>
  iamPostRaw(`${SYNCER_TEST_CONNECTION(id)}`);

export const deleteIamMigrationSyncer = (id: string) => iamDelete(`${SYNCER_BY_ID(id)}`);
