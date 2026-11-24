# Contributing to GuliangIAM

## 开发环境

- Go 版本以 `go.mod` 为准；Node.js 与 pnpm 版本以 `web/package.json` 为准
- PostgreSQL 18（CI 验证基线；生产支持矩阵见 ADR-023）
- Git

```bash
cp .env.example .env.local   # 配置数据库连接
go run .                     # 启动后端

cd web && pnpm install && pnpm dev
```

## 规范入口（唯一权威源）

- 工程规范与门禁口径：[docs/GuliangIAM-开发规范.md](docs/GuliangIAM-开发规范.md)
- 分层/依赖/模块边界：[docs/architecture/layers.md](docs/architecture/layers.md)
- 按域设计：[docs/design/README.md](docs/design/README.md)
- 数据库结构：声明式 Schema（`contracts/schema/*.yaml`，ADR-022）；DDL/迁移/回滚/前端类型均为生成物，禁止手改。

## 本地门禁

```bash
# python 门禁依赖（PyYAML 等；CI 与本机共用同一份清单）
python3 -m pip install -r scripts/requirements.txt

# 文档与代码门禁（完整）
GOCACHE=$PWD/.gotmp/gocache PNPM_HOME=$PWD/.gotmp/pnpmhome \
  RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh

# 常用单项
go build ./...
bash scripts/check-layers.sh
cd web && pnpm lint && pnpm format:check && pnpm build && pnpm test -- --run
```

## 新增领域模块

按[分层规则](docs/architecture/layers.md)在对应层落地：Domain 实体与端口 → App 用例 → Infra 仓储/适配器 → Transport 路由 → Bootstrap 装配 → 前端 API 模块/页面/路由/菜单。路由必须经 `routes.go` 声明并同步 OpenAPI；数据库变更只改声明式 Schema 后重新生成，不手写迁移。

## Git 工作流

- 分支：`feature/{module}-{desc}`、`bugfix/{module}-{desc}`、`hotfix/{desc}`
- 提交：Conventional Commits（`feat|fix|refactor|docs|test|chore|perf`）
- PR：通过全部门禁与测试、更新对应文档、在根 `CHANGELOG.md` 登记；至少 1 人评审后 squash 合并。

## 评审检查

- 分层依赖与模块边界无违规（`check-layers`）
- 错误统一走 `platform/errors`；SQL 全部参数化；无硬编码密钥
- 新路由/契约/表/权限点/错误码/环境变量已登记并生成
- 前端路由 lazy、i18n 键完整、a11y 与预算门禁通过
- 文档与设计一致；生成物零差异
