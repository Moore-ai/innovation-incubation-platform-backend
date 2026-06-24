// 企业端路由集成测试
// 运行: npx tsx src/test-enterprise.ts
// 需要先启动后端服务器

import { ApiClient } from "./api.js";

const api = new ApiClient();
const carApi = new ApiClient();
const ts = Date.now();

let enterpriseToken = "";
let carrierToken = "";
let carrierId = 0;
let incubationId = 0;
let changeId = 0;
let changeType = "";

async function assertOk(label: string, ok: boolean) {
  console.log(`  ${ok ? "✅" : "❌"} ${label}`);
  if (!ok) process.exitCode = 1;
}

async function main() {
  // ====================== 准备数据 ======================
  console.log("=== 准备: 注册企业用户 ===");
  const regEnt = await api.post("/auth/register", {
    password: "test123456",
    role: "enterprise",
    phone: `138${ts}`,
    enterprise_name: `测试企业${ts}`,
    enterprise_credit_code: `CRED${ts}`,
    enterprise_industry: "信息技术",
    enterprise_scale: "小型",
  });
  assertOk("注册企业成功", regEnt.code === 0);
  enterpriseToken = regEnt.data!.token;

  console.log("\n=== 准备: 注册载体用户 ===");
  const regCar = await carApi.post("/auth/register", {
    password: "test123456",
    role: "carrier",
    phone: `139${ts}`,
    carrier_name: `测试载体${ts}`,
    carrier_type: "众创空间",
    carrier_area: "南山区",
  });
  assertOk("注册载体成功", regCar.code === 0);
  carrierToken = regCar.data!.token;

  // 切换载体 token 获取载体 ID
  carApi.setToken(carrierToken);
  const carInfo = await carApi.get("/carrier/info");
  assertOk("获取载体信息成功", carInfo.code === 0);
  carrierId = carInfo.data?.id;
  assertOk("载体 ID 存在", typeof carrierId === "number" && carrierId > 0);
  console.log(`  → carrier_id = ${carrierId}`);

  // 切换回企业 token
  api.setToken(enterpriseToken);

  // ====================== 企业信息 ======================
  console.log("\n=== 企业信息 ===");
  const info = await api.get("/enterprise/my-info");
  assertOk("获取企业信息成功", info.code === 0);
  assertOk("企业名称正确", info.data?.name === `测试企业${ts}`);
  assertOk("信用代码正确", info.data?.credit_code === `CRED${ts}`);

  // ====================== 变更类型 ======================
  console.log("\n=== 变更类型 ===");
  const changeTypes = await api.get("/enterprise/changes/types");
  assertOk("获取变更类型列表", changeTypes.code === 0);
  const types = changeTypes.data as string[];
  assertOk("变更类型是数组", Array.isArray(types) && types.length > 0);
  changeType = types[0];
  console.log(`  → 可用变更类型: ${types.join(", ")}`);

  // ====================== 入驻申请 ======================
  console.log("\n=== 入驻申请 ===");
  const incubate = await api.post("/enterprise/incubation", {
    carrier_id: carrierId,
    incubate_start: "2026-01-01",
    incubate_end: "2028-12-31",
  });
  assertOk("提交入驻申请", incubate.code === 0);
  incubationId = incubate.data?.id;
  assertOk("返回入驻记录 ID", typeof incubationId === "number" && incubationId > 0);
  console.log(`  → incubation_id = ${incubationId}`);

  // ====================== 入驻列表 ======================
  console.log("\n=== 入驻列表 ===");
  const incubateList = await api.get("/enterprise/incubation/list");
  assertOk("获取入驻列表", incubateList.code === 0);
  assertOk("列表包含新建记录", Array.isArray(incubateList.data?.list) && incubateList.data.list.length > 0);

  // ====================== 入驻详情 ======================
  console.log("\n=== 入驻详情 ===");
  const incubateDetail = await api.get(`/enterprise/incubation/${incubationId}`);
  assertOk("获取入驻详情", incubateDetail.code === 0);
  assertOk("ID 匹配", incubateDetail.data?.id === incubationId);
  assertOk("载体 ID 匹配", incubateDetail.data?.carrier_id === carrierId);

  // ====================== 变更申请 ======================
  console.log("\n=== 变更申请 ===");
  const change = await api.post("/enterprise/changes", {
    change_type: changeType,
    change_content: "企业信息变更测试",
    new_value: { name: "新企业名称" },
  });
  assertOk("提交变更申请", change.code === 0);
  changeId = change.data?.id;
  assertOk("返回变更记录 ID", typeof changeId === "number" && changeId > 0);
  console.log(`  → change_id = ${changeId}`);

  // ====================== 变更列表 ======================
  console.log("\n=== 变更列表 ===");
  const changeList = await api.get("/enterprise/changes/list");
  assertOk("获取变更列表", changeList.code === 0);
  assertOk("列表包含新建记录", Array.isArray(changeList.data?.list) && changeList.data.list.length > 0);

  // ====================== 变更详情 ======================
  console.log("\n=== 变更详情 ===");
  const changeDetail = await api.get(`/enterprise/changes/${changeId}`);
  assertOk("获取变更详情", changeDetail.code === 0);
  assertOk("ID 匹配", changeDetail.data?.id === changeId);

  // ====================== 重新编辑变更 ======================
  // 注意: 仅当变更为 returned 状态时才能重新编辑，新建的变更为 pending 状态
  console.log("\n=== 重新编辑变更 ===");
  const reedit = await api.put(`/enterprise/changes/${changeId}`, {
    change_type: changeType,
    change_content: "重新编辑后的变更内容",
    new_value: { name: "新企业名称 V2" },
  });
  assertOk("pending 变更不可编辑（需先退回）", reedit.code !== 0);

  // ====================== 政策列表 ======================
  console.log("\n=== 政策列表 ===");
  const policies = await api.get("/enterprise/policies");
  assertOk("获取政策列表", policies.code === 0);
  console.log(`  → 可用政策数: ${policies.data?.total ?? 0}`);

  // ====================== 政策申报 (仅当有政策时) ======================
  console.log("\n=== 政策申报 ===");
  const firstPolicy = policies.data?.list?.[0];
  if (firstPolicy) {
    const apply = await api.post(`/enterprise/policies/${firstPolicy.id}/apply`, {
      form_data: { company_name: "测试企业" },
    });
    assertOk("提交政策申报", apply.code === 0);
    console.log(`  → application_id = ${apply.data?.id}`);
  } else {
    console.log("  ⏭️  无可用政策，跳过申报测试");
  }

  // ====================== 申报列表 ======================
  console.log("\n=== 申报列表 ===");
  const apps = await api.get("/enterprise/applications/list");
  assertOk("获取申报列表", apps.code === 0);
  assertOk("列表是数组", Array.isArray(apps.data?.list));

  // ====================== AI 接口 ======================
  console.log("\n=== AI 接口 ===");
  if (firstPolicy) {
    const recommend = await api.get(`/enterprise/policies/${firstPolicy.id}/recommend`);
    // AI 接口可能因 API Key 等问题失败，但不应返回 500
    assertOk("政策匹配推荐已响应", recommend.code !== 500);
    console.log(`  → 推荐结果 code=${recommend.code}`);
  } else {
    console.log("  ⏭️  无可用政策，跳过 AI 推荐测试");
  }

  const prefill = await api.post("/enterprise/policies/prefill", {
    policy_id: firstPolicy?.id ?? 0,
    material_template_id: 0,
  });
  // 即使找不到政策，也应返回业务错误而非 500
  assertOk("AI 预填已响应", prefill.code !== 500);

  // ====================== 错误场景 ======================
  console.log("\n=== 错误场景 ===");
  const badInc = await api.post("/enterprise/incubation", {
    carrier_id: 99999,
    incubate_start: "bad-date",
  });
  assertOk("无效 carrier_id 返回错误", badInc.code !== 0);

  const badChange = await api.post("/enterprise/changes", {});
  assertOk("空变更请求返回错误", badChange.code !== 0);

  const notFound = await api.get("/enterprise/incubation/99999");
  assertOk("不存在的入驻返回错误", notFound.code !== 0);

  const badPrefill = await api.post("/enterprise/policies/prefill", {});
  assertOk("空预填请求返回错误", badPrefill.code !== 0);

  // ====================== 权限验证 ======================
  console.log("\n=== 权限验证 ===");
  api.setToken("");
  const noAuth = await api.get("/enterprise/my-info");
  assertOk("无 token 被拒绝", noAuth.code !== 0);

  api.setToken(carrierToken);
  const wrongRole = await api.get("/enterprise/my-info");
  assertOk("载体不能访问企业接口", wrongRole.code !== 0);

  // ====================== 汇总 ======================
  console.log("\n" + (process.exitCode ? "❌ 有测试失败" : "✅ 全部通过"));
}

main().catch(console.error);
