# Security scan: DeepSec

AI 安全审计工具（Shield 代码审计 + Spear 渗透测试）。本项目只用 **Shield** 给 AI 生成代码与安全敏感改动做安全体检。Spear 属授权渗透测试，未经书面授权勿用。

## 安装

- 工具：`deepsec`（GitHub Release 的 whl，**PyPI 未同步**，勿 `pip install deepsec`）
- 安装：下载 `deepsec-0.2.0-py3-none-any.whl` 后 `python -m pip install <whl>`
- 要求：Python 3.10+；L1/L2 完全本地离线，**无需 API Key、不上传代码**

## 调用

```bash
python -m deepsec shield scan <path> [--layer all|l1|l2] [--format text|json|sarif|markdown|html] [-o report]
```

## 扫描范围

- 后端：`backend/`
- 前端：`frontend/src/`
- **勿扫 repo 根或含 `node_modules` 的目录**：`node_modules/.bin` 里的 npm 残留临时文件会让 deepsec 的 `rglob` stat 崩溃（`OSError: WinError 1920`），按核心目录逐扫即可。

## 层

- **L1**（正则 + 熵分析）：幻觉包、硬编码密钥、不安全配置、AI 错误模式 —— 默认开
- **L2**（Tree-sitter AST）：SQL 注入 / XSS / SSRF / 路径穿越 / 命令注入 —— 默认开
- **L3**（LLM 语义）：需配置 `DEEPSEC_LLM_API_KEY` 且 `--remote-l3` 显式开启，**会上传代码**，默认关闭、勿开

## 已知误报（人工判定，勿当漏洞提交）

- `backend/internal/logger/redact.go` 的 `credentialURLRe` 脱敏正则被误判为 Database URL —— 完全误报
- `docker-compose.prod.yml` 的 `DATABASE_URL: postgres://${DB_USER:-forklift}:${DB_PASSWORD}@...` —— 环境变量占位符，密码经 `.env` 注入，非硬编码
- `docker-compose.yml` / `docker-compose.local.yml` 的本地开发密码 `forklift123` —— 开发栈已知密码，非生产泄漏（生产走 `.env`）
- `backend/cmd/import-reference-content/courses.go` 的 `n1CredentialCode = "forklift_n1"` 等证件 code 常量被误判为硬编码密钥 —— 常量是证件分区标识（CONTEXT.md 术语），非凭据
- `backend/internal/service/checkin_service.go:34` 的 SQL 字符串拼接 —— 表名/列为代码内白名单常量，无用户输入参与拼接，参数值全部走占位符

## 使用时机

改动触及以下任一时，跑一次 Shield 并确认无新增 critical/high（除上面已知误报）：

- 认证 / 授权 / 会话 / JWT
- 数据库连接串 / 密钥 / 敏感配置
- 文件上传 / 反序列化 / 模板渲染
- AI 生成或 AI 参与编写的代码

## 报告与清理

- `--format json|markdown --output <path>` 写报告；扫完删除产物，勿留在 repo 根污染 git。
