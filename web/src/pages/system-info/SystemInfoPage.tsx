import { Card, Col, Descriptions, Row, Skeleton, Statistic, Tag, Tooltip } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  CodeOutlined,
  DatabaseOutlined,
  HeartOutlined,
  InfoCircleOutlined,
  ReloadOutlined,
  RocketOutlined,
} from "@ant-design/icons";
import { getSystemVersion, getSystemInfo, getSystemHealth } from "@/shared/api/iam/system";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`;
}

export default function SystemInfoPage() {
  const { t } = useTranslation("common");

  const versionQuery = useQuery({
    queryKey: ["system-version"],
    queryFn: () => getSystemVersion(),
    staleTime: 300_000,
  });

  const infoQuery = useQuery({
    queryKey: ["system-info"],
    queryFn: () => getSystemInfo(),
    staleTime: 10_000,
    refetchInterval: 30_000,
  });

  const healthQuery = useQuery({
    queryKey: ["system-health"],
    queryFn: () => getSystemHealth(),
    staleTime: 10_000,
    refetchInterval: 30_000,
  });

  const version = versionQuery.data;
  const info = infoQuery.data;
  const health = healthQuery.data;
  const isLoading = versionQuery.isLoading || infoQuery.isLoading || healthQuery.isLoading;

  if (isLoading) {
    return (
      <div className="gl-content__inner--reading">
        <div className="gl-empty-state">
          <Skeleton active paragraph={{ rows: 10 }} />
        </div>
      </div>
    );
  }

  return (
    <div className="gl-content__inner--reading">
      <Row gutter={[16, 16]}>
        {/* Version Card */}
        <Col span={24}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={
              <span>
                <RocketOutlined style={{ marginRight: 8 }} />
                {t("systemInfo.version")}
              </span>
            }
            extra={
              <Tooltip title={t("systemInfo.autoRefresh")}>
                <ReloadOutlined
                  spin={infoQuery.isFetching || healthQuery.isFetching}
                  style={{ color: "var(--ant-color-text-secondary)" }}
                />
              </Tooltip>
            }
          >
            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} bordered size="small">
              <Descriptions.Item label={t("systemInfo.versionLabel")}>
                <Tag color="blue">{version?.version ?? "—"}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t("systemInfo.gitCommit")}>
                <code style={{ fontSize: 12 }}>{version?.commitHash ?? "—"}</code>
              </Descriptions.Item>
              <Descriptions.Item label={t("systemInfo.buildDate")}>
                {version?.buildTime ?? "—"}
              </Descriptions.Item>
              <Descriptions.Item label={t("systemInfo.goVersion")}>
                {version?.goVersion ?? info?.goVersion ?? "—"}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>

        {/* Health Card */}
        <Col span={24}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={
              <span>
                <HeartOutlined style={{ marginRight: 8 }} />
                {t("systemInfo.health")}
              </span>
            }
          >
            <Descriptions column={{ xs: 1, sm: 2, md: 3 }} bordered size="small">
              <Descriptions.Item label={t("systemInfo.healthStatus")}>
                <Tag color={health?.status === "ok" ? "green" : "red"}>{health?.status ?? "—"}</Tag>
              </Descriptions.Item>
              {health?.uptime && (
                <Descriptions.Item label={t("systemInfo.uptime")}>
                  {health.uptime}
                </Descriptions.Item>
              )}
              {health?.database && (
                <Descriptions.Item label={t("systemInfo.database")}>
                  <Tag color={health.database === "ok" ? "green" : "red"}>{health.database}</Tag>
                </Descriptions.Item>
              )}
            </Descriptions>
          </Card>
        </Col>

        {/* Runtime Card */}
        <Col xs={24} md={12}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={
              <span>
                <InfoCircleOutlined style={{ marginRight: 8 }} />
                {t("systemInfo.runtime")}
              </span>
            }
          >
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.numCPU")}
                  value={info?.numCPU ?? "—"}
                  prefix={<InfoCircleOutlined />}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.numGoroutine")}
                  value={info?.numGoroutine ?? "—"}
                  prefix={<CodeOutlined />}
                />
              </Col>
            </Row>
          </Card>
        </Col>

        {/* Memory Card */}
        <Col xs={24} md={12}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={
              <span>
                <DatabaseOutlined style={{ marginRight: 8 }} />
                {t("systemInfo.memory")}
              </span>
            }
          >
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.heapAlloc")}
                  value={info ? formatBytes(info.heapAlloc) : "—"}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.sysMemory")}
                  value={info ? formatBytes(info.heapSys) : "—"}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.stackInuse", "Stack in use")}
                  value={info ? formatBytes(info.stackInuse) : "—"}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title={t("systemInfo.platform", "Platform")}
                  value={info ? `${info.os}/${info.arch}` : "—"}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
