import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  MinusCircleOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import {
  ACTION_OPTIONS,
  RULE_TYPE_OPTIONS,
  SCOPE_OPTIONS,
  type RuleFormValues,
  type useRuleDetailPageController,
} from "./useRuleDetailPageController";

type RuleDetailPageController = ReturnType<typeof useRuleDetailPageController>;

export function RuleDetailPageView({ ctrl }: { ctrl: RuleDetailPageController }) {
  const {
    id,
    tc,
    tr,
    navigate,
    form,
    ruleType,
    validationResult,
    validating,
    detailQuery,
    updateMutation,
    deleteMutation,
    handleValidate,
  } = ctrl;

  if (detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  if (!detailQuery.data) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("rule.notFound")}
          description={tr("rule.notFoundDesc", { id })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/rules")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  return (
    <ResourceFormPage
      title={tr("rule.detailTitle", { name: detailQuery.data.name })}
      description={tr("rule.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/rules")}
      onFinish={async (values: RuleFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <Popconfirm
          title={tc("deleteConfirm")}
          description={tr("rule.deleteDesc")}
          onConfirm={() => deleteMutation.mutate()}
          okText={tc("delete")}
          cancelText={tc("cancel")}
          okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
        >
          <Button danger>{tc("delete")}</Button>
        </Popconfirm>
      }
    >
      <FormSection title={tc("overview", "Overview")}>
        <Form.Item label={tc("statusLabel")}>
          {detailQuery.data.isEnabled ? (
            <Tag color="green">{tc("enabled")}</Tag>
          ) : (
            <Tag color="red">{tc("disabled")}</Tag>
          )}
        </Form.Item>
      </FormSection>

      <FormSection title={tc("ruleBasicInfo")}>
        <Form.Item
          label={tc("name")}
          name="name"
          rules={[{ required: true, message: tc("validation.nameRequired") }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label={tc("scope")} name="scope">
          <Select options={SCOPE_OPTIONS} />
        </Form.Item>
        <Form.Item label={tc("ruleType")} name="ruleType">
          <Select options={RULE_TYPE_OPTIONS} />
        </Form.Item>
        <Form.Item label={tc("enabled")} name="isEnabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </FormSection>

      <FormSection title={tc("ruleConditions")}>
        {ruleType === "IP" && (
          <Form.Item label={tc("ipRange")} name="ipRange">
            <Input placeholder={tc("ipRangePlaceholder")} />
          </Form.Item>
        )}
        {ruleType === "WAF" && (
          <Form.Item label={tc("wafPattern")} name="wafPattern">
            <Input.TextArea rows={3} placeholder={tc("wafPatternPlaceholder")} />
          </Form.Item>
        )}
      </FormSection>

      <FormSection title={tc("ruleAction")}>
        <Form.Item label={tr("rule.action")} name="action">
          <Select options={ACTION_OPTIONS} />
        </Form.Item>
        <Form.Item label={tc("priority")} name="priority">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
      </FormSection>

      <FormSection title={tc("matchConditions")}>
        <Form.List name="conditions">
          {(fields, { add, remove }) => (
            <>
              {fields.map(({ key, name, ...restField }) => (
                <Space key={key} style={{ display: "flex", marginBottom: 8 }} align="baseline">
                  <Form.Item
                    {...restField}
                    name={[name, "field"]}
                    rules={[{ required: true, message: tr("rule.validation.fieldRequired") }]}
                  >
                    <Input placeholder={tc("field")} />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, "operator"]}
                    rules={[{ required: true, message: tr("rule.validation.operatorRequired") }]}
                  >
                    <Input placeholder={tc("operator")} style={{ width: 100 }} />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, "value"]}
                    rules={[{ required: true, message: tr("rule.validation.valueRequired") }]}
                  >
                    <Input placeholder={tc("value")} />
                  </Form.Item>
                  <MinusCircleOutlined onClick={() => remove(name)} />
                </Space>
              ))}
              <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                {tc("addCondition")}
              </Button>
            </>
          )}
        </Form.List>
      </FormSection>

      <FormSection title={tc("validateExpressions")}>
        <Button
          icon={
            validationResult?.valid ? (
              <CheckCircleOutlined />
            ) : validationResult ? (
              <CloseCircleOutlined />
            ) : undefined
          }
          loading={validating}
          onClick={handleValidate}
        >
          {tc("validateNow")}
        </Button>
        {validationResult && (
          <Alert
            style={{ marginTop: 8 }}
            type={validationResult.valid ? "success" : "error"}
            title={validationResult.valid ? tc("validExpressions") : tc("invalidExpressions")}
            description={
              validationResult.errors?.length ? (
                <ul style={{ margin: 0, paddingLeft: 16 }}>
                  {validationResult.errors.map((e, i) => (
                    <li key={i}>{e}</li>
                  ))}
                </ul>
              ) : undefined
            }
            showIcon
          />
        )}
      </FormSection>
    </ResourceFormPage>
  );
}
