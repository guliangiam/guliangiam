# ADR-016: PostgreSQL RLS 多租户隔离

- 状态：[Current]/[Target] — RLS GUC 基础设施、AfterConnect fail-closed 默认 + PrepareConn 逐次注入 + AfterRelease 重置、HTTP 中间件注入及事务缺失上下文 fail-closed 已落地；显式跨租户 target 精确过滤与超管/system 旁路审计已随 ADR-036 落地（`OPEN-API-42` 已关闭）；生产 schema 策略逐表运行时验证为 Target（详见复审条件）。复核注记：角色旁路条款由 ADR-036 取代，RLS `ENABLE`/`FORCE`、GUC 名称、`set_tenant_context` 与策略强制层继续有效。
- 日期：2026-09-14
- 关系：与 ADR-003（App/Casbin 前置校验）叠加为第三层纵深；与 ADR-024（关系元组）叠加，元组表同受 RLS；角色旁路条款由 ADR-036 取代，其余条款继续有效

## 背景

应用层多租户隔离依赖"每条查询都正确书写 `WHERE organization_id = $1`"。随着仓储数量增长（PostgreSQL 适配器子包数以 `ls -d internal/infra/postgres/*/` 输出为准），任何一次漏写或字符串拼接失误都会造成跨租户数据泄露——STD 已记录多起白名单/范围注入偏差，证明人肉纪律会失灵。需要一个不依赖应用代码自觉的数据库层兜底，使得"漏写范围"的查询在物理上只返回本租户行。

## 决策

在应用层 `organization_id` 过滤之外，为租户相关表启用 PostgreSQL Row-Level Security：经会话变量 `app.current_organization_id` 与 `app.current_role` 在数据库层强制隔离；跨租户查询对普通角色返回空集；超级管理员与后台任务通过独立的显式策略（`is_super_admin()` 检测 GUC 角色）放行，旁路本身必须可审计。

## 实现架构

### GUC 常量——单一来源

所有 RLS GUC 字符串集中在 `internal/platform/db/rls.go`，禁止硬编码：

```go
const (
    RLS_GUCRole          = "app.current_role"
    RLSGUCOrganizationID = "app.current_organization_id"

    RLSRoleSuperAdmin = "super_admin"
    RLSRoleMember     = "member"
)
```

### 连接生命周期：fail-closed 默认 + 逐次注入 + 归还重置

现行连接装配（事实源 `internal/platform/db/postgres.go` 的 `NewPostgresPool` 注释与实现）由三个钩子组成：

1. **AfterConnect**（`SetDefaultRLSForPoolAfterConnect`，`internal/platform/db/rls.go`）：新物理连接建立时设 **fail-closed 空默认**（`role=''`、`org=''`，SESSION 级）。此时 `is_super_admin()=false` 且 `current_organization_id()=NULL`，RLS 策略拒绝所有租户行。**池默认不是 super_admin，任何路径都不继承旁路。**
2. **PrepareConn**：每次 `pool.Acquire(ctx)` 前从 ctx 读 `TenantScope`，经 `rlsRoleForMode` 映射 role 并设置 SESSION 级 GUC，覆盖 `pool.QueryRow` / `pool.Exec` / `pool.Acquire` / `UOW.WithinTransaction` 所有取连接路径。无 scope → 保持 fail-closed；`TenantFilterSystem`/`TenantFilterSuperAdmin` → `super_admin`；`strict`/`optional`/`cross` → `member`（cross 携带单一非空 target，见 ADR-036）。
3. **AfterRelease**：连接归还池前把两个 GUC reset 回空（fail-closed）；reset 失败即销毁连接，防止 SESSION GUC 在不同请求间泄漏。

```
┌─────────────────────────────────────────────────────────────────┐
│                    GuliangIAM RLS 上下文注入链路                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  【HTTP 请求链路】                    【后台 Worker 链路】        │
│  (TenantFilterMiddleware)           (入口注入 TenantFilterSystem)│
│           │                           │                         │
│           ▼                           ▼                         │
│  pool.Acquire(ctx)                pool.Acquire(ctx)             │
│  → PrepareConn 注入 scope          → PrepareConn 注入 system     │
│           │                           │                         │
│           ▼                           ▼                         │
│  WithinTransaction(ctx)          Worker SQL（事务内同样经        │
│  → setTenantContext(ctx, tx)       setTenantContext 覆盖）       │
│  → set_tenant_context()             │                           │
│  → LOCAL(true) member+org_id   → LOCAL(true) super_admin        │
│           │                           │                         │
│           ▼                           ▼                         │
│  事务内所有 SQL                  系统作用域 SQL                  │
│  RLS: is_super_admin()=false   RLS: is_super_admin()=true       │
│  → 按 org_id 隔离               → 旁路放行（必须可审计）         │
│                                                                 │
│  无 TenantScope 的任何路径 → fail-closed 空默认 → 拒绝所有租户行 │
│  conn.Release() → AfterRelease reset 回 fail-closed             │
└─────────────────────────────────────────────────────────────────┘
```

#### 链路 1：HTTP 请求（租户隔离生效）

```
HTTP Request
  │
  ├─ SessionContextMiddleware  ── 提取 session.OrganizationSlug
  │
  ├─ TenantFilterMiddleware    ── 注入 TenantScope(ctx)
  │     · 未认证 + StrictMode=false → fallback "built-in"
  │     · 已认证 → session.OrganizationSlug
  │     · ScopeAuthorizer 批准显式 target → TenantFilterCross + target（ADR-036）
  │
  ├─ pool.Acquire(ctx) → PrepareConn 按 scope 设 SESSION 级 role+org_id
  │
  ├─ API handler 调用 WithinTransaction(ctx, fn)
  │     ├─ BEGIN
  │     ├─ setTenantContext(ctx, tx) ← 关键 RLS 注入点
  │     │     ├─ 没有 TenantScope → member + NULL organization_id（fail-closed）
  │     │     ├─ 从 TenantScope 取 org slug
  │     │     ├─ slug 解析前临时 LOCAL 提级 super_admin（仅事务内），
  │     │     │   查 organizations 解析 UUID 后立即设回正确 role
  │     │     ├─ tx.Exec("SELECT guliangiam.set_tenant_context($1, $2)")
  │     │     │     → set_config('app.current_organization_id', uuid, true)  LOCAL
  │     │     │     → set_config('app.current_role', 'member', true)         LOCAL
  │     │     │     ↑ LOCAL=true → 只在当前事务(block)内有效，
  │     │     │       事务回滚/提交后自动消失，不会泄漏到下一个请求
  │     │     └─ 角色覆盖：TenantFilterSuperAdmin/System → "super_admin"；
  │     │        cross → member + 非空 target（不得落入空组织旁路）
  │     ├─ fn(ctx, txUOW) ← 所有 repo 查询
  │     │     ↑ RLS 策略检查：is_super_admin() OR id = current_organization_id()
  │     └─ COMMIT / ROLLBACK
  │           ↑ 事务结束 → LOCAL 设置自动清除
  │
  └─ conn.Release() → AfterRelease reset 回 fail-closed 空默认
```

`WithinTransaction` 必须始终在事务内覆盖池连接的会话级 GUC。无 `TenantScope` 时按普通 member 和空组织设置，租户策略不返回任何租户行；只有显式标记 `TenantFilterSuperAdmin`/`TenantFilterSystem` 才会在该事务内启用旁路。连接池直接查询路径（不经事务）由 PrepareConn 按同一 ctx scope 注入，无 scope 同样 fail-closed。

#### 链路 2：后台 Worker / Cron / Migration（显式系统作用域）

```
Worker goroutine
  │
  ├─ 入口注入 TenantFilterSystem（框架层内部任务专用，不得由 HTTP 请求触发）
  │
  ├─ pgxpool.Acquire(ctx)
  │     ↓ 新物理连接：AfterConnect 设 fail-closed 空默认（role=''、org=''）
  │     ↓ PrepareConn：从 ctx 读到 system scope → SESSION 级 super_admin
  │
  ├─ Worker SQL（事务内由 setTenantContext 以 LOCAL 覆盖为 super_admin）
  │     ↑ is_super_admin()=true → 旁路放行
  │     ↑ 未注入 system scope 的 Worker → fail-closed，查不到任何租户行
  │
  └─ conn.Release()
        ↑ AfterRelease 把 GUC reset 回 fail-closed；reset 失败则销毁连接
        ↑ pgxpool v5 自身只 reset prepared statement 状态，不 reset GUC，
          因此归还重置必须由 AfterRelease 显式完成
```

Migration/Seed 不依赖池默认：迁移执行器在每个迁移事务内显式以 LOCAL 设置 `super_admin`（`internal/platform/db/migrate.go` 的 `applyMigration`），事务结束即失效。

**安全限制**：池的会话级默认是 fail-closed 空值，任何直接使用池的查询（包括非 HTTP 的事务调用）在无显式 scope 时都被 RLS 拒绝；`super_admin` 只能来自显式注入的 `TenantFilterSystem`/`TenantFilterSuperAdmin`（或 ADR-036 定义的受控旁路），且旁路必须可审计——HTTP 无 target 平台管理员旁路经 `AuditTenantBypass` 写审计哈希链，Worker/system 旁路在启动期集中审计（`worker.system_scope`），取证见 `OPEN-API-42` 闭环记录（`contracts/registries/open-items.yaml`）。`app.current_role` 是应用可设置的自定义 GUC，不能单独作为抵御任意 SQL 执行或已泄露应用数据库凭据的授权边界；超管访问需要独立受控的通道与审计。

### GUC 设置：LOCAL vs SESSION 选择规则

| 调用场景 | 设置方式 | `set_config` 第三个参数 | 有效期 | 代码位置 |
|---------|---------|------------------------|-------|---------|
| pgxpool AfterConnect 钩子（fail-closed 空默认） | SESSION | **false** | 整个连接(session)生命周期，直至 PrepareConn 覆盖或 AfterRelease 重置 | `db/rls.go: SetDefaultRLSForPoolAfterConnect` |
| pgxpool PrepareConn 钩子（每次 Acquire 前按 ctx scope 注入） | SESSION | **false** | 本次连接使用期间 | `db/postgres.go: prepareRLSConn` |
| pgxpool AfterRelease 钩子（归还前 reset 回空） | SESSION | **false** | 重置后回到 fail-closed | `db/postgres.go: NewPostgresPool` |
| WithinTransaction 事务内 | LOCAL | **true** | 当前事务(block)内 | `uow.go: setTenantContext` |
| 迁移事务内（Migration/Seed） | LOCAL | **true** | 当前迁移事务内 | `internal/platform/db/migrate.go: applyMigration` |
| `guliangiam.set_tenant_context` SQL 函数 | LOCAL | **true** | 当前事务内 | `V1__guliangiam_schema_init.sql` |

### DDL 辅助函数

```sql
-- db/postgresql/V1__guliangiam_schema_init.sql

CREATE OR REPLACE FUNCTION guliangiam.set_tenant_context(
    p_organization_id uuid, p_role text
) RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_organization_id',
                        COALESCE(p_organization_id::text, ''), true);  -- LOCAL
    PERFORM set_config('app.current_role',
                        COALESCE(p_role, ''), true);                   -- LOCAL
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION guliangiam.current_organization_id() RETURNS uuid AS $$
    SELECT NULLIF(current_setting('app.current_organization_id', true), '')::uuid;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION guliangiam.is_super_admin() RETURNS boolean AS $$
    SELECT COALESCE(current_setting('app.current_role', true) = 'super_admin', false);
$$ LANGUAGE sql STABLE;
```

### RLS 策略模板（所有租户表统一风格）

```sql
-- organizations 表：组织自身隔离
CREATE POLICY tenant_isolation ON guliangiam.organizations
    FOR ALL USING (guliangiam.is_super_admin()
                   OR id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin()
                   OR id = guliangiam.current_organization_id());

-- 业务表：按 organization_id 外键隔离
CREATE POLICY tenant_isolation ON guliangiam.users
    FOR ALL USING (guliangiam.is_super_admin()
                   OR organization_id = guliangiam.current_organization_id())
            WITH CHECK (guliangiam.is_super_admin()
                   OR organization_id = guliangiam.current_organization_id());

ALTER TABLE guliangiam.organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE guliangiam.users FORCE ROW LEVEL SECURITY;
```

`FORCE` 确保表 owner（`guliangiam` 本身）也受 RLS 约束——唯一绕过通道是 `is_super_admin()=true`。

## ⚠️ 排雷清单——每一条都已经踩过

### 坑 1：`set_config(..., true)` 在 AfterConnect hook 里**不持久化**

**症状**：`AfterConnect` 里 `set_config('app.current_role','super_admin', true)` 返回成功，紧接着同一连接上 `current_setting('app.current_role', true)` 读出来是空。

**根因**：`set_config(key, val, true)` 第三个参数 `true` 表示 **LOCAL**——LOCAL 设置只在**当前事务 block** 内有效。AfterConnect hook 在 pgxpool 建立新连接时被调用，此时**没有任何事务**，所以 LOCAL 设置作用域是"空"——瞬间消失。

**修复**：AfterConnect 必须用 SESSION 级 `false`（现行实现设的是 fail-closed 空默认，不是 super_admin）：
```go
// 错误 ❌ —— LOCAL 在 hook 里瞬间消失
conn.Exec(ctx, "SELECT set_config('app.current_role','',true)")

// 正确 ✅ —— 第三个参数 false = SESSION 级（现行 fail-closed 默认）
conn.Exec(ctx, "SELECT set_config('app.current_role','',false), set_config('app.current_organization_id','',false)")
```

**约束**：`AfterConnect` 中 `set_config` 的作用域必须与连接复用策略一致——LOCAL（事务级）在连接复用下失效；应使用 SESSION 级并在归还池时 RESET，否则会出现「HOOK 已执行、`set_config` 无错、读取为空」的矛盾现象。

### 坑 2：pgxpool v5 **不会**自动 reset session GUC

**误解**：以为 `pool.Acquire()` 会把连接 reset 成"干净"状态，所以可以放心在 AfterConnect 里设一次，然后在 HTTP 请求里再覆盖。

**事实**：pgxpool v5 的 health check 和连接归还逻辑只 reset **prepared statement** 状态（`DEALLOCATE ALL`），**不** reset `application_name`、GUC、search_path 等 session 级属性。这意味着：
- 任何钩子设的 SESSION 级 GUC 会留在该物理连接上，直到被显式覆盖或连接销毁重建
- HTTP 请求在事务内设的 LOCAL GUC 会在事务结束后清除（事务级，正确）
- 若不显式重置，上一个请求的 role/org_id 会泄漏给下一个拿到同一连接的使用者

**现行对策**：`NewPostgresPool` 装配 **AfterRelease** 钩子，在连接归还池前把两个 GUC reset 回 fail-closed 空值；reset 失败只能返回 false 销毁连接——否则下一个请求拿到带 GUC 的连接会泄漏隔离。PrepareConn 则在每次 Acquire 前按 ctx scope 重新注入，二者共同保证「池默认拒绝、逐次授权、归还清零」。**不得**以「AfterConnect 设一次 super_admin 默认」作为安全基线——那会使所有无 scope 路径 fail-open，与现行实现相反。

### 坑 3：`setTenantContext` 里 slug 解析**必须在 super_admin 角色下**

**问题**：`setTenantContext` 先要用 slug 查 `organizations` 表解析 UUID。如果此时 RLS 已经生效（比如 role 已经被设成 member），而且 slug 对应的 org 不是当前租户，那查询会返回 0 行。

**当前设计为什么能工作**：
1. WithinTransaction 里的执行顺序是：`BEGIN` → `setTenantContext` → 业务代码
2. `setTenantContext` 在 slug 解析前**临时以 LOCAL 提级 super_admin**（仅当前事务内），解析出 UUID 后立即用 `set_tenant_context` 设回 member + org_id（`internal/infra/postgres/uow/uow.go`）
3. PrepareConn 路径同理：`prepareRLSConn` 先临时切 super_admin 做 slug→UUID 解析（带 TTL 缓存），随后**无条件**设回正确 role + org_id（`internal/platform/db/postgres.go`）
4. 解析用的提级不依赖任何池默认值，事务结束/归还连接后即失效

**⚠️ 如果将来有人改代码把 set_tenant_context 调用提前**，或者在解析完成前就把 role 固定为 member 且不再提级，slug 解析会被 RLS 拦成 0 行。保护措施：在 `setTenantContext`/`prepareRLSConn` 里保留显式注释。

### 坑 4：未设置租户上下文时 RLS **fail-closed**——查询返回 0 行

RLS 策略是 `is_super_admin() OR id = current_organization_id()`。如果 `app.current_role` 是空（不是 super_admin），且 `current_organization_id()` 也是空（没有组织上下文），那么整条策略就是 `false OR id = NULL` → 全 false。结果：**所有查询返回空集**，不是报错。

这是**安全设计**（fail-closed 比 fail-open 好），但非常容易让调试时困惑——"明明有数据，为什么 COUNT(*) 是 0？"

**诊断口诀**：
```sql
-- 任何时候查不到数据，先看这两行：
SELECT current_setting('app.current_role', true);
SELECT current_setting('app.current_organization_id', true);
-- 空 = RLS 拦截了；有值 = RLS 生效中
```

**pgxpool 下**：AfterConnect 设的是 fail-closed 空默认，**任何无 TenantScope 的路径（含 Worker）都会被拦成 0 行**；Worker 必须在入口注入 `TenantFilterSystem`，HTTP 请求经 WithinTransaction 覆盖。查不到数据时先确认是不是**没有注入 scope 或没有走 WithinTransaction**。

### 坑 5：Migration / Seed / DDL 脚本的 RLS 提级是显式的，不靠池默认

迁移执行器在**每个迁移事务内**显式以 LOCAL 设置 `super_admin`（`internal/platform/db/migrate.go` 的 `applyMigration`），事务结束即失效；池的 AfterConnect 默认是 fail-closed 空值，不提供任何隐式提级。如果有人用 `psql` 直连、或者绕过 pgxpool 用裸 `pgx.Connect()`，**默认 role 是空**，RLS 会把所有查询拦成 0 行。

**诊断脚本**：`psql -c "SELECT current_setting('app.current_role', true)"` —— 空 = 当前连接没有 super_admin 提级（裸连接，或未经 scope 注入/迁移事务的池连接）。

### 坑 6：RLS 策略在 CREATE POLICY 时**不会报错**，但会静默拦截

PostgreSQL 对不存在的函数（比如你拼错了 `current_organization_id()` 写成 `current_org_id()`）在 CREATE POLICY 时**不会报错**——因为策略函数是延迟求值的。直到你执行第一条查询时才会出现 "function does not exist" 错误。

**保护**：每次新建 RLS 策略后，必须立即跑一遍诊断脚本验证。

## 安全模型总结

```
┌────────────────────────┬─────────────┬────────────────────────────┬──────────────┐
│  调用场景                │  RLS GUC    │  is_super_admin()          │  效果         │
├────────────────────────┼─────────────┼────────────────────────────┼──────────────┤
│  pgxpool.AfterConnect   │ SESSION 空  │  false（fail-closed 默认）  │  拒绝所有租户行│
│  无 scope 的任何池路径   │ SESSION 空  │  false                     │  拒绝所有租户行│
│  Worker（system scope） │ SESSION/LOCAL│ true（显式注入，须审计）    │  旁路放行     │
│  HTTP WithinTransaction │ LOCAL       │  false（member+org_id）    │  仅本租户行   │
│  显式跨租户（cross）     │ LOCAL       │  false（member+target）    │  仅 target 行 │
│  Migration / Seed       │ LOCAL       │  true（迁移事务内显式提级） │  旁路放行     │
│  conn.Release()         │ AfterRelease│  reset 回空                │  回到 fail-closed│
│  psql 裸连接            │ 未设置      │  false（默认空）            │  RLS 拦截=0行 │
│  DBA 手动 SET           │ SESSION     │  取决于 SET 值              │  显式可控     │
└────────────────────────┴─────────────┴────────────────────────────┴──────────────┘
```

## 理由与后果

- 正面：连接/事务级兜底，漏写 WHERE 不再等于泄露；策略与表同生，新增查询默认受保护；与 ADR-003 的 App/Casbin 形成第三层纵深；fail-closed 池默认 + PrepareConn 逐次注入 + AfterRelease 重置确保无 scope 路径一律拒绝、GUC 不跨请求泄漏；LOCAL 级事务内覆盖确保 HTTP 请求租户隔离严格；Worker/Migration 的提级是显式、受控且可审计的。
- 负面：所有取连接的路径都必须理解"SESSION vs LOCAL"的陷阱；调试排查时不能假设"没数据 = 真没数据"（无 scope 即空集）；pgxpool v5 不自动 reset GUC，归还重置依赖 AfterRelease 显式实现；跨租户查询（运维审计、跨组织报表）必须显式提级到 super_admin 角色或走 ADR-036 的 cross 精确过滤，提级通道必须审计。

## 替代方案

- **仅依赖应用层 WHERE organization_id（现状固化）**：放弃理由作为最终方案——STD 白名单前缀豁免等偏差证明"每查询都写对"不可持续；应用层过滤保留为第一道（也是查询裁剪必需的一道），但不能是唯一一道。
- **每租户独立数据库 / 独立 schema**：放弃理由——全量表结构与每租户一套 schema 的组合运维不可行，迁移、备份与跨租户管理面爆炸，DDL 重放纪律（ADR-022）也会被打破；仅在极端合规租户的私有化交付中作为复审选项。
- **在服务网关/数据访问框架层统一注入租户条件**：放弃理由——仍属应用层机制，可被绕过（裸 SQL/后台作业/迁移脚本）；可作为 RLS 之外的补充纵深，不替代库内强制。

## 验证证据

| 检查点 | 命令/测试 | 结果 |
|-------|----------|------|
| AfterConnect fail-closed 空默认 | `db/rls.go: SetDefaultRLSForPoolAfterConnect` 设 `role=''`、`org=''`（SESSION）；无 scope 时 `is_super_admin()=false` 且 `current_organization_id()=NULL` | 策略拒绝所有租户行 ✅ |
| PrepareConn 逐次注入 / AfterRelease 重置 | `db/postgres.go: NewPostgresPool` 装配 `prepareRLSConn` 与归还 reset；`TestSetTenantContextFailsClosedAndResetsPooledConnection`（OPEN-API-42 闭环取证，真实库 PASS） | PASS ✅ |
| 事务内 slug→UUID 解析 | `setTenantContext` 临时 LOCAL 提级解析后设回 member+org_id | 解析可达、隔离生效 ✅ |
| member+org_id LOCAL 设置 | `SELECT is_super_admin()` | `false` ✅ |
| 显式跨租户 cross 精确过滤、写他租户拒绝、旁路审计 | `TestTenantFilterCrossScopesToSingleTenant`（真实库）+ `AuditTenantBypass`/`worker.system_scope`，见 `OPEN-API-42` 闭环记录（`contracts/registries/open-items.yaml`） | PASS ✅ |
| 非 owner、无 `BYPASSRLS` 权限的应用角色下读/写隔离与 `FORCE ROW LEVEL SECURITY` | 同一测试创建策略 fixture 并 `SET LOCAL ROLE guliangiam_app` | PASS ✅（验证 PostgreSQL 执行语义；不替代所有生产表策略逐表审计） |
| 生产 schema 中所有 RLS 策略逐表隔离 | 逐表运行时策略测试 | 尚未验证（Target） |
| pgxpool build | `go build ./...` | OK ✅ |
| go vet | `go vet ./...` | OK ✅ |

## 复审条件

单库内需要 Schema 级更高隔离、或转向每租户独立库（私有化合规要求）时复审；ADR-023 目标方言对 RLS 的等价能力（MySQL 视图/行级安全等）实测结论出炉后重定支持级别；发现任何 RLS 旁路通道被非授权路径使用时立即安全整改。
