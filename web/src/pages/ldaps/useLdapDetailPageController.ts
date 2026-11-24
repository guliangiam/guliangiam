import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import { deleteIamLdap, getIamLdap, patchIamLdap, getIamLdapPreviewUsers } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { buildLdapPatchPayload, type LdapPatchFormValues } from "./ldap-patch-payload";

export type LdapFormValues = LdapPatchFormValues;

export function useLdapDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<LdapFormValues>();
  const [previewOpen, setPreviewOpen] = useState(false);

  const detailQuery = useQuery({
    queryKey: queryKeys.ldaps.detail(id!),
    queryFn: () => getIamLdap(id!),
    enabled: !!id,
  });

  const previewQuery = useQuery({
    queryKey: ["ldap", "iam", "preview", id],
    queryFn: () => getIamLdapPreviewUsers(id!),
    enabled: !!id && previewOpen,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const d = detailQuery.data;
      const customAttrsStr =
        d.customAttributes && Object.keys(d.customAttributes).length > 0
          ? JSON.stringify(d.customAttributes, null, 2)
          : "";
      form.setFieldsValue({
        displayName: d.displayName,
        serverName: d.serverName,
        type: d.type || "OpenLDAP",
        serverHost: d.host,
        port: d.port,
        enableSsl: d.enableSsl,
        allowSelfSignedCert: d.allowSelfSignedCert,
        baseDn: d.baseDn,
        filter: d.filter,
        filterFields: d.filterFields || [],
        admin: d.admin,
        // 后端不返回绑定密码；留空表示沿用数据库中的值。
        password: "",
        passwordType: d.passwordType || "Plain",
        attributeName: d.attributeName || "uid",
        defaultGroups: d.defaultGroups || [],
        enableGroups: d.enableGroups,
        customAttributes: customAttrsStr,
        autoSync: (d.syncInterval ?? 0) > 0,
        syncInterval: d.syncInterval || 60,
        isEnabled: d.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: LdapFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamLdap(
        detailQuery.data.id,
        buildLdapPatchPayload(values, detailQuery.data.revision),
      );
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["ldaps"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamLdap(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.ldaps.all });
      navigate("/ldaps");
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tc, "common:error"));
    },
  });

  return {
    id,
    tc,
    navigate,
    form,
    previewOpen,
    setPreviewOpen,
    detailQuery,
    previewQuery,
    updateMutation,
    deleteMutation,
  };
}
