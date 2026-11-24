import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { deleteGroup, getGroup, getGroupsPaginated, updateGroup } from "@/shared/api/iam/group";
import type { GroupDetail } from "@/shared/api/iam/group";
import { getRoles } from "@/shared/api/iam/role";
import { getOrganizationIdentityBySlug } from "@/shared/api/iam/organization";
import { GroupMembersDrawer } from "./GroupMembersDrawer";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { GroupTreeSelect } from "@/shared/ui/GroupTreeSelect";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { GlFormRow } from "@/shared/ui/GlForm";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { localizedError } from "@/shared/lib/server-error";

type GroupFormValues = {
  displayName: string;
  type: string;
  parentId: string;
  homepage: string;
  manager: string;
  contactEmail: string;
  isTopGroup: boolean;
  isEnabled: boolean;
  users: string[];
};

/**
 * 父组候选必须排除当前组与全部后代，否则会形成环（后端也不做该校验）。
 * 在渲染期直接计算：组列表规模小，无需 memo 化。
 */
function computeExcludedParentIds(
  currentGroupId: string | undefined,
  groupList: Array<{ id: string; parentId?: string | null }> | undefined,
): string[] {
  if (!currentGroupId || !groupList) return [];
  const childrenByParent = new Map<string, string[]>();
  for (const group of groupList) {
    if (!group.parentId) continue;
    const list = childrenByParent.get(group.parentId) ?? [];
    list.push(group.id);
    childrenByParent.set(group.parentId, list);
  }
  const result: string[] = [];
  const queue: string[] = [currentGroupId];
  const seen = new Set<string>();
  while (queue.length > 0) {
    const id = queue.shift() as string;
    if (seen.has(id)) continue;
    seen.add(id);
    result.push(id);
    for (const child of childrenByParent.get(id) ?? []) queue.push(child);
  }
  return result;
}

export default function GroupDetailPage() {
  const { organizationName, groupName } = useParams<{
    organizationName: string;
    groupName: string;
  }>();
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<GroupFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();
  const GROUP_TYPE_OPTIONS = [
    { label: tr("group.typePhysical"), value: "physical" },
    { label: tr("group.typeVirtual"), value: "virtual" },
    { label: tr("group.typeDepartment"), value: "department" },
  ];

  // 路由参数是组织 slug（或 id）：先解析成组织 id，再带着它查列表与详情。
  // 不解析就查会把请求钉在会话组织上，超管查看其他组织的组时必然 404。
  const organizationQuery = useQuery({
    queryKey: ["organizations", "identity", organizationName],
    queryFn: () => getOrganizationIdentityBySlug(organizationName ?? ""),
    enabled: !!organizationName,
  });
  const organizationId = organizationQuery.data?.id ?? "";

  const listQuery = useQuery({
    queryKey: ["groups", "auth", "detail-match", organizationId, groupName],
    queryFn: async () => {
      // 大组织下首屏 50 条可能不含目标组，导致详情页误报“不存在”。
      // 先用服务端 keyword 精确定位；未命中再退回全量翻页扫描（上限 10 页）。
      if (!organizationId || !groupName) return [];
      const searched = await getGroupsPaginated({
        page: 1,
        pageSize: 50,
        search: groupName,
        organizationId,
      });
      if (searched.items.some((g) => g.name === groupName)) return searched.items;
      const first = await getGroupsPaginated({ page: 1, pageSize: 50, organizationId });
      const totalPages = Math.min(Math.ceil(first.total / 50), 10);
      let all = first.items;
      for (let page = 2; page <= totalPages; page++) {
        const next = await getGroupsPaginated({ page, pageSize: 50, organizationId });
        all = all.concat(next.items);
      }
      return all;
    },
    enabled: !!organizationId && !!groupName,
  });

  const matchedGroup = listQuery.data?.find(
    (g) => g.name === groupName && (g.owner === organizationId || g.owner === organizationName),
  );

  const detailQuery = useQuery({
    queryKey: ["group", "auth", "detail", matchedGroup?.id],
    queryFn: async () => {
      if (!matchedGroup?.id) return null;
      return getGroup(matchedGroup.id, organizationId || undefined);
    },
    enabled: !!matchedGroup?.id,
  });

  // React Compiler 对 react-query 返回的数组做保守判定（认为可能被就地修改），
  // 手写 useMemo 依赖无法通过 preserve-manual-memoization 校验；组列表规模小，
  // 直接在渲染期计算，无缓存必要。
  const groupList = listQuery.data;
  const currentGroupId = detailQuery.data?.id ?? matchedGroup?.id;
  // 父组候选必须排除当前组与全部后代，否则会形成环（后端也不做该校验）。
  const excludedParentIds = computeExcludedParentIds(currentGroupId, groupList);

  const groupNavItems = useMemo<SectionNavItem[]>(
    () => [
      { key: "basic", anchor: "group-section-basic", label: tr("group.basicInfo") },
      { key: "contact", anchor: "group-section-contact", label: tr("group.contactInfo") },
      { key: "members", anchor: "group-section-members", label: tr("group.memberUsers") },
    ],
    [tr],
  );

  const [membersOpen, setMembersOpen] = useState(false);

  // G3：组↔角色关联为只读展示——数据源是角色的 groupIds（无独立绑定端点）。
  const rolesQuery = useQuery({
    queryKey: ["roles", "auth", "group-related", organizationId],
    queryFn: () => getRoles(organizationId),
    enabled: !!organizationId,
  });

  useEffect(() => {
    if (detailQuery.data) {
      const detail = detailQuery.data as GroupDetail;
      form.setFieldsValue({
        displayName: detail.displayName,
        // 闭集外的遗留值（如历史 admin/operator/空串）不回填：原样提交会被
        // 后端枚举校验拒绝；置空后保存省略 type，保留原值直到用户显式改选。
        type: ["virtual", "physical", "department"].includes(detail.type) ? detail.type : undefined,
        parentId: detail.parentId,
        homepage: detail.homepage ?? "",
        manager: detail.manager,
        contactEmail: detail.contactEmail,
        isTopGroup: detail.isTopGroup,
        isEnabled: detail.isEnabled,
        users: detail.users ?? [],
      });
    }
  }, [detailQuery.data, form]);

  const updateMutation = useMutation({
    mutationFn: async (values: GroupFormValues) => {
      if (!detailQuery.data) throw new Error(tr("group.detailNotLoaded"));
      return updateGroup(
        detailQuery.data.id,
        {
          displayName: values.displayName,
          type: values.type,
          parentId: values.parentId,
          homepage: values.homepage,
          manager: values.manager,
          contactEmail: values.contactEmail,
          isTopGroup: values.isTopGroup,
          isEnabled: values.isEnabled,
          users: values.users ?? [],
          revision: detailQuery.data.revision,
        },
        organizationId || undefined,
      );
    },
    onSuccess: () => {
      message.success(tc("updateSuccess"));
      // 详情键是单数 ["group","auth","detail",id]；只失效 ["groups"] 会让
      // revision 停留在旧值，同页第二次保存必 409。
      void queryClient.invalidateQueries({ queryKey: ["groups"] });
      void queryClient.invalidateQueries({ queryKey: ["group"] });
      // users 成员变更会反映到用户详情的所属组列表（镜像 GroupMembersDrawer）。
      void queryClient.invalidateQueries({ queryKey: ["user"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!detailQuery.data) throw new Error(tr("group.detailNotLoaded"));
      return deleteGroup(detailQuery.data.id, organizationId || undefined);
    },
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      void queryClient.invalidateQueries({ queryKey: ["groups"] });
      void queryClient.invalidateQueries({ queryKey: ["user"] });
      navigate("/groups");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  if (listQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (listQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(listQuery.error)}
        extra={
          <Button type="primary" onClick={() => listQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  if (!matchedGroup) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("group.notFound")}
          description={tr("group.notFoundDesc", {
            organization: organizationName,
            group: groupName,
          })}
        />
        <div style={{ marginTop: 16 }}>
          <Button onClick={() => navigate("/groups")}>{tc("backToList")}</Button>
        </div>
      </Card>
    );
  }

  const detail = detailQuery.data as GroupDetail | undefined;
  const memberOrganizationId = detail?.organizationId ?? matchedGroup.owner;
  const parentCandidates = (listQuery.data ?? []).filter((g) => g.owner === matchedGroup.owner);
  const relatedRoles = (rolesQuery.data ?? []).filter(
    (role) => currentGroupId && role.groupIds.includes(currentGroupId),
  );

  return (
    <ResourceFormPage
      title={tr("group.detailTitle", { name: matchedGroup.displayName || groupName })}
      description={tr("group.editDescription")}
      loading={detailQuery.isLoading}
      saving={updateMutation.isPending}
      form={form}
      enableDraft
      draftRoute="groups"
      draftEntityId={matchedGroup.id}
      onCancel={() => navigate("/groups")}
      submitPermission={{ permission: "group:write", resource: "groups" }}
      onFinish={async (values: GroupFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
      extra={
        <>
          <Button onClick={() => setMembersOpen(true)} disabled={!detail}>
            {tr("group.manageMembers", { name: detail?.displayName || detail?.name || "" })}
          </Button>
          <PermissionGuard action="group:write" resource="groups" fallback={null}>
            <Popconfirm
              title={tc("deleteConfirm")}
              description={tr("group.deleteDesc")}
              onConfirm={() => deleteMutation.mutate()}
              okText={tc("delete")}
              cancelText={tc("cancel")}
              okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
            >
              <Button danger>{tc("delete")}</Button>
            </Popconfirm>
          </PermissionGuard>
        </>
      }
    >
      <div style={{ gridColumn: "1 / -1", minWidth: 0 }}>
        <SectionNavShell items={groupNavItems}>
          <div className="gl-section-anchors">
            <div id="group-section-basic" className="gl-section-anchor">
              <FormSection title={tr("group.basicInfo")}>
                <Form.Item label={tc("name")}>
                  <span>{matchedGroup.name}</span>
                </Form.Item>
                <Form.Item label={tr("group.organization")}>
                  <span>{orgNameMap.get(matchedGroup.owner) ?? matchedGroup.owner}</span>
                </Form.Item>
                <Form.Item label={tc("statusLabel")}>
                  {matchedGroup.isEnabled ? (
                    <Tag color="green">{tc("enabled")}</Tag>
                  ) : (
                    <Tag color="red">{tc("disabled")}</Tag>
                  )}
                </Form.Item>
                <Form.Item name="isEnabled" valuePropName="checked">
                  <GlFormRow label={tc("enabled")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
                <Form.Item
                  name="displayName"
                  rules={[{ required: true, message: tr("group.validation.displayNameRequired") }]}
                >
                  <GlFormRow label={tc("displayName")} required>
                    <Input />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="type">
                  <GlFormRow label={tr("group.type")}>
                    <Select options={GROUP_TYPE_OPTIONS} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="parentId">
                  <GlFormRow label={tr("group.parentId")}>
                    <GroupTreeSelect
                      groups={parentCandidates.map((g) => ({
                        id: g.id,
                        name: g.name,
                        displayName: g.displayName,
                        parentId: g.parentId,
                      }))}
                      excludeIds={excludedParentIds}
                      placeholder={tr("group.selectParent")}
                    />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="isTopGroup" valuePropName="checked">
                  <GlFormRow label={tr("group.isTop")} inline>
                    <Switch />
                  </GlFormRow>
                </Form.Item>
              </FormSection>
            </div>

            <div id="group-section-contact" className="gl-section-anchor">
              <FormSection title={tr("group.contactInfo")}>
                <Form.Item
                  name="homepage"
                  rules={[{ type: "url", message: tr("form.invalidFormat") }]}
                >
                  <GlFormRow label={tr("group.homepage")}>
                    <Input placeholder="https://example.com" />
                  </GlFormRow>
                </Form.Item>
                <Form.Item name="manager">
                  <GlFormRow label={tr("group.manager")}>
                    <Input />
                  </GlFormRow>
                </Form.Item>
                <Form.Item
                  name="contactEmail"
                  rules={[{ type: "email", message: tr("form.invalidFormat") }]}
                >
                  <GlFormRow label={tr("group.contactEmail")}>
                    <Input />
                  </GlFormRow>
                </Form.Item>
              </FormSection>
            </div>

            <div id="group-section-members" className="gl-section-anchor">
              <FormSection title={tr("group.memberUsers")}>
                <Form.Item name="users">
                  <GlFormRow label={tr("group.members")}>
                    <UserPaginateSelect organizationId={memberOrganizationId} />
                  </GlFormRow>
                </Form.Item>
                <Form.Item label={tr("group.relatedRoles")}>
                  {relatedRoles.length > 0 ? (
                    <Space wrap>
                      {relatedRoles.map((role) => (
                        <Tag key={role.id}>{role.displayName || role.name}</Tag>
                      ))}
                    </Space>
                  ) : (
                    <span>{tr("group.noRelatedRoles")}</span>
                  )}
                </Form.Item>
              </FormSection>
            </div>
          </div>
        </SectionNavShell>
      </div>
      {membersOpen && (
        <GroupMembersDrawer
          group={detail ?? null}
          organizationId={organizationId || memberOrganizationId}
          onClose={() => setMembersOpen(false)}
        />
      )}
    </ResourceFormPage>
  );
}
