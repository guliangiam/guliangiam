import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  deleteIamSyncer,
  getIamSyncer,
  parseSyncerColumnMapping,
  patchIamSyncer,
  serializeSyncerColumnMapping,
  testIamSyncerConnection,
  runIamSyncer,
  type IamPatchSyncerPayload,
} from "@/shared/api/iam";
import { getCertOptions } from "@/shared/api/iam/cert";
import { queryKeys } from "@/shared/api/query-keys";
import type { SyncerColumnMapping } from "@/shared/ui/SyncerColumnMappingEditor";
import { localizedError } from "@/shared/lib/server-error";

export type SyncerFormValues = {
  displayName: string;
  type: string;
  databaseType: string;
  sslMode: string;
  host: string;
  port: number;
  username: string;
  password: string;
  database: string;
  tableName: string;
  syncInterval: number;
  isReadOnly: boolean;
  affinity: string;
  columnMapping: SyncerColumnMapping[];
  sshType: string;
  sshHost: string;
  sshPort: number;
  sshUsername: string;
  sshPassword: string;
  sshCertId: string;
  affiliationTable: string;
  avatarBaseUrl: string;
  isEnabled: boolean;
};

export const SYNCER_TYPE_VALUES = [
  "Database",
  "Keycloak",
  "WeCom",
  "Azure AD",
  "Active Directory",
  "Google Workspace",
  "DingTalk",
  "Lark",
  "Okta",
  "SCIM",
  "AWS IAM",
] as const;
export const DATABASE_TYPE_VALUES = ["postgres", "mysql", "mssql", "oracle", "sqlite3"] as const;
export const SSL_MODE_VALUES = ["disable", "require", "verify-ca", "verify-full"] as const;
export const SSH_TYPE_VALUES = ["None", "Password", "Cert"] as const;
export const CLOUD_SYNCER_TYPES = new Set([
  "Keycloak",
  "WeCom",
  "Azure AD",
  "Active Directory",
  "Google Workspace",
  "DingTalk",
  "Lark",
  "Okta",
  "SCIM",
  "AWS IAM",
]);

export function useSyncerDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SyncerFormValues>();

  const DATABASE_TYPE_OPTIONS = DATABASE_TYPE_VALUES.map((v) => ({
    label: tr(`syncer.databaseTypeOption.${v}`),
    value: v,
  }));
  const SSL_MODE_OPTIONS = SSL_MODE_VALUES.map((v) => ({
    label: tr(`syncer.sslModeOption.${v}`),
    value: v,
  }));
  const SSH_TYPE_OPTIONS = SSH_TYPE_VALUES.map((v) => ({
    label: tr(`syncer.sshTypeOption.${v}`),
    value: v,
  }));

  const syncerType = Form.useWatch("type", form) ?? "Database";
  const isCloud = CLOUD_SYNCER_TYPES.has(syncerType);
  const isDatabase = !isCloud;
  const isPostgres = Form.useWatch("databaseType", form) === "postgres";

  const detailQuery = useQuery({
    queryKey: queryKeys.syncers.detail(id!),
    queryFn: () => getIamSyncer(id!),
    enabled: !!id,
  });

  const certOptionsQuery = useQuery({
    queryKey: ["cert-options"],
    queryFn: () => getCertOptions(),
    staleTime: 60_000,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data;
      form.setFieldsValue({
        displayName: detail.displayName,
        type: detail.type || "Database",
        databaseType: detail.databaseType || "postgres",
        sslMode: detail.sslMode || "disable",
        host: detail.host,
        port: detail.port,
        // `SyncerDetail` deliberately omits credentials; leave them blank so
        // saving without re-entry cannot overwrite the stored values.
        username: "",
        password: "",
        database: detail.database || "",
        tableName: detail.tableName || "",
        syncInterval: detail.syncInterval || 0,
        isReadOnly: detail.isReadOnly,
        affinity: detail.affinity || "",
        columnMapping: parseSyncerColumnMapping(detail.columnMapping),
        sshType: detail.sshType || "None",
        sshHost: detail.sshHost || "",
        sshPort: detail.sshPort || 22,
        sshUsername: detail.sshUsername || "",
        sshPassword: "",
        sshCertId: detail.sshCertId || "",
        affiliationTable: detail.affiliationTable || "",
        avatarBaseUrl: detail.avatarBaseUrl || "",
        isEnabled: detail.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: SyncerFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      const payload: IamPatchSyncerPayload = {
        displayName: values.displayName,
        host: values.host,
        port: values.port,
        database: values.database,
        tableName: values.tableName,
        isEnabled: values.isEnabled,
        syncInterval: values.syncInterval,
        databaseType: values.databaseType,
        sslMode: values.sslMode,
        sshType: values.sshType,
        sshHost: values.sshHost,
        sshPort: values.sshPort,
        sshUsername: values.sshUsername,
        sshCertId: values.sshCertId,
        columnMapping: serializeSyncerColumnMapping(values.columnMapping),
        isReadOnly: values.isReadOnly,
        affinity: values.affinity,
        affiliationTable: values.affiliationTable,
        avatarBaseUrl: values.avatarBaseUrl,
        revision: detailQuery.data.revision,
      };
      // Credentials are not returned by the detail read, so only send the
      // ones the operator actually typed; empty strings must not wipe them.
      if (values.username.trim()) payload.username = values.username;
      if (values.password.trim()) payload.password = values.password;
      if (values.sshPassword.trim()) payload.sshPassword = values.sshPassword;
      return patchIamSyncer(detailQuery.data.id, payload);
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.syncers.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamSyncer(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.syncers.all });
      navigate("/syncers");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const testConnectionMutation = useMutation({
    mutationFn: () => testIamSyncerConnection(id!),
    onSuccess: () => message.success(tr("syncer.connectionTestSuccess")),
    onError: (error) =>
      message.error(
        tr("syncer.connectionTestFailed", {
          error: localizedError(error, tr, "syncer.unknownError"),
        }),
      ),
  });

  const runSyncMutation = useMutation({
    mutationFn: () => runIamSyncer(id!),
    onSuccess: () => message.success(tr("syncer.syncStarted")),
    onError: (error) =>
      message.error(
        tr("syncer.syncFailed", {
          error: localizedError(error, tr, "syncer.unknownError"),
        }),
      ),
  });

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    syncerType,
    isCloud,
    isDatabase,
    isPostgres,
    detailQuery,
    certOptionsQuery,
    DATABASE_TYPE_OPTIONS,
    SSL_MODE_OPTIONS,
    SSH_TYPE_OPTIONS,
    // Mutations
    updateMutation,
    deleteMutation,
    testConnectionMutation,
    runSyncMutation,
  };
}
