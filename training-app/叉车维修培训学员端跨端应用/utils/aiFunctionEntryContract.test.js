/**
 * AI 助手「功能入口常驻 + 专项通道口径」契约（#998）—— 源码契约测试缝（读源文本、断言形态）。
 *
 * 本文件锁四件**写错不报错、只会静默坏**的事：
 *
 * ① **宫格必须在滚动容器之外**：这是「滑动时不丢失」的**结构性保证**。刻意不用 `position: sticky`
 *    —— 该属性在整个移动端项目里 **0 处先例**，uvue 是否支持亦无文档，写错会静默失效（不报错、
 *    只是滑动时入口消失）。把宫格放进 `scroll-view` 里就等于回到 #998 的原缺陷形态。
 * ② **不得残留悬浮入口 / 半屏 sheet**：改形态前的 FAB + sheet 已删除；残留会变成「同一功能两个入口」。
 * ③ **`^` 收起/展开**：默认展开、不跨会话记忆；收起态只影响头部自身，不碰滚动。
 * ④ **专项通道不吃「自定义模型」**：带 `featureKey` 的一轮由后端按管理端单绑定解析模型
 *    （`ai_config_service.go:494` 明确「忽略选择子中的模型来源字段（防绕过）」）⇒ 客户端既不该被
 *    「请先配置自定义模型」的前置校验拦住，也不该把 `custom_*` 字段送上去。（维护者 2026-09-15 口径）
 *    **落点已随形态变更搬家（#1040 / ADR-0009 修订 ②）**：专项通道不再是「通用页带键发一轮」，
 *    而是独立的参数化专项页 `pages/ai-assistant/ai-feature.uvue` ⇒ 第 ④ 组断言的是那一页。
 *
 * 设计沿用本仓既有守护形态（见 utils/aiAssistantScrollContract.test.js、aiFeatureEntryWiringContract.test.js）：
 * 先对**注入违规**的变形样本断言检测有效（防空跑假绿），再对真实文件断言合规。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE = readText(path.join(ROOT, 'pages/ai-assistant/ai-assistant.uvue'));
const FEATURE_PAGE = readText(path.join(ROOT, 'pages/ai-assistant/ai-feature.uvue'));
const SHEET_PATH = path.join(ROOT, 'components/ai-chat/ai-chat-function-sheet.uvue');

/** 去掉模板/JS/CSS 注释 —— 「注释里提到某标识」不等于「代码里用了它」 */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 取某个函数声明起（含）到其大括号配平处的函数体原文 */
function fnBody(src, decl) {
  const start = src.indexOf(decl);
  if (start < 0) throw new Error(`找不到函数声明：${decl}`);
  const open = src.indexOf('{', start);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  throw new Error(`函数大括号未配平：${decl}`);
}

/** 宫格是否在滚动容器之外（「滑动不丢失」的结构性判据） */
function gridOutsideScroll(src) {
  const grid = src.indexOf('class="function-grid"');
  const scroll = src.indexOf('<scroll-view');
  const main = src.indexOf('class="main"');
  return grid >= 0 && scroll >= 0 && main >= 0 && grid < main && grid < scroll;
}

/** 头部收起态相关的三要素（默认值 / 切换 / 绑定）是否齐备 */
function headerToggleWired(src) {
  return (
    /const functionHeaderCollapsed = ref<boolean>\(false\)/.test(src) &&
    /function toggleFunctionHeader\(\) : void \{\s*functionHeaderCollapsed\.value = !functionHeaderCollapsed\.value/.test(src) &&
    /class="func-head-handle" @click="toggleFunctionHeader"/.test(src) &&
    /v-if="!functionHeaderCollapsed"/.test(src)
  );
}

describe('AI 助手「功能入口常驻」契约（#998）', () => {
  describe('① 宫格在滚动容器之外（滑动不丢失的结构性保证）', () => {
    it('function-grid 出现在 .main / scroll-view 之前', () => {
      expect(gridOutsideScroll(PAGE)).toBe(true);
    });

    it('把宫格塞回 scroll-view 内的变形样本会被检出（防检测器空跑假绿）', () => {
      const gridTag = PAGE.match(/[ \t]*<view v-if="!functionHeaderCollapsed" class="function-grid">[\s\S]*?<\/view>/);
      expect(gridTag).not.toBeNull();
      // 把宫格整块搬到 scroll-view 之后 ⇒ 顺序判据必须变红
      const moved = PAGE.replace(gridTag[0], '') + gridTag[0];
      expect(gridOutsideScroll(moved)).toBe(false);
    });

    it('头部容器不得依赖 position 定位（sticky 本项目零先例、uvue 无文档）', () => {
      const head = PAGE.match(/\.func-head\s*\{[\s\S]*?\}/);
      expect(head).not.toBeNull();
      expect(head[0]).not.toMatch(/position\s*:/);
    });
  });

  describe('② 不残留悬浮入口与半屏 sheet（同一功能两个入口 = 缺陷）', () => {
    it('页面无 func-fab / AiChatFunctionSheet 残留', () => {
      const code = stripCodeComments(PAGE);
      expect(code).not.toMatch(/func-fab/);
      expect(code).not.toMatch(/AiChatFunctionSheet/);
    });

    it('旧的 sheet 组件文件已删除', () => {
      expect(fs.existsSync(SHEET_PATH)).toBe(false);
    });

    it('把 func-fab 加回来的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = stripCodeComments(PAGE).replace(
        '<view class="main">',
        '<view class="func-fab">功能</view>\n\t\t<view class="main">'
      );
      expect(broken).toMatch(/func-fab/);
    });
  });

  describe('③ `^` 收起 / 展开（默认展开、不记忆）', () => {
    it('默认展开：functionHeaderCollapsed 初值为 false', () => {
      expect(PAGE).toMatch(/const functionHeaderCollapsed = ref<boolean>\(false\)/);
    });

    it('切换函数、点击绑定、宫格的 v-if 三处齐备', () => {
      expect(headerToggleWired(PAGE)).toBe(true);
    });

    it('默认收起 / 未绑定 v-if 的变形样本会被检出（防检测器空跑假绿）', () => {
      // 必须锚定**完整声明**：页面里还有别的 `ref<boolean>(false)`（侧栏、thinking 等），
      // 用裸 `ref<boolean>(false)` 做 replace 会改到别处、样本失真（本文件自测踩过）。
      const defaultCollapsed = PAGE.replace(
        'const functionHeaderCollapsed = ref<boolean>(false)',
        'const functionHeaderCollapsed = ref<boolean>(true)'
      );
      expect(headerToggleWired(defaultCollapsed)).toBe(false);
      expect(headerToggleWired(PAGE.replace(' v-if="!functionHeaderCollapsed"', ''))).toBe(false);
    });

    it('箭头随收起态切换（收起显示 v）', () => {
      expect(PAGE).toMatch(/\{\{ functionHeaderCollapsed \? 'v' : '\^' \}\}/);
    });
  });

  describe('④ 专项通道不吃「自定义模型」能力（#1040 形态变更后落在专项页）', () => {
    // 形态变更（ADR-0009 2026-09-15 修订 ②）：专项通道不再是「通用页带键发一轮」，
    // 而是独立的参数化专项页 ⇒ 本组断言搬到那一页，且判据更强：专项页**压根不加载模型列表**，
    // 请求体里没有 model_source / custom_* 可送（旧的「早退」写法要求读代码的人相信它真的早退）。
    it('专项页请求体只送 session_id + 历史 + 本轮键', () => {
      const fn = fnBody(FEATURE_PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).toMatch(/params\['feature_key'\] = featureKey/);
      expect(fn).not.toMatch(/model_source|custom_api_key|config_id|user_model_id/);
    });

    it('专项页不做「请先配置自定义模型」前置校验（它没有可自选的模型）', () => {
      const fn = fnBody(FEATURE_PAGE, 'function onInputSend');
      expect(fn).not.toMatch(/请先配置自定义模型/);
      expect(fn).not.toMatch(/暂无可用的 AI 模型/);
    });

    it('通用页的该校验不再按键分流（通用页恒无键）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/if \(currentModelSource\.value == 'custom' && customModel\.value\.trim\(\)\.length == 0\)/);
      expect(fn).not.toMatch(/featureKey\.length == 0/);
    });

    it('「键只属于那一轮」的旧不变量改由**结构**保证（#999 的失效形态不可表达）', () => {
      // #999 真机实测的失效形态（先清后取 ⇒ 键恒为空）以「存在一个本轮键状态」为前提。
      // 现在：通用页无键状态、专项页的键是页面常量 ⇒ 该缺陷类写不出来，而不是「小心维护」。
      expect(stripCodeComments(PAGE)).not.toMatch(/pendingFeatureKey/);
      expect(FEATURE_PAGE).toMatch(/const currentFeatureKey = ref<string>\(''\)/);
      // 专项页的键不经过任何「发送入口赋值」的路径
      expect(fnBody(FEATURE_PAGE, 'function onInputSend')).not.toMatch(/currentFeatureKey\.value\s*=/);
    });

    it('去掉 featureKey 字段的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = FEATURE_PAGE.replace("params['feature_key'] = featureKey", '');
      const fn = fnBody(broken, 'const buildBody : ChatBodyBuilder');
      expect(fn).not.toMatch(/feature_key/);
    });
  });
});
