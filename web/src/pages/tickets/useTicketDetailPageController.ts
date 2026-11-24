import { useEffect, useState } from "react";
import { App, Form } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  deleteIamTicket,
  getIamTicket,
  patchIamTicket,
  listTicketMessages,
  addTicketMessage,
} from "@/shared/api/iam";
import { useSessionStore } from "@/features/session/session-store";
import { TICKET_STATUSES } from "@/shared/constants/enums";
import { TICKET_TRANSITIONS, allowedTargets } from "@/shared/constants/lifecycles";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

export const PRIORITY_COLORS: Record<string, string> = {
  Low: "default",
  Medium: "blue",
  High: "orange",
  Urgent: "red",
};

export type TicketFormValues = {
  displayName: string;
  type: string;
  status: string;
  priority: string;
  assignee: string;
  description: string;
};

export function useTicketDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<TicketFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();
  const [replyText, setReplyText] = useState("");
  const account = useSessionStore((state) => state.account);
  // 列表页跳转时携带行的 organizationId：平台管理员跨组织访问靠它过可信租户裁决。
  const [searchParams] = useSearchParams();
  const organizationId = searchParams.get("organizationId") || undefined;

  const detailQuery = useQuery({
    queryKey: [...queryKeys.tickets.detail(id!), organizationId],
    queryFn: () => getIamTicket(id!, organizationId),
    enabled: !!id,
  });

  const messagesQuery = useQuery({
    queryKey: [...queryKeys.tickets.messages(id!), organizationId],
    queryFn: () => listTicketMessages(id!, { organizationId }),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        type: detailQuery.data.type,
        status: detailQuery.data.status,
        priority: detailQuery.data.priority,
        assignee: detailQuery.data.assignee,
        description: detailQuery.data.description,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: TicketFormValues) => {
      if (!detailQuery.data) throw new Error(tr("ticket.detailNotLoaded"));
      return patchIamTicket(
        detailQuery.data.id,
        {
          displayName: values.displayName,
          type: values.type,
          status: values.status,
          priority: values.priority,
          assignee: values.assignee,
          description: values.description,
          revision: detailQuery.data.revision,
        },
        organizationId,
      );
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("ticket.detailNotLoaded"));
      return deleteIamTicket(detailQuery.data.id, organizationId);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all });
      navigate("/tickets");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const replyMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data || !replyText.trim())
        throw new Error(tr("ticket.validation.replyRequired"));
      return addTicketMessage(
        detailQuery.data.id,
        {
          // 回复人是当前登录用户，而不是工单创建者。
          author: account?.displayName || account?.name || "system",
          content: replyText.trim(),
        },
        organizationId,
      );
    },
    onSuccess: () => {
      message.success(tr("ticket.replySent"));
      setReplyText("");
      void queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "ticket.replyFailed"));
    },
  });

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    replyText,
    setReplyText,
    detailQuery,
    messagesQuery,
    updateMutation,
    deleteMutation,
    replyMutation,
    TICKET_STATUSES,
    TICKET_TRANSITIONS,
    allowedTargets,
    PRIORITY_COLORS,
  };
}
