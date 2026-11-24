import { Card, Col, Row, Typography, Empty, Skeleton } from "antd";
import { AppstoreOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { getApplicationOptions, type ApplicationOption } from "@/shared/api/iam";

const { Title, Text } = Typography;

type AppCard = {
  id: string;
  name: string;
  displayName: string;
  logo?: string;
  homepageUrl?: string;
};

type ApplicationOptionWithExtras = ApplicationOption & {
  logo?: string;
  homepageUrl?: string;
};

export default function AppsPage() {
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();

  const { data: apps, isLoading } = useQuery({
    queryKey: ["apps", "quick-launch"],
    queryFn: async (): Promise<AppCard[]> => {
      const options = await getApplicationOptions();
      return options.map((o) => {
        const ext = o as ApplicationOptionWithExtras;
        return {
          id: o.id,
          name: o.slug,
          displayName: o.displayName || o.slug,
          logo: ext.logo,
          homepageUrl: ext.homepageUrl,
        };
      });
    },
  });

  if (isLoading) {
    return (
      <div className="gl-empty-state" style={{ padding: 48 }}>
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }

  if (!apps || apps.length === 0) {
    return (
      <Card>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tc("apps.noApplications")} />
      </Card>
    );
  }

  return (
    <div>
      <Title level={4} style={{ marginBottom: 24 }}>
        <AppstoreOutlined style={{ marginRight: 8 }} />
        {tc("apps.title")}
      </Title>
      <Row gutter={[16, 16]}>
        {apps.map((app) => (
          <Col key={app.name} xs={12} sm={8} md={6} lg={4}>
            <Card
              hoverable
              className="gl-app-card"
              variant="borderless"
              tabIndex={0}
              aria-label={app.displayName}
              onClick={() => {
                if (app.id) {
                  navigate(`/applications/${app.id}`);
                }
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && app.id) {
                  navigate(`/applications/${app.id}`);
                }
              }}
              style={{
                textAlign: "center",
                height: "100%",
                display: "flex",
                flexDirection: "column",
                justifyContent: "center",
              }}
            >
              <div
                style={{
                  width: 48,
                  height: 48,
                  margin: "0 auto 12px",
                  borderRadius: 12,
                  background: "var(--gl-color-primary-bg)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 24,
                  color: "var(--gl-color-primary)",
                }}
              >
                {app.logo ? (
                  <img
                    src={app.logo}
                    alt={app.displayName}
                    style={{ width: 32, height: 32, borderRadius: 8, objectFit: "cover" }}
                  />
                ) : (
                  <AppstoreOutlined />
                )}
              </div>
              <Text strong ellipsis style={{ display: "block" }}>
                {app.displayName}
              </Text>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
}
