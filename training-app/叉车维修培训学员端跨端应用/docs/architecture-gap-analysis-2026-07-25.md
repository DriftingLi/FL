# 叉车维修培训学员端跨端应用 —— 架构评审与对照分析报告（Gap Analysis）

> 评审人：高见远（架构师）｜对象：设计文档 v1.0（2026-07-24，19 页）vs 现有工作空间源码
> 方法：已逐一读取 `manifest.json`、`App.uvue`、`main.uts`、`pages.json`、`stores/*`、`api/*`、`types/*`、`constants/*`、`config/*`、`utils/*` 及全部 7 个页面、2 个组件，结论均基于实际代码。
> 存档：2026-07-25，由主理人齐活林归档，供后续标准 SOP 阶段引用。

---

## 1. 总体结论

**现状是一套「配置驱动的脚手架」：认证链路 / HTTP 层 / 仪表盘 / 课程列表 / 个人中心 / 考试中心壳已较扎实，但与设计文档存在「根本性技术栈路线分歧」——文档假设标准 uni-app（Vue3 + Pinia + `.vue` + uview-plus），而代码实际是 uni-app x（UTS + `.uts`/`.uvue` + 自定义响应式单例 + 自绘组件）。文档要求的 16+ 组件中仅 2 个存在，核心业务页（course-detail / chapter-view / question-bank / mock-exam / wrong-questions / ai-assistant）全部缺失，状态管理、图表、媒体预览、学习计时、离线缓存均未落地。最大风险是「在错误的基座上推进」——路线必须先拍板。**

---

## 2. 技术栈路线分歧（重点）

### 2.1 本质差异（已代码佐证）

| 维度 | 设计文档假设（标准 uni-app） | 现有代码（uni-app x） | 证据 |
|---|---|---|---|
| 语言/渲染 | Vue3 + `.vue`，JS 引擎（V8/JSCore）运行 | UTS 语言 + `.uvue` 原生渲染，编译为 Kotlin/Swift/ArkTS | `manifest.json` 含 `"uni-app-x": {...}` 块、`"vueVersion":"3"`；页面用 `<script setup lang="uts">` |
| 状态管理 | **Pinia** | 自定义 Vue3 `ref`/`computed` 模块级单例 | `stores/auth.uts` 注释「无需 pinia」；`stores/index.uts` 仅 re-export，无 Pinia 引入；仓库**无 package.json**（仅空 `package-lock.json`） |
| UI 库 | uni-ui + **uview-plus** | 自绘组件 + `App.uvue` 全局 CSS + `uni.scss` 变量 | 无任何 uview-plus/uni-ui 引用，无 npm 依赖声明 |
| 图表 | ucharts / echarts-for-uniapp | 无图表库（统计仅加载未可视化） | — |
| 媒体/MD | uni.createVideoContext / marked+highlight.js | 无 VideoPlayer / DocumentViewer / PptViewer / ImageViewer | — |
| 入口 | `main.ts` + `createApp` | `main.uts` + `createSSRApp`（Vue3 API 子集） | `main.uts` |

### 2.2 核心影响
- **Pinia 在 uni-app x 原生运行时不适用**（Pinia 依赖 Web 端完整 Vue3 运行时，x 的 `vue` 是其子集实现）。文档的「状态管理 = Pinia」无法直接落地——代码已用自定义单例绕开，这是 x 下的正确做法。
- **uview-plus / uni-ui 在 x 原生端大概率无法开箱即用**（它们是面向 JS 版 uni-app 的 Vue 组件库），文档 UI 选型存疑。
- **echarts / marked 是 JS 库**，在 x 的 Android/iOS/Harmony 原生端不可用，需替代方案（x 版图表 / webview 内嵌 / 自绘 / 砍功能）。
- 现有代码已深度使用 x 特性：`UTSJSONObject` 手动逐字段解析、大量 `#ifdef WEB` 多端分支、`Kotlin ClassCastException` 防御、`#ifndef WEB` 局域网 baseURL——说明团队已为 x 投入适配成本。

### 2.3 两条路线利弊

| 路线 | 说明 | 利 | 弊 |
|---|---|---|---|
| **A. 沿用 uni-app x（推荐，对齐现状）** | 以 x 为基座，修订文档：去 Pinia→自定义单例、去 uview-plus→自绘、图表/MD/视频换 x 兼容方案 | 原生性能、真原生 App；**现有代码几乎全部复用**；多端（含 App/Harmony）目标可达 | 生态较小，JS 库需重实现或 webview 兜底；文档需大改 |
| **B. 回退标准 uni-app（对齐文档）** | 把 `.uts`/`.uvue` 重写为 `.vue` + Pinia + uni-ui/uview-plus | 完整文档栈可落地；Pinia/uni-ui/uview-plus/echarts/marked 生态齐备 | **全部现有代码需重写**（UTS 方言如手动 JSON 解析、`#ifdef` 无处不在）；丢失原生性能；工作量大、风险高 |

### 2.4 推荐路线
**推荐路线 A（沿用 uni-app x）**：现有脚手架已成型且 x 化深度足够，回退成本远高于前进成本；文档应修订以匹配 x 现实。

### 2.5 🔴 需向用户确认的关键决策点（决策开关）
1. **平台路线总开关**：确认是 **uni-app x（现状）** 还是 **回退标准 uni-app（文档假设）**？这是后续一切的前提。
2. **状态管理**：若走 x → 确认沿用现有「自定义 reactive 单例」并规范化（补 user/course store），还是引入 x 兼容轻量 store 封装。
3. **UI 组件库**：若走 x → 确认自绘组件体系（现状）还是寻找 x 版组件库。
4. **图表 / Markdown / 视频**：确认替代方案（ucharts-x / webview 内嵌 / 自绘 / 本期砍掉）。
5. **后端 API 契约**：现有接口带 mock 回退、字段约定散落在代码注释（如 `/auth/login` 返回平铺 user、`/student/profile` 返回嵌套结构），需与后端对齐正式契约。

---

## 3. 模块完成度矩阵（文档要求 vs 现状）

> 图例：✅ 已实现 ｜ 🟡 部分 ｜ ❌ 缺失 ｜（注：部分项含「硬编码/占位/无交互」等限制）

### 3.1 页面（学员端）
| 文档要求页 | 现状 | 状态 | 说明 |
|---|---|---|---|
| login | `pages/login/login.uvue` | ✅ | 表单/校验/登录跳转完整 |
| register | `pages/register/register.uvue` | ✅ | 完整，含确认密码校验 |
| dashboard | `pages/dashboard/dashboard.uvue` | 🟡 | profile 经 API+mock；但「最近学习」列表硬编码、九宫格内联未复用组件 |
| course-list | `pages/courses/courses.uvue` | 🟡 | 列表渲染在，但**数据硬编码、点击无反应**（未接 course API） |
| course-detail | — | ❌ | 未实现 |
| chapter-view | — | ❌ | 未实现 |
| exam（答题） | — | ❌ | 未实现 |
| question-bank | （GRID 占位 `available:false`） | ❌ | 未实现 |
| mock-exam | （GRID 占位 `available:false`） | ❌ | 未实现 |
| level-exam | `pages/exam/exam.uvue`（仅「考试中心」壳） | 🟡 | 列表+统计硬编码，**点击无反应**，非真实答题 |
| wrong-questions | （GRID 占位 `available:false`） | ❌ | 未实现 |
| ai-assistant | （GRID **已注释**） | ❌ | 未实现 |
| profile | `pages/profile/profile.uvue` | 🟡 | stats 硬编码，logout 已接 |
| index/launch（入口） | `pages/index/index.uvue` | ✅ | 文档未列，作为登录态分发入口存在 |

### 3.2 组件（文档 16+ 个）
| 文档组件 | 现状 | 状态 |
|---|---|---|
| TabBarLayout | `components/tab-bar/tab-bar.uvue` | 🟡 自定义 tab-bar，但用 `redirectTo` 切页（状态丢失）、选中色 `#2979ff`≠文档 `#0EA5E9` |
| grid-card | `components/grid-card/grid-card.uvue` | ✅ 但 dashboard 未复用（内联 HTML） |
| AppNavbar | — | ❌ 各页自绘 status-bar+header |
| CourseCard | — | ❌ courses.uvue 内联 |
| EmptyState / LoadingMask / ProgressBar / QuickCard / ChapterNav / VideoPlayer / DocumentViewer / PptViewer / ImageViewer / StudyTimer / QuestionCard / AnswerSheet / ExamResult / ScoreCircle | — | ❌ 全部缺失 |

### 3.3 Pinia Store / Composables（文档）
| 文档 | 现状 | 状态 |
|---|---|---|
| `stores/auth.ts` | `stores/auth.uts`（自定义单例，等价能力） | ✅ |
| `stores/user.ts` | 缺失（student.uts 提供 API，页面直接调，未抽 store） | ❌ |
| `stores/course.ts` | 缺失 | ❌ |
| `composables/` (useAuth/useCourse/useExam/useStudyTimer) | **无目录** | ❌ |

### 3.4 API 层（文档）
| 文档 | 现状 | 状态 |
|---|---|---|
| http 封装（uni.request） | `api/request.uts` | ✅ 完整（Authorization / X-Silent / 401→登录） |
| auth API | `api/auth.uts` | ✅ login/register/logout/me + mock |
| student（档案/统计/记录） | `api/student.uts` | ✅（覆盖 user store 的 profile/records，文档未单列） |
| course API | 缺失 | ❌ |
| exam API | 缺失 | ❌ |
| question API | 缺失 | ❌ |

### 3.5 数据模型（文档）
| 文档模型 | 现状 | 状态 |
|---|---|---|
| UserInfo | `types/index.uts` 有 | ✅ |
| Course / Chapter / ChapterFile | 缺失 | ❌ |
| Question（含题型枚举 single_choice/multi_choice/true_false/short_answer/fault_image） | 缺失 | ❌ |
| ExamResult | 缺失 | ❌ |
| StudyStats / CourseProgress | `StudyStats`、`CourseProgressItem` 有（doc 未精确定义，语义相近） | 🟡 |
| 分类 CATEGORY_01~04 / 等级 beginner~expert | 缺失（courses.uvue 硬编码「基础操作」等分类，无枚举） | ❌ |
| 其它已有类型 | LoginParams/LoginResult/RegisterParams/StudentProfile/StudyRecord/StudyRecordsResult/GridItem/AuthStore/ApiResponse | ✅ |

---

## 4. 架构层面对照

### 4.1 分层架构
| 文档分层 | 现状 | 符合度 |
|---|---|---|
| 表现层（H5/小程序/App） | pages `.uvue` | ✅ 四端目标保留（`config/env.uts` `#ifdef WEB`） |
| 业务层（Pages+Components+Layouts） | pages + 2 组件，无 layouts/ | 🟡 组件严重缺失 |
| 逻辑层（Pinia + Composables） | 自定义 reactive 单例 + 页面内联逻辑，无 composables | 🟡 欠结构化 |
| 数据层（uni.storage + HTTP） | `storage.uts` + `request.uts` | ✅ |

### 4.2 目录结构
- **顶级目录一致**（pages/components/api/stores/types/constants/config/utils 均存在），方向对。
- **差异在粒度**：文档要求 `pages/(auth/student/tutor/admin)`、`components/(common/student/exam)` 子等；现状为**扁平结构**（无子目录）；`composables/` 缺失；`types` 为单文件 `index.uts`（非按领域拆分）；`stores` 仅 auth+index。

### 4.3 状态管理
- 文档：**Pinia**。现状：**自定义 `ref`/`computed` 模块级单例**（`useAuthStore()` 返回共享状态）。见 §2，x 下此为合理替代，但需规范化（补 user/course store，抽 composables）。

### 4.4 导航守卫
- 文档：**App.vue 登录鉴权 + 角色前缀校验**（auth/student/tutor/admin 路由）。
- 现状：**🟡 部分**——`App.uvue` `onLaunch` 调 `validateToken()`（无角色校验）；`index.uvue` 按 `isLoggedIn` 分发到 dashboard/login；各页 `onLoad` 各自 `restoreFromStorage()`。**无集中式全局路由守卫、无角色前缀路由**。
- TabBar：文档要求选中色 `#0EA5E9`；现状 `tab-bar.uvue` 与 `uni.scss` 均为 **`#2979ff`** ❌ 不一致。

---

## 5. 关键风险与待明确事项（阻塞项）

| # | 风险 | 等级 | 说明 / 待明确 |
|---|---|---|---|
| R1 | **技术栈路线未对齐** | 🔴 致命 | doc 假设标准 uni-app+Pinia+uview-plus，代码是 uni-app x。必须先与用户拍板（见 §2.5），否则后续全错。 |
| R2 | Pinia 不可用 | 🔴 高 | x 下无法用 Pinia；doc 状态设计需重写，补 user/course store。 |
| R3 | UI 库不兼容 | 🟠 高 | uview-plus/uni-ui 在 x 原生端大概率不可用；需补全 15+ 自绘组件。 |
| R4 | 图表/MD/视频库缺失+选型存疑 | 🟠 高 | echarts/marked 为 JS 库，x 原生不可用；替代方案待定。 |
| R5 | 后端 API 契约未确认 | 🟠 高 | 现有 mock 回退 + 字段约定散落注释，需正式契约对齐（含 `/auth/login` 平铺、`/student/profile` 嵌套等）。 |
| R6 | 核心业务零覆盖 | 🔴 高 | course-detail/chapter-view/question-bank/mock-exam/wrong-questions/ai-assistant 全缺；exam 仅为壳。是工作量主体。 |
| R7 | 学习计时/进度上报/断点续练未实现 | 🟠 中 | doc 要求每秒计时、60s 增量上报、onHide/onUnload 上报、后端持久化 current_index+answers_state——现状无 StudyTimer/useStudyTimer。 |
| R8 | 离线缓存未实现 | 🟠 中 | doc 要求课程列表 1h 缓存、章节内容按 `courseId_chapterId` 缓存；现状仅 token/user 持久化。 |
| R9 | 导航/TabBar 实现瑕疵 | 🟡 中 | `redirectTo` 切 tab（页面销毁、状态丢失）；应 `switchTab`+pages.json tabBar 或 keep-alive；选中色不一致（#2979ff vs #0EA5E9）；无全局守卫/角色路由。 |
| R10 | 主题色不一致 | 🟡 低 | doc `#0EA5E9` vs 现状 `#2979ff`（uni.scss `$primary-color` 同）。需统一。 |
| R11 | 多端视觉/交互验证缺失 | 🟡 低 | 仅 H5 可快速预览，小程序/App/Harmony 未联调（#ifdef 分支待真机验证）。 |

---

## 6. 建议的下一步路线（分阶段、可落地优先序）

**Phase 0 — 决策对齐（1–2 天，阻塞后续）**
- 与用户确认 §2.5 的 5 个决策点；修订设计文档至 v1.1 对齐 uni-app x 现实（去 Pinia/uview-plus/echarts，定替代方案）。

**Phase 1 — 基座补齐（3–5 天）**
- 统一主题色（#0EA5E9 或确认采用 #2979ff 并改 doc）。
- 规范化状态层：保留自定义单例，补 `user`/`course` store，建立 `composables/`(useAuth/useCourse/useExam/useStudyTimer)。
- 统一导航守卫：登录校验集中化 + 角色路由；TabBar 改 `switchTab`+pages.json 或 keep-alive。

**Phase 2 — 课程学习链路（1–2 周，核心）**
- `types` 补 Course/Chapter/ChapterFile/Question/ExamResult + 枚举（题型/分类/等级）。
- `api` 补 course.ts（含 mock）。
- 页面补 course-detail / chapter-view（视频/文档/PPT/图片预览 → 需定预览方案）。
- StudyTimer 学习计时 + 进度上报 + 断点续练；离线缓存策略（uni.storage）。

**Phase 3 — 考试系统（1–2 周）**
- 组件：QuestionCard/AnswerSheet/ExamResult/ScoreCircle/ProgressBar/LoadingMask/EmptyState 等。
- 页面：question-bank / mock-exam / level-exam（替换现有壳）/ wrong-questions。
- 考试逻辑：答题状态、倒计时、5 分钟警告自动交卷。

**Phase 4 — 增强与收尾（1 周）**
- ai-assistant（确认后端/大模型接入方案）。
- 统计可视化（替代 echarts）。
- tutor/*、admin/*（若本期范围包含）。
- 多端联调（H5/小程序/App）+ 主题统一 + 打磨。

---

### 附：源码核实要点（供后续 SOP 引用）
- 技术栈判定铁证：`manifest.json` 的 `"uni-app-x"` 块 + 全量 `.uts`/`.uvue` + `lang="uts"`。
- 无 `package.json`（仅空 `package-lock.json`）→ 无 Pinia/uview-plus/uni-ui/echarts 等 npm 依赖。
- 进度看板机制：`constants/app.uts` 的 `GRID_ITEMS` 用 `available:true/false` 标记功能就绪（question-bank/mock-exam/wrong-questions/ai-assistant = false，ai-assistant 已注释），印证「配置驱动脚手架」定位。
- 已落地扎实部分：认证链路（登录/注册/token 校验/持久化/401）、HTTP 层（request.uts）、仪表盘、课程列表、个人中心（均带 mock 回退）、考试中心壳。

> 本报告仅做分析，未修改任何文件。所有结论基于实际读取的源码；无法从代码确认的项（如后端契约细节、x 对具体 JS 库的兼容边界）已标注「待核实/待确认」。
