import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Card, Table, Button, Steps, Tag, Space, Alert, Skeleton, Typography, App } from "antd";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  getIamLdapPreviewUsers,
  syncIamLdap,
  testIamLdapConnection,
  type IamLdapPreviewUser,
  type IamLdapSyncResult,
} from "@/shared/api/iam";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { serverErrorMessage } from "@/shared/lib/server-error";

const { Title, Text } = Typography;

export default function LdapSyncPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();

  const [currentStep, setCurrentStep] = useState(0);
  const [selectedUUIDs, setSelectedUUIDs] = useState<string[]>([]);
  const [syncResult, setSyncResult] = useState<IamLdapSyncResult | null>(null);

  // Step 0: Preview users
  const previewQuery = useQuery({
    queryKey: ["ldap", id, "preview-users"],
    queryFn: () => getIamLdapPreviewUsers(id!),
    enabled: !!id && currentStep === 0,
  });

  // Test connection mutation
  const testConnectionMutation = useMutation({
    mutationFn: () => testIamLdapConnection(id!),
  });

  // Sync mutation
  const syncMutation = useMutation({
    mutationFn: (uuids: string[]) => syncIamLdap(id!, { uuids }),
    onSuccess: (data) => {
      setSyncResult(data as unknown as IamLdapSyncResult);
      setCurrentStep(2);
    },
    onError: (error) => {
      message.error(serverErrorMessage(error, tc, "common:operationFailed"));
    },
  });

  const previewData = previewQuery.data;
  const users = previewData?.users ?? [];
  const newUsers = users.filter((u) => !u.existsLocal);

  const handleSelectNewUsers = () => {
    setSelectedUUIDs(newUsers.map((u) => u.uuid));
  };

  const handleSelectAll = () => {
    setSelectedUUIDs(users.map((u) => u.uuid));
  };

  const handleClearSelection = () => {
    setSelectedUUIDs([]);
  };

  const handleSync = () => {
    if (selectedUUIDs.length === 0) return;
    syncMutation.mutate(selectedUUIDs);
  };

  const columns = [
    {
      title: "UID",
      dataIndex: "uid",
      key: "uid",
      width: 120,
    },
    {
      title: t("ldap.sync.cn"),
      dataIndex: "cn",
      key: "cn",
      width: 150,
    },
    {
      title: tc("email"),
      dataIndex: "email",
      key: "email",
      width: 200,
    },
    {
      title: tc("auth:phone"),
      dataIndex: "phone",
      key: "phone",
      width: 140,
    },
    {
      title: t("ldap.sync.department"),
      dataIndex: "department",
      key: "department",
      width: 120,
    },
    {
      title: tc("statusLabel"),
      dataIndex: "existsLocal",
      key: "existsLocal",
      width: 100,
      render: (v: boolean) =>
        v ? (
          <Tag color="blue">{t("ldap.sync.existing")}</Tag>
        ) : (
          <Tag color="green">{t("ldap.sync.newUser")}</Tag>
        ),
    },
  ];

  return (
    <Card>
      <Title level={4}>{t("ldap.sync.title")}</Title>

      <Steps
        current={currentStep}
        onChange={(step) => {
          // 只允许回退或进入已具备数据的步骤，避免直跳空白结果页。
          if (step <= currentStep) setCurrentStep(step);
          else if (step === 1 && previewData) setCurrentStep(step);
          else if (step === 2 && syncResult) setCurrentStep(step);
        }}
        style={{ marginBottom: 24 }}
        items={[
          { title: t("ldap.sync.stepPreview") },
          { title: t("ldap.sync.stepSelect") },
          { title: t("ldap.sync.stepResult") },
        ]}
      />

      {/* Step 0: Preview */}
      {currentStep === 0 && (
        <Space orientation="vertical" style={{ width: "100%" }} size="large">
          <Space>
            <PermissionGuard action="ldaps:write" resource="ldaps" fallback={null}>
              <Button
                onClick={() => testConnectionMutation.mutate()}
                loading={testConnectionMutation.isPending}
              >
                {t("ldap.sync.testConnection")}
              </Button>
            </PermissionGuard>
            {testConnectionMutation.isSuccess && (
              <Tag color="green">{t("ldap.sync.connectionOk")}</Tag>
            )}
            {testConnectionMutation.isError && (
              <Alert
                type="error"
                showIcon
                title={t("ldap.sync.connectionFailed")}
                description={serverErrorMessage(
                  testConnectionMutation.error,
                  tc,
                  "common:operationFailed",
                )}
              />
            )}
          </Space>

          {previewQuery.isLoading && <Skeleton active paragraph={{ rows: 6 }} />}
          {previewQuery.isError && (
            <Alert
              type="error"
              title={serverErrorMessage(previewQuery.error, tc, "common:operationFailed")}
            />
          )}

          {previewData && (
            <>
              <Space>
                <Text>{t("ldap.sync.totalCount", { count: previewData.totalCount })}</Text>
                <Text>
                  {t("ldap.sync.newCount", {
                    count: previewData.totalCount - previewData.existCount,
                  })}
                </Text>
                <Text>{t("ldap.sync.existCount", { count: previewData.existCount })}</Text>
              </Space>
              <Table
                dataSource={users}
                columns={columns}
                rowKey="uuid"
                size="small"
                scroll={{ x: 830 }}
                pagination={{ pageSize: 20 }}
              />
              <Button type="primary" onClick={() => setCurrentStep(1)}>
                {t("ldap.sync.nextToSelect")}
              </Button>
            </>
          )}
        </Space>
      )}

      {/* Step 1: Select & Sync */}
      {currentStep === 1 && (
        <Space orientation="vertical" style={{ width: "100%" }} size="large">
          <Space>
            <Button size="small" onClick={handleSelectNewUsers}>
              {t("ldap.sync.selectNew")}
            </Button>
            <Button size="small" onClick={handleSelectAll}>
              {t("ldap.sync.selectAll")}
            </Button>
            <Button size="small" onClick={handleClearSelection}>
              {t("ldap.sync.clearSelection")}
            </Button>
            <Text>{t("ldap.sync.selectedCount", { count: selectedUUIDs.length })}</Text>
          </Space>

          <Table
            dataSource={users}
            columns={columns}
            rowKey="uuid"
            size="small"
            scroll={{ x: 890 }}
            pagination={{ pageSize: 20 }}
            rowSelection={{
              selectedRowKeys: selectedUUIDs,
              onChange: (keys) => setSelectedUUIDs(keys as string[]),
              getCheckboxProps: (record) => ({
                disabled: (record as IamLdapPreviewUser).existsLocal,
              }),
            }}
          />

          <Space>
            <Button onClick={() => setCurrentStep(0)}>{tc("auth:back")}</Button>
            <PermissionGuard action="ldaps:write" resource="ldaps" fallback={null}>
              <Button
                type="primary"
                onClick={handleSync}
                loading={syncMutation.isPending}
                disabled={selectedUUIDs.length === 0}
              >
                {t("ldap.sync.startSync")}
              </Button>
            </PermissionGuard>
          </Space>
        </Space>
      )}

      {/* Step 2: Result */}
      {currentStep === 2 && syncResult && (
        <Space orientation="vertical" style={{ width: "100%" }} size="large">
          <Alert
            type={syncResult.failedUsers > 0 ? "warning" : "success"}
            title={
              syncResult.failedUsers > 0 ? t("ldap.sync.syncPartial") : t("ldap.sync.syncSuccess")
            }
            showIcon
          />

          <Card title={t("ldap.sync.resultSummary")} size="small">
            <Space orientation="vertical">
              <Text>{t("ldap.sync.resultTotal", { count: syncResult.totalUsers })}</Text>
              <Text>
                <Tag color="green">
                  {t("ldap.sync.resultAdded", { count: syncResult.addedUsers })}
                </Tag>
                <Tag color="blue">
                  {t("ldap.sync.resultUpdated", { count: syncResult.updatedUsers })}
                </Tag>
                <Tag color="red">
                  {t("ldap.sync.resultFailed", { count: syncResult.failedUsers })}
                </Tag>
              </Text>
              {syncResult.totalGroups > 0 && (
                <Text>
                  {t("ldap.sync.resultGroups", {
                    added: syncResult.addedGroups,
                    failed: syncResult.failedGroups,
                  })}
                </Text>
              )}
            </Space>
          </Card>

          {syncResult.errors && syncResult.errors.length > 0 && (
            <Card title={t("ldap.sync.errors")} size="small">
              {syncResult.errors.map((err, i) => (
                <Text key={i} type="danger" style={{ display: "block" }}>
                  {err}
                </Text>
              ))}
            </Card>
          )}

          <Button type="primary" onClick={() => navigate("/ldaps")}>
            {tc("auth:back")}
          </Button>
        </Space>
      )}
    </Card>
  );
}
