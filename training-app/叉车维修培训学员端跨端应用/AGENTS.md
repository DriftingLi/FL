# 叉车维修培训学员端（移动端）执行单

**本页只写一件事：一个运行时面 PR 从开工到收尾。** 参考表在按需层，别往这里抄——抄一次就是一处漂移源。为什么只剩八步见 [`docs/adr/0048-常驻层收编为一页执行单.md`](docs/adr/0048-常驻层收编为一页执行单.md)。

- 系统级约定在仓库根 [`AGENTS.md`](../../AGENTS.md)；领域词汇表根 `CONTEXT.md`；派活一行 = `按 training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md 执行 <issue URL>`。
- **移动端 ADR 是独立编号体系**（与根 `docs/adr/` 的 `ADR-0001+` 无关）⇒ 引用**写全路径** `training-app/叉车维修培训学员端跨端应用/docs/adr/…`，否则在根会解析到同名他文。
- 主树 `D:\FL`；**别照抄任何 `E:\FL` 路径**（原备胎盘已废弃）。

## 八步执行单

| 步 | 命令 | 判据（读数行） | 失败先查 |
| --- | --- | --- | --- |
| 1 开工 | `pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`；建树后立刻建分支、尽早推 origin | `git status --porcelain` 无 `training-app/` | 宿主树里改 = ③ 门静默假绿（#1144）。判据看**意图**（含未 add 的新文件），不看 `git diff` |
| 2 改完自检（**含页面观测**） | ① `npm run hx:compile-only` 拿编译期诊断；② `.uvue` / 样式 / UI 状态改动 **PR 前看真实页面**，显示类看页面、交互类必须实操：有设备走 `npm run hx:run`；没设备走小程序运行时（微信开发者工具 + automator）；都不通才写 `页面观测：BLOCKED（原因）`，不许用编译或单测替代 | `HX_RUN mode=` 无 `ERROR`；页面观测写 `PASS / FAIL / BLOCKED`（三档见 dev-loop「页面观测三档」） | **编译成功 ≠ 页面验证成功**（#2804）；`style:loop` 本机未验证、**不作默认**；真运行 / 截图 / 只编译三者不可互替 |
| 3 ③ 两腿 | `npm run test:unit:changed`（接线全量 + 行为**受影响子集**；全量两腿走 `npm run test:unit:gate`） | `TEST_UNIT_CHANGED_RESULT` / `TEST_BEHAVIOR_CHANGED_RESULT … mode=changed planned=N` | 子集红而 CI 绿 → 先自查，**不当 flake**；选错腿的 config = 0 套件假绿 |
| 4 ④c（+④a） | `npm run build:kotlin-all`；命中三份 json / 新增页 / 模块手术再加 `npm run build:compile` | 退出码 0 | ④a 命中**必须清缓存**：`pages.json` 改一行不清 = 产物没更新还报编译成功 |
| 5 可选构建面检查（② 已退役） | 需要时手动跑 `npm run build:mp-weixin-buildonly`（**不是门**、不判红、不产截图）；**发行上传目录 `unpackage/dist/build/mp-weixin`**（[ADR-0064](docs/adr/0064-小程序端发布范围仅学员端与唯一发行路径.md)，非 dev 产物） | 无门禁判据；结果行 `MP_WEIXIN_BUILDONLY_RESULT` 只作人工诊断读数 | 日常**不要**为凑证据跑 ② 或启开发者工具；真要诊断才手动启，截图只作参考、不进证据 |
| 6 证据 | 正文 `## 验收证据` 分三段 `### 编译 / ### 测试 / ### 页面观测`（模板见根 `.github/PULL_REQUEST_TEMPLATE.md`） | 编译段一行 `- ④ 跑于 <sha7> · <读数行原样>`；测试段 ③ 读数或「见 CI `ci-summary`」；页面段 `页面观测：PASS/FAIL/BLOCKED（原因）` + `人工验收：待确认`（「通过」只有人能写） | **编译段不得替代页面段**：没观测就写 `BLOCKED`，不许留空、不许把 ④ 抄进页面段；图不作证据（#2320） |
| 7 开 PR 与合并 | `pwsh scripts/preflight-pr.ps1`（**仓库根**跑）预检绿 → `pwsh scripts/submit-pr.ps1` 建 PR | 证据齐 → agent 直接合并 | 例外通道只认「已接受未验证风险」一句，**且由人合并** |
| 8 收尾 | `pwsh scripts/dev-close.ps1 -Task <票号>`（仓库根；默认 dry-run，`-Yes` 执行）；**认树看目录 `wt-<票号>`、身份看登记分支**（`fix/…` 也认，别按 `feat/<票号>` 猜；裁定见根 `docs/adr/ADR-0092-树身份-目录为入口与登记分支为身份.md`） | `DEV_CLOSE_RESULT … verdict=ok`（dry-run 只 `verdict=planned`、无动作是 `verdict=noop`，**都别当已收尾**） | `verdict=blocked reason=pr-not-merged` = 还没合并，本就不该拆；`tree-branch-mismatch` = 目录在但判据没匹配上（必红，别当无树）；树被占用 ⇒ 关掉占用的工具再跑，别 kill |

**第 5 步的纪律面**：② 门已于 2026-11-05 整体退役（#2320）——日常 PR **不跑 ②**，要诊断才手动跑一次、只进日志；风险由发版前真机冒烟（ADR-0043）兜底，见 ADR-0008 顶部条。

**成本尺**：≈12 min（同树）/ 17 min（新树）；超 2× 先查排队（HBuilderX 串行）。

**第 2 步四条红线**（全表在 [`docs/agents/uvue-css.md`](docs/agents/uvue-css.md)）：`<style>` 必须 `lang="scss"`；选择器只认 `.class`（tag / ID / 伪类 / 属性选择器一律不可用）；文字类样式只落 `<text>` / `<button>` / `<input>` / `<textarea>`（落 `<view>` 上静默忽略并逐处打 error，机检 `utils/uvueFontCarrierContract.test.js`）；`gap` / `calc()` / `env()` / `var(--x)` / `transition` / `animation` 原生端不支持。**WARNING 不阻断但可能不生效**（`gap` 被静默忽略就是这样把布局搞坏的）。

## 验收门与合并纪律

- **证据 = PR 正文一行 `跑于 <sha7>` + 门读数行原样**（A1 形态）。sha 绑定门评论 / 三候选集 / `-PostToIssue` / 展示副本**全退役**；日常 PR 不产截图、图不作证据（#2320）。判据见 [`docs/adr/0008-移动端验收门与证据.md`](docs/adr/0008-移动端验收门与证据.md)（**写全路径**）。
- **红是「看见」不是「拦截」**：ruleset 必检只有 `ci-summary` 与 `ensure-testing-smoke` 两条，`pr-evidence` 不在必检 ⇒ 证据体系是荣誉制，没人替你兜底。
- **① 整门退役**：①a 降为发版前一次（发版冒烟）；①b 与能力面人签**整体退役**——命中能力面的 PR 合并前**零人签**、不要设备在线，命中才写一行 `能力面：<类别> · 已记入发版冒烟`。发版冒烟固定人工场景清单**全做、不按 diff 生成**（判据以 ADR 为准），载体是移动端 `docs/adr/0043-真机门降级常态化与发版冒烟（2026-10-23 裁定）.md`「发版冒烟固定人工场景清单」段；它管能力场景，决策 5「按 diff 选改动页」管页级视觉；清单不进本页（ADR-0048 ③）。
- **③④ 照旧必过**；云打包按打包面触发（三份 json / `uni_modules` app-android / 新增 aar·jar），正式发版前必跑并装机自测。门脚本须**全访问权限**执行。**③ 全量本机跑不通**（#2876）：读数由 CI 承担，见 docs/agents/dev-loop.md「全量 ③ 门该不该跑」。
- **⑤ 不门化（2026-10-29，#2130）**：命中运行时面时 PR 前**建议**跑一次 `npm run build:mp-weixin-buildonly`——可选、不判红，`build=skipped(fingerprint)` 也算编译过；构建面检查不替代运行时诊断。
- 非运行时面 PR（纯文档 / 测试 / CI 配置）agent 自行合并；例外通道仍由人合并。

## 开发内循环（移动端 UI 迭代）

两个入口：`npm run hx:compile-only`（=`hx-run.ps1 -CompileOnly`，**仅编译**，只拿诊断不推设备）与 `npm run hx:run`（真运行；**不重装基座**，到没到设备看**设备侧事实相对基线前进**，不看「前台是不是基座」）。门全量、日常分层；时间账与 `--compile` 坑位见 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)。

### 三层节奏

内循环 `hx:compile-only`（热缓存 257 s / 冷启 826–901 s）/ 中循环每个视觉满意点一次（`npm run test:unit:changed` + `npm run build:kotlin-all` + 一次提交）/ 外循环主题收口一次，见 `dev-loop.md`；**都不是门**。

**一个视觉主题一个 commit**——别把色值微调与结构删除压同一提交（`56d0fe3`）。

## 反模式（逐条禁）

在真机验本该编译期就红的事 · 每次微调都全量重编译 + 全量门 + 提交 · kill `cli` 或 HBuilderX · 拿仿真机当热刷新 · 跑完不查工作树。原因见 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)。

## 并发会话纪律（多会话并行时必读）

HBuilderX 的 CLI 步骤**由仓内锁串行化**——不是平台上限（本仓选择，官方未记载多实例）。纪律 6 行，本页**唯一入口**（血账 [`docs/agents/multi-agent-git.md`](../../docs/agents/multi-agent-git.md)，预算 [`docs/agents/concurrency.md`](docs/agents/concurrency.md)）：

```
1. 一票一树一分支：建树只走 `pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`，别裸用 `git worktree add`。
2. 提交前验归属：`git branch --show-current` + `git log --oneline origin/master..HEAD`；只 `git add <本次文件>`，绝不 `git add -A`。
3. `pages.json` / `manifest.json` / `platformConfig.json` 只改工作区，不 add、不 commit（「别人持有」要两条判据同时成立）。
4. 撞锁就等 120 s，超时 exit 2；换活只换不吃 HBuilderX 的。并发上限 2。**先查现场再排队**：`npm run hx:run -- -Status`（零副作用，`HX_STATUS` 一行说明谁占着、多久、是否陈旧）；要等更久给 `-WaitSeconds <秒>`，立刻退给 `-NoWait`（详见 `docs/agents/concurrency.md`「快查占用」）。
5. 收尾一条命令：`pwsh scripts/dev-close.ps1 -Task <票号>`（默认 dry-run，`-Yes` 执行）。
6. 删树前把 `.ci-verify/` `.scratch/` 里会被引用的产物拷出或入仓 —— 被 gitignore 的文件会跟着树一起消失。
```

## 发布流程

权威在根 [`docs/agents/release.md`](../../docs/agents/release.md)：分支 + PR + ruleset（拒直推、`ci-summary` 必检且要求 up-to-date）+ squash 直发 production。**git/gh 写操作一律后台跑**——`timeout` 截半比失败更糟。收尾见第 8 步。

## 前端 UI 约定

与 Web 栈**不同**的两条 hint 例外（论坛属地披露 / Markdown 边界提示）与「删 hint 同步删 CSS class」见 [`docs/agents/ui-hints.md`](docs/agents/ui-hints.md)。页面整洁通则与 Tailwind 四条规则对本线**不适用**（无 Tailwind、无 `src/`、无 brand 色域）。

## 代码规范

文件 kebab-case、组件 PascalCase；导入顺序 `@vue/*` → uni-app API → 第三方 → 本地。`v-for` 必须带 `key`、**避免用 `index`**（`docs/adr/0025`）。props / emit 优先，跨层才上 provide / inject 或 Pinia；**页面壳层零自持状态**——`ref` / `reactive` / `computed` 落进 `composables/` 并进显式结果类型，页面只解构消费（契约测试按页机检）；平台差异走 `#ifdef`；样式用 rpx、触摸目标 ≥44×44。api 层**禁裸打印整段 response**，走 `api/request.uts` 的 `sanitizeForLog`。

## 注意事项

- 一次改动一个 commit；改动后补测试、跑过再回话。**改导出符号（常量 / 函数改名）= 同批改钉名断言**——契约测试钉死名字与字面量，只改实现必红；hint 常量同步 `docs/agents/ui-hints.md` 登记行。
- **动图片前先读 `docs/adr/0066`**：① 包内**禁 webp**（小程序 image 不解析包内 webp，真机/体验版空白）② 喂**网络图**的 `<image>` 必须写 `:webp="true"`（后端把 jpg/png/bmp 转 webp 存，封面/头像/验证码一律网络 webp）③ 新图用**逐张取优的调色板 PNG**，不放大。
- **动 `static/` 里的图之前先算主包余量**：`G7a` 只打读数**不判红** ⇒ 破线是静默的；读数跑 `npm run build:mp-weixin-buildonly` 拿 `mainBytes=`，别拿 dev 产物或旧记录估（2026-11-14 换 7 图后 1,419,854 B，距 2 MiB 硬限 677,298 B）。
- **切图落位按 [`docs/agents/image-assets.md`](docs/agents/image-assets.md) 执行**：五档 verdict 决定何时做（`PLACEHOLDER` 可插队、`UNIMPLEMENTED` 跟实现票、`UNUSED` 并入结构改造批），**别单开换图票**——「先补图、后改结构」的返工源。
- 定时器挂 uni-app 生命周期，别裸用 `setTimeout`/`setInterval`。
- 跑完 HBuilderX 步骤先 `git status`：`manifest.json` 的 appid 会被回写置空、`pages.json` 会被写入启动页 `condition`——本地开发配置、禁止提交。删树前先 `cli project close --path <项目>`。
- 一件票一个会话，冷启动只读票 + 对应 ADR；新决策回写 ADR 再关票。写 ADR 守中文文件名 + 固定六段 + ≤25 行，见 `docs/refactor-decisions.md`。

## 按需层（本线 `docs/agents/`）

`guards.md` 守护分类与新增三问 · `issue-dispatch.md` 派活与阶段闸门 · `uvue-css.md` 样式与 UTS 约束全表 · `concurrency.md` 锁与时间预算 · `dev-loop.md` 内循环时间账与坑位原文 · `skills.md` 技能供给 · `ai-workflow.md` 专项 AI 工作流与让位表 · `ui-hints.md` UI hint 例外 · `image-assets.md` 切图落位口径 · `path-encoding.md` 中文路径四边界（动 git 路径输出 / 中文路径判据前读） · `domain.md` / `issue-tracker.md` / `triage-labels.md` / `credential-scope.md` 各按其名。

修改搜索页或首页宫格视觉时，先读 [`DESIGN.md`](DESIGN.md) 中对应的视觉约定。

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.
AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.
