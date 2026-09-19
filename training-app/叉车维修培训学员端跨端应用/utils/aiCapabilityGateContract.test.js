/**
 * AI 助手「专业版能力门」契约测试 —— 源码契约测试缝（读源文本、断言语义）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` 无法在 jest import（uvue/UTS），
 * 而能力门是**多条路径共用**的横切关注点 —— 2026-09-13 实测漏过一条：
 * 输入区的模型芯片与附件按钮都按 `proUnlocked` 渲染了，**唯独抽屉里的「自定义模型」项没过门**，
 * 未解锁用户仍能从抽屉进入该页（票 #926）。本测试把「所有专业版入口必须同门」钉下来。
 *
 * 口径：能力门 `proUnlocked` 的**事实源是积分商城 SKU「ai_pro」的已拥有**（#920 落地；
 * 由 `composables/useAiPro.uts` 单点适配器读出，实测无权益读面时用支出流水推导）。未解锁时——
 *   ① 输入区：模型芯片不渲染、附件按钮不渲染；
 *   ② 抽屉菜单：不显示「自定义模型」；
 *   ③ 模型芯片点击：不打开选择器（走档位入口）；
 *   ④ 「解锁专业版」横幅用**实色**（按设计：两态用实色更干净）。
 *      注：2026-09-13 真机实测已证伪早前那句「本机型不绘制 linear-gradient」的归因 ——
 *      合规语法（角度/关键字 + 恰好 2 个颜色值、无百分比停靠）是画得出来的（#937，守护见
 *      `utils/gradientSyntaxContract.test.js`）。横幅保持实色是**设计选择**，不是能力限制。
 *   ⑥ **对话设置页是同一条门**（#939 口径 A）：该页的「＋ 新增」与空态卡按门隐藏 ——
 *      `pages/ai-assistant/custom-models` 有两条入口链，只收抽屉那条（#926/#936）不算收口。
 *      **#1024 升级**：口径 A 关的是**页面入口**，不是**能力本身** —— 该页的自带 key 配置面
 *      与已保存模型的选用当时仍未过门（本测试 ⑥⑦ 补钉），且该页的门改为与 AI 助手页**同源**
 *      （`useAiPro` 单点，删掉 #936 时期那个为避开 #942 而自持的页面级 `ref(false)` 常量）。
 *   ⑦ 对话设置页的能力面（① 配置表单 / ② 已保存模型列表）在门内；未解锁时来源选择器只剩
 *      「平台模型」一项（不留「选中来源却下方空白」的死行）。
 *   ⑧ 模型选择器的三条自带 key 路径都过门：③ 管理入口 / ④ 自定义 API Key / ⑤ 已保存模型行。
 *   ⑨ 存量来源回收：未解锁时两页都把 user / custom 来源回收为平台模型并落盘 —— 只关入口会漏掉
 *      「存量已选仍在生效」（`buildBody` 会照送 `user_model_id` / 明文 `custom_api_key`）。
 *      （抽屉自身那两个**真机实测**踩到的坑 —— 类型名义重复导致的 `ClassCastException`、`<view>`
 *      承载文字样式 —— 由 `utils/aiChatDrawerContract.test.js` 守护。）
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const PAGE = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const CONSTS = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant-constants.uts');
const ADAPTER = path.join(__dirname, '..', 'composables', 'useAiPro.uts');
const SETTINGS = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-settings.uvue');
const read = (p) => readText(p);

describe('AI 助手专业版能力门（proUnlocked）契约', () => {
  const page = read(PAGE);
  const consts = read(CONSTS);
  const adapter = read(ADAPTER);
  const settings = read(SETTINGS);

  it('① 能力门由 useAiPro 单点适配器派生，且默认关闭（fail-closed）', () => {
    // #920：门不再是页面里的写死常量，而是适配器读出的事实（事实源 = 已拥有 ai_pro）
    expect(page).toMatch(/const \{[\s\S]*?proUnlocked[\s\S]*?\} = useAiPro\(\)/);
    expect(page).not.toMatch(/const\s+proUnlocked\s*=\s*ref<boolean>\(false\)/);
    expect(adapter).toMatch(/const proUnlocked = ref<boolean>\(false\)/);
  });

  it('② 输入区：模型芯片与附件按钮都按 proUnlocked 渲染，芯片在锁定时只读', () => {
    expect(page).toMatch(/:show-model-bar="proUnlocked"/);
    expect(page).toMatch(/:model-readonly="!proUnlocked"/);
    expect(page).toMatch(/:show-image-button="proUnlocked"/);
  });

  it('③ 模型芯片点击走 onModelBarClick（而不是直接打开选择器）', () => {
    expect(page).toMatch(/@model-click="onModelBarClick"/);
    const fn = page.slice(page.indexOf('function onModelBarClick'));
    expect(fn).toMatch(/if\s*\(!proUnlocked\.value\)/);
    expect(fn).toMatch(/showModelPicker\.value\s*=\s*true/);
  });

  it('④ 抽屉菜单：未解锁时不显示「自定义模型」（#926 修的漏网路径）', () => {
    // 菜单必须是 computed（受能力门影响），不能是常量直赋
    expect(page).not.toMatch(/const\s+rightMenuItems\s*=\s*RIGHT_MENU_ITEMS\s*$/m);
    expect(page).toMatch(/const\s+rightMenuItems\s*=\s*computed<MenuItem\[\]>/);
    // 且过滤条件必须是 custom-models 这个 key
    const fn = page.slice(page.indexOf('const rightMenuItems = computed'));
    expect(fn).toMatch(/filter\(/);
    expect(fn).toMatch(/m\.key\s*!=\s*'custom-models'/);
    // 常量表仍保留全量（解锁后可见），可见性由门决定 —— 两者不能同时被改掉
    expect(consts).toMatch(/'custom-models'/);
  });

  it('⑤ 「解锁专业版」横幅用实色（设计选择：两态用实色；非能力限制）', () => {
    expect(page).toMatch(/class="pro-banner"/);
    // 只断言横幅自己的样式块 —— 不能全局断言（页面别处的注释里就写着 linear-gradient 这个词，
    // 全局匹配会把注释当实现，属假阳性）
    const from = page.indexOf('.pro-banner {');
    const style = page.slice(from, page.indexOf('.pro-banner-go {', from));
    expect(style).toMatch(/background-color:\s*#/);
    expect(style).not.toMatch(/gradient/);
  });

  it('⑥ 对话设置页与 AI 助手页**同门同源**（#1024：改读 useAiPro 单点，不再自持常量）', () => {
    // #1024：该页不再是页面级 ref(false) 常量（#936 为避免与 #942 撞车才那样写，撞车前提已消失）
    expect(settings).toMatch(/const \{[\s\S]*?proUnlocked[\s\S]*?\} = useAiPro\(\)/);
    expect(settings).not.toMatch(/const\s+proUnlocked\s*=\s*ref<boolean>\(false\)/);
    // ③ 入口面（#939 口径 A）：新增入口、空态卡
    expect(settings).toMatch(/<text v-if="proUnlocked" class="section-add"/);
    expect(settings).toMatch(/v-if="proUnlocked && userModels\.length == 0"/);
    // 列表必须 v-else-if：若沿用 v-else，锁定时会把空态卡藏了却渲染一个空卡片
    expect(settings).toMatch(/v-else-if="userModels\.length > 0"/);
    // 门后唯一去处仍是自定义模型页（口径 A 只把入口收进同一门，不改目标页）
    expect(settings).toMatch(/url:\s*'\/pages\/ai-assistant\/custom-models'/);
    // 门的用法有顺序语义：先 loadData（存量设置 + 平台模型），再读门，**最后**才回收 ——
    // 在门回来之前回收会把已解锁用户的来源误清（proUnlocked 初值 false 只是 fail-closed 起点）
    const show = settings.slice(settings.indexOf('async function refreshProStateAndReclaim'));
    expect(show).toMatch(/await loadData\(\)/);
    expect(show.indexOf('await loadAiProState()')).toBeGreaterThan(show.indexOf('await loadData()'));
  });

  it('⑦ 对话设置页的能力面（#1024 ①②）：配置面与使用面都在门内，未解锁只剩「平台模型」一项', () => {
    // ① 自带 key 的**配置面**
    expect(settings).toMatch(/v-if="proUnlocked && settings\.model_source == 'custom'"/);
    // ② 自带 key 的**使用面**（维护者 2026-09-15 裁定：选用已保存的自定义模型归 pro）
    expect(settings).toMatch(
      /v-if="proUnlocked && \(settings\.model_source == 'user' \|\| settings\.model_source == 'custom'\)"/
    );
    // 来源选择器：被锁的两行整行不渲染（不留「点了却看到空段」的死行），平台模型那行不受门影响
    expect(settings).not.toMatch(/<view class="source-row" @click="onSelectSource\('user'\)">/);
    expect(settings).not.toMatch(/<view class="source-row" @click="onSelectSource\('custom'\)">/);
    expect(settings).toMatch(/<view v-if="proUnlocked" class="source-row" @click="onSelectSource\('user'\)">/);
    expect(settings).toMatch(/<view v-if="proUnlocked" class="source-row" @click="onSelectSource\('custom'\)">/);
    expect(settings).toMatch(/<view class="source-row" @click="onSelectSource\('admin'\)">/);
    // 兜底：即便有别的路径把来源置成 user / custom，选择器函数也不接受
    const sel = settings.slice(settings.indexOf('function onSelectSource'), settings.indexOf('function onSelectUserModel'));
    expect(sel).toMatch(/if\s*\(!proUnlocked\.value && source != 'admin'\) return/);
  });

  it('⑧ 模型选择器的三条自带 key 路径都过门（#1024 ③④⑤）', () => {
    // ③ 管理入口：未解锁走本页档位 sheet（与模型芯片点击同一入口），不跳目标页
    const manage = page.slice(page.indexOf('function goManageModels'), page.indexOf('function onCustomClick'));
    expect(manage).toMatch(/if\s*\(!proUnlocked\.value\)/);
    expect(manage).toMatch(/showProSheet\.value\s*=\s*true/);
    expect(manage).toMatch(/url:\s*'\/pages\/ai-assistant\/custom-models'/);
    // ④ 「自定义 API Key」不再直连弹窗
    expect(page).not.toMatch(/@custom="showCustomForm = true"/);
    expect(page).toMatch(/@custom="onCustomClick"/);
    const custom = page.slice(page.indexOf('function onCustomClick'), page.indexOf('function closeCustomForm'));
    expect(custom).toMatch(/if\s*\(!proUnlocked\.value\)/);
    expect(custom).toMatch(/showProSheet\.value\s*=\s*true/);
    expect(custom).toMatch(/showCustomForm\.value\s*=\s*true/);
    // ⑤ 已保存的自定义模型不进选择器列表（点选即 currentModelSource='user'）
    const all = page.slice(page.indexOf('const allModels = computed'), page.indexOf('const currentModelName'));
    expect(all).toMatch(/if\s*\(proUnlocked\.value\)/);
  });

  it('⑨ 存量来源回收（#1024）：未解锁时两页都把「自带 key 的来源」回收为平台模型并落盘', () => {
    // 两页都做；且都只在门状态**已知**时回收（否则会把已解锁用户的来源误清）
    const reclaim = page.slice(page.indexOf('function tryReclaimLockedSource'), page.indexOf('function applyPendingSelectedUserModel'));
    expect(reclaim).toMatch(/if\s*\(!proStateSettled\.value \|\| !modelsHydrated\.value\) return/);
    expect(reclaim).toMatch(/if\s*\(proUnlocked\.value\) return/);
    expect(reclaim).toMatch(/currentModelSource\.value = 'admin'/);
    expect(reclaim).toMatch(/persistSettings\(\)/);
    // 「刚在自定义模型页新增并选用」的信号同样指向自带 key 的模型：唯一消费点，未解锁不应用
    const apply = page.slice(page.indexOf('function applyPendingSelectedUserModel'), page.indexOf('async function refreshProState'));
    expect(apply).toMatch(/if\s*\(!proStateSettled\.value\) return/);
    expect(apply).toMatch(/removeStorageSync\(STORAGE_KEY_AI_SELECTED_USER_MODEL\)/);
    expect(apply).toMatch(/if\s*\(!proUnlocked\.value\) return/);
    const reclaim2 = settings.slice(settings.indexOf('function reclaimLockedModelSource'), settings.indexOf('async function loadData'));
    expect(reclaim2).toMatch(/if\s*\(proUnlocked\.value\) return/);
    expect(reclaim2).toMatch(/settings\.value\.model_source = 'admin'/);
    expect(reclaim2).toMatch(/saveAiChatSettings\(settings\.value\)/);
  });
});