# AI 网关（aigateway）设计方案

> 设计锚点：[DESIGN:ai-gateway/README]

> **定位**：AI 网关域目标态设计规范。结构事实源 `contracts/schema/75_ai_gateway.yaml`；路由事实源 `internal/transport/http/api/v1/aigateway/routes.go`；权限点事实源 `contracts/registries/permissions.yaml`（aigateway 域）
> **关联 ADR**：ADR-034（部署形态：同进程 vs 独立数据面）、ADR-005（端口接口）、ADR-006（无 ORM）、ADR-009（密钥托管）、ADR-015（密钥轮换）、ADR-016（RLS）、ADR-022（声明式 Schema）、ADR-023（多数据库方言）、ADR-008/017（Outbox）

## 1. 设计前提：为什么不是单一 OpenAI 兼容面

当前 LLM 供应商格局已经发生变化——Anthropic Messages 已成为独立事实标准（Claude Code、Cline 原生使用），Gemini 的 `generateContent` 结构无法无损降维到 OpenAI 格式，部分国产模型同时暴露 OpenAI 和 Anthropic 协议面。如果坚持"单一 OpenAI 兼容面"，调用方使用原生 SDK 时会被迫做双重协议转换（调用方 SDK 协议 → OpenAI 兼容面 → 上游原生协议），引入语义损失。

因此本方案的设计原则是：**以 Canonical（内部规范）表示作为转换中枢，对外暴露多协议面，对内按供应商协议直连或适配**。

---

## 2. 目标与边界

### 2.1 AI Hub 管理面

供应商注册、物理模型、逻辑模型别名、路由策略、API Key、配额策略、价格版本、连通性测试、启停维护。

### 2.2 AI Gateway 数据面

多协议推理入口（OpenAI 兼容 / Anthropic 兼容 / Gemini 原生），鉴权、限流、路由、协议适配、流式透传、用量记录。

### 2.3 边界声明

- **不实现推理**（不含任何模型权重或本地推理引擎）
- **不缓存对话内容**（prompt/completion 正文不落库）
- **不代理非 LLM 协议**（仅 chat / embedding / tool_use / thinking）
- **不建计费模块**（支付走 GuliangPay 域，本域仅输出用量元数据）
- **不替代 agents/mcpservers 域**（三者拓扑关系见 §8.1）

### 2.4 能力清单

| 能力 | 说明 | 代码归属 |
|------|------|---------|
| Canonical 核心 | `CanonicalRequest` / `CanonicalResponse` / `CanonicalStreamEvent` 内部规范 | Domain 层 |
| 三协议面适配器 | OpenAI / Anthropic / Gemini 请求解析 → Canonical → 响应逆转换 | App + Infra 层 |
| 上游适配器 | OpenAICompat / AnthropicUpstream / GeminiUpstream / LocalModel（Ollama/vLLM） | Infra 层 |
| 路由引擎 | 别名解析 → 候选路由 → 健康过滤 → 负载均衡 → failover | App 层 |
| API Key 管理 | 哈希存储 + 作用域 + RPM/TPM 限制 | Domain + App 层 |
| 配额治理 | RPM/TPM/月度预算 + 多维限流 | App + Redis |
| 流式管道 | SSE 透传 + 事件格式转换 + 心跳 | Transport + Infra |
| 用量计量 | Token 提取 + 成本计算 + Outbox 投递 | App 层 |
| 熔断健康 | `llm_provider_health` 状态 + 半开探测 | App + 定时任务 |
| 工具调用链路 | CanonicalTool ↔ 三协议面 tool_use 格式 ↔ MCP Server 调用 | App 层 |

### 2.5 复用 GuliangIAM 现有能力

| 能力域 | 复用方式 | 直接/扩展 |
|--------|---------|----------|
| 多租户 + RLS | 所有 `llm_*` 表带 `organization_id` + RLS（ADR-016） | 直接 |
| Casbin 授权 | 新增权限点登记到 `permissions.yaml` | 扩展 |
| Outbox | 用量事件 `ai.usage.recorded`；配额超限 `ai.quota.exceeded` | 直接 |
| 审计 records | 供应商变更、密钥操作、配额调整高风险操作 | 直接 |
| encryption_keys | `api_key_sealed` 信封加密（ADR-009/015） | 直接 |
| 计量管道 | 用量事件 `ai.usage.recorded` 经 Outbox 消费者写入商业计量域 `metering_events` 作计费归集（详见 §14；桥接实现 `internal/app/aigateway/outbox_handlers.go`） | 扩展 |
| SSRF 防护 | 出站 HTTP 走 `ssrf.SafeHTTPClient` | 直接 |
| UoW | 管理面写操作同事务 | 直接 |
| 通知 | 配额超限/熔断触发通知 | 直接 |
| OTel + Prometheus | 指标 + Trace | 直接 |
| 前端设计系统 | Ant Design + 现有路由守卫 | 直接 |

---

## 3. 协议矩阵与供应商分组

### 3.1 对外协议面

| 协议面 | 路径前缀 | 路径示例 | 适配对象 |
|--------|---------|---------|---------|
| OpenAI 兼容 | `/api/v1/ai/faces/openai/` | `chat/completions`、`embeddings`、`models` | OpenAI SDK 生态、绝大多数国产模型 |
| Anthropic 兼容 | `/api/v1/ai/faces/anthropic/` | `chat/completions`、原生 `messages`（同一 Messages 语义，入站 body 自动识别）、`embeddings`、`models` | Anthropic Messages 语义调用方（含 Claude SDK 默认 `POST /v1/messages` 路径） |
| Gemini 原生 | `/api/v1/ai/faces/gemini/` | `models/{model}:generateContent`、`models/{model}:streamGenerateContent` | Google 官方 SDK、Gemini 生态 |

> **路径规范**：统一采用 `/api/v1/ai/faces/{protocol}/...`——`faces` 段显式表达"协议面"，避免管理面 `v1` 与协议 `v1` 的双版本冲突。需同步更新《开发规范手册》4.1.3 协议面家族表新增"AI 协议面"条目。
>
> **版本演进**：如需新增协议（如 `/faces/azure-openai/`），只需加一路径前缀和一组适配器，Canonical 核心不变。

### 3.2 对内供应商分组

| 分组 | 供应商 | 上游协议 | 适配策略 |
|------|--------|---------|---------|
| **A · OpenAI 兼容型** | OpenAI、DeepSeek、GLM、Kimi、Qwen、Seed、Ollama、vLLM | OpenAI Chat Completions | 直通适配器：Canonical ↔ OpenAI 格式 |
| **B · Anthropic 原生型** | Anthropic | Anthropic Messages | Anthropic 适配器：system 提升、content block 映射 |
| **C · Gemini 原生型** | Google Gemini | Gemini generateContent | Gemini 适配器：Canonical ↔ contents/parts |
| **D · 混合协议型** | DeepSeek V4 Pro、GLM Coding Plan | 同时支持 OpenAI + Anthropic | 按 `preferred_protocol` + 调用方协议面选择最优适配器 |

---

## 4. 核心架构：Canonical 内部规范

### 4.1 分层模型

```
┌─────────────────────────────────────────────────────────┐
│  Transport（协议面 HTTP 入口）                             │
│  OpenAI Face / Anthropic Face / Gemini Face              │
├─────────────────────────────────────────────────────────┤
│  App（协议适配器 + Canonical 核心 + 路由引擎）              │
│  ProtocolAdapters  │  CanonicalCore  │  RouterEngine     │
├─────────────────────────────────────────────────────────┤
│  Domain（实体 + Port 接口）                                │
│  providers / models / aliases / routes / api_keys /     │
│  quota_policies / price_versions / usage_logs           │
├─────────────────────────────────────────────────────────┤
│  Infra（上游适配器 + Repository 实现）                      │
│  UpstreamAdapters（OpenAICompat / AnthropicUpstream /   │
│  GeminiUpstream / LocalModel）                            │
│  Repository（infra/postgres/aigateway / infra/mysql/...）│
└─────────────────────────────────────────────────────────┘
```

### 4.2 Canonical 数据结构（Domain 层定义）

`CanonicalRequest` 作为所有协议的"汇点"和"源点"。Go 结构体字段用 CamelCase（符合 Go 惯例），JSON 序列化键用 snake_case（与 OpenAI 兼容格式对齐）——字段命名裁决：

| Go 字段名 | JSON 键 | 类型 | 语义 |
|-----------|---------|------|------|
| `ModelAlias` | `model_alias` | `string` | 逻辑模型别名（路由查找键） |
| `System` | `system` | `string` | 系统指令（统一为纯文本） |
| `Messages` | `messages` | `[]CanonicalMessage` | 对话历史（角色/内容块归一化） |
| `Tools` | `tools` | `[]CanonicalTool` | 工具声明 |
| `ToolChoice` | `tool_choice` | `*ToolChoiceConfig` | 工具选择策略（auto / required / none / specific） |
| `Stream` | `stream` | `bool` | 是否流式 |
| `Temperature` | `temperature` | `float64` | 采样温度（内部归一化到 [0,1]） |
| `MaxTokens` | `max_tokens` | `int` | 最大输出（统一用 OpenAI 语义，Anthropic `max_output_tokens` 在适配器层转换） |
| `Stop` | `stop` | `[]string` | 停止序列 |
| `Thinking` | `thinking` | `*ThinkingConfig` | 思考模式（budget tokens） |

`CanonicalMessage`：

| Go 字段名 | 语义 |
|-----------|------|
| `Role` | 归一化为 `user` / `assistant` / `tool` |
| `Content` | `[]CanonicalContentBlock`（type: text / image_url / tool_use / tool_result） |
| `ToolCallID` | 工具调用关联 ID |

`CanonicalStreamEvent`：

| 事件类型 | 语义 |
|---------|------|
| `message_start` | 流开始 |
| `content_delta` | 文本增量 |
| `tool_call_delta` | 工具调用增量 |
| `message_delta` | 顶层更新（usageMetadata 等） |
| `message_stop` | 流结束 |
| `ping` | 心跳（仅透传 Anthropic） |

### 4.3 协议适配器关键映射规则

**System 指令映射**：
- OpenAI → Anthropic：`messages[role=system]` 提取拼接为顶层 `system` 字段
- OpenAI → Gemini：`role=system` 提取为 `systemInstruction`
- Anthropic → OpenAI：顶层 `system` 转为 `messages` 首条 `role=system`
- Gemini → OpenAI：`systemInstruction` 转为 `role=system` 消息

**角色名映射**：
- Gemini `model` → Canonical `assistant` → OpenAI/Anthropic `assistant`
- OpenAI `developer` role → 并入顶层 `system`

**流式适配**：
- 对 Anthropic 协议面：保持命名事件格式（`event: content_block_delta`）
- 对 OpenAI 协议面：只输出 `data:` 行 + `[DONE]`
- 对 Gemini 协议面：输出 SSE `data: {json}` 格式

**Token 用量映射**：
- OpenAI: `usage.prompt_tokens` / `completion_tokens` / `total_tokens`
- Anthropic: `usage.input_tokens` / `output_tokens`
- Gemini: `usageMetadata.promptTokenCount` / `candidatesTokenCount` / `totalTokenCount`

---

## 5. 数据模型（contracts/schema/75_ai_gateway.yaml）

> **Schema 格式对齐**：声明格式与 `contracts/schema/70_commerce.yaml` 同源——顶级键为 `table_name` / `pk` / `columns` / `constraints` / `indexes` / `soft_delete` / `revision` / `rls` / `fk_constraints`，由 `scripts/gen_schema.py` 生成 migration；本文件不复制 YAML 声明示例。

### 5.1 数据模型事实源

表清单（`table_name`）、列、约束、索引、软删除、revision、RLS 与分区声明以 `contracts/schema/75_ai_gateway.yaml` 为准（复现：`python3 scripts/gen_schema.py --check`）；正文不复制表结构与 YAML 示例。

- **分区与 RLS 交互**：RLS 声明在父表并自动继承到所有分区；分区必须由声明式 Schema 生成，生成器尚不支持时先扩展生成器，禁止以手写迁移 DDL 绕过。
- **`llm_provider_health` 受 RLS 保护**：`rls: true`，谓词经 `provider_id` 关联 `llm_providers.organization_id` 判定租户归属（super admin 旁路），声明见 `contracts/schema/75_ai_gateway.yaml`。健康状态虽是供应商级快照，仍按供应商归属租户隔离，不存在 RLS 豁免。

---

## 6. Clean Architecture 四层定义

### 6.1 Domain 层（`internal/domain/aigateway/`）

```
internal/domain/aigateway/
├── provider.go              // LLM Provider 实体 + Validate + 状态迁移
├── model.go                 // 物理模型实体
├── model_alias.go           // 逻辑别名实体
├── route.go                 // 路由候选实体
├── api_key.go               // API Key 实体（含哈希生成/校验）
├── quota_policy.go          // 配额策略实体
├── price_version.go         // 价格快照实体
├── provider_health.go       // 熔断状态实体（healthy→open→half_open 状态机）
├── usage_log.go             // 用量日志实体（含成本计算）
├── canonical.go             // CanonicalRequest / CanonicalResponse / CanonicalStreamEvent
├── adapter.go               // UpstreamAdapter Port 接口
├── router.go                // RouterEngine Port 接口
└── repository.go            // 所有 Repository 接口
```

#### 核心 Port 接口

```go
// UpstreamAdapter 是上游协议适配器的统一接口（ADR-005 端口接口）。
type UpstreamAdapter interface {
    Name() string
    SupportedProtocol() string
    Chat(ctx context.Context, req *CanonicalRequest) (*CanonicalResponse, error)
    ChatStream(ctx context.Context, req *CanonicalRequest) (<-chan *CanonicalStreamEvent, error)
    Embedding(ctx context.Context, req *CanonicalRequest) (*CanonicalResponse, error)
}

// RouterEngine 是路由引擎的 Port 接口。
type RouterEngine interface {
    Resolve(ctx context.Context, alias string, req *CanonicalRequest) (*RouteResolution, error)
}

// ProviderRegistry 是上游适配器注册表（进程内单例）
type ProviderRegistry interface {
    Register(protocol string, adapter UpstreamAdapter)
    Get(protocol string) (UpstreamAdapter, bool)
    List() []string
}
```

### 6.2 App 层（`internal/app/aigateway/`）

```
internal/app/aigateway/
├── service.go                // 编排总入口
├── provider_service.go       // 供应商 CRUD + 连通性测试
├── model_service.go          // 物理模型 CRUD
├── alias_service.go          // 别名 CRUD
├── route_service.go          // 路由候选 CRUD
├── api_key_service.go        // API Key 管理（创建/哈希/校验/撤销）
├── quota_service.go          // 配额策略 + 限流检查
├── price_service.go          // 价格版本管理
├── usage_service.go          // 用量查询 + 成本聚合
├── canonical_service.go      // Canonical 转换（协议面 ↔ Canonical）
├── router_service.go         // 路由引擎实现
├── gateway_service.go        // 数据面推理编排
├── health_service.go         // 健康探测 + 熔断状态机
├── tool_service.go           // 工具调用链路（CanonicalTool ↔ 协议面 ↔ MCP）
├── dto.go                    // 应用层 DTO
├── mapper.go                 // DTO ↔ Domain Entity
└── new.go                    // Service 构造函数（依赖注入）
```

#### 关键编排：数据面 Gateway 流程

```go
// app/aigateway/gateway_service.go 核心流程（伪代码）
func (s *GatewayService) Chat(ctx context.Context, canonical *CanonicalRequest, face string) (*CanonicalResponse, error) {
    // 1. 鉴权（API Key 或会话）
    apiKey, err := s.authz.Authenticate(ctx, canonical)
    // 2. 配额预检（RPM/TPM/预算）
    if err := s.quota.Check(ctx, apiKey, canonical.ModelAlias); err != nil {
        return nil, ErrAIGWQuotaExceeded
    }
    // 3. 路由解析
    resolution, err := s.router.Resolve(ctx, canonical.ModelAlias, canonical)
    // 4. 熔断检查
    if resolution.Health.State == "open" {
        return nil, ErrAIGWProviderCircuitOpen
    }
    // 5. 上游调用（带 failover）
    result, attempt, failoverFrom, err := s.callWithFailover(ctx, canonical, resolution)
    // 6. 用量计量（同事务写 usage_logs + Outbox）
    s.usage.Record(ctx, canonical, resolution, result, attempt, failoverFrom)
    // 7. 返回（协议面适配器逆转换由 Transport 层负责）
    return result, nil
}
```

### 6.3 Infra 层（`internal/infra/`）

```
internal/infra/
├── postgres/aigateway/          // PostgreSQL Repository 实现
│   ├── sql.go                   // DDL 对应查询
│   ├── mapper.go                // Row ↔ Domain Entity
│   └── repository.go            // 实现 domain/aigateway.Repository
├── mysql/aigateway/             // MySQL Repository 实现（并行支持，ADR-023）
│   ├── sql.go
│   ├── mapper.go
│   └── repository.go
├── aiupstream/                  // 上游适配器实现
│   ├── openai_compat.go         // OpenAI 兼容上游（直连 OpenAI + DeepSeek + Qwen 等）
│   ├── anthropic_upstream.go    // Anthropic 原生 Messages API
│   ├── gemini_upstream.go       // Gemini generateContent API
│   ├── local_model.go           // Ollama / vLLM（OpenAI 兼容端点 + base_url 白名单）
│   ├── stream.go                // SSE 流式解析/转发工具
│   ├── factory.go               // 适配器工厂（按 provider_type + preferred_protocol 创建）
│   └── ssrf_client.go           // SafeHTTPClient 封装（复用 platform/ssrf）
└── aicache/                     // [规划态，未建成] 配置缓存（Redis + 本地缓存，TTL 30s，写路径主动失效），见 ADR-034
    └── config_cache.go          // [规划态] 落地前以 internal/infra/ 目录实测为准
```

### 6.4 Transport 层（`internal/transport/http/api/v1/aigateway/`）

```
internal/transport/http/api/v1/aigateway/
├── chi.go               // 路由编排（注册管理面 + 数据面路由）
├── handler.go           // 管理面 CRUD handler
├── handler_gateway.go   // 数据面推理 handler（三协议面入口）
├── handler_stream.go    // 流式 handler（SSE 透传）
├── handler_tool.go      // 工具调用 handler（可选，用于 tool_use 直接触发 MCP）
├── routes.go            // 返回 []route.Definition（含 permission）
├── request.go           // 请求结构体（三协议面各自定义）
├── response.go          // 响应结构体（含 OpenAPI 契约）
└── canonical_converter.go  // 请求 → CanonicalRequest / 响应 ← CanonicalResponse 转换
```

#### 管理面与数据面路由声明

管理面与数据面路由以 `internal/transport/http/api/v1/aigateway/routes.go` 的 `route.Definition`（方法、路径、权限点、API 面）为准；生成物为 `openapi/openapi.yaml` 的 `management: aigateway` 与 `protocol: aigateway` tag（复现：`python3 scripts/gen_openapi.py --check`）。正文不复制路由清单。

数据面按协议面路径族组织，路径族前缀登记于 `contracts/registries/surfaces.yaml`；三协议面路由的 `Permission` 字段为路由契约占位，实际鉴权由 handler 内的 `api_key_service.Authenticate` 完成，跳过 Casbin PEP。

#### API Key 鉴权规范

三协议面的 API Key 鉴权在 **handler 内** 完成（通过 `app/aigateway/api_key_service.Authenticate`），而非中间件层。具体：

1. 路由进入 handler 后，读取 `Authorization` 或 `x-api-key` 头信息；`Authorization` 支持 `Bearer <key>` 与裸 `<key>` 两种形态，两个头同时存在时 `Authorization` 优先；
2. 调用 `api_key_service.Authenticate(ctx, key)` 校验哈希匹配；
3. 校验通过后，跳过 Casbin PEP（数据面鉴权不走管理面权限点）；
4. 路由定义中 `Permission` 字段置为 `aigateway:chat`（路由契约占位，登记于 `contracts/registries/permissions.yaml` 的 aigateway 域 chat 资源）；
5. 鉴权失败返回 `ai_api_key_invalid` / `ai_api_key_revoked` / `ai_api_key_expired` 错误码。

| Header | 场景 | 协议面 |
|--------|------|--------|
| `Authorization: Bearer sk-xxx` | 通用 | 全部 |
| `Authorization: sk-xxx`（无 `Bearer` 前缀） | 直接携带 key 的客户端 | 全部 |
| `x-api-key: sk-xxx` | Anthropic SDK 兼容 | 全部 |

`x-goog-api-key` 不在读取集合内；Gemini 协议面同样使用上表头。

---

## 7. 工具调用（tool_use）完整流程


### 7.1 CanonicalTool 结构

```go
type CanonicalTool struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    InputSchema map[string]any `json:"input_schema"` // JSON Schema
    Allowed     bool           `json:"allowed,omitempty"`
    // 运行时绑定（由 mcpservers 域注入，不在 Canonical 核心）
    MCPTarget   *MCPBinding    `json:"-"`
}

type MCPBinding struct {
    McpServerID string  // mcpservers 域实体 ID
    ToolName    string  // MCP server 上的实际 tool 名（可能与 CanonicalTool.Name 不同）
}
```

### 7.2 CanonicalTool ↔ 三协议面格式映射

| Canonical | OpenAI | Anthropic | Gemini |
|-----------|--------|-----------|--------|
| `Tools[]` | `tools[]`（`type: function`, `function.name/description/parameters`） | `tools[]`（`name/description/input_schema`） | `tools[].functionDeclarations[]` |
| `ToolChoice` | `tool_choice`（`auto`/`required`/`none`/`{"function":{"name":...}}`） | `tool_choice`（`auto`/`any`/`tool`/`none` + `name`） | `toolConfig.functionCallingConfig.mode` |
| 工具调用响应 | `message.tool_calls[]`（`id/function.name/function.arguments`） | `content[].type=tool_use`（`id/name/input`） | `candidates[].content.parts[].functionCall` |
| 工具结果回传 | `role=tool, tool_call_id, content` | `content[].type=tool_result`（`tool_use_id/content`） | `content.parts[].functionResponse` |

### 7.3 工具调用运行时链路

```
[协议面入口]
  ↓ Canonical 转换
[CanonicalRequest + Tools[]]
  ↓ 路由解析（route.go 正常流程）
[上游调用 → 模型返回 tool_calls]
  ↓ 协议面逆转换
[CanonicalResponse.ToolCalls[]]
  ↓ 应用层判断
  │ 如果调用方请求了 tool_use 自动执行：
  │   └─ [tool_service.Execute()]
  │       ├─ 查询 MCPBinding → mcpservers 域
  │       ├─ 构造 MCP 协议请求
  │       ├─ infra/mcp/proxy.go 透传到 MCP Server
  │       ├─ 接收 MCP tool result
  │       └─ 封装为 CanonicalMessage.ToolResult
  │
  │ 如果调用方请求手动执行：
  │   └─ 返回 CanonicalToolCall[] 给调用方，调用方自行执行后回传
[CanonicalResponse → 协议面逆转换]
[响应返回]
```

**MCP 服务器对接**：复用现有 `internal/infra/mcp/proxy.go` 透传能力。CanonicalTool 的 `MCPBinding` 由 App 层的 `tool_service` 从 `mcpservers` 域查询注入——两个域通过 `application_id` 间接关联，Domain 层无直接依赖。

---

## 8. 前端设计

### 8.1 与既有 AI 相关域的整合决策

AI 网关与既有三个 AI 相关域的职责边界与整合决策（后端域目录保持独立，仅前端菜单入口并入同一顶级分组，后端 API 路径与权限点不变）：

| 既有域 | 前端菜单分组 | 实体核心字段 | 职责 | 与 AI 网关重叠度 | 整合决策 |
|--------|---------|-------------|---------|----------------|---------|
| **agents** | infraRoutes | `host, port, url, token, last_ping_at, application_id`（列声明见 `contracts/schema/20_identity.yaml`） | SA 型远程 AI 服务凭据管理（Agent 是 LLM 调用方的服务账号） | ⭐⭐⭐⭐ 运行时组件统一管理 | **迁入 AI 网关菜单组** |
| **mcpservers** | securityRoutes | `url, token, tools[], enabled` + `infra/mcp/proxy.go` 透传 | MCP 协议服务器注册 + 代理 tool_call（AI tool_use 场景直接依赖） | ⭐⭐⭐⭐⭐ 运行时依赖 | **迁入 AI 网关菜单组** |
| **gateway** (servers + sites) | supportRoutes | Server: `endpoint, protocol`；Site: `host, ssl, health_check` | 通用反向代理 / API 网关 | ⭐ 纯通用能力 | **保留原位** |

**整合原则**：后端 Domain 层保持独立目录，Clean Architecture 一个域一个目录原则不变；只是前端菜单入口合并到同一顶级分组，后端 API 路径和权限点零改动。

### 8.2 实体关系澄清（谁在上游谁在下游）

```
  调用方（下行）                    通道（侧链）                  被调用方（上行）
┌──────────────────┐          ┌──────────────────┐         ┌──────────────────┐
│   agents 域       │          │   mcpservers 域    │         │  aigateway 域      │
│                  │          │                    │         │  llm_providers    │
│ "谁在调用 AI      │─────────▶│ "工具调用通道"     │◀────────│ "AI 网关路由到     │
│  网关"            │  LLM 调用 │  MCP 协议透传      │ tool_use │  哪个供应商"       │
│ host/port/url    │          │ url + tools[]     │         │ base_url          │
│ token (SA 凭据)   │          │ token             │         │ api_key_sealed    │
└──────────────────┘          └──────────────────┘         └──────────────────┘
   身份维度                       能力维度                    供应商维度
   AI 网关客户端                  AI 网关扩展                 AI 网关后端
```

| 域 | 角色 | 注册什么 |
|----|------|---------|
| **agents** | AI 网关**调用方**身份凭据 | 远程 Agent 服务的地址和 SA token |
| **mcpservers** | AI 网关 **tool_use** 能力通道 | MCP 协议服务器端点 + tools 声明 |
| **aigateway.llm_providers** | AI 网关**上游供应商** | 实际 LLM 供应商 API Key 和端点 |

### 8.3 路由分组：`web/src/app/router/ai-routes.tsx`

`/ai` 顶级分组聚合三域路由：aigateway（providers 列表/新建/详情/测试、models、aliases、routes、api-keys、quotas、prices、usage 与 usage/stats、health）+ agents 域（列表/新建/详情）+ mcpservers 域（列表/新建/详情/store）；`/ai` 根路径重定向到 `/ai/providers`。路由、页面组件与菜单的唯一事实源为 `web/src/app/router/ai-routes.tsx` 与 `docs/design/frontend/registry.md` 生成块（复现：`python3 scripts/gen_frontend_registry.py --check`），本文不复制路由清单。

---

### 8.4 gateway 域（servers/sites）保留原位

**决策**：不并入 AI 网关菜单组。`gateway.Server` 是通用后端服务注册；`gateway.Site` 是前端站点（`host, ssl, cert_id, health_check, alert`）——可代理任何业务系统，不是 AI 特有。把"业务站点健康检查"和"AI 供应商熔断健康"放在同一菜单组会增加管理员心智负担。

例外：`gateway/app/mcp.go` 的 MCP 代理属于 tool_use 基础设施，目标态归入 aigateway 域；gateway 域的 servers/sites 保持独立。

### 8.5 前端文件契约

| 文件/模块 | 职责 | 门禁 |
|-----------|------|------|
| `web/src/app/router/ai-routes.tsx` | AI 网关路由组装 | `check-frontend-coverage.py` |
| `web/src/app/router/router.tsx` | 聚合 `...aiRoutes` | `check-ui-wiring.py` |
| `web/src/features/navigation/menu.tsx` | AI 网关顶级分组 + 子菜单 | `menu.test.ts` |
| `docs/design/frontend/README.md` | 菜单树目标态 | `check-frontend-coverage.py` |

---

## 9. 错误码登记

AI 网关错误码（`ai_*` 前缀：配额/熔断/API Key/别名解析/上游超时与错误/凭据解密）唯一事实源为 `contracts/registries/errors.yaml` 的 application 节（渲染层 `docs/generated/errors-registry.md`；复现：`python3 scripts/gen_registry_md.py && python3 scripts/check-registry-sync.py --strict`），本文不复制码表。语义要点：配额超限 429 + `Retry-After`；熔断 open/half_open 503 可重试；API Key 无效/吊销/过期 401 不可重试；上游超时 504、上游错误与路由解析失败 502 可重试。

---

## 10. 权限点登记

权限点唯一事实源为 `contracts/registries/permissions.yaml` 的 **aigateway 域**（渲染层 `docs/generated/permissions-registry.md`；复现：`python3 scripts/gen_registry_md.py`），本文不复制资源/动作清单。语义要点：

- 管理面资源：providers / models / aliases / routes / api_keys / quotas / prices（CRUD 归一为 read/write/delete；连通性测试与 API Key 吊销归 write）、usage / health（只读）。
- 数据面：chat 资源（read/write），路由 `Permission` 为 `aigateway:chat` 契约占位；实际鉴权由 handler 内 API Key 校验完成（§6.4），不走 Casbin PEP。
- 路由声明与注册表一致性由 `python3 scripts/check-registry-sync.py --strict` 门禁保证。

---

## 11. 状态机登记


### 11.1 熔断状态机（`llm_provider_health.state`）

状态闭集事实源：`contracts/schema/75_ai_gateway.yaml` 的 `ck_llm_health_state` CHECK（healthy/degraded/open/half_open）与 `internal/domain/aigateway/provider_health.go`；`contracts/registries/state-machines.yaml` 的 provider_health 条目须与该闭集一致（一致性由 `check-registry-sync.py` 门禁收敛）。

```yaml
aigateway_provider_health:
  states:
    healthy:
      transitions: [degraded, open]
    degraded:
      transitions: [healthy, open]
    open:
      transitions: [half_open]
    half_open:
      transitions: [healthy, open]
```

### 11.2 供应商状态枚举（`llm_providers.status`）

```yaml
aigateway_provider_status:
  values: [active, disabled, testing]
```

### 11.3 API Key 状态枚举（`llm_api_keys.status`）

```yaml
aigateway_api_key_status:
  values: [active, revoked, expired]
```

### 11.4 Provider Type 独立枚举

> **与 IAM 域 `providers` 表无复用关系**——登记为 `aigateway.provider_type` 命名空间。

```yaml
aigateway_provider_type:
  values: [openai, anthropic, gemini, deepseek, glm, kimi, qwen, seed, ollama, vllm, custom]
```

---

## 12. 命名空间登记


```yaml
canonical:
  fields:
    - model_alias
    - system
    - messages
    - tools
    - tool_choice
    - stream
    - temperature
    - max_tokens
    - stop
    - thinking

aigateway_paths:
  segment_faces: faces           # 协议面路径段：/api/v1/ai/faces/{protocol}/...
  protocol_families:             # 协议家族枚举
    - openai
    - anthropic
    - gemini

aigateway_enums:
  provider_type: openai|anthropic|gemini|deepseek|glm|kimi|qwen|seed|ollama|vllm|custom
  provider_status: active|disabled|testing
  api_key_status: active|revoked|expired
  health_state: healthy|degraded|open|half_open
```

---

## 13. 审计事件登记（Outbox + records）

事件名注册表为 `contracts/registries/events.yaml` 的 `ai` 命名空间；数据面发出的 `ai.usage.recorded`（`internal/app/aigateway/gateway_quota.go`）须登记于该命名空间（一致性由 `check-registry-sync.py` 门禁收敛）。


```yaml
ai:
  - event: ai.usage.recorded
    severity: info
    fields: [organization_id, api_key_id, alias_id, provider_id, model_id, input_tokens, output_tokens, cost_amount, currency, request_id]
    # Outbox 投递：Consumer 写入 llm_metering_events（AI 审计）+ metering_events（商业计费归集），见 internal/app/aigateway/outbox_handlers.go
  - event: ai.quota.exceeded
    severity: high
    fields: [organization_id, api_key_id, limit_type, current_usage, limit_value, request_id]
    # Outbox 投递：Consumer 触发通知
  - event: ai.provider.circuit_opened
    severity: high
    fields: [organization_id, provider_id, failure_count]
    # Outbox 投递：Consumer 触发通知 + 更新 llm_provider_health
  - event: ai.provider.circuit_closed
    severity: info
    fields: [organization_id, provider_id]
    # Outbox 投递：Consumer 更新 llm_provider_health
```

**records 同事务写入**（高风险操作）：

| 操作 | records 写入 | actor |
|------|-------------|-------|
| 供应商创建/更新/删除 | ✅ | 管理员 user_id / agent application_id |
| API Key 创建/撤销 | ✅ | 同上 |
| 配额策略变更 | ✅ | 同上 |
| 熔断状态迁移（open→closed 除外，自动） | ✅ | 系统（actor=system） |

---

## 14. Metering 对接


### 14.1 计量指标

| 指标名 | 单位 | 写入时机 | 说明 |
|--------|------|---------|------|
| `ai.tokens.input` | tokens | 每次推理完成后 | 输入 Token |
| `ai.tokens.output` | tokens | 每次推理完成后 | 输出 Token |
| `ai.requests` | count | 每次推理完成后 | 请求次数（成功/失败均计数） |
| `ai.cost` | micro_usd | 每次推理完成后 | 成本（微美元，DECIMAL(19,6) 换算） |

### 14.2 事件唯一 ID

`event_uid = sha256(request_id + attempt_no)`——`request_id` 是全链路 Trace ID，`attempt_no` 是调用尝试编号（failover 时 attempt_no=2+）。同一 `(request_id, attempt_no)` 组合保证幂等。

### 14.3 事件流

```
数据面推理完成
  ├─ 写 llm_usage_logs
  ├─ 写 Outbox (event_type='ai.usage.recorded', event_uid=sha256(...))
  └─ COMMIT（同事务）
        │
        ▼ Outbox Worker（internal/app/aigateway/outbox_handlers.go）
        ├─ llm_metering_events（AI 网关审计表，所有 aggregate_type）
        └─ metering_events（商业计费表，仅 llm_usage + ai.usage.recorded）
```

商业计量域边界见《[commerce/metering.md](../commerce/metering.md)》：Token 计数/配额/价格归本域，计费归集账期归商业计量域。

---

## 15. 成本计算口径


| 维度 | 规则 |
|------|------|
| 价格来源 | `llm_price_versions` 表快照（按 `model_id` + `effective_from` 选最新版本） |
| 成本计算 | `cost = (input_tokens / 1_000_000) * input_price_per_1m + (output_tokens / 1_000_000) * output_price_per_1m` |
| 币种 | 价格币种 ≠ 成本币种：按汇率换算为统一币种（默认 USD），汇率由日更新的 `exchange_rates` 配置提供 |
| 缓存 | [规划态] 价格快照缓存在 `aicache`（TTL 30s），写路径主动失效；该包未建成（ADR-034），当前价格读取无缓存层 |
| 失败请求 | 失败请求不写成本（`cost_amount = 0`），但仍写 Token 用量（如果上游返回了 usage） |
| failover | 多次尝试的成本按**最终成功的尝试**计算；失败尝试的成本单独计入但标记 `failover_from` |

---

## 16. 初始化数据（种子）

平台级共享供应商、物理模型与价格快照属于**引导数据**，在 `db/seeds/V1__guliangiam_seed_init.sql` 单文件内幂等追加（人工编写、外键用业务键子查询、自带冲突仲裁；禁止新增种子版本，见 AGENTS.md §2.6），不承载结构变更；租户级供应商由管理面创建并执行连通性测试。种子清单以 `db/seeds/` 与声明式 Schema 为准，不在文档固化 SQL。

## 17. 安全设计

| 维度 | 要求 |
|------|------|
| 密钥 | `api_key_sealed` 信封加密；写侧走 `encryption_keys` 状态机（ADR-009/015）；读侧仅 `api_key_fingerprint` + 密钥子集展示 |
| API Key | 仅存 SHA-256 哈希；展示前缀（如 `sk-abc***xyz`）；支持轮换与吊销 |
| SSRF | 出站 HTTP 走 `ssrf.SafeHTTPClient`；本地模型（Ollama/vLLM）`base_url` 白名单（127.0.0.1 / 10.0.0.0/8 / 192.168.0.0/16 / 172.16.0.0/12） |
| 日志脱敏 | 审计日志不记录 API Key 原文、不记录上游密钥原文；prompt/completion 正文不落库 |
| 配额 | 组织级 / API Key 级 / 别名级多维限流；超限返回 429 + `Retry-After` |
| 租户隔离 | 所有 `llm_*` 表受 RLS 保护；`llm_provider_health` 经 `provider_id` 关联供应商归属租户判定（`rls: true`，见 §5.1） |
| 熔断 | 连续失败触发 open → half_open 探测 → closed；熔断期间路由层自动 skip 该供应商 |
| Fail-close | API Key 哈希查询不可用时 → 保守拒绝；配额检查 Redis 不可用时 → 保守拒绝；RLS predicate 不可用时 → 保守拒绝 |

---

## 18. 可观测性

### 18.1 指标

| 指标 | 类型 | 标签（禁止高基数） |
|------|------|-------------------|
| `ai_gateway_requests_total` | Counter | `protocol_face`, `upstream_protocol`, `provider_type`, `status` |
| `ai_gateway_tokens_total` | Counter | `provider_type`, `direction`（input/output） |
| `ai_gateway_cost` | Counter | `provider_type`, `currency` |
| `ai_gateway_latency_seconds` | Histogram | `provider_type`, `streaming` |
| `ai_gateway_ttft_seconds` | Histogram | `provider_type` |
| `ai_gateway_failover_total` | Counter | `from_provider_type`, `to_provider_type`, `reason` |
| `ai_gateway_circuit_state` | Gauge | `provider_type` |
| `ai_gateway_quota_rejected_total` | Counter | `scope`, `limit_type` |

### 18.2 Trace

贯穿：协议面入口 → Canonical 转换 → 路由引擎 → 上游适配器 → 供应商 HTTP → 响应处理 → 计量。每个 span 携带 `request_id` / `provider_type` / `alias_id` / `protocol_face`。

### 18.3 日志

结构化 JSON，必含 `request_id` / `trace_id` / `span_id` / `organization_id`。prompt/completion 正文**禁止写入日志**（脱敏规则由 `platform/logging/sanitizer.go` 统一管理）。

---

## 19. 验收条件（AC-AIGW-001~010）


| AC 编号 | Given | When | Then | 门禁 |
|--------|-------|------|------|------|
| **AC-AIGW-001** | OpenAI SDK 客户端 | 调用 `POST /api/v1/ai/faces/openai/chat/completions`（alias=`gpt-4o`） | 返回 200 + 响应体符合 OpenAI 格式 + Token 用量正确 | `check-openapi-contract.sh` + 集成测试 |
| **AC-AIGW-002** | Anthropic Messages 语义客户端 | 调用 `POST /api/v1/ai/faces/anthropic/chat/completions` 或原生 `POST /api/v1/ai/faces/anthropic/messages`（alias=`claude-sonnet`） | 返回 200 + 响应体符合 Anthropic Messages 格式 | 集成测试（两路径同契约） |
| **AC-AIGW-003** | 同一请求带相同 `Idempotency-Key` | 重复调用 Chat Completions | 第二次返回缓存响应 + 响应头 `Idempotency-Replayed: true` | 集成测试 |
| **AC-AIGW-004** | 已配置 RPM 限制为 60 的 API Key | 1 秒内发送 61 次请求 | 第 61 次返回 429 + `Retry-After` | 集成测试 |
| **AC-AIGW-005** | 路由配置 provider A（权重 100）+ provider B（权重 100），A 熔断 open | 发起 Chat Completions | 路由自动 failover 到 B + `failover_from=provider_a` 写入 `llm_usage_logs` | 集成测试 |
| **AC-AIGW-006** | 三协议面同时注册 | 同一 alias 从 OpenAI 协议面调用时配置使用 Anthropic 供应商 | Canonical → Anthropic 适配器 → Anthropic Upstream 正确转换，无语义损失 | 集成测试（断流/取消场景覆盖） |
| **AC-AIGW-007** | API Key 被撤销 | 用该 Key 调用数据面 | 返回 401 + 错误码 `ai_api_key_revoked` | 集成测试 |
| **AC-AIGW-008** | 组织 A 的 API Key（`organization_id=A`） | 尝试访问组织 B 的 alias | 路由解析失败（RLS 拦截） + 返回 404 | RLS 负向测试 |
| **AC-AIGW-009** | 启用了 tool 的 alias + MCP Server 注册了工具 | 请求含 `tools[]` + `tool_choice=auto` | 模型返回 tool_use → AI 网关调用 MCP Server → 工具结果回传给模型继续推理 | 集成测试 |
| **AC-AIGW-010** | OpenAI SDK 调用方请求流式 | 发起 Chat Completions + `stream: true` | SSE 透传 + 首 Token 延迟可观测 + 心跳正常 | 集成测试 |

---

## 20. 门禁对接清单


| 门禁 | 触发点 | 对接方式 |
|------|--------|---------|
| `check-schema.sh` | `contracts/schema/75_ai_gateway.yaml` 完整性 | 新增表声明完整（PK/列/约束/索引/软删除/RLS/外键） |
| `check-db-conventions.sh` | 新增表 DDL | 全部合规（主键 UUIDv7、外键索引、RLS ENABLE+FORCE、CHECK 约束命名 `ck_{table}_{column}`） |
| `check-openapi-contract.sh` | 管理面 + 数据面路由 | 路由定义含 `operationId` + `permission`；OpenAPI 生成物零差异 |
| `check-api-contract.sh` | API 契约一致性 | 管理面/数据面路由均有 OpenAPI 声明 |
| `check-layers.sh` | 新增代码分层 | Domain 只 import Platform；App 只 import Domain+Platform；Infra 只 import Domain+Platform；Transport 只 import App+Platform |
| `check-i18n-keys.py` | 新增 i18n 键 | 菜单分组 + 状态枚举 + 页面标题在全部语言包一致 |
| `check-frontend-coverage.py` | 前端覆盖闭合 | 新增页面进路由/菜单/设计报告；agents/mcpservers 路由迁入同步更新 |
| `check-ui-wiring.py` | 路由注释一致性 | `infra-routes.tsx` / `security-routes.tsx` 文件头 JSDoc 移除 agents/mcp-servers 引用 |
| `check-ac-traceability.py` | AC 追溯完备性 | 全部 AC-AIGW-* 由追溯门禁校验（矩阵见 `docs/traceability-matrix.md`） |
| `check-registry-sync.py` | 注册表同步 | 错误码/权限点/状态机/命名空间/审计事件与代码一致 |
| `check-naming.sh` | 命名规范 | 禁用拼写为零；`aigateway` 前缀统一 |
| `check-generated-zero-diff.sh` | 生成物零漂移 | Schema/OpenAPI/前端注册表由生成器产出，无手写 |

---

## 21. 关键设计裁决摘要

| 议题 | 裁决 | 理由 | ADR |
|------|------|------|-----|
| 对外协议面 | 暴露 OpenAI + Anthropic + Gemini 三个协议面 | 调用方使用原生 SDK 零改造；避免双重转换语义损失 | — |
| 协议面路径 | `/api/v1/ai/faces/{protocol}/...` | 避免双 `v1` 冲突；`faces` 段显式表达协议面 | — |
| 内部表示 | Canonical 结构作为唯一转换中枢 | 新增供应商只加上游适配器，路由核心不变 | — |
| `MaxTokens` 裁决 | Go 结构体 CamelCase，JSON `max_tokens`；Anthropic 的 `max_output_tokens` 在适配器层转换 | 统一 JSON 键与 OpenAI 生态对齐 | — |
| 同进程 vs 独立数据面 | 首版同进程；三触发条件任一命中独立 | 初期流量可控；Canonical 核心与进程位置解耦 | ADR-034 |
| Legacy 路径 | **不保留** | 不提供 deprecated alias；若未来兼容历史客户端，在 ADR 中记录 | — |
| `llm_provider_health` RLS | **受保护** | `rls: true`，经 `provider_id` 关联供应商归属租户（`contracts/schema/75_ai_gateway.yaml`） | ADR-016 |
| 熔断状态机 | `healthy→degraded→open→half_open→healthy` | 半开探测模式；连续失败阈值 5 次；熔断时长 30s | — |
| API Key 鉴权位置 | handler 内完成，跳过 Casbin PEP | API Key 不是会话令牌，不走管理面权限点 | — |
| tool_use 运行时绑定 | CanonicalTool.MCPBinding 由 App 层注入 | Domain 层无对 mcpservers 域的硬依赖 | — |
| `provider_type` 枚举 | 独立于 IAM 域 providers | 两个域职责不同（AI 网关供应商 vs IAM 身份供应商） | — |

---
