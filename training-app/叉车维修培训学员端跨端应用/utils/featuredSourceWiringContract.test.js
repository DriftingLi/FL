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

describe('内容精选详情页 source 外链接线（#1939）', () => {
  it('判别与投影来自 utils/featuredSource 单点（页面不另抄协议判断）', () => {
    expect(PAGE).toMatch(/import\s*\{[^}]*isExternalSourceUrl[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/featuredSource'/);
    expect(PAGE).toMatch(/import\s*\{[^}]*sourceDomainOf[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/featuredSource'/);
  });

  it('「查看原文」只在 hasSourceLink 成立时渲染（名称形态不造按钮）', () => {
    expect(PAGE).toContain('v-if="hasSourceLink"');
    expect(PAGE).toContain('查看原文');
  });

  it('点击委托给统一编排（页面内不出现第二份 showModal 确认）', () => {
    expect(PAGE).toMatch(/import\s*\{[^}]*dispatchExternalLinkTap[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/externalLinkDispatch'/);
    expect(PAGE).toContain('dispatchExternalLinkTap(detail.value!.source)');
    expect(PAGE).not.toContain('uni.showModal');
  });
});
