/**
 * `source` 外链入口的接线锁（#1939）
 *
 * 行为腿（featuredSource.test.js）证明判别函数对，这里证明**页面真的接了它**：
 * 接线断了两条都会绿——这是本仓「行为测试会自己搭依赖」的已知盲区。
 */
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const PAGE = readText(path.join(ROOT, 'pages', 'featured', 'featured-detail.uvue'));

/** 抹注释（先例 dashboardContract）：本页注释里也写着「return / loading」，不抹会把解释文字当判据 */
const stripComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');

describe('内容精选详情页 source 外链接线（#1939）', () => {
  it('判别与投影来自 utils/featuredSource 单点（页面不另抄协议判断）', () => {
    expect(PAGE).toMatch(/import\s*\{[^}]*isExternalSourceUrl[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/featuredSource'/);
    expect(PAGE).toMatch(/import\s*\{[^}]*sourceDomainOf[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/featuredSource'/);
  });

  it('「查看原文」只在 hasSourceLink 成立时渲染（名称形态不造按钮）', () => {
    expect(PAGE).toContain('v-if="hasSourceLink"');
    expect(PAGE).toContain('查看原文');
  });

  it('缺 id / id 非法 ⇒ 落「内容不存在」态，不停在加载中（#2097）', () => {
    // 判据钉的是**产物路径**：提前 return 之前必须先把 loading 放下，
    // 否则 contentId<=0 的分支永远渲染 v-if="loading" 那一屏（② 门截图即此形态）。
    const body = stripComments(PAGE.slice(PAGE.indexOf('async function loadDetail')));
    const early = body.slice(0, body.indexOf('loading.value = true'));
    expect(early).toContain('contentId.value <= 0');
    expect(early).toContain('loading.value = false');
    // 反向：不许出现「先 return、后面才放 loading」的旧形态
    expect(early.indexOf('loading.value = false')).toBeLessThan(early.indexOf('return'));
  });

  it('点击委托给统一编排（页面内不出现第二份 showModal 确认）', () => {
    expect(PAGE).toMatch(/import\s*\{[^}]*dispatchExternalLinkTap[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/externalLinkDispatch'/);
    expect(PAGE).toContain('dispatchExternalLinkTap(detail.value!.source)');
    expect(PAGE).not.toContain('uni.showModal');
  });
});
