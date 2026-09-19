/**
 * AI 助手页滚动容器契约（#998）—— 源码契约测试缝（读源文本、断言样式形态）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` 的内容区在真机上**完全滚不动**，
 * 后果是「有了对话之后，上方欢迎区（横幅 / 5 宫格 / 快捷提示语）被顶出视口且划不回来」——
 * 那些入口就点不到了（#998）。
 *
 * 真机实测出来的根因（2026-09-15，小米 2510DRK44C）：
 * 滚动容器只写 `flex: 1` 时**会长到内容高度**。给 `.pro-scroll` 上唯一背景色后用像素量，
 * 它占 1692px（≈846rpx），**比窗口高度 771 还大** ⇒ 没有可滚动余量 ⇒ 手势划不动。
 * 而 `scroll-into-view` 仍会把视图**视觉上跳到底部**（把欢迎区顶出视口），所以表面上
 * 「像是滚了」，更难发现。
 *
 * 修法与同项目已工作的兄弟页一致（`pages/ai-assistant/ai-settings.uvue` 的 `.content-scroll`）：
 * **`flex: 1` 必须配 `height: 0`**，用零基准高度让 flex 分配生效，容器才被约束成视口高度。
 *
 * 这个坑的特点：**写错不报错、不警告**，只是「手势静默无效」——只能靠契约测试锁住形态。
 * （本仓同类守护见 utils/gradientSyntaxContract.test.js、utils/deviceCaptureContract.test.js。）
 *
 * 设计沿用既有守护测试的形态：先对「注入违规」的变形样本断言检测有效（防空跑假绿），
 * 再对真实文件断言合规。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE = readText(path.join(ROOT, 'pages/ai-assistant/ai-assistant.uvue'));

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

/** 去掉 CSS 注释 —— 注释里写着 `height: 0` 不算实现（本文件就是主要假阳性来源） */
function stripCssComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '');
}

describe('AI 助手页滚动容器契约（#998）', () => {
  describe('① 滚动容器必须被约束（flex:1 + height:0）', () => {
    it('.pro-scroll 同时有 flex:1 与 height:0，且 height 在 flex 之后', () => {
      const body = stripCssComments(classRule(PAGE, 'pro-scroll'));
      expect(body).toMatch(/flex\s*:\s*1\s*;/);
      expect(body).toMatch(/height\s*:\s*0\s*;/);
      // height 必须晚于 flex 出现：若 height:0 在前、flex:1 在后，后者会把基准高度重新撑开
      expect(body.indexOf('height: 0')).toBeGreaterThan(body.indexOf('flex: 1'));
    });

    it('缺 height:0 的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = `
	.pro-scroll {
		flex: 1;
		padding: 24rpx;
	}`;
      const body = stripCssComments(classRule(broken, 'pro-scroll'));
      expect(body).not.toMatch(/height\s*:\s*0\s*;/);
    });
  });

  describe('② 竖向滚动开关用本仓主流写法 scroll-y="true"', () => {
    it('scroll-view 带 scroll-y="true"', () => {
      const tag = PAGE.match(/<scroll-view[^>]*class="pro-scroll"[^>]*>/);
      expect(tag).not.toBeNull();
      expect(tag[0]).toMatch(/scroll-y="true"/);
    });

    it('裸 scroll-y 的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = '<scroll-view scroll-y class="pro-scroll">';
      expect(broken).not.toMatch(/scroll-y="true"/);
    });
  });

  describe('③ 根容器有确定高度（与 ai-settings.uvue 同款）', () => {
    it('根节点绑定 pageHeight，onLoad 里用 windowHeight 赋值', () => {
      expect(PAGE).toMatch(/class="ai-page"\s*:style="\{ height: pageHeight \+ 'px' \}"/);
      expect(PAGE).toMatch(/const pageHeight = ref<number>\(0\)/);
      expect(PAGE).toMatch(/pageHeight\.value = sysInfo\.windowHeight/);
    });
  });
});
