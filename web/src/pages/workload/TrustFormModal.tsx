import { useState } from "react";
import { App, Button, Form, Input, Modal, Select, Space, Tag, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { CreateFederationTrustRequest, SubjectMatch } from "@/shared/api/iam/identity-pools";
import {
  useCreateFederationTrust,
  useMatchFederationTrustSubject,
} from "@/shared/hooks/use-identity-pools";
import { handleMutationError } from "@/shared/hooks/useMutationError";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import { isValidHttpsUri, isValidSubjectPattern } from "./workload-utils";

type TrustFormModalProps = {
  open: boolean;
  poolId: string;
  onClose: () => void;
};

type TrustFormValues = CreateFederationTrustRequest & {
  samples: string[];
};

export function TrustFormModal({ open, poolId, onClose }: TrustFormModalProps) {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const [form] = Form.useForm<TrustFormValues>();
  const [matches, setMatches] = useState<SubjectMatch[]>([]);
  const createMutation = useCreateFederationTrust(poolId);
  const previewMutation = useMatchFederationTrustSubject(poolId);

  const close = () => {
    form.resetFields();
    setMatches([]);
    createMutation.reset();
    previewMutation.reset();
    onClose();
  };

  const preview = async () => {
    const values = await form.validateFields(["subjects", "samples"]).catch(() => undefined);
    if (!values) return;
    try {
      const result = await previewMutation.mutateAsync({
        patterns: values.subjects,
        samples: values.samples,
      });
      setMatches(result);
    } catch (error) {
      handleMutationError(
        error,
        tc("workload.previewFailed", { defaultValue: "Failed to preview subject matches" }),
      );
    }
  };

  const submit = async () => {
    const values = await form.validateFields().catch(() => undefined);
    if (!values) return;
    const input: CreateFederationTrustRequest = {
      name: values.name,
      issuer: values.issuer,
      jwksUri: values.jwksUri,
      audience: values.audience,
      subjects: values.subjects,
    };

    try {
      await createMutation.mutateAsync(input);
      message.success(tc("workload.trustCreated", { defaultValue: "Federation trust created" }));
      close();
    } catch (error) {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<TrustFormValues>(form), tc);
      }
      handleMutationError(
        error,
        tc("workload.trustCreateFailed", { defaultValue: "Failed to create federation trust" }),
      );
    }
  };

  const httpsValidator = (_rule: unknown, value: string) =>
    isValidHttpsUri(value)
      ? Promise.resolve()
      : Promise.reject(
          new Error(tc("workload.httpsRequired", { defaultValue: "Enter an absolute HTTPS URI" })),
        );

  return (
    <Modal
      open={open}
      title={tc("workload.createTrust", { defaultValue: "Create Federation Trust" })}
      okText={tc("create", { defaultValue: "Create" })}
      cancelText={tc("cancel", { defaultValue: "Cancel" })}
      confirmLoading={createMutation.isPending}
      onOk={() => void submit()}
      onCancel={close}
      width={720}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{ audience: [], subjects: [], samples: [] }}
        onValuesChange={() => setMatches([])}
      >
        <Form.Item
          label={tc("name", { defaultValue: "Name" })}
          name="name"
          rules={[{ required: true, whitespace: true }]}
        >
          <Input placeholder="github-actions-production" />
        </Form.Item>
        <Form.Item
          label={tc("workload.issuer", { defaultValue: "Issuer" })}
          name="issuer"
          rules={[{ required: true, whitespace: true }, { validator: httpsValidator }]}
        >
          <Input placeholder="https://token.actions.githubusercontent.com" />
        </Form.Item>
        <Form.Item
          label={tc("workload.jwksUri", { defaultValue: "JWKS URI" })}
          name="jwksUri"
          rules={[{ required: true, whitespace: true }, { validator: httpsValidator }]}
        >
          <Input placeholder="https://issuer.example.com/.well-known/jwks.json" />
        </Form.Item>
        <Form.Item
          label={tc("workload.audience", { defaultValue: "Audience" })}
          name="audience"
          rules={[
            {
              required: true,
              message: tc("workload.audienceRequired", {
                defaultValue: "Add at least one audience",
              }),
            },
          ]}
        >
          <Select mode="tags" tokenSeparators={[","]} placeholder="api://workload" />
        </Form.Item>
        <Form.Item
          label={tc("workload.subjects", { defaultValue: "Subject patterns" })}
          name="subjects"
          extra={tc("workload.subjectWildcardHelp", {
            defaultValue:
              "Use exact subjects or a single trailing /* wildcard (for example repo:acme/*). Bare * and substring wildcards are not allowed.",
          })}
          rules={[
            {
              required: true,
              message: tc("workload.subjectsRequired", {
                defaultValue: "Add at least one subject pattern",
              }),
            },
            {
              validator: (_rule, value: string[]) =>
                value.length > 0 && value.every(isValidSubjectPattern)
                  ? Promise.resolve()
                  : Promise.reject(
                      new Error(
                        tc("workload.invalidSubjectPattern", {
                          defaultValue:
                            "Subject patterns must be exact or end with one /* wildcard",
                        }),
                      ),
                    ),
            },
          ]}
        >
          <Select mode="tags" tokenSeparators={[","]} placeholder="repo:acme/*" />
        </Form.Item>
        <Form.Item
          label={tc("workload.samples", { defaultValue: "Sample subjects" })}
          name="samples"
          rules={[
            {
              required: true,
              message: tc("workload.samplesRequired", {
                defaultValue: "Add at least one sample subject",
              }),
            },
          ]}
        >
          <Select mode="tags" tokenSeparators={[","]} placeholder="repo:acme/api:ref:main" />
        </Form.Item>
        <Space orientation="vertical" style={{ width: "100%" }}>
          <Button onClick={() => void preview()} loading={previewMutation.isPending}>
            {tc("workload.previewMatches", { defaultValue: "Preview subject matches" })}
          </Button>
          {matches.length > 0 && (
            <div aria-live="polite">
              <Typography.Text strong>
                {tc("workload.matchResults", { defaultValue: "Match results" })}
              </Typography.Text>
              <Space orientation="vertical" style={{ display: "flex", marginTop: 8 }}>
                {matches.map((match) => (
                  <Space key={match.subject}>
                    <Tag color={match.allowed ? "green" : "red"}>
                      {match.allowed
                        ? tc("workload.allowed", { defaultValue: "Allowed" })
                        : tc("workload.denied", { defaultValue: "Denied" })}
                    </Tag>
                    <Typography.Text code>{match.subject}</Typography.Text>
                  </Space>
                ))}
              </Space>
            </div>
          )}
        </Space>
      </Form>
    </Modal>
  );
}
