import { useState } from "react";
import { App, Button, Drawer, Popconfirm, Space, Table, Tag } from "antd";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { updateGroup, type GroupDetail } from "@/shared/api/iam/group";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { localizedError } from "@/shared/lib/server-error";

type GroupMembersDrawerProps = {
  group: GroupDetail | null;
  organizationId?: string;
  onClose: () => void;
};

/**
 * 组成员管理抽屉：展示当前成员，支持添加、批量移除，保存走 PATCH updateGroup
 * （users + revision round-trip），与详情页表单保存共用同一端点。
 * 由父组件条件挂载：每次打开都以最新 group.users 重新初始化本地状态。
 */
export function GroupMembersDrawer({ group, organizationId, onClose }: GroupMembersDrawerProps) {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [members, setMembers] = useState<string[]>(() => group?.users ?? []);
  const [selected, setSelected] = useState<string[]>([]);
  const [pendingUsers, setPendingUsers] = useState<string[]>([]);

  // 成员口径为 username（种子、Casbin sub 与 UserPaginateSelect 一致）；
  // 服务端 normalizeGroupUsers 接受 username/ID 两种形式并校验租户归属。
  const saveMutation = useMutation({
    mutationFn: () => {
      if (!group) throw new Error("group not loaded");
      return updateGroup(
        group.id,
        { users: members, revision: group.revision },
        organizationId || undefined,
      );
    },
    onSuccess: () => {
      message.success(tr("group.membersSaved"));
      void queryClient.invalidateQueries({ queryKey: ["groups"] });
      void queryClient.invalidateQueries({ queryKey: ["group"] });
      // 用户详情页展示 groupMemberships：成员变更同样要失效 ["user"] 前缀，
      // 否则打开中的详情页继续显示旧的所属组。
      void queryClient.invalidateQueries({ queryKey: ["user"] });
      onClose();
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const addPending = () => {
    if (pendingUsers.length === 0) return;
    setMembers((prev) => Array.from(new Set([...prev, ...pendingUsers])));
    setPendingUsers([]);
  };

  const removeSelected = () => {
    setMembers((prev) => prev.filter((user) => !selected.includes(user)));
    setSelected([]);
  };

  return (
    <Drawer
      title={tr("group.manageMembers", { name: group?.displayName ?? "" })}
      open
      onClose={onClose}
      size={520}
      extra={
        <PermissionGuard action="group:write" resource="groups" fallback={null}>
          <Button
            type="primary"
            loading={saveMutation.isPending}
            disabled={group === null}
            onClick={() => saveMutation.mutate()}
          >
            {tc("save")}
          </Button>
        </PermissionGuard>
      }
    >
      <Space style={{ marginBottom: 12 }} wrap>
        <UserPaginateSelect
          organizationId={organizationId ?? group?.organizationId ?? ""}
          value={pendingUsers}
          onChange={(value) => setPendingUsers(value)}
          placeholder={tr("group.addMembersPlaceholder")}
        />
        <Button onClick={addPending} disabled={pendingUsers.length === 0}>
          {tr("group.addMembers")}
        </Button>
        <Popconfirm
          title={tr("group.removeSelectedConfirm", { count: selected.length })}
          onConfirm={removeSelected}
          disabled={selected.length === 0}
          okText={tc("ok")}
          cancelText={tc("cancel")}
        >
          <Button danger disabled={selected.length === 0}>
            {tr("group.removeSelected")}
          </Button>
        </Popconfirm>
      </Space>
      <Table<string>
        size="small"
        rowKey={(user) => user}
        dataSource={members}
        pagination={false}
        rowSelection={{
          selectedRowKeys: selected,
          onChange: (keys) => setSelected(keys as string[]),
        }}
        columns={[
          {
            title: tc("username"),
            key: "user",
            render: (_: unknown, user: string) => <Tag>{user}</Tag>,
          },
          {
            title: tc("actions"),
            key: "actions",
            width: 90,
            render: (_, user) => (
              <Button
                type="link"
                size="small"
                danger
                onClick={() => setMembers((prev) => prev.filter((m) => m !== user))}
              >
                {tr("group.removeMember")}
              </Button>
            ),
          },
        ]}
        locale={{ emptyText: tr("group.noMembers") }}
      />
    </Drawer>
  );
}
