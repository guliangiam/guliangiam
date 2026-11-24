import { Button, Card, Col, Row, Skeleton, Empty, message } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  getOnlineMcpServers,
  createIamMcpServer,
  type OnlineMcpServer,
} from "@/shared/api/iam/mcp-server";
import { useSessionStore } from "@/features/session/session-store";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { QueryErrorResult } from "@/shared/ui/QueryErrorResult";
import { localizedError } from "@/shared/lib/server-error";

export default function McpStorePage() {
  const { t: tr } = useTranslation("resource");
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();

  const query = useQuery({
    queryKey: ["mcp-servers", "online"],
    queryFn: () => getOnlineMcpServers(),
    enabled: account !== null,
  });

  const installMutation = useMutation({
    // The registry exposes a single `name`; the console uses it as the display
    // name too, because `CreateMcpServerRequest.displayName` is a required
    // string and omitting it would store an empty one.
    mutationFn: (server: OnlineMcpServer) => {
      // 组织上下文缺失时明确报错，而不是把空 organizationId 提交给后端掩盖问题。
      const organizationId = organization?.id ?? account?.organization?.id;
      if (!organizationId) {
        return Promise.reject(
          new Error(tr("mcpStore.noOrganization", "No organization context available")),
        );
      }
      return createIamMcpServer({
        organizationId,
        name: server.name,
        displayName: server.name,
        url: server.url,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      messageApi.success(tr("mcpStore.installSuccess", "Server installed successfully"));
    },
    onError: (error) => {
      messageApi.error(localizedError(error, tr, "mcpStore.installFailed"));
    },
  });

  const servers = query.data ?? [];
  const categories = [...new Set(servers.map((s) => s.category))];

  return (
    <div className="gl-page">
      {contextHolder}
      <div
        style={{
          marginBottom: 16,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <h2 style={{ margin: 0 }}>{tr("mcpStore.title", "MCP Server Store")}</h2>
      </div>

      {query.isLoading ? (
        <div style={{ textAlign: "center", padding: 48 }}>
          <Skeleton active paragraph={{ rows: 6 }} />
        </div>
      ) : query.isError ? (
        <QueryErrorResult error={query.error} onRetry={() => query.refetch()} />
      ) : servers.length === 0 ? (
        <Empty description={tr("mcpStore.noServers", "No online servers available")} />
      ) : (
        categories.map((category) => (
          <div key={category} style={{ marginBottom: 24 }}>
            <h3 style={{ marginBottom: 12 }}>{category}</h3>
            <Row gutter={[16, 16]}>
              {servers
                .filter((s) => s.category === category)
                .map((server) => (
                  <Col key={server.name} xs={24} sm={12} md={8} lg={6}>
                    <Card
                      title={server.name}
                      size="small"
                      actions={[
                        <PermissionGuard
                          key="install"
                          action="mcp-server:write"
                          resource="mcp-servers"
                          fallback={null}
                        >
                          <Button
                            type="primary"
                            size="small"
                            loading={installMutation.isPending}
                            onClick={() => installMutation.mutate(server)}
                          >
                            {tr("mcpStore.install", "Install")}
                          </Button>
                        </PermissionGuard>,
                      ]}
                    >
                      <p
                        style={{
                          margin: 0,
                          /* WCAG 1.4.3 Contrast — replace #666 with compliant color (B-02) */ color:
                            "var(--gl-color-text-secondary, #595959)",
                          fontSize: 13,
                        }}
                      >
                        {server.description}
                      </p>
                      <p
                        style={{
                          margin: "8px 0 0",
                          fontSize: 12,
                          /* WCAG 1.4.3 Contrast — replace #999 with compliant color (B-01) */ color:
                            "var(--gl-color-text-tertiary, #8c8c8c)",
                        }}
                      >
                        {server.url}
                      </p>
                    </Card>
                  </Col>
                ))}
            </Row>
          </div>
        ))
      )}
    </div>
  );
}
