/**
 * cert-selector 浮层遮罩渲染契约（refs #1671）
 *
 * 现象（#1650 三形态取证）：mp-weixin 端组件内 `.cert-dropdown-mask` 的
 * `position: fixed` 全屏遮罩一帧都不绘制（12 帧 mask=NONE、darkFrac≤0.03），
 * 面板因此塌成内容高。同页先例：dashboard 页面级 `.filter-mask` 用
 * `position: absolute`（源码即如此，编译产物忠实）一直工作正常。
 * 修复（#1671）：组件级遮罩 fixed → absolute，锚定宿主 `.container`
 * （uvue.wxss 基座给页面 view 默认 position:relative ⇒ 它是定位祖先），
 * 四向 inset:0 / z-index / 遮罩色 / 点击关闭语义全部保留。
 *
 * 本文件锁的是这条渲染事实的**源码形状**：遮罩规则必须是 absolute 且永不回退 fixed，
 * 铺满四向、层级、半透明遮罩色、@click 关闭语义缺一不可；embedded 形态类不得被波及。
 * 分类：接线守护（读源码文本断言），不构成 ③ 门证据；判别力由注入违规样本的必红面自证。
 */
const h = require('./contractHarness');

const SRC_REL = 'components/cert-selector/cert-selector.uvue';

/** 提取指定 class 选择器的规则体（第一个 `{...}` 内文本）；找不到返回 null */
function ruleBodyOf(src, selector) {
  const m = src.match(new RegExp(selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\{'));
  if (!m) return null;
  const open = src.indexOf('{', m.index);
  const close = src.indexOf('}', open);
  if (close === -1) return null;
  return src.slice(open + 1, close);
}

/** 抹注释（块注释 + 行注释，`://` 例外防误杀 URL）。先例 dashboardContract.stripComments。
 *  必须先抹再判：遮罩规则体内带着解释 #1671 的注释，注释文本本身含「position:fixed」字样。 */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/** 遮罩契约检测器：返回违规清单（空 = 守约）。真实文件与注入样本共用同一条检测路径。 */
function overlayViolations(src) {
  const v = [];
  const body = stripComments(ruleBodyOf(src, '.cert-dropdown-mask') || '');
  if (!ruleBodyOf(src, '.cert-dropdown-mask')) {
    v.push('.cert-dropdown-mask 规则缺失（遮罩语义被拆）');
    return v;
  }
  if (/position\s*:\s*fixed/.test(body)) {
    v.push('遮罩禁止 position:fixed —— mp-weixin 组件内 fixed 全屏遮罩不绘制（#1671 / #1650 取证）');
  }
  if (!/position\s*:\s*absolute/.test(body)) {
    v.push('遮罩必须 position:absolute（锚定宿主 .container 定位祖先；页面级 .filter-mask 同构）');
  }
  for (const dir of ['left', 'right', 'top', 'bottom']) {
    if (!new RegExp(dir + '\\s*:\\s*0\\s*;').test(body)) {
      v.push('遮罩铺满缺失: ' + dir + ':0（全屏覆盖语义）');
    }
  }
  if (!/z-index\s*:\s*300\s*;/.test(body)) {
    v.push('遮罩层级缺失: z-index:300（须盖住页面内容与 filter-mask 的 200）');
  }
  if (!/background-color\s*:\s*rgba\(0,\s*0,\s*0,\s*0\.4\)/.test(body)) {
    v.push('遮罩半透明底色缺失: rgba(0,0,0,0.4)（压暗语义，#1650 机检行判据 darkFrac 依赖它）');
  }
  return v;
}

/* ══ 真实文件（必不红面） ══ */
describe('cert-selector 遮罩渲染契约（真实源码，#1671）', () => {
  const src = h.read(SRC_REL);

  it('遮罩规则守约：absolute + 四向铺满 + z-index:300 + 半透明底色，且零违规', () => {
    expect(overlayViolations(src)).toEqual([]);
  });

  it('遮罩点击关闭语义仍在模板上（@click="onMaskTap"）', () => {
    expect(src).toContain('@click="onMaskTap"');
  });

  it('面板仍是遮罩的普通 flex 子（flex:1 依赖遮罩的确定高度，#1473 先例）', () => {
    const panel = ruleBodyOf(src, '.cert-dropdown');
    expect(panel).not.toBeNull();
    expect(panel).toMatch(/flex\s*:\s*1/);
  });

  it('embedded 形态类不受本次修复波及（自身不定位、吃满宿主内容区）', () => {
    const embed = ruleBodyOf(src, '.cert-selector-embed');
    expect(embed).not.toBeNull();
    expect(embed).toMatch(/flex\s*:\s*1/);
    expect(embed).not.toMatch(/position\s*:/);
  });
});

/* ══ 判别力（必红面）：注入违规样本，检测器必须红 ══ */
describe('检测器判别力（注入违规样本必红）', () => {
  // 修复前的 master 形状（fixed 遮罩）——检测器对它必须报「禁止 fixed」
  const FIXED_SAMPLE = [
    '<template><view class="cert-dropdown-mask" @click="onMaskTap"></view></template>',
    '<style>',
    '.cert-dropdown-mask {',
    '    position: fixed;',
    '    left: 0;',
    '    right: 0;',
    '    top: 0;',
    '    bottom: 0;',
    '    z-index: 300;',
    '    background-color: rgba(0, 0, 0, 0.4);',
    '}',
    '</style>',
  ].join('\n');

  it('fixed 遮罩样本（修复前形状）→ 报「禁止 fixed」', () => {
    const v = overlayViolations(FIXED_SAMPLE);
    expect(v.some((x) => x.includes('position:fixed'))).toBe(true);
  });

  it('删掉 absolute（只删定位不动其余）→ 报「必须 absolute」', () => {
    const v = overlayViolations(FIXED_SAMPLE.replace(/position\s*:\s*fixed;/, ''));
    expect(v.some((x) => x.includes('必须 position:absolute'))).toBe(true);
  });

  it('删掉半透明底色 → 报「遮罩语义缺失」', () => {
    const src = h.read(SRC_REL).replace(/background-color:\s*rgba\(0,\s*0,\s*0,\s*0\.4\);/, '');
    expect(overlayViolations(src).some((x) => x.includes('遮罩半透明底色缺失'))).toBe(true);
  });

  it('整条规则被删 → 报「规则缺失」', () => {
    expect(overlayViolations('<style>\n.other { color: #fff; }\n</style>')).toEqual([
      '.cert-dropdown-mask 规则缺失（遮罩语义被拆）',
    ]);
  });
});
