/**
 * AI 助手「功能入口悬浮」契约（#998 剩余部分）—— 源码契约测试缝（读源文本、断言形态）。
 *
 * 为什么需要它：#998 的原始缺陷是「有对话后上方 5 宫格被顶出视口、且划不回来」。滚动修好
 * （`.pro-scroll` 补 `height: 0`）后，宫格**能**滑回去够到，但入口仍不在手边 ⇒ 本票补一个悬浮入口
 * （FAB → 半屏 sheet）。这里锁三件**写错不报错、只会静默坏**的形态：
 *
 * ① **只有有对话时出现**（`messages.length > 0`）：无对话时宫格本就在首屏，常驻即「同一功能两个
 *    入口」，既冗余又永久占屏。
 * ② **FAB 必须避开底部输入区**（`.input-card` 实测 202rpx 高，见页面样式注释），否则遮挡输入；
 *    且 `z-index` 必须低于半屏 sheet 的 300/301，否则开 sheet 后会浮在遮罩之上。
 * ③ **宫格样式自带且与页面逐值一致**：`manifest.json` 的 `styleIsolationVersion: "2"` 使**组件样式
 *    默认隔离** ⇒ sheet 组件拿不到页面 `.function-grid` 等规则（对照证据：`ai-chat-bubble.uvue`
 *    自己重复定义了 `.message-*`）。于是同一套宫格视觉必然存在**两份**定义，必须防漂移。
 * ④ **不得新增第二条发送路径**：悬浮入口与首屏宫格必须共用 `onFunctionClick`。两条路径必然漂移
 *    （一条带功能键、一条不带），那正是 #999「每轮送空键、专项通道静默失效」的成因形态。
 *
 * 设计沿用既有守护测试的形态（见 utils/aiAssistantScrollContract.test.js）：先对**注入违规**的变形
 * 样本断言检测有效（防空跑假绿），再对真实文件断言合规。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const PAGE = fs.readFileSync(path.join(ROOT, 'pages/ai-assistant/ai-assistant.uvue'), 'utf8');
const SHEET = fs.readFileSync(path.join(ROOT, 'components/ai-chat/ai-chat-function-sheet.uvue'), 'utf8');
const CONSTANTS = fs.readFileSync(path.join(ROOT, 'pages/ai-assistant/ai-assistant-constants.uts'), 'utf8');

/** 去掉 CSS 注释 —— 注释里写着某个值不算实现（本仓契约测试的主要假阳性来源） */
function stripCssComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '');
}

/** 去掉模板/JS 注释 —— 「注释里提到某标识」不等于「代码里用了它」 */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 取某个 class 的规则体（从 `.name {` 切到大括号配平处） */
function classRule(src, name) {
  const m = src.match(new RegExp('\\.' + name + '\\s*\\{'));
  if (!m) throw new Error(`找不到样式规则 .${name}`);
  const open = src.indexOf('{', m.index);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error(`.${name} 大括号未配平`);
}

/** 取某个 class 的某个属性值（未声明返回 null） */
function ruleVal(src, cls, prop) {
  const body = stripCssComments(classRule(src, cls));
  const m = body.match(new RegExp('(?:^|;|\\{)\\s*' + prop + '\\s*:\\s*([^;]+);'));
  return m ? m[1].trim() : null;
}

/**
 * 取带指定 class 的那个模板行。
 * 刻意**不**用 `<view[^>]*class="x"[^>]*>`：`[^>]*` 会被属性值里的 `>` 截断
 * （本票的判据正好是 `messages.length > 0`，用它必然取不到标签 —— 自测踩过）。
 */
function classLine(src, cls) {
  const hit = src.split('\n').find((l) => l.indexOf('class="' + cls + '"') >= 0);
  return hit == null ? null : hit;
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

/** 数字解析（用于 bottom / z-index 的数值比较；解析不出返回 NaN 以便断言失败） */
function num(v) {
  if (v == null) return NaN;
  const m = String(v).match(/-?\d+(\.\d+)?/);
  return m ? Number(m[0]) : NaN;
}

/** 悬浮按钮的 `messages.length > 0` 判据是否在位 */
function fabHasConversationGuard(src) {
  const line = classLine(src, 'func-fab');
  return line != null && /messages\.length > 0/.test(line);
}

/** FAB 的 bottom（rpx 数值） */
function fabBottom(src) {
  return num(ruleVal(src, 'func-fab', 'bottom'));
}

/** FAB 的 z-index 数值 */
function fabZIndex(src) {
  return num(ruleVal(src, 'func-fab', 'z-index'));
}

// 底部输入区实际占用（rpx）：见 pages/ai-assistant/ai-assistant.uvue 的 .func-fab 注释推导
const INPUT_AREA_RPX = 202;
const FAB_MIN_BOTTOM_RPX = 240;

// 组件样式隔离下必须逐值一致的两套宫格类（页面类名 ↔ 组件类名）
const GRID_STYLE_PAIRS = [
  ['function-item', 'func-item', 'width'],
  ['function-icon-box', 'func-icon-box', 'width'],
  ['function-icon-box', 'func-icon-box', 'height'],
  ['function-icon-box', 'func-icon-box', 'margin-bottom'],
  ['function-icon-img', 'func-icon-img', 'width'],
  ['function-icon-img', 'func-icon-img', 'height'],
  ['function-icon-emoji', 'func-icon-emoji', 'font-size'],
  ['function-title', 'func-title', 'font-size'],
  ['function-title', 'func-title', 'color'],
];

describe('AI 助手「功能入口悬浮」契约（#998 剩余部分）', () => {
  describe('① 只有有对话时出现（messages.length > 0）', () => {
    it('页面 FAB 带 messages.length > 0 判据', () => {
      expect(fabHasConversationGuard(PAGE)).toBe(true);
    });

    it('去掉判据的变形样本会被检出（防检测器空跑假绿）', () => {
      // 必须**只**改 func-fab 那一行：页面 L37 的消息区同样是 `v-if="messages.length > 0"`，
      // 用全局 replace 会改到那一处，样本失真（自测踩过）。
      const fabLine = classLine(PAGE, 'func-fab');
      const broken = PAGE.replace(fabLine, fabLine.replace(/ v-if="messages\.length > 0"/, ''));
      expect(fabHasConversationGuard(broken)).toBe(false);
    });
  });

  describe('② 悬浮定位：避开输入区 + 位于 sheet 之下', () => {
    it('FAB 是 position: fixed（否则不悬浮）', () => {
      expect(ruleVal(PAGE, 'func-fab', 'position')).toBe('fixed');
    });

    it(`FAB 的 bottom >= ${FAB_MIN_BOTTOM_RPX}rpx（输入区实测 ${INPUT_AREA_RPX}rpx）`, () => {
      expect(fabBottom(PAGE)).toBeGreaterThanOrEqual(FAB_MIN_BOTTOM_RPX);
      expect(fabBottom(PAGE)).toBeGreaterThan(INPUT_AREA_RPX);
    });

    it('bottom 过小的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = PAGE.replace(/bottom:\s*240rpx;/, 'bottom: 100rpx;');
      expect(fabBottom(broken)).toBeLessThan(FAB_MIN_BOTTOM_RPX);
    });

    it('FAB 的 z-index 低于半屏 sheet 的 300（否则会浮在遮罩之上）', () => {
      expect(fabZIndex(PAGE)).toBeLessThan(300);
      expect(num(ruleVal(SHEET, 'func-sheet-mask', 'z-index'))).toBe(300);
      expect(num(ruleVal(SHEET, 'func-sheet', 'z-index'))).toBe(301);
    });
  });

  describe('③ 宫格样式自带（组件样式隔离）且与页面逐值一致', () => {
    it('sheet 组件定义了自带的宫格类（不依赖页面样式）', () => {
      for (const cls of ['func-grid', 'func-item', 'func-icon-box', 'func-icon-img', 'func-icon-emoji', 'func-title']) {
        expect(classRule(SHEET, cls)).toBeTruthy();
      }
    });

    it.each(GRID_STYLE_PAIRS)('页面 .%s 与组件 .%s 的 %s 逐值一致', (pageCls, sheetCls, prop) => {
      const a = ruleVal(PAGE, pageCls, prop);
      const b = ruleVal(SHEET, sheetCls, prop);
      expect(a).not.toBeNull();
      expect(b).toBe(a);
    });

    it('两侧的图标渲染判据一致（图片优先、emoji 兜底）', () => {
      const cond = 'item.iconPath && item.iconPath.length > 0';
      expect(PAGE).toContain(cond);
      expect(SHEET).toContain(cond);
    });

    it('组件内不得另立功能清单（items 只能由调用方经 props 传入）', () => {
      // 剥注释后判定：组件注释里会**提到** FUNCTION_ITEMS（说明「传的是同一份」），那是说明不是用法。
      expect(stripCodeComments(SHEET)).not.toMatch(/FUNCTION_ITEMS/);
      expect(SHEET).toMatch(/items\?: AIFunctionItem\[\]/);
      expect(PAGE).toMatch(/<AiChatFunctionSheet[^>]*:items="functionItems"/);
    });

    it('在组件内另立清单的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = stripCodeComments(SHEET).replace(
        'visible?: boolean',
        "import { FUNCTION_ITEMS } from './x'\n\t\tvisible?: boolean"
      );
      expect(broken).toMatch(/FUNCTION_ITEMS/);
    });

    it('页面 functionItems 出自唯一事实源 FUNCTION_ITEMS', () => {
      expect(PAGE).toMatch(/const functionItems = FUNCTION_ITEMS/);
      expect(CONSTANTS).toMatch(/export const FUNCTION_ITEMS\s*:\s*AIFunctionItem\[\]/);
    });

    it('样式漂移的变形样本会被检出（防检测器空跑假绿）', () => {
      const drifted = SHEET.replace(/\.func-item\s*\{[\s\S]*?\}/, '.func-item { align-items: center; width: 25%; }');
      expect(ruleVal(drifted, 'func-item', 'width')).not.toBe(ruleVal(PAGE, 'function-item', 'width'));
    });
  });

  describe('④ 悬浮入口与首屏宫格共用同一条发送路径', () => {
    it('sheet 的 select 事件接到 onFunctionEntrySelect', () => {
      expect(PAGE).toMatch(/<AiChatFunctionSheet[^>]*@select="onFunctionEntrySelect"/);
    });

    it('onFunctionEntrySelect 先收起 sheet、再调用 onFunctionClick', () => {
      const fn = fnBody(PAGE, 'function onFunctionEntrySelect');
      expect(fn).toContain('showFunctionSheet.value = false');
      expect(fn).toContain('onFunctionClick(item)');
    });

    it('该处理函数**不**自己发消息（不得长出第二条发送路径）', () => {
      const fn = fnBody(PAGE, 'function onFunctionEntrySelect');
      for (const forbidden of ['sendText(', 'sendInput(', 'pendingFeatureKey', 'buildBody']) {
        expect(fn).not.toContain(forbidden);
      }
    });

    it('长出发送路径的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = fnBody(PAGE, 'function onFunctionEntrySelect').replace(
        'onFunctionClick(item)',
        "pendingFeatureKey.value = item.featureKey ?? ''\nsendText(item.prompt)"
      );
      expect(broken).toContain('pendingFeatureKey');
      expect(broken).toContain('sendText(');
    });

    it('onFunctionClick 仍是「置键 → 发 prompt」的唯一入口', () => {
      const fn = fnBody(PAGE, 'function onFunctionClick');
      expect(fn).toContain("pendingFeatureKey.value = item.featureKey ?? ''");
      expect(fn).toContain('sendText(item.prompt)');
    });
  });

  describe('⑤ 组件交互闭合（遮罩可关、选中回传整项）', () => {
    it('遮罩点击 emit close', () => {
      const fn = fnBody(SHEET, 'function onMaskClick');
      expect(fn).toContain("emit('close')");
    });

    it('选中回传整项 AIFunctionItem（调用方才能拿到 featureKey）', () => {
      const fn = fnBody(SHEET, 'function onSelect');
      expect(fn).toContain("emit('select', item)");
      expect(SHEET).toMatch(/\(e: 'select', item: AIFunctionItem\): void/);
    });
  });
});
