import { useState } from "react";
import { Button, Card, Radio, Row, Col, Typography, Skeleton, Alert, Tag, App } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { getIamPricing, getIamPricings, getIamPlan, placeOrder } from "@/shared/api/iam";
import { useSessionStore } from "@/features/session/session-store";
import { ResourceGate } from "@/app/layout/ResourceGate";
import { localizedError, serverErrorMessage } from "@/shared/lib/server-error";

const { Title, Text, Paragraph } = Typography;

type PlanWithDetails = {
  id: string;
  name: string;
  displayName: string;
  period: string;
  price: number;
  currency: string;
  description: string;
  isExclusive: boolean;
  /** 计划指向的真实商品 ID；下单目录解析只认 products 表（BE-C-13）。 */
  product?: string;
};

export default function PricingSelectPage() {
  return (
    <ResourceGate>
      <PricingSelectContent />
    </ResourceGate>
  );
}

function PricingSelectContent() {
  const { orgId, pricingName } = useParams<{ orgId: string; pricingName: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("resource");
  const { t: _tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const [selectedPeriod, setSelectedPeriod] = useState<string | null>(null);
  // 防重入：下单进行中的计划 id；非空时所有购买按钮禁用，避免连点重复下单。
  const [orderingPlanId, setOrderingPlanId] = useState<string | null>(null);

  // First get the pricing by listing with orgId + keyword
  const pricingQuery = useQuery({
    queryKey: ["public-pricing", orgId, pricingName],
    queryFn: async () => {
      // Try to find by name via list API with keyword filter
      const result = await getIamPricings({
        organizationId: orgId,
        search: pricingName,
        pageSize: 10,
      });
      const found = result.find((p) => p.name === pricingName);
      if (!found && result.length === 1) {
        return getIamPricing(result[0].id);
      }
      if (found) {
        return getIamPricing(found.id);
      }
      throw new Error(t("pricing.select.notFound"));
    },
    enabled: !!orgId && !!pricingName,
  });

  const pricing = pricingQuery.data;

  // Load all plans for this pricing
  const plansQuery = useQuery({
    queryKey: ["public-plans", pricing?.id],
    queryFn: async () => {
      if (!pricing?.plans?.length) return [];
      const planPromises = pricing.plans.map((planId: string) => getIamPlan(planId));
      const results = await Promise.all(planPromises);
      return results as PlanWithDetails[];
    },
    enabled: !!pricing?.plans?.length,
  });

  const plans = plansQuery.data ?? [];
  const periods = [...new Set(plans.map((p) => p.period).filter(Boolean))];
  const effectivePeriod = selectedPeriod ?? periods[0] ?? "";
  const filteredPlans = effectivePeriod ? plans.filter((p) => p.period === effectivePeriod) : plans;

  if (pricingQuery.isLoading || plansQuery.isLoading) {
    return (
      <div className="gl-content__inner--reading" style={{ padding: 48, textAlign: "center" }}>
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }

  if (pricingQuery.error || !pricing) {
    return (
      <div className="gl-content__inner--reading" style={{ padding: 48 }}>
        <Alert
          type="error"
          showIcon
          title={t("pricing.select.notFound")}
          description={t("pricing.select.notFoundDesc")}
        />
      </div>
    );
  }

  // 计划加载失败（例如某个计划已被删除导致整批请求失败）必须显式报错，
  // 而不是渲染 0 张卡片让用户误以为该定价没有计划。
  if (plansQuery.isError) {
    return (
      <div className="gl-content__inner--reading" style={{ padding: 48 }}>
        <Alert
          type="error"
          showIcon
          title={t("pricing.select.plansLoadFailed", "Failed to load plans")}
          description={serverErrorMessage(plansQuery.error, _tc, "loadFailed")}
          action={
            <Button size="small" onClick={() => plansQuery.refetch()}>
              {_tc("retry")}
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <div className="gl-content__inner--reading" style={{ padding: "48px 24px" }}>
      <div style={{ textAlign: "center", marginBottom: 40 }}>
        <Title level={2}>{pricing.displayName}</Title>
        {pricing.description && (
          <Paragraph type="secondary" style={{ fontSize: 16 }}>
            {pricing.description}
          </Paragraph>
        )}
        {pricing.trialDuration > 0 && (
          <Text type="secondary" italic>
            {t("pricing.select.freeTrial")} {pricing.trialDuration}-{t("pricing.select.daysTrial")}
          </Text>
        )}
      </div>

      {periods.length > 1 && (
        <div style={{ textAlign: "center", marginBottom: 32 }}>
          <Radio.Group
            value={effectivePeriod}
            size="large"
            buttonStyle="solid"
            onChange={(e) => setSelectedPeriod(e.target.value)}
          >
            {periods.map((period) => (
              <Radio.Button key={period} value={period}>
                {period}
              </Radio.Button>
            ))}
          </Radio.Group>
        </div>
      )}

      <Row gutter={[24, 24]} justify="center">
        {filteredPlans.map((plan) => (
          <Col key={plan.id} xs={24} sm={12} md={8}>
            <Card
              variant="borderless"
              className="gl-panel"
              style={{ textAlign: "center", height: "100%" }}
            >
              <Title level={4}>{plan.displayName}</Title>
              {plan.description && <Paragraph type="secondary">{plan.description}</Paragraph>}
              <div style={{ margin: "24px 0" }}>
                <Text style={{ fontSize: 36, fontWeight: 700 }}>
                  {plan.currency} {plan.price}
                </Text>
                {plan.period && <Text type="secondary"> / {plan.period}</Text>}
              </div>
              {plan.isExclusive && (
                <Tag color="gold" style={{ marginBottom: 16 }}>
                  Exclusive
                </Tag>
              )}
              <Button
                type="primary"
                size="large"
                block
                loading={orderingPlanId === plan.id}
                disabled={orderingPlanId !== null}
                onClick={async () => {
                  if (orderingPlanId !== null) return;
                  if (!account) {
                    navigate(
                      `/signup/${pricing.application}?plan=${plan.name}&pricing=${pricing.name}`,
                    );
                    return;
                  }
                  // BE-C-13: 后端 placeOrder 的目录解析只查 products 表，
                  // plan.id/pricing.id 不是商品 ID，传它们必然 404/422。
                  // 必须传 plan.product 指向的真实商品 ID。
                  if (!plan.product) {
                    message.error(t("pricing.select.orderFailed"));
                    return;
                  }
                  setOrderingPlanId(plan.id);
                  try {
                    const result = await placeOrder({
                      organizationId: orgId,
                      productId: plan.product,
                    });
                    if (result.payUrl) {
                      window.location.href = result.payUrl;
                    } else {
                      navigate(`/orders/${result.id}`);
                    }
                  } catch (err) {
                    message.error(localizedError(err, t, "pricing.select.orderFailed"));
                  } finally {
                    setOrderingPlanId(null);
                  }
                }}
              >
                {t("pricing.select.buyNow")}
              </Button>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
}
