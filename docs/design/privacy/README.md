# 隐私与数据主体权利设计（DSAR / 同意 / 可携 / 擦除）

> 设计锚点：[DESIGN:privacy/README]
> 需求锚点：`non-functional.md §5.2`（PRIV-001/009/010）；驻留口径见 [multi-region.md §1](../../architecture/multi-region.md)。
> 结构承载：表与列以声明式 Schema 的最终声明为准；本文件只约束语义、状态机与验收口径。

## 1. 定位

统一承载数据主体权利请求（DSAR）：访问、更正、冻结、擦除、可携、撤回同意。所有请求带 SLA 时钟（≤30 天）与不可变审计；擦除与「审计/合规留存」冲突时按双轨裁决，不得以擦除为由删除审计链。

## 2. 数据模型（目标结构）

| 表 | 作用 | 隔离 |
|---|---|---|
| `dsar_requests` | 请求主体、类型、状态、SLA 到期 | 租户表（RLS） |
| `dsar_events` | 状态迁移与处置证据（actor/动作/结果） | 租户表（RLS） |
| `consent_records` | 同意版本与撤回（按主体 + 目的 + 版本） | 租户表（RLS） |

列清单以 `contracts/schema/80_governance.yaml` 为准（复现：`grep -n "dsar_requests\|dsar_events\|consent_records" contracts/schema/80_governance.yaml`）。三张表由声明式 Schema 基线声明（租户 RLS、状态机 CHECK、SLA 到期列、同意版本唯一键）。

## 3. DSAR 状态机

```
received → verifying → in_progress → fulfilled
                    ↘ rejected（身份无法核验/请求越权/法律豁免）
```

| 转换 | 条件 |
|---|---|
| received → verifying | 收到请求并登记 SLA 起始时间 |
| verifying → in_progress | 身份核验通过且请求范围可执行 |
| verifying → rejected | 身份无法核验、请求超出范围或命中法律豁免（必须记录理由） |
| in_progress → fulfilled | 处置完成并产出证据包（导出/擦除记录/更正 diff）；erase 请求须待冷静期结束 |
| in_progress → rejected | erase 请求在冷静期内由数据主体撤回（可恢复，必须记录理由） |

终态 `fulfilled`/`rejected` 不可逆；超 SLA 未终结必须告警并计入合规指标。

状态机实现（唯一声明）：`internal/domain/privacy/dsar.go` 的 `transitions` 表；`DsarRequest.TransitionTo` 强制终态不可逆、`rejected` 必须带理由、终态写 `closedAt`；`SLABudget` = 30 天，超期由 `IsOverdue` 判定。

**擦除冷静期（AC-PRIV-002）**：erase 请求进入 in_progress 时登记 cooling_until = now + CoolingWindow（7 天）；冷静期内允许 in_progress → rejected 撤回（可恢复、写审计），冷静期结束前拒绝 in_progress → fulfilled（处置不可逆）。实现锚点：DsarRequest.InCooling / CoolingWindow，列 dsar_requests.cooling_until。

## 4. 请求类型与处置

| 类型 | 处置 | 证据 |
|---|---|---|
| 访问 / 可携 | 生成导出包：JSON Lines（或 CSV）+ 清单 + 校验和；跨区域数据只从归属区域导出 | 导出清单与校验和 |
| 更正 | 生成字段级 diff 并应用到主体记录 | diff + 审计 |
| 冻结 | 置冻结标记，冻结期间拒绝处理性访问（认证/营销），不影响法定留存 | 状态与审计 |
| 擦除 | 按「个人数据最小化」清除或匿名化；审计记录与法定留存数据按双轨保留 | 擦除范围清单 |
| 撤回同意 | 写入带回溯时间的撤回记录，后续处理以最新版本为准 | consent_records 版本链 |

- 同意版本链实现：`internal/domain/privacy/consent.go` + `internal/infra/postgres/privacy/consent.go`；授予追加版本（version+1），撤回对最新版本写 withdrawn_at，历史版本只追加不删除；端点 GET/POST /api/v1/privacy/consents、POST /api/v1/privacy/consents/{purpose}/withdraw；审计事件 privacy.consent.granted / privacy.consent.withdrawn。
- 导出实现：`internal/domain/privacy/export.go`（记录归一化排序、确定性 JSON Lines、清单与 SHA-256 校验和）+ `internal/infra/postgres/privacy/export.go`（用户/profile/consent 跨表主体聚合，不导出凭据/MFA/密钥）；端点 `GET /api/v1/privacy/exports/{id}` 返回 JSON Lines 并设置 `X-Checksum-Sha256` / `X-Record-Count` / `X-Region`。
- 擦除实现：`internal/domain/privacy/erase.go`——留存项逐项列明法律依据；`Blocks=true` 的法律留存返回 409 并附依据；无可擦除项不得视为已完成。擦除复核端点 `POST /api/v1/privacy/dsar/{id}/erase-review` 输出 Erase/Retain 清单并写 `privacy.erasure.reviewed` 审计；实际跨表删除按运营处置流程执行。

**擦除与审计冲突裁决**：审计与合规留存数据不属于擦除范围；擦除范围清单必须逐项列出保留项与法律依据，由合规负责人复核。

## 5. API 与页面

- 端点族：`/api/v1/privacy/dsar`（创建/列表/详情/状态迁移）、`/api/v1/privacy/consents`（查询/撤回）、`/api/v1/privacy/exports/{id}`（下载证据包）。端点以 `internal/transport/http/api/v1/privacy/routes.go` 声明并经 `gen_openapi.py` 生成的 OpenAPI 为准，本设计只约束语义、错误码与权限点。
- 端点（管理面，租户取自会话）：`GET/POST /api/v1/privacy/dsar`、`GET /api/v1/privacy/dsar/{id}`、`POST /api/v1/privacy/dsar/{id}/transition`、`GET /api/v1/privacy/dsar/overdue`、`GET /api/v1/privacy/exports/{id}`（下载 fulfilled access/portability 证据包）、`POST /api/v1/privacy/dsar/{id}/erase-review`（`privacy:erase`，返回 Erase/Retain 清单，法律留存阻断 409）；权限点 `privacy:read` / `privacy:write`；传输层路径 `internal/transport/http/api/v1/privacy`；审计由 `internal/bootstrap/privacy_runner.go` 适配器在请求+事件同事务写 `records`/outbox（`Tx.EmbedAudit`）。
- 权限点：`privacy:read` / `privacy:write` / `privacy:erase`（后者仅合规角色），登记 `contracts/registries/permissions.yaml`；擦除复核端点 `POST /api/v1/privacy/dsar/{id}/erase-review` 使用 `privacy:erase`。
- 页面：隐私工作台（DSAR 列表 + 详情 + 导出）；路由与页面文件以 `docs/design/frontend/registry.md` 为准。
- 应用层实现：`internal/app/privacy/dsar_service.go`（创建/迁移/超期巡检；仓储端口契约要求请求+事件+审计同事务）；域模型见 `internal/domain/privacy/dsar.go`。
- 基础设施：`internal/infra/postgres/privacy`（请求+事件同事务写入、租户过滤、超期列表）；审计嵌入见 `internal/bootstrap/privacy_runner.go`，HTTP 端点声明见 `internal/transport/http/api/v1/privacy/routes.go`。

## 6. 审计与错误码

| 场景 | 事件 | 处置 |
|---|---|---|
| 请求创建/状态迁移 | `dsar.request.created` / `dsar.request.updated` | 写 `records`（同事务） |
| 请求完成/拒绝 | `dsar.request.fulfilled` / `dsar.request.rejected` | 写 `records` + 证据包引用 |
| 同意授予/撤回 | `privacy.consent.granted` / `privacy.consent.withdrawn` | 写 `records`（版本链） |
| 数据导出 | `privacy.data_exported` | 写 `records`（不含导出内容） |
| 擦除复核 | `privacy.erasure.reviewed` | 写 `records`（擦除/留存清单与法律依据） |
| 数据擦除执行 | `privacy.data_erased` | 写 `records`（不含被擦除内容） |

错误码：越权/跨租户一律 404 `not_found`（防枚举）；SLA 超期不改变 HTTP 语义，只触发告警；擦除被法律留存拒绝返回 409 `conflict` 并附保留依据。

## 7. 验收锚点

- `AC-PRIV-001`：DSAR 工作台六类请求、SLA ≤30 天、状态机与全程审计。
- `AC-PRIV-002`：同意版本与冷静期。
- `AC-DR-002` / PRIV-010：个人数据不跨区域复制，跨区域访问受控并审计。

## 8. 边界与未决

- 不新建独立隐私数据库；全部表在租户 RLS 下。
- 导出包不得包含其他主体数据；单次导出上限与分片策略以应用层实现声明为准（`internal/app/privacy/dsar_service.go`）。
- 擦除的法律依据与保留清单需合规负责人确认。
