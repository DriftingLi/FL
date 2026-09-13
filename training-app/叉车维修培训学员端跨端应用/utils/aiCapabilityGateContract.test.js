/**
 * AI 助手「专业版能力门」契约测试 —— 源码契约测试缝（读源文本、断言语义）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` 无法在 jest import（uvue/UTS），
 * 而能力门是**多条路径共用**的横切关注点 —— 2026-09-13 实测漏过一条：
 * 输入区的模型芯片与附件按钮都按 `proUnlocked` 渲染了，**唯独抽屉里的「自定义模型」项没过门**，
 * 未解锁用户仍能从抽屉进入该页（票 #926）。本测试把「所有专业版入口必须同门」钉下来。
 *
 * 口径：`proUnlocked == false` 时（积分兑换未上线期间的恒态）——
 *   ① 输入区：模型芯片只读、附件按钮不渲染；
 *   ② 抽屉菜单：不显示「自定义模型」；
 *   ③ 模型芯片点击：只给提示、不打开选择器；
 *   ④ 「解锁专业版」横幅用**实色**（本机型实测 background: linear-gradient(...) 静默不绘制）；
 *   ⑥ **对话设置页是同一条门**（#939 口径 A）：该页的「＋ 新增」与空态卡按门隐藏 ——
 *      `pages/ai-assistant/custom-models` 有两条入口链，只收抽屉那条（#926/#936）不算收口。
 *      （抽屉自身那两个**真机实测**踩到的坑 —— 类型名义重复导致的 `ClassCastException`、`<view>`
 *      承载文字样式 —— 由 `utils/aiChatDrawerContract.test.js` 守护。）
 */
const fs = require('fs');
const path = require('path');

const PAGE = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const CONSTS = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant-constants.uts');
const SETTINGS = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-settings.uvue');
const read = (p) => fs.readFileSync(p, 'utf8');

describe('AI 助手专业版能力门（proUnlocked）契约', () => {
  const page = read(PAGE);
  const consts = read(CONSTS);
  const settings = read(SETTINGS);

  it('① 能力门存在且默认关闭（兑换未上线期间恒为 false）', () => {
    expect(page).toMatch(/const\s+proUnlocked\s*=\s*ref<boolean>\(false\)/);
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

  it('④ 抽屉菜单：未解锁时不显示「自定义模型」（本票 #926 修的漏网路径）', () => {
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

  it('⑤ 「解锁专业版」横幅用实色，不用 gradient（本机型不绘制渐变）', () => {
    expect(page).toMatch(/class="pro-banner"/);
    // 只断言横幅自己的样式块 —— 不能全局断言（页面别处的注释里就写着 linear-gradient 这个词，
    // 全局匹配会把注释当实现，属假阳性）
    const from = page.indexOf('.pro-banner {');
    const style = page.slice(from, page.indexOf('.pro-banner-go {', from));
    expect(style).toMatch(/background-color:\s*#/);
    expect(style).not.toMatch(/gradient/);
  });

  it('⑥ 对话设置页同门（#939 口径 A）：「＋ 新增」与空态卡按门隐藏', () => {
    // 该页必须有**同一形态**的页面级门（ADR 0009 的口径：页面级 proUnlocked；#920 上线后两页一并改读后端判定）
    expect(settings).toMatch(/const\s+proUnlocked\s*=\s*ref<boolean>\(false\)/);
    // 两条漏径都在门内：新增入口、空态卡
    expect(settings).toMatch(/<text v-if="proUnlocked" class="section-add"/);
    expect(settings).toMatch(/v-if="proUnlocked && userModels\.length == 0"/);
    // 列表必须 v-else-if：若沿用 v-else，锁定时会把空态卡藏了却渲染一个空卡片
    expect(settings).toMatch(/v-else-if="userModels\.length > 0"/);
    // 门后唯一去处仍是自定义模型页（口径 A 只把入口收进同一门，不改目标页）
    expect(settings).toMatch(/url:\s*'\/pages\/ai-assistant\/custom-models'/);
  });
});
