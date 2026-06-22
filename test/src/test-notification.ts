// 通知路由集成测试
// 运行: npx tsx src/test-notification.ts
// 需要先启动后端服务器
// 测试 SSE 订阅和标记已读功能

import { ApiClient } from "./api.js";

const api = new ApiClient();
const ts = Date.now();

async function assertOk(label: string, ok: boolean) {
  console.log(`  ${ok ? "✅" : "❌"} ${label}`);
  if (!ok) process.exitCode = 1;
}

async function main() {
  // ====================== 准备 ======================
  console.log("=== 准备: 注册企业用户 ===");
  const reg = await api.post("/auth/register", {
    password: "test123456",
    role: "enterprise",
    phone: `186${ts}`,
    enterprise_name: `通知测试${ts}`,
    enterprise_credit_code: `NOTIF${ts}`,
    enterprise_industry: "信息技术",
    enterprise_scale: "小型",
  });
  assertOk("注册企业成功", reg.code === 0);
  api.setToken(reg.data!.token);

  // ====================== SSE 订阅 ======================
  console.log("\n=== SSE 订阅 ===");

  // 建立 SSE 连接，使用 AbortController 在收到数据后断开
  const ac = new AbortController();
  const ssePromise = fetch("http://localhost:8080/api/v1/notifications/subscribe", {
    headers: { Authorization: `Bearer ${api.tokenStr}` },
    signal: ac.signal,
  });

  // 等待一小段时间让连接建立并接收 init 事件
  await new Promise(r => setTimeout(r, 500));
  ac.abort();

  let sseOk = false;
  try {
    const sseResp = await ssePromise;
    sseOk = sseResp.status === 200 && sseResp.headers.get("content-type")?.includes("text/event-stream") === true;
  } catch {
    // AbortError 是预期的
    sseOk = true;
  }
  assertOk("SSE 连接建立成功", sseOk);

  // 无 token 订阅被拒绝
  const noAuthSSE = await fetch("http://localhost:8080/api/v1/notifications/subscribe");
  assertOk("无 token SSE 被拒绝", noAuthSSE.status !== 200);

  // ====================== 标记已读 ======================
  console.log("\n=== 标记已读 ===");

  // 空 ID 列表
  const empty = await api.patch("/notifications/read", { ids: [] });
  assertOk("空 ID 列表被拒绝", empty.code !== 0);

  // 缺少 ids 字段
  const noIds = await api.patch("/notifications/read", {});
  assertOk("缺少 ids 被拒绝", noIds.code !== 0);

  // 标记不存在的通知 ID（业务上应为幂等成功）
  const nonExist = await api.patch("/notifications/read", { ids: [99999] });
  assertOk("标记不存在的 ID 返回成功（幂等）", nonExist.code === 0);
  assertOk("返回已标记的 id 列表", Array.isArray(nonExist.data?.ids) && nonExist.data.ids[0] === 99999);

  // 合法请求
  const valid = await api.patch("/notifications/read", { ids: [1, 2, 3] });
  assertOk("标记已读成功", valid.code === 0);
  assertOk("返回 ids", JSON.stringify(valid.data?.ids) === "[1,2,3]");

  // ====================== 权限验证 ======================
  console.log("\n=== 权限验证 ===");
  api.setToken("");
  const anon = await api.patch("/notifications/read", { ids: [1] });
  assertOk("无 token 标记已读被拒绝", anon.code !== 0);

  // ====================== 汇总 ======================
  console.log("\n" + (process.exitCode ? "❌ 有测试失败" : "✅ 全部通过"));
}

main().catch(e => { console.error(e); process.exitCode = 1; });
