# 创新创业孵化载体管理平台

面向创新创业孵化场景的数字化管理平台，服务于企业、载体和政务三方角色，提供从入驻到毕业的全流程管理与智能辅助。

## 角色

| 角色 | 核心功能 |
|------|----------|
| **企业** | 入驻申请、政策申报与匹配、重大事项变更、诉求直达、AI 对话助手 |
| **载体**（孵化器/产业园） | 入驻审核、政策申报审核、绩效申报、AI 对话助手 |
| **政务**（科技局等管理部门） | 政策发布与管理、综合查询、绩效评估、数据分析报告 |

## 核心能力

### 三方协同

覆盖企业从入驻、孵化、变更到毕业的完整生命周期。入驻申请需经载体审核→政务备案，政策申报需经载体初审→政务终审，全程留痕。

### AI 智能助手

每个角色配备 AI 对话助手，支持自然语言查询政策、入驻状态、诉求进度等。助手内置大量业务工具，可自动选择工具完成多步操作，支持实时 SSE 流式回复。

### 数据驱动的政策管理

政策发布时自动 AI 提取结构化字段并生成向量索引。支持双模检索（结构化匹配 + 向量语义搜索），根据配置灵活切换。

### 政务数据分析

政务角色可通过 AI 助手一键生成数据分析报告。报告以 PDF 或 DOCX 格式输出，内含表格与 Mermaid 图表，支持文件下载。

### 全流程通知

多种业务场景自动触发站内通知，通过 SSE 实时推送。可操作通知（入驻审核、绩效评分等）通过轮询分配器分配给政务人员。

## 系统架构

单体后端 + 微服务 sidecar 的组合架构：

- **Go 后端**：提供 RESTful API 和 SSE 流式接口
- **Python 微服务**：文件解析（Markitdown）、报告格式转换（md2pdf-mermaid）
- **PostgreSQL**：主数据库，含 pgvector 向量扩展
- **Redis**：令牌桶限流
- **JWT + Casbin RBAC**：认证与权限控制

## 环境准备

### 基础依赖

- Go 1.26+
- PostgreSQL（推荐 15+）+ pgvector 扩展
- Redis 7+
- Python 3.11+

### 环境变量

```bash
cp .env.example .env
# 编辑 .env，填入实际值
```

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_HOST` | PostgreSQL 主机 | `127.0.0.1` |
| `DB_PORT` | PostgreSQL 端口 | `5432` |
| `DB_USER` | 数据库用户名 | `postgres` |
| `DB_PASSWORD` | 数据库密码 | — |
| `DB_NAME` | 数据库名 | `incubation_platform` |
| `JWT_SECRET` | JWT 签名密钥 | — |
| `AI_API_KEY` | AI 大模型 API Key | — |
| `AI_BASE_URL` | AI API 地址 | `https://api.deepseek.com/v1` |
| `AI_MODEL` | AI 模型名称 | `deepseek-chat` |
| `EMBEDDING_API_KEY` | Embedding API Key | — |
| `EMBEDDING_BASE_URL` | Embedding API 地址 | — |
| `EMBEDDING_MODEL` | Embedding 模型名称 | `text-embedding-v3` |
| `SERVER_PORT` | 服务端口 | `8080` |
| `SERVER_MODE` | 运行模式 | `debug` |
| `REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `REDIS_DB` | Redis 数据库编号 | `0` |

### 虚拟环境

项目包含两个 Python 微服务，各自使用独立的虚拟环境：

```bash
# 文件解析微服务
python -m venv sidecar/file-parser/venv
sidecar/file-parser/venv/bin/pip install -r sidecar/file-parser/requirements.txt     # Linux
sidecar/file-parser/venv/Scripts/pip install -r sidecar/file-parser/requirements.txt # Windows

# 报告格式转换微服务（PDF/DOCX）
python -m venv sidecar/report-converter/venv
sidecar/report-converter/venv/bin/pip install -r sidecar/report-converter/requirements.txt     # Linux
sidecar/report-converter/venv/Scripts/pip install -r sidecar/report-converter/requirements.txt # Windows
```

### 启动

```bash
go run ./cmd/server/
```

后端启动时会自动拉起 Python 微服务（文件解析）和连接 Redis/PostgreSQL。报告转换服务需手动启动：

```bash
sidecar/report-converter/venv/bin/python sidecar/report-converter/server.py &     # Linux
sidecar/report-converter/venv/Scripts/python sidecar/report-converter/server.py & # Windows
```

### 数据集导入

```bash
go run ./cmd/seed/ --from=0 --limit=10  # 分批导入政策数据
```

### 测试

```bash
# 端到端测试（需 Node.js，位于 test/ 目录）
cd test && npx tsx src/register.ts --role enterprise --phone 13900000001 --name 某公司
cd test && npx tsx src/test-search-dataset.ts
```
