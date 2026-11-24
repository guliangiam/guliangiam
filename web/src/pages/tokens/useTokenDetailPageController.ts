import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  createToken,
  deleteToken,
  revokeToken,
  getToken,
  getUserOptions,
  getApplicationOptions,
  type CreatedToken,
} from "@/shared/api/iam";
import { useSessionStore } from "@/features/session/session-store";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

export type TokenCreateFormValues = {
  organizationId: string;
  userId: string;
  applicationId: string;
  scopes: string[];
  audience: string[];
  expiresInSeconds: number;
  metadataText: string;
};

export function useTokenDetailPageController() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { tokenName = "new" } = useParams();
  const organization = useSessionStore((state) => state.organization);
  const isNew = tokenName === "new";
  const [form] = Form.useForm<TokenCreateFormValues>();
  const [createdToken, setCreatedToken] = useState<CreatedToken | null>(null);
  const [showFullTokenValue, setShowFullTokenValue] = useState(false);
  const { nameMap: orgNameMap } = useOrganizationNames();

  const organizationId = organization?.id ?? "";

  useEffect(() => {
    if (!isNew || organizationId === "") return;
    form.setFieldsValue({
      organizationId,
      expiresInSeconds: 3600,
      scopes: ["read"],
      metadataText: '{\n  "source": "auth-ui"\n}',
    });
  }, [form, isNew, organizationId]);

  const usersQuery = useQuery({
    queryKey: ["users", "auth", "options", organizationId],
    queryFn: () => getUserOptions(organizationId),
    enabled: isNew && organizationId !== "",
  });
  const applicationsQuery = useQuery({
    queryKey: ["applications", "auth", "options", organizationId],
    queryFn: () => getApplicationOptions(organizationId),
    enabled: isNew && organizationId !== "",
  });
  const detailQuery = useQuery({
    queryKey: ["token", "auth", tokenName],
    queryFn: () => getToken(tokenName),
    enabled: !isNew,
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("token.notFound"));
      await deleteToken(detailQuery.data.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["tokens", "auth"] });
      message.success(tr("token.deleteSuccess"));
      navigate("/tokens");
    },
    onError: (error) => message.error(localizedError(error, tr, "token.deleteFailed")),
  });

  const revokeMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("token.notFound"));
      await revokeToken(detailQuery.data.id);
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tokens", "auth"] }),
        queryClient.invalidateQueries({ queryKey: ["token", "auth", tokenName] }),
      ]);
      message.success(tr("token.revokeSuccess"));
    },
    onError: (error) => message.error(localizedError(error, tr, "token.revokeFailed")),
  });

  const createMutation = useMutation({
    mutationFn: async (values: TokenCreateFormValues) => {
      let metadata: Record<string, unknown> = {};
      const metadataText = values.metadataText?.trim() ?? "";
      if (metadataText !== "") {
        let parsed: unknown;
        try {
          parsed = JSON.parse(metadataText);
        } catch {
          throw new Error(
            tr("token.validation.metadataInvalidJson", "Metadata must be valid JSON"),
          );
        }
        if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
          throw new Error(tr("token.validation.metadataMustBeObject"));
        }
        metadata = parsed as Record<string, unknown>;
      }
      return createToken({
        organizationId: values.organizationId,
        userId: values.userId,
        applicationId: values.applicationId,
        scopes: values.scopes,
        audience: values.audience ?? [],
        expiresInSeconds: values.expiresInSeconds,
        metadata,
      });
    },
    onSuccess: async (result) => {
      setCreatedToken(result);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tokens", "auth"] }),
        queryClient.invalidateQueries({ queryKey: ["token", "auth", result.token.id] }),
      ]);
      message.success(tr("token.createSuccess"));
    },
    onError: (error) => message.error(localizedError(error, tr, "token.createFailed")),
  });

  return {
    tc,
    tr,
    navigate,
    tokenName,
    isNew,
    orgNameMap,
    form,
    createdToken,
    setCreatedToken,
    showFullTokenValue,
    setShowFullTokenValue,
    // Queries
    usersQuery,
    applicationsQuery,
    detailQuery,
    // Mutations
    createMutation,
    deleteMutation,
    revokeMutation,
  };
}
