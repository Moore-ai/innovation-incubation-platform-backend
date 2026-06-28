// 认证模块测试 — 注册 → 登录 → 获取用户信息
// 运行: npx tsx src/test-auth.ts
// 需要先启动后端: go run ./cmd/server/

import { ApiClient } from "./api.js";

const api = new ApiClient();
const ts = Date.now();

async function assert(label: string, ok: boolean) {
  console.log(`  ${ok ? "✅" : "❌"} ${label}`);
  if (!ok) process.exitCode = 1;
}

async function main() {
  console.log("--- 注册测试 ---");

  // 1. 注册企业
  const regEnt = await api.post("/auth/register", {
    password: "test123456",
    role: "enterprise",
    phone: `138${ts}`,
    enterprise_name: "测试企业",
    enterprise_credit_code: `CRED${ts}`,
    enterprise_industry: "信息技术",
    enterprise_scale: "小型",
  });
  assert("注册企业成功", regEnt.code === 0);
  assert("返回 token", !!regEnt.data?.token);
  assert("返回 credit_code", regEnt.data?.user?.credit_code === `CRED${ts}`);

  // 2. 注册载体
  const regCar = await api.post("/auth/register", {
    password: "test123456",
    role: "carrier",
    phone: `139${ts}`,
    carrier_name: "测试载体",
    carrier_type: "众创空间",
    carrier_area: "南山区",
  });
  assert("注册载体成功", regCar.code === 0);
  assert("载体 token 存在", !!regCar.data?.token);

  console.log("\n--- 登录测试 ---");

  // 4. 企业用信用代码登录
  const loginEnt = await api.post("/auth/login", {
    credential: `CRED${ts}`,
    password: "test123456",
    role: "enterprise",
  });
  assert("企业登录成功", loginEnt.code === 0);
  assert("返回 token", !!loginEnt.data?.token);
  assert("返回 credit_code", loginEnt.data?.user?.credit_code === `CRED${ts}`);

  // 5. 错误密码
  const badPwd = await api.post("/auth/login", {
    credential: `CRED${ts}`,
    password: "wrongpassword",
    role: "enterprise",
  });
  assert("错误密码被拒绝", badPwd.code !== 0);

  // 6. 不存在的用户
  const noUser = await api.post("/auth/login", {
    credential: "NOTEXIST",
    password: "test123456",
    role: "enterprise",
  });
  assert("不存在的用户被拒绝", noUser.code !== 0);

  console.log("\n--- /me 测试 ---");

  // 7. 用企业 token 获取用户信息
  api.setToken(loginEnt.data?.token);
  const me = await api.get("/auth/me");
  assert("获取用户信息成功", me.code === 0);
  assert("角色是企业", me.data?.role === "enterprise");
  assert("包含 credit_code", !!me.data?.credit_code);

  api.setToken("");
  // 8. 无 token 访问 /me
  const anon = await api.get("/auth/me");
  assert("未认证请求被拒绝", anon.code !== 0);

  console.log("\n" + (process.exitCode ? "❌ 有测试失败" : "✅ 全部通过"));
}

main().catch(console.error);
