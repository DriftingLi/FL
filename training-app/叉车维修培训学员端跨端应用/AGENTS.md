# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。架构、领域词汇与评审记录见下方文件：

- **领域词汇表**：`CONTEXT.md`（repo 根）
- **架构决策记录（ADRs）**：`docs/adr/`
- **AI/agent 工作约定**：`docs/agents/`

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 前端 UI 约定

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

### Tailwind 增量共存四条边界规则

项目已引入 Tailwind CSS v4，与既有 `<style scoped>` 长期共存（详细背景见 `.workbuddy/plans/student-ui-redesign.md`）。共存期间遵守：

| 规则 | 内容 |
|---|---|
| **R1 原子类区** | `src/components/ui/**` 与已纳入改造的页面：模板只用原子类；`<style scoped>` 仅保留伪元素、`:deep()` 改 Element Plus、keyframes、媒体查询 |
| **R2 冻结区** | 未列入当期改造的页面与组件，scoped 样式**一行不动**。改共用件时新特性一律走 prop + 默认值等于现状，让未传值的调用方零 diff |
| **R3 禁双写** | 同一元素同一属性不允许既有 scoped 类又有原子类。需覆盖 Element Plus 外观时二选一：① scoped 内 `:deep()`；② 原子类加 `!` 后缀（`!px-4`） |
| **R4 迁移动作** | 页面改用原子类后，删除 scoped 块中已被替代的规则（沿用上一段「删 hint 同步删 CSS」的约定） |

**变体一律追加覆盖，不改写现有规则**：新增外观分支写成 `.xx.is-dark { … }` 这类多一个类的选择器追加在样式块末尾，特异性天然高于原单类规则，无需 `!important`。

**样式入口只有一个**：`src/assets/styles/tailwind.css`，新增全局样式写进它或它 `@import` 的文件，不要在 `main.ts` 里再加 import。

### 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `layouts/ValuationLayout.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。
- 残值模块（`pages/student/valuation/**`、`components/valuation/**`）本轮冻结，批量替换色值等机械操作时记得排除。

### uni-app-x (uvue) CSS 兼容性规则

本项目使用 uni-app-x (uvue 模式) 编译到 Android/iOS 原生端。uvue 的 CSS 引擎是**原生渲染器**，仅支持 CSS 属性的子集，与 Web CSS 有显著差异。编写 `.uvue` 文件的 `<style>` 时必须遵守：

#### 必须使用 `<style lang="scss">`

`.uvue` 文件的 `<style>` 块**必须**声明 `lang="scss"`，否则 SCSS 变量不会被预处理，直接传递给 uvue CSS 引擎会报错。

#### 选择器限制（严格）

uvue 原生端**只支持 class 选择器**，以下选择器均不可用：

| 选择器类型 | 示例 | 是否支持 |
|---|---|---|
| class 选择器 | `.className {}` | ✅ 支持 |
| group 选择器 | `.a, .b {}` | ✅ 支持 |
| descendant（后代） | `.parent .child {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| child combinator | `.parent > .child {}` | ⚠️ 支持但有运行时性能损耗 |
| adjacent sibling | `.a + .b {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| **tag 选择器** | `view {}`, `text {}` | ❌ **不支持** |
| **tag + combinator** | `> view + view` | ❌ **不支持** |
| ID 选择器 | `#id {}` | ❌ 不支持 |
| 伪类/伪元素 | `:first-child`, `::before` | ❌ 不支持 |
| 属性选择器 | `[attr] {}` | ❌ 不支持 |

**编写 CSS 时只能使用 `.class-name` 形式的选择器。** 模板中的 `view`、`text` 等标签名不能出现在选择器中。

#### 不支持的 CSS 属性与值

| 不支持的语法 | 替代方案 | 说明 |
|---|---|---|
| `gap` / `row-gap` / `column-gap` | 子元素加 margin class | 在 template 的子元素上添加 `ml-N`、`mr-N mb-N` 等 class |
| `text-decoration` | `border-bottom` 模拟 | 用具体颜色值（不支持 `currentColor`） |
| `calc() + env()` | 固定 rpx 值 | 安全区域需通过 `uni.getSystemInfoSync()` 动态获取 |
| `vh` / `vw` 单位 | 固定 rpx 值 | 仅支持 `number` 和 `pixel`（含 `rpx`） |
| `align-items: baseline` | `flex-start` 或 `center` | 仅支持 `center`/`flex-start`/`flex-end`/`stretch` |
| `max-height: 百分比` | 固定 rpx 值 | 仅支持 `number` 和 `pixel` |
| CSS 自定义属性 `var(--xxx)` | 直接写色值或用 SCSS 变量 | uvue 原生端不支持 CSS 变量 |
| `currentColor` | 具体颜色值 | 如 `#2979ff`、`#999999` |
| `display: grid` / `grid-*` | flex 布局 | grid 布局不支持 |
| `transition` / `animation` | uni-app API 动画 | 原生端不支持 CSS 动画 |

#### `gap` 替换模式

由于 uvue 不支持 tag 选择器，`gap` 替换必须在 **template 的子元素上直接添加 margin class**：

```vue
<template>
  <view class="flex-row">
    <view class="item">A</view>
    <view class="item ml-16">B</view>
    <view class="item ml-16">C</view>
  </view>
</template>

<style lang="scss">
  .flex-row { flex-direction: row; }
  .ml-16 { margin-left: 16rpx; }
</style>
```

- **非换行水平排列**：首个子元素不加 class，后续子元素加 `ml-N`
- **换行排列**：所有子元素加 `mr-N mb-N`
- **v-for 循环**：用 `:class="{ 'base': true, 'ml-N': idx > 0 }"` 条件添加

#### UTS 类型系统限制

UTS（uni-app-x 的 TypeScript 变体）不支持以下 TypeScript 语法：
- **交叉类型 + 内联对象字面量**：`type C = A & { field: type }` → 必须展平为独立类型定义
- **联合字面量类型用于运行时强转**：`'student' | 'tutor'` 编译到 Kotlin 后无法用 `as` 强转 → 统一用 `string`

##### Kotlin 编译期常见错误（云打包 / 发行模式全量编译会暴露）

| 错误写法 | 报错 | 正确写法 |
|---|---|---|
| `undefined` | 找不到名称 undefined | 空值统一 `null` |
| `String(x)` | None of the following candidates is applicable | `x.toString()` |
| `let x : any = null` | Null cannot be a value of non-null 'Any' | `let x : number \| null = null` 等可空类型 |
| `.catch((e : any) => …)` | None of the following candidates is applicable | `.catch((e) => …)` 或 `.catch((e : any \| null) => …)` |
| `Record<string,string> = {a:1}` | Cannot create an instance of an abstract class | `new Map<string,string>()` + `.set()` |
| 事件回调 `(e : any)` 访问 `e.detail` | Unresolved reference 'detail' | `e as UTSJSONObject` + `e['detail']` 索引 |
| 函数定义引用后声明的变量 | Unresolved reference（无 hoisting） | 变量声明前置到函数之前 |
| 模板对可空 `?:` 字段做 `>`/`<` 比较 | Operator call is prohibited on nullable receiver | `(x ?? 0) > 0` 兜底 |
| `getStorageSync` 返回值传给 `setStorageSync` | Argument type mismatch: Any? vs Any | `if (x != null) setStorageSync(k, x as any)` |

> `any` 在 UTS 编译成 Kotlin 的**非空** `Any`；需要可空时用 `any | null` 或具体可空类型。模板属性访问走宽松路径不报错，但 `<script>` 是严格 Kotlin 检查——报错全在 script，别被模板的"安静"误导。

#### 编译验证

修改 `.uvue` 文件后，在 HBuilderX 中重新编译，检查控制台：
- **ERROR** = 阻断编译，必须修复
- **WARNING** = 不阻断但原生端可能不生效（如 `gap` 被静默忽略，布局会坏）

## 测试与检查流程

改动后**必须**跑完对应栈的检查，全绿才能提交：

- **后端（`backend/`）**：Go 工具链在 `~/go/bin`（`export PATH=/home/root86155/go/bin:$PATH`）
  - `gofmt -l .`（应无输出）
  - `go vet ./...`
  - `golangci-lint run ./...`（errcheck 等静态检查）
  - `go test ./...`
  - 已知例外：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，与改动无关，可忽略
- **前端（`frontend/`）**：`cd frontend`
  - `npm run type-check`（vue-tsc）
  - `npm test`（vitest）
- **部署配置**：改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验
- **安全检测**：改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。

## 发布流程（push / PR / merge）

master 有仓库 ruleset「protect master」保护（直接 push 会被拒，`push declined due to repository rule violations`），且限定 squash 合并。发布必须走分支 + PR：

1. **本地提交**（只 add 本次改动的文件，勿 `git add -A`）。
2. **建分支推送**：若提交已在本地 master 上，`git branch feat/xxx` 后 `git reset --hard origin/master` 还原本地 master；然后 `git push -u origin feat/xxx`。
3. **分支 push = CI + testing 冒烟**：push 触发 `ci.yml` 全量 CI（不再同时跑 `pull_request`，一次改动只跑一遍）。CI 全绿**且该分支已有开启的 PR** 时，`ci-summary` 用 `workflow_dispatch --ref <分支>` 派发 `cd.yml` 部署 testing（起栈 → 健康检查 → 立即 down）。孤儿分支/无 PR 分支不占 testing。
4. **创建 PR**：`gh pr create --base master --head feat/xxx --title "..." --body "..."`。**PR 事件不触发任何 CI/CD**；PR 页上的 `ci-summary` 等检查来自第 3 步分支 push 的同 commit run。若 CI 在开 PR 之前就已经跑完（纯文档改动约 20 秒），该 commit 会缺冒烟记录，`testing-smoke.yml` 会在 PR 开启时补发一次（仅此一种条件下动作，不重跑 CI）。
5. **等门禁**：`gh run watch <id> --exit-status` 等 CI 全绿（纯前端改动时 backend-* / migration-check 跳过属正常，`ci-summary` 仍会 success）。ruleset 把 `ci-summary` 设为必检并要求分支 up-to-date —— 未跑绿、或 master 已前进时不能合并（后者要 `git merge origin/master` 同步后重推、重跑 CI）。
6. **Squash merge → 直发 production**：`gh pr merge <n> --squash --delete-branch`。master 的 push **不跑 CI**，直接触发 `cd.yml` 的 `gate` job：从 commit 主题解析 `(#N)` → 校验该 PR head 的 `ci-summary=success`、该 commit 的 testing 冒烟 `success`（冒烟可能晚于合并，gate 最多轮询 15 分钟）→ 通过后才构建镜像并部署 production。若报 "requirements have not been met"，用 `gh pr view <n> --json statusCheckRollup` 排查。
7. **收尾**：`git fetch --prune` → `git checkout master && git pull --ff-only` → 删除本地 feat 分支（若 gh 已自动删）。

**应急通道**：gate 阻断但确认可以上生产时，手动放行 `gh workflow run cd.yml -f environment=production -f ref=<master sha>`（dispatch 不经门禁）。若是 testing 冒烟失败，先到 Actions 重跑该 CD run，再重新合并或走应急通道。

> ⚠️ 本次流水线变更**合并前**切出的分支：其 `ci-summary` 受旧条件约束（仅 master 上报），在 PR 上显示 skipped，既不满足必检也会被 gate 判为 `other`。先 `git merge origin/master` 重推、等 CI 重跑，再走合并流程。

> ⚠️ **不要用 `timeout N` 包裹 git/gh 的写操作**（merge / push / rebase / checkout）。被 SIGTERM 杀掉的是**执行到一半**的操作，比失败更糟：曾因 `timeout 180 gh pr merge --squash --delete-branch` 被中断，残留 `.git/index.lock` 且分支清理删了一半，`frontend/src` 下 265 个文件被删。这类操作一律用后台任务跑并等其自然结束。

Skills provide specialized instructions and workflows for specific tasks.
Use the skill tool to load a skill when a task matches its description.
<available_skills>
  <skill>
    <name>agent-browser-cli</name>
    <description>使用 agent-browser-cli 进行浏览器感知与控制、页面交互、截图/PDF、Cookie/CDP 和排障。</description>
    <location>C:\Users\ZHENG\.agents\skills\agent-browser-cli\SKILL.md</location>
  </skill>
  <skill>
    <name>architecture-decision-records</name>
    <description>Write and maintain Architecture Decision Records (ADRs) following best practices for technical decision documentation. Use when documenting significant technical decisions, reviewing past architectural choices, or establishing decision processes.</description>
    <location>C:\Users\ZHENG\.agents\skills\architecture-decision-records\SKILL.md</location>
  </skill>
  <skill>
    <name>bazi-mingli</name>
    <description>八字命理 (Bazi / Four Pillars of Destiny) divination skill. Use when users request birth chart analysis, fortune telling based on birth date/time, or Chinese astrology. Triggers on: 八字, 四柱, 命盤, 排盤, 算命, 命理, 生辰八字, 天干地支, 五行, 流年, 大運, bazi, four pillars, Chinese astrology, birth chart.</description>
    <location>C:\Users\ZHENG\.agents\skills\bazi-mingli\SKILL.md</location>
  </skill>
  <skill>
    <name>computer-use</name>
    <description>Use Orca's computer-use CLI to inspect and operate local desktop app windows through accessibility trees, screenshots, and safe UI actions. Use for desktop app interaction: list apps/windows, get app state, read visible UI, click controls, type, press keys, scroll, drag, set values, or perform accessibility actions. Also use for browser windows, webviews, Orca app UI, or other desktop UI. Triggers include "computer use", "orca computer", "read Spotify", "read Slack", "control/click/read in a desktop app", or "get app state".</description>
    <location>C:\Users\ZHENG\.agents\skills\computer-use\SKILL.md</location>
  </skill>
  <skill>
    <name>customize-opencode</name>
    <description>Use ONLY when the user is editing or creating opencode's own configuration: opencode.json, opencode.jsonc, files under .opencode/, or files under ~/.config/opencode/. Also use when creating or fixing opencode agents, subagents, skills, plugins, MCP servers, or permission rules. Do not use for the user's own application code, or for any project that is not configuring opencode itself.</description>
    <location>&lt;built-in&gt;</location>
  </skill>
  <skill>
    <name>execute-task</name>
    <description>Execute a single task from a feature plan. Use when the user says "execute task", "执行任务", or wants to implement a specific task from the tasks.md file. Each task = one focused AI session with clean context.</description>
    <location>E:\FL\training-app\叉车维修培训学员端跨端应用\.opencode\skills\execute-task\SKILL.md</location>
  </skill>
  <skill>
    <name>find-skills</name>
    <description>Helps users discover and install agent skills when they ask questions like "how do I do X", "find a skill for X", "is there a skill that can...", or express interest in extending capabilities. This skill should be used when the user is looking for functionality that might exist as an installable skill.</description>
    <location>C:\Users\ZHENG\.agents\skills\find-skills\SKILL.md</location>
  </skill>
  <skill>
    <name>orca-cli</name>
    <description>Use the public `orca` CLI to operate Orca-managed worktrees, folder contexts, terminals, repos, automations, artifacts, worktree comments, and the browser embedded inside the Orca app. Use when the user says "$orca-cli", "use orca cli", "Orca worktree", "child worktree", "cardStatus", "spawn codex/claude in a worktree", "read/wait/send Orca terminal", "terminal send", "full handoff", "handover", "give this to another agent", "another worktree", "Orca browser", "orca artifacts", "share HTML/Markdown", "public artifact link", or "control the browser inside Orca". Prefer this over raw `git worktree`, ad hoc PTYs, Playwright, or Computer Use when the task touches Orca-managed state. Use Computer Use for browser windows, webviews, or desktop UI outside Orca's embedded browser.</description>
    <location>C:\Users\ZHENG\.agents\skills\orca-cli\SKILL.md</location>
  </skill>
  <skill>
    <name>orchestration</name>
    <description>Use Orca orchestration for structured multi-agent coordination: threaded messages, blocking ask/reply flows, task dispatch, worker_done/escalation waits, task DAGs, decision gates, coordinator loops, or decomposing work across agents. Use `orca-cli` instead for full ownership handoffs, including requests phrased as "hand off", "handoff", "handover", "give this to another agent", or "another worktree" when the user did not explicitly ask to supervise, monitor, wait for results, or coordinate a DAG. Use `orca-cli` for ordinary terminal control, lightweight terminal prompts, shell commands, Orca worktree management, reading or waiting on terminals, and automation of the browser embedded inside Orca. Use Computer Use for browser windows, webviews, orca app UI, or desktop UI outside Orca's embedded browser.</description>
    <location>C:\Users\ZHENG\.agents\skills\orchestration\SKILL.md</location>
  </skill>
  <skill>
    <name>plan-feature</name>
    <description>Structured feature planning workflow. Use when the user says "plan feature", "做计划", "写 PRD", "planning", or wants to plan a new feature before implementation. Covers steps 1-4: free-form plan → PRD → issues → tasks.</description>
    <location>E:\FL\training-app\叉车维修培训学员端跨端应用\.opencode\skills\plan-feature\SKILL.md</location>
  </skill>
  <skill>
    <name>review-code</name>
    <description>Structured code review with six review rounds. Use when the user says "review code", "代码审查", or wants to review a PR or feature implementation. Covers logic errors, operation ordering, bad practices, security, magic strings, and pattern improvements.</description>
    <location>E:\FL\training-app\叉车维修培训学员端跨端应用\.opencode\skills\review-code\SKILL.md</location>
  </skill>
  <skill>
    <name>ziwei-doushu</name>
    <description>紫微斗數 (Zi Wei Dou Shu / Purple Star Astrology) divination skill. Use when users request astrolabe chart analysis, fortune telling based on birth date/time, or Chinese star astrology. Triggers on: 紫微斗數, 紫微, 斗數, 命盤, 排盤, 星盤, 十二宮, 命宮, 四化, 大限, 流年, ziwei, purple star, astrolabe.</description>
    <location>C:\Users\ZHENG\.agents\skills\ziwei-doushu\SKILL.md</location>
  </skill>
</available_skills>

## 注意事项

每次改动完成后，都必须创建一个对应的git commit，以便后续追踪和回滚。每次改动后，都必须编写或更新相关测试，并在交互给用户前，确保所有测试和验证全部通过

---

# 叉车维修培训学员端 - 移动端开发约定

> 本文件为移动端（uni-app-x 跨端应用）的 AI agent 工作约定，覆盖开发、测试、构建、发布全流程。
> 根目录 `E:\FL\AGENTS.md` 提供系统级全局约定，本文件补充移动端特有规范。

## 项目概述

- **技术栈**：uni-app-x + Vue 3 + TypeScript
- **目标平台**：Android、iOS、微信小程序、支付宝小程序
- **代码风格**：Composition API + `<script setup>` 语法

## 开发约定

### 文件结构
```
src/
├── pages/           # 页面组件（按功能模块分组）
├── components/      # 公共组件
├── composables/     # 组合式函数
├── stores/          # 状态管理（Pinia）
├── api/             # API 接口封装
├── utils/           # 工具函数
├── types/           # TypeScript 类型定义
├── constants/       # 常量定义
├── static/          # 静态资源
└── assets/          # 样式资源
```

### 代码规范
- **命名**：文件名使用 kebab-case，组件名使用 PascalCase
- **导入顺序**：@vue/* → uni-app API → 第三方库 → 本地模块
- **类型安全**：所有 props、emit、状态必须有 TypeScript 类型
- **组件通信**：优先使用 props/emit，跨层用 provide/inject 或 Pinia

### 平台兼容性
- **条件编译**：使用 `#ifdef` / `#ifndef` 处理平台差异
- **API 选择**：优先使用 uni-app API，原生 API 用条件编译包装
- **样式适配**：使用 rpx 单位，避免固定像素值
- **触摸交互**：确保触摸区域不小于 44px × 44px

## 测试约定

### 自动化测试（E2E）
- 使用 uni-app 官方测试框架 `@dcloudio/uni-automator`
- 测试文件命名：`*.test.js`，放在被测试文件同级目录
- 测试脚本：`npm run test`（默认 H5）、`npm run test:mp-weixin`（微信小程序）

### 测试配置
- **jest.config.js**：Jest 配置文件，定义测试环境和平台参数
- **env.js**：测试设备配置（H5/Android/iOS/微信小程序）
- **示例测试**：`pages/index/index.test.js`

### 测试覆盖要求
- 工具函数：100% 覆盖
- 组件：核心交互逻辑覆盖
- API 模块：mock 测试覆盖

### 运行测试
```bash
# 安装依赖
npm install

# 运行 H5 测试
npm run test:h5

# 运行微信小程序测试
npm run test:mp-weixin

# 运行 Android 测试
npm run test:android
```

## 构建与发布

### 开发环境（使用 HBuilderX）
```bash
# 启动 H5 开发服务器
# 在 HBuilderX 中运行：运行 → 运行到浏览器 → Chrome

# 启动微信小程序开发
# 在 HBuilderX 中运行：运行 → 运行到小程序模拟器 → 微信开发者工具

# 启动 Android 开发
# 在 HBuilderX 中运行：运行 → 运行到手机或模拟器 → Android
```

### 生产构建（使用 HBuilderX）
```bash
# 构建 H5
# 在 HBuilderX 中发行：发行 → H5-手机版

# 构建微信小程序
# 在 HBuilderX 中发行：发行 → 小程序-微信

# 构建 Android APK
# 在 HBuilderX 中发行：发行 → 原生App-云打包
```

### 发布流程
1. **代码审查**：所有改动必须通过 PR 审查
2. **测试验证**：单元测试 + E2E 测试全绿
3. **构建验证**：生产构建无错误
4. **平台审核**：微信小程序提交审核，Android/iOS 打包测试
5. **灰度发布**：先小范围验证，再全量发布

## 常见问题

### 跨端兼容性问题
- **问题**：某些 API 在特定平台不可用
- **解决**：使用条件编译 + 平台检测 + 降级方案

### 性能优化
- **列表渲染**：使用 `v-for` 加 `key`，避免 `index` 作为 key
- **图片优化**：使用懒加载，压缩图片大小
- **内存管理**：及时销毁定时器、事件监听

### 调试技巧
- **H5**：使用浏览器开发者工具
- **小程序**：使用微信开发者工具
- **Android**：使用 Chrome DevTools 远程调试

## 注意事项

1. **每次改动完成后，都必须创建一个对应的 git commit，以便后续追踪和回滚。**
2. **每次改动后，都必须编写或更新相关测试，并在交互给用户前，确保所有测试和验证全部通过。**
3. **提交前必须运行**：`npm run type-check`（类型检查）和 `npm test`（单元测试）
4. **避免使用**：`setTimeout`/`setInterval` 等可能造成内存泄漏的 API，优先使用 uni-app 生命周期管理

## 相关文档

- **ADRs（移动端独立编号）**：`docs/adr/` —— `0001`-`0006`：SSE 流式传输、轻量状态管理、手动 JSON 类型映射、生物识别门控、自研安全存储、改密不吊销会话缺口；与根仓库 `docs/adr/`（`ADR-0001`+ 编号）互不相关，引用时注意区分
- **Git 工作流**：`docs/GIT_WORKFLOW.md`
- **UI 规范**：`docs/ui-spec.md`
- **技术规范**：`docs/technical-spec.md`
- **产品设计**：`docs/product-design.md`