import { LinkOutlined } from "@ant-design/icons";
import { Alert, Button, Card, Cascader, Input, Select, Space, Tag, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { UserDetailItem } from "@/shared/types/domain";
import type { PromptPageReadyState } from "./usePromptPageController";

function getLinkedValue(user: UserDetailItem, providerType?: string | null) {
  if (!providerType) {
    return "";
  }

  const key = providerType.toLowerCase() as keyof UserDetailItem;
  const value = user[key];
  return typeof value === "string" ? value : "";
}

function getStringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

export function PromptPageCard(props: PromptPageReadyState) {
  const {
    addressOptions,
    addressOptionsLoading,
    affiliationOptions,
    affiliationOptionsLoading,
    application,
    canSubmit,
    draftUser,
    handleLinkProvider,
    handleSubmit,
    hasAffiliationOptionsError,
    isLinkingProvider,
    isSubmitting,
    promptedProviderItems,
    shouldPromptAffiliation,
    shouldPromptRegion,
    shouldUseStructuredAffiliationInputs,
    updateDraftUser,
  } = props;

  const { t } = useTranslation("auth");

  const addressCode = Array.isArray(draftUser.address)
    ? draftUser.address[draftUser.address.length - 1]
    : "";

  return (
    <div className="login-content" style={{ margin: "auto" }}>
      <div style={{ marginBottom: 10, textAlign: "center" }}>
        <Card
          style={{
            width: 560,
            maxWidth: "calc(100vw - 32px)",
            margin: "72px auto",
            borderRadius: 16,
          }}
          styles={{ body: { padding: 32 } }}
        >
          <div style={{ textAlign: "center", marginBottom: 24 }}>
            <Typography.Title level={3} style={{ marginBottom: 8 }}>
              {t("promptTitle")}
            </Typography.Title>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {application.displayName || application.name} {t("promptDesc")}
            </Typography.Paragraph>
          </div>

          <Space orientation="vertical" size={20} style={{ width: "100%" }}>
            {shouldPromptAffiliation ? (
              <Card size="small" title={t("prompt.affiliation")}>
                {hasAffiliationOptionsError ? (
                  <Alert
                    style={{ marginBottom: 12 }}
                    type="warning"
                    showIcon
                    title={t("affiliationOptionsLoadFailed")}
                    description={t("affiliationOptionsLoadFailedDesc")}
                  />
                ) : null}

                {shouldUseStructuredAffiliationInputs ? (
                  <Space orientation="vertical" size={12} style={{ width: "100%" }}>
                    <div>
                      <Typography.Text>{t("prompt.address")}</Typography.Text>
                      <Cascader
                        style={{ width: "100%", marginTop: 8 }}
                        options={addressOptions}
                        disabled={addressOptionsLoading}
                        value={Array.isArray(draftUser.address) ? draftUser.address : []}
                        onChange={(value) => {
                          updateDraftUser({
                            address: value as UserDetailItem["address"],
                            affiliation: "",
                            score: 0,
                          });
                        }}
                        placeholder={t("selectAddress")}
                      />
                    </div>
                    <div>
                      <Typography.Text>{t("prompt.affiliation")}</Typography.Text>
                      <Select
                        style={{ width: "100%", marginTop: 8 }}
                        loading={affiliationOptionsLoading}
                        disabled={!addressCode}
                        value={draftUser.affiliation || undefined}
                        placeholder={t("selectAffiliation")}
                        options={affiliationOptions.map((item) => ({
                          label: item.name || "",
                          value: item.name || "",
                        }))}
                        onChange={(value) => {
                          const matched = affiliationOptions.find((item) => item.name === value);
                          updateDraftUser({ affiliation: value, score: matched?.id ?? 0 });
                        }}
                      />
                    </div>
                  </Space>
                ) : (
                  <Input
                    value={getStringValue(draftUser.affiliation)}
                    placeholder={t("enterAffiliation")}
                    onChange={(event) => updateDraftUser({ affiliation: event.target.value })}
                  />
                )}
              </Card>
            ) : null}

            {shouldPromptRegion ? (
              <Card size="small" title={t("prompt.countryRegion")}>
                <Input
                  value={getStringValue(draftUser.region)}
                  placeholder={t("enterRegionCode")}
                  onChange={(event) =>
                    updateDraftUser({ region: event.target.value.toUpperCase() })
                  }
                />
              </Card>
            ) : null}

            {promptedProviderItems.length > 0 ? (
              <Card size="small" title={t("prompt.bindingProviders")}>
                <Space orientation="vertical" size={12} style={{ width: "100%" }}>
                  {promptedProviderItems.map((providerItem) => {
                    const provider = providerItem.provider;
                    const linkedValue = getLinkedValue(draftUser, provider?.type);
                    const isLinked = linkedValue !== "";
                    const providerName =
                      provider?.displayName || provider?.name || provider?.type || "Provider";

                    return (
                      <div
                        key={provider?.name || providerName}
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          gap: 12,
                          alignItems: "center",
                          flexWrap: "wrap",
                        }}
                      >
                        <div>
                          <Typography.Text strong>{providerName}</Typography.Text>
                          <div style={{ marginTop: 4 }}>
                            {isLinked ? (
                              <Tag color="success">{t("linked")}</Tag>
                            ) : (
                              <Tag color="warning">{t("notLinked")}</Tag>
                            )}
                            {isLinked ? (
                              <Typography.Text type="secondary">{linkedValue}</Typography.Text>
                            ) : null}
                          </div>
                        </div>
                        {isLinked ? null : (
                          <Button
                            type="primary"
                            icon={<LinkOutlined />}
                            loading={isLinkingProvider === (provider?.name || providerName)}
                            onClick={() => {
                              void handleLinkProvider(providerItem);
                            }}
                          >
                            {t("bind")}
                          </Button>
                        )}
                      </div>
                    );
                  })}
                </Space>
                <Alert style={{ marginTop: 16 }} type="info" showIcon title={t("bindInfo")} />
              </Card>
            ) : null}

            <div style={{ display: "flex", justifyContent: "center", paddingTop: 8 }}>
              <Button
                type="primary"
                size="large"
                disabled={!canSubmit}
                loading={isSubmitting}
                onClick={() => {
                  void handleSubmit();
                }}
              >
                {t("submitAndContinue")}
              </Button>
            </div>
          </Space>
        </Card>
      </div>
    </div>
  );
}
