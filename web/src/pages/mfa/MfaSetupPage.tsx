import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Steps,
  Typography,
} from "antd";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import i18n from "@/shared/i18n";
import { useTranslation } from "react-i18next";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { toDialCode } from "@/shared/ui/country-data";
import { useMfaSetupPageController, type SetupFormValues } from "./useMfaSetupPageController";
import { localizedError } from "@/shared/lib/server-error";

function getTypeHint(mfaType: string) {
  const tr = i18n.getFixedT(null, "resource");
  switch (mfaType) {
    case "app":
      return tr("mfa.typeHintApp");
    case "email":
      return tr("mfa.typeHintEmail");
    case "sms":
      return tr("mfa.typeHintSms");
    case "radius":
      return tr("mfa.typeHintRadius");
    case "push":
      return tr("mfa.typeHintPush");
    default:
      return tr("mfa.typeHintDefault");
  }
}
function formatRecoveryCodes(value?: string[]) {
  if (!value || value.length === 0) return "-";
  return value.join("\n");
}

export default function MfaSetupPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const {
    owner,
    name,
    returnTo,
    form,
    initiated,
    verifiedRecoveryCodes,
    showFullSecret,
    setShowFullSecret,
    showFullInitiatedSecret,
    setShowFullInitiatedSecret,
    organizationIdentityQuery,
    userIdentityQuery,
    detailQuery,
    currentMfa,
    mfaType,
    initiateMutation,
    verifyMutation,
    enableMutation,
    MFA_TYPE_OPTIONS,
    currentStep,
    showDestField,
    showCountryCodeField,
    showSecretInfo,
    resetSetupState,
  } = useMfaSetupPageController();

  const watchedCountryCode = Form.useWatch("countryCode", form) as string | undefined;
  const dialCode = toDialCode(watchedCountryCode ?? "");

  const steps = [
    { title: t("mfaSetup.stepInitiate"), content: tr("mfa.stepInitiate") },
    { title: t("mfaSetup.stepVerify"), content: tr("mfa.stepVerify") },
    { title: t("mfaSetup.stepEnable"), content: tr("mfa.stepEnable") },
  ];

  if (!organizationIdentityQuery.isLoading && organizationIdentityQuery.data === null) {
    return <Alert type="error" showIcon title={tr("mfa.organizationNotFound")} />;
  }
  if (!userIdentityQuery.isLoading && userIdentityQuery.data === null) {
    return <Alert type="error" showIcon title={tr("mfa.userNotFound")} />;
  }
  if (detailQuery.isError) {
    return (
      <Alert
        type="error"
        showIcon
        title={localizedError(detailQuery.error, tr, "mfa.userDetailLoadFailed")}
      />
    );
  }

  return (
    <div className="gl-content__inner--form" style={{ display: "grid", gap: 16 }}>
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <div className="gl-resource-card__header">
          <div>
            <h1 style={{ marginBottom: 8 }}>{t("mfaSetup.title")}</h1>
            <p style={{ marginBottom: 0 }}>{tr("mfa.setupDescription")}</p>
          </div>
          <Space>
            {owner && name ? (
              <Link to={`/users/${owner}/${name}`}>{tr("mfa.backToUserDetail")}</Link>
            ) : null}
            {returnTo ? (
              <Button type="link" onClick={() => navigate(returnTo)}>
                {tr("mfa.backToReturnPage")}
              </Button>
            ) : null}
          </Space>
        </div>

        <Steps current={currentStep} items={steps} style={{ marginBottom: 24 }} />

        <Form
          form={form}
          layout="vertical"
          initialValues={{
            owner,
            name,
            mfaType: searchParams.get("mfaType") ?? "app",
            rememberInHours: 0,
          }}
          onValuesChange={(changedValues) => {
            if (changedValues.mfaType) {
              resetSetupState();
              const nextParams = new URLSearchParams(searchParams);
              nextParams.set("mfaType", changedValues.mfaType);
              setSearchParams(nextParams, { replace: true });
            }
          }}
        >
          <div
            style={{
              display: "grid",
              gap: 16,
              gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
            }}
          >
            <Form.Item
              label={tc("organization")}
              name="owner"
              rules={[{ required: true, message: tr("mfa.validation.enterOrganization") }]}
            >
              <Input disabled={owner !== ""} />
            </Form.Item>
            <Form.Item
              label={tc("username")}
              name="name"
              rules={[{ required: true, message: tr("mfa.validation.enterUsername") }]}
            >
              <Input disabled={name !== ""} />
            </Form.Item>
            <Form.Item
              label={tr("mfa.type")}
              name="mfaType"
              rules={[{ required: true, message: tr("mfa.validation.selectType") }]}
            >
              <Select options={MFA_TYPE_OPTIONS} />
            </Form.Item>
            <Form.Item label={tr("mfa.rememberHours")} name="rememberInHours">
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
          </div>

          <Alert showIcon type="info" style={{ marginBottom: 16 }} title={getTypeHint(mfaType)} />

          {showDestField ? (
            <Form.Item
              label={
                mfaType === "email"
                  ? tr("mfa.receiveEmail")
                  : mfaType === "sms"
                    ? tr("mfa.phoneNumber")
                    : tr("mfa.targetAccount")
              }
              name="dest"
            >
              <Input
                placeholder={tr("mfa.placeholder.fillByProvider")}
                prefix={mfaType === "sms" ? dialCode || undefined : undefined}
              />
            </Form.Item>
          ) : null}
          {showCountryCodeField ? (
            <Form.Item
              label={tr("mfa.countryCode")}
              name="countryCode"
              rules={[{ required: true, message: tc("auth:selectCountryCode") }]}
            >
              <CountryCodeSelect ariaLabel={tr("mfa.countryCode")} style={{ width: "100%" }} />
            </Form.Item>
          ) : null}

          <Form.Item label={t("mfaSetup.passcode")} name="passcode">
            <Input placeholder={tr("mfa.placeholder.passcode")} />
          </Form.Item>

          <Space wrap>
            <Button
              type="primary"
              loading={initiateMutation.isPending}
              disabled={!userIdentityQuery.data}
              onClick={() => {
                void form.validateFields(["owner", "name", "mfaType"]).then((values) => {
                  resetSetupState();
                  return initiateMutation.mutate(values);
                });
              }}
            >
              {tr("mfa.generateConfig")}
            </Button>
            <Button
              loading={verifyMutation.isPending}
              disabled={!initiated || !userIdentityQuery.data}
              onClick={() => {
                void form
                  .validateFields(["owner", "name", "mfaType", "passcode", "dest", "countryCode"])
                  .then((values) => verifyMutation.mutate(values));
              }}
            >
              {tr("mfa.verifyCode")}
            </Button>
            <Button
              type="primary"
              loading={enableMutation.isPending}
              disabled={verifiedRecoveryCodes.length === 0 || !userIdentityQuery.data}
              onClick={() => {
                void form
                  .validateFields()
                  .then((values: SetupFormValues) => enableMutation.mutate(values));
              }}
            >
              {tr("mfa.enable")}
            </Button>
          </Space>
        </Form>
      </Card>

      <Card className="gl-panel gl-resource-card" variant="borderless">
        <div className="gl-resource-card__header">
          <div>
            <h2 style={{ marginBottom: 8 }}>{tr("mfa.currentStatus")}</h2>
            <p style={{ marginBottom: 0 }}>{tr("mfa.currentStatusDesc")}</p>
          </div>
        </div>

        <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
          <Descriptions.Item label={tc("user")}>
            {owner && name ? `${owner}/${name}` : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={tr("mfa.currentType")}>{mfaType}</Descriptions.Item>
          <Descriptions.Item label={tc("enabled")}>
            {currentMfa?.enabled ? tc("yes") : tc("no")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("mfa.preferred")}>
            {currentMfa?.isPreferred ? tc("yes") : tc("no")}
          </Descriptions.Item>
          <Descriptions.Item label={tr("mfa.currentSecret")}>
            {currentMfa?.secret ? (
              <Space>
                <Typography.Text code>
                  {showFullSecret
                    ? currentMfa.secret
                    : `${currentMfa.secret.slice(0, 4)}${"*".repeat(8)}`}
                </Typography.Text>
                <Button type="link" size="small" onClick={() => setShowFullSecret((v) => !v)}>
                  {showFullSecret ? tr("mfa.hideSecret", "Hide") : tr("mfa.showSecret", "Show")}
                </Button>
              </Space>
            ) : (
              "-"
            )}
          </Descriptions.Item>
          <Descriptions.Item label={tr("mfa.currentProviderUrl")}>
            {currentMfa?.url || "-"}
          </Descriptions.Item>
        </Descriptions>

        {showSecretInfo && (
          <div style={{ display: "grid", gap: 16, marginTop: 16 }}>
            <div>
              <Space align="center" style={{ marginBottom: 4 }}>
                <Typography.Text strong>{tr("mfa.initiatedSecret", "Secret")}</Typography.Text>
                <Button
                  type="link"
                  size="small"
                  onClick={() => setShowFullInitiatedSecret((v) => !v)}
                >
                  {showFullInitiatedSecret
                    ? tr("mfa.hideSecret", "Hide")
                    : tr("mfa.showSecret", "Show")}
                </Button>
              </Space>
              <Input.TextArea
                rows={3}
                readOnly
                value={
                  showFullInitiatedSecret
                    ? initiated?.secret || ""
                    : initiated?.secret
                      ? `${initiated.secret.slice(0, 4)}${"*".repeat(8)}`
                      : ""
                }
                placeholder={tr("mfa.placeholder.initiatedSecret")}
              />
            </div>
            <Input.TextArea
              rows={3}
              readOnly
              value={initiated?.url || ""}
              placeholder={tr("mfa.placeholder.initiatedUrl")}
            />
          </div>
        )}

        {verifiedRecoveryCodes.length > 0 ? (
          <Alert
            style={{ marginTop: 16 }}
            type="success"
            showIcon
            title={tr("mfa.verifySuccessEnableReady")}
            description={
              <Input.TextArea
                rows={6}
                readOnly
                value={formatRecoveryCodes(verifiedRecoveryCodes)}
              />
            }
          />
        ) : initiated?.recoveryCodes && initiated.recoveryCodes.length > 0 ? (
          <Alert
            style={{ marginTop: 16 }}
            type="info"
            showIcon
            title={tr("mfa.initRecoveryCodes")}
            description={
              <Input.TextArea
                rows={6}
                readOnly
                value={formatRecoveryCodes(initiated.recoveryCodes)}
              />
            }
          />
        ) : null}

        {detailQuery.isFetching && (
          <Alert
            style={{ marginTop: 16 }}
            type="info"
            showIcon
            title="Refreshing user MFA status…"
          />
        )}
      </Card>
    </div>
  );
}
