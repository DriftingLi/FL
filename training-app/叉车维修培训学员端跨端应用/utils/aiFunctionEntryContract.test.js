/**
 * AI 助手「功能入口常驻 + 专项通道口径」契约（#998）—— 源码契约测试缝（读源文本、断言形态）。
 *
 * 本文件锁四件**写错不报错、只会静默坏**的事：
 *
 * ① **宫格必须在滚动容器之外**：这是「滑动时不丢失」的**结构性保证**。刻意不用 position:sticky
 *    —— 该属性在整个移动端项目里 **0 处先例**，uvue 是否支持亦无文档，写错会静默失效（不报错、
 *    只是滑动时入口消失）。把宫格放进 scroll-view 里就等于回到 #998 的原缺陷形态。
 * ② **不得残留悬浮入口 / 半屏 sheet**：改形态前的 FAB + sheet 已删除；残留会变成「同一功能两个入口」。
 * ③ **^ 收起/展开**：默认展开、不跨会话记忆；收起态只影响头部自身，不碰滚动。
 * ④ **专项通道不吃「自定义模型」**：带 feature_key 的一轮由后端按管理端单绑定解析模型
 *    （config.go:498 明确「忽略选择子中的模型来源字段（防绕过）」）⇒ 客户端既不该被
 *    「请先配置自定义模型」的前置校验拦住，也不该把 custom_* 字段送上去。（维护者 2026-09-15 口径）
 *    **落点已随形态变更搬家两次**：#1040 / ADR-0009 修订 ② 先搬到独立专项页；
 *    **#2593 / ADR-0009 2026-11-16 修订 ②** 又把专项页并成同一页的一个 scope（activeScope）
 *    ⇒ 第 ④ 组断言的是同一份 buildBody 的 scoped 分支，而不是第二个页面。
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
const PAGES_JSON = readText(path.join(ROOT, 'pages.json'));
const RETIRED_PAGE_FILE = path.join(ROOT, 'pages/ai-assistant/ai-feature.uvue');
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
  if (start < 0) throw new Error('找不到函数声明：' + decl);
  const open = src.indexOf('{', start);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  throw new Error('函数大括号未配平：' + decl);
}

/** 宫格是否在滚动容器之外（「滑动不丢失」的结构性判据） */
function gridOutsideScroll(src) {
  const grid = src.indexOf('class="function-grid"');
  const scroll = src.indexOf('<scroll-view');
  const main = src.indexOf('class="main"');
  return grid >= 0 && scroll >= 0 && main >= 0 && grid < main && grid < scroll;
}

const GRID_TAG_RE = /[ \t]*<view v-if="[^"]*" class="function-grid">[\s\S]*?<\/view>/;

/** 宫格的 v-if：非专项 scope 且未收起（#2593 起专项 scope 收起宫格、改显 chip） */
const HEAD_VIF = ' v-if="!isScoped && !functionHeaderCollapsed"';

/** 头部收起态相关的四要素（默认值 / 切换 / 绑定 / 宫格的 v-if）是否齐备 */
function headerToggleWired(src) {
  return (
    /const functionHeaderCollapsed = ref<boolean>\(false\)/.test(src) &&
    /function toggleFunctionHeader\(\) : void \{\s*functionHeaderCollapsed\.value = !functionHeaderCollapsed\.value/.test(src) &&
    /class="func-head-handle" @click="toggleFunctionHeader"/.test(src) &&
    src.includes(HEAD_VIF)
  );
}

/**
 * scoped 分支是否「先写键、再在写任何模型字段之前 return」。
 *
 * 三条一起判：键在函数里、early return 在 model_source 之前、early return 之前的段落不含模型字段
 * （只删 return 或把它挪到 model_source 之后，任一处都会让本函数返回 false）。
 */
function scopedBranchReturnsEarly(src) {
  const fn = fnBody(src, 'const buildBody : ChatBodyBuilder');
  const keyIdx = fn.indexOf("params['feature_key'] = featureKey");
  const retIdx = fn.search(/if \(featureKey\.length > 0\) \{\s*return params/);
  const modelIdx = fn.indexOf("params['model_source']");
  if (keyIdx < 0 || retIdx < 0 || modelIdx < 0) return false;
  const scopedSeg = fn.slice(0, retIdx);
  if (/model_source|custom_api_key|config_id|user_model_id/.test(scopedSeg)) return false;
  return retIdx > keyIdx && retIdx < modelIdx;
}

describe('AI 助手「功能入口常驻」契约（#998）', () => {
  describe('① 宫格在滚动容器之外（滑动不丢失的结构性保证）', () => {
    it('function-grid 出现在 .main / scroll-view 之前', () => {
      expect(gridOutsideScroll(PAGE)).toBe(true);
    });

    it('把宫格塞回 scroll-view 内的变形样本会被检出（防检测器空跑假绿）', () => {
      const gridTag = PAGE.match(GRID_TAG_RE);
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

    it('专项页已退役：pages.json 不再注册、文件已删（#2593）', () => {
      expect(PAGES_JSON).not.toMatch(/pages\/ai-assistant\/ai-feature/);
      expect(fs.existsSync(RETIRED_PAGE_FILE)).toBe(false);
    });

    it('把 func-fab 加回来的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = stripCodeComments(PAGE).replace(
        '<view class="main">',
        '<view class="func-fab">功能</view>\n\t\t<view class="main">'
      );
      expect(broken).toMatch(/func-fab/);
    });

    it('把退役页路径写回 pages.json 的变形样本会被检出', () => {
      const broken = PAGES_JSON.replace('"pages": [', '"pages": [\n\t\t{ "path": "pages/ai-assistant/ai-feature" },');
      expect(broken).toMatch(/pages\/ai-assistant\/ai-feature/);
    });
  });

  describe('③ ^ 收起 / 展开（默认展开、不记忆）', () => {
    it('默认展开：functionHeaderCollapsed 初值为 false', () => {
      expect(PAGE).toMatch(/const functionHeaderCollapsed = ref<boolean>\(false\)/);
    });

    it('切换函数、点击绑定、宫格的 v-if 三处齐备（宫格额外要求非专项 scope）', () => {
      expect(headerToggleWired(PAGE)).toBe(true);
    });

    it('默认收起 / 未绑定 v-if 的变形样本会被检出（防检测器空跑假绿）', () => {
      // 必须锚定**完整声明**：页面里还有别的 ref<boolean>(false)（侧栏、thinking 等），
      // 用裸 ref<boolean>(false) 做 replace 会改到别处、样本失真（本文件自测踩过）。
      const defaultCollapsed = PAGE.replace(
        'const functionHeaderCollapsed = ref<boolean>(false)',
        'const functionHeaderCollapsed = ref<boolean>(true)'
      );
      expect(headerToggleWired(defaultCollapsed)).toBe(false);
      expect(headerToggleWired(PAGE.replace(HEAD_VIF, ''))).toBe(false);
    });

    it('箭头随收起态切换（收起显示 v）', () => {
      expect(PAGE).toMatch(/\{\{ functionHeaderCollapsed \? 'v' : '\^' \}\}/);
    });
  });

  describe('④ 专项通道不吃「自定义模型」能力（#998 / #1040 / #2593：同一页的 scoped 分支）', () => {
    // #2593 起专项通道不再是独立页，而是同一页的一个 scope ⇒ 判据落在同一份 buildBody：
    // scoped 段在写模型字段**之前** return（结构上不存在可送的模型字段），通用段照旧送三条来源。
    it('功能键恒写，且 scoped 分支在写模型字段之前就 return', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).toMatch(/params\['feature_key'\] = featureKey/);
      expect(scopedBranchReturnsEarly(PAGE)).toBe(true);
    });

    it('scoped 段不含任何模型来源字段（model_source / custom_* / config_id / user_model_id）', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      const retIdx = fn.search(/if \(featureKey\.length > 0\) \{\s*return params/);
      expect(retIdx).toBeGreaterThan(-1);
      expect(fn.slice(0, retIdx)).not.toMatch(/model_source|custom_api_key|config_id|user_model_id/);
    });

    it('scoped 模式不做「请先配置自定义模型」前置校验（模型由管理端单绑定解析）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/const scoped = activeScope\.value\.length > 0/);
      const guard = fn.indexOf('if (!scoped) {');
      const customCheck = fn.indexOf('请先配置自定义模型');
      expect(guard).toBeGreaterThan(-1);
      // 两条模型校验都必须在通用分支里（专项 scope 跳过）
      expect(customCheck).toBeGreaterThan(guard);
      expect(fn.indexOf('平台模型未配置')).toBeGreaterThan(guard);
    });

    it('通用分支仍保留自定义模型校验与来源三分支', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/if \(currentModelSource\.value == 'custom' && customModel\.value\.trim\(\)\.length == 0\)/);
      const body = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      expect(body).toMatch(/params\['config_id'\] = currentModelId\.value/);
      expect(body).toMatch(/params\['user_model_id'\] = currentModelId\.value/);
      expect(body).toMatch(/params\['custom_api_key'\] = customApiKey\.value\.trim\(\)/);
    });

    it('键是**页面 scope**而不是「本轮键」状态（#999 的失效形态不可表达）', () => {
      // #999 真机实测的失效形态（先清后取 ⇒ 键恒为空）以「存在一个本轮键状态」为前提。
      // 现在：键是页面状态 activeScope，发送入口不写它 ⇒ 该缺陷类写不出来，而不是「小心维护」。
      expect(stripCodeComments(PAGE)).not.toMatch(/pendingFeatureKey/);
      expect(stripCodeComments(PAGE)).not.toMatch(/currentFeatureKey/);
      expect(PAGE).toMatch(/const activeScope = ref<string>\(''\)/);
      expect(fnBody(PAGE, 'function onInputSend')).not.toMatch(/activeScope\.value\s*=[^=]/);
    });

    it('去掉 scoped 早退的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = PAGE.replace(/if \(featureKey\.length > 0\) \{\s*return params\s*\}/, '');
      expect(scopedBranchReturnsEarly(broken)).toBe(false);
    });

    it('把早退挪到模型字段之后的变形样本会被检出（顺序判据不可绕过）', () => {
      const reordered = PAGE
        .replace(/if \(featureKey\.length > 0\) \{\s*return params\s*\}/, '')
        .replace(
          "params['model_source'] = currentModelSource.value",
          "params['model_source'] = currentModelSource.value\n\t\tif (featureKey.length > 0) { return params }"
        );
      expect(scopedBranchReturnsEarly(reordered)).toBe(false);
    });
  });
});
