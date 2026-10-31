# 叉车维修培训学员端（移动端）执行单

**本页只写一件事：一个运行时面 PR 从开工到收尾。** 别的参考表在按需层，别往这里抄——抄一次就是一处漂移源。为什么只剩八步见 [`docs/adr/0048-常驻层收编为一页执行单.md`](docs/adr/0048-常驻层收编为一页执行单.md)。

- 系统级约定在仓库根 [`AGENTS.md`](../../AGENTS.md)；领域词汇表根 `CONTEXT.md`；派活一行 = `按 training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md 执行 <issue URL>`。
- **移动端 ADR 是独立编号体系**（与根 `docs/adr/` 的 `ADR-0001+` 无关）⇒ 引用**写全路径** `training-app/叉车维修培训学员端跨端应用/docs/adr/…`，否则在根会解析到同名他文。
- 主树 `D:\FL`；**别照抄任何 `E:\FL` 路径**（原备胎盘已废弃）。

## 八步执行单

| 步 | 命令 | 判据（读数行） | 失败先查 |
| --- | --- | --- | --- |
| 1 开工 | `pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`；建树后立刻建分支、尽早推 origin | `git status --porcelain` 无 `training-app/` | 宿主树里改 = ③ 门静默假绿（#1144）。判据看**意图**（含未 add 的新文件），不看 `git diff` |
| 2 改完自检 | `npm run hx:compile-only` | 无 `ERROR` | 四条红线见下；这一步**不碰设备**，别拿真机验编译期的事 |
| 3 ③ 两腿 | `npm run test:unit:changed`（接线全量 + 行为**受影响子集**；全量两腿走 `npm run test:unit:gate`） | `TEST_UNIT_CHANGED_RESULT` / `TEST_BEHAVIOR_CHANGED_RESULT … mode=changed planned=N` | 子集红而 CI 绿 → 先自查，**不当 flake**；选错腿的 config = 0 套件假绿 |
| 4 ④c（+④a） | `npm run build:kotlin-all`；命中三份 json / 新增页 / 模块手术再加 `npm run build:compile` | 退出码 0 | ④a 命中**必须清缓存**：`pages.json` 改一行不清 = 产物没更新还报编译成功 |
| 5 ② 改动页 | `npm run build:mp-weixin-check`（**② 微信开发者工具门 = 半自动门**，执行人允许「agent 执行」） | `MP_WEIXIN_RESULT` 整行 + `coverage=changed:N\|smoke:2\|none` | `none` = 覆盖判据不可用（git diff 取不到，只剩入口页；不会退化成冒烟集）**不阻断**，但须原样进正文；清单为空才跑冒烟两页；`publish` 每 PR **只一次**，重试不再编译 |
| 6 证据 | — | 正文一行 `跑于 <sha7>`；门读数行**原样**跟在后面（A1 已无「结论」栏）；图只作附证 | 图不是判据；`anchors=0` 不判红（人看该页） |
| 7 开 PR 与合并 | `pwsh scripts/preflight-pr.ps1`（**仓库根**跑）预检绿 → `pwsh scripts/submit-pr.ps1` 建 PR | 证据齐 → agent 直接合并 | 例外通道只认「已接受未验证风险」一句，**且由人合并** |
| 8 收尾 | `pwsh scripts/dev-close.ps1 -Task <票号>`（仓库根；默认 dry-run，`-Yes` 执行） | 树摘净、分支残留机检绿 | 该脚本尚未落地：暂用 `training-app/叉车维修培训学员端跨端应用/scripts/remove-worktree.ps1 -Task <n>`，再在干净树里 `git fetch && git merge --ff-only origin/master`。拆树别手搓两步 |

**第 5 步的纪律面**：纯 `.uvue` / `.uts` 命中运行时面时，② 属**纪律必跑**——校验器对这一支**不判**（`needs2` 只认 MP-WEIXIN 面），但 ADR-0043 决策 3 已把「每个运行时面 PR 都执行 ②」定为常态主门；**别把校验器的「免」当成不必执行**。

**成本尺**：常态 ≈12 min（同树）/ 17 min（新树）。超 2× 先查是不是在排队（HBuilderX 单实例，不是脚本慢）。

**第 2 步四条红线**（全表在 [`docs/agents/uvue-css.md`](docs/agents/uvue-css.md)）：`<style>` 必须 `lang="scss"`；选择器只认 `.class`（tag / ID / 伪类 / 属性选择器一律不可用）；文字类样式只落 `<text>` / `<button>` / `<input>` / `<textarea>`（落 `<view>` 上静默忽略并逐处打 error，机检 `utils/uvueFontCarrierContract.test.js`）；`gap` / `calc()` / `env()` / `var(--x)` / `transition` / `animation` 原生端不支持。**WARNING 不阻断但可能不生效**——`gap` 被静默忽略把布局搞坏就是这一类。

## 验收门与合并纪律

- **证据 = PR 正文一行 `跑于 <sha7>` + 门读数行原样**（A1 形态）。sha 绑定门评论 / 三候选集 / `-PostToIssue` / 展示副本**全退役**；图走 GitHub 附件、只作附证。判据全文与来路见 [`docs/adr/0008-移动端验收门与证据.md`](docs/adr/0008-移动端验收门与证据.md)（**写全路径**）。
- **红是「看见」不是「拦截」**：ruleset 必检只有 `ci-summary`，`pr-evidence` 不在必检 ⇒ 证据体系是荣誉制，别人不替你兜底。
- **① 整门退役**：①a 降为发版前一次（发版冒烟）；①b 与能力面人签**已整体退役**——命中能力面的 PR 合并前**零人签**、不要求设备在线，正文命中才写一行 `能力面：<类别> · 已记入发版冒烟`。发版冒烟的人工场景清单**固定全做、不按 diff 生成**（5 条 = 3 条常态必做 + 2 条条件项），载体是移动端 ADR `docs/adr/0043-真机门降级常态化与发版冒烟（2026-10-23 裁定）.md`「发版冒烟固定人工场景清单」段——它管**能力场景人工操作**，与决策 5「按 diff 选改动页」的人看范围（管**页级视觉**）是两个不同对象；ADR-0048 ③ 已裁定清单不进本页。
- **③④ 照旧必过**；云打包按打包面触发（三份 json / `uni_modules` app-android / 新增 aar·jar），正式发版前必跑并装机自测。门脚本须**全访问权限**执行。
- **⑤ 不门化（2026-10-29，#2130）**：改的命中运行时面、而 ② 命中位未命中时（`manifest.json` / `platformConfig.json` 未改且 diff 无 MP-WEIXIN 指令行），PR 前**建议**跑一次 `npm run build:mp-weixin-buildonly`——可选、不判红；`build=skipped(fingerprint)` 也算编译过。
- 非运行时面 PR（纯文档 / 测试 / CI 配置）agent 自行合并；例外通道仍由人合并。

## 开发内循环（移动端 UI 迭代）

只留两个入口：`npm run hx:compile-only`（= `scripts/hx-run.ps1 -CompileOnly`，官方语义的**仅编译**，只拿诊断、不推设备）与 `npm run hx:run`（真运行；**不重装基座**、无干净缓存重建，到没到设备靠**设备侧事实相对基线前进**判，不靠「当前前台是不是基座」）。**门是全量的，日常是分层的**；时间账、`--compile` 语义坑位与样式内循环全在 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)。

### 三层节奏

内循环（`hx:compile-only` 拿编译期诊断：热缓存 257 s、冷启动 826–901 s）/ 中循环（每个视觉满意点一次：`npm run test:unit:changed` + `npm run build:kotlin-all` + 一次提交）/ 外循环（主题收口一次）的分层与耗时见 `dev-loop.md`。这三个循环**都不是门**，不进 `## 验收证据`。

**提交粒度：一个视觉主题一个 commit**——色值微调与结构删除别压在同一个提交里（反例 `56d0fe3`，+21 / −71：回滚无从下手）。

## 反模式（逐条禁）

在真机验本该编译期就红的事 · 每次微调都全量重编译 + 全量门 + 提交 · kill `cli` 或 HBuilderX 主程序 · 拿仿真机当热刷新用 · 跑完 HBuilderX 步骤不查工作树。逐条原因与成本账两条（sha 无害前进就重跑 / ② 失败不带 `-SkipBuild` 重试）的反例在 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)。

## 并发会话纪律（多会话并行时必读）

HBuilderX 是**单实例串行资源**——这是架构约束，不是可调优的缺陷。纪律只有这 6 行，本页是**唯一入口**（历史血账在 [`docs/agents/multi-agent-git.md`](../../docs/agents/multi-agent-git.md)，时间预算在 [`docs/agents/concurrency.md`](docs/agents/concurrency.md)）：

```
1. 一票一树一分支：建树只走 `pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`，别裸用 `git worktree add`。
2. 提交前验归属：`git branch --show-current` + `git log --oneline origin/master..HEAD`；只 `git add <本次文件>`，绝不 `git add -A`。
3. `pages.json` / `manifest.json` / `platformConfig.json` 只改工作区，不 add、不 commit（「别人持有」要两条判据同时成立）。
4. 撞锁就等 120 s，超时 exit 2；换活只换成不吃 HBuilderX 的活。并发上限 2。
5. 收尾一条命令：`pwsh scripts/dev-close.ps1 -Task <票号>`（默认 dry-run，`-Yes` 执行）。
6. 删树前把 `.ci-verify/` `.scratch/` 里会被引用的产物拷出或入仓 —— 被 gitignore 的文件会跟着树一起消失。
```

## 发布流程

权威在根 [`docs/agents/release.md`](../../docs/agents/release.md)：分支 + PR + ruleset（拒直推、`ci-summary` 必检且要求 up-to-date）+ squash 直发 production。**git/gh 写操作一律后台跑、等它自然结束**——`timeout` 会把它截半，比失败更糟。合并后收尾见八步第 8 步。

## 前端 UI 约定

与 Web 栈**不同**的两条 hint 例外（论坛属地披露 / Markdown 边界提示）与「删 hint 同步删 CSS class」见 [`docs/agents/ui-hints.md`](docs/agents/ui-hints.md)。页面整洁通则与 Tailwind 四条规则对本线**不适用**（2026-10-03 现测：未引入 Tailwind、无 `src/`、无 brand 色域消费点）。

## 代码规范

文件 kebab-case、组件 PascalCase；导入顺序 `@vue/*` → uni-app API → 第三方 → 本地。`v-for` 必须带 `key`、**避免用 `index` 当 key**（`docs/adr/0025` 引它）。props / emit 优先，跨层才上 provide / inject 或 Pinia；平台差异走 `#ifdef`；样式用 rpx、触摸目标 ≥44×44。api 层**禁裸打印整段 response**，调试走 `api/request.uts` 的 `sanitizeForLog` 管道。

## 注意事项

- 一次改动一个 commit；改动后补测试、跑过再回话。**改导出符号（常量 / 函数改名）= 同批改钉名断言**——契约测试把名字与字面量都钉死，只改实现必然红；hint 常量还得同步 `docs/agents/ui-hints.md` 的登记行。
- 定时器挂 uni-app 生命周期上管理，别裸用 `setTimeout` / `setInterval`。
- 跑完 HBuilderX 步骤先 `git status`：`manifest.json` 的 appid 会被回写置空、`pages.json` 会被写入启动页 `condition`——都是本地开发配置、禁止提交。删树前先 `cli project close --path <项目>`。
- 一件票一个会话，冷启动只读票 + 对应 ADR；新决策回写 ADR 再关票（issue 评论不是必读面）。写 ADR 守中文文件名 + 固定六段 + ≤25 行，见 `docs/refactor-decisions.md`。

## 按需层（本线 `docs/agents/`）

`guards.md` 守护分类与新增守护三问 · `issue-dispatch.md` 派活与阶段闸门 · `uvue-css.md` 样式与 UTS 约束全表 · `concurrency.md` 锁与时间预算 · `dev-loop.md` 内循环时间账与坑位原文 · `skills.md` 技能供给 · `ui-hints.md` UI hint 例外 · `domain.md` / `issue-tracker.md` / `triage-labels.md` / `credential-scope.md` 各按其名。

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.
AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.
