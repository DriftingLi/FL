# ①a 真机执行记录与**未通过**结论 · issue #1159

> ⚠️ **本目录不是某个 PR 的 ①a 通过证据**，而是一份**失败记录**：它证明 #1159 的验收标准
> （「本页点 chapter / question 各一条 → 进入对应页面且正文渲染」）**在真机上不成立**，
> 且**不成立的原因不是本票修的那处**。命名用 `issue-1159`（不套 `<PR号>`）以表明它不是 PR 证据。

## 被测树与环境

- 分支 `fix/1159-personal-activity-favorite-landing`，tip `f6bd15f5`（= `origin/master` `bee21af4` + 本票 2 个提交；采样时 **PR #1147 尚未合并**，与本票零文件交集）。
- 设备：小米 `23049RAD8C`（serial `b32d8398`），HBuilderX 调试基座。
- 部署：`npm run hx:run`，机检行 `HX_RUN_DEPLOY deployed=true www=1789714037->1789715283`；深链 `cli.exe launch app-android --project <项目> --pagePath pages/profile/personal-activity`。
- 页面路径：个人动态 → 赞/收藏 → **收藏**（`likesSubTab === 'favorited'`）。

## 已确认成立（正面结论）

- 深链进页正常；页面渲染正常；**收藏列表把 `chapter` 与 `question` 两类条目都列出来了**，且 tag 文案是原始
  `target_type`（`course` / `chapter` / `question` / `topic`），可逐字核验 —— 见 `01-favorites-subtab-verified-on-screen-1529.png`。
- **夹具是账号里天然存在的数据，未新造**：chapter =「内燃机·总体构造与工作原理」(2026-09-18)，
  question =「叉车作业时可以载人行驶、站在货叉上作业。」(2026-09-17)。与 #1147 的 ①a 是同一批数据。
- 页面**部分可交互**：tab 栏（我的帖子 / 赞·收藏 / 收藏）、右上角 ⚙、「我的帖子」列表的帖子行 ——
  三者都能被 `adb input` 与**手指**触发（点帖子行成功进入 `pages/forum/forum-detail?id=20`）。

## **未**通过（决定性结论）

**该子页的收藏条目整行不可点击 —— 注入点击与手指点击一致失败。**

判据（都是量出来的）：

1. **人指复现**：在**已核对 chapter 行在屏**（`[ensure] 就位：chapter 行 y=1086  question 行 y=1763`）之后，
   录制 **7 分钟**（15:29:34–15:36:29，采样 235 帧、留帧 136）期间请维护者用**手指**依次点 chapter 行与 question 行：
   - logcat 里 **`进入页面` 行数 = 0**（该窗口内没有任何页面进入）；
   - 画面尺寸全程稳定在 ~189K（收藏列表本身），无任何跳转态 —— 对比 `01`（15:29:34）与
     `02`（15:36:26，7 分钟后）两张截图**是同一个列表**。
2. **注入侧同结论**（排除「只是手指没点到」）：对同一张 chapter 卡试过 **5 个坐标**
   （标题中心 (540,1086)、左侧 (200,1086)、tag 行 (143,1160)、首张卡 (540,860)、topic 行 (540,1989)）
   × **2 种手势**（瞬时 `input tap`；同点 120ms `input swipe`），全部无后续；
   `MotionEvent` 均干净到达（`ACTION_DOWN/UP moveCount:0`）但**无 console、无跳转、无 toast**。
   垂直 `input swipe`（540,1700→540,900）**也完全不影响该列表滚动**（目标行 y 恒为 1086）。
3. **结构性对照**：同一屏上 tab 栏、⚙、以及**同一 `<scroll-view>` 内**的「我的帖子」帖子行**都能点** ⇒
   不是「整页不可交互」，而是**该子页的那份列表**不可交互。

## 因此对 #1159 的影响（必须写实）

- 票面「根因」只写了 `onFavoriteClick` 缺 `chapter` / `question` 分支 —— **那个缺陷确实存在、本票也已修掉**
  （单元层面 URL 逐字钉死，先红后绿 + 7 变异自检全过）；
- 但**用户可见症状（点了没反应）的主因不在这里**：handler 根本没被调用到。
  ⇒ 本票的实现是**必要但不充分**，**验收标准不成立**，票**不能 close**。
- 票面「`topic` / `course` / `featured` 正常」这一句**也与现测不符**：`course` 行同样点不动。
  （维护者复核时若曾见某类可点，需以那次的具体条目与页面状态为准 —— 本条只声明本次现测。）

## 复现步骤（下一会话可直接照做）

1. `npm run hx:run`（设备 `b32d8398`）→ `cli.exe launch ... --pagePath pages/profile/personal-activity`；
2. 点「赞/收藏」→ 点「收藏」子 tab（这两个 tab **点得动**）；
3. 点任意一张收藏卡片（例如带 `chapter` 标签那张）—— **无任何反应**；
4. 对照：点「我的帖子」tab 里的帖子行 —— **能进** `pages/forum/forum-detail`。

## 待查方向（**假设，不是结论**，留给下一会话）

- 该列表比「我的帖子」列表长（7 条 vs 1 条）⇒ 怀疑与 `scroll-view` 在**内容可滚**时的命中测试/滚动接管有关；
- 该子页的列表比 `posts` 多一层 `v-if` 嵌套（`currentTab === 'likes'` → `likesSubTab === 'favorited'`）；
- `.content-scroll` 的 `flex: 1; height: 0` 在 uvue 原生渲染器下的命中区域语义；
- `@tap` 经 `ActivityTopicCard` 的 `emit` 链在滚动容器内的传递（tab 栏同样是组件 emit 链且**可以**，故此项存疑）；
- 佐证材料：本目录的 `ui-dump-favorites-list.content-desc.txt`（含每行 bounds 与中心点，可直接用于重放）。

## 产物清单

| 文件 | 说明 |
| --- | --- |
| `01-favorites-subtab-verified-on-screen-1529.png` | 15:29:34 的收藏子页（7 张卡，`chapter` / `question` 标签在屏）—— 人手点击前的已核对状态 |
| `02-same-list-after-human-taps-7min-later-1536.png` | 15:36:26（7 分钟、两次手指点击之后）—— **同一个列表，零跳转** |
| `ui-dump-favorites-list.content-desc.txt` | 该列表的 `uiautomator dump` 文案 + bounds + 中心点 |

口径：移动端 `docs/adr/0008-移动端验收门与证据.md`（①a 由 agent 出证）与 `0016`（①a 按一次分支收口）。
本次 ①a **未通过**，按该口径**不得**据此填 `## 验收证据` 的 ①a 行为「已通过」。
