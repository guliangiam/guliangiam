import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamOrder,
  getIamOrder,
  patchIamOrder,
  cancelExistingOrder,
  payExistingOrder,
} from "@/shared/api/iam";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { ORDER_STATES } from "@/shared/constants/enums";
import { ORDER_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

export type OrderFormValues = {
  displayName: string;
  state: string;
};

export function useOrderDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<OrderFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.orders.detail(id!),
    queryFn: () => getIamOrder(id!),
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
    mutationFn: async (values: OrderFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamOrder(detailQuery.data.id, {
        displayName: values.displayName,
        state: values.state,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamOrder(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
      navigate("/orders");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const cancelMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return cancelExistingOrder(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("order.cancelled"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "order.cancelFailed"));
    },
  });

  const payMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      const paymentRef = detailQuery.data.tradeNo || detailQuery.data.payUrl;
      return payExistingOrder(detailQuery.data.id, paymentRef);
    },
    onSuccess: () => {
      message.success(tr("order.paymentSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.orders.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "order.paymentFailed"));
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
    cancelMutation,
    payMutation,
    ORDER_STATES,
    ORDER_TRANSITIONS,
    allowedTargets,
  };
}
