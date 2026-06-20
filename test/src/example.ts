// 示例：注册企业 → 登录 → 查看政策
// 运行: npx tsx src/example.ts

import { ApiClient } from "./api.js";

async function main() {
  const api = new ApiClient();

  // 1. 注册企业
  const reg = await api.post("/auth/register", {
    password: "test123456",
    role: "enterprise",
    phone: "13800138000",
    enterprise_name: "测试企业",
    enterprise_credit_code: "TEST" + Date.now(),
    enterprise_industry: "信息技术",
    enterprise_scale: "小型",
  });
  console.log("注册:", reg.message, reg.data?.user?.credit_code);

  // 2. 登录
  const login = await api.post("/auth/login", {
    credential: "TEST" + Date.now(), // 上面注册时用的 credit_code
    password: "test123456",
    role: "enterprise",
  });
  api.setToken(login.data?.token);
  console.log("登录:", login.data?.token ? "成功" : "失败");

  // 3. 查看我的信息
  const me = await api.get("/auth/me");
  console.log("我的信息:", me.data);

  // 4. 查看可申报政策
  const policies = await api.get("/enterprise/policies");
  console.log("政策列表:", policies.data?.list?.length ?? 0, "条");
}

main().catch(console.error);
