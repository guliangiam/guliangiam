import { Card, Row, Col, Tag, Typography, Empty, Skeleton } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { LinkButton } from "@/shared/ui/LinkButton";
import { getIamProductStore } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";

const { Title, Text } = Typography;

export default function ProductStorePage() {
  const { t } = useTranslation("resource");
  const query = useQuery({
    queryKey: queryKeys.products.store(),
    queryFn: () => getIamProductStore(),
  });

  const products = query.data ?? [];

  return (
    <Card>
      <Title level={4}>{t("product.store.title")}</Title>
      {query.isLoading && <Skeleton active paragraph={{ rows: 4 }} />}
      {query.isError && <Empty description={t("product.store.loadError")} />}
      {!query.isLoading && products.length === 0 && (
        <Empty description={t("product.store.noProducts")} />
      )}
      <Row gutter={[16, 16]}>
        {products.map((product) => (
          <Col key={product.id} xs={24} sm={12} md={8} lg={6}>
            {/* WCAG 2.1.1 Keyboard — remove hoverable since interaction is in actions area (C-04) */}
            <Card
              cover={
                product.image ? (
                  <img
                    alt={product.displayName}
                    src={product.image}
                    style={{ height: 200, objectFit: "cover" }}
                  />
                ) : (
                  <div
                    style={{
                      height: 200,
                      background: "var(--ant-color-fill-tertiary, rgba(0,0,0,0.04))",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    }}
                  >
                    <Text>{product.displayName}</Text>
                  </div>
                )
              }
              actions={[
                <LinkButton key="buy" type="primary" to={`/products/${product.id}/buy`}>
                  {t("product.store.buyNow")}
                </LinkButton>,
              ]}
            >
              <Card.Meta
                title={product.displayName}
                description={
                  <>
                    <Text strong>
                      {product.currency} {product.price}
                    </Text>
                    {product.tag && <Tag style={{ marginLeft: 8 }}>{product.tag}</Tag>}
                    <div style={{ marginTop: 8 }}>
                      <Text type="secondary" ellipsis>
                        {product.description}
                      </Text>
                    </div>
                  </>
                }
              />
            </Card>
          </Col>
        ))}
      </Row>
    </Card>
  );
}
