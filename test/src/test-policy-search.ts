import { ApiClient } from "./api.js";

const api = new ApiClient();

async function main() {
  // 1. 政务登录 → 发布政策（确保有可搜索的数据）
  console.log("=== 政务登录 ===");
  const login = await api.post("/auth/login", {
    phone: process.env.GOV_USER || "13800138000",
    password: process.env.GOV_PASS || "admin123",
    role: "government",
  });
  if (login.code !== 0) {
    console.error("政务登录失败:", login.message);
    // 继续用已存在的政策
  } else {
    api.setToken(login.data!.token);
  }

  // 2. 企业登录 → 搜索
  console.log("\n=== 企业登录 ===");
  const entLogin = await api.post("/auth/login", {
    credit_code: process.env.ENT_USER || "13800138001",
    password: process.env.ENT_PASS || "admin123",
    role: "enterprise",
  });
  if (entLogin.code !== 0) {
    console.error("企业登录失败:", entLogin.message);
    return;
  }
  api.setToken(entLogin.data!.token);
  console.log("  token:", entLogin.data!.token!.slice(0, 20) + "...");

  // 3. 搜索
  console.log("\n=== 政策搜索 ===");
  const queries = [
    "数据服务",
    "中小企业资金补贴",
  ];
  for (const q of queries) {
    console.log(`\n  --- 搜索: "${q}" ---`);
    const res = await api.post("/enterprise/policies/search", { query: q });
    if (res.code === 0) {
      const list = res.data || [];
      console.log(`  找到 ${list.length} 条政策`);
      for (const p of list) {
        console.log(`  - [${p.id}] ${p.title} (${p.status})`);
      }
    } else {
      console.log(`  搜索失败 (${res.code}): ${res.message}`);
    }
  }
}

main().catch(e => { console.error(e); process.exitCode = 1; });
