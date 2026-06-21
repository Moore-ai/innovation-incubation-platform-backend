// 文件路由集成测试
// 运行: npx tsx src/test-files.ts
// 需要先启动后端服务器
// 测试完成后会尝试清理上传的文件（政务用户需后台创建）
// 注意: 政务角色无法通过注册接口创建（仅允许 enterprise/carrier 注册）

import { ApiClient } from "./api.js";

const api = new ApiClient();
const ts = Date.now();

let fileId = 0;
let entToken = "";

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
    phone: `187${ts}`,
    enterprise_name: `文件测试${ts}`,
    enterprise_credit_code: `FILE${ts}`,
    enterprise_industry: "信息技术",
    enterprise_scale: "小型",
  });
  assertOk("注册企业成功", reg.code === 0);
  entToken = reg.data!.token;
  api.setToken(entToken);

  // ====================== 获取上传限制 ======================
  console.log("\n=== 上传限制 ===");
  const limit = await api.get("/files/limit");
  assertOk("获取上传限制", limit.code === 0);
  assertOk("返回 max_size_mb", typeof limit.data?.max_size_mb === "number");
  console.log(`  → max_size_mb = ${limit.data?.max_size_mb}`);

  // ====================== 上传文件 ======================
  // 使用纯 ASCII 内容，避免字符编码长度与字节长度的差异
  console.log("\n=== 上传文件 ===");
  const fileContent = `hello-test-file-${ts}`;
  const blob = new Blob([fileContent], { type: "text/plain" });
  const upload = await api.uploadFile("/files/upload", blob, `test-${ts}.txt`);
  assertOk("上传文件成功", upload.code === 0);
  fileId = upload.data?.file_id;
  assertOk("返回 file_id", typeof fileId === "number" && fileId > 0);
  assertOk("返回文件名", upload.data?.filename === `test-${ts}.txt`);
  assertOk("返回大小正确", Number(upload.data?.size) === fileContent.length);
  console.log(`  → file_id = ${fileId}, size = ${upload.data?.size}`);

  // ====================== 下载文件 ======================
  console.log("\n=== 下载文件 ===");
  const down = await api.getRaw(`/files/${fileId}/download`);
  assertOk("下载响应 200", down.status === 200);
  const body = await down.text();
  assertOk("文件内容正确", body === fileContent);
  const disposition = down.headers.get("content-disposition") || "";
  assertOk("包含 Content-Disposition", disposition.includes("test-"));
  console.log(`  → Content-Disposition: ${disposition}`);

  // ====================== 文件列表 ======================
  console.log("\n=== 文件列表 ===");
  const list = await api.get("/files/list");
  assertOk("获取文件列表", list.code === 0);
  assertOk("列表包含刚上传的文件", list.data?.list?.some((f: any) => f.id === fileId));

  // ====================== 权限检查 ======================
  console.log("\n=== 权限检查 ===");

  // 无 token 上传
  api.setToken("");
  const noAuth = await api.uploadFile("/files/upload", blob, "noauth.txt");
  assertOk("无 token 上传被拒绝", noAuth.code !== 0);

  // 企业不能删除文件（需 government 角色）
  api.setToken(entToken);
  const forbidden = await api.del(`/files/${fileId}`);
  assertOk("企业删除文件被拒绝", forbidden.code !== 0);

  // ====================== 清理 ======================
  // 注意: 政务角色无法通过公开注册接口创建，如需完整测试删除流程，
  //       需手动创建 government 用户并调用 DELETE /api/v1/files/:id
  //       当前测试仅验证企业无删除权限，文件记录可后续通过管理接口清理
  console.log("\n=== 清理 ===");

  // 尝试以企业身份直接删除文件（预期被拒，实际未清理）
  // 此处仅记录 file_id，供后续人工清理
  console.log(`  ⏭️  文件 #${fileId} 未被删除（仅 government 角色可删除）`);

  // ====================== 错误场景 ======================
  console.log("\n=== 错误场景 ===");
  const notFound = await api.get(`/files/99999/download`);
  assertOk("下载不存在的文件返回错误", notFound.code !== 0);

  // ====================== 汇总 ======================
  console.log("\n" + (process.exitCode ? "❌ 有测试失败" : "✅ 全部通过"));
}

main().catch(e => { console.error(e); process.exitCode = 1; });
