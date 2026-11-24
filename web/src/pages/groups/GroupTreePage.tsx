import { useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Empty,
  Input,
  Row,
  Skeleton,
  Space,
  Tag,
  Tree,
  Typography,
} from "antd";
import { SearchOutlined } from "@ant-design/icons";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { getGroup, getGroups } from "@/shared/api/iam";
import { LinkButton } from "@/shared/ui/LinkButton";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { localizedError } from "@/shared/lib/server-error";

interface GroupItem {
  id: string;
  owner: string;
  name: string;
  displayName: string;
  type: string;
  parentId: string;
  isTopGroup: boolean;
  isEnabled: boolean;
}

interface TreeNode {
  key: string;
  title: React.ReactNode;
  children: TreeNode[];
  group: GroupItem;
}

const EMPTY_GROUPS: GroupItem[] = [];

function buildTree(
  groups: GroupItem[],
  renderTitle: (group: GroupItem) => React.ReactNode,
): TreeNode[] {
  const map = new Map<string, TreeNode>();
  const roots: TreeNode[] = [];
  const visited = new Set<string>();

  // Create all nodes first
  for (const g of groups) {
    map.set(g.id, {
      key: g.id,
      title: renderTitle(g),
      children: [],
      group: g,
    });
  }

  // Build parent-child relationships, detecting circular references
  for (const g of groups) {
    visited.clear();
    let currentParentId = g.parentId;

    // Walk up the tree to detect circular references
    while (currentParentId) {
      if (currentParentId === g.id) {
        // Circular reference detected - treat as root
        break;
      }
      if (visited.has(currentParentId)) {
        // Already visited this ancestor - circular reference
        break;
      }
      visited.add(currentParentId);
      const parentNode = map.get(currentParentId);
      if (!parentNode) {
        // Parent doesn't exist - treat as root
        break;
      }
      currentParentId = parentNode.group.parentId;
    }

    if (g.parentId && map.has(g.parentId) && !isCircularParent(g, map)) {
      map.get(g.parentId)!.children.push(map.get(g.id)!);
    } else {
      roots.push(map.get(g.id)!);
    }
  }

  return roots;
}

function isCircularParent(group: GroupItem, map: Map<string, TreeNode>): boolean {
  const visited = new Set<string>();
  let currentId = group.parentId;
  while (currentId) {
    if (currentId === group.id) return true;
    if (visited.has(currentId)) return true;
    visited.add(currentId);
    const node = map.get(currentId);
    if (!node) break;
    currentId = node.group.parentId;
  }
  return false;
}

/**
 * 只过滤已取回的组数据（不发请求）：保留命中节点及其祖先链，避免命中项被
 * 挂在被裁掉的父节点下面而断树。
 */
function filterTree(nodes: TreeNode[], rawQuery: string): TreeNode[] {
  const query = rawQuery.trim().toLowerCase();
  if (!query) return nodes;

  const visit = (node: TreeNode): TreeNode | null => {
    const children = node.children.map(visit).filter((child): child is TreeNode => child !== null);
    const matches =
      node.group.name.toLowerCase().includes(query) ||
      (node.group.displayName ?? "").toLowerCase().includes(query);
    if (!matches && children.length === 0) return null;
    return { ...node, children };
  };

  return nodes.map(visit).filter((node): node is TreeNode => node !== null);
}

export default function GroupTreePage() {
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const organizationId = searchParams.get("organizationId") ?? undefined;
  const groupParam = searchParams.get("group") ?? undefined;
  const [filterText, setFilterText] = useState("");

  const {
    data: groups,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["groups", "iam", "tree", organizationId],
    queryFn: () => getGroups(organizationId),
  });

  const groupList = groups ?? EMPTY_GROUPS;
  // URL 是选中的唯一来源：刷新/深链能恢复，未知 id 回落到未选中（不报错、不请求详情）。
  const selectedGroupId =
    groupParam && groupList.some((group) => group.id === groupParam) ? groupParam : undefined;
  const selectedGroup = selectedGroupId
    ? groupList.find((group) => group.id === selectedGroupId)
    : undefined;

  const detailQuery = useQuery({
    queryKey: ["group", "iam", "detail", selectedGroupId, selectedGroup?.owner],
    // 跨组织树视图下详情 GET 必须携带行所属组织，否则回落会话组织 → 404。
    queryFn: () => getGroup(selectedGroupId!, selectedGroup?.owner),
    enabled: !!selectedGroupId,
  });

  const fullTree = useMemo(() => {
    const renderTitle = (group: GroupItem) => (
      <span>
        <Typography.Text strong>{group.displayName || group.name}</Typography.Text>
        {!group.isEnabled && (
          <Typography.Text type="secondary">{tr("group.treeDisabledSuffix")}</Typography.Text>
        )}
        {group.type && (
          <Typography.Text type="secondary">
            {tr("group.treeTypeSuffix", { type: group.type })}
          </Typography.Text>
        )}
      </span>
    );
    return buildTree(groupList, renderTitle);
  }, [groupList, tr]);

  const treeData = useMemo(() => filterTree(fullTree, filterText), [fullTree, filterText]);

  const handleSelect = (selectedKeys: React.Key[]) => {
    const key = selectedKeys[0];
    const nextGroupId = typeof key === "string" && key ? key : undefined;
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        if (nextGroupId) params.set("group", nextGroupId);
        else params.delete("group");
        return params;
      },
      { replace: true },
    );
  };

  if (isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 8 }} />
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Alert
          type="error"
          showIcon
          title={tr("group.treeLoadError")}
          description={localizedError(error, tc, "error")}
        />
      </Card>
    );
  }

  const detail = detailQuery.data;
  const members = detail?.users ?? [];

  return (
    <div className="gl-page">
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={10}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={tr("group.treeTitle")}
            extra={
              <Space>
                <PermissionGuard action="group:write" resource="groups" fallback={null}>
                  <LinkButton size="small" type="primary" to="/groups/new">
                    {tr("group.newGroup")}
                  </LinkButton>
                </PermissionGuard>
                <Typography.Link onClick={() => navigate("/groups")}>
                  {tr("group.switchToList")}
                </Typography.Link>
              </Space>
            }
          >
            {groupList.length === 0 ? (
              <Empty description={tr("group.noGroups")} />
            ) : (
              <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
                <Input
                  allowClear
                  prefix={<SearchOutlined />}
                  placeholder={tr("group.searchPlaceholder")}
                  aria-label={tr("group.searchPlaceholder")}
                  value={filterText}
                  onChange={(event) => setFilterText(event.target.value)}
                />
                {treeData.length === 0 ? (
                  <Empty description={tr("group.noGroups")} />
                ) : (
                  <Tree
                    // 过滤后重建树：让 defaultExpandAll 重新展开保留下来的祖先链。
                    key={filterText}
                    defaultExpandAll
                    selectedKeys={selectedGroupId ? [selectedGroupId] : []}
                    treeData={treeData}
                    onSelect={handleSelect}
                  />
                )}
              </Space>
            )}
          </Card>
        </Col>
        <Col xs={24} xl={14}>
          <Card
            className="gl-panel gl-resource-card"
            variant="borderless"
            title={
              selectedGroup
                ? tr("group.detailTitle", {
                    name: selectedGroup.displayName || selectedGroup.name,
                  })
                : tr("group.memberUsers")
            }
            extra={
              selectedGroup ? (
                <PermissionGuard action="group:write" resource="groups" fallback={null}>
                  <LinkButton
                    size="small"
                    to={`/groups/${selectedGroup.owner}/${selectedGroup.name}`}
                  >
                    {tc("action.editDetails")}
                  </LinkButton>
                </PermissionGuard>
              ) : undefined
            }
          >
            {!selectedGroup ? (
              <Empty description={tr("group.treeSelectPrompt")} />
            ) : detailQuery.isLoading ? (
              <Skeleton active paragraph={{ rows: 6 }} />
            ) : detailQuery.isError ? (
              <Alert
                type="error"
                showIcon
                title={tr("group.treeLoadError")}
                description={localizedError(detailQuery.error, tc, "error")}
                action={
                  <Button size="small" onClick={() => detailQuery.refetch()}>
                    {tc("retry")}
                  </Button>
                }
              />
            ) : (
              <>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label={tc("name")}>{selectedGroup.name}</Descriptions.Item>
                  <Descriptions.Item label={tc("organization")}>
                    {selectedGroup.owner}
                  </Descriptions.Item>
                  <Descriptions.Item label={tr("group.type")}>
                    {selectedGroup.type || "-"}
                  </Descriptions.Item>
                  <Descriptions.Item label={tc("statusLabel")}>
                    {selectedGroup.isEnabled ? (
                      <Tag color="green">{tc("enabled")}</Tag>
                    ) : (
                      <Tag color="red">{tc("disabled")}</Tag>
                    )}
                  </Descriptions.Item>
                </Descriptions>
                <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
                  {tr("group.treeReadOnlyHint")}
                </Typography.Paragraph>
                <Divider style={{ margin: "12px 0" }} />
                <Typography.Text strong>
                  {tr("group.memberUsers")} ({members.length})
                </Typography.Text>
                <div style={{ marginTop: 8 }}>
                  {members.length === 0 ? (
                    <Typography.Text type="secondary">{tr("group.noMembers")}</Typography.Text>
                  ) : (
                    <Space wrap>
                      {members.map((member) => (
                        <Tag key={member}>{member}</Tag>
                      ))}
                    </Space>
                  )}
                </div>
              </>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
