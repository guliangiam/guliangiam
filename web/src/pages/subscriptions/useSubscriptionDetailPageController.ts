import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamSubscription, getIamSubscription, patchIamSubscription } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { SUBSCRIPTION_STATES } from "@/shared/constants/enums";
import { SUBSCRIPTION_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

export type SubscriptionFormValues = {
  displayName: string;
  state: string;
};

export function useSubscriptionDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SubscriptionFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.subscriptions.detail(id!),
    queryFn: () => getIamSubscription(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        state: detailQuery.data.state,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: SubscriptionFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamSubscription(detailQuery.data.id, {
        displayName: values.displayName,
        state: values.state,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.subscriptions.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamSubscription(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.subscriptions.all });
      navigate("/subscriptions");
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
    SUBSCRIPTION_STATES,
    SUBSCRIPTION_TRANSITIONS,
    allowedTargets,
  };
}
