import {
  App,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Popconfirm,
  Result,
  Select,
  Skeleton,
} from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  getGatewayServers,
  getGatewayServer,
  updateGatewayServer,
  deleteGatewayServer,
  type GatewayServerDetail,
} from "@/shared/api/iam/gateway";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { FormSection } from "@/shared/ui/FormSection";
import { NotFoundResult } from "@/shared/ui/QueryErrorResult";
import { GlFormRow } from "@/shared/ui/GlForm";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { localizedError } from "@/shared/lib/server-error";

type ServerFormValues = {
  name: string;
  endpoint: string;
  protocol: string;
  status: string;
};

export default function ServerDetailPage() {
  const { organizationName, serverName } = useParams();
  const account = useSessionStore((state) => state.account);
  const { message } = App.useApp();
  const { t: tc } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ServerFormValues>();
  const { nameMap: orgNameMap } = useOrganizationNames();

  const listQuery = useQuery({
    queryKey: ["gateway-servers", "auth"],
    queryFn: () => getGatewayServers(),
    enabled: account !== null,
  });

  const matchedServer = listQuery.data?.find(
    (s) => s.organizationId === organizationName && s.name === serverName,
  );

  const detailQuery = useQuery({
    queryKey: ["gateway-server", "auth", "detail", matchedServer?.id],
    queryFn: () => getGatewayServer(matchedServer!.id),
    enabled: !!matchedServer?.id,
  });

  const server = detailQuery.data;

  const updateMutation = useMutation({
    mutationFn: async (values: ServerFormValues) => {
      return updateGatewayServer(matchedServer!.id, {
        name: values.name,
        endpoint: values.endpoint,
        protocol: values.protocol,
        status: values.status,
        revision: server!.revision,
      });
    },
    onSuccess: (_result, values) => {
      message.success(tc("updateSuccess"));
      const renamed = values.name !== serverName;
      if (renamed) {
        // 名称路由：先把列表缓存里的名字同步改掉，避免 invalidate 重取窗口内
        // 新名称匹配不到旧缓存而闪出 NotFound。
        queryClient.setQueryData<GatewayServerDetail[]>(["gateway-servers", "auth"], (prev) =>
          prev?.map((s) => (s.id === matchedServer?.id ? { ...s, name: values.name } : s)),
        );
      }
      queryClient.invalidateQueries({ queryKey: ["gateway-servers"] });
      // 详情键是单数 ["gateway-server",…]，不失效它 revision 陈旧 → 二次保存 409。
      queryClient.invalidateQueries({ queryKey: ["gateway-server"] });
      if (renamed) {
        navigate(`/servers/${organizationName}/${encodeURIComponent(values.name)}`, {
          replace: true,
        });
      }
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteGatewayServer(matchedServer!.id),
    onSuccess: () => {
      message.success(tc("deleteSuccess"));
      queryClient.invalidateQueries({ queryKey: ["gateway-servers"] });
      navigate("/servers");
    },
    onError: (error) => {
      message.error(localizedError(error, tc, "error"));
    },
  });

  if (listQuery.isLoading || detailQuery.isLoading) {
    return (
      <Card className="gl-panel gl-resource-card" variant="borderless">
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (!matchedServer) {
    return <NotFoundResult title={tc("notFound")} description={tc("notFoundDesc")} />;
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

  if (detailQuery.isError) {
    return (
      <Result
        status="error"
        title={tc("loadFailed")}
        subTitle={String(detailQuery.error)}
        extra={
          <Button type="primary" onClick={() => detailQuery.refetch()}>
            {tc("retry")}
          </Button>
        }
      />
    );
  }

  return (
    <ResourceFormPage
      title={tr("gateway.editMcpServer")}
      description={tr("gateway.editMcpServerDescription")}
      saving={updateMutation.isPending}
      form={form}
      onCancel={() => navigate("/servers")}
      onFinish={async (values: ServerFormValues) => {
        await updateMutation.mutateAsync(values);
      }}
    >
      {server && (
        <>
          <FormSection title={tc("overview")}>
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="ID">{server.id}</Descriptions.Item>
              <Descriptions.Item label={tc("organizationId")}>
                {orgNameMap.get(server.organizationId) || server.organizationId}
              </Descriptions.Item>
              <Descriptions.Item label={tr("gateway.revision")}>
                {server.revision}
              </Descriptions.Item>
            </Descriptions>
          </FormSection>
          <FormSection title={tr("basicInfo")}>
            <Form.Item name="name" initialValue={server.name} rules={[{ required: true }]}>
              <GlFormRow label={tc("name")} required>
                <Input />
              </GlFormRow>
            </Form.Item>
            <Form.Item name="endpoint" initialValue={server.endpoint} rules={[{ required: true }]}>
              <GlFormRow label={tc("endpoint")} required>
                <Input />
              </GlFormRow>
            </Form.Item>
            <Form.Item name="protocol" initialValue={server.protocol} rules={[{ required: true }]}>
              <GlFormRow label={tr("gateway.protocol")} required>
                <Select>
                  <Select.Option value="sse">SSE</Select.Option>
                  <Select.Option value="streamable-http">Streamable HTTP</Select.Option>
                  <Select.Option value="stdio">Stdio</Select.Option>
                </Select>
              </GlFormRow>
            </Form.Item>
            <Form.Item name="status" initialValue={server.status}>
              <GlFormRow label={tc("statusLabel")}>
                <Select>
                  <Select.Option value="active">{tr("gateway.active")}</Select.Option>
                  <Select.Option value="disabled">{tc("disabled")}</Select.Option>
                </Select>
              </GlFormRow>
            </Form.Item>
          </FormSection>
          <Popconfirm
            title={tr("gateway.confirmDeleteServer")}
            onConfirm={() => deleteMutation.mutate()}
            okText={tc("confirmDelete")}
            cancelText={tc("cancel")}
          >
            <Button danger loading={deleteMutation.isPending}>
              {tr("gateway.deleteServer")}
            </Button>
          </Popconfirm>
        </>
      )}
    </ResourceFormPage>
  );
}
