# 安全运营：Secret 轮换与密钥生命周期

> 目标态规范「安全运营：Secret 轮换与密钥生命周期」；写作约束见 [docs/README.md](../README.md)。

---

## 1. 安全基线

生产和非生产环境都禁止因缺少密钥而把敏感字段降级为明文。`NewFieldEncryptor()` 在所有环境缺少 `ENCRYPTION_KEY_CURRENT`（或 `ENCRYPTION_KEY`）时 `panic` 拒绝启动，`IsEnabled()` 恒为 `true`，加密器不存在"禁用"状态。密钥读取、轮换、失败和旧版本撤销必须产生安全审计事件，但审计事件不得包含密钥值。

---

## 2. 密钥轮换操作步骤

> 源章节：§3。生产密钥托管方式为 KMS/Vault（ADR-009/015）；信封版本化启用前，运维按以下双密钥变量步骤执行。

### 2.1 双人审批门禁

任何生产密钥轮换必须先走轮换工单，申请人与审批人必须为不同自然人（职责分离）。紧急轮换允许事后 24h 内补单，但执行时仍需第二名运维在线确认。

### 2.2 环境变量注入口径

| 环境变量 | 必填 | 说明 |
|---------|------|------|
| `ENCRYPTION_KEY_CURRENT` | 是（生产） | 当前活跃的加密密钥，32 字节，Base64 编码 |
| `ENCRYPTION_KEY_PREVIOUS` | 否 | 前一个加密密钥，仅用于解密回退 |
| `ENCRYPTION_KEY` | 否（向后兼容） | 旧版 fallback；新部署应优先使用 CURRENT |

密钥解析优先级：当前密钥 `ENCRYPTION_KEY_CURRENT > ENCRYPTION_KEY`；前一个密钥 `ENCRYPTION_KEY_PREVIOUS`（独立，无 fallback）。

### 2.3 轮换四步法

**步骤 1：生成新密钥（不回显）**

```bash
umask 077
openssl rand -base64 32 > /tmp/encryption-key-current.new
```

**步骤 2：以 K8s Secret 配置双密钥并滚动重启**

```bash
kubectl create secret generic guliangiam-encryption-keys \
  --from-file=ENCRYPTION_KEY_CURRENT=/tmp/encryption-key-current.new \
  --from-file=ENCRYPTION_KEY_PREVIOUS=/tmp/encryption-key-previous.old \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deployment/guliangiam
kubectl rollout status deployment/guliangiam
```

**步骤 3：批量重加密数据**

批量重加密由管理面作业/Admin CLI 触发（游标分批、断点续跑，禁止全量载入）。`RotateEncryptedData()` 行为：已用当前密钥加密 → 原值返回；用前一个密钥加密 → 解密后重加密并标记 `wasRotated=true`；无前一个密钥 → 原值返回。

**步骤 4：验证并移除旧密钥**

确认全量重加密完成（`wasRotated` 全部为 `false`）→ 从 Secret 中删除 `ENCRYPTION_KEY_PREVIOUS` → 滚动重启 → KMS/Vault 中保留旧密钥材料的审计化销毁记录。

### 2.4 回滚（双密钥热回退，轮换窗口内零停机）

回滚 = 切换 CURRENT 指针，不需要停止服务。已重加密的数据（由新密钥加密）在回退后用新密钥（现为 PREVIOUS）解密、旧密钥（现为 CURRENT）继续承接新写入，数据不丢失、无需从备份恢复。

**例外——根密钥灾难**：当怀疑 KMS 根密钥本身泄露或 KMS 整体失效时，热回退不适用。必须按 §4 L0/L1 应急时限执行阻断（摘流/隔离）、重建根密钥并重新解包/重封所有下层 KEK/DEK。

---

## 3. 其他高价值 Secret 轮换

### 3.1 JWT/JWKS

- 新密钥先发布为可签发 key，旧密钥保留为只读验证 key
- 新 Token 使用新 `kid`；旧 Token 在其最大生命周期内继续验证
- 验证窗口结束后撤销旧 `kid`，确认所有缓存和 OIDC Discovery 文档已失效

### 3.2 Session 和 Refresh Token

- 会话来源为仓库会话行（`sessions` 表）；不使用 HMAC 签名会话 Cookie / `SESSION_HMAC_KEY`，无独立会话密钥轮换；会话吊销走仓库状态 + 事件驱动失效
- Refresh Token 轮换必须保留 family id；发现重用时吊销整个 family
- 轮换不会直接打印或导出 Token 明文；验证只能通过受控探针完成

### 3.3 Webhook、Provider 和 TLS

- Webhook/Provider Secret：重新生成 → 双端短期重叠 → 验证投递 → 撤销旧 Secret
- TLS 证书续期后验证完整链、SNI、OCSP/吊销状态和反向代理配置
- 任何 Secret 轮换必须记录 actor、resource、key id、时间、结果和 request/trace id，不记录 Secret 值

### 3.4 TLS 配置

cert-manager 自动签发续期（ACME `letsencrypt-prod`）。Ingress 注解加固：TLSv1.2+TLSv1.3、ECDHE-ECDSA/RSA AES-GCM 套件、HSTS 一年 + preload。手动 TLS（非 cert-manager 环境）禁止在生产使用。

---

## 4. 密钥泄露应急响应

> 源章节：《[GuliangIAM-开发规范](../GuliangIAM-开发规范.md)》§14.3–14.4（密钥轮换与管理红线）；轮换操作步骤见本文 §2，安全事件环境基线见 [runbook.md](runbook.md) §7。

| 级别 | 确认 | 阻断 | 轮换 | 通知目标 |
|------|------|------|------|---------|
| L0 根密钥 | 15min | 30min | 1h | 24h |
| L1 数据加密 | 30min | 1h | 4h | 48h |
| L2 会话签名 | 1h | 2h | 8h | 72h |
| L3 JWT/JWKS | 1h | 2h | 24h | 72h |
| L4 外部 Secret | 4h | 8h | 48h | 1 周 |

流程：确认范围 → 阻断访问 → 轮换/撤销 → 吊销相关 Session/Token → 审计追溯 → 合规判断 → 通知 → 复盘和复演。

**批量吊销要求**：管理面提供批量吊销入口（API/后台作业，游标分批、幂等、可续跑）；吊销即写影响面审计；JWKS/OIDC 缓存与各副本完成失效联动验证；每季度演练批量吊销并记录实际耗时。

---

## 5. 密钥分类、生命周期与合规

### 5.1 密钥分级

| 级别 | 资产 | 存储方式 | 常规轮换周期 |
|------|------|---------|-------------|
| L0 信任根 / Root KEK | KMS/HSM 主密钥 | KMS/HSM，不可导出 | ≤365 天或按供应商策略 |
| L1 KEK 与字段 DEK | 信封加密中间层 + `ENCRYPTION_KEY_CURRENT` | KMS/Vault；信封版本化（启用前双版本 CURRENT/PREVIOUS） | KEK ≤180 天；DEK ≤90 天 |
| L2 会话完整性 | 会话来源为仓库会话行（`sessions` 表，DB 强制）；不使用 HMAC 签名 Cookie / `SESSION_HMAC_KEY` | 无（无独立密钥材料） | 随会话 TTL |
| L3 JWT/JWKS | RS256/ES256 signing key（按 `kid` 版本化） | KMS/私钥库；新 kid 先发布、旧 key 只读验证 | ≤90 天；旧 kid 保留覆盖最长 token 生命周期 |
| L4 外部与基础设施 Secret | Provider/Webhook/LDAP Secret、DB 密码、TLS 私钥 | 外部 Secret 密文入库；DB 密码在 Secret Manager；TLS 私钥在证书 Secret | 外部 Secret ≤90 天；DB 密码 ≤90 天；TLS 私钥随证书续期更换 |
| L5 Token 类凭据 | Refresh Token、Access Secret | 只存哈希（SHA-256），不落明文 | 短期、滑动旋转；重用即吊销 family |

### 5.2 密钥生命周期状态机

> 完整状态机定义见 [../generated/state-machine-registry.md](../generated/state-machine-registry.md) §3.3。

主干迁移：`pending → active → retiring → retired`。合法状态取值：`pending` / `active` / `retiring` / `retired` / `revoked`。关键边：`retiring` 只允许迁往 `retired` / `revoked`，**不存在 `retiring → active` 热回退边**——轮换窗口需要回退时走新版本密钥发布流程，不得把 retiring 密钥改回 active；`revoked` 可从任一非终态（`pending` / `active` / `retiring`）跳入（泄露场景优先走 `revoked`）。状态机唯一事实源：`contracts/registries/state-machines.yaml` 的 `encryption_keys` 条目（代码生成 `internal/domain/encryption_keys/state_generated.go`）。

### 5.3 轮换审计字段口径

轮换审计统一字段集：`resource_type` + `resource_id`（多态引用）、actor、ip、user_agent、工单号、结果、request_id/trace_id、`old_version`/`new_version`。不记录密钥值本身。

### 5.4 合规映射

| 标准 | 控制 | 证据 |
|------|------|------|
| 等保 2.0 | 密码技术、密钥管理、审计 | 加密清单、轮换记录、审计查询 |
| GDPR | 字段加密、密钥分离、生命周期 | 字段清单、环境矩阵、擦除记录 |
| SOC 2 | 轮换、访问控制、变更审计 | KMS IAM、审批和发布证据 |
| PCI DSS（如适用） | HSM/双人审批/轮换 | KMS/HSM 配置和审批记录 |

---

## 6. API 参考

### 6.1 NewFieldEncryptor()

从环境变量创建字段加密器。任何环境未设置密钥时 `panic`（拒绝启动）。

### 6.2 EncryptField(plaintext) / DecryptField(ciphertext)

加密：使用当前密钥加密。解密：优先当前密钥，失败回退前一个密钥，回退成功时输出警告日志提示需重加密。

### 6.3 RotateEncryptedData(ciphertext)

将使用前一个密钥加密的数据重加密为当前密钥。返回新密文（或原值）、是否实际执行重加密、错误。行为矩阵：空字符串 → 原值；无前一个密钥 → 原值；已用当前密钥 → 原值；用前一个密钥 → 重加密；前一个密钥也无法解密 → 错误。

---

## 7. 安全注意事项

1. 密钥应通过 KMS/Vault 注入，禁止硬编码在代码或配置文件中
2. 密钥在进程间传输时应使用加密通道，避免通过日志、监控系统泄露
3. 不存在全资产统一"90 天"周期——按 §5.1 分级表分层执行；任何层级一经确认泄露立即应急轮换（§4），不等待周期
4. 轮换完成后及时移除 `ENCRYPTION_KEY_PREVIOUS`，减少攻击面
5. 日志在解密回退时输出警告，但**不会**记录密钥值或明文数据
6. `FieldEncryptor` 并发安全；但 `GlobalFieldEncryptor()` 的首次调用和密钥轮换不应并发执行
7. 执行批量重加密前应备份数据库（backup-dr.md）
8. 对 `HasPreviousKey() == true` 的持续时间设置告警；轮换状态持续超过 24h 触发
9. 不同环境（开发/测试/生产）必须使用不同密钥，禁止跨环境复用
10. 旧密钥从环境变量移除后，应确保其在内存中的引用也被 GC 回收
