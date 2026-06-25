// 政策搜索交互测试
// 运行: npx tsx src/search-interactive.ts
// 需要先启动后端服务器

import { ApiClient } from "./api.js";
import * as readline from "readline";

const api = new ApiClient();
const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

function ask(query: string): Promise<string> {
  return new Promise((resolve) => rl.question(query, resolve));
}

async function main() {
  console.log("=== 政策搜索交互测试 ===\n");

  // 1. 输入企业基本信息
  console.log("请填写您的企业基本信息（直接回车使用默认值）：");
  const name = (await ask("  企业名称 [数据服务有限公司]: ")) || "数据服务有限公司";
  const industry = (await ask("  所属行业 [信息传输、软件和信息技术服务业]: ")) || "信息传输、软件和信息技术服务业";
  const scale = (await ask("  企业规模（微型/小型/中型/大型）[小型]: ")) || "小型";
  const address = (await ask("  所在区域 [安徽省合肥市]: ")) || "安徽省合肥市";

  // 2. 注册企业账号
  const phone = `139${String(Date.now()).slice(0, 8)}`;
  const creditCode = `CREDIT${Date.now()}`;
  console.log(`\n正在注册企业账号...（手机号: ${phone}）`);
  const reg = await api.post("/auth/register", {
    phone, password: "test123", role: "enterprise",
    enterprise_name: name, enterprise_credit_code: creditCode,
    enterprise_industry: industry, enterprise_scale: scale,
    enterprise_address: address,
  });
  if (reg.code !== 0) {
    console.error("注册失败:", reg.message);
    return;
  }
  api.setToken(reg.data!.token!);
  console.log("注册成功！\n");

  // 3. 搜索循环
  console.log("可以开始搜索了，输入 exit 退出。\n");
  while (true) {
    const query = await ask("请输入您的需求: ");
    if (query.toLowerCase() === "exit") break;
    if (!query.trim()) continue;

    console.log("\n正在搜索...");
    const res = await api.post("/enterprise/policies/search", { query });
    if (res.code === 0) {
      const result = res.data || {};
      const list = result.policies || [];
      const analysis = result.analysis || "";
      if (analysis) {
        console.log(`\n🔍 AI 分析：\n${analysis}\n`);
      }
      if (list.length === 0) {
        console.log("未找到匹配的政策。\n");
        continue;
      }
      console.log(`\n找到 ${list.length} 条匹配政策：`);
      console.log("=".repeat(60));
      for (let i = 0; i < list.length; i++) {
        const p = list[i];
        const ef = p.extracted_fields || {};
        console.log(`\n${i + 1}. ${p.title}`);
        console.log(`   状态: ${p.status === 'published' ? '进行中' : p.status}`);
        console.log(`   有效期: ${p.start_date} ~ ${p.end_date}`);
        if (ef.policy_summary) console.log(`   摘要: ${ef.policy_summary}`);
        if (ef.subsidy_type) console.log(`   支持方式: ${ef.subsidy_type}`);
        if (ef.subsidy_amount) console.log(`   金额: ${ef.subsidy_amount}`);
        console.log(`   适用行业: ${(ef.applicable_industries || []).join("、")}`);
        console.log(`   适用规模: ${(ef.applicable_scales || []).join("、")}`);
        if (ef.applicable_status) console.log(`   适用状态: ${ef.applicable_status}`);
      }
      console.log("=".repeat(60) + "\n");
    } else {
      console.log(`搜索失败: ${res.message}\n`);
    }
  }

  rl.close();
  console.log("\n再见！");
}

main().catch((e) => { console.error(e); process.exitCode = 1; });
