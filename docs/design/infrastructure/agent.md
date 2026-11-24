# Agent 运行时与注册中心

> 设计锚点：[DESIGN:infrastructure/agent]

> 目标态规范「Agent 运行时与注册中心」；写作约束见 [docs/README.md](../../README.md)。能力族开放项：OPEN-API-83。

---

## 功能定位与边界

Agent 是 GuliangIAM 与外部身份基础设施之间的**运行时代理**，承担跨网络的目录同步、MCP 工具桥接、协议端点挂载等工作。GuliangIAM 本体以注册中心方式管理 Agent 元数据，Agent 自身作为进程在外部部署，通过 HTTP 令牌向 GuliangIAM 上报存活与执行结果。

| 维度 | 说明 |
|------|------|
| **是什么** | 注册中心（Agent 实例的 CRUD + 令牌下发）；运行时心跳接收；出站代理跳板（MCP 工具调用、同步作业调度） |
| **不是什么** | 不在 GuliangIAM 进程内启动或管理子进程（放弃嵌入式 sidecar 进程化方案）；不承担认证/授权主链路——授权仍由 Casbin + ReBAC 决策 |
| **与 ADR-029 区分** | ADR-029 Identity Actions 是认证链路内同步扩展点（WASM/沙箱，宿主进程内执行）；Agent 是**跨进程/跨网络**的外部代理，定位完全不同 |

**在 GuliangIAM 系统中的角色**：

```
┌─────────────┐    HTTP + Token    ┌──────────────┐    MCP / LDAP / 自研协议    ┌──────────────┐
│ GuliangIAM  │ ──────────────────► │ Agent 进程    │ ───────────────────────────► │ 外部基础设施   │
│ （注册中心） │ ◄────────────────── │ （外部部署）  │ ◄────────────────────────── │ （MCP Server/ │
└─────────────┘   心跳 / 执行上报   └──────────────┘                              │  目录服务）    │
                                                                                 └──────────────┘
```

**Agent 类型枚举**（`agents.type`，列声明见 `contracts/schema/20_identity.yaml`）：

| Type | 职责 | 事实源 |
|------|------|-------------------|
| `gateway-proxy` | MCP 工具代理，将 MCP Server 注册为 GuliangIAM 可调用的工具集 | 元数据登记见 `internal/app/agents/`；MCP 代理实现见 `internal/infra/mcp/proxy.go`（与注册中心的整合为本文目标态设计） |
| `sync-agent` | 目录同步代理（LDAP/AD 增量同步），承接 `syncers` 出站作业 | 出站同步链路见 [integration.md](integration.md) |
| `protocol-endpoint` | 在 Agent 侧挂载协议端点（如 SCIM v2 / RADIUS / CAS SP），GuliangIAM 本体不直接暴露 | 协议端点契约见 [integration.md](integration.md) |

## 接口规范

### 管理面 CRUD（GuliangIAM → 管理面用户）

| 方法 | 端点 | OperationID | 权限 | 事实源 |
|------|------|-------------|------|-------------------|
| GET | `/api/v1/agents` | ListAgents | `agents:read` | 见 `internal/transport/http/api/v1/agents/routes.go` |
| POST | `/api/v1/agents` | CreateAgent | `agents:write` | 见 `internal/transport/http/api/v1/agents/routes.go` |
| GET | `/api/v1/agents/{id}` | GetAgent | `agents:read` | 见 `internal/transport/http/api/v1/agents/routes.go` |
| PATCH | `/api/v1/agents/{id}` | PatchAgent | `agents:write` | 见 `internal/transport/http/api/v1/agents/routes.go` |
| DELETE | `/api/v1/agents/{id}` | DeleteAgent | `agents:write` | 见 `internal/transport/http/api/v1/agents/routes.go` |

**事实源**：端点、方法、权限一律以 `internal/transport/http/api/v1/agents/routes.go` 的 `route.Definition` 声明为准；请求/响应 schema 由 `gen_openapi.py` 从 Go struct 反射生成。

### Agent 上报接口（Agent 进程 → GuliangIAM，目标态契约）

> 上报链路为本文目标态设计；运行期路由以 `internal/transport/http/api/v1/agents/routes.go` 声明为准（管理面 CRUD 之外不含上报端点）。

| 方法 | 端点 | 用途 | 鉴权 | 事实源 |
|------|------|------|------|-------------------|
| POST | `/api/v1/agents/{id}/ping` | 存活心跳 + 能力上报，写入 `agents.last_ping_at` | `Authorization: Bearer {agent_token}` | 目标态设计（本文） |
| POST | `/api/v1/agents/{id}/events` | 执行事件上报（同步完成/失败、工具调用结果） | 同上 | 目标态设计（本文） |
| GET | `/api/v1/agents/{id}/commands` | Agent 拉取调度命令（同步作业、工具调用请求） | 同上 | 目标态设计（本文） |

### Agent 令牌（`agents.token`）

- **来源**：token 由**客户端在 Create/PATCH 请求中提交**，服务端按提交值存储（`internal/app/agents/service.go` 的 CreateAgent/`Token: c.Token`）；服务端不生成、不覆盖客户端 token。令牌的随机性由 Agent 部署方负责（建议 `crypto/rand` ≥ 32 字节随机值）。
- **静态加密**：token 为机密列（DB-058），落库经字段加密映射器 `agentEncryptedMapper`（`internal/infra/postgres/agents/repository.go`，AES-256-GCM 密文 + AAD 绑定表/列/主键）；密钥治理见 [security-model.md](../../architecture/security-model.md) §2.7（AES-256-GCM 字段加密）与 `contracts/registries/crypto.yaml`；`ENCRYPTION_KEY_CURRENT` 缺失时所有环境拒绝启动，无明文降级路径（ADR-021）。
- **响应脱敏**：API 响应中 token 仅展示前 4 + 后 4 位，中间 `****` 掩码（`internal/app/agents/service.go` 的 toListItem）。
- **轮换**：PATCH Agent 提交新 token 完成轮换；轮换审计口径见 [security-model.md](../../architecture/security-model.md) §2.13。
- **废弃**：Agent 被 Delete 或 IsEnabled=false 后，旧 token 立即失效；Agent 进程需在 ≤ `30s` 内感知（由 ping 返回 401 触发客户端重新注册，目标态契约）。

### 速率限制

Agent 上报端点不单独配置限流策略，继承 `DefaultAPIRateLimit`（100 r/s，每 IP）。Agent 进程侧必须实现客户端退避（指数退避 + 抖动，上限见 limits.yaml），避免 GuliangIAM 重启后瞬时重试风暴。

## 通信协议

### HTTP 核心（REST + Bearer Token）

Agent 与 GuliangIAM 之间默认走 HTTPS（scheme 白名单 `https`，开发环境允许 `http` 并在启动日志标注，与 integration.md §SSRF 出口管控一致）。请求/响应 JSON，时间戳 ISO 8601 UTC。

### 流式 SSE（目标态）

Agent 调度命令的**下行**推送采用 SSE（复用 integration.md §控制台实时事件流规范）：

- 端点：`GET /api/v1/agents/{id}/stream`（Agent 进程 → GuliangIAM 建立长连）
- Content-Type：`text/event-stream`
- 事件类别：`event: command`（调度命令）、`event: ping`（心跳保持，30s 间隔）、`event: revoke`（令牌撤销通知）
- 续传：`Last-Event-ID` 从版本点续传；Agent 进程断线重连由客户端处理
- **不采用 WebSocket**——IAM 场景以单向推送为主，SSE 复用 HTTP 认证与 CORS 且客户端自带重连（同 integration.md §SSE 选型理由）

### 消息格式

心跳（Agent → GuliangIAM）：

```json
{
  "agent_id": "uuid-v7",
  "version": "1.2.3",
  "capabilities": ["gateway-proxy:mcp", "sync:ldap-incremental"],
  "system_info": { "os": "linux", "arch": "amd64" },
  "load": { "cpu": 0.23, "mem_mb": 512 }
}
```

调度命令（GuliangIAM → Agent）：

```json
{
  "id": "uuid-v7",
  "type": "sync.schedule",
  "payload": { "job_id": "...", "connector": "ldap://..." },
  "timeout_ms": 30000,
  "issued_at": "2026-10-11T08:00:00Z"
}
```

执行事件（Agent → GuliangIAM）：

```json
{
  "id": "uuid-v7",
  "command_id": "...",
  "status": "succeeded | failed",
  "result": { "...": "..." },
  "error": { "code": "LDAP_TIMEOUT", "message": "..." },
  "duration_ms": 1823
}
```

## 部署模式

### 嵌入式单体（不采用）

**放弃**在 GuliangIAM 主进程内启动 Agent 子模块。理由：Agent 类型差异巨大（LDAP/SCIM/MCP/RADIUS 各自引入重量级依赖），混入主进程导致二进制膨胀、启动时延与故障域扩大。当前代码中 `agents` 包仅为注册中心元数据管理，不含运行时。

### Sidecar 容器（推荐）

Agent 进程以独立容器与业务容器同 Pod 部署（Kubernetes），共享网络命名空间，通过 `localhost` 与 GuliangIAM 通信。适用场景：

- 低延迟要求（LDAP 目录增量同步、RADIUS 协议端点）
- 需要与业务容器共享本地卷（如临时文件缓存）
- 单租户独占 Agent（token 生命周期 = Pod 生命周期）

### 独立进程（推荐）

Agent 进程独立部署，跨网络通过 HTTPS 与 GuliangIAM 通信。适用场景：

- MCP Gateway Proxy（跨集群多租户共享）
- 受控网络环境下的同步代理（DMZ、零信任架构）
- 与第三方系统有长时间保持连接需求（如 LDAP persistent search）

**部署模式选择矩阵**：

| 场景 | Sidecar | 独立进程 | 说明 |
|------|---------|---------|------|
| MCP Gateway Proxy | ⚠️ | ✅ | 通常跨集群多租户共享 |
| LDAP/AD 增量同步 | ✅ | ✅ | 取决于与目录服务的网络距离 |
| SCIM v2 端点挂载 | ✅ | ⚠️ | SCIM 端点通常在 DMZ 暴露 |
| RADIUS 端点 | ✅ | ✅ | UDP 协议限制决定了部署位置 |

## 运维指南

### 健康检查端点

| 端点 | 响应 | 说明 | 事实源 |
|------|------|------|-------------------|
| `GET /health` | `{"status":"ok","version":"..."}` | GuliangIAM 本体健康检查 | 见 `internal/bootstrap/chi.go` |
| `GET /api/v1/agents/{id}/health` | 代理到 Agent 进程侧 `/health`，透传状态码 | Agent 健康检查由 GuliangIAM 代理，避免调用方持有 Agent 地址 | 目标态设计（本文） |
| `GET /ready` | readiness probe | 区分启动中/就绪/不可用 | 见 `internal/bootstrap/chi.go` |

Agent 进程侧健康检查（**由 Agent 实现，GuliangIAM 只负责代理**）：

| 检查项 | 标准 |
|--------|------|
| 令牌有效性 | 向 GuliangIAM `/api/v1/agents/{id}/ping` 能返回 200 |
| 外部依赖可达 | LDAP/MCP Server 端点能完成一次轻量探测 |
| 自身资源 | CPU 使用率 < 80%，内存 < 配额 |

### 日志字段规范

Agent 相关日志事件（GuliangIAM 侧）必须携带：

| 字段 | 取值 | 说明 |
|------|------|------|
| `trace_id` | OpenTelemetry trace ID | 全链路关联 |
| `organization_id` | Agent 所属租户 | 多租户隔离 |
| `agent_id` | Agent UUID | 资源定位 |
| `agent_type` | `gateway-proxy` / `sync-agent` / ... | 类型区分 |
| `request_id` | HTTP 请求 ID | 单次调用定位 |

Agent 进程侧日志（**由 Agent 自身规范**）：

| 字段 | 说明 |
|------|------|
| `agent_id` + `version` | 自报身份与版本 |
| `peer_addr` | GuliangIAM 实例地址 |
| `correlation_id` | 从 GuliangIAM 下发的 command/event ID |
| `dep_addr` | 外部依赖（LDAP/MCP）地址 |

### Prometheus 指标

GuliangIAM 本体导出的 Agent 相关指标（目标态）：

| 指标名 | 类型 | 标签 | 说明 | 事实源 |
|--------|------|------|------|-------------------|
| `guliangiam_agents_total` | Gauge | `organization_id`, `type`, `is_enabled` | 注册的 Agent 实例数 | 目标态设计（本文） |
| `guliangiam_agents_last_ping_seconds` | Gauge | `agent_id` | 距上次心跳的秒数（>30s 视为离线） | 目标态设计（本文） |
| `guliangiam_agent_ping_received_total` | Counter | `agent_id` | 心跳接收计数 | 目标态设计（本文） |
| `guliangiam_agent_event_received_total` | Counter | `agent_id`, `status` | 执行事件上报计数 | 目标态设计（本文） |
| `guliangiam_agent_command_dispatched_total` | Counter | `agent_id`, `type` | 调度命令下发计数 | 目标态设计（本文） |
| `guliangiam_mcp_tools_calls_total` | Counter | `server_id`, `tool`, `status` | MCP 工具调用次数（复用 gateway 指标路径） | MCP 代理独立计数（见 `internal/infra/mcp/proxy.go`） |
| `guliangiam_mcp_tools_calls_duration_seconds` | Histogram | `server_id`, `tool` | MCP 工具调用耗时 | MCP 代理独立计数 |

**Agent 进程侧指标由 Agent 自身暴露并由 Prometheus 抓取**，GuliangIAM 不直接采集。

## 异常处理

### 超时与重试

| 链路 | 超时 | 重试策略 | 说明 |
|------|------|---------|------|
| GuliangIAM → Agent（命令下发） | 10s | 指数退避 `2^n`，上限 300s，±20% 抖动 | 复用 outbox.md §Webhook 投递参数 |
| Agent → GuliangIAM（心跳） | 3s | 固定 30s 间隔，连续 3 次失败后 Agent 自标 unhealthy | 心跳不阻塞主链路，失败仅告警 |
| Agent → 外部依赖（LDAP/MCP） | 30s | Agent 内部实现，GuliangIAM 不感知单次失败 | 结果通过 `events` 端点上报 |
| MCP 工具调用（GuliangIAM → MCP Server） | 30s | SSRF 校验前置；失败返回 502，客户端决定重试 | 由 `infra/mcp/proxy.go` 实现 |

### 降级

| 场景 | 降级行为 |
|------|---------|
| Agent 全部离线 | 管理面 CRUD 仍可用（元数据只读）；依赖 Agent 的功能同步返回 `503 service unavailable` 并标注 `Retry-After` |
| MCP Proxy 未配置 | `ListMCPTools` / `CallMCPTool` 返回 `Unavailable`（实现位置见 `internal/app/agents/service.go`） |
| Redis 限流失败 | Agent 上报端点降级为进程内本地限流，`limit` 保守设为原分布式值的 50% |
| GuliangIAM 重启 | Agent 心跳连续失败后自退；GuliangIAM 恢复后 Agent 需在 1min 内重连 |

### 熔断

**熔断主体 = Agent 进程**（GuliangIAM 不做跨进程熔断，因为它只是注册中心）。Agent 进程对外部依赖（LDAP/MCP）自实现熔断器：

| 阈值 | 行为 |
|------|------|
| 连续 5 次超时/失败 | 进入 OPEN 状态，快速失败（立即返回 503） |
| OPEN 状态 30s 后 | 进入 HALF-OPEN，放行 1 次探测请求 |
| 探测成功 → CLOSE；探测失败 → OPEN | — |

**GuliangIAM 侧熔断（对 Agent 上报端点）**：不做熔断——Agent 上报是低频（心跳 30s/次，事件按需），不需要熔断保护；仅通过 429 限流 + 503 降级处置。

### 离线判定

`last_ping_at` 超过 90s（3× 心跳间隔）的 Agent 视为**离线**，GuliangIAM 在 ListAgents 响应中用 `StateTag` 标注（见 `web/src/pages/agents/`）；离线 Agent 在 24h 内未自行恢复，GuliangIAM 自动置 `is_enabled=false` 并发送审计事件。

## 数据模型

### `agents` 表

列、类型、默认值、约束与索引的唯一事实源为 `contracts/schema/20_identity.yaml` 的 `guliangiam.agents` 声明（复现：`python3 scripts/gen_schema.py --check`），本文不复制字段清单。语义要点：

- 租户隔离：`organization_id` + RLS；`name` 在租户内唯一（`uq_agents_org_name`）。
- `type` 默认 `gateway-proxy`；`token` 为机密列（静态加密与脱敏见上文「Agent 令牌」）。
- `application_id` 可选关联应用（OAuth Client 绑定）；`last_ping_at` 记录心跳时间戳；`revision` 乐观锁。

**事实源**：DDL 由 `contracts/schema/*.yaml` 经生成器产出，仓库规范见 docs/README.md。

## 边界与复用

- 复用 `infra/mcp/proxy.go` 现有 MCP 代理能力——目标态将其整合进 `gateway-proxy` 类型 Agent，而非平行存在
- 复用 `outbox.md` 重试/退避/SSRF 出口管控参数（GuliangIAM → Agent 出站 HTTP 全覆盖）
- 复用 `integration.md` §SSE 流式规范与 Discovery 端点
- 复用 `security-model.md` 密钥轮换审计口径
- **不复用 ADR-029 WASM 沙箱**——Agent 是独立进程，不在宿主内执行第三方代码

## 互链

- MCP 代理实现 → `internal/infra/mcp/proxy.go`
- MCP Gateway Service → `internal/app/gateway/service.go`
- 同步器 → `docs/design/infrastructure/integration.md`（LDAP/Syncer 出站）
- Outbox 投递 → `docs/design/infrastructure/outbox.md`
- SSRF 出口管控 → `docs/design/infrastructure/integration.md`
- 审计哈希证据链 → `docs/design/infrastructure/audit.md`
- Identity Actions 沙箱 → `docs/adr/ADR-029-identity-actions-sandbox.md`
- 安全模型（token 加密与轮换）→ `docs/architecture/security-model.md`
- 前端页面 → `docs/design/frontend/registry.md` §Agents 路由
