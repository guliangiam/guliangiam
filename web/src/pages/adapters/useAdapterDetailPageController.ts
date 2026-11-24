import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamAdapter,
  getIamAdapter,
  patchIamAdapter,
  getOrganizationOptions,
} from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import type { IamAdapterDetail } from "@/shared/api/iam/adapter";
import { localizedError } from "@/shared/lib/server-error";

export const DATABASE_TYPE_OPTIONS = [
  { label: "PostgreSQL", value: "postgresql" },
  { label: "MySQL", value: "mysql" },
  { label: "SQL Server", value: "mssql" },
  { label: "Oracle", value: "oracle" },
  { label: "SQLite3", value: "sqlite3" },
];

export type AdapterDetail = IamAdapterDetail & {
  owner?: string;
  tableName?: string;
  useSameDb?: boolean;
  databaseType?: string;
  host?: string;
  port?: number;
  dbUser?: string;
  dbPassword?: string;
  databaseName?: string;
};

export type AdapterFormValues = {
  owner: string;
  displayName: string;
  tableName: string;
  useSameDb: boolean;
  databaseType: string;
  host: string;
  port: number;
  dbUser: string;
  dbPassword: string;
  databaseName: string;
  policy: string;
  isEnabled: boolean;
};

export function useAdapterDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<AdapterFormValues>();

  const detailQuery = useQuery({
    queryKey: queryKeys.adapters.detail(id!),
    queryFn: () => getIamAdapter(id!),
    enabled: !!id,
  });

  const useSameDb = Form.useWatch("useSameDb", form) ?? true;

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data as AdapterDetail;
      form.setFieldsValue({
        owner: detail.owner ?? "",
        displayName: detail.displayName,
        tableName: detail.tableName ?? "",
        useSameDb: detail.useSameDb ?? true,
        databaseType: detail.databaseType ?? "postgresql",
        host: detail.host ?? "",
        port: detail.port ?? 5432,
        dbUser: detail.dbUser ?? "",
        dbPassword: detail.dbPassword ?? "",
        databaseName: detail.databaseName ?? "",
        policy: detail.policy,
        isEnabled: detail.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: AdapterFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamAdapter(detailQuery.data.id, {
        owner: values.owner,
        displayName: values.displayName,
        tableName: values.tableName,
        useSameDb: values.useSameDb,
        databaseType: values.databaseType,
        host: values.host,
        port: values.port,
        dbUser: values.dbUser,
        // 读 DTO 只回 dbPasswordSet/Masked，表单回填恒为空；PATCH 空串=清空。
        // 未输入新密码时必须省略该键，否则每次保存都会清掉已存连接密码。
        dbPassword: values.dbPassword ? values.dbPassword : undefined,
        databaseName: values.databaseName,
        policy: values.policy,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["adapters"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamAdapter(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["adapters"] });
      navigate("/adapters");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    useSameDb,
    detailQuery,
    updateMutation,
    deleteMutation,
    DATABASE_TYPE_OPTIONS,
    getOrganizationOptions,
  };
}
