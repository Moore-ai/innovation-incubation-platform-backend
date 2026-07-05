// 政策发布与 AI 提取测试
// 运行: npx tsx src/test-publish-policy.ts
// 需要先启动后端服务器
// 需配置政务账号信息（通过环境变量或脚本内修改）

import { ApiClient } from "./api.js";
import * as fs from "fs";
import * as path from "path";

const api = new ApiClient();
const ts = Date.now();

async function assertOk(label: string, ok: boolean) {
  console.log(`  ${ok ? "[PASS]" : "[FAIL]"} ${label}`);
  if (!ok) process.exitCode = 1;
}

async function main() {
  // ===== 1. 政务登录 =====
  console.log("=== 政务登录 ===");
  const govUser = process.env.GOV_USER || "13800138000";
  const govPass = process.env.GOV_PASS || "admin123";
  const login = await api.post("/auth/login", {
    phone: govUser,
    password: govPass,
    role: "government",
  });
  assertOk("政务登录成功", login.code === 0);
  api.setToken(login.data!.token);
  console.log(`  -> token: ${login.data!.token!.slice(0, 20)}...`);

  // ===== 2. 上传政策文件 =====
  console.log("\n=== 上传政策文件 ===");
  const docPath = path.resolve("../sample/安徽省数据要素改革发展规划专项资金重点支持方向（2026年版）.docx");
  if (!fs.existsSync(docPath)) {
    console.error("  [FAIL] 文件不存在:", docPath);
    process.exitCode = 1;
    return;
  }
  const fileBuf = fs.readFileSync(docPath);
  const blob = new Blob([fileBuf], { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" });
  const upload = await api.uploadFile("/files/upload", blob, "安徽省数据要素改革发展规划专项资金重点支持方向（2026年版）.docx");
  assertOk("上传文件成功", upload.code === 0);
  const fileId = upload.data?.file_id;
  assertOk("返回 file_id", typeof fileId === "number" && fileId > 0);
  console.log(`  -> file_id = ${fileId}`);

  // ===== 3. 读取政策配置 =====
  console.log("\n=== 读取政策配置 ===");
  const policyStr = fs.readFileSync(path.resolve("../sample/policies.json"), "utf-8");
  const policyData = JSON.parse(policyStr);
  const rawPolicy = policyData.policies[0];
  console.log(`  -> 政策标题: ${rawPolicy.title}`);

  // 将文件 ID 填入 legal_basis
  rawPolicy.requirements.legal_basis[0].file_id = fileId;
  // material_template 字段不能为空字符串，改为 null 以支持 omitempty
  if (rawPolicy.requirements.application_materials?.[0]?.material_template === "") {
    rawPolicy.requirements.application_materials[0].material_template = null;
  }

  // ===== 4. 发布政策 =====
  console.log("\n=== 发布政策 ===");
  const publishBody = {
    target_role: rawPolicy.target_role,
    title: `${rawPolicy.title}（测试${ts}）`,
    requirements: rawPolicy.requirements,
    start_date: rawPolicy.start_date,
    end_date: rawPolicy.end_date,
  };
  console.log("  请求体:", JSON.stringify(publishBody, null, 2).slice(0, 500) + "...");

  const pub = await api.post("/gov/policies", publishBody);
  if (pub.code === 0) {
    assertOk("发布政策成功", true);

    // 查询文件详情（含摘要）
    console.log("\n  ===== 法律依据文件详情 =====");
    const fileInfo = await api.get("/files?page=1&per_page=5");
    if (fileInfo.code === 0 && fileInfo.data?.list) {
      const matched = fileInfo.data.list.find((f: any) => f.id === fileId);
      if (matched) {
        console.log("  文件名:", matched.filename);
        console.log("  文件摘要:", matched.summary || "(摘要为空)");
      }
    }

    console.log("\n  ===== AI 提取结果 (ExtractedFields) =====");
    if (pub.data?.extracted_fields?.policy_summary) {
      console.log("\n  --- 政策摘要 ---");
      console.log("  " + pub.data.extracted_fields.policy_summary);
      console.log("");
    }
    console.log(JSON.stringify(pub.data?.extracted_fields, null, 2));
  } else {
    console.log(`  [FAIL] 发布失败 (code=${pub.code}): ${pub.message}`);
    console.log("  [WARN] AI 提取未执行 - 请检查 AI 配置 (AI_BASE_URL/AI_API_KEY)");
  }

  // ===== 5. 汇总 =====
  console.log("\n" + (process.exitCode ? "[FAIL] 有测试失败" : "[PASS] 全部通过"));
}

main().catch(e => { console.error(e); process.exitCode = 1; });
