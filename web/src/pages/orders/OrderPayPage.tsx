import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Card, Descriptions, Space, Spin, Typography } from "antd";
import { CheckCircleOutlined, LoadingOutlined } from "@ant-design/icons";
import { QRCodeSVG } from "qrcode.react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getIamOrder, getIamOrderIfChanged, type IamOrderDetail } from "@/shared/api/iam";
import { isHttpError } from "@/shared/api/http";
import { queryKeys } from "@/shared/api/query-keys";
import { StateTag } from "@/shared/ui/StateTag";
import { localizedError } from "@/shared/lib/server-error";

const { Title, Text } = Typography;

/**
 * What the page is doing about new server state.
 *
 * A buyer watching a QR code has exactly three honest answers — an update is in
 * flight, the page is waiting for its next scheduled read, or it stopped — and
 * the reason it stopped has to be visible, because "nothing is happening" and
 * "we gave up" look identical in a spinner.
 */
type PollStatus = "checking" | "waiting" | "paused" | "stopped";

const BASE_BACKOFF_SECONDS = 2;
const MAX_BACKOFF_SECONDS = 60;

export default function OrderPayPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const queryClient = useQueryClient();

  const [pollStatus, setPollStatus] = useState<PollStatus>("waiting");
  const [failureCount, setFailureCount] = useState(0);
  /** Response validator for the conditional read (§42.3.11 item 1). */
  const etagRef = useRef<string | undefined>(undefined);

  const orderId = id ?? "";
  const detailKey = queryKeys.orders.detail(orderId);

  const query = useQuery({
    queryKey: detailKey,
    queryFn: () => getIamOrder(orderId),
    enabled: !!orderId,
  });

  const order = query.data;

  /**
   * Whether the server still expects this order to move.
   *
   * `awaitingPayment` and `terminal` are the server's own reading of the state
   * registry, so the page never decides for itself whether the order "should"
   * change. Inferring that locally is what produced the previous build's
   * `state === "Pending"` comparisons: values the server has never sent, which
   * silently disabled both the QR code and the success confirmation.
   */
  const shouldPoll = Boolean(order && !order.terminal && order.awaitingPayment);

  useEffect(() => {
    if (!orderId || !shouldPoll) return;

    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let failures = 0;

    const clearTimer = () => {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
    };

    const schedule = (seconds: number) => {
      clearTimer();
      if (disposed) return;
      const bounded = Math.min(Math.max(seconds, 1), MAX_BACKOFF_SECONDS);
      timer = setTimeout(() => void run(), bounded * 1000);
    };

    const run = async () => {
      if (disposed) return;
      if (typeof document !== "undefined" && document.hidden) {
        clearTimer();
        setPollStatus("paused");
        return;
      }

      setPollStatus("checking");
      try {
        const result = await getIamOrderIfChanged(orderId, etagRef.current);
        if (disposed) return;

        failures = 0;
        setFailureCount(0);

        if (!result.notModified && result.body) {
          const next = result.body;
          queryClient.setQueryData<IamOrderDetail>(detailKey, next);
          if (result.etag) etagRef.current = result.etag;
          if (next.terminal || !next.awaitingPayment) {
            setPollStatus("stopped");
            return;
          }
        } else if (result.etag) {
          etagRef.current = result.etag;
        }

        // The interval comes from the resource, not from this file (§42.3.11
        // item 2). The cache is read rather than the captured `order` so the
        // effect does not re-subscribe on every data update.
        const current = queryClient.getQueryData<IamOrderDetail>(detailKey);
        setPollStatus("waiting");
        schedule(current?.pollAfterSeconds ?? 15);
      } catch (error) {
        if (disposed) return;

        // A resource that is gone or is no longer ours will not become readable
        // by asking again, so stop instead of hammering the endpoint.
        if (isHttpError(error) && [401, 403, 404].includes(error.status)) {
          setPollStatus("stopped");
          return;
        }

        failures += 1;
        setFailureCount(failures);
        // A server-suggested delay always wins; the growing delay below is only
        // for the case where the server said nothing at all.
        const retryAfter = isHttpError(error) ? error.retryAfterSeconds : undefined;
        setPollStatus("waiting");
        schedule(retryAfter ?? BASE_BACKOFF_SECONDS * 2 ** (failures - 1));
      }
    };

    const onVisibilityChange = () => {
      if (disposed) return;
      if (document.hidden) {
        clearTimer();
        setPollStatus("paused");
        return;
      }
      // Coming back to the tab is the moment a reader most expects fresh data,
      // so the remaining wait is skipped rather than resumed.
      void run();
    };

    document.addEventListener("visibilitychange", onVisibilityChange);
    schedule(queryClient.getQueryData<IamOrderDetail>(detailKey)?.pollAfterSeconds ?? 15);

    return () => {
      disposed = true;
      clearTimer();
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [orderId, shouldPoll, detailKey, queryClient]);

  const refresh = useCallback(() => {
    void query.refetch();
  }, [query]);

  if (query.isLoading) {
    return (
      <div className="gl-content__inner--reading">
        <Card>
          <div style={{ textAlign: "center", padding: 24 }}>
            <Spin indicator={<LoadingOutlined style={{ fontSize: 32 }} spin />} />
          </div>
        </Card>
      </div>
    );
  }

  if (query.isError) {
    return (
      <div className="gl-content__inner--reading">
        <Card>
          <Alert
            type="error"
            showIcon
            title={t("order.pay.notFound")}
            description={String(query.error)}
            action={
              <Button size="small" onClick={refresh}>
                {tc("retry")}
              </Button>
            }
          />
        </Card>
      </div>
    );
  }

  if (!order) {
    return (
      <div className="gl-content__inner--reading">
        <Card>
          <Alert type="error" showIcon title={t("order.pay.notFound")} />
        </Card>
      </div>
    );
  }

  const settled = Boolean(order.paidAt);
  const productLabel =
    order.productInfos && order.productInfos.length > 0
      ? order.productInfos.map((info) => info.displayName || info.name).join(", ")
      : order.displayName || order.name;

  return (
    <div className="gl-content__inner--reading">
      <Card>
        <Title level={4}>{t("order.pay.title")}</Title>
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label={t("order.pay.product")}>{productLabel}</Descriptions.Item>
          <Descriptions.Item label={t("order.pay.amount")}>
            <Text strong>
              {order.currency} {order.price}
            </Text>
          </Descriptions.Item>
          <Descriptions.Item label={tc("statusLabel")}>
            <StateTag value={order.state} />
          </Descriptions.Item>
          {order.tradeNo ? (
            <Descriptions.Item label={t("order.tradeNo")}>{order.tradeNo}</Descriptions.Item>
          ) : null}
          {order.paidAt ? (
            <Descriptions.Item label={t("order.pay.paidAtLabel")}>{order.paidAt}</Descriptions.Item>
          ) : null}
          <Descriptions.Item label={tc("updatedAt")}>{order.updatedAt}</Descriptions.Item>
        </Descriptions>

        {settled && (
          <Alert
            style={{ marginTop: 16 }}
            type="success"
            showIcon
            icon={<CheckCircleOutlined />}
            title={t("order.pay.paid")}
            description={order.paidAt ? t("order.pay.paidAt", { time: order.paidAt }) : undefined}
          />
        )}

        {shouldPoll && order.paymentUrl && (
          <div style={{ marginTop: 24, textAlign: "center" }}>
            <Space orientation="vertical" align="center" size="middle">
              <div>
                <QRCodeSVG value={order.paymentUrl} size={200} />
              </div>
              <Text type="secondary">{t("order.pay.scanToPay")}</Text>
              <Text type="secondary">
                {pollStatus === "checking" ? (
                  <>
                    <LoadingOutlined spin /> {t("order.pay.waitingPayment")}
                  </>
                ) : pollStatus === "paused" ? (
                  t("order.pay.pollPaused")
                ) : pollStatus === "stopped" ? (
                  t("order.pay.pollStopped")
                ) : failureCount > 0 ? (
                  t("order.pay.pollRetrying", { attempt: failureCount })
                ) : (
                  t("order.pay.pollWaiting")
                )}
              </Text>
              <Space>
                <Button
                  type="primary"
                  size="large"
                  href={order.paymentUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t("order.pay.payNow")}
                </Button>
                <Button size="large" onClick={refresh}>
                  {tc("reload")}
                </Button>
              </Space>
            </Space>
          </div>
        )}

        {shouldPoll && !order.paymentUrl && (
          <Alert
            style={{ marginTop: 16 }}
            type="warning"
            showIcon
            title={t("order.pay.noPaymentUrl")}
            description={order.message || undefined}
            action={
              <Button size="small" onClick={refresh}>
                {tc("reload")}
              </Button>
            }
          />
        )}

        {!shouldPoll && !settled && (
          <Alert
            style={{ marginTop: 16 }}
            type="warning"
            showIcon
            title={t("order.pay.terminalTitle")}
            description={localizedError(order, t, "order.pay.terminalHint")}
          />
        )}

        <div style={{ marginTop: 16, textAlign: "center" }}>
          <Button onClick={() => navigate(-1)}>{tc("backToList")}</Button>
        </div>
      </Card>
    </div>
  );
}
