/**
 * 注册页默认《使用条款》与《隐私政策》正文。
 *
 * 应用可在配置中提供自定义 termsOfUse（HTML）覆盖默认使用条款；隐私政策默认由
 * 平台统一提供。正文按 GuliangIAM 多租户身份与访问管理（IAM）产品的设计撰写：
 * 涵盖账号注册、用户义务、个人信息与敏感信息（实名证件、人脸底图加密存储、MFA）
 * 处理、第三方登录、数据共享与委托处理、留存与权利、安全措施、免责与责任限制等。
 *
 * 这是内容（如同应用配置的 termsOfUse HTML），不是 UI 文案键，故不走 i18n 键 parity；
 * 渲染前必须经 sanitizeHtml 净化。zh* 语言返回中文，其余回退英文。
 */

function isChinese(lang: string): boolean {
  return /^zh\b/i.test(lang || "");
}

const TERMS_ZH = `
<h2>使用条款</h2>
<p>欢迎使用 GuliangIAM（以下简称“本服务”）。本服务由平台运营方及其租户（组织）提供，
用于身份注册、认证、授权与账号安全管理。注册或使用本服务即表示您已阅读并同意本条款。
若您代表某一组织使用本服务，您须确保已获得该组织的合法授权。</p>
<h3>1. 账号注册与资格</h3>
<p>您须以真实、准确、完整的信息注册账号，并在信息变更时及时更新。您须达到所在司法辖区
规定的最低年龄，并具备订立本条款的法律能力。账号凭据（口令、验证码、MFA 因子等）由您
自行保管，因您保管不善导致的损失由您承担。</p>
<h3>2. 用户义务与禁止行为</h3>
<p>您不得利用本服务从事任何违反法律法规、侵犯他人合法权益或破坏系统安全的行为，包括但
不限于：冒用他人身份、批量恶意注册、暴力破解、绕过认证与限流、上传违法内容、干扰服务
正常运行。我们有权对异常行为采取限流、冻结或终止账号等措施。</p>
<h3>3. 认证与安全</h3>
<p>本服务支持口令、邮箱/手机验证码、多因素认证（MFA）、WebAuthn/通行密钥、人脸核身与
第三方登录等方式。为保障安全，我们可能要求人机验证（验证码）、实名核验或额外的身份
证明。您理解并同意这些措施用于防止自动化滥用与身份冒用。</p>
<h3>4. 知识产权</h3>
<p>本服务的软件、界面、文档与相关知识产权归运营方或相应权利人所有。除法律允许或书面
授权外，您不得复制、修改、反向工程或用于衍生产品。</p>
<h3>5. 服务变更与中断</h3>
<p>我们可能因维护、升级、安全或合规需要变更、暂停或终止部分或全部服务，并尽可能提前
通知。因不可抗力、基础电信故障、第三方服务中断等导致的服务不可用，我们在法律允许范围
内不承担责任。</p>
<h3>6. 免责与责任限制</h3>
<p>本服务按“现状”提供。在法律允许的最大范围内，运营方不对间接、附带、特殊或惩罚性损害
承担责任；对直接损害的累计责任以您就相关服务实际支付的费用（如有）为限。</p>
<h3>7. 条款修订与适用法律</h3>
<p>我们可能适时修订本条款，修订后将在页面公布。继续使用即视为接受修订。本条款的订立、
效力与解释适用运营方所在地法律；争议应友好协商，协商不成的提交有管辖权的法院解决。</p>
`;

const PRIVACY_ZH = `
<h2>隐私政策</h2>
<p>本政策说明 GuliangIAM 如何收集、使用、存储、共享与保护您的个人信息，以及您享有的权利。
我们遵循合法、正当、必要与最小化原则处理个人信息。</p>
<h3>1. 我们收集的信息</h3>
<p>为提供身份与访问管理服务，我们可能收集：账号标识（用户名、显示名）、联系方式（邮箱、
手机号）、口令哈希（不存储明文口令）、个人资料（头像等您主动提供的信息）、登录与设备
信息（IP、时间、设备标识，用于安全审计）、以及您选择启用的认证因子（MFA 密钥、通行密钥
公钥、第三方登录绑定标识）。</p>
<h3>2. 敏感个人信息</h3>
<p>当您启用实名核验或人脸核身时，我们会处理证件信息与人脸特征。此类敏感信息仅在取得您的
单独同意后进行：证件号码等身份信息绝不写入日志；人脸底图不以明文图片存储，而以加密特征
形式保存；实名核验仅保留去标识化的核验结果引用。我们采取严格访问控制与加密保护。</p>
<h3>3. 信息的使用</h3>
<p>我们使用上述信息用于：完成注册与认证、保障账号与系统安全（风控、审计、限流）、提供
您请求的功能、履行法定义务。我们不会将个人信息用于与上述目的无关的用途。</p>
<h3>4. 共享、委托处理与披露</h3>
<p>我们不会出售您的个人信息。仅在以下情形共享：取得您的同意；为完成认证所必需的第三方
服务（如您选择的第三方登录、短信/邮件验证码服务商、实名/人脸核验服务商），且仅共享必要
字段；根据法律法规或监管要求。受托方须按我们的指示与保密义务处理信息。</p>
<h3>5. 存储与留存</h3>
<p>个人信息存储于受控数据库，按租户隔离。留存期限为实现目的所必需的最短时间，法律法规
另有规定的从其规定。超出留存期限后我们将删除或匿名化处理。</p>
<h3>6. 您的权利</h3>
<p>在适用法律范围内，您有权访问、更正、补充、删除您的个人信息，撤回同意，注销账号，以及
获取个人信息副本。您可通过个人资料页自助编辑大部分信息，或联系所在组织管理员/运营方行使
权利。撤回同意不影响撤回前基于同意已进行的处理。</p>
<h3>7. 安全措施</h3>
<p>我们采用加密存储、传输加密（TLS）、访问控制、最小权限、安全审计与脱敏展示等措施保护
个人信息。凭据类信息在读取接口一律脱敏。尽管已采取合理措施，互联网环境并非绝对安全。</p>
<h3>8. 未成年人保护</h3>
<p>本服务主要面向成年人。若您是未成年人，应在监护人指导下使用并取得其同意。</p>
<h3>9. 政策更新与联系</h3>
<p>我们可能适时更新本政策并在页面公布。如您对个人信息处理有任何疑问、投诉或行使权利，
请联系所在组织管理员或运营方公布的联系方式。</p>
`;

const TERMS_EN = `
<h2>Terms of Use</h2>
<p>Welcome to GuliangIAM (the "Service"). The Service is provided by the platform operator and
its tenants (organizations) for identity registration, authentication, authorization and account
security. By registering or using the Service you agree to these Terms. If you use the Service on
behalf of an organization, you must be duly authorized by it.</p>
<h3>1. Registration and eligibility</h3>
<p>You must register with truthful, accurate and complete information and keep it updated. You must
meet the minimum age and legal capacity required by your jurisdiction. You are responsible for
safeguarding your credentials (password, codes, MFA factors).</p>
<h3>2. User obligations and prohibited conduct</h3>
<p>You must not use the Service for unlawful, infringing or system-disrupting activities, including
identity impersonation, abusive bulk registration, brute force, bypassing authentication or rate
limits, uploading illegal content, or interfering with normal operation. We may rate-limit, freeze
or terminate accounts for abnormal behavior.</p>
<h3>3. Authentication and security</h3>
<p>The Service supports password, email/SMS codes, MFA, WebAuthn/passkeys, face verification and
third-party login. To protect security we may require human verification (captcha), real-name
verification or additional proof of identity. You agree these measures prevent automated abuse and
impersonation.</p>
<h3>4. Intellectual property</h3>
<p>The software, UI, documentation and related IP belong to the operator or respective rights
holders. Except as permitted by law or written authorization, you may not copy, modify, reverse
engineer or create derivative works.</p>
<h3>5. Changes and interruption</h3>
<p>We may change, suspend or terminate part or all of the Service for maintenance, upgrades, security
or compliance, with notice where practicable. To the extent permitted by law, we are not liable for
unavailability caused by force majeure, telecom failures or third-party outages.</p>
<h3>6. Disclaimer and limitation of liability</h3>
<p>The Service is provided "as is". To the maximum extent permitted by law, the operator is not liable
for indirect, incidental, special or punitive damages; aggregate liability for direct damages is
limited to fees you actually paid for the relevant Service (if any).</p>
<h3>7. Amendments and governing law</h3>
<p>We may amend these Terms and publish updates on the page. Continued use constitutes acceptance.
These Terms are governed by the laws of the operator's locality; disputes shall be resolved amicably
or by the competent court.</p>
`;

const PRIVACY_EN = `
<h2>Privacy Policy</h2>
<p>This Policy explains how GuliangIAM collects, uses, stores, shares and protects your personal
information, and your rights. We process personal information lawfully, fairly, necessarily and on a
data-minimization basis.</p>
<h3>1. Information we collect</h3>
<p>To provide identity and access management we may collect: account identifiers (username, display
name), contact details (email, phone), password hashes (never plaintext passwords), profile data you
provide (avatar, etc.), login and device information (IP, time, device identifiers for security
audit), and authentication factors you enable (MFA secrets, passkey public keys, third-party login
binding identifiers).</p>
<h3>2. Sensitive personal information</h3>
<p>When you enable real-name or face verification we process ID document information and facial
features. Such sensitive information is processed only with your separate consent: ID numbers are
never written to logs; face base images are not stored as plaintext pictures but as encrypted
features; real-name verification retains only a de-identified result reference. Strict access control
and encryption apply.</p>
<h3>3. How we use information</h3>
<p>We use the above to: complete registration and authentication, protect account and system security
(risk control, audit, rate limiting), provide requested features, and meet legal obligations. We do
not use personal information for unrelated purposes.</p>
<h3>4. Sharing, entrusted processing and disclosure</h3>
<p>We do not sell your personal information. We share only: with your consent; with third-party
services necessary for authentication (your chosen third-party login, SMS/email code providers,
real-name/face verification providers), sharing only necessary fields; or as required by law or
regulators. Processors must act on our instructions and under confidentiality obligations.</p>
<h3>5. Storage and retention</h3>
<p>Personal information is stored in controlled databases with tenant isolation. Retention is the
shortest period necessary for the purposes, unless law provides otherwise. After retention we delete
or anonymize the data.</p>
<h3>6. Your rights</h3>
<p>Where applicable law grants them, you may access, correct, supplement or delete your personal
information, withdraw consent, close your account, and obtain a copy. You can self-edit most
information on the profile page, or contact your organization administrator / the operator. Withdrawal
does not affect processing already carried out based on consent.</p>
<h3>7. Security measures</h3>
<p>We use encrypted storage, TLS in transit, access control, least privilege, security audit and
masked display. Credential fields are always masked on read APIs. Despite reasonable measures, no
internet environment is absolutely secure.</p>
<h3>8. Minors</h3>
<p>The Service is primarily for adults. If you are a minor, use it under guardian guidance and with
their consent.</p>
<h3>9. Updates and contact</h3>
<p>We may update this Policy and publish it on the page. For questions, complaints or to exercise your
rights regarding personal information, contact your organization administrator or the operator's
published contact.</p>
`;

export function defaultTermsOfUseHtml(lang: string): string {
  return isChinese(lang) ? TERMS_ZH : TERMS_EN;
}

export function defaultPrivacyPolicyHtml(lang: string): string {
  return isChinese(lang) ? PRIVACY_ZH : PRIVACY_EN;
}
