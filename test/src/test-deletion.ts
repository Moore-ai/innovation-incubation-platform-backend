// 账号注销功能集成测试（完整版：包含 SSE 实时推送验证）
// 运行: npx tsx src/test-deletion.ts
// 需要先启动后端服务器

import { ApiClient } from "./api.js";

const api = new ApiClient();
const carApi = new ApiClient();
const govApi = new ApiClient();
const ts = Date.now();
const GOV_PHONE = `199${ts}`;

async function assertOk(label: string, ok: boolean) {
  console.log(`  ${ok ? "✅" : "❌"} ${label}`);
  if (!ok) process.exitCode = 1;
}

/** 通过 SSE 订阅获取 init 事件中的通知列表 */
async function fetchNotifications(token: string, timeoutMs = 3000): Promise<any[]> {
  const ac = new AbortController();
  const timer = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const resp = await fetch("http://localhost:8080/api/v1/notifications/subscribe", {
      headers: { Authorization: `Bearer ${token}` },
      signal: ac.signal,
    });
    if (!resp.ok) return [];
    const reader = resp.body?.getReader();
    if (!reader) return [];
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      if (buffer.includes("\n\n")) break;
    }
    ac.abort();
    const match = buffer.match(/event: init\ndata: (\[.*?\])\n\n/);
    if (match) return JSON.parse(match[1]);
  } catch { /* skip */ }
  clearTimeout(timer);
  return [];
}

/** 保持 SSE 连接打开，等待一条 event:update 实时通知 */
async function waitForSSEUpdate(
  token: string,
  trigger: () => Promise<void>,
  timeoutMs = 5000,
): Promise<any | null> {
  const ac = new AbortController();
  const timer = setTimeout(() => ac.abort(), timeoutMs);
  const resp = await fetch("http://localhost:8080/api/v1/notifications/subscribe", {
    headers: { Authorization: `Bearer ${token}` },
    signal: ac.signal,
  });
  if (!resp.ok) { clearTimeout(timer); return null; }
  const reader = resp.body?.getReader();
  if (!reader) { clearTimeout(timer); return null; }
  const decoder = new TextDecoder();
  let buffer = "";

  // 消费 init 事件
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    if (buffer.includes("\n\n")) break;
  }

  // 执行触发操作
  await trigger();

  // 等待 event: update
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const match = buffer.match(/event: update\ndata: ({.*?})\n\n/);
    if (match) {
      clearTimeout(timer);
      ac.abort();
      return JSON.parse(match[1]);
    }
  }
  clearTimeout(timer);
  return null;
}

/** 通过 psql 创建政务账号并登录获取 token */
async function createGovernmentUser(): Promise<string> {
  const reg = await api.post("/auth/register", {
    password: "test123456", role: "enterprise", phone: GOV_PHONE,
    enterprise_name: `临时${ts}`, enterprise_credit_code: `TMP${ts}`,
    enterprise_industry: "IT", enterprise_scale: "小型",
  });
  assertOk("临时用户注册成功", reg.code === 0);
  api.setToken(reg.data!.token);

  const me = await api.get("/auth/me");
  const userId = me.data?.id;
  assertOk("获取到 user_id", typeof userId === "number" && userId > 0);

  const { execSync } = await import("child_process");
  const env = { ...process.env, PGPASSWORD: "Moore20060810" };
  const psql = (sql: string) =>
    execSync(`psql -h 127.0.0.1 -p 5433 -U Moore -d incubation_platform -c "${sql}"`, { env, stdio: "pipe" });

  psql(`DELETE FROM enterprises WHERE user_id = ${userId}`);
  psql(`UPDATE users SET role = 'government', phone = '${GOV_PHONE}' WHERE id = ${userId}`);

  const govLogin = await govApi.post("/auth/login", {
    credential: GOV_PHONE, password: "test123456", role: "government",
  });
  assertOk("政务账号创建并登录成功", govLogin.code === 0);
  return govLogin.data!.token;
}

async function main() {
  // ====================== 准备 ======================
  console.log("=== 准备: 注册账号 ===");
  const regEnt = await api.post("/auth/register", {
    password: "test123456", role: "enterprise", phone: `192${ts}`,
    enterprise_name: `注销企业${ts}`, enterprise_credit_code: `DEL${ts}`,
    enterprise_industry: "信息技术", enterprise_scale: "小型",
  });
  assertOk("注册企业成功", regEnt.code === 0);
  const entToken = regEnt.data!.token;

  const regCar = await carApi.post("/auth/register", {
    password: "test123456", role: "carrier", phone: `193${ts}`,
    carrier_name: `注销载体${ts}`, carrier_type: "众创空间", carrier_area: "南山区",
  });
  assertOk("注册载体成功", regCar.code === 0);
  const carToken = regCar.data!.token;

  const regEnt2 = await api.post("/auth/register", {
    password: "test123456", role: "enterprise", phone: `194${ts}`,
    enterprise_name: `将被删除${ts}`, enterprise_credit_code: `DEL2${ts}`,
    enterprise_industry: "IT", enterprise_scale: "小型",
  });
  assertOk("注册第二个企业", regEnt2.code === 0);
  const ent2Token = regEnt2.data!.token;

  console.log("\n=== 准备: 创建政务账号 ===");
  const govToken = await createGovernmentUser();
  govApi.setToken(govToken);

  // ====================== 权限验证 ======================
  console.log("\n=== 权限验证 ===");
  api.setToken("");
  const noAuth = await api.post("/enterprise/account/deletion", { reason: "test" });
  assertOk("无 token 被拒绝", noAuth.code !== 0);
  api.setToken(entToken);
  const noReason = await api.post("/enterprise/account/deletion", {});
  assertOk("空原因被拒绝", noReason.code !== 0);
  const crossRole = await carApi.post("/enterprise/account/deletion", { reason: "test" });
  assertOk("载体不能访问企业端点", crossRole.code !== 0);

  // ====================== 企业申请注销 → 政务通知 ======================
  console.log("\n=== 企业申请注销 → 政务通知 ===");
  const govNotifsBefore = await fetchNotifications(govToken);
  api.setToken(entToken);
  const entApply = await api.post("/enterprise/account/deletion", { reason: "业务调整，申请注销" });
  assertOk("企业提交成功", entApply.code === 0);
  const govNotifsAfter = await fetchNotifications(govToken);
  assertOk("政务收到 deletion_applied 通知", govNotifsAfter.length > govNotifsBefore.length);
  assertOk("通知类型正确", !!govNotifsAfter.find((n: any) => n.type === "deletion_applied"));

  // ====================== 政务查看申请列表 ======================
  console.log("\n=== 政务查看申请列表 ===");
  const deletionList = await govApi.get("/gov/account/deletions");
  assertOk("获取申请列表", deletionList.code === 0);
  assertOk("列表有记录", Array.isArray(deletionList.data?.list) && deletionList.data.list.length > 0);
  const reqId = deletionList.data.list[0].id;

  // ====================== SSE 实时推送验证 ======================
  console.log("\n=== SSE 实时推送验证 ===");
  // 政务保持 SSE 连接 → 企业提交注销 → 政务在同一连接上收到 event:update
  const realtimeNotif = await waitForSSEUpdate(govToken, async () => {
    api.setToken(ent2Token);
    await api.post("/enterprise/account/deletion", { reason: "实时推送测试" });
  });
  assertOk("SSE 实时推送收到通知", !!realtimeNotif);
  assertOk("通知类型为 deletion_applied", realtimeNotif?.type === "deletion_applied");

  // ====================== 政务审核 → 企业通知 ======================
  console.log("\n=== 政务审核通过 → 企业通知 ===");
  const entNotifsBefore = await fetchNotifications(entToken);
  const approve = await govApi.post(`/gov/account/deletions/${reqId}/review`, {
    action: "approve", comment: "同意注销",
  });
  assertOk("审核通过", approve.code === 0);
  const entNotifsAfter = await fetchNotifications(entToken);
  assertOk("企业收到 deletion_approved 通知", entNotifsAfter.length > entNotifsBefore.length);
  assertOk("通知类型正确", !!entNotifsAfter.find((n: any) => n.type === "deletion_approved"));

  // ====================== 政务直接删除企业 → 载体通知 ======================
  console.log("\n=== 政务直接删除企业 → 载体通知 ===");
  api.setToken(ent2Token);
  const ent2Info = await api.get("/enterprise/my-info");
  const ent2Id = ent2Info.data?.id;
  assertOk("获取企业 ID", typeof ent2Id === "number" && ent2Id > 0);
  carApi.setToken(carToken);
  const carInfo = await carApi.get("/carrier/info");
  const carrierId = carInfo.data?.id;
  api.setToken(ent2Token);
  await api.post("/enterprise/incubation", {
    carrier_id: carrierId, incubate_start: "2026-01-01", incubate_end: "2028-12-31",
  });
  const carNotifsBefore = await fetchNotifications(carToken);
  govApi.setToken(govToken);
  const delEnt = await govApi.del(`/gov/enterprises/${ent2Id}`);
  assertOk("政务直接删除企业", delEnt.code === 0);
  const carNotifsAfter = await fetchNotifications(carToken);
  assertOk("载体收到 account_deleted 通知", carNotifsAfter.length > carNotifsBefore.length);
  assertOk("通知类型正确", !!carNotifsAfter.find((n: any) => n.type === "account_deleted"));

  // ====================== 错误场景 ======================
  console.log("\n=== 错误场景 ===");
  const badMethodResp = await fetch("http://localhost:8080/api/v1/enterprise/account/deletion", {
    method: "GET", headers: { Authorization: `Bearer ${entToken}` },
  });
  assertOk("GET 请求 404", badMethodResp.status === 404);
  const badPathResp = await fetch("http://localhost:8080/api/v1/enterprise/account/deletions", {
    method: "POST",
    headers: { Authorization: `Bearer ${entToken}`, "Content-Type": "application/json" },
    body: JSON.stringify({ reason: "test" }),
  });
  assertOk("错误路径 404", badPathResp.status === 404);
  const badReview = await govApi.post("/gov/account/deletions/99999/review", { action: "approve" });
  assertOk("审核不存在申请返回错误", badReview.code !== 0);
  const badAction = await govApi.post(`/gov/account/deletions/${reqId}/review`, { action: "invalid" });
  assertOk("非法 action 被拒绝", badAction.code !== 0);
  const badDel = await govApi.del("/gov/enterprises/99999");
  assertOk("删除不存在企业返回错误", badDel.code !== 0);

  console.log("\n" + (process.exitCode ? "❌ 有测试失败" : "✅ 全部通过"));
}

main().catch(e => { console.error(e); process.exitCode = 1; });
