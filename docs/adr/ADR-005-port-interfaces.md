# ADR-005: 端口接口隔离基础设施

- 状态：[Current]
- 日期：2026-09-14

## 背景

模块化单体（ADR-001）要在没有进程边界的情况下防止模块互相穿透：App 用例若直接 new 具体的 PostgreSQL 仓储、Casbin 实现或外部 IdP 客户端，那么未来替换数据库方言（ADR-023）、接入 KMS（ADR-009）或把某模块拆出进程时，都要改动业务代码；用例测试也会被基础设施绑死。必须在项目早期把依赖方向做成编译期约束，而不是靠评审自觉。

## 决策

Domain 层定义能力端口（`Repository`、`CasbinPolicyManager`、`TokenIssuer`、`DeliverySender` 等接口），Infra 层提供适配器实现；App 层只依赖端口接口，不导入 Infra；Transport 经 App 服务访问领域；`internal/bootstrap` 是唯一组合根，负责把具体适配器注入用例。

## 理由与后果

- 正面：用例测试可用内存 fake 替身，不依赖真实数据库；基础设施（PG 方言、KMS、IdP、支付通道）可整体替换而业务代码不动；依赖方向可由静态脚本强制检查；未来微服务拆分时端口可直接平移为 RPC 契约。
- 负面：端口/适配器/DTO mapper 带来样板代码与初始开发量；接口数量需要治理，避免“一个实现一个空端口”；分层门禁会持续报出历史告警，团队必须承担登记与逐项清零的纪律成本。

## 替代方案

- **App 直接导入 Infra 具体类型（具体仓储/具体客户端）**：放弃理由——替换或替身都要改业务代码，与 ADR-023 的方言抽象、ADR-009 的 KMS 可替换性直接冲突，且用例无法脱离外部设施单测。
- **用微服务进程边界替代端口边界**：放弃理由——为获得隔离而过早承担网络调用、分布式事务与运维成本（见 ADR-011/ADR-008 的推迟逻辑）；正确顺序是先有进程内端口，拆分信号出现时再把端口抬到网络上。

## 验证证据（Current）

以下命令均可在仓库根复现：

- `grep -rn 'interface {' internal/domain --include='*.go' | wc -l`、`find internal/domain -name repository.go | wc -l`、`find internal/infra/postgres -mindepth 1 -maxdepth 1 -type d | wc -l` → 领域端口接口数、仓储端口子包数、PostgreSQL 适配器子包数均以命令输出为准（不在文档手写计数；三者随域模块扩张而增长，以命令输出为准）。
- `bash scripts/check-layers.sh .` → **ALL CHECKS PASSED（9 条规则全部 0 违规，核验 2026-11-21）**：Rule 1（`app/ → infra/`）、Rule 2（`transport/ → domain/`）、Rule 3（`domain/ → infra|app`）、Rule 4 `fmt.Errorf` 包装风格、Rule 5 stdlib log、Rule 6 `TODO(phase3)`、Rule 7 `platform/ → app|infra|transport`、Rule 8 `infra/ → app/`（Rule 7/8 为 informational 计数）、Rule 9 `superAdminCtx` 回潮均为 0。历史 transport→domain 告警与 platform→infra 越层均已逐项清零（租户作用域原语下沉 `platform/tenantctx`、平台审计端口改最小 `dbtx` 接口、cron worker 落 app 层）。
- 旁证：Casbin 仅经 `CasbinManager` 端口被 App 调用（见 ADR-003 证据）；UUID 生成器经 `uuid.Generator` 端口在组合根注入 `V7Generator`（见 ADR-007 证据）。

## 复审条件

Rule 2 历史告警已清零，门禁全绿证据已补登（见验证证据；Rule 7/8 为 informational 计数，当前均为 0），该触发条件已履行；端口与适配器出现系统性失衡（端口无实现、或适配器绕过端口被 App 直接引用）且门禁红灯时复审；ADR-001 触发服务拆分、或 ADR-023 方言层落地时，复核端口粒度是否需要调整。
