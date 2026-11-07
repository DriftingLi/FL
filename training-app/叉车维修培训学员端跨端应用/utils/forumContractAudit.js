/**
 * forum 契约断言的**逐条分类登记**（#2332，A 口径：接线族归位、只治假绿）
 *
 * 为什么要有这张表：`utils/forumContract.test.js` 的断言全是**源码文本锁**，它到底证明了什么、
 * 没证明什么，读断言本身看不出来 —— 于是「测试把 bug 钉成规范」（#1713）就是这么长出来的：
 * 有人以为那些 `toContain` 在验证行为。`docs/agents/guards.md:37` 早已写明这类守护
 * 「不构成 ③ 门证据，但是合法的第二类」，本表把这句话**落到每一条用例上**。
 *
 * 每行三个字段：
 *   · `kind`     —— `wiring`（接线：守调用点/字段名/常量，改坏行为不红）|
 *                   `behavior-claim`（标题自称「验证行为 / 语义保留 / 逐字保留」）
 *   · `verdict`  —— `keep-wiring`（保留，显式标注为接线）|
 *                   `retitle-wiring`（改写标题不再自称行为，断言不动）|
 *                   `rewrite-behavior`（行为判据移到行为腿真跑，本文件只留调用点锁）
 *   · `note`     —— 判据落点：真判据在哪个套件，或归 ①a 发版冒烟
 *
 * 三处消费者，同一份真源：
 *   ① `utils/forumContract.test.js` 末组「断言分类登记」—— 1:1 覆盖（漏标/失效条目都红）；
 *   ② `scripts/gen-2332-assertion-audit.mjs` —— 渲染人读版 `docs/2332-断言处置表.md`；
 *   ③ 人 —— 想知道某条断言算不算证据时查这里，而不是猜。
 *
 * 改契约测试时：新增/改名/删除 `it` **必须同批改本表**，否则 ③ 门第一腿立刻红。
 */
'use strict';

/**
 * 一条用例的键是 `desc::title`（标题在本文件里有重复：`组件文件存在于 …%s.uvue` 列表页与详情页各一条，
 * 故不能只用 title）。
 */
const AUDIT = [
  { desc: "列表页组件接线契约（Q17 安置：pages/forum/components/ 显式 import）", title: "组件文件存在于 pages/forum/components/%s.uvue", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "列表页组件接线契约（Q17 安置：pages/forum/components/ 显式 import）", title: "forum.uvue 显式 import 组件 %s", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "列表页组件接线契约（Q17 安置：pages/forum/components/ 显式 import）", title: "主页面模板实际使用全部组件标签（非只 import 不用）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "详情页组件接线契约", title: "组件文件存在于 pages/forum/components/%s.uvue", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "详情页组件接线契约", title: "forum-detail.uvue 显式 import 组件 %s", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "详情页组件接线契约", title: "详情页模板实际使用全部组件标签", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "列表页两个 composable 存在且以显式 UseXxxResult 返回（error18 先例）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "列表页壳层仅接线 useTopicFeed；资源 composable 由 panel 组件自持消费（数据所有权单一）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "列表页壳层 script 不持有招聘 mock 与金币公式（下沉验证）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "详情页三个 composable 存在且以显式 UseXxxResult 返回", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "详情页壳层接线三 composable；回复点赞直读后端权威字段（本地态已删，不再本地优先）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "composer 提交经 onPosted 回调上抛列表侧变更（禁 defineExpose 反向桥接）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "举报原因 reason 归弹窗组件自持（打开即重挂载清空），提交经 submit 事件参数上抛", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "列表查询语义逐字保留：广场 general/discussion，问答 all/question", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "composable 下沉契约（列表两域 + 详情三域状态离开壳层）", title: "展示纯函数集中在 utils/forumDisplay.uts 与 forumDetailDisplay.uts，组件经 computed/局部函数包装（守护规则 S：模板禁直调 import 函数）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "列表摘要接格式轴（#1273：预览与正文同源，摘要不留第二实现）", title: "卡片走扁平 `contentFormat` prop 转发到唯一实现，不自持截断（#687：可选对象 prop 成员直读 = error18）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "列表摘要接格式轴（#1273：预览与正文同源，摘要不留第二实现）", title: "广场壳层把该帖的 `content_format` 扁平下发（不下发 = 论坛首页仍直出源串）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "列表摘要接格式轴（#1273：预览与正文同源，摘要不留第二实现）", title: "摘要层不自己解析、不自己判档 —— 只经 `utils/forumBody` 那一条格式轴口（投影与渲染同源）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "tab-bar 有备考经验 tab，点击 onSwitch(experience)", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "列表查询语义（ADR-0040）：备考经验 scope=all + is_experience=true（判据是管理端认定，不是 category），最多赞→hot、最新→created（#727 排序档）、默认→latest", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "expSort 状态与 onExpSortChange 沉在 useTopicFeed（数据所有权单一，壳层只接线）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "壳层接线排序条：仅备考经验 tab 显示，帖子流复用广场 TopicCard（variant 表达式不变）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "排序条组件为默认/最多赞/最新三 chip（原型 forum-6screens 备考经验屏）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "发帖入口（ADR-0040）：经验 tab 退为只读策展流，发布口不再产出 experience，分类行无「备考经验」", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）", title: "经验 tab 无任何发布入口（只读策展流）：搜索栏（含发帖 pill）与空态按钮按 currentTab 隐藏", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "api：featured 参数按后端契约透传（featured=true|false，空串不传=不过滤；参数名勿改）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "api：buildTopic 回显后端 DTO 字段 is_featured（勿改名）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "types：ForumTopic 携带 is_featured", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "feed：featuredFilter 态与 onFeaturedChange 沉在 useTopicFeed（数据所有权单一），加载时透传", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "壳层接线：三 Tab 显示精选筛选条（资源 Tab 不显示）；切 Tab 精选筛选回全部帖（跨 Tab 不延续，对齐 Web）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "筛选条组件：全部帖 / ★ 精选 双 chip（选项与 Web 一致），点击 emit featuredChange", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）", title: "列表标识：卡片 is_featured 由壳层扁平下发（R2 默认 false 零 diff），精选徽章文案与 Web 一致", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "api：is_experience 参数按后端契约透传（isExperience=true，空串不传=不过滤；参数名勿改）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "api：category 注释不再写「取值含 experience」的旧口径（意图只有两值）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "types：ForumTopic 携带 is_experience（#836 收尾）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "api：buildTopic 映射后端 is_experience 字段（#836 收尾）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "列表项扁平下发 is_experience（#836 收尾）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "卡片组件携带 isExperience prop 并渲染「备考经验」徽章（对齐 Web）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "feed：经验分支清空 category、置 isExperience=true（不把 is_experience 当 category 别名）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）", title: "经验 tab 空态不引导发布（只读策展流，对齐 Web）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "招聘 tab 退场契约（#705 退场部分先行；简历入口重挂留 #705 后续）", title: "tab-bar / 壳层 / feed 均不再出现 recruit 分发点", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "招聘 tab 退场契约（#705 退场部分先行；简历入口重挂留 #705 后续）", title: "招聘死件文件已删除（不留未接线的组件与 composable）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "壳层仍把头像点击接到个人动态（跳转语义的真判据归 ①a 发版冒烟）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "跳转语义的真判据在真机（①a 发版冒烟）；本条只守壳层仍指向个人动态这一调用点。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "壳层仍把资源格子接到 forum-create 资源 tab（#760 复用发布页；跳转语义归 ①a）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "跳转与发布页复用形态归 ①a；本条只守壳层仍跳 forum-create 资源 tab。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "TopicCard 广场/问答共用，变体由壳层 currentTab 下发", kind: 'behavior-claim', verdict: 'keep-wiring', note: "结构 / 编译风险区锁：守 .uvue 的写法不变（本条标题已自认是接线守护），像素判据归 ①a。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "onShow 分支仍在：非资源 tab 调 reload、资源区不拉（生命周期时序归 ①a）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "生命周期时序与「资源区不重复拉取」的观测归 ①a；本条只守 onShow 分支没被删。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "详情页头部删除/编辑/举报三态 v-if 组合逐字保留", kind: 'behavior-claim', verdict: 'keep-wiring', note: "结构 / 编译风险区锁：守 .uvue 的写法不变（本条标题已自认是接线守护），像素判据归 ①a。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "回复栏留壳层、草稿仍由页面持有（跨组件 v-model 属 uvue 编译风险区 ⇒ 走显式 prop + emit）", kind: 'behavior-claim', verdict: 'keep-wiring', note: "结构 / 编译风险区锁：守 .uvue 的写法不变（本条标题已自认是接线守护），像素判据归 ①a。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "onToggleReplyLike / onToggleLike 的调用点仍在（行为判据见 utils/forumReplyLikeBehavior.test.js）", kind: 'behavior-claim', verdict: 'rewrite-behavior', note: "行为判据已移入 utils/forumReplyLikeBehavior.test.js（真跑 useTopicDetail.uts）；本条只守调用点仍在。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "onLoad 解析与 loadDetail 调用点仍在（⑪-4 键盘监听注册/注销；时序真判据归 ①a）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "键盘监听注册/注销顺序的真判据在真机（①a）；本条守 onLoad 里的调用点没被删。" },
  { desc: "行为保持契约（源码接线侧：跳转/时序/乐观更新的真判据在行为腿与 ①a，不在本组）", title: "⑪ 回复栏改半屏浮层：常驻触发条 + 遮罩 + 贴键盘面板（接线守护，像素判据归 ①a）", kind: 'behavior-claim', verdict: 'keep-wiring', note: "结构 / 编译风险区锁：守 .uvue 的写法不变（本条标题已自认是接线守护），像素判据归 ①a。" },
  { desc: "api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）", title: "forum.uts 引入 getMapped/postMapped/requestMapped 出口，不再 import 裸 get/put", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）", title: "GET 面列表两站点经信封 getPaged（#1737 三参壳消亡），详情仍经既有 builder", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）", title: "POST 面经 postMapped；PUT/DELETE 经 requestMapped 显式 method（保请求形态不变）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）", title: "守护规则 H 清零：forum.uts 无 catch 参数 : any 注解", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）", title: "my-* 三接口经信封 getPaged 且不再吞错（ADR-0037 PR-A 首刀：#701 后「降级为空列表」纯遗留已删，reject 交页面既有 catch→toast）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "types：帖子 / 回复 / 详情三处 DTO 都带 ip_province + ip_city（空串仍是字段，不是可空）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "api：三个 builder 原样映射两字段（列表 DTO 也带，但展示口径由组件决定）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "展示口径单点在 regionLabel：市优先、市为空退到省", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "披露文案单点导出，输入区不另抄字面量（#1240 P3 起承载点是共享输入区组件）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "详情壳层扁平下发属地：帖子作者行 + 每条回复（组件不自行拼口径）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "回复署名行：属地接在相对时间之后；为空整段不渲染（分隔符也在条件节点内，不悬空）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "帖子作者行：属地与作者名 / 时间同一行；为空整段不渲染", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "发布前披露在场：两个输入区各一行（ADR-0045 明确例外，勿清）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "IP 属地契约（ADR-0045：发布那一刻的快照，市优先退省，为空整段不渲染）", title: "列表卡片不加属地（ADR-0045：卡片信息密度已高）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "api：详情接口透传 page/page_size（分页是回复列表的唯一读取形态，参数名勿改）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "api：buildTopicDetail 的回复分页走信封出口，hasMore 由 total 折算（采纳接口无三元组时落空页，不退化成「还有更多」）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "api：buildReply 映射被回复人 parent_name / parent_avatar_url（ADR-0042 一并新增，勿丢）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "types：ForumTopicDetail 携带 total / hasMore 分页元信息（page、pages 已删）；ForumReply 携带被回复人两字段", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "detail：分页态的声明仍在 useTopicDetail（接线锁；分页行为待行为套件，债务登记见 docs/2332-断言处置表.md）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "分页语义目前没有行为套件：本条只是源码声明锁，已在处置表「行为债」段登记（待行为腿补齐）。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "detail：loadDetail 仍以第 1 页替换整列（接线锁；替换语义待行为套件，债务登记见 docs/2332-断言处置表.md）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "分页语义目前没有行为套件：本条只是源码声明锁，已在处置表「行为债」段登记（待行为腿补齐）。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "detail：loadMoreReplies 仍按 hasMore 追加并去重（接线锁；追加语义待行为套件，债务登记见 docs/2332-断言处置表.md）", kind: 'behavior-claim', verdict: 'retitle-wiring', note: "分页语义目前没有行为套件：本条只是源码声明锁，已在处置表「行为债」段登记（待行为腿补齐）。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "list：有「加载更多」入口、到底显示结束态，事件经 loadMore 上抛", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "list：标题计数读 total（分页后 items.length 只是已加载条数），文案在脚本层拼接", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "壳层接线：分页态与 loadMore 事件扁平下发（页面不含分页逻辑）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "回复分页契约（ADR-0042 / #850：详情回复改分页读取 + 「加载更多」）", title: "采纳 / 取消采纳后重载第 1 页：置顶是后端事实，本地翻标记改不了位置（对齐 Web）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "DTO 构造层拆分锁（#1218：请求形态与响应构造分家）", title: "api/forum.uts 从 ./forumDto 取构造层（4 个 builder 都在 import 里，#1737 起列表壳消亡）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "DTO 构造层拆分锁（#1218：请求形态与响应构造分家）", title: "api/forum.uts 不再自带构造层（extract* / build* 的 `function` 定义一个都不留）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "DTO 构造层拆分锁（#1218：请求形态与响应构造分家）", title: "api/forumDto.uts 八个构造函数都在（extract 4 + build 4，#1737 起列表壳消亡），且零请求出口", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
  { desc: "DTO 构造层拆分锁（#1218：请求形态与响应构造分家）", title: "拆后的两个文件都在 600 行以下（拆分的**目的**，别只看现状：这条同时是回潮锁）", kind: 'wiring', verdict: 'keep-wiring', note: "接线锁：守调用点 / 字段名 / 常量；改坏行为不会红（guards.md 第二类），不构成 ③ 门证据。" },
];

/**
 * 从契约测试源码里抽 `describe` / `it` 的**源码形态**（`it.each` 的标题保留 `%s`，不展开）。
 *
 * 为什么按源码而不是按运行时标题：登记表要能 1:1 覆盖**声明处**，`it.each` 展开出的 N 个运行时用例
 * 共用一条声明、共用一条处置 —— 按运行时标题登记会让表随数据规模膨胀，也就没人维护了。
 *
 * 边界（写实）：只认 `it('…'` / `it.each(… )('…'` 两种形态；本仓契约测试没有第三种写法，
 * 出现第三种会**不被抽出** ⇒ 登记表「每条都在」那条会通过、而新用例其实漏标 —— 故另有一条读数锁
 * 钉住声明总数（80）与 `it.each` 条数（4）。改写法时先改这里的正则。
 */
const DESC_RE = /^[ \t]*describe\(\s*'((?:[^'\\]|\\.)*)'/gm;
const IT_RE = /^[ \t]*it(\.[a-z]+\([^)]*\))?\(\s*'((?:[^'\\]|\\.)*)'/gm;

/**
 * @param {string} src 契约测试源码（调用方负责 EOL 归一，见 utils/utsHarness.js 的 readText）
 * @returns {{line:number, kind:'it'|'each', desc:string, title:string}[]}
 */
function extractCases(src) {
  const descs = [];
  let m;
  DESC_RE.lastIndex = 0;
  while ((m = DESC_RE.exec(src)) !== null) descs.push({ idx: m.index, title: m[1] });
  const out = [];
  IT_RE.lastIndex = 0;
  while ((m = IT_RE.exec(src)) !== null) {
    const owner = descs.filter((d) => d.idx < m.index).pop();
    out.push({
      line: src.slice(0, m.index).split('\n').length,
      kind: m[1] ? 'each' : 'it',
      desc: owner ? owner.title : '',
      title: m[2],
    });
  }
  return out;
}

module.exports = { AUDIT, extractCases };
