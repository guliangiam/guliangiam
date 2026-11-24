import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { App, Form } from "antd";
import {
  deleteIamCert,
  getIamCert,
  patchIamCert,
  checkDomainExpiration,
  type IamCertDetail,
} from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

export type CertFormValues = {
  displayName: string;
  scope: string;
  type: string;
  algorithm: string;
  bitSize: number;
  expireInYears: number;
  certificate: string;
  privateKey: string;
  authorityPublicKey: string;
  authorityRootPublicKey: string;
  acmeIssuer: string;
  acmeEndpoint: string;
  provider: string;
  account: string;
  accessKey: string;
  accessSecret: string;
};

export function useCertDetailPageController() {
  const { id } = useParams<{ id: string }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CertFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const certType = Form.useWatch("type", form) ?? "JWT";
  const algorithm = Form.useWatch("algorithm", form) ?? "RS256";
  const isSSL = certType === "SSL";
  const isES = algorithm.startsWith("ES");

  const detailQuery = useQuery({
    queryKey: queryKeys.certs.detail(id!),
    queryFn: () => getIamCert(id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data as IamCertDetail;
      form.setFieldsValue({
        displayName: detail.displayName,
        scope: detail.scope || "JWT",
        type: detail.type || "JWT",
        algorithm: detail.cryptoAlgorithm || "RS256",
        bitSize: detail.bitSize || 2048,
        expireInYears: detail.expireInYears || 1,
        certificate: detail.certificate,
        privateKey: detail.privateKey,
        authorityPublicKey: detail.authorityPublicKey,
        authorityRootPublicKey: detail.authorityRootPublicKey,
        acmeIssuer: detail.acmeIssuer,
        acmeEndpoint: detail.acmeEndpoint,
        provider: detail.provider,
        account: detail.account,
        accessKey: detail.accessKey,
        accessSecret: detail.accessSecret,
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: CertFormValues) => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return patchIamCert(detailQuery.data.id, {
        displayName: values.displayName,
        scope: values.scope,
        type: values.type,
        cryptoAlgorithm: values.algorithm,
        bitSize: values.bitSize,
        expireInYears: values.expireInYears,
        certificate: values.certificate,
        privateKey: values.privateKey,
        authorityPublicKey: values.authorityPublicKey,
        authorityRootPublicKey: values.authorityRootPublicKey,
        acmeIssuer: values.acmeIssuer,
        acmeEndpoint: values.acmeEndpoint,
        provider: values.provider,
        account: values.account,
        accessKey: values.accessKey,
        accessSecret: values.accessSecret,
        revision: detailQuery.data.revision,
      });
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["certs"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return deleteIamCert(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["certs"] });
      navigate("/certs");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const checkDomainMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tc("detailNotLoaded"));
      return checkDomainExpiration(detailQuery.data.id);
    },
    onSuccess: () => {
      message.success(tc("domainExpireChecked"));
      void queryClient.invalidateQueries({ queryKey: ["certs"] });
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => message.success(tc("copySuccess")),
      () => message.error(tc("copyFailed")),
    );
  };

  const detail = detailQuery.data as IamCertDetail;
  const certValue = form.getFieldValue("certificate") ?? detail?.certificate ?? "";
  const keyValue = form.getFieldValue("privateKey") ?? detail?.privateKey ?? "";

  return {
    id,
    tc,
    tr,
    navigate,
    form,
    orgNameMap,
    isSSL,
    isES,
    detail,
    certValue,
    keyValue,
    // Queries
    detailQuery,
    // Mutations
    updateMutation,
    deleteMutation,
    checkDomainMutation,
    // Handlers
    handleCopy,
  };
}
