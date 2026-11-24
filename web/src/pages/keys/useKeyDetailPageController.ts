import { useEffect } from "react";
import dayjs from "dayjs";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamKey, getIamKey, patchIamKey } from "@/shared/api/iam";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";
import { buildKeyPatchPayload, type KeyFormValues } from "./key-patch-payload";

export type { KeyFormValues };

export const KEY_TYPE_OPTIONS = [
  { label: "Organization", value: "Organization" },
  { label: "Application", value: "Application" },
  { label: "User", value: "User" },
  { label: "General", value: "General" },
  { label: "Prometheus", value: "Prometheus" },
];

export const KEY_ALGORITHM_OPTIONS = [
  { label: "RSA", value: "RSA" },
  { label: "ECDSA", value: "ECDSA" },
  { label: "AES", value: "AES" },
];

/** 与后端 PatchKeyRequest 完全一致的字段集合（定义见 key-patch-payload.ts）。 */

export function useKeyDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<KeyFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.keys.detail(id!),
    queryFn: () => getIamKey(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data;
      form.setFieldsValue({
        keyType: detail.keyType,
        algorithm: detail.algorithm ?? "",
        publicKey: detail.publicKey ?? "",
        // 详情接口不回传私钥：留空表示“保持不变”，提交时省略该字段（BE-B-6）。
        privateKey: "",
        displayName: detail.displayName,
        expiresAt: detail.expiresAt ? dayjs(detail.expiresAt) : null,
        isEnabled: detail.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: KeyFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamKey(
        detailQuery.data.id,
        buildKeyPatchPayload(values, detailQuery.data.revision),
      );
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamKey(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["keys"] });
      navigate("/keys");
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
    orgNameMap,
    detailQuery,
    updateMutation,
    deleteMutation,
  };
}
