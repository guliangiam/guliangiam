import { Button, Descriptions, Drawer, Popconfirm, Space, Tag, Tooltip, Typography } from "antd";
import { Link } from "react-router-dom";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { LinkButton } from "@/shared/ui/LinkButton";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { PopconfirmModal } from "@/shared/ui/PopconfirmModal";
import { formatDateTime } from "@/shared/lib/date-time";
import {
  useInvitationsPageController,
  type InvitationBulkAction,
} from "./useInvitationsPageController";
import type { InvitationDetail } from "@/shared/api/iam/invitation";

const { Text } = Typography;

const STATUS_COLORS: Record<string, string> = {
  pending: "blue",
  accepted: "green",
  expired: "orange",
  revoked: "red",
};

function renderInvitationBulkActions({
  sendLabel,
  revokeLabel,
  resendLabel,
  sendConfirm,
  revokeConfirm,
  resendConfirm,
  disabled,
  onRun,
}: {
  sendLabel: string;
  revokeLabel: string;
  resendLabel: string;
  sendConfirm: string;
  revokeConfirm: string;
  resendConfirm: string;
  disabled: boolean;
  onRun: (action: InvitationBulkAction) => void;
}) {
  return (
    <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
      <PopconfirmModal
        size="small"
        variant="default"
        disabled={disabled}
        text={sendLabel}
        title={sendConfirm}
        onConfirm={() => onRun("send")}
      />
      <PopconfirmModal
        size="small"
        variant="danger"
        disabled={disabled}
        text={revokeLabel}
        title={revokeConfirm}
        onConfirm={() => onRun("revoke")}
      />
      <PopconfirmModal
        size="small"
        variant="default"
        disabled={disabled}
        text={resendLabel}
        title={resendConfirm}
        onConfirm={() => onRun("resend")}
      />
    </PermissionGuard>
  );
}

export default function InvitationsPage() {
  const {
    t,
    tr,
    account,
    orgFilter,
    orgContext,
    orgNameMap,
    handleOrgChange,
    list,
    revokeMutation,
    resendMutation,
    sendMutation,
    bulkActionMutation,
    detailTarget,
    setDetailTarget,
    copyAcceptLink,
    roleName,
    groupName,
    userNameMap,
  } = useInvitationsPageController();

  const statusTag = (v: string) => (
    <Tag color={STATUS_COLORS[v] ?? "default"}>
      {tr(`invitation.status${v.charAt(0).toUpperCase()}${v.slice(1)}`, v)}
    </Tag>
  );

  const inviterLabel = (v?: string | null) => {
    if (!v) return "-";
    const name = userNameMap.get(v);
    if (name) return name;
    return <Tooltip title={v}>{`${v.slice(0, 8)}…`}</Tooltip>;
  };

  return (
    <>
      {renderCrudListView(
        {
          title: tr("invitation.title", "Invitations"),
          description: tr(
            "invitation.description",
            "Manage user invitations, view status, revoke or resend.",
          ),
          searchPlaceholder: tr("invitation.searchPlaceholder", "Filter by email"),
          searchableColumns: [{ dataIndex: "email", label: t("email") }],
          filters: [
            {
              key: "status",
              label: t("statusLabel"),
              options: [
                { label: tr("invitation.statusPending", "Pending"), value: "pending" },
                { label: tr("invitation.statusAccepted", "Accepted"), value: "accepted" },
                { label: tr("invitation.statusExpired", "Expired"), value: "expired" },
                { label: tr("invitation.statusRevoked", "Revoked"), value: "revoked" },
              ],
            },
          ],
          rowKey: "id",
          toolbar: (
            <Space>
              {account?.isAdmin ? (
                <OrganizationSelect
                  value={orgFilter}
                  onChange={handleOrgChange}
                  style={{ width: 180 }}
                  allowClear
                />
              ) : orgContext ? (
                // 非平台管理员从组织列表快捷入口进入时，也要让组织上下文可见、可清除。
                <Tag closable onClose={() => handleOrgChange("")} color="blue">
                  {t("organization")}: {orgNameMap.get(orgContext) ?? orgContext}
                </Tag>
              ) : null}
              <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                <LinkButton type="primary" to="/invitations/new">
                  {tr("invitation.newInvitation", "New Invitation")}
                </LinkButton>
              </PermissionGuard>
            </Space>
          ),
          // 无删除端点，enableBulkDelete 仅用于开启复选框列；动作由 bulkActions 提供。
          enableBulkDelete: true,
          rowSelectable: (record) => record.status === "pending",
          bulkActions: ({ selectedKeys, records }) => {
            const selected = records.filter(
              (record) => selectedKeys.includes(record.id) && record.status === "pending",
            );
            const run = (action: InvitationBulkAction) => {
              if (selected.length === 0) return;
              bulkActionMutation.mutate({ action, ids: selected.map((record) => record.id) });
            };
            return renderInvitationBulkActions({
              sendLabel: tr("invitation.bulkSend", "Bulk Send"),
              revokeLabel: tr("invitation.bulkRevoke", "Bulk Revoke"),
              resendLabel: tr("invitation.bulkResend", "Bulk Resend"),
              sendConfirm: tr("invitation.bulkSendConfirm", { count: selected.length }),
              revokeConfirm: tr("invitation.bulkRevokeConfirm", { count: selected.length }),
              resendConfirm: tr("invitation.bulkResendConfirm", { count: selected.length }),
              disabled: bulkActionMutation.isPending || selected.length === 0,
              onRun: run,
            });
          },
          columns: [
            {
              title: t("email"),
              dataIndex: "email",
              key: "email",
              width: 220,
              fixed: "left",
              render: (v: string, record: InvitationDetail) =>
                record.organizationId ? (
                  <Link to={`/invitations/${record.organizationId}/${record.email}`}>{v}</Link>
                ) : (
                  v
                ),
            },
            {
              title: t("statusLabel"),
              dataIndex: "status",
              key: "status",
              render: (v: string) => statusTag(v),
            },
            {
              title: tr("invitation.invitedBy", "Invited By"),
              dataIndex: "invitedBy",
              key: "invitedBy",
              render: (v?: string | null) => inviterLabel(v),
            },
            {
              title: tr("invitation.expiresAt", "Expires At"),
              dataIndex: "expiresAt",
              key: "expiresAt",
              render: (v?: string | null) => formatDateTime(v),
            },
            {
              title: tr("invitation.createdAt", "Created At"),
              dataIndex: "createdAt",
              key: "createdAt",
              render: (v?: string | null) => formatDateTime(v),
            },
            {
              title: t("actions"),
              key: "actions",
              fixed: "right",
              width: 320,
              render: (_, record: InvitationDetail) => (
                <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
                  <Button type="link" size="small" onClick={() => setDetailTarget(record)}>
                    {t("detail")}
                  </Button>
                  <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                    <Popconfirm
                      title={tr("invitation.sendConfirm", "Send this invitation?")}
                      onConfirm={() => sendMutation.mutate(record.id)}
                    >
                      <Button type="link" size="small" disabled={record.status !== "pending"}>
                        {tr("invitation.send", "Send")}
                      </Button>
                    </Popconfirm>
                  </PermissionGuard>
                  <Button
                    type="link"
                    size="small"
                    disabled={!record.code}
                    onClick={() => void copyAcceptLink(record)}
                  >
                    {tr("invitation.copyAcceptLink", "Copy Accept Link")}
                  </Button>
                  <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                    <Popconfirm
                      title={tr("invitation.revokeConfirm", "Revoke this invitation?")}
                      onConfirm={() => revokeMutation.mutate(record.id)}
                    >
                      <Button
                        type="link"
                        size="small"
                        danger
                        disabled={record.status !== "pending"}
                      >
                        {tr("invitation.revoke", "Revoke")}
                      </Button>
                    </Popconfirm>
                  </PermissionGuard>
                  <PermissionGuard action="invitation:write" resource="invitations" fallback={null}>
                    <Popconfirm
                      title={tr("invitation.resendConfirm", "Resend this invitation?")}
                      onConfirm={() => resendMutation.mutate(record.id)}
                    >
                      <Button type="link" size="small" disabled={record.status !== "pending"}>
                        {tr("invitation.resend", "Resend")}
                      </Button>
                    </Popconfirm>
                  </PermissionGuard>
                </div>
              ),
            },
          ],
        },
        list,
      )}
      <Drawer
        title={tr("invitation.detailTitle", "Invitation Detail")}
        open={detailTarget !== null}
        onClose={() => setDetailTarget(null)}
        size={480}
      >
        {detailTarget && (
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label={t("email")}>{detailTarget.email}</Descriptions.Item>
            <Descriptions.Item label={t("statusLabel")}>
              {statusTag(detailTarget.status)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.code", "Invitation Code")}>
              {detailTarget.code ? (
                <Text copyable={{ text: detailTarget.code }}>{detailTarget.code}</Text>
              ) : (
                "-"
              )}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.roles", "Roles")}>
              {detailTarget.roleIds.length > 0
                ? detailTarget.roleIds.map(roleName).join(", ")
                : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.groups", "Groups")}>
              {detailTarget.groupIds && detailTarget.groupIds.length > 0
                ? detailTarget.groupIds.map(groupName).join(", ")
                : "-"}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.invitedBy", "Invited By")}>
              {inviterLabel(detailTarget.invitedBy)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.expiresAt", "Expires At")}>
              {formatDateTime(detailTarget.expiresAt)}
            </Descriptions.Item>
            <Descriptions.Item label={tr("invitation.acceptedAt", "Accepted At")}>
              {formatDateTime(detailTarget.acceptedAt)}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </>
  );
}
