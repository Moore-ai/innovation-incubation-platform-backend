// 注册工具 — 通过启动参数注册企业/载体/政务账号
// 用法:
//   npx tsx src/register.ts --role enterprise --phone 13912345678 --pass test123 --name 某公司 --credit 91340100MA...
//   npx tsx src/register.ts --role carrier   --phone 13912345678 --pass test123 --name 某载体 --type 孵化器 --area 合肥
//   npx tsx src/register.ts --role government --phone 13912345678 --pass test123

import { ApiClient } from "./api.js";

const api = new ApiClient();

function usage() {
  console.log(`用法:
  npx tsx src/register.ts <参数>

必填:
  --role <enterprise|carrier|government>  角色
  --phone <手机号>                          登录手机号

选填:
  --pass <密码>                             默认 admin123

企业额外信息:
  --name <企业名称>                         默认 "测试企业"
  --credit <统一社会信用代码>                默认自动生成
  --industry <行业>                         默认 "信息传输、软件和信息技术服务业"
  --scale <规模>                            默认 "小型"
  --address <地址>                          默认 "安徽省合肥市"

载体额外信息:
  --name <载体名称>                         默认 "测试载体"
  --type <类型>                             默认 "孵化器"
  --area <区域>                             默认 "安徽省合肥市"
  --specialty <专业方向,逗号分隔>             默认 "信息技术"

政务额外信息:
  --name <姓名>                             默认 "政务管理员"
  --department <部门>                        默认 "综合管理科"
`);
}

function parseArgs() {
  const args = process.argv.slice(2);
  const map: Record<string, string> = {};
  for (let i = 0; i < args.length; i++) {
    if (args[i].startsWith("--")) {
      const key = args[i].slice(2);
      map[key] = args[i + 1] && !args[i + 1].startsWith("--") ? args[i + 1] : "true";
      if (map[key] !== "true") i++;
    }
  }
  return map;
}

async function main() {
  const args = parseArgs();

  if (!args.role || !args.phone) {
    usage();
    process.exit(1);
  }

  const role = args.role;
  const phone = args.phone;
  const password = args.pass || "admin123";

  const body: Record<string, string> = {
    phone,
    password,
    role,
  };

  let creditCode = "";
  if (role === "enterprise") {
    creditCode = args.credit || `CREDIT${Date.now()}`;
    body.enterprise_name = args.name || "测试企业";
    body.enterprise_credit_code = creditCode;
    body.enterprise_industry = args.industry || "信息传输、软件和信息技术服务业";
    body.enterprise_scale = args.scale || "小型";
    body.enterprise_address = args.address || "安徽省合肥市";
  } else if (role === "carrier") {
    body.carrier_name = args.name || "测试载体";
    body.carrier_type = args.type || "孵化器";
    body.carrier_area = args.area || "安徽省合肥市";
    body.carrier_phone = phone;
    body.carrier_specialty_fields = args.specialty || "信息技术";
  } else if (role === "government") {
    body.gov_name = args.name || "政务管理员";
    body.gov_department = args.department || "综合管理科";
  } else {
    console.error(`未知角色: ${role}，支持 enterprise / carrier / government`);
    process.exit(1);
  }

  console.log(`注册 ${role} 账号...`);
  const res = await api.post("/auth/register", body);
  if (res.code === 0) {
    console.log(`成功: ${res.message}`);
    console.log(`  用户ID: ${res.data?.user_id || "?"}`);
    if (creditCode) {
      console.log(`  信用代码: ${creditCode}`);
      console.log(`  （企业登录时使用信用代码作为凭证）`);
    }
    console.log(`  Token: ${(res.data?.token || "").slice(0, 20)}...`);
  } else {
    console.error(`失败 (${res.code}): ${res.message}`);
    process.exit(1);
  }
}

main().catch((e) => { console.error(e); process.exitCode = 1; });
