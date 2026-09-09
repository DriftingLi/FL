# docs/ 目录索引与存储规范

> 本目录混存「入库文档」与「本地协作文档」两类，按 `.gitignore` 的 `docs/*` 例外规则区分：
> **入库** = `adr/`（架构决策记录，核心资产）、`agents/`（AI/agent 工作约定，随 AGENTS.md 导航拆分入库，2026-09-09 起）、本 README；
> **本地不入库** = `plans/`、`reference/`、`archive/` 及其余未列外名的文件。

## 目录结构

| 目录 | 入库 | 用途 |
| --- | --- | --- |
| `docs/adr/` | ✅ | 架构决策记录（`ADR-0001-…`，38 篇；移动端另有独立 ADR 体系见 `training-app/…/docs/adr/`） |
| `docs/agents/` | ✅ | AI/agent 工作约定，由根 `AGENTS.md` 导航（issue-tracker / triage-labels / domain / security-scan / ui-conventions / checks / release / multi-agent-git） |
| `docs/README.md` | ✅ | 本索引 |
| `docs/plans/` | ❌ | 产品 / 技术方案、实施计划（当前为空；命名建议 `主题-方案.md`） |
| `docs/reference/` | ❌ | 参考资料：业务数据表、评估填报界面、代码 Wiki 等 |
| `docs/archive/` | ❌ | 已归档 / 被取代的方案（如 try-uniapp-mvp-isolated/） |

## 存储约定

1. **需版本化的约定 / 规范类文档**放 `docs/agents/`（或 ADR 形态放 `docs/adr/`），并在根 `AGENTS.md` 挂导航。
2. 新方案、改造计划放 `docs/plans/`（本地），命名 `主题-方案.md`；定稿并需要沉淀的结论升格为 ADR 或 agents 约定后入库。
3. 参考资料（xlsx / png / 题库 / Wiki 等）放 `docs/reference/`；已完结或被取代的方案移入 `docs/archive/`，不直接删除。
4. 工具目录（`.trae/`、`.workbuddy/`、`.codex/`）只保留工具自身状态，不再存放项目文档。
5. 敏感文件（密钥、私钥、证书）禁止进入 `docs/`，统一放入 `.secrets-quarantine/` 并保持不入库；生产密钥建议尽快轮换并转移到离线保管。
6. 清理时确认无用的文件先移入 `.trash/`，复查后再手动删除。
7. 保留原位的特殊文件：`.trae/docs/N1题库导入脚本/`（`backend/migrations/000001_init_baseline.up.sql` 注释引用其路径）。

## 索引

### docs/agents/（入库）

| 文件 | 内容 |
| --- | --- |
| `issue-tracker.md` | Issue 存放与使用（GitHub Issues + `gh` CLI） |
| `triage-labels.md` | 五个 canonical triage labels |
| `domain.md` | Single-context 领域文档纪律（CONTEXT.md + ADR） |
| `security-scan.md` | DeepSec（Shield）安全审计用法与已知误报 |
| `ui-conventions.md` | 前端 UI 约定（UI 词汇 / Tailwind R1-R4 / 不得触碰的边界） |
| `checks.md` | 测试与检查流程（后端 Windows/WSL 双环境、前端、部署配置、安全检测） |
| `release.md` | 发布流程（分支 + PR + ruleset 门禁 + squash 直发 production） |
| `multi-agent-git.md` | 多 Agent 并发与 git 隔离（worktree / 游离提交 / add 纪律） |

### docs/adr/（入库）

38 篇，编号 `ADR-0001-…`（如 `ADR-0001-验证码通道适配器seam`、`0028-打卡积分直记化与每日登录事实源迁移`）。清单与最新决策见仓库 `docs/adr/` 目录本身，不在此重复维护。

### docs/plans/（本地，当前为空）

### docs/reference/（本地）

- CODE_WIKI.md（代码 Wiki）
- 叉车残值评估填报.png
- 叉车价格对照表.xlsx
- 叉车配置大全.xlsx
- 二手叉车评估表.xlsx

### docs/archive/（本地）

- try-uniapp-mvp-isolated/（uniapp MVP 试点方案：spec / tasks / checklist）
