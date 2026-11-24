/**
 * AI 助手页输入卡双态契约（#2788）—— 源码契约测试缝（读源文本、断言形态）。
 *
 * 为什么需要它：双态是「给既有容器换样式类」的实现（**不搬节点**，票面否决 ①：原生 input 搬进滚动容器
 * 会在聚焦同层渲染与重建丢字两件事上翻车），两处 class 由**同一个** computed 驱动。形态一旦漂移，
 * 症状是**静默的**，且每一条都长得像「设计稿没对齐」而不是报错：
 *   - 只给主区换类、忘了滚动区 ⇒ 欢迎态输入卡仍贴底，或滚动区零基准高度把内容整块吞掉；
 *   - 两处各写一份判据 ⇒ 一半欢迎一半对话，没人说得清当下是哪一态；
 *   - 为「保险」给输入区补 height / position ⇒ 微信键盘弹起时被顶飞（ADR-0073 记过这条）。
 * 相邻契约：滚动容器本身的 `flex:1 + height:0` 归 aiAssistantScrollContract 管（#998，真机血账），
 * 本文件管双态，并在 ④ 里钉住「**不许**为双态牺牲那条」。
 *
 * 设计沿用既有守护的形态：先对注入违规的变形样本断言检测有效（防空跑假绿），再对真实文件断言合规。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE = readText(path.join(ROOT, 'pages/ai-assistant/ai-assistant.uvue'));
const INPUT = readText(path.join(ROOT, 'components/ai-chat/ai-chat-input.uvue'));
const CONSTANTS = readText(path.join(ROOT, 'pages/ai-assistant/ai-assistant-constants.uts'));

/** 取某个 class 的规则体（从 `.name {` 切到大括号配平处） */
function classRule(src, name) {
  const m = src.match(new RegExp('\\.' + name + '\\s*\\{'));
  if (!m) throw new Error('找不到样式规则 .' + name);
  const open = src.indexOf('{', m.index);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error('.' + name + ' 大括号未配平');
}

/** 去掉 CSS 注释 —— 注释里写着 height: auto 不算实现 */
function stripCssComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '');
}

/** 判据本体：唯一 computed 同时看「非专项」与「消息为空」 */
function judgeOf(src) {
  const m = src.match(/const isWelcomeState = computed<boolean>\(\(\) : boolean => ([^\n]+)\)/);
  return m ? m[1] : '';
}

/** 某个基类节点上绑的欢迎态类名（该节点 :class 三元里的第一个分支） */
function boundWelcomeClass(src, base) {
  const at = src.indexOf('class="' + base + '"');
  if (at < 0) return '';
  const tag = src.slice(at, src.indexOf('>', at));
  const marker = ":class=\"isWelcomeState ? '";
  const i = tag.indexOf(marker);
  if (i < 0) return '';
  const rest = tag.slice(i + marker.length);
  return rest.slice(0, rest.indexOf("'"));
}

describe('AI 助手页输入卡双态契约（#2788）', () => {
  describe('① 判据唯一：一个 computed 看两件事', () => {
    it('isWelcomeState = 非专项 && 消息为空，且全文只声明一次', () => {
      const judge = judgeOf(PAGE);
      expect(judge).toContain('!isScoped.value');
      expect(judge).toContain('messages.value.length == 0');
      expect((PAGE.match(/const isWelcomeState\b/g) || []).length).toBe(1);
    });

    it('漏掉任一条件的变形样本会被检出（防检测器空跑假绿）', () => {
      const onlyScoped = judgeOf('const isWelcomeState = computed<boolean>(() : boolean => !isScoped.value)');
      expect(onlyScoped).not.toContain('messages.value.length == 0');
      const onlyEmpty = judgeOf('const isWelcomeState = computed<boolean>(() : boolean => messages.value.length == 0)');
      expect(onlyEmpty).not.toContain('!isScoped.value');
    });
  });

  describe('② 两处绑定同一判据（主区 + 滚动区，缺一处就半态）', () => {
    it('主区绑 main-welcome、滚动区绑 scroll-welcome', () => {
      expect(boundWelcomeClass(PAGE, 'main')).toBe('main-welcome');
      expect(boundWelcomeClass(PAGE, 'pro-scroll')).toBe('scroll-welcome');
    });

    it('只绑主区、滚动区漏绑的变形样本会被检出', () => {
      const onlyMain = '<view class="main" :class="isWelcomeState ? \'main-welcome\' : \'\'">';
      expect(boundWelcomeClass(onlyMain, 'pro-scroll')).toBe('');
    });
  });

  describe('③ 欢迎态真的「高度随内容」', () => {
    it('主区与滚动区都把 flex-grow 归零、基准高度回到 auto', () => {
      const main = stripCssComments(classRule(PAGE, 'main-welcome'));
      expect(main).toMatch(/flex-grow\s*:\s*0\s*;/);
      expect(main).toMatch(/flex-basis\s*:\s*auto\s*;/);
      const scroll = stripCssComments(classRule(PAGE, 'scroll-welcome'));
      expect(scroll).toMatch(/flex-grow\s*:\s*0\s*;/);
      // 基础规则里是 height:0；欢迎态必须放回 auto，否则滚动区塌成 0 高、欢迎内容整块看不见
      expect(scroll).toMatch(/height\s*:\s*auto\s*;/);
    });

    it('仍写 flex:1 / 没有 height:auto 的变形样本会被检出', () => {
      const broken = '.scroll-welcome { flex-grow: 0; flex-basis: auto; }';
      const body = stripCssComments(classRule(broken, 'scroll-welcome'));
      expect(body).not.toMatch(/height\s*:\s*auto\s*;/);
    });
  });

  describe('④ 对话态没被牺牲（与 #998 滚动契约互补）', () => {
    it('基础 .pro-scroll 仍是 flex:1 + height:0', () => {
      const body = stripCssComments(classRule(PAGE, 'pro-scroll'));
      expect(body).toMatch(/flex\s*:\s*1\s*;/);
      expect(body).toMatch(/height\s*:\s*0\s*;/);
    });

    it('把 height:0 挪进欢迎态的变形样本会被检出', () => {
      const broken = '.pro-scroll { flex: 1; padding: 24rpx; } .scroll-welcome { height: 0; }';
      expect(stripCssComments(classRule(broken, 'pro-scroll'))).not.toMatch(/height\s*:\s*0\s*;/);
    });
  });

  describe('⑤ 三张提问卡只活在欢迎态（#2814）', () => {
    /** 取 prompt-list 节点的 v-if 表达式（判据源） */
    function promptGateOf(src) {
      const m = src.match(/<view v-if="([^"]+)" class="prompt-list">/);
      return m ? m[1] : '';
    }

    /** QUICK_PROMPTS 的条目数（常量里每条是一行带引号的字符串） */
    function quickPromptCount(src) {
      const m = src.match(/export const QUICK_PROMPTS : string\[\] = \[([\s\S]*?)\]/);
      if (!m) return -1;
      return (m[1].match(/'[^']*'/g) || []).length;
    }

    /** 把 ① 的判据本体代入 (scoped, 消息条数) 求值 —— 只替换两个输入，不掺第二套条件 */
    function renderCardCount(scoped, messageCount) {
      const judge = judgeOf(PAGE);
      const expr = judge
        .replace(/isScoped\.value/g, String(scoped))
        .replace(/messages\.value\.length/g, String(messageCount));
      const visible = Function('return (' + expr + ')')();
      return visible ? quickPromptCount(CONSTANTS) : 0;
    }

    it('prompt-list 的 v-if 就是 isWelcomeState，不再另写 !isScoped', () => {
      expect(promptGateOf(PAGE)).toBe('isWelcomeState');
    });

    it('欢迎态 3 张、对话态 0 张（同一条判据代入两态）', () => {
      expect(quickPromptCount(CONSTANTS)).toBe(3);
      expect(renderCardCount(false, 0)).toBe(3);
      expect(renderCardCount(false, 1)).toBe(0);
      // 专项 scope 本来就无提问卡（回归：别因为这次收窄把它放出来）
      expect(renderCardCount(true, 0)).toBe(0);
    });

    it('把 v-if 写回「仅通用区」的变形样本会被检出', () => {
      const broken = '<view v-if="!isScoped" class="prompt-list">';
      expect(promptGateOf(broken)).not.toBe('isWelcomeState');
      expect(promptGateOf(broken)).toBe('!isScoped');
    });
  });

  describe('⑥ 不搬节点、不新增键盘/定位手段（票面否决 ② 与 AC 末两条）', () => {
    it('输入卡仍在滚动容器之外', () => {
      const inputAt = PAGE.indexOf('<AiChatInput ');
      expect(inputAt).toBeGreaterThan(-1);
      expect(inputAt).toBeGreaterThan(PAGE.indexOf('</scroll-view>'));
    });

    it('页面与输入卡都没有键盘位移属性（再叠一层补偿会把输入区顶出屏幕）', () => {
      for (const src of [PAGE, INPUT]) {
        expect(src).not.toContain('keyboardheightchange');
        expect(src).not.toContain('adjust-position');
      }
    });

    it('双态样式类里没有 position', () => {
      for (const name of ['main-welcome', 'scroll-welcome']) {
        expect(stripCssComments(classRule(PAGE, name))).not.toMatch(/position\s*:/);
      }
    });

    it('把输入卡塞进滚动容器的变形样本会被检出', () => {
      const broken = '<scroll-view class="pro-scroll"><AiChatInput /></scroll-view>';
      expect(broken.indexOf('<AiChatInput ')).toBeLessThan(broken.indexOf('</scroll-view>'));
    });
  });
});
