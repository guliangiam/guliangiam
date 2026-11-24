import { Card, Result, Descriptions, Button, Skeleton } from "antd";
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getIamPayment } from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import { StateTag } from "@/shared/ui/StateTag";

const STATUS_ICONS: Record<string, React.ReactNode> = {
  succeeded: <CheckCircleOutlined />,
  created: <ClockCircleOutlined />,
  processing: <ClockCircleOutlined />,
};

export default function PaymentResultPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  const query = useQuery({
    queryKey: queryKeys.payments.result(id!),
    queryFn: () => getIamPayment(id!),
    enabled: !!id,
  });

  if (query.isLoading) {
    return (
      <Card>
        <Skeleton active paragraph={{ rows: 4 }} />
      </Card>
    );
  }
  if (!query.data) {
    return (
      <Card>
        <Result status="error" title={t("payment.result.notFound")} />
      </Card>
    );
  }

  const payment = query.data;
  // 服务端枚举为小写（PAYMENT_STATES）：succeeded=成功，created/processing=待决。
  const isSuccess = payment.state === "succeeded";
  const isPending = payment.state === "created" || payment.state === "processing";
  const resultStatus = isSuccess ? "success" : isPending ? "info" : "error";
  const resultIcon = STATUS_ICONS[payment.state] || <CloseCircleOutlined />;
  const resultTitle = isSuccess
    ? t("payment.result.success")
    : isPending
      ? t("payment.result.pending")
      : t("payment.result.failed");
  const resultSubTitle = isSuccess
    ? t("payment.result.successDesc")
    : isPending
      ? t("payment.result.pendingDesc")
      : t("payment.result.failedDesc");

  return (
    <div className="gl-content__inner--reading">
      <Card>
        <Result
          status={resultStatus}
          icon={resultIcon}
          title={resultTitle}
          subTitle={resultSubTitle}
        />
        <Descriptions column={1} bordered size="small" style={{ marginTop: 16 }}>
          <Descriptions.Item label={t("payment.result.product")}>
            {payment.productName}
          </Descriptions.Item>
          <Descriptions.Item label={t("payment.result.amount")}>
            {payment.currency} {payment.price}
          </Descriptions.Item>
          <Descriptions.Item label={t("payment.result.provider")}>
            {payment.providerName}
          </Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={payment.state} />
          </Descriptions.Item>
        </Descriptions>
        <div style={{ marginTop: 24, textAlign: "center" }}>
          <Button type="primary" onClick={() => navigate("/store")}>
            {t("payment.result.backToStore")}
          </Button>
        </div>
      </Card>
    </div>
  );
}
