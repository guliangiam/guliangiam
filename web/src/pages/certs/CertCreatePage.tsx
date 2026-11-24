import { App, Form, Input, InputNumber, Select } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { createIamCert } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { GlFormRow } from "@/shared/ui/GlForm";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { localizedError } from "@/shared/lib/server-error";

// 后端 app/certs/validation.go：scope ∈ {JWT, SSL}，type ∈ {x509, SSL}。
const SCOPE_OPTIONS = [
  { label: "JWT", value: "JWT" },
  { label: "SSL", value: "SSL" },
];

const CERT_TYPE_OPTIONS = [
  { label: "x509", value: "x509" },
  { label: "SSL", value: "SSL" },
];

const JWT_ALGORITHM_OPTIONS = [
  { label: "RS256", value: "RS256" },
  { label: "RS384", value: "RS384" },
  { label: "RS512", value: "RS512" },
  { label: "ES256", value: "ES256" },
  { label: "ES384", value: "ES384" },
  { label: "ES512", value: "ES512" },
  { label: "PS256", value: "PS256" },
  { label: "PS384", value: "PS384" },
  { label: "PS512", value: "PS512" },
];

// 后端只接受 RS/ES/PS 系列 cryptoAlgorithm（RSA/ECC 会被 validation 拒绝）。

const BIT_SIZE_OPTIONS = [
  { label: "1024", value: 1024 },
  { label: "2048", value: 2048 },
  { label: "4096", value: 4096 },
];

const SSL_PROVIDER_OPTIONS = [
  { label: "GoDaddy", value: "GoDaddy" },
  { label: "Aliyun", value: "Aliyun" },
];

type CertFormValues = {
  organizationId: string;
  name: string;
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

export default function CertCreatePage() {
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<CertFormValues>();

  const certType = Form.useWatch("type", form) ?? "JWT";
  const algorithm = Form.useWatch("algorithm", form) ?? "RS256";
  const isSSL = certType === "SSL";
  const isES = algorithm.startsWith("ES");

  const mutation = useMutation({
    mutationFn: async (values: CertFormValues) => {
      return createIamCert({
        organizationId: values.organizationId,
        name: values.name,
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
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.certs.all });
      message.success(tc("createSuccess"));
      navigate("/certs");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  return (
    <ResourceFormPage
      title={tc("certCreateTitle")}
      description={tc("certCreateDescription")}
      saving={mutation.isPending}
      form={form}
      onCancel={() => navigate("/certs")}
      onFinish={async (values: CertFormValues) => {
        await mutation.mutateAsync(values);
      }}
    >
      <Form.Item
        name="organizationId"
        rules={[{ required: true, message: tc("validation.organizationIdRequired") }]}
      >
        <GlFormRow label={tc("organizationId")} required>
          <OrganizationSelect />
        </GlFormRow>
      </Form.Item>
      <Form.Item name="name" rules={[{ required: true, message: tc("validation.nameRequired") }]}>
        <GlFormRow label={tc("name")} required>
          <Input placeholder="e.g. cert-tls" />
        </GlFormRow>
      </Form.Item>

      <FormSection title={tc("certBasicInfo")}>
        <Form.Item name="scope" initialValue="JWT">
          <GlFormRow label="Scope">
            <Select options={SCOPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="type" initialValue="x509" rules={[{ required: true }]}>
          <GlFormRow label={tc("type")} required>
            <Select options={CERT_TYPE_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="algorithm" initialValue="RS256" rules={[{ required: true }]}>
          <GlFormRow label={tc("algorithm")} required>
            <Select options={JWT_ALGORITHM_OPTIONS} />
          </GlFormRow>
        </Form.Item>
        {!isES && !isSSL && (
          <Form.Item name="bitSize" initialValue={2048}>
            <GlFormRow label="Bit Size">
              <Select options={BIT_SIZE_OPTIONS} />
            </GlFormRow>
          </Form.Item>
        )}
        {!isSSL && (
          <Form.Item name="expireInYears" initialValue={1}>
            <GlFormRow label="Expire In Years">
              <InputNumber min={1} max={30} style={{ width: "100%" }} />
            </GlFormRow>
          </Form.Item>
        )}
        <Form.Item
          name="displayName"
          rules={[{ required: true, message: tc("validation.displayNameRequired") }]}
        >
          <GlFormRow label={tc("displayName")} required>
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      {isSSL && (
        <FormSection title={tc("certSslConfig")}>
          <Form.Item name="provider">
            <GlFormRow label="Provider">
              <Select
                options={SSL_PROVIDER_OPTIONS}
                allowClear
                placeholder={tc("selectSslProvider")}
              />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="account">
            <GlFormRow label="Account">
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="accessKey">
            <GlFormRow label="Access Key">
              <Input />
            </GlFormRow>
          </Form.Item>
          <Form.Item name="accessSecret">
            <GlFormRow label="Access Secret">
              <Input.Password />
            </GlFormRow>
          </Form.Item>
        </FormSection>
      )}

      <FormSection title={tc("certificateContent")}>
        <Form.Item name="certificate">
          <GlFormRow label={tc("certificate")}>
            <Input.TextArea
              rows={6}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("certificatePlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="privateKey">
          <GlFormRow label={tc("privateKey")}>
            <Input.TextArea
              rows={6}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("privateKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
      </FormSection>

      <FormSection title={tc("certAuthority")}>
        <Form.Item name="authorityPublicKey">
          <GlFormRow label={tc("authorityPublicKey")}>
            <Input.TextArea
              rows={4}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("authorityPublicKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="authorityRootPublicKey">
          <GlFormRow label={tc("authorityRootPublicKey")}>
            <Input.TextArea
              rows={4}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              placeholder={tc("authorityRootPublicKeyPlaceholder")}
            />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="acmeIssuer">
          <GlFormRow label="ACME Issuer">
            <Input />
          </GlFormRow>
        </Form.Item>
        <Form.Item name="acmeEndpoint">
          <GlFormRow label="ACME Endpoint">
            <Input />
          </GlFormRow>
        </Form.Item>
      </FormSection>
    </ResourceFormPage>
  );
}
