# AGENTS.md — GuliangIAM 工程代理契约

> 适用对象：在本仓库内执行编码、审查、迁移、文档或发布任务的 AI Agent 与人类协作者。
> 唯一权威源优先级：`docs/GuliangIAM-开发规范.md` 与 `docs/adr/` > `contracts/**` 声明/注册表 > 现有代码。
> 本文件是操作契约，不复制规范正文；冲突时以权威源为准，并在同一变更中修正本文件。

## 1. 项目速览

| 维度    | 事实                                                                                                       |
| ----- | -------------------------------------------------------------------------------------------------------- |
| 产品    | 多租户 IAM：身份、认证、授权、协议（OAuth2/OIDC/SAML/CAS/SCIM/LDAP）、商业、AI 网关、审计                                          |
| 后端    | Go 模块化单体，Clean Architecture：`domain → app → infra → transport`，`platform` 为横切                            |
| 前端    | React 19 + Vite + TypeScript + Ant Design 6，代码在 `web/`                                                   |
| 数据库   | PostgreSQL 唯一运行期基线；MySQL 为 infra 镜像方言，可启动，能力边界见 `open-items.yaml`（ADR-023）                                              |
| 结构事实源 | `contracts/schema/*.yaml`（ADR-022）；DDL、迁移、OpenAPI、前端类型均为生成物                                              |
| 治理事实源 | `contracts/registries/*.yaml`、`openapi/openapi.yaml`、`CHANGELOG.md`                                      |
| 入口    | 后端 `main.go` → `internal/bootstrap`（启动参数 `-migrate`/`-seed`，默认关闭）；迁移 `cmd/migrate`；前端 `web/src/main.tsx` |

### 目录地图

```
internal/domain/<域>/        实体、端口接口、状态机（不 import app/infra）
internal/app/<域>/           用例编排、事务边界、审计/Outbox 嵌入
internal/infra/postgres/     主数据库仓储与适配器（SQL 仅限 internal/infra/**，见不变量 2）
internal/infra/mysql/        MySQL 镜像仓储（能力边界见 open-items.yaml）
internal/transport/http/     路由、DTO、错误映射；路由声明见各模块 routes.go / chi.go
internal/platform/           横切：config、errors、clock、uuid、scopeauthorizer、outbox…
internal/bootstrap/          组合根：wire_*.go、迁移/种子执行器
web/src/pages/               页面（Controller/View 分离）
web/src/shared/{api,ui,lib}/ API client、共享组件、工具
contracts/schema/            声明式 Schema（唯一结构事实源）
contracts/registries/        路由/权限/错误/环境变量/开放项等注册表
db/postgresql/               生成的结构迁移（禁手改）
db/seeds/                    人工编写的种子（数据链唯一源）
db/rollback/                 生成的配对回滚
scripts/                     门禁、生成器、重放验证
```

## 2. 不可违反的不变量

1. **结构只改声明层**：不要手改 `db/postgresql/*.sql`、`internal/bootstrap/schema_sql/*.sql` 或前端生成类型；改 `contracts/schema/*.yaml` 后跑生成器与零差异门禁。
2. **分层单向**：`domain` 不 import `app/infra/transport`；`app` 不 import `infra/transport`；`transport` 不直接写 SQL；业务 SQL 只在 `internal/infra/**`（`platform/db`、`platform/outbox`、`platform/crypto` 维护作业豁免，边界见 `docs/architecture/layers.md` §1；`app/transport/domain` 非测试 SQL 语句字面量基线 0，Rule 10 固化）。门禁：`bash scripts/check-layers.sh`。
3. **错误统一**：返回/包装只用 `internal/platform/errors`；HTTP 错误只经 `route.WriteError`。禁止 `fmt.Errorf` 丢失 `%w` 或自造无状态错误类型。
4. **租户来源可信**：请求 query/body/header 的 `organizationId` 不是可信输入。handler 一律用 `authtransport.TrustedOrganizationID` 或 tenantctx；仓储显式 `organization_id` 过滤（MySQL 无 RLS）。
5. **凭据与敏感字段**：不写固定生产密钥/真实凭据/真实个人信息到仓库；读接口凭据必须脱敏（`XxxSet` + `XxxMasked`）。开发引导口令仅存在于种子哨兵与文档。
6. **种子数据链**：只改 `db/seeds/V1__guliangiam_seed_init.sql`；禁止新增种子版本；每条 INSERT 幂等、外键用业务键子查询；改完运行 `bash scripts/sync-seed-mirror.sh` 并提交镜像。服务启动默认不重放迁移、不导入种子；只有 `-migrate`/`-seed` 或显式 `AUTO_MIGRATE`/`AUTO_SEED` 才执行（见 §4.3）。**data-enrich 例外**：`internal/bootstrap/seed_sql/V2__admin_profile_enrich.sql` 是历史数据富化版本（migrate.go 直接声明，无 db/seeds 对应源；镜像门禁按「版本严格大于种子最大版本」接受），已应用版本受账本校验和不可变约束（`internal/platform/db/migrate.go`），删改均 fatal；其演示个人信息已按用户显式批准替换为明显合成的占位数据（示例公司 / `+8613800000000` / `DEMO000000000000` / 示例地址，不对应真实个人；`OPEN-API-88` 已关闭），该文件自此仍不得再编辑（含注释）；新增 data-enrich 版本（V3+）仍禁止，如需引入须另行显式裁决。
7. **契约登记**：新增/修改路由、权限点、错误码、环境变量、表、枚举，必须同步对应注册表与生成物；禁止只改一处。
8. **文档红线**（`docs/README.md`）：不手写可生成事实（表数/路由数/字段清单）、不写变更叙事、不登记现状偏差；历史只写 `CHANGELOG.md`。

## 3. 开发引导口令（仅本地/测试）

| 项      | 值                                                                                           |
| ------ | ------------------------------------------------------------------------------------------- |
| 管理员账号  | `admin`（`built-in` 组织，platform admin + `all` 租户 scope）                                      |
| 开发哨兵口令 | `Guliang@2026`                                                                              |
| 强制改密   | `user_credentials.need_update_password = true`                                              |
| 生效前提   | 先导入种子（服务启动 `-seed` 或 `migrate up --seed`）；裸启动不写入任何管理员凭据                                     |
| 覆盖规则   | 设置 `BOOTSTRAP_ADMIN_PASSWORD` 后执行 `migrate up --seed` 或服务启动 `-seed`：环境口令替换哨兵，用户自定义的非空哈希不被覆盖 |
| 生产边界   | `APP_ENV=production` 且执行迁移/播种时检测到哨兵则失败（必须注入 Secret 口令）；staging 由策略禁止，代码不拦截                  |

禁止把该口令用于 staging/production；禁止把它写入日志、API 响应或前端构建产物。

## 4. 能力与工作流

### 4.1 能力矩阵

| 能力          | 触发信号             | 先读                                                         | 执行步骤                                                                      | 最小验证                                                                            |
| ----------- | ---------------- | ---------------------------------------------------------- | ------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Schema/迁移变更 | 新增表/列/索引/约束      | `contracts/schema/*.yaml`、ADR-022、`scripts/gen_schema.py`  | 改声明 → 生成 V{N} 与回滚 → 更新快照 → 重生成四方言                                         | `check-migrations.sh`、`check-schema-parity.py`、`check-generated-zero-diff.sh`   |
| 种子变更        | 内置组织/管理员/目录/测试数据 | `db/seeds/V1…sql`、本文件 §2.6                                 | 改 V1 → `sync-seed-mirror.sh` → 隔离库重放两次 → 启动组合回归                           | `check-migrations.sh`、`validate_schema_replay.sh`、`verify_startup_scenarios.sh` |
| 后端领域/接口     | 新端点、新用例、新仓储      | 目标域 `domain/app/infra/transport` 现有实现                      | Domain 端口 → App 用例（事务/审计/Outbox） → Infra SQL → Transport + routes/OpenAPI | `go build/vet/test`、`check-layers.sh`、`check-openapi-contract.sh`               |
| 前端页面/表单     | 新页面、表单/选择器/宽度    | `web/src/pages` 同族页面、`shared/ui/ResourceFormPage.tsx`      | Controller/View 分离 → 语义宽度类 → 复用选择器 → i18n → registry                      | `pnpm typecheck/lint/format:check/test/build`、路由/页面审计                           |
| 安全/租户       | 认证、授权、跨租户、凭据     | `security-model.md`、`tenant_resolution.go`、scopeauthorizer | 可信租户裁决 → 授权前置 → 审计嵌入 → 回归拒绝路径                                             | `check-rls.sh`、`check-credential-masking.py`、安全用例                               |
| 缺陷修复        | 复现失败、运行期 5xx、回归  | 相关模块 + 最近报告                                                | 复现 → 最小修复 → 回归用例 → 记录未验证边界                                                | 对应门禁 + 目标用例                                                                     |
| 文档/注册表治理    | 设计文档、注册表 YAML    | `docs/README.md`、`contracts/registries`                    | 改 YAML/正文 → 重新生成渲染层 → 链接/锚点门禁                                             | `gen_registry_md.py`、`gen_traceability.py`、`check-doc-*`                        |
| 发布/回滚       | 部署、迁移 Job、回滚演练   | `docs/operations/{deployment,runbook,backup-dr}.md`        | 先迁移后应用 → 备份快照 → 观察窗口 → 回滚脚本配对                                             | `verify-docs.sh`、部署检查清单                                                         |

### 4.2 调用逻辑（决策树）

1. 请求涉及 `contracts/schema`、DDL、迁移 → **Schema 流**；不要先改代码。
2. 请求涉及 `db/seeds`、内置管理员/目录 → **种子流**；禁止新增种子版本。
3. 请求涉及 `internal/transport`、`openapi`、权限点 → **API 流**；先查现有路由声明与注册表。
4. 请求涉及 `web/src/pages`、`shared/ui` → **前端流**；先找同族页面与共享壳。
5. 请求涉及认证/授权/租户/凭据/审计 → **安全流**；先确认可信租户与拒绝路径。
6. 请求是"审阅/分析/报告" → **只读流**；不改代码，只产出证据与缺口。
7. 请求范围、凭据、目标环境、是否允许破坏性操作不明确 → **先提问**，不猜测执行。

### 4.3 启动参数（服务进程）

服务进程默认不碰数据库内容：裸启动只校验 schema 是否存在。重放与导入必须显式开启。

| 参数         | 作用                      | 默认                      |
| ---------- | ----------------------- | ----------------------- |
| `-migrate` | 启动前应用待执行的结构迁移（幂等）       | 取 `AUTO_MIGRATE`，未设置则关闭 |
| `-seed`    | 启动前导入种子数据；隐含 `-migrate` | 取 `AUTO_SEED`，未设置则关闭    |

- 优先级：命令行 > 显式 `AUTO_MIGRATE`/`AUTO_SEED`（`true`/`1`/`yes`/`on`） > 默认关闭；`-migrate=false` 可覆盖环境变量。
- 空库裸启动 fail-fast（`schema is empty and AUTO_MIGRATE is not enabled`）；已初始化库裸启动正常起服，账本与业务数据不变。
- 日志：`startup database switches resolved`（开关及来源 `flag`/`env`/`default`）、`database bootstrap skipped|start|finished`（finished 带 `applied_now`/`elapsed_ms`）。
- 实现：`internal/bootstrap/startup_flags.go`（解析与裁决）、`internal/bootstrap/migrate.go`（执行与日志）；`cmd/migrate up [--seed]` 语义不变。
- 回归：`ALLOW_SCHEMA_RESET=1 DATABASE_URL=<隔离库维护 DSN> APP_DATABASE_URL=<同库非 superuser 服务 DSN> APP_DATABASE_ROLE=<服务角色> bash scripts/dev/verify_startup_scenarios.sh`。服务 DSN 省略时回退到 `DATABASE_URL`；服务角色省略时脚本不调整其授权。

## 5. 边界与禁行项

- 不编辑生成物（`db/postgresql`、`schema_sql`、`openapi/openapi.yaml` 生成段、前端契约 TS、`docs/generated`）。
- 不把过程报告/审计快照写入 `docs/` 根目录；过程证据写入 `.trae/documents/`（门禁 `scripts/check-docs-root.py`）。
- 不新增种子版本，不在种子中写结构 DDL，不把 `AUTO_SEED` 用于生产。
- 不让服务启动默认重放/导入：裸启动只校验 schema；重放与导入必须显式 `-migrate`/`-seed` 或环境变量开启。
- 不在 `app/transport/domain` 写 SQL；不跨层 import；不绕过 `UnitOfWork` 事务。
- 不跳过 hooks/门禁（`--no-verify`）；不推送远端；不执行 `git reset --hard`、`push --force`、删除开发库等破坏性操作。
- 不在日志、响应、测试快照中暴露凭据；不把真实密钥写进 `.env` 提交。
- 不为了让测试通过而放宽门禁基线；基线调整必须写 `CHANGELOG` 并附证据。
- 除非另行显式指令，不得在代码落地国家网络身份认证（NCI）能力（`internal/`、`web/src/`、`db/`、`contracts/schema/`、`openapi/`、`contracts/registries/` 中的端口、适配器、表、路由、注册表条目与前端入口均属代码落地；设计边界见 `docs/design/auth/network-identity.md` §1）。

## 6. 验证矩阵（按变更类型取最小集）

```bash
# 通用（python 门禁依赖先装：清单唯一来源 scripts/requirements.txt）
python3 -m pip install -r scripts/requirements.txt
go build ./... && go vet ./... && go test ./...
bash scripts/check-layers.sh
bash scripts/check-migrations.sh

# 种子/迁移（隔离库，勿指向开发库）
bash scripts/sync-seed-mirror.sh
ALLOW_SCHEMA_RESET=1 DATABASE_URL=postgres://.../guliangiam_seed_audit \
  BOOTSTRAP_ADMIN_PASSWORD='...' bash scripts/dev/validate_schema_replay.sh
go run ./cmd/migrate up --seed -dsn "$POSTGRES_DSN"

# 服务启动的显式开关（默认关闭；-seed 隐含 -migrate，命令行优先于环境变量）
go run . -migrate
go run . -migrate -seed

# 三场景回归（隔离库：空库拒绝 / 已初始化库跳过 / 仅重放 / 重放+导入）
ALLOW_SCHEMA_RESET=1 DATABASE_URL=postgres://operator@.../guliangiam_startup_verify \
  APP_DATABASE_URL=postgres://guliangiam@.../guliangiam_startup_verify \
  MIGRATION_DATABASE_URL=postgres://guliangiam_migrator@.../guliangiam_startup_verify \
  APP_DATABASE_ROLE=guliangiam \
  MIGRATION_DATABASE_ROLE=guliangiam_migrator \
  bash scripts/dev/verify_startup_scenarios.sh

# 前端
pnpm --dir web typecheck && pnpm --dir web lint && pnpm --dir web format:check
pnpm --dir web test --run && pnpm --dir web build

# 文档/生成物全量
python3 scripts/gen_registry_md.py && python3 scripts/gen_traceability.py
SQL_CHECK_DSN=postgres://... RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh
```

E2E（需要真实后端 + 隔离库 + 显式测试限流覆盖）：

```bash
CI= VITE_API_TARGET=http://localhost:8001 ENABLE_E2E=true \
E2E_USERNAME=admin E2E_PASSWORD='<测试口令>' \
pnpm --dir web e2e:pw --project=viewport-1440 --workers=2
```

## 7. 当前记忆与开放项

- MySQL 方言的能力边界与开放项状态以 `contracts/registries/open-items.yaml` 为准（`OPEN-API-35` / `OPEN-API-36` / `OPEN-API-37` / `OPEN-API-73` / `OPEN-API-74`，均 closed）；`DB_DRIVER=mysql` 可启动（`internal/bootstrap/runtime_factory.go`），`mysqlusers.Preflight` 不匹配时受控 `unavailable`；不得在没有真实 MySQL 重放证据时宣称 MySQL 完全可用。
- known-red 基线见 `contracts/registries/known-red.yaml`；不得静默提高基线。
- ADR-023（多方言边界）、ADR-031（MySQL 审计链边界）、ADR-022（声明式 Schema/迁移）。
- 开发引导口令规则见 §3；种子哨兵检测与 `BOOTSTRAP_ADMIN_PASSWORD` 覆盖逻辑在 `internal/bootstrap/migrate.go`。
- 种子 data-enrich V2 边界见不变量 6 例外口径：演示个人信息已替换为合成占位值（`OPEN-API-88` 已关闭），V2 文件此后不可变；再次改写属破坏性操作，须另行显式批准。
- 启动参数（`-migrate`/`-seed`、优先级与日志）见 §4.3；默认关闭，`cmd/migrate` 仍是运维迁移入口。
- NCI 边界：国家网络身份认证（NCI）仅为规划态设计（`docs/design/auth/network-identity.md`）；未获外部依赖与同批批准前，`internal/`、`web/src/`、`db/`、`contracts/schema/`、`openapi/`、`contracts/registries/` 不落地端口、适配器、表、路由、注册表条目或前端入口（除非另行显式指令）。

## 8. 交接/报告模板

```markdown
## 变更
- 范围：
- 涉及文件：
- 数据/接口/文档影响：

## 证据
- 命令与结果：
- 失败用例与修复：
- 未验证边界：

## 风险与下一轮
- 遗留项（登记编号）：
- 建议下一轮：
```
