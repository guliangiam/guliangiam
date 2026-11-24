import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  deleteIamWebhook,
  getIamWebhook,
  patchIamWebhook,
  getIamWebhookEvents,
  replayIamWebhookEvent,
  deleteIamWebhookEvent,
  type IamWebhookEventItem,
} from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { getOrganizationOptions } from "@/shared/api/iam/organization";
import { getCertOptions } from "@/shared/api/iam/cert";
import { localizedError } from "@/shared/lib/server-error";
import { buildWebhookPatchPayload, type WebhookFormValues } from "./webhook-patch-payload";

export type { WebhookFormValues };

export const METHOD_VALUES = ["POST", "GET", "PUT", "DELETE"] as const;
export const CONTENT_TYPE_VALUES = [
  "application/json",
  "application/x-www-form-urlencoded",
] as const;
export const WEBHOOK_EVENTS = [
  "user.created",
  "user.updated",
  "user.deleted",
  "organization.created",
  "organization.updated",
  "organization.deleted",
  "application.created",
  "application.updated",
  "application.deleted",
  "provider.created",
  "provider.updated",
  "provider.deleted",
  "role.created",
  "role.updated",
  "role.deleted",
  "permission.created",
  "permission.updated",
  "permission.deleted",
  "group.created",
  "group.updated",
  "group.deleted",
  "session.created",
  "session.revoked",
  "token.issued",
  "token.revoked",
  "login.success",
  "login.failure",
  "password.changed",
  "password.reset",
  "mfa.enabled",
  "mfa.disabled",
];
export const WEBHOOK_OBJECT_FIELDS = [
  "All",
  "owner",
  "name",
  "createdTime",
  "displayName",
  "type",
  "isEnabled",
  "description",
  "organization",
  "email",
  "phone",
];

export function useWebhookDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<WebhookFormValues>();

  const METHOD_OPTIONS = METHOD_VALUES.map((v) => ({
    label: tr(`webhook.methodOption.${v}`),
    value: v,
  }));
  const CONTENT_TYPE_OPTIONS = CONTENT_TYPE_VALUES.map((v) => ({
    label: tr(`webhook.contentTypeOption.${v}`),
    value: v,
  }));

  // ─── Queries ─────────────────────────────────────────────────
  const detailQuery = useQuery({
    queryKey: queryKeys.webhooks.detail(id!),
    queryFn: () => getIamWebhook(id!),
    enabled: !!id,
  });

  const orgOptionsQuery = useQuery({
    queryKey: ["org-options"],
    queryFn: () => getOrganizationOptions(),
    staleTime: 60_000,
  });

  const certOptionsQuery = useQuery({
    queryKey: ["cert-options"],
    queryFn: () => getCertOptions(),
    staleTime: 60_000,
  });

  const eventsQuery = useQuery({
    queryKey: ["webhook-events", "iam", id],
    queryFn: () => getIamWebhookEvents(id!),
    enabled: !!id,
  });

  // ─── Form init ───────────────────────────────────────────────
  useEffect(() => {
    if (detailQuery.data) {
      form.setFieldsValue({
        displayName: detailQuery.data.displayName,
        organization: detailQuery.data.organization || "",
        url: detailQuery.data.url,
        method: detailQuery.data.method || "POST",
        contentType: detailQuery.data.contentType || "application/json",
        headers: detailQuery.data.headers || "",
        events: detailQuery.data.events || [],
        customBody: detailQuery.data.customBody || "",
        customHeaders: detailQuery.data.customHeaders || "",
        certId: detailQuery.data.certId || "",
        // 详情只回传脱敏投影：留空 = 保持已存储签名密钥（BE-B-7）。
        secret: "",
        isUserExtended: detailQuery.data.isUserExtended,
        tokenFields: detailQuery.data.tokenFields || [],
        singleOrgOnly: detailQuery.data.singleOrgOnly,
        objectFields: detailQuery.data.objectFields || [],
        maxRetries: detailQuery.data.maxRetries || 3,
        retryInterval: detailQuery.data.retryInterval || 60,
        useExponentialBackoff: detailQuery.data.useExponentialBackoff,
        timeoutMs: detailQuery.data.timeoutMs || 5000,
        isEnabled: detailQuery.data.isEnabled,
      });
    }
  }, [detailQuery.data, form]);

  // ─── Mutations ───────────────────────────────────────────────
  const updateMutation = useMutation({
    mutationFn: async (values: WebhookFormValues) => {
      if (!detailQuery.data) throw new Error(tr("webhook.detailNotLoaded"));
      return patchIamWebhook(
        detailQuery.data.id,
        buildWebhookPatchPayload(values, detailQuery.data.revision),
      );
    },
    onSuccess: () => {
      message.success(tr("webhook.updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["webhooks"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tr, "webhook.updateFailed"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("webhook.detailNotLoaded"));
      return deleteIamWebhook(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["webhooks"] });
      navigate("/webhooks");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const replayEventMutation = useMutation({
    mutationFn: (eventId: string) => replayIamWebhookEvent(eventId),
    onSuccess: () => {
      message.success(tr("webhook.eventReplayed", "Event replayed successfully"));
      void queryClient.invalidateQueries({ queryKey: ["webhook-events"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteEventMutation = useMutation({
    mutationFn: (eventId: string) => deleteIamWebhookEvent(eventId),
    onSuccess: () => {
      message.success(tr("webhook.eventDeleted", "Event deleted"));
      void queryClient.invalidateQueries({ queryKey: ["webhook-events"] });
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
    METHOD_OPTIONS,
    CONTENT_TYPE_OPTIONS,
    WEBHOOK_EVENTS,
    WEBHOOK_OBJECT_FIELDS,
    // Queries
    detailQuery,
    orgOptionsQuery,
    certOptionsQuery,
    eventsQuery,
    // Mutations
    updateMutation,
    deleteMutation,
    replayEventMutation,
    deleteEventMutation,
  };
}

export type { IamWebhookEventItem };
