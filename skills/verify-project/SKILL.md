---
name: verify-project
description: 全栈项目变更验证流程。写完代码或提交 PR 后自动执行，覆盖后端/前端/部署配置三类检查，全部通过才允许宣告完成。
---

# verify-project

适用于本仓库（叉车维修培训与残值评估系统）的变更后验证流程。每次完成代码改动或准备提交 PR 时，按顺序执行以下检查。

## 前置条件

- Go 工具链在 PATH 中（`~/go/bin`）
- Node.js 可用（frontend 目录）
- Docker / docker compose 可用（仅验证配置文件时）

## 步骤 1：后端检查（backend/）

在 `backend/` 目录下依次执行：

```bash
gofmt -l .
go vet ./...
golangci-lint run ./...
go test ./...
```

逐条判断：

- `gofmt -l .`：**必须无输出**。有任何文件路径说明有未格式化代码，直接失败。
- `go vet ./...`：**必须无错误**。有输出即失败。
- `golangci-lint run ./...`：**必须无错误**。有输出即失败。
- `go test ./...`：**必须全部通过**。注意：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，属于已知例外，可忽略，其余测试失败必须修复。

如果任何一条失败，记录失败命令和输出，停止后续步骤，返回修复。

## 步骤 2：前端检查（frontend/）

进入 `frontend/` 目录，依次执行：

```bash
npm run type-check
npm test
```

逐条判断：

- `npm run type-check`（vue-tsc）：**必须无错误**。有 TypeScript 类型错误即失败。
- `npm test`（vitest）：**必须全部通过**。有测试失败即失败。

如果任何一条失败，记录失败命令和输出，停止后续步骤，返回修复。

## 步骤 3：部署配置语法校验

如果本次改动涉及以下文件，必须额外执行语法校验：

- `docker-compose*.yml`
- `deploy.sh`

执行：

```bash
docker compose -f docker-compose.prod.yml config -q
```

**必须无输出且退出码为 0**。有任何输出或报错即失败。

如果 `docker-compose.prod.yml` 不存在或不适用，改为校验实际使用的 compose 文件。

## 步骤 4：构建检查（可选但推荐）

如果改动涉及编译产物或构建配置，执行一次完整构建：

- 后端：在 `backend/` 执行 `go build -o bin/server ./cmd/server`
- 前端：在 `frontend/` 执行 `npm run build`

构建必须成功完成，无错误。

## 汇报格式

检查全部通过后，输出简洁的通过报告：

```
✅ verify-project 通过
  后端: gofmt / go vet / golangci-lint / go test 全部通过
  前端: type-check / test 全部通过
  部署配置: docker compose config -q 通过
  构建: 通过（如执行）
```

如果有任何步骤失败，输出失败报告：

```
❌ verify-project 失败
  步骤: [步骤名称]
  命令: [失败的完整命令]
  输出:
  [关键错误输出，前 20 行]
  修复建议: [根据错误类型给出修复方向]
```

## 注意事项

- 每步命令必须**完整执行完毕再判断结果**，不要提前中断。
- `go test ./...` 在 Windows 上如果遇到已知例外（`TestStaticOtherResource`），记录为已知问题但不阻塞，其余失败必须修复。
- 前端检查必须在 `frontend/` 目录下执行，不能在其他目录。
- 如果改动只涉及某一端（例如只改了前端），可跳过未改动端的检查，但必须在报告中注明跳过了哪一步及原因。
- 本 Skill 只覆盖能明确判断对错的自动化检查；UI 视觉、交互体验等主观判断不属于本 Skill 范围，需人工验收。
