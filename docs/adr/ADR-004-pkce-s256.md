# ADR-004: Authorization Code + PKCE S256

- 状态：[Current] — PKCE S256 策略已落地，`internal/app/auth` 整包测试绿灯（见验证证据）
- 日期：2026-09-14

## 背景

浏览器与移动端属于公开客户端（Public Client），无法安全保管客户端密钥；授权码一旦在重定向环节被截获，若无附加证明，攻击者即可用码换令牌。Implicit Grant 把令牌直接放进 URL 片段，经浏览器历史、Referer 与日志泄露的风险高，且已被 OAuth 2.1 废弃。项目在协议面建设初期即须选定唯一授权模式并把校验做成强制分支，而不是作为可选项。

## 决策

浏览器和移动端 OAuth/OIDC 默认且仅允许使用 Authorization Code + PKCE S256，禁止 implicit grant；redirect URI 必须与授权阶段登记值精确匹配（不做前缀/通配匹配）；对声明要求 PKCE 的应用（`PkceRequired`），无 challenge 或无 verifier 一律拒绝。

## 理由与后果

- 正面：截获授权码但拿不到 `code_verifier` 的攻击者无法完成换码；S256 使 challenge 在信道中只以摘要形式出现；redirect 精确匹配封死借壳回调；重复 code、过期 code、错误客户端均返回统一错误，不泄露码状态细节。
- 负面：客户端集成需要实现 verifier 生成与 SHA-256 计算，接入成本略高于 implicit；历史 implicit 客户端不兼容（有意为之，不提供回退）；服务端必须保证 PKCE 校验真正位于换码强制路径上并被测试覆盖。

## 替代方案

- **保留 Implicit Grant 作为兼容选项**：放弃理由——令牌经 URL 片段泄露、无刷新令牌、无法被发送方约束（见 ADR-014），OAuth 2.1 已删除该模式。
- **允许 PKCE `plain` 方法**：放弃理由——plain 不提供任何截获防护；代码对非 S256 方法直接拒绝（`internal/app/auth/validation.go` 的 `validatePKCEBinding`、`token_exchange.go` 的 `validateAuthorizationCodeExchange`），不留配置口子。
- **仅对机密客户端强制 PKCE、公开客户端可选**：放弃理由——公开客户端恰是码截获风险最高的群体；实现上授权与换码双侧均设强制分支（`authorize.go` 调 `validatePKCEBinding`；`validateAuthorizationCodeExchange` 对缺 challenge/非 S256/缺 verifier 一律拒绝）。

## 验证证据（Current）

以下命令均可在仓库根复现（锚点以符号名为准，行号随代码演进；核验 2026-11-21）：

- `grep -n 'S256' internal/app/auth/validation.go` → `validatePKCEBinding`：`codeChallengeMethod` 非空且不为 `S256` 即返回 `codeChallengeMethod must be S256 when provided`；方法与 challenge 必须成对出现。
- `grep -n 'IsPKCEMethodValid' internal/domain/authorizationcodes/code.go` → 领域模型仅承认 `S256`。
- `grep -n 'validateAuthorizationCodeExchange\|buildPKCEChallenge' internal/app/auth/token_exchange.go` 核对换码强制路径：过期/redirect URI 不一致统一返回 `authorization code is invalid or expired`（精确匹配、不泄露码状态）；缺 challenge、方法非 S256、缺 `codeVerifier`、`buildPKCEChallenge`（SHA-256 + base64url 无填充）比对失败均拒绝。现行实现对**所有客户端**（公开与机密）强制 PKCE（OAuth 2.1 I-D §4.1.1），是本决策「PkceRequired 应用强制」的超集收紧。
- `go test ./internal/domain/authorizationcodes/ -count=1` → ok，授权码/PKCE 领域校验通过。
- `go test ./internal/app/auth/ -count=1` → `ok github.com/guliangiam/guliangiam/internal/app/auth 30.518s`（2026-11-21），整包绿灯，PKCE 用例（错误 verifier、缺 challenge、缺 method、非法 method）随整包通过。

## 复审条件

OAuth/OIDC 规范更新对 PKCE 的强制范围或算法提出新要求；引入需要单独授权安全配置（RAR、JARM 等）的新 grant 类型时连带复核。
