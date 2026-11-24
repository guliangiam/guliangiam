import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type BrowserContext,
} from "@playwright/test";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { shouldRunE2E } from "./helpers";

/**
 * 业务写流程运行期验证（真实后端 + 真实数据库）。
 *
 * 覆盖四条会真正落库的写链路，断言「写 → 回读 → 状态/审计」闭环：
 *   1. WLI：身份池 → 联邦信任 → Service Account（一次性 Secret：创建/轮换各返回
 *      一次，随后 GET 永不回显）；同一套 API 驱动 /identity-pools 页面。
 *   2. IGA：访问活动创建 → 启动 → 完成（列表回读）；访问申请提交后进入
 *      pending_manager，自审被 403 拒绝（审批守卫）。
 *   3. IGA：经理 + SAP 两阶段审批全部通过（新建审批人并绑定 built-in-editor，
 *      验证用户角色绑定会投影为 Casbin g 规则，否则非管理员审批人会被 403）。
 *   4. Risk：复核一条登录风险评估决策 → 决策值/workflow/history 回读。
 *
 * 与 route-sweep 的分工：那里只证明页面能渲染，这里证明写接口在运行期可用。
 * 需要 `ENABLE_E2E=true` 与会话 storageState（auth.setup 预置）。
 */

const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

const AUTH_STATE = resolve(dirname(fileURLToPath(import.meta.url)), ".auth/admin.json");
// 与 route-sweep/a11y 共用 auth.setup 落盘的会话；`context.request` 会带上同一
// cookie jar，因此业务写接口与浏览器页面看到的是同一个会话。
if (existsSync(AUTH_STATE)) test.use({ storageState: AUTH_STATE });

// 独立 API 客户端需要绝对地址：Playwright 的 baseURL 只在 test fixture 上生效。
const API_BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:7101";
const APPROVER_PASSWORD = "Approver-E2e-2026!";

async function csrfToken(context: BrowserContext): Promise<string> {
  const cookies = await context.cookies();
  return cookies.find((cookie) => cookie.name === "csrf_token")?.value ?? "";
}

function mutationHeaders(token: string) {
  return { "X-CSRF-Token": token };
}

async function readOrganizationId(request: APIRequestContext): Promise<string> {
  const response = await request.get("/api/v1/auth/account");
  expect(
    response.ok(),
    `session 应可从 /api/v1/auth/account 读取 (status=${response.status()} body=${await response.text()})`,
  ).toBeTruthy();
  const body = (await response.json()) as { organization?: { id?: string } };
  const organizationId = body.organization?.id;
  expect(organizationId, "会话缺少 organization.id").toBeTruthy();
  return organizationId as string;
}

async function firstItemId(request: APIRequestContext, path: string): Promise<string> {
  const response = await request.get(path);
  expect(response.ok(), `${path} 应返回 2xx`).toBeTruthy();
  const body = (await response.json()) as { items?: Array<{ id?: string }> };
  const id = body.items?.[0]?.id;
  expect(id, `${path} 没有可用对象`).toBeTruthy();
  return id as string;
}

async function organizationSlug(request: APIRequestContext): Promise<string> {
  const response = await request.get("/api/v1/auth/account");
  expect(response.ok(), await response.text()).toBeTruthy();
  const body = (await response.json()) as { organization?: { name?: string } };
  expect(body.organization?.name, "会话缺少 organization.name").toBeTruthy();
  return body.organization?.name as string;
}

async function builtInEditorRoleId(
  request: APIRequestContext,
  organizationId: string,
): Promise<string> {
  const response = await request.get(
    `/api/v1/roles?page=1&pageSize=100&organizationId=${organizationId}`,
  );
  expect(response.ok(), await response.text()).toBeTruthy();
  const body = (await response.json()) as { items?: Array<{ id?: string; name?: string }> };
  const role = body.items?.find((item) => item.name === "built-in-editor") ?? body.items?.[0];
  expect(role?.id, "组织内必须至少有一个可审批的角色").toBeTruthy();
  return role?.id as string;
}

/**
 * 审批人必须先通过用户写接口创建并绑定角色：实现会把绑定投影成 Casbin `g`
 * 规则。若缺少该投影，下面的 approve 请求会以 403 insufficient permissions 失败，
 * 因此这条用例同时是 K-3（角色绑定 → 授权）的端到端回归。
 */
async function createApprover(
  request: APIRequestContext,
  token: string,
  params: { organizationId: string; roleId: string; username: string },
): Promise<string> {
  const response = await request.post("/api/v1/users", {
    headers: mutationHeaders(token),
    data: {
      organizationId: params.organizationId,
      username: params.username,
      userType: "normal-user",
      status: "active",
      password: APPROVER_PASSWORD,
      roleBindingIds: [params.roleId],
      profile: {
        displayName: params.username,
        // user_profiles.email 有唯一索引，同一批次的两个审批人必须使用不同邮箱。
        email: `${params.username}@e2e.test`,
      },
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { id: string };
  expect(body.id, "创建审批人应返回用户 id").toBeTruthy();
  return body.id;
}

/**
 * 审批人需要独立会话：共享 context 的 cookie jar 会覆盖 admin 会话，所以用
 * playwright.request.newContext() 拿一套隔离的 cookie。
 */
async function loginWithIsolatedContext(params: {
  username: string;
  organizationSlug: string;
  applicationId: string;
}): Promise<APIRequestContext> {
  const client = await apiRequest.newContext({
    baseURL: API_BASE_URL,
    // 显式清空存储：即使测试带了 storageState，也要保证审批人不会复用 admin
    // 会话——登录会按 S9 撤销请求里携带的旧会话，复用会把 admin 一起登出。
    storageState: { cookies: [], origins: [] },
  });
  // GET /auth/account 即使返回 401 也会下发 csrf_token cookie（Double Submit）。
  await client.get("/api/v1/auth/account");
  const state = await client.storageState();
  const csrf = state.cookies.find((cookie) => cookie.name === "csrf_token")?.value ?? "";
  const login = await client.post("/api/v1/auth/login", {
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    data: {
      username: params.username,
      password: APPROVER_PASSWORD,
      organizationSlug: params.organizationSlug,
      applicationId: params.applicationId,
    },
  });
  expect(login.ok(), `审批人登录失败 (${login.status()}): ${await login.text()}`).toBeTruthy();
  return client;
}

describeE2E("业务写流程运行期验证", () => {
  test.beforeEach(() => {
    // 写流程只需跑一遍，避免两套配色各写一次数据。
    test.skip(test.info().project.name !== "chromium-light", "写流程只在 chromium-light 执行");
  });

  test("WLI: 身份池 → 联邦信任 → Service Account 一次性 Secret", async ({ context }) => {
    const request = context.request;
    const organizationId = await readOrganizationId(request);
    const token = await csrfToken(context);
    const suffix = Date.now();
    const query = `organizationId=${organizationId}`;

    const poolResponse = await request.post(`/api/v1/identity-pools?${query}`, {
      headers: mutationHeaders(token),
      data: { name: `e2e-pool-${suffix}`, displayName: `E2E Pool ${suffix}` },
    });
    expect(poolResponse.status(), await poolResponse.text()).toBe(201);
    const pool = (await poolResponse.json()) as { id: string; name: string };
    expect(pool.name).toBe(`e2e-pool-${suffix}`);

    const trustResponse = await request.post(
      `/api/v1/identity-pools/${pool.id}/federation-trusts?${query}`,
      {
        headers: mutationHeaders(token),
        data: {
          name: `e2e-trust-${suffix}`,
          issuer: "https://issuer.e2e.test",
          jwksUri: "https://issuer.e2e.test/jwks.json",
          audience: ["e2e"],
          subjects: ["repo:example/*"],
        },
      },
    );
    expect(trustResponse.status(), await trustResponse.text()).toBe(201);
    const trust = (await trustResponse.json()) as { id: string; status: string };
    expect(trust.status).toBe("active");

    const accountResponse = await request.post(
      `/api/v1/identity-pools/${pool.id}/service-accounts?${query}`,
      {
        headers: mutationHeaders(token),
        data: {
          name: `e2e-sa-${suffix}`,
          scopes: ["read"],
          accessTokenTtlSeconds: 900,
        },
      },
    );
    expect(accountResponse.status(), await accountResponse.text()).toBe(201);
    const account = (await accountResponse.json()) as {
      id: string;
      clientSecretPlain?: string;
      clientId: string;
      revision: number;
      secretSet: boolean;
    };
    expect(account.clientSecretPlain, "创建响应必须一次性返回明文 Secret").toMatch(/^gl_cs_/);
    expect(account.secretSet).toBe(true);

    const fetched = await request.get(
      `/api/v1/identity-pools/${pool.id}/service-accounts/${account.id}?${query}`,
    );
    expect(fetched.ok()).toBeTruthy();
    const fetchedBody = (await fetched.json()) as {
      clientSecretPlain?: string;
      secretSet: boolean;
      clientId: string;
    };
    expect(fetchedBody.clientSecretPlain, "回读不得再暴露明文 Secret").toBeUndefined();
    expect(fetchedBody.secretSet).toBe(true);
    expect(fetchedBody.clientId).toBe(account.clientId);

    const rotatedResponse = await request.post(
      `/api/v1/identity-pools/${pool.id}/service-accounts/${account.id}/rotate-secret?${query}`,
      { headers: mutationHeaders(token), data: { revision: account.revision } },
    );
    expect(rotatedResponse.status(), await rotatedResponse.text()).toBe(200);
    const rotated = (await rotatedResponse.json()) as { clientSecretPlain?: string };
    expect(rotated.clientSecretPlain, "轮换必须返回新的一次性 Secret").toMatch(/^gl_cs_/);
    expect(rotated.clientSecretPlain).not.toBe(account.clientSecretPlain);

    // UI 回读：/identity-pools 页面能列出新建的身份池。
    const poolList = await request.get(`/api/v1/identity-pools?${query}`);
    expect(poolList.ok()).toBeTruthy();
    const poolListBody = (await poolList.json()) as { items?: Array<{ id: string }> };
    expect(poolListBody.items?.some((item) => item.id === pool.id)).toBe(true);
  });

  test("IGA: 访问活动 创建 → 启动 → 完成", async ({ context }) => {
    const request = context.request;
    await readOrganizationId(request);
    const token = await csrfToken(context);
    const suffix = Date.now();
    const startDate = new Date().toISOString();
    const endDate = new Date(Date.now() + 86_400_000).toISOString();

    const created = await request.post("/api/v1/iga/access-campaigns", {
      headers: mutationHeaders(token),
      data: { name: `e2e-campaign-${suffix}`, startDate, endDate },
    });
    expect(created.status(), await created.text()).toBe(201);
    const campaign = (await created.json()) as { id: string; name: string; status: string };
    expect(campaign.status).toBe("draft");

    const activated = await request.patch(`/api/v1/iga/access-campaigns/${campaign.id}/status`, {
      headers: mutationHeaders(token),
      data: { status: "active" },
    });
    expect(activated.ok(), await activated.text()).toBeTruthy();
    expect(((await activated.json()) as { status: string }).status).toBe("active");

    const completed = await request.patch(`/api/v1/iga/access-campaigns/${campaign.id}/status`, {
      headers: mutationHeaders(token),
      data: { status: "completed" },
    });
    expect(completed.ok(), await completed.text()).toBeTruthy();
    expect(((await completed.json()) as { status: string }).status).toBe("completed");

    const list = await request.get("/api/v1/iga/access-campaigns");
    expect(list.ok()).toBeTruthy();
    const listBody = (await list.json()) as { items?: Array<{ id: string; status: string }> };
    const persisted = listBody.items?.find((item) => item.id === campaign.id);
    expect(persisted?.status, "列表回读应看到 completed 状态").toBe("completed");
  });

  test("IGA: 访问申请提交后进入 pending_manager，自审被 403 拒绝", async ({ context }) => {
    const request = context.request;
    const organizationId = await readOrganizationId(request);
    const token = await csrfToken(context);
    const suffix = Date.now();

    const roleId = await firstItemId(
      request,
      `/api/v1/roles?page=1&pageSize=1&organizationId=${organizationId}`,
    );
    const applicationId = await firstItemId(request, "/api/v1/applications?page=1&pageSize=1");
    const enforcerId = await firstItemId(
      request,
      `/api/v1/enforcers?page=1&pageSize=1&organizationId=${organizationId}`,
    );

    const created = await request.post("/api/v1/iga/access-requests", {
      headers: mutationHeaders(token),
      data: {
        roleId,
        applicationId,
        enforcerId,
        resource: `e2e-resource-${suffix}`,
        action: "read",
        reason: "runtime verification",
        startsAt: new Date().toISOString(),
        expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
      },
    });
    expect(created.status(), await created.text()).toBe(201);
    const ticket = (await created.json()) as { id: string; status: string; revision: number };
    expect(ticket.status).toBe("submitted");

    const submitted = await request.post(`/api/v1/iga/access-requests/${ticket.id}/submit`, {
      headers: mutationHeaders(token),
      data: { revision: ticket.revision, comment: "submit" },
    });
    expect(submitted.ok(), await submitted.text()).toBeTruthy();
    const pending = (await submitted.json()) as { status: string; revision: number };
    expect(pending.status).toBe("pending_manager");

    const selfApprove = await request.post(`/api/v1/iga/access-requests/${ticket.id}/approve`, {
      headers: mutationHeaders(token),
      data: { stage: "manager", revision: pending.revision },
    });
    expect(selfApprove.status(), "申请人不能自审").toBe(403);

    const detail = await request.get(`/api/v1/iga/access-requests/${ticket.id}`);
    expect(detail.ok()).toBeTruthy();
    const detailBody = (await detail.json()) as {
      request?: { status: string };
      messages?: Array<{ action?: string }>;
      approvalChain?: string[];
    };
    expect(detailBody.request?.status).toBe("pending_manager");
    expect(detailBody.messages?.length, "审批流水应记录提交事件").toBeGreaterThan(0);
    expect(detailBody.approvalChain?.length).toBeGreaterThan(0);
  });

  test("IGA: 经理 + SAP 两阶段审批全流程", async ({ context }) => {
    const request = context.request;
    const organizationId = await readOrganizationId(request);
    const orgSlug = await organizationSlug(request);
    const token = await csrfToken(context);
    const applicationId = await firstItemId(request, "/api/v1/applications?page=1&pageSize=1");
    const enforcerId = await firstItemId(
      request,
      `/api/v1/enforcers?page=1&pageSize=1&organizationId=${organizationId}`,
    );
    const roleId = await builtInEditorRoleId(request, organizationId);
    const suffix = Date.now();
    const managerUsername = `e2e_mgr_${suffix}`;
    const sapUsername = `e2e_sap_${suffix}`;

    // 两个审批人都通过写接口创建并绑定角色；能登录并审批即证明角色绑定已被
    // 投影为 Casbin g 规则（否则是非管理员主体 → 403）。
    const managerId = await createApprover(request, token, {
      organizationId,
      roleId,
      username: managerUsername,
    });
    const sapId = await createApprover(request, token, {
      organizationId,
      roleId,
      username: sapUsername,
    });

    const managerClient = await loginWithIsolatedContext({
      username: managerUsername,
      organizationSlug: orgSlug,
      applicationId,
    });
    const sapClient = await loginWithIsolatedContext({
      username: sapUsername,
      organizationSlug: orgSlug,
      applicationId,
    });

    try {
      const created = await request.post("/api/v1/iga/access-requests", {
        headers: mutationHeaders(token),
        data: {
          roleId,
          applicationId,
          enforcerId,
          resource: `e2e-two-stage-${suffix}`,
          action: "read",
          reason: "two-stage approval runtime verification",
          startsAt: new Date().toISOString(),
          expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
        },
      });
      expect(created.status(), await created.text()).toBe(201);
      const ticket = (await created.json()) as { id: string; status: string; revision: number };
      expect(ticket.status).toBe("submitted");

      const submitted = await request.post(`/api/v1/iga/access-requests/${ticket.id}/submit`, {
        headers: mutationHeaders(token),
        data: { revision: ticket.revision, comment: "submit" },
      });
      expect(submitted.ok(), await submitted.text()).toBeTruthy();
      const pending = (await submitted.json()) as { status: string; revision: number };
      expect(pending.status).toBe("pending_manager");

      const managerCsrf = (await managerClient.storageState()).cookies.find(
        (cookie) => cookie.name === "csrf_token",
      )?.value;
      const managerApproved = await managerClient.post(
        `/api/v1/iga/access-requests/${ticket.id}/approve`,
        {
          headers: { "X-CSRF-Token": managerCsrf ?? "" },
          data: { stage: "manager", revision: pending.revision, comment: "manager approve" },
        },
      );
      expect(managerApproved.status(), await managerApproved.text()).toBe(200);
      const afterManager = (await managerApproved.json()) as {
        status: string;
        revision: number;
        managerApproverId: string;
        sapApproverId: string;
      };
      expect(afterManager.status).toBe("pending_sap");
      expect(afterManager.managerApproverId).toBe(managerId);
      expect(afterManager.sapApproverId).toBe("");

      const sapCsrf = (await sapClient.storageState()).cookies.find(
        (cookie) => cookie.name === "csrf_token",
      )?.value;
      const sapApproved = await sapClient.post(`/api/v1/iga/access-requests/${ticket.id}/approve`, {
        headers: { "X-CSRF-Token": sapCsrf ?? "" },
        data: { stage: "sap", revision: afterManager.revision, comment: "sap approve" },
      });
      expect(sapApproved.status(), await sapApproved.text()).toBe(200);
      const afterSap = (await sapApproved.json()) as {
        status: string;
        managerApproverId: string;
        sapApproverId: string;
        lifecycleId: string;
      };
      expect(afterSap.status).toBe("provisioned");
      expect(afterSap.sapApproverId).toBe(sapId);
      expect(afterSap.managerApproverId).toBe(managerId);
      expect(afterSap.lifecycleId, "两阶段通过后应产生 JIT 生命周期").toBeTruthy();

      const detail = await request.get(`/api/v1/iga/access-requests/${ticket.id}`);
      expect(detail.ok()).toBeTruthy();
      const detailBody = (await detail.json()) as {
        request?: { status: string; managerApproverId: string; sapApproverId: string };
        messages?: Array<{ action?: string }>;
        approvalChain?: string[];
      };
      expect(detailBody.request?.status).toBe("provisioned");
      expect(detailBody.request?.managerApproverId).toBe(managerId);
      expect(detailBody.request?.sapApproverId).toBe(sapId);
      expect(detailBody.approvalChain).toEqual(["manager", "sap"]);
      const actions = detailBody.messages?.map((message) => message.action) ?? [];
      expect(actions).toContain("approved_manager");
      expect(actions).toContain("provisioned");
    } finally {
      await managerClient.dispose();
      await sapClient.dispose();
    }
  });

  test("Provider: 创建 → 详情回读 → 编辑持久化", async ({ context }) => {
    const request = context.request;
    const organizationId = await readOrganizationId(request);
    const token = await csrfToken(context);
    const suffix = Date.now();
    const name = `e2e-provider-${suffix}`;

    const created = await request.post("/api/v1/providers", {
      headers: mutationHeaders(token),
      data: {
        organizationId,
        name,
        displayName: "E2E Provider",
        category: "OAuth",
        providerType: "Google",
        clientId: `e2e-client-${suffix}`,
        clientSecret: "e2e-secret-value",
        authUrl: "https://accounts.google.com/o/oauth2/v2/auth",
        tokenUrl: "https://oauth2.googleapis.com/token",
        userInfoUrl: "https://openidconnect.googleapis.com/v1/userinfo",
        scopes: ["openid", "email"],
        userMapping: { id: "sub", email: "email" },
        port: 443,
      },
    });
    // 表单与请求结构必须逐字段对齐：DisallowUnknownFields 下多一个字段就是 400。
    expect(created.status(), await created.text()).toBe(201);

    const list = await request.get(`/api/v1/providers?keyword=${name}`);
    expect(list.ok(), await list.text()).toBeTruthy();
    const listBody = (await list.json()) as { items?: Array<{ id: string; name: string }> };
    const row = listBody.items?.find((item) => item.name === name);
    expect(row?.id, "列表应能按名称查到新建提供商").toBeTruthy();

    const detail = await request.get(`/api/v1/providers/${row!.id}`);
    expect(detail.ok(), await detail.text()).toBeTruthy();
    const detailBody = (await detail.json()) as {
      displayName: string;
      authUrl: string;
      tokenUrl: string;
      scopes: string[];
      userMapping: Record<string, unknown>;
      clientSecretSet: boolean;
      clientSecret?: string;
      revision: number;
    };
    expect(detailBody.authUrl).toBe("https://accounts.google.com/o/oauth2/v2/auth");
    expect(detailBody.tokenUrl).toBe("https://oauth2.googleapis.com/token");
    expect(detailBody.scopes).toEqual(["openid", "email"]);
    expect(detailBody.userMapping).toEqual({ id: "sub", email: "email" });
    expect(detailBody.clientSecretSet, "客户端密钥应记录为已设置").toBe(true);
    expect(detailBody.clientSecret, "详情不得回显明文密钥").toBeUndefined();

    const patched = await request.patch(`/api/v1/providers/${row!.id}`, {
      headers: mutationHeaders(token),
      data: { displayName: "E2E Provider Updated", revision: detailBody.revision },
    });
    expect(patched.ok(), await patched.text()).toBeTruthy();

    const reread = await request.get(`/api/v1/providers/${row!.id}`);
    const rereadBody = (await reread.json()) as { displayName: string; authUrl: string };
    expect(rereadBody.displayName, "编辑后的显示名称应持久化").toBe("E2E Provider Updated");
    expect(rereadBody.authUrl, "未提交的字段不应被补丁清空").toBe(
      "https://accounts.google.com/o/oauth2/v2/auth",
    );
  });

  test("Risk: 复核决策 → 新决策/复核人/历史回读", async ({ context }) => {
    const request = context.request;
    await readOrganizationId(request);
    const token = await csrfToken(context);

    const list = await request.get("/api/v1/risk/decisions?page=1&pageSize=20");
    expect(list.ok()).toBeTruthy();
    const listBody = (await list.json()) as {
      items?: Array<{
        id: string;
        revision: number;
        workflow?: { review?: { status?: string } };
      }>;
    };
    const candidate = listBody.items?.find((item) => item.workflow?.review?.status === "pending");
    expect(candidate, "登录风险应至少产生一条待复核决策").toBeTruthy();

    const reviewed = await request.post(`/api/v1/risk/decisions/${candidate!.id}/review`, {
      headers: mutationHeaders(token),
      data: {
        decision: "deny",
        reason: `e2e runtime review ${Date.now()}`,
        revision: candidate!.revision,
      },
    });
    expect(reviewed.status(), await reviewed.text()).toBe(201);
    const reviewedBody = (await reviewed.json()) as {
      decision: string;
      workflow?: {
        review?: { status?: string; actorId?: string; reason?: string };
        history?: Array<{ type?: string }>;
      };
    };
    expect(reviewedBody.decision).toBe("deny");
    expect(reviewedBody.workflow?.review?.status).toBe("completed");
    expect(reviewedBody.workflow?.review?.actorId).toBeTruthy();
    expect(
      reviewedBody.workflow?.history?.some((entry) => entry.type === "decision.reviewed"),
    ).toBe(true);

    const fetched = await request.get(`/api/v1/risk/decisions/${candidate!.id}`);
    expect(fetched.ok()).toBeTruthy();
    expect(((await fetched.json()) as { decision: string }).decision).toBe("deny");

    // 同一工作台的另外两个读面在运行期可用（signals / session-governance）。
    expect((await request.get("/api/v1/risk/signals?page=1&pageSize=5")).ok()).toBeTruthy();
    expect((await request.get("/api/v1/risk/session-governance?page=1&pageSize=5")).ok()).toBe(
      true,
    );
  });
});
