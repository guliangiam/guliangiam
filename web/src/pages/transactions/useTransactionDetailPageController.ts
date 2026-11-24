import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamTransaction, getIamTransaction, patchIamTransaction } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { TRANSACTION_STATUSES, stateCaptionKey } from "@/shared/constants/enums";
import { TRANSACTION_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { localizedError } from "@/shared/lib/server-error";

export type TransactionFormValues = {
  status: string;
};

export function useTransactionDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<TransactionFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.transactions.detail(id!),
    queryFn: () => getIamTransaction(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        status: detailQuery.data.status,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: TransactionFormValues) => {
      if (!detailQuery.data) throw new Error(tr("transaction.dataNotLoaded"));
      return patchIamTransaction(detailQuery.data.id, {
        status: values.status,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tr("transaction.updated"));
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("transaction.dataNotLoaded"));
      return deleteIamTransaction(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("transaction.deleted"));
      void queryClient.invalidateQueries({ queryKey: ["transactions"] });
      navigate("/transactions");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "transaction.deleteFailed"));
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
    TRANSACTION_STATUSES,
    TRANSACTION_TRANSITIONS,
    allowedTargets,
    stateCaptionKey,
  };
}
