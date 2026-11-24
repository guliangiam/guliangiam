# 备份、恢复与灾备

> 目标态规范「备份、恢复与灾备」；写作约束见 [docs/README.md](../README.md)。

---

## 1. 备份体系总览

| 组件 | 机制 | 频率 | RPO 贡献 |
|------|------|------|---------|
| PostgreSQL 数据 | pg_basebackup 物理基备 + WAL 连续归档（PITR） | 基备每周 / WAL 连续 | 分钟级 |
| PostgreSQL 数据（逻辑补充） | `pg_dump -Fc` | 每日 | = 导出间隔 |
| 应用配置 / 结构与迁移 | Git 仓库（声明式 Schema + 版本化迁移） | 每次变更 | 0 |
| Secret/密钥 | KMS/Secret Manager 托管（不在备份文件中） | — | 0（按引用恢复） |

**概念澄清**：`pg_dump` 产出的是逻辑备份，不含 WAL 段、没有可连续回放的时间轴，只能整库恢复到导出那一刻。`recovery.signal` 只对物理基备 + WAL 归档的组合生效。「`pg_dump` 基备 + `recovery.signal` = PITR」是错误指引，不得照做。

---

## 2. 物理基备：pg_basebackup

> 建议流程：pg_basebackup 物理全量备份（每周一次，systemd timer / cron 触发）；运维脚本化与演练记录的出口条件见《开发规范》I.2。

关键参数（不复写完整代码）：

| 参数 | 值 | 说明 |
|------|-----|------|
| 备份账号 | 需 REPLICATION 权限 | 非超级用户 |
| 基备格式 | `-Fp`（纯文件）+ `-X stream`（同步流式拉 WAL） | 物理恢复必需 |
| 加密 | AES-256-CBC + PBKDF2（600000 迭代） | 密文是唯一长期留存形态 |
| 异地存储 | S3 + SSE-KMS | 跨区复制见 §5 |
| 本地保留 | 4 代（≥28 天） | 覆盖 L3 RPO 目标（`contracts/registries/dr.yaml`）与月度演练窗口 |

脚本约束：`set -euo pipefail`，任一步骤失败即退出，禁止产出残缺"成功备份"。备份口令来自 Secret/KMS，禁止回显到日志或命令行。

---

## 3. WAL 连续归档

> 源章节：《[GuliangIAM-开发规范](../GuliangIAM-开发规范.md)》§13.6（备份与恢复）。

主库 `postgresql.conf` 关键参数：

```ini
wal_level = replica
archive_mode = on
archive_timeout = 60        # 无写入也强制切换，约束 RPO 上界
archive_command = 'aws s3 cp %p s3://your-wal-bucket/guliangiam/wal/%f'
```

归档健康巡检：

```bash
psql "$DSN" -c "SELECT archived_count, failed_count, last_archived_time, last_failed_time FROM pg_stat_archiver;"
# failed_count 持续增长 = 归档断裂：pg_wal 目录将持续膨胀
```

**红线**：禁止 `test ! -f` 形态的 archive_command（目标已存在时返回 1 会被当成失败）。归档断裂时**严禁直接删 pg_wal 段**，必须先修复归档出口。

---

## 4. PITR 恢复（物理基备 + WAL 归档）

> 建议流程：基于基础备份 + WAL 归档做 PITR 恢复到目标时间点（在备用主机执行）；脚本化与恢复演练的出口条件见《开发规范》I.2。

关键步骤摘要：

1. 解密并还原物理基备（保留 `backup_label`）
2. 写恢复参数：`restore_command` 从 WAL 归档拉段、`recovery_target_time` 设目标点、`recovery_target_action = promote`
3. 放置 `recovery.signal`
4. 启动并跟踪日志直到 archive recovery complete 且 promoted

恢复后校验：

```bash
# 已退出恢复模式
psql -c "SELECT pg_is_in_recovery();"   # 必须为 f

# 关键表抽查（与故障前基线对比）
psql -c "SELECT count(*) FROM guliangiam.users;"

# 应用切回
kubectl -n guliangiam rollout restart deployment/guliangiam
```

**RPO 约束**：可恢复的最早点 = 所选物理基备的完成时刻；最远点 = WAL 归档保留末段。

---

## 5. pg_dump 逻辑补充

> 建议流程：pg_dump 逻辑备份（每日）；脚本化与定期恢复校验的出口条件见《开发规范》I.2。

逻辑备份用途：发布前快照、表级抽取、跨版本迁移演练、与物理恢复结果快速核对。**不可 PITR**。

逻辑恢复（整库重建）：

```bash
pg_restore -h "$PGHOST" -U "$PGUSER" -d guliangiam --clean --if-exists guliangiam_YYYYMMDD_HHMMSS.dump
```

### 5.1 FORCE RLS 下的备份通道

本库对业务表启用 `FORCE ROW LEVEL SECURITY`（策略见 ADR-016）。FORCE 的语义是「表 owner 也受策略约束」，因此**应用账号不能用于 `pg_dump`**：

- 应用账号即使持有表 owner 身份，在没有 `app.current_role='super_admin'` 会话上下文时，`pg_dump` 只能导出当前租户策略放行的行；缺少 schema/表权限时会直接报 `permission denied for schema guliangiam`（SQLSTATE 42501）。
- 结论：每日逻辑备份、发布前快照与回滚快照一律走高权通道；应用账号不是备份账号。

| 通道 | 用途 | 约束 |
|------|------|------|
| superuser socket / 专用 `BYPASSRLS` 备份角色 | 每日逻辑备份、发布前/回滚快照 | 凭据来自 KMS/Secret；禁止写入仓库、日志或命令行；`sslmode=require` 及以上 |
| `pg_basebackup`（REPLICATION 权限） | 物理基备 | 由复制协议直读数据文件，不经过 RLS，与逻辑备份互补 |

禁止为了完成备份而临时提升应用账号权限或关闭 RLS：这会把生产库暴露在租户策略之外。恢复校验（表数、策略数、关键表计数）同样走高权通道。

---

## 6. 备份加密、异地副本与保留

| 维度 | 要求 |
|------|------|
| 传输加密 | DSN `sslmode=require` 及以上；`pg_basebackup` 走 TLS |
| 静态加密 | 双层：客户端 `openssl enc`（加密口令来自 KMS 下发的数据密钥）+ 对象存储 SSE-KMS |
| 异地副本 | 桶开启跨区域复制（CRR）或双区域双写；异地桶与主区域凭据隔离 |
| 防误删 | 桶开启版本化；删除保护（对象锁/合规保留）按合规配置 |
| 保留代次 | 物理基备 4 代（周备，≥28 天）；WAL 归档 14 天；`pg_dump` 30 天；年度归档密文 1 份 |
| 完整性 | 每次上传后核对 SHA256SUMS；月度演练时至少抽 1 代基备做解密 + 校验和核对 |

---

## 7. RTO/RPO 三档与恢复演练

> RTO/RPO 数值与演练频率的唯一事实源：[`contracts/registries/dr.yaml`](../../contracts/registries/dr.yaml) 的 `tiers[].rto` / `tiers[].rpo` / `tiers[].drill_frequency`（本文不复制数值）。

| 档 | 场景 | 依赖机制 |
|----|------|---------|
| L1 | Pod / 单实例应用故障 | K8s 自愈（探针）+ 滚动重建 |
| L2 | 数据库主库故障 | Patroni 自动 failover + 同步副本 |
| L3 | 数据库损坏 | PITR（物理基备 + WAL 连续归档）或逻辑恢复兜底 |

演练硬性要求：记录检测时间、恢复时间、实测 RTO/RPO 与目标值差距、数据一致性核验、告警证据与整改负责人。实测未达标即登记差距项并追踪。

---

## 8. 应用层恢复速查

| 场景 | 恢复动作 |
|------|---------|
| Pod crash | Kubernetes auto-restarts (livenessProbe) |
| Deployment rollback | `kubectl rollout undo deployment/guliangiam` |
| Config misconfiguration | Revert ConfigMap/Secret, rolling restart |
| Database failover | Patroni 自动切换经读写 VIP 漂移；无 VIP 时更新 Secret 中 `POSTGRES_DSN` 后滚动重启 |
| Full cluster failure | Restore from backup on new cluster（PITR / 逻辑恢复），update DNS |

---

## 9. 备份验证清单

- [ ] Backup completes without errors（检查 cron/systemd 日志）
- [ ] Backup file size > 0 字节
- [ ] Offsite copy uploaded successfully（SHA256SUMS 核对）
- [ ] 每月单实例恢复演练通过（L1 档）
- [ ] 每季度数据库备份恢复演练通过（L2 档）
- [ ] 每季度 PITR 抽演通过（L3 档）
- [ ] WAL archiving lag < 5 分钟（`pg_stat_archiver` 无 failed 增长）
- [ ] 备份加密口令/KMS 引用可独立恢复（备份系统自身不是单点）
