import { Alert, Button, Card, Empty, Skeleton, Space, Tag, Timeline, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { EntryTranscriptPageControllerState } from "./useEntryTranscriptPageController";
import { LinkButton } from "@/shared/ui/LinkButton";

const kindColor: Record<string, string> = {
  span: "blue",
  log: "gold",
  metrics: "purple",
};

function MessageBody({
  bodyText,
  bodyOmitted,
  bodyBytes,
}: {
  bodyText: string;
  bodyOmitted: boolean;
  bodyBytes: number;
}) {
  const { t } = useTranslation("resource");

  if (bodyOmitted) {
    // The server replaced an oversized record with a marker instead of serving it,
    // so that one huge message cannot block the rest of the sequence (43.2.6 条 6).
    return (
      <Alert
        showIcon
        type="warning"
        title={t("entry.transcriptBodyOmitted", { bytes: bodyBytes })}
      />
    );
  }

  if (bodyText.length === 0) {
    return <Typography.Text type="secondary">{t("entry.transcriptBodyEmpty")}</Typography.Text>;
  }

  return (
    <pre
      style={{
        margin: 0,
        maxHeight: 320,
        overflow: "auto",
        fontSize: 12,
        whiteSpace: "pre-wrap",
        wordBreak: "break-all",
      }}
    >
      {bodyText}
    </pre>
  );
}

/**
 * Presentational half of the transcript viewer. It renders exactly what the
 * controller resolved and owns no read logic: no redaction, no cursor handling,
 * no state inference (28.9 交互契约).
 */
export function EntryTranscriptPageCard(controller: EntryTranscriptPageControllerState) {
  const { t } = useTranslation("resource");

  if (controller.status === "loading") {
    return (
      <Card className="gl-panel" title={t("entry.transcriptTitle")}>
        <Skeleton active paragraph={{ rows: 8 }} />
      </Card>
    );
  }

  if (controller.status === "notFound") {
    return (
      <Card className="gl-panel" title={t("entry.transcriptTitle")}>
        <Empty
          description={
            <Space orientation="vertical" size={4}>
              <Typography.Text>{t("entry.transcriptNotFound")}</Typography.Text>
              <Typography.Text type="secondary">
                {t("entry.transcriptNotFoundDesc")}
              </Typography.Text>
            </Space>
          }
        >
          <LinkButton type="primary" to="/entries">
            {t("entry.transcriptBackToList")}
          </LinkButton>
        </Empty>
      </Card>
    );
  }

  if (controller.status === "error") {
    return (
      <Card className="gl-panel" title={t("entry.transcriptTitle")}>
        <Space orientation="vertical" size={16} style={{ width: "100%" }}>
          <Alert showIcon type="error" title={controller.errorMessage} />
          <Button type="primary" onClick={controller.onRetry}>
            {t("entry.transcriptRetry")}
          </Button>
        </Space>
      </Card>
    );
  }

  return (
    <Card className="gl-panel" title={t("entry.transcriptTitle")}>
      <Space orientation="vertical" size={16} style={{ width: "100%" }}>
        <Space wrap>
          <Typography.Text type="secondary">{t("entry.transcriptEntryId")}: </Typography.Text>
          <Typography.Text code copyable>
            {controller.entryId}
          </Typography.Text>
          <Tag>{t("entry.transcriptMessageCount", { count: controller.messages.length })}</Tag>
          <Button size="small" loading={controller.isRefreshing} onClick={controller.onRefresh}>
            {t("entry.transcriptRefresh")}
          </Button>
          <LinkButton size="small" to={`/entries/${controller.entryId}`}>
            {t("entry.transcriptBackToDetail")}
          </LinkButton>
        </Space>

        <Typography.Text type="secondary">{t("entry.transcriptAppendOrderHint")}</Typography.Text>

        {controller.redactedFields.length > 0 ? (
          <Alert
            showIcon
            type="info"
            title={t("entry.transcriptRedactedNotice", {
              count: controller.redactedFields.length,
            })}
            description={
              <Typography.Text code style={{ fontSize: 12 }}>
                {controller.redactedFields.join(", ")}
              </Typography.Text>
            }
          />
        ) : null}

        {controller.loadMoreFailed ? (
          <Alert showIcon type="warning" title={t("entry.transcriptLoadMoreFailed")} />
        ) : null}

        {controller.messages.length === 0 ? (
          <Empty description={t("entry.transcriptEmpty")} />
        ) : (
          <Timeline
            items={controller.messages.map((message) => ({
              key: String(message.index),
              color: message.bodyOmitted ? "orange" : undefined,
              children: (
                <Space orientation="vertical" size={4} style={{ width: "100%" }}>
                  <Space wrap size={8}>
                    <Tag color="default">#{message.index}</Tag>
                    <Tag color={kindColor[message.kind] ?? "default"}>{message.kind}</Tag>
                    {message.name ? <Typography.Text strong>{message.name}</Typography.Text> : null}
                    {message.scope ? (
                      <Typography.Text type="secondary">{message.scope}</Typography.Text>
                    ) : null}
                    <Typography.Text type="secondary">
                      {message.timestamp ? new Date(message.timestamp).toLocaleString() : "-"}
                    </Typography.Text>
                  </Space>
                  <MessageBody
                    bodyText={message.bodyText}
                    bodyOmitted={message.bodyOmitted}
                    bodyBytes={message.bodyBytes}
                  />
                </Space>
              ),
            }))}
          />
        )}

        {controller.hasMore ? (
          <Button
            block
            loading={controller.isLoadingMore}
            onClick={controller.onLoadMore}
            disabled={controller.isLoadingMore}
          >
            {t("entry.transcriptLoadMore")}
          </Button>
        ) : (
          <Typography.Text type="secondary" style={{ textAlign: "center", display: "block" }}>
            {t("entry.transcriptEnd")}
          </Typography.Text>
        )}
      </Space>
    </Card>
  );
}
