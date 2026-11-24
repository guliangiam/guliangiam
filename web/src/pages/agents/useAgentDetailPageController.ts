import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamAgent, getIamAgent, patchIamAgent } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

export type AgentFormValues = {
  displayName: string;
  host: string;
  port: number;
  isEnabled: boolean;
};

export function useAgentDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<AgentFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.agents.detail(id!),
    queryFn: () => getIamAgent(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        host: detailQuery.data.host,
        port: detailQuery.data.port,
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: AgentFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamAgent(detailQuery.data.id, {
        displayName: values.displayName,
        host: values.host,
        port: values.port,
        isEnabled: values.isEnabled,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamAgent(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
      navigate("/ai/agents");
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
