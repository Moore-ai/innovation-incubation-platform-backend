import { ApiClient } from "./api.js";

const api = new ApiClient();

function printPolicy(p: any, index: number) {
  const ef = p.extracted_fields || {};
  const summary = ef.policy_summary ? ef.policy_summary : "";
  const industries = (ef.applicable_industries || []).join(",");
  const scales = (ef.applicable_scales || []).join(",");
  const subsidies = (ef.subsidies || []).map((s: any) => s.amount).filter(Boolean).join("; ");
  console.log(`\n${index}. ${p.title}`);
  if (summary) console.log(`   摘要: ${summary}`);
  if (industries) console.log(`   行业: ${industries}`);
  if (scales) console.log(`   规模: ${scales}`);
  if (subsidies) console.log(`   补贴: ${subsidies}`);
}

async function search(query: string) {
  console.log(`\n搜索: "${query}"`);
  const res = await api.post("/enterprise/policies/search", { query });
  if (res.code !== 0) {
    console.log(`失败 (${res.code}): ${res.message}`);
    return;
  }
  const result = res.data || {};
  const list = result.policies || [];
  console.log(`结果: ${list.length} 条`);
  if (result.analysis) {
    console.log(`\nAI 分析:\n${result.analysis}\n`);
  }
  for (let i = 0; i < list.length; i++) {
    printPolicy(list[i], i + 1);
  }
  console.log();
}

const batchQueries = [
  "数字化转型补贴",
  "智能制造补贴",
  "绿色工厂补贴",
  "专精特新奖补",
  "人工智能补贴",
  "跨境电商补贴",
  "新能源汽车补贴",
  "中小企业资金支持",
  "制造业技术改造补贴",
  "外贸企业支持政策",
  "农业科技项目",
  "服务业集聚区",
  "有没有针对小微企业的减税政策",
  "企业上市有什么奖励",
  "人才引进有什么补贴政策",
  "研发费用补助",
  "企业创新平台支持",
  "国家绿色工厂奖补",
  "支持省级先进制造业集群培育建设",
  "技术改造项目贷款贴息",
];

async function main() {
  const args = process.argv.slice(2);
  const queryIdx = args.indexOf("--query");
  const singleQuery = queryIdx >= 0 ? args[queryIdx + 1] : null;
  const ccIdx = args.indexOf("--credit-code");
  const creditCode = ccIdx >= 0 ? args[ccIdx + 1] : (process.env.ENT_CREDIT_CODE || "");
  const passIdx = args.indexOf("--pass");
  const password = passIdx >= 0 ? args[passIdx + 1] : (process.env.ENT_PASS || "admin123");

  console.log("=== 企业登录 ===");
  const login = await api.post("/auth/login", {
    credit_code: creditCode,
    password,
    role: "enterprise",
  });
  if (login.code !== 0) {
    console.error("登录失败:", login.message);
    return;
  }
  api.setToken(login.data!.token!);
  console.log("登录成功\n");

  if (singleQuery) {
    await search(singleQuery);
  } else {
    console.log(`=== 政策搜索测试（${batchQueries.length} 个查询）===\n`);
    for (let i = 0; i < batchQueries.length; i++) {
      console.log(`[${i + 1}/${batchQueries.length}]`);
      await search(batchQueries[i]);
    }
    console.log("=== 测试完成 ===");
  }
}

main().catch((e) => { console.error(e); process.exitCode = 1; });
