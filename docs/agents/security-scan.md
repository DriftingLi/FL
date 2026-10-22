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
- `backend/internal/checkin/service.go:37` 的 SQL 字符串拼接 —— 表名/列为代码内白名单常量，无用户输入参与拼接，参数值全部走占位符

## 使用时机

改动触及以下任一时，跑一次 Shield 并确认无新增 critical/high（除上面已知误报）：

- 认证 / 授权 / 会话 / JWT
- 数据库连接串 / 密钥 / 敏感配置
- 文件上传 / 反序列化 / 模板渲染
- AI 生成或 AI 参与编写的代码

## 报告与清理

- `--format json|markdown --output <path>` 写报告；扫完删除产物，勿留在 repo 根污染 git。

## 依赖侧（Dependabot + Go 安全版本）

Shield 扫的是**我们自己写的代码**，看不到「依赖里的已知漏洞」和「工具链本身的漏洞」这两层。补法：

### 1. Dependabot

- **告警与安全更新**：仓库设置里开启（`Dependabot alerts` + `Dependabot security updates`，2026-10-22 用 API 打开）。
  这层由 GitHub 按 CVE 触发，不受配置文件控制；有漏洞的依赖会自动开 PR（不需要等周更）。
- **版本更新**：`.github/dependabot.yml` 覆盖 `gomod:/backend`、`npm:/frontend`、`github-actions:/`、
  `docker:/{backend,frontend,libreoffice-sidecar}`，每周一 09:00（Asia/Shanghai）跑一次，minor+patch 合成一个 PR。
- **刻意不覆盖移动端**（`training-app/叉车维修培训学员端跨端应用`）：那一侧由 @zhengcookie 负责，
  PR 要过真机验收门（四门 + 证据），机器开的依赖单到不了那道门，只会变成红单。

### 2. Go 标准库漏洞看的是**构建工具链**，不是 `go.mod`

`backend/go.mod` 的 `go 1.26.0` 只是**语言版本下限**，真正决定二进制里那份标准库的是构建它的 Go：

- 生产镜像：`backend/Dockerfile` 的 `FROM golang:<版本>-alpine`；
- CI：`.github/workflows/ci.yml` / `cd.yml` 的 `GO_VERSION`（两者必须同源）；
- 本机 `go version` 只影响本地开发，不影响线上。

所以「某个标准库漏洞有没有修」的判据是：**这四处里产生产物用的那个版本是否 ≥ 公告的修复版本**。
例：`GO-2026-6809`（CVE-2026-111943，`crypto/x509` 错误信任已撤销的中间证书）修在 `1.26.10` / `1.27.3`；
本仓镜像与 CI 都在 `1.26.10`（不受影响），而本机 `1.27.1` 低于 `1.27.3`（本地构建属受影响区间）。

### 3. 想知道「哪些依赖漏洞真的能被走到」

OSV / Dependabot 给的是「版本命中」，不是「可达」。要判可达性用 `govulncheck`：

```bash
cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

本仓实测（2026-10-22）：用**生产同款** `go1.26.10` 跑 → `0 vulnerabilities`（标准库 0 条可达）；
用当时的本机 `go1.27.1` 跑 → `13 vulnerabilities from the Go standard library` 全部可达。
同一份代码、同一天，差别只在工具链 —— 所以「本机扫出来一片红」先别慌，先看用的是哪个 Go。

它按调用图报告「这些漏洞有代码路径能到达」，是排优先级的第一手依据；只按版本清单排，会把
大量根本没被调到的传递依赖排到前面。