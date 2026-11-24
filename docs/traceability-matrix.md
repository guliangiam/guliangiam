# GuliangIAM 需求追溯矩阵

**生成方式**：`python3 scripts/gen_traceability.py`（自动扫描全仓文档锚点）

## 锚点格式约定

| 类型 | 格式 | 示例 |
|------|------|------|
| 需求（定义） | `docs/requirements/**` 表格行（首列为需求编号） | `AUTH-001` 行 |
| 需求（引用） | `[REQ:FAM-XXX]` / AC 标题括号 / `SRS FAM-XXX` | `[REQ:AUTH-001]` |
| 设计 | `[DESIGN:path]` | `[DESIGN:auth/oauth2]` |
| 验收条件 | `[AC:MODULE-XXX]` | `[AC:AUTH-001]` |
| ADR | `[ADR:NNN]` | `[ADR:004]` |
| 开放项 | `[OPEN:xxx-XXX]` | `[OPEN:OPEN-API-05]` |

> 逐号定义只来自 `docs/requirements/**/*.md` 的表格行；章节 `[REQ:… ~ …]` 仅声明编号空间。

## 锚点统计

| 类型 | 数量 | 锚点前缀 |
|------|------|---------|
| 需求（已定义） | **280** | `AUTH/ORG/USER/APP/PROV/PERM/COMM/INFRA/SEC/FE/REBAC/IGA/RISK/B2B/WLI/EXT/METER/PRIV/DR/DATA/DB-REQ` |
| 设计 | **39** | `path 或 section 编号 (如 auth/oauth2, 14.6.3)` |
| 验收条件 | **200** | `AC-AUTH/AC-TOKEN/AC-PROTO/AC-JWKS/AC-SESSION/AC-...` |
| ADR | **38** | `ADR-001 ~ ADR-038` |
| 开放项 | **101** | `OPEN-API/OPEN-DESIGN/OPEN-...` |

## 需求（已定义）清单（280 项）

| 锚点 | 出现文件 |
|------|---------|
| `APP-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-004` | `docs/requirements/functional.md` |
| `APP-005` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-006` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-007` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-008` | `docs/requirements/functional.md` |
| `APP-009` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `APP-010` | `docs/requirements/functional.md` |
| `APP-011` | `docs/requirements/functional.md` |
| `APP-012` | `docs/requirements/functional.md` |
| `AUTH-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-005` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-006` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-007` | `docs/requirements/functional.md` |
| `AUTH-008` | `docs/requirements/functional.md` |
| `AUTH-009` | `docs/requirements/functional.md`, `docs/design/auth/login.md` |
| `AUTH-010` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-011` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-013` | `docs/requirements/functional.md` |
| `AUTH-014` | `docs/requirements/functional.md` |
| `AUTH-015` | `docs/requirements/functional.md` |
| `AUTH-016` | `docs/requirements/functional.md` |
| `AUTH-017` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-018` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-019` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-020` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-021` | `docs/requirements/functional.md` |
| `AUTH-022` | `docs/requirements/functional.md` |
| `AUTH-023` | `docs/requirements/functional.md` |
| `AUTH-024` | `docs/requirements/functional.md` |
| `AUTH-025` | `docs/requirements/functional.md` |
| `AUTH-026` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-027` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-028` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-029` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-030` | `docs/requirements/functional.md` |
| `AUTH-031` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-032` | `docs/requirements/functional.md` |
| `AUTH-033` | `docs/requirements/functional.md` |
| `AUTH-034` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-035` | `docs/requirements/functional.md` |
| `AUTH-036` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-037` | `docs/requirements/functional.md` |
| `AUTH-038` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-039` | `docs/requirements/functional.md` |
| `AUTH-040` | `docs/requirements/functional.md` |
| `AUTH-041` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/design/auth/webauthn.md` |
| `AUTH-042` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-043` | `docs/requirements/functional.md` |
| `AUTH-044` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-045` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-046` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-047` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-048` | `docs/requirements/functional.md` |
| `AUTH-049` | `docs/requirements/functional.md` |
| `AUTH-050` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-051` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-052` | `docs/requirements/functional.md` |
| `AUTH-053` | `docs/requirements/functional.md` |
| `AUTH-054` | `docs/requirements/functional.md` |
| `AUTH-055` | `docs/requirements/functional.md` |
| `AUTH-056` | `docs/requirements/functional.md` |
| `AUTH-057` | `docs/requirements/functional.md` |
| `AUTH-058` | `docs/requirements/functional.md` |
| `AUTH-059` | `docs/requirements/functional.md` |
| `AUTH-060` | `docs/requirements/functional.md` |
| `AUTH-061` | `docs/requirements/functional.md` |
| `AUTH-062` | `docs/requirements/functional.md`, `docs/design/auth/faceid.md` |
| `AUTH-063` | `docs/requirements/functional.md` |
| `AUTH-064` | `docs/requirements/functional.md` |
| `AUTH-065` | `docs/requirements/functional.md` |
| `AUTH-066` | `docs/requirements/functional.md` |
| `AUTH-067` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `AUTH-068` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-069` | `docs/requirements/functional.md` |
| `AUTH-070` | `docs/requirements/functional.md` |
| `AUTH-071` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-072` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md` |
| `AUTH-073` | `docs/requirements/functional.md` |
| `AUTH-074` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-075` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-076` | `docs/requirements/functional.md` |
| `AUTH-077` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-078` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-079` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-080` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-093` | `docs/requirements/functional.md`, `docs/adr/ADR-026-workload-identity-federation.md` |
| `AUTH-094` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `AUTH-095` | `docs/requirements/functional.md` |
| `AUTH-096` | `docs/requirements/functional.md`, `docs/design/iam/organizations.md` |
| `AUTH-097` | `docs/requirements/functional.md` |
| `AUTH-098` | `docs/requirements/functional.md` |
| `AUTH-099` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md` |
| `AUTH-100` | `docs/requirements/functional.md` |
| `AUTH-101` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md` |
| `AUTH-102` | `docs/requirements/functional.md` |
| `AUTH-104` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `AUTH-105` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `B2B-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `B2B-002` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `B2B-003` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `B2B-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `B2B-005` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `B2B-006` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `COMM-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-005` | `docs/requirements/functional.md` |
| `COMM-006` | `docs/requirements/functional.md` |
| `COMM-007` | `docs/requirements/functional.md` |
| `COMM-008` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-009` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-010` | `docs/requirements/functional.md` |
| `COMM-011` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-012` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `COMM-013` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `DATA-AS-001` | `docs/requirements/data.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `DATA-AU-001` | `docs/requirements/data.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `DATA-MT-001` | `docs/requirements/data.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `DATA-RD-001` | `docs/requirements/data.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `DATA-WR-001` | `docs/requirements/data.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `DB-REQ-005` | `docs/requirements/non-functional.md`, `docs/requirements/data.md` |
| `DR-001` | `docs/requirements/non-functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/data-model.md` ... (+3) |
| `DR-002` | `docs/requirements/non-functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/multi-region.md`, `docs/requirements/data.md` |
| `DR-003` | `docs/requirements/non-functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/multi-region.md`, `docs/requirements/data.md` |
| `DR-004` | `docs/requirements/non-functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/multi-region.md` |
| `DR-005` | `docs/requirements/non-functional.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/multi-region.md` |
| `EXT-001` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-029-identity-actions-sandbox.md`, `docs/architecture/data-model.md` |
| `EXT-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-029-identity-actions-sandbox.md` |
| `EXT-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-029-identity-actions-sandbox.md` |
| `EXT-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-029-identity-actions-sandbox.md` |
| `FE-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `FE-002` | `docs/requirements/functional.md` |
| `FE-003` | `docs/requirements/functional.md` |
| `FE-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `FE-005` | `docs/requirements/functional.md` |
| `FE-006` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `FE-007` | `docs/requirements/functional.md` |
| `FE-008` | `docs/requirements/functional.md` |
| `FE-009` | `docs/requirements/functional.md` |
| `FE-010` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `FE-011` | `docs/requirements/functional.md` |
| `FE-012` | `docs/requirements/functional.md` |
| `FE-013` | `docs/requirements/functional.md` |
| `FE-014` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `FE-015` | `docs/requirements/functional.md` |
| `FE-016` | `docs/requirements/functional.md` |
| `FE-017` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `IGA-001` | `docs/requirements/functional.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/architecture/data-model.md` |
| `IGA-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md` |
| `IGA-003` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md` |
| `IGA-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md` |
| `IGA-005` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/architecture/data-model.md` |
| `IGA-007` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md` |
| `IGA-008` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md` |
| `IGA-010` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `INFRA-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `INFRA-002` | `docs/requirements/functional.md` |
| `INFRA-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `INFRA-004` | `docs/requirements/functional.md` |
| `INFRA-005` | `docs/requirements/functional.md` |
| `INFRA-006` | `docs/requirements/functional.md` |
| `INFRA-007` | `docs/requirements/functional.md` |
| `INFRA-008` | `docs/requirements/functional.md` |
| `INFRA-009` | `docs/requirements/functional.md` |
| `INFRA-010` | `docs/requirements/functional.md` |
| `INFRA-011` | `docs/requirements/functional.md` |
| `INFRA-012` | `docs/requirements/functional.md` |
| `INFRA-013` | `docs/requirements/functional.md` |
| `INFRA-014` | `docs/requirements/functional.md` |
| `INFRA-015` | `docs/requirements/functional.md` |
| `INFRA-016` | `docs/requirements/functional.md` |
| `INFRA-017` | `docs/requirements/functional.md` |
| `INFRA-018` | `docs/requirements/functional.md` |
| `INFRA-019` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `INFRA-020` | `docs/requirements/functional.md` |
| `INFRA-021` | `docs/requirements/functional.md` |
| `INFRA-022` | `docs/requirements/functional.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md` |
| `METER-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `METER-002` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-029-identity-actions-sandbox.md` |
| `METER-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/architecture/data-model.md` |
| `METER-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/architecture/multi-region.md` |
| `METER-005` | `docs/requirements/functional.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/data-model.md`, `docs/architecture/multi-region.md`, `docs/design/commerce/metering.md` ... (+1) |
| `ORG-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `ORG-008` | `docs/requirements/functional.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `ORG-009` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-028-multi-level-org-domain-federation.md` |
| `ORG-016` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/data-model.md` |
| `ORG-019` | `docs/requirements/functional.md`, `docs/design/iam/organizations.md` |
| `ORG-020` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-005` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-006` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-007` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-008` | `docs/requirements/functional.md` |
| `PERM-009` | `docs/requirements/functional.md` |
| `PERM-010` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-011` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PERM-012` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PRIV-001` | `docs/requirements/non-functional.md`, `docs/acceptance-criteria.md`, `docs/design/privacy/README.md` |
| `PRIV-009` | `docs/requirements/non-functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/multi-region.md` |
| `PRIV-010` | `docs/requirements/non-functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` ... (+6) |
| `PROV-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PROV-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `PROV-003` | `docs/requirements/functional.md` |
| `PROV-004` | `docs/requirements/functional.md` |
| `PROV-005` | `docs/requirements/functional.md` |
| `PROV-006` | `docs/requirements/functional.md` |
| `PROV-007` | `docs/requirements/functional.md` |
| `PROV-008` | `docs/requirements/functional.md` |
| `PROV-009` | `docs/requirements/functional.md` |
| `PROV-010` | `docs/requirements/functional.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md`, `docs/design/protocol/scim.md` |
| `PROV-011` | `docs/requirements/functional.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md` |
| `REBAC-001` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md`, `docs/architecture/data-model.md` |
| `REBAC-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-003` | `docs/requirements/functional.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-004` | `docs/requirements/functional.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-005` | `docs/requirements/functional.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-006` | `docs/requirements/functional.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-007` | `docs/requirements/functional.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md` |
| `REBAC-008` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `REBAC-010` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md`, `docs/architecture/data-model.md` |
| `RISK-001` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md`, `docs/architecture/data-model.md` |
| `RISK-005` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `RISK-006` | `docs/requirements/functional.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `RISK-007` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `RISK-008` | `docs/requirements/functional.md` |
| `RISK-009` | `docs/requirements/functional.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `RISK-010` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md`, `docs/architecture/data-model.md` |
| `RISK-012` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md` |
| `RISK-013` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/data-model.md` ... (+2) |
| `RISK-014` | `docs/requirements/functional.md` |
| `SEC-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `SEC-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `SEC-003` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/requirements/non-functional.md` |
| `SEC-004` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `SEC-005` | `docs/requirements/functional.md` |
| `SEC-006` | `docs/requirements/functional.md` |
| `SEC-007` | `docs/requirements/functional.md` |
| `SEC-008` | `docs/requirements/functional.md` |
| `SEC-009` | `docs/requirements/functional.md` |
| `SEC-010` | `docs/requirements/functional.md` |
| `SEC-011` | `docs/requirements/functional.md` |
| `SEC-012` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-001` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-003` | `docs/requirements/functional.md` |
| `USER-004` | `docs/requirements/functional.md` |
| `USER-005` | `docs/requirements/functional.md`, `docs/requirements/README.md` |
| `USER-006` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-007` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-008` | `docs/requirements/functional.md` |
| `USER-009` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-010` | `docs/requirements/functional.md` |
| `USER-011` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-012` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-013` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-014` | `docs/requirements/functional.md` |
| `USER-015` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-016` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `USER-017` | `docs/requirements/functional.md` |
| `USER-018` | `docs/requirements/functional.md` |
| `USER-019` | `docs/requirements/functional.md` |
| `USER-020` | `docs/requirements/functional.md` |
| `USER-021` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md` |
| `WLI-001` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-026-workload-identity-federation.md`, `docs/architecture/data-model.md` |
| `WLI-002` | `docs/requirements/functional.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-026-workload-identity-federation.md`, `docs/architecture/data-model.md` |
| `WLI-003` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-026-workload-identity-federation.md` |
| `WLI-004` | `docs/requirements/functional.md`, `docs/adr/ADR-026-workload-identity-federation.md`, `docs/architecture/data-model.md` |
| `WLI-005` | `docs/requirements/functional.md`, `docs/adr/ADR-026-workload-identity-federation.md` |
| `WLI-006` | `docs/requirements/functional.md`, `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-026-workload-identity-federation.md` |

## 设计清单（39 项）

| 锚点 | 出现文件 |
|------|---------|
| `actions/README` | `docs/design/actions/README.md` |
| `ai-gateway/README` | `docs/design/ai-gateway/README.md` |
| `auth/faceid` | `docs/design/auth/faceid.md` |
| `auth/login` | `docs/design/auth/login.md` |
| `auth/mfa` | `docs/design/auth/mfa.md` |
| `auth/network-identity` | `docs/design/auth/network-identity.md` |
| `auth/oauth2` | `docs/design/auth/oauth2.md` |
| `auth/qrcode` | `docs/design/auth/qrcode.md` |
| `auth/saml` | `docs/design/auth/saml.md` |
| `auth/webauthn` | `docs/design/auth/webauthn.md` |
| `b2b/README` | `docs/design/b2b/README.md` |
| `commerce/metering` | `docs/design/commerce/metering.md` |
| `commerce/order` | `docs/design/commerce/order.md` |
| `commerce/payment` | `docs/design/commerce/payment.md` |
| `frontend/README` | `docs/design/frontend/README.md` |
| `frontend/casdoor-parity-checklist` | `docs/design/frontend/casdoor-parity-checklist.md` |
| `frontend/interaction` | `docs/design/frontend/interaction.md` |
| `frontend/registry` | `docs/design/frontend/registry.md` |
| `frontend/security` | `docs/design/frontend/security.md` |
| `frontend/testing` | `docs/design/frontend/testing.md` |
| `frontend/workbenches` | `docs/design/frontend/workbenches.md` |
| `iam/applications` | `docs/design/iam/applications.md` |
| `iam/organizations` | `docs/design/iam/organizations.md` |
| `iam/permissions` | `docs/design/iam/permissions.md` |
| `iam/roles` | `docs/design/iam/roles.md` |
| `iam/users` | `docs/design/iam/users.md` |
| `iga/README` | `docs/design/iga/README.md` |
| `infrastructure/agent` | `docs/design/infrastructure/agent.md` |
| `infrastructure/audit` | `docs/design/infrastructure/audit.md` |
| `infrastructure/integration` | `docs/design/infrastructure/integration.md` |
| `infrastructure/outbox` | `docs/design/infrastructure/outbox.md` |
| `infrastructure/providers` | `docs/design/infrastructure/providers.md` |
| `privacy/README` | `docs/design/privacy/README.md` |
| `protocol/ldap` | `docs/design/protocol/ldap.md` |
| `protocol/oauth2` | `docs/design/protocol/oauth2.md` |
| `protocol/scim` | `docs/design/protocol/scim.md` |
| `rebac/README` | `docs/design/rebac/README.md` |
| `risk/README` | `docs/design/risk/README.md` |
| `workload/README` | `docs/design/workload/README.md` |

## 验收条件清单（200 项）

| 锚点 | 出现文件 |
|------|---------|
| `AC-AIGW-001` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-002` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-003` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-004` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-005` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-006` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-007` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-008` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-009` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-AIGW-010` | `docs/acceptance-criteria.md`, `docs/design/ai-gateway/README.md` |
| `AC-API-001` | `docs/acceptance-criteria.md` |
| `AC-API-002` | `docs/acceptance-criteria.md` |
| `AC-API-003` | `docs/acceptance-criteria.md` |
| `AC-API-004` | `docs/acceptance-criteria.md` |
| `AC-API-005` | `docs/acceptance-criteria.md` |
| `AC-API-006` | `docs/acceptance-criteria.md` |
| `AC-API-007` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-API-008` | `docs/acceptance-criteria.md` |
| `AC-API-009` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-API-010` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-API-011` | `docs/acceptance-criteria.md` |
| `AC-APP-001` | `docs/acceptance-criteria.md` |
| `AC-APP-002` | `docs/acceptance-criteria.md` |
| `AC-ARCH-001` | `docs/acceptance-criteria.md`, `docs/architecture/layers.md` |
| `AC-ARCH-002` | `docs/acceptance-criteria.md` |
| `AC-ARCH-003` | `docs/acceptance-criteria.md` |
| `AC-AUDIT-DENIAL-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUDITCHAIN-001` | `docs/acceptance-criteria.md`, `docs/architecture/README.md`, `docs/architecture/security-model.md` |
| `AC-AUDITCHAIN-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUTH-001` | `docs/acceptance-criteria.md` |
| `AC-AUTH-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUTH-003` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUTH-004` | `docs/acceptance-criteria.md` |
| `AC-AUTH-005` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUTH-006` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-AUTH-007` | `docs/acceptance-criteria.md` |
| `AC-AUTH-008` | `docs/acceptance-criteria.md` |
| `AC-AUTH-009` | `docs/acceptance-criteria.md` |
| `AC-AUTH-010` | `docs/acceptance-criteria.md` |
| `AC-AUTH-011` | `docs/acceptance-criteria.md` |
| `AC-AUTH-012` | `docs/acceptance-criteria.md` |
| `AC-AUTH-013` | `docs/acceptance-criteria.md` |
| `AC-AUTH-014` | `docs/acceptance-criteria.md` |
| `AC-AUTH-015` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/design/risk/README.md` |
| `AC-AUTH-016` | `docs/acceptance-criteria.md` |
| `AC-B2B-001` | `docs/acceptance-criteria.md`, `docs/design/b2b/README.md` |
| `AC-B2B-002` | `docs/acceptance-criteria.md`, `docs/design/b2b/README.md` |
| `AC-COMM-001` | `docs/acceptance-criteria.md` |
| `AC-COMM-002` | `docs/acceptance-criteria.md` |
| `AC-COMM-003` | `docs/acceptance-criteria.md` |
| `AC-COMM-004` | `docs/acceptance-criteria.md` |
| `AC-COMM-005` | `docs/acceptance-criteria.md` |
| `AC-COMM-006` | `docs/acceptance-criteria.md` |
| `AC-COMM-007` | `docs/acceptance-criteria.md` |
| `AC-CRYPTO-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-CRYPTO-002` | `docs/acceptance-criteria.md` |
| `AC-CRYPTO-003` | `docs/acceptance-criteria.md` |
| `AC-DATA-001` | `docs/acceptance-criteria.md` |
| `AC-DATA-002` | `docs/acceptance-criteria.md` |
| `AC-DATA-003` | `docs/acceptance-criteria.md` |
| `AC-DATA-004` | `docs/acceptance-criteria.md` |
| `AC-DATA-005` | `docs/acceptance-criteria.md` |
| `AC-DATA-006` | `docs/acceptance-criteria.md` |
| `AC-DATA-007` | `docs/acceptance-criteria.md` |
| `AC-DATA-008` | `docs/acceptance-criteria.md` |
| `AC-DATA-015` | `docs/acceptance-criteria.md` |
| `AC-DR-001` | `docs/acceptance-criteria.md` |
| `AC-DR-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md`, `docs/design/privacy/README.md` |
| `AC-EVENT-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-EVENT-002` | `docs/acceptance-criteria.md` |
| `AC-EXT-001` | `docs/acceptance-criteria.md`, `docs/design/actions/README.md` |
| `AC-EXT-002` | `docs/acceptance-criteria.md`, `docs/design/actions/README.md` |
| `AC-FE-001` | `docs/acceptance-criteria.md` |
| `AC-FE-002` | `docs/acceptance-criteria.md` |
| `AC-FE-003` | `docs/acceptance-criteria.md` |
| `AC-FE-004` | `docs/acceptance-criteria.md` |
| `AC-FE-005` | `docs/acceptance-criteria.md` |
| `AC-FE-006` | `docs/acceptance-criteria.md` |
| `AC-FE-007` | `docs/acceptance-criteria.md` |
| `AC-FE-008` | `docs/acceptance-criteria.md` |
| `AC-FE-009` | `docs/acceptance-criteria.md` |
| `AC-FE-010` | `docs/acceptance-criteria.md` |
| `AC-FE-011` | `docs/acceptance-criteria.md` |
| `AC-FE-012` | `docs/acceptance-criteria.md` |
| `AC-FE-013` | `docs/acceptance-criteria.md` |
| `AC-FE-014` | `docs/acceptance-criteria.md` |
| `AC-FE-015` | `docs/acceptance-criteria.md` |
| `AC-FE-016` | `docs/acceptance-criteria.md` |
| `AC-FE-017` | `docs/acceptance-criteria.md` |
| `AC-FE-018` | `docs/acceptance-criteria.md` |
| `AC-FEGOV-001` | `docs/acceptance-criteria.md` |
| `AC-FEGOV-002` | `docs/acceptance-criteria.md` |
| `AC-FEGOV-003` | `docs/acceptance-criteria.md` |
| `AC-FEGOV-004` | `docs/acceptance-criteria.md`, `docs/design/frontend/testing.md` |
| `AC-FEGOV-007` | `docs/acceptance-criteria.md` |
| `AC-FEGOV-008` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-FEGOV-010` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-FEGOV-011` | `docs/acceptance-criteria.md`, `docs/design/frontend/interaction.md` |
| `AC-IDEM-001` | `docs/acceptance-criteria.md` |
| `AC-IDEM-002` | `docs/acceptance-criteria.md` |
| `AC-IDEM-003` | `docs/acceptance-criteria.md` |
| `AC-IGA-001` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/iga/README.md` |
| `AC-IGA-002` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/iga/README.md` |
| `AC-JWKS-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-JWKS-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-KEY-001` | `docs/acceptance-criteria.md` |
| `AC-KEY-002` | `docs/acceptance-criteria.md` |
| `AC-KEY-003` | `docs/acceptance-criteria.md`, `docs/adr/ADR-021-environment-variable-semantics.md`, `docs/architecture/security-model.md` |
| `AC-METER-001` | `docs/acceptance-criteria.md` |
| `AC-METER-002` | `docs/acceptance-criteria.md`, `docs/requirements/functional.md` |
| `AC-MIGRATE-001` | `docs/acceptance-criteria.md` |
| `AC-MIGRATE-002` | `docs/acceptance-criteria.md` |
| `AC-NCIDAS-001` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-002` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-003` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-004` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-005` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-006` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-007` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-NCIDAS-008` | `docs/acceptance-criteria.md`, `docs/design/auth/network-identity.md` |
| `AC-OAUTH-001` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md` |
| `AC-OUTBOX-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-OUTBOX-002` | `docs/acceptance-criteria.md` |
| `AC-OUTBOX-003` | `docs/acceptance-criteria.md` |
| `AC-PAY-001` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-002` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-003` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-004` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-005` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-006` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-007` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-008` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-009` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PAY-010` | `docs/acceptance-criteria.md`, `docs/design/commerce/payment.md` |
| `AC-PERM-001` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-PERM-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-PERM-003` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-PERM-004` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-003-casbin-tenant-boundary.md`, `docs/architecture/security-model.md` |
| `AC-PRIV-001` | `docs/acceptance-criteria.md`, `docs/design/privacy/README.md` |
| `AC-PRIV-002` | `docs/acceptance-criteria.md`, `docs/design/privacy/README.md` |
| `AC-PROTO-001` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md` |
| `AC-PROTO-002` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md` |
| `AC-PROTO-003` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md` |
| `AC-PROTO-004` | `docs/acceptance-criteria.md` |
| `AC-PROTO-005` | `docs/acceptance-criteria.md` |
| `AC-PROTO-006` | `docs/acceptance-criteria.md` |
| `AC-PROTO-007` | `docs/acceptance-criteria.md` |
| `AC-PROV-001` | `docs/acceptance-criteria.md` |
| `AC-RATE-001` | `docs/acceptance-criteria.md` |
| `AC-RATE-002` | `docs/acceptance-criteria.md` |
| `AC-RATELIMIT-001` | `docs/acceptance-criteria.md` |
| `AC-RATELIMIT-002` | `docs/acceptance-criteria.md` |
| `AC-REBAC-001` | `docs/acceptance-criteria.md`, `docs/design/rebac/README.md` |
| `AC-REBAC-002` | `docs/acceptance-criteria.md`, `docs/design/rebac/README.md` |
| `AC-RELEASE-001` | `docs/acceptance-criteria.md` |
| `AC-RELEASE-002` | `docs/acceptance-criteria.md` |
| `AC-RISK-001` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/risk/README.md` |
| `AC-RISK-002` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/risk/README.md` |
| `AC-RISK-003` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/risk/README.md` |
| `AC-RLS-001` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-RLS-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-SEC-001` | `docs/acceptance-criteria.md` |
| `AC-SEC-002` | `docs/acceptance-criteria.md` |
| `AC-SEC-003` | `docs/acceptance-criteria.md` |
| `AC-SEC-004` | `docs/acceptance-criteria.md` |
| `AC-SEC-005` | `docs/acceptance-criteria.md` |
| `AC-SEC-006` | `docs/acceptance-criteria.md` |
| `AC-SESSION-001` | `docs/acceptance-criteria.md` |
| `AC-SESSION-002` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-SESSION-003` | `docs/acceptance-criteria.md` |
| `AC-SESSION-004` | `docs/acceptance-criteria.md` |
| `AC-SLO-001` | `docs/acceptance-criteria.md` |
| `AC-SLO-002` | `docs/acceptance-criteria.md` |
| `AC-TENANT-001` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TENANT-002` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TENANT-003` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TENANT-004` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-001` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-002` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-003` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-004` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-005` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-006` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-007` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-008` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-009` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-010` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-011` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-012` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-013` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-014` | `docs/acceptance-criteria.md` |
| `AC-TOKEN-015` | `docs/acceptance-criteria.md`, `docs/architecture/security-model.md` |
| `AC-TOKEN-016` | `docs/acceptance-criteria.md` |
| `AC-USER-001` | `docs/acceptance-criteria.md` |
| `AC-USER-002` | `docs/acceptance-criteria.md` |
| `AC-USER-003` | `docs/acceptance-criteria.md` |
| `AC-WEBHOOK-001` | `docs/acceptance-criteria.md` |
| `AC-WEBHOOK-002` | `docs/acceptance-criteria.md` |
| `AC-WLI-001` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/workload/README.md` |
| `AC-WLI-002` | `docs/acceptance-criteria.md`, `docs/design/frontend/workbenches.md`, `docs/design/workload/README.md` |

## ADR清单（38 项）

| 锚点 | 出现文件 |
|------|---------|
| `001` | `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-003-casbin-tenant-boundary.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-019-distributed-rate-limit.md`, `docs/adr/README.md` ... (+3) |
| `002` | `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-010-redis-cache.md`, `docs/adr/ADR-017-domain-events.md`, `docs/adr/ADR-019-distributed-rate-limit.md`, `docs/adr/ADR-023-multi-dialect-abstraction.md` ... (+2) |
| `003` | `docs/adr/ADR-003-casbin-tenant-boundary.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-016-rls-tenant-isolation.md`, `docs/adr/ADR-024-rebac-relation-tuple-authz.md`, `docs/adr/README.md` ... (+4) |
| `004` | `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-004-pkce-s256.md`, `docs/adr/README.md`, `docs/architecture/README.md` |
| `005` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-006-no-orm.md`, `docs/adr/ADR-033-face-id-provider-spi.md` ... (+4) |
| `006` | `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-006-no-orm.md`, `docs/adr/ADR-022-declarative-schema-versioned-migration.md`, `docs/adr/README.md`, `docs/architecture/data-model.md` ... (+1) |
| `007` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-007-uuidv7.md`, `docs/adr/ADR-022-declarative-schema-versioned-migration.md`, `docs/adr/README.md` ... (+2) |
| `008` | `docs/GuliangIAM-开发规范.md`, `docs/README.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-005-port-interfaces.md` ... (+17) |
| `009` | `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-009-kms-vault.md`, `docs/adr/ADR-015-key-version-rotation.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/adr/README.md` ... (+9) |
| `010` | `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-010-redis-cache.md`, `docs/adr/ADR-013-session-state-machine.md`, `docs/adr/ADR-019-distributed-rate-limit.md`, `docs/adr/README.md` ... (+7) |
| `011` | `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-008-outbox.md`, `docs/adr/ADR-011-time-partitioning.md`, `docs/adr/ADR-017-domain-events.md` ... (+10) |
| `012` | `docs/README.md`, `docs/adr/ADR-012-openapi-source.md`, `docs/adr/README.md`, `docs/architecture/README.md`, `docs/architecture/api-surfaces.md` ... (+2) |
| `013` | `docs/acceptance-criteria.md`, `docs/adr/ADR-013-session-state-machine.md`, `docs/adr/README.md`, `docs/architecture/layers.md`, `docs/architecture/security-model.md` ... (+1) |
| `014` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-004-pkce-s256.md`, `docs/adr/ADR-014-token-binding.md`, `docs/adr/README.md` ... (+2) |
| `015` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-009-kms-vault.md`, `docs/adr/ADR-015-key-version-rotation.md`, `docs/adr/ADR-021-environment-variable-semantics.md` ... (+11) |
| `016` | `docs/GuliangIAM-开发规范.md`, `docs/README.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-003-casbin-tenant-boundary.md` ... (+23) |
| `017` | `docs/acceptance-criteria.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-008-outbox.md`, `docs/adr/ADR-017-domain-events.md`, `docs/adr/ADR-018-unified-idempotency.md` ... (+2) |
| `018` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-008-outbox.md`, `docs/adr/ADR-017-domain-events.md`, `docs/adr/ADR-018-unified-idempotency.md` ... (+6) |
| `019` | `docs/acceptance-criteria.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-010-redis-cache.md`, `docs/adr/ADR-019-distributed-rate-limit.md` ... (+8) |
| `020` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-019-distributed-rate-limit.md` ... (+6) |
| `021` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-019-distributed-rate-limit.md`, `docs/adr/ADR-021-environment-variable-semantics.md`, `docs/adr/README.md`, `docs/architecture/api-surfaces.md` ... (+6) |
| `022` | `docs/GuliangIAM-开发规范.md`, `docs/README.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-001-modular-monolith.md`, `docs/adr/ADR-002-postgresql-source-of-truth.md` ... (+22) |
| `023` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-002-postgresql-source-of-truth.md`, `docs/adr/ADR-005-port-interfaces.md`, `docs/adr/ADR-006-no-orm.md`, `docs/adr/ADR-007-uuidv7.md` ... (+18) |
| `024` | `docs/GuliangIAM-开发规范.md`, `docs/README.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-003-casbin-tenant-boundary.md`, `docs/adr/ADR-016-rls-tenant-isolation.md` ... (+8) |
| `025` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/adr/README.md` ... (+2) |
| `026` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-026-workload-identity-federation.md`, `docs/adr/README.md`, `docs/architecture/data-model.md`, `docs/architecture/security-model.md` ... (+3) |
| `027` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-027-adaptive-auth-continuous-eval.md`, `docs/adr/README.md`, `docs/architecture/security-model.md`, `docs/architecture/system-overview.md` ... (+2) |
| `028` | `docs/adr/ADR-028-multi-level-org-domain-federation.md`, `docs/adr/README.md`, `docs/architecture/system-overview.md`, `docs/design/b2b/README.md`, `docs/requirements/functional.md` |
| `029` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-029-identity-actions-sandbox.md`, `docs/adr/README.md`, `docs/architecture/system-overview.md`, `docs/architecture/technology-selection.md` ... (+3) |
| `030` | `docs/adr/ADR-025-iga-jit-access-lifecycle.md`, `docs/adr/ADR-030-scim-outbound-jml-provisioning.md`, `docs/adr/README.md`, `docs/architecture/README.md`, `docs/architecture/system-overview.md` ... (+1) |
| `031` | `docs/GuliangIAM-开发规范.md`, `docs/acceptance-criteria.md`, `docs/adr/ADR-022-declarative-schema-versioned-migration.md`, `docs/adr/ADR-023-multi-dialect-abstraction.md`, `docs/adr/ADR-025-iga-jit-access-lifecycle.md` ... (+16) |
| `032` | `docs/adr/ADR-032-frontend-controller-view-separation.md`, `docs/adr/README.md` |
| `033` | `docs/adr/ADR-033-face-id-provider-spi.md`, `docs/adr/ADR-037-provider-registry-boundary.md`, `docs/adr/README.md` |
| `034` | `docs/adr/ADR-034-aigateway-deployment-topology.md`, `docs/adr/README.md`, `docs/design/ai-gateway/README.md` |
| `035` | `docs/adr/ADR-033-face-id-provider-spi.md`, `docs/adr/ADR-035-payment-channel-spi.md`, `docs/adr/ADR-037-provider-registry-boundary.md`, `docs/adr/README.md`, `docs/design/commerce/payment.md` ... (+1) |
| `036` | `docs/adr/ADR-016-rls-tenant-isolation.md`, `docs/adr/ADR-036-scope-aware-rls-hybrid.md`, `docs/adr/ADR-037-provider-registry-boundary.md`, `docs/adr/README.md`, `docs/architecture/data-model.md` ... (+2) |
| `037` | `docs/adr/ADR-037-provider-registry-boundary.md`, `docs/adr/README.md`, `docs/design/infrastructure/providers.md` |
| `038` | `docs/adr/ADR-038-access-token-subject-model.md`, `docs/adr/README.md` |

## 开放项清单（101 项）

| 锚点 | 出现文件 |
|------|---------|
| `OPEN-API-05` | `contracts/registries/open-items.yaml` |
| `OPEN-API-11` | `contracts/registries/open-items.yaml` |
| `OPEN-API-12` | `contracts/registries/open-items.yaml` |
| `OPEN-API-13` | `contracts/registries/open-items.yaml` |
| `OPEN-API-14` | `contracts/registries/open-items.yaml` |
| `OPEN-API-15` | `contracts/registries/open-items.yaml` |
| `OPEN-API-16` | `docs/design/auth/qrcode.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-17` | `contracts/registries/open-items.yaml` |
| `OPEN-API-18` | `contracts/registries/open-items.yaml` |
| `OPEN-API-19` | `contracts/registries/open-items.yaml` |
| `OPEN-API-20` | `docs/adr/ADR-021-environment-variable-semantics.md`, `docs/architecture/system-overview.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-21` | `contracts/registries/open-items.yaml` |
| `OPEN-API-22` | `contracts/registries/open-items.yaml` |
| `OPEN-API-23` | `contracts/registries/open-items.yaml` |
| `OPEN-API-24` | `contracts/registries/open-items.yaml` |
| `OPEN-API-25` | `contracts/registries/open-items.yaml` |
| `OPEN-API-26` | `contracts/registries/open-items.yaml` |
| `OPEN-API-27` | `contracts/registries/open-items.yaml` |
| `OPEN-API-28` | `contracts/registries/open-items.yaml` |
| `OPEN-API-29` | `contracts/registries/open-items.yaml` |
| `OPEN-API-30` | `contracts/registries/open-items.yaml` |
| `OPEN-API-31` | `contracts/registries/open-items.yaml` |
| `OPEN-API-32` | `contracts/registries/open-items.yaml` |
| `OPEN-API-33` | `contracts/registries/open-items.yaml` |
| `OPEN-API-34` | `contracts/registries/open-items.yaml` |
| `OPEN-API-35` | `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/architecture/data-model.md`, `docs/architecture/technology-selection.md`, `docs/design/infrastructure/providers.md`, `docs/operations/deployment.md` ... (+1) |
| `OPEN-API-36` | `docs/architecture/data-model.md`, `docs/design/infrastructure/providers.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-37` | `docs/GuliangIAM-开发规范.md`, `docs/adr/ADR-015-key-version-rotation.md`, `docs/adr/ADR-023-multi-dialect-abstraction.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `docs/adr/ADR-036-scope-aware-rls-hybrid.md` ... (+7) |
| `OPEN-API-38` | `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-39` | `docs/acceptance-criteria.md`, `docs/adr/ADR-031-region-residency-sm-crypto-audit-chain.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-40` | `docs/adr/ADR-011-time-partitioning.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-41` | `contracts/registries/open-items.yaml` |
| `OPEN-API-42` | `docs/adr/ADR-016-rls-tenant-isolation.md`, `docs/adr/ADR-036-scope-aware-rls-hybrid.md`, `docs/architecture/data-model.md`, `docs/architecture/security-model.md`, `docs/design/iam/permissions.md` ... (+1) |
| `OPEN-API-43` | `docs/design/protocol/scim.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-44` | `contracts/registries/open-items.yaml` |
| `OPEN-API-45` | `contracts/registries/open-items.yaml` |
| `OPEN-API-47` | `docs/acceptance-criteria.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-48` | `contracts/registries/open-items.yaml` |
| `OPEN-API-49` | `contracts/registries/open-items.yaml` |
| `OPEN-API-50` | `docs/design/auth/login.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-51` | `contracts/registries/open-items.yaml` |
| `OPEN-API-52` | `contracts/registries/open-items.yaml` |
| `OPEN-API-53` | `contracts/registries/open-items.yaml` |
| `OPEN-API-54` | `contracts/registries/open-items.yaml` |
| `OPEN-API-55` | `docs/operations/runbook.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-56` | `contracts/registries/open-items.yaml` |
| `OPEN-API-57` | `docs/operations/runbook.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-58` | `contracts/registries/open-items.yaml` |
| `OPEN-API-59` | `contracts/registries/open-items.yaml` |
| `OPEN-API-60` | `docs/adr/ADR-038-access-token-subject-model.md`, `docs/design/iam/applications.md`, `docs/design/protocol/oauth2.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-61` | `contracts/registries/open-items.yaml` |
| `OPEN-API-62` | `contracts/registries/open-items.yaml` |
| `OPEN-API-63` | `contracts/registries/open-items.yaml` |
| `OPEN-API-64` | `docs/adr/ADR-038-access-token-subject-model.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-65` | `docs/adr/ADR-038-access-token-subject-model.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-66` | `contracts/registries/open-items.yaml` |
| `OPEN-API-67` | `contracts/registries/open-items.yaml` |
| `OPEN-API-68` | `contracts/registries/open-items.yaml` |
| `OPEN-API-69` | `contracts/registries/open-items.yaml` |
| `OPEN-API-70` | `contracts/registries/open-items.yaml` |
| `OPEN-API-71` | `docs/design/commerce/payment.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-72` | `docs/design/iam/permissions.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-73` | `docs/adr/ADR-023-multi-dialect-abstraction.md`, `docs/operations/deployment.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-74` | `docs/adr/ADR-023-multi-dialect-abstraction.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-75` | `docs/design/protocol/ldap.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-76` | `contracts/registries/open-items.yaml` |
| `OPEN-API-77` | `contracts/registries/open-items.yaml` |
| `OPEN-API-78` | `docs/architecture/multi-region.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-79` | `docs/design/protocol/oauth2.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-80` | `docs/design/auth/webauthn.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-81` | `contracts/registries/open-items.yaml` |
| `OPEN-API-82` | `docs/design/auth/saml.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-83` | `docs/design/infrastructure/agent.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-84` | `contracts/registries/open-items.yaml` |
| `OPEN-API-85` | `docs/operations/deployment.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-86` | `docs/design/protocol/oauth2.md`, `docs/operations/deployment.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-87` | `docs/design/protocol/scim.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-88` | `contracts/registries/open-items.yaml` |
| `OPEN-API-89` | `docs/design/protocol/scim.md`, `contracts/registries/open-items.yaml` |
| `OPEN-API-90` | `contracts/registries/open-items.yaml` |
| `OPEN-API-91` | `contracts/registries/open-items.yaml` |
| `OPEN-API-92` | `contracts/registries/open-items.yaml` |
| `OPEN-API-93` | `contracts/registries/open-items.yaml` |
| `OPEN-API-94` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-01` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-02` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-03` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-04` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-05` | `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-06` | `docs/requirements/functional.md`, `contracts/registries/open-items.yaml` |
| `OPEN-DESIGN-07` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-01` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-02` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-03` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-04` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-05` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-06` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-07` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-08` | `contracts/registries/open-items.yaml` |
| `OPEN-FE-09` | `contracts/registries/open-items.yaml` |
| `OPEN-PERF-01` | `docs/architecture/README.md`, `docs/design/frontend/testing.md`, `contracts/registries/open-items.yaml` |

## 需求 ↔ 验收条件 交叉索引

| 需求编号 | 关联 AC | 需求定义文件 |
|---------|---------|-------------|
| `APP-001` | `AC-APP-001` | `docs/requirements/functional.md` |
| `APP-002` | `AC-APP-001` | `docs/requirements/functional.md` |
| `APP-003` | `AC-APP-001` | `docs/requirements/functional.md` |
| `APP-004` | — | `docs/requirements/functional.md` |
| `APP-005` | `AC-APP-001` | `docs/requirements/functional.md` |
| `APP-006` | `AC-APP-002` | `docs/requirements/functional.md` |
| `APP-007` | `AC-APP-002` | `docs/requirements/functional.md` |
| `APP-008` | — | `docs/requirements/functional.md` |
| `APP-009` | `AC-APP-002` | `docs/requirements/functional.md` |
| `APP-010` | — | `docs/requirements/functional.md` |
| `APP-011` | — | `docs/requirements/functional.md` |
| `APP-012` | — | `docs/requirements/functional.md` |
| `AUTH-001` | `AC-AUTH-001` | `docs/requirements/functional.md` |
| `AUTH-002` | `AC-AUTH-001` | `docs/requirements/functional.md` |
| `AUTH-003` | `AC-AUTH-001` | `docs/requirements/functional.md` |
| `AUTH-004` | `AC-AUTH-003` | `docs/requirements/functional.md` |
| `AUTH-005` | `AC-AUTH-003` | `docs/requirements/functional.md` |
| `AUTH-006` | `AC-AUTH-003` | `docs/requirements/functional.md` |
| `AUTH-007` | — | `docs/requirements/functional.md` |
| `AUTH-008` | — | `docs/requirements/functional.md` |
| `AUTH-009` | — | `docs/requirements/functional.md` |
| `AUTH-010` | `AC-AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-011` | `AC-AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-012` | — | `docs/requirements/functional.md` |
| `AUTH-013` | — | `docs/requirements/functional.md` |
| `AUTH-014` | — | `docs/requirements/functional.md` |
| `AUTH-015` | — | `docs/requirements/functional.md` |
| `AUTH-016` | — | `docs/requirements/functional.md` |
| `AUTH-017` | `AC-OAUTH-001` | `docs/requirements/functional.md` |
| `AUTH-018` | `AC-OAUTH-001` | `docs/requirements/functional.md` |
| `AUTH-019` | `AC-TOKEN-009` | `docs/requirements/functional.md` |
| `AUTH-020` | `AC-TOKEN-001`、`AC-TOKEN-005` | `docs/requirements/functional.md` |
| `AUTH-021` | — | `docs/requirements/functional.md` |
| `AUTH-022` | — | `docs/requirements/functional.md` |
| `AUTH-023` | — | `docs/requirements/functional.md` |
| `AUTH-024` | — | `docs/requirements/functional.md` |
| `AUTH-025` | — | `docs/requirements/functional.md` |
| `AUTH-026` | `AC-TOKEN-002` | `docs/requirements/functional.md` |
| `AUTH-027` | `AC-TOKEN-006` | `docs/requirements/functional.md` |
| `AUTH-028` | `AC-TOKEN-007` | `docs/requirements/functional.md` |
| `AUTH-029` | `AC-TOKEN-007` | `docs/requirements/functional.md` |
| `AUTH-030` | — | `docs/requirements/functional.md` |
| `AUTH-031` | `AC-TOKEN-008` | `docs/requirements/functional.md` |
| `AUTH-032` | — | `docs/requirements/functional.md` |
| `AUTH-033` | — | `docs/requirements/functional.md` |
| `AUTH-034` | `AC-PROTO-001` | `docs/requirements/functional.md` |
| `AUTH-035` | — | `docs/requirements/functional.md` |
| `AUTH-036` | `AC-PROTO-001` | `docs/requirements/functional.md` |
| `AUTH-037` | — | `docs/requirements/functional.md` |
| `AUTH-038` | `AC-PROTO-001` | `docs/requirements/functional.md` |
| `AUTH-039` | — | `docs/requirements/functional.md` |
| `AUTH-040` | — | `docs/requirements/functional.md` |
| `AUTH-041` | `AC-AUTH-007` | `docs/requirements/functional.md` |
| `AUTH-042` | `AC-AUTH-008` | `docs/requirements/functional.md` |
| `AUTH-043` | `AC-AUTH-007` | `docs/requirements/functional.md` |
| `AUTH-044` | `AC-AUTH-004` | `docs/requirements/functional.md` |
| `AUTH-045` | `AC-AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-046` | `AC-AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-047` | `AC-AUTH-013` | `docs/requirements/functional.md` |
| `AUTH-048` | — | `docs/requirements/functional.md` |
| `AUTH-049` | — | `docs/requirements/functional.md` |
| `AUTH-050` | `AC-AUTH-004` | `docs/requirements/functional.md` |
| `AUTH-051` | `AC-AUTH-014` | `docs/requirements/functional.md` |
| `AUTH-052` | — | `docs/requirements/functional.md` |
| `AUTH-053` | — | `docs/requirements/functional.md` |
| `AUTH-054` | — | `docs/requirements/functional.md` |
| `AUTH-055` | — | `docs/requirements/functional.md` |
| `AUTH-056` | — | `docs/requirements/functional.md` |
| `AUTH-057` | — | `docs/requirements/functional.md` |
| `AUTH-058` | — | `docs/requirements/functional.md` |
| `AUTH-059` | — | `docs/requirements/functional.md` |
| `AUTH-060` | — | `docs/requirements/functional.md` |
| `AUTH-061` | — | `docs/requirements/functional.md` |
| `AUTH-062` | — | `docs/requirements/functional.md` |
| `AUTH-063` | — | `docs/requirements/functional.md` |
| `AUTH-064` | — | `docs/requirements/functional.md` |
| `AUTH-065` | — | `docs/requirements/functional.md` |
| `AUTH-066` | — | `docs/requirements/functional.md` |
| `AUTH-067` | `AC-AUTH-016` | `docs/requirements/functional.md` |
| `AUTH-068` | `AC-AUTH-016` | `docs/requirements/functional.md` |
| `AUTH-069` | — | `docs/requirements/functional.md` |
| `AUTH-070` | — | `docs/requirements/functional.md` |
| `AUTH-071` | `AC-AUTH-002` | `docs/requirements/functional.md` |
| `AUTH-072` | `AC-PROTO-002` | `docs/requirements/functional.md` |
| `AUTH-073` | — | `docs/requirements/functional.md` |
| `AUTH-074` | `AC-PERM-004`、`AC-TOKEN-007` | `docs/requirements/functional.md` |
| `AUTH-075` | `AC-AUTH-012` | `docs/requirements/functional.md` |
| `AUTH-076` | — | `docs/requirements/functional.md` |
| `AUTH-077` | `AC-NCIDAS-001`、`AC-NCIDAS-003` | `docs/requirements/functional.md` |
| `AUTH-078` | `AC-NCIDAS-002`、`AC-NCIDAS-008` | `docs/requirements/functional.md` |
| `AUTH-079` | `AC-NCIDAS-004` | `docs/requirements/functional.md` |
| `AUTH-080` | `AC-NCIDAS-005`、`AC-NCIDAS-006`、`AC-NCIDAS-007` | `docs/requirements/functional.md` |
| `AUTH-093` | — | `docs/requirements/functional.md` |
| `AUTH-094` | — | `docs/requirements/functional.md` |
| `AUTH-095` | — | `docs/requirements/functional.md` |
| `AUTH-096` | — | `docs/requirements/functional.md` |
| `AUTH-097` | — | `docs/requirements/functional.md` |
| `AUTH-098` | — | `docs/requirements/functional.md` |
| `AUTH-099` | — | `docs/requirements/functional.md` |
| `AUTH-100` | — | `docs/requirements/functional.md` |
| `AUTH-101` | — | `docs/requirements/functional.md` |
| `AUTH-102` | — | `docs/requirements/functional.md` |
| `AUTH-104` | `AC-AUTH-014` | `docs/requirements/functional.md` |
| `AUTH-105` | `AC-AUTH-015`、`AC-RISK-001` | `docs/requirements/functional.md` |
| `B2B-001` | `AC-B2B-001` | `docs/requirements/functional.md` |
| `B2B-002` | `AC-B2B-001` | `docs/requirements/functional.md` |
| `B2B-003` | — | `docs/requirements/functional.md` |
| `B2B-004` | `AC-B2B-002` | `docs/requirements/functional.md` |
| `B2B-005` | `AC-B2B-002` | `docs/requirements/functional.md` |
| `B2B-006` | — | `docs/requirements/functional.md` |
| `COMM-001` | — | `docs/requirements/functional.md` |
| `COMM-002` | `AC-COMM-001` | `docs/requirements/functional.md` |
| `COMM-003` | `AC-COMM-001` | `docs/requirements/functional.md` |
| `COMM-004` | `AC-COMM-007` | `docs/requirements/functional.md` |
| `COMM-005` | — | `docs/requirements/functional.md` |
| `COMM-006` | — | `docs/requirements/functional.md` |
| `COMM-007` | — | `docs/requirements/functional.md` |
| `COMM-008` | `AC-COMM-002`、`AC-METER-002` | `docs/requirements/functional.md` |
| `COMM-009` | `AC-COMM-002` | `docs/requirements/functional.md` |
| `COMM-010` | — | `docs/requirements/functional.md` |
| `COMM-011` | `AC-COMM-006` | `docs/requirements/functional.md` |
| `COMM-012` | `AC-COMM-006` | `docs/requirements/functional.md` |
| `COMM-013` | `AC-COMM-006` | `docs/requirements/functional.md` |
| `DATA-AS-001` | `AC-IDEM-001`、`AC-OUTBOX-001` | `docs/requirements/data.md` |
| `DATA-AU-001` | `AC-AUDITCHAIN-001`、`AC-AUDITCHAIN-002` | `docs/requirements/data.md` |
| `DATA-MT-001` | `AC-METER-001` | `docs/requirements/data.md` |
| `DATA-RD-001` | `AC-API-006`、`AC-TENANT-002` | `docs/requirements/data.md` |
| `DATA-WR-001` | `AC-AUDITCHAIN-001`、`AC-OUTBOX-001` | `docs/requirements/data.md` |
| `DB-REQ-005` | — | `docs/requirements/non-functional.md` |
| `DR-001` | `AC-DR-002` | `docs/requirements/non-functional.md` |
| `DR-002` | `AC-DR-001` | `docs/requirements/non-functional.md` |
| `DR-003` | — | `docs/requirements/non-functional.md` |
| `DR-004` | `AC-CRYPTO-001` | `docs/requirements/non-functional.md` |
| `DR-005` | — | `docs/requirements/non-functional.md` |
| `EXT-001` | `AC-EXT-001` | `docs/requirements/functional.md` |
| `EXT-002` | `AC-EXT-001` | `docs/requirements/functional.md` |
| `EXT-003` | `AC-EXT-002` | `docs/requirements/functional.md` |
| `EXT-004` | `AC-EXT-001` | `docs/requirements/functional.md` |
| `FE-001` | `AC-FE-018` | `docs/requirements/functional.md` |
| `FE-002` | — | `docs/requirements/functional.md` |
| `FE-003` | — | `docs/requirements/functional.md` |
| `FE-004` | `AC-FE-018` | `docs/requirements/functional.md` |
| `FE-005` | — | `docs/requirements/functional.md` |
| `FE-006` | `AC-FE-018` | `docs/requirements/functional.md` |
| `FE-007` | — | `docs/requirements/functional.md` |
| `FE-008` | — | `docs/requirements/functional.md` |
| `FE-009` | — | `docs/requirements/functional.md` |
| `FE-010` | `AC-FE-018` | `docs/requirements/functional.md` |
| `FE-011` | — | `docs/requirements/functional.md` |
| `FE-012` | — | `docs/requirements/functional.md` |
| `FE-013` | — | `docs/requirements/functional.md` |
| `FE-014` | — | `docs/requirements/functional.md` |
| `FE-015` | — | `docs/requirements/functional.md` |
| `FE-016` | — | `docs/requirements/functional.md` |
| `FE-017` | — | `docs/requirements/functional.md` |
| `IGA-001` | — | `docs/requirements/functional.md` |
| `IGA-002` | `AC-IGA-002` | `docs/requirements/functional.md` |
| `IGA-003` | `AC-IGA-001` | `docs/requirements/functional.md` |
| `IGA-004` | `AC-IGA-001`、`AC-IGA-002` | `docs/requirements/functional.md` |
| `IGA-005` | — | `docs/requirements/functional.md` |
| `IGA-007` | `AC-SEC-005` | `docs/requirements/functional.md` |
| `IGA-008` | — | `docs/requirements/functional.md` |
| `IGA-010` | `AC-IGA-002` | `docs/requirements/functional.md` |
| `INFRA-001` | `AC-WEBHOOK-001`、`AC-WEBHOOK-002` | `docs/requirements/functional.md` |
| `INFRA-002` | — | `docs/requirements/functional.md` |
| `INFRA-003` | `AC-COMM-003`、`AC-WEBHOOK-002` | `docs/requirements/functional.md` |
| `INFRA-004` | — | `docs/requirements/functional.md` |
| `INFRA-005` | — | `docs/requirements/functional.md` |
| `INFRA-006` | — | `docs/requirements/functional.md` |
| `INFRA-007` | — | `docs/requirements/functional.md` |
| `INFRA-008` | — | `docs/requirements/functional.md` |
| `INFRA-009` | — | `docs/requirements/functional.md` |
| `INFRA-010` | — | `docs/requirements/functional.md` |
| `INFRA-011` | — | `docs/requirements/functional.md` |
| `INFRA-012` | — | `docs/requirements/functional.md` |
| `INFRA-013` | — | `docs/requirements/functional.md` |
| `INFRA-014` | — | `docs/requirements/functional.md` |
| `INFRA-015` | — | `docs/requirements/functional.md` |
| `INFRA-016` | — | `docs/requirements/functional.md` |
| `INFRA-017` | — | `docs/requirements/functional.md` |
| `INFRA-018` | — | `docs/requirements/functional.md` |
| `INFRA-019` | `AC-SEC-005` | `docs/requirements/functional.md` |
| `INFRA-020` | — | `docs/requirements/functional.md` |
| `INFRA-021` | — | `docs/requirements/functional.md` |
| `INFRA-022` | — | `docs/requirements/functional.md` |
| `METER-001` | `AC-METER-001`、`AC-METER-002`、`AC-OUTBOX-002` | `docs/requirements/functional.md` |
| `METER-002` | `AC-METER-001` | `docs/requirements/functional.md` |
| `METER-003` | `AC-METER-001` | `docs/requirements/functional.md` |
| `METER-004` | `AC-METER-002` | `docs/requirements/functional.md` |
| `METER-005` | — | `docs/requirements/functional.md` |
| `ORG-004` | `AC-TENANT-001`、`AC-TENANT-002`、`AC-TENANT-003` | `docs/requirements/functional.md` |
| `ORG-008` | — | `docs/requirements/functional.md` |
| `ORG-009` | `AC-FE-005` | `docs/requirements/functional.md` |
| `ORG-016` | — | `docs/requirements/functional.md` |
| `ORG-019` | — | `docs/requirements/functional.md` |
| `ORG-020` | `AC-METER-002` | `docs/requirements/functional.md` |
| `PERM-001` | `AC-PERM-001` | `docs/requirements/functional.md` |
| `PERM-002` | `AC-PERM-002` | `docs/requirements/functional.md` |
| `PERM-003` | `AC-PERM-001` | `docs/requirements/functional.md` |
| `PERM-004` | `AC-PERM-002` | `docs/requirements/functional.md` |
| `PERM-005` | `AC-PERM-002` | `docs/requirements/functional.md` |
| `PERM-006` | `AC-PERM-002` | `docs/requirements/functional.md` |
| `PERM-007` | `AC-PERM-001` | `docs/requirements/functional.md` |
| `PERM-008` | — | `docs/requirements/functional.md` |
| `PERM-009` | — | `docs/requirements/functional.md` |
| `PERM-010` | `AC-PERM-001`、`AC-PERM-003` | `docs/requirements/functional.md` |
| `PERM-011` | `AC-PERM-001` | `docs/requirements/functional.md` |
| `PERM-012` | `AC-PERM-004` | `docs/requirements/functional.md` |
| `PRIV-001` | `AC-PRIV-001` | `docs/requirements/non-functional.md` |
| `PRIV-009` | `AC-CRYPTO-001` | `docs/requirements/non-functional.md` |
| `PRIV-010` | `AC-DR-002` | `docs/requirements/non-functional.md` |
| `PROV-001` | `AC-PROV-001` | `docs/requirements/functional.md` |
| `PROV-002` | `AC-PROV-001` | `docs/requirements/functional.md` |
| `PROV-003` | — | `docs/requirements/functional.md` |
| `PROV-004` | — | `docs/requirements/functional.md` |
| `PROV-005` | — | `docs/requirements/functional.md` |
| `PROV-006` | — | `docs/requirements/functional.md` |
| `PROV-007` | — | `docs/requirements/functional.md` |
| `PROV-008` | — | `docs/requirements/functional.md` |
| `PROV-009` | — | `docs/requirements/functional.md` |
| `PROV-010` | — | `docs/requirements/functional.md` |
| `PROV-011` | — | `docs/requirements/functional.md` |
| `REBAC-001` | `AC-REBAC-001` | `docs/requirements/functional.md` |
| `REBAC-002` | `AC-REBAC-002` | `docs/requirements/functional.md` |
| `REBAC-003` | — | `docs/requirements/functional.md` |
| `REBAC-004` | — | `docs/requirements/functional.md` |
| `REBAC-005` | `AC-REBAC-002` | `docs/requirements/functional.md` |
| `REBAC-006` | — | `docs/requirements/functional.md` |
| `REBAC-007` | — | `docs/requirements/functional.md` |
| `REBAC-008` | — | `docs/requirements/functional.md` |
| `REBAC-010` | — | `docs/requirements/functional.md` |
| `RISK-001` | — | `docs/requirements/functional.md` |
| `RISK-005` | `AC-RISK-001` | `docs/requirements/functional.md` |
| `RISK-006` | — | `docs/requirements/functional.md` |
| `RISK-007` | `AC-RISK-002` | `docs/requirements/functional.md` |
| `RISK-008` | `AC-RISK-002` | `docs/requirements/functional.md` |
| `RISK-009` | — | `docs/requirements/functional.md` |
| `RISK-010` | — | `docs/requirements/functional.md` |
| `RISK-012` | — | `docs/requirements/functional.md` |
| `RISK-013` | `AC-AUDITCHAIN-001`、`AC-AUDITCHAIN-002`、`AC-CRYPTO-003`、`AC-RISK-003` | `docs/requirements/functional.md` |
| `RISK-014` | `AC-AUDITCHAIN-002` | `docs/requirements/functional.md` |
| `SEC-001` | `AC-SEC-004` | `docs/requirements/functional.md` |
| `SEC-002` | `AC-SEC-004` | `docs/requirements/functional.md` |
| `SEC-003` | `AC-RATELIMIT-001`、`AC-SEC-001` | `docs/requirements/functional.md` |
| `SEC-004` | `AC-SEC-002` | `docs/requirements/functional.md` |
| `SEC-005` | — | `docs/requirements/functional.md` |
| `SEC-006` | — | `docs/requirements/functional.md` |
| `SEC-007` | — | `docs/requirements/functional.md` |
| `SEC-008` | — | `docs/requirements/functional.md` |
| `SEC-009` | — | `docs/requirements/functional.md` |
| `SEC-010` | — | `docs/requirements/functional.md` |
| `SEC-011` | — | `docs/requirements/functional.md` |
| `SEC-012` | `AC-AUTH-012`、`AC-RATELIMIT-001` | `docs/requirements/functional.md` |
| `USER-001` | `AC-USER-001` | `docs/requirements/functional.md` |
| `USER-002` | `AC-USER-001` | `docs/requirements/functional.md` |
| `USER-003` | — | `docs/requirements/functional.md` |
| `USER-004` | — | `docs/requirements/functional.md` |
| `USER-005` | — | `docs/requirements/functional.md` |
| `USER-006` | `AC-USER-002` | `docs/requirements/functional.md` |
| `USER-007` | `AC-USER-002` | `docs/requirements/functional.md` |
| `USER-008` | — | `docs/requirements/functional.md` |
| `USER-009` | `AC-USER-002` | `docs/requirements/functional.md` |
| `USER-010` | — | `docs/requirements/functional.md` |
| `USER-011` | `AC-USER-002` | `docs/requirements/functional.md` |
| `USER-012` | `AC-USER-003` | `docs/requirements/functional.md` |
| `USER-013` | `AC-USER-003` | `docs/requirements/functional.md` |
| `USER-014` | — | `docs/requirements/functional.md` |
| `USER-015` | `AC-USER-003` | `docs/requirements/functional.md` |
| `USER-016` | `AC-USER-003` | `docs/requirements/functional.md` |
| `USER-017` | — | `docs/requirements/functional.md` |
| `USER-018` | — | `docs/requirements/functional.md` |
| `USER-019` | — | `docs/requirements/functional.md` |
| `USER-020` | — | `docs/requirements/functional.md` |
| `USER-021` | `AC-USER-001` | `docs/requirements/functional.md` |
| `WLI-001` | `AC-WLI-001` | `docs/requirements/functional.md` |
| `WLI-002` | `AC-WLI-002` | `docs/requirements/functional.md` |
| `WLI-003` | `AC-WLI-002` | `docs/requirements/functional.md` |
| `WLI-004` | — | `docs/requirements/functional.md` |
| `WLI-005` | — | `docs/requirements/functional.md` |
| `WLI-006` | — | `docs/requirements/functional.md` |

---

> **本文件由 `scripts/gen_traceability.py` 自动生成，禁止手改。**
> 重新生成：`python3 scripts/gen_traceability.py`
> 零差异门禁：`python3 scripts/gen_traceability.py --check`
