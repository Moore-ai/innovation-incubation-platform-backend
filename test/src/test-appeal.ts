import { ApiClient } from "./api.js";

const api = new ApiClient();

const ENT_PHONE = "13800000901";
const CARRIER_PHONE = "13900000901";
const PASSWORD = "test123456";

async function main() {
  // ============================================================
  // 1. 注册企业用户
  // ============================================================
  console.log("=== 注册企业用户 ===");
  const entReg = await api.post("/auth/register", {
    phone: ENT_PHONE,
    password: PASSWORD,
    role: "enterprise",
    enterprise_name: "诉求测试企业",
    enterprise_credit_code: "91340000TEST0001",
    enterprise_industry: "信息技术",
    enterprise_scale: "中型",
  });
  console.log("注册企业:", entReg.code === 0 ? "成功" : `失败 (${entReg.message})`);

  // ============================================================
  // 2. 注册载体用户
  // ============================================================
  console.log("\n=== 注册载体用户 ===");
  const carrierReg = await api.post("/auth/register", {
    phone: CARRIER_PHONE,
    password: PASSWORD,
    role: "carrier",
    carrier_name: "诉求测试载体",
    carrier_type: "孵化器",
    carrier_area: "合肥市",
  });
  console.log("注册载体:", carrierReg.code === 0 ? "成功" : `失败 (${carrierReg.message})`);

  // ============================================================
  // 3. 企业登录
  // ============================================================
  console.log("\n=== 企业登录 ===");
  const entLogin = await api.post("/auth/login", {
    phone: "91340000TEST0001",
    password: PASSWORD,
    role: "enterprise",
  });
  if (entLogin.code !== 0) {
    console.error("企业登录失败:", entLogin.message);
    return;
  }
  api.setToken(entLogin.data!.token);
  console.log("企业登录成功");

  // ============================================================
  // 4. 企业提交诉求
  // ============================================================
  console.log("\n=== 企业提交诉求 ===");
  const entSubmit = await api.post("/enterprise/appeals", {
    identifier: "91340000TEST0001",
    problem_type: "tax",
    department: "税务局",
    content: "企业所得税优惠政策不明确，建议出台实施细则",
  });
  console.log("企业提交诉求:", JSON.stringify(entSubmit, null, 2));
  if (entSubmit.code !== 0) {
    console.error("企业提交诉求失败");
    return;
  }
  const appealId1 = entSubmit.data!.id;

  // ============================================================
  // 5. 企业查看自己的诉求列表
  // ============================================================
  console.log("\n=== 企业查看诉求列表 ===");
  const entList = await api.get("/enterprise/appeals");
  console.log("企业诉求列表:", JSON.stringify(entList, null, 2));

  // ============================================================
  // 6. 载体登录
  // ============================================================
  console.log("\n=== 载体登录 ===");
  api.clearToken();
  const carrierLogin = await api.post("/auth/login", {
    phone: CARRIER_PHONE,
    password: PASSWORD,
    role: "carrier",
  });
  if (carrierLogin.code !== 0) {
    console.error("载体登录失败:", carrierLogin.message);
    return;
  }
  api.setToken(carrierLogin.data!.token);
  console.log("载体登录成功");

  // ============================================================
  // 7. 载体提交诉求
  // ============================================================
  console.log("\n=== 载体提交诉求 ===");
  const carrierSubmit = await api.post("/carrier/appeals", {
    identifier: CARRIER_PHONE,
    problem_type: "financing",
    department: "财政局",
    content: "中小企业融资渠道单一，建议出台专项融资政策",
  });
  console.log("载体提交诉求:", JSON.stringify(carrierSubmit, null, 2));
  if (carrierSubmit.code !== 0) {
    console.error("载体提交诉求失败");
    return;
  }

  // ============================================================
  // 8. 政务登录
  // ============================================================
  console.log("\n=== 政务登录 ===");
  api.clearToken();
  const govLogin = await api.post("/auth/login", {
    phone: "13800138000",
    password: "admin123",
    role: "government",
  });
  if (govLogin.code !== 0) {
    console.error("政务登录失败:", govLogin.message);
    return;
  }
  api.setToken(govLogin.data!.token);
  console.log("政务登录成功");

  // ============================================================
  // 9. 政务查看所有诉求
  // ============================================================
  console.log("\n=== 政务查看所有诉求 ===");
  const govList = await api.get("/gov/appeals");
  console.log("政务诉求列表:", JSON.stringify(govList, null, 2));

  // ============================================================
  // 10. 政务按状态筛选
  // ============================================================
  console.log("\n=== 政务按状态筛选 (pending) ===");
  const pendingList = await api.get("/gov/appeals", { status: "pending" });
  console.log("筛选结果:", JSON.stringify(pendingList, null, 2));

  // ============================================================
  // 11. 政务按问题类型筛选
  // ============================================================
  console.log("\n=== 政务按问题类型筛选 (tax) ===");
  const taxList = await api.get("/gov/appeals", { problem_type: "tax" });
  console.log("筛选结果:", JSON.stringify(taxList, null, 2));

  // ============================================================
  // 12. 政务标记诉求为已处理
  // ============================================================
  console.log("\n=== 政务标记诉求为已处理 ===");
  const updateRes = await api.patch(`/gov/appeals/${appealId1}/status`, {
    status: "processed",
  });
  console.log("更新结果:", JSON.stringify(updateRes, null, 2));

  // ============================================================
  // 13. 验证状态已更新
  // ============================================================
  console.log("\n=== 验证状态更新 ===");
  const processedList = await api.get("/gov/appeals", { status: "processed" });
  console.log("已处理诉求:", JSON.stringify(processedList, null, 2));

  // ============================================================
  // 14. 测试参数校验
  // ============================================================
  console.log("\n=== 测试参数校验 ===");
  const invalidRes = await api.post("/enterprise/appeals", {
    identifier: "",
    problem_type: "invalid_type",
    content: "测试",
  });
  console.log("无效参数测试:", JSON.stringify(invalidRes, null, 2));

  console.log("\n=== 测试完成 ===");
}

main().catch(console.error);
