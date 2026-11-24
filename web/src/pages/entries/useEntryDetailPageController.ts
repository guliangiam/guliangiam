import { useEffect } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteIamEntry, getIamEntry, patchIamEntry } from "@/shared/api/iam";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

export type EntryFormValues = {
  displayName: string;
};

export function useEntryDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<EntryFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const detailQuery = useQuery({
    queryKey: queryKeys.entries.detail(id!),
    queryFn: () => getIamEntry(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({ displayName: detailQuery.data.displayName });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: EntryFormValues) => {
      if (!detailQuery.data) throw new Error(tr("entry.dataNotLoaded"));
      return patchIamEntry(detailQuery.data.id, {
        displayName: values.displayName,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tr("entry.updated"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.entries.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("entry.dataNotLoaded"));
      return deleteIamEntry(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tr("entry.deleted"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.entries.all });
      navigate("/entries");
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "entry.deleteFailed"));
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
