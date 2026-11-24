import { Button, Tag, Modal, Form, Select, Input, App } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  listRiskDecisions,
  reviewRiskDecision,
  type RiskDecisionItem,
  type ReviewRiskDecisionRequest,
} from "@/shared/api/iam/risk";
import { RISK_DECISION_ACTIONS } from "@/shared/constants/enums";
import { ResourceListCard } from "@/shared/ui/ResourceListCard";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { isHttpError } from "@/shared/api/http";
import { formatDateTime } from "@/shared/lib/date-time";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  serverErrorMessage,
} from "@/shared/lib/server-error";

const decisionColor: Record<string, string> = {
  allow: "green",
  challenge: "orange",
  deny: "red",
};

const decisionLabel: Record<string, string> = {
  allow: "Allow",
  challenge: "Challenge",
  deny: "Deny",
};

type ReviewFormValues = Omit<ReviewRiskDecisionRequest, "revision">;

export default function RiskDecisionsTab() {
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  // 复核必须携带记录当前 revision（后端乐观并发必填），因此保存整条记录而非仅 id。
  const [reviewTarget, setReviewTarget] = useState<RiskDecisionItem | null>(null);
  const [form] = Form.useForm<ReviewFormValues>();

  const query = useQuery({
    queryKey: ["risk", "decisions"],
    queryFn: listRiskDecisions,
    select: (data) => data?.items ?? [],
  });

  const reviewMutation = useMutation({
    mutationFn: (input: ReviewFormValues) =>
      reviewTarget
        ? reviewRiskDecision(reviewTarget.id, { ...input, revision: reviewTarget.revision })
        : Promise.reject(new Error("no review target")),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["risk", "decisions"] });
      message.success(tc("updateSuccess"));
      setReviewTarget(null);
      form.resetFields();
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(form, err, buildServerFieldResolver<ReviewFormValues>(form), tc);
      }
      message.error(serverErrorMessage(err, tc, "common:error"));
    },
  });

  return (
    <>
      <ResourceListCard
        title={tc("risk.decisions", "Risk Decisions")}
        description={tc(
          "risk.decisionsDescription",
          "Review and adjudicate risk decisions from the adaptive engine.",
        )}
        query={query}
        rowKey="id"
        columns={[
          {
            title: tc("risk.decision", "Decision"),
            dataIndex: "decision",
            key: "decision",
            render: (v: string) => (
              <Tag color={decisionColor[v] ?? "default"}>{v?.toUpperCase()}</Tag>
            ),
          },
          { title: tc("risk.score", "Score"), dataIndex: "score", key: "score" },
          {
            title: tc("risk.reason", "Reason"),
            key: "reason",
            render: (_, record) => record.workflow?.review?.reason || "-",
          },
          {
            title: tc("risk.reviewedBy", "Reviewed By"),
            key: "reviewedBy",
            render: (_, record) => record.workflow?.review?.actorId || "-",
          },
          {
            title: tc("createdAt"),
            dataIndex: "createdAt",
            key: "createdAt",
            render: (v: string) => formatDateTime(v),
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 120,
            render: (_, record) => (
              <PermissionGuard action="risk:write" resource="risk" fallback={null}>
                <Button type="link" size="small" onClick={() => setReviewTarget(record)}>
                  {tc("risk.review", "Review")}
                </Button>
              </PermissionGuard>
            ),
          },
        ]}
      />

      <Modal
        open={!!reviewTarget}
        title={tc("risk.reviewDecision", "Review Risk Decision")}
        onCancel={() => setReviewTarget(null)}
        onOk={() => {
          void form
            .validateFields()
            .then((values) => reviewMutation.mutate(values))
            .catch(() => undefined);
        }}
        confirmLoading={reviewMutation.isPending}
      >
        <Form form={form} layout="vertical" initialValues={{ decision: "allow" }}>
          <Form.Item
            label={tc("risk.newDecision", "New Decision")}
            name="decision"
            rules={[{ required: true }]}
          >
            <Select
              options={RISK_DECISION_ACTIONS.map((value) => ({
                value,
                label: tc(`risk.${value}`, decisionLabel[value]),
              }))}
            />
          </Form.Item>
          <Form.Item
            label={tc("risk.reason", "Reason")}
            name="reason"
            rules={[{ required: true }, { max: 2000 }]}
          >
            <Input.TextArea rows={2} maxLength={2000} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
