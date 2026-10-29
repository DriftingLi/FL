/**
 * 内容精选详情页的口径锁（#2097）
 *
 * 前身是 `utils/featuredSourceWiringContract.test.js`：那个文件整体钉的是 #1939 的 `source` **双形态**
 * 接线，而双形态已被移动端 ADR-0045「修订（2026-10-28，#1940 裁定）」列入**明确不做**（`source` = 来源
 * 名称，外链由正文 Markdown 链接承载），随回退一并删除。这里只留回退后仍必须成立的判据：
 *
 *  1. 缺 id / 非法 id ⇒ 落「内容不存在」，不是永久「加载中」（#2097 由 ② 门截图暴露的真缺陷）；
 *  2. `source` 列不得再被当 URL 用 —— 协议判别、域名投影、「查看原文」按钮都不许回到本页
 *     （存量媒体名会被误当 URL，正是该 ADR「明确不做」点名的事故形态）；
 *  3. 外链入口的唯一形态 = 正文 Markdown 链接经 `utils/featuredLinks.uts` 收成「参考链接」列表，
 *     点击委托 `externalLinkDispatch`：页面既不判协议，也不自建确认弹窗（闸只许有分词器那一处）。
 *
 * 判据形态 = 接线锁（读产物文本），不是行为断言：接线断了两条都会绿，这是本仓「行为测试会自己搭
 * 依赖」的已知盲区（先例 `dashboardContract.test.js`）。行为面另在 `featuredLinks.test.js`（真执行）。
 */
const path = require('path');
const fs = require('fs');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const PAGE = readText(path.join(ROOT, 'pages', 'featured', 'featured-detail.uvue'));
const LINKS = readText(path.join(ROOT, 'utils', 'featuredLinks.uts'));

/** 抹注释（先例 dashboardContract）：本页注释里也写着「return / loading」，不抹会把解释文字当判据 */
const stripComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');

describe('内容精选详情页：缺 id 落态 + source 单用途 + 外链只经正文链接（#2097）', () => {
  it('缺 id / id 非法 ⇒ 落「内容不存在」态，不停在加载中', () => {
    // 判据钉的是**产物路径**：提前 return 之前必须先把 loading 放下，
    // 否则 contentId<=0 的分支永远渲染 v-if="loading" 那一屏（② 门截图即此形态）。
    const body = stripComments(PAGE.slice(PAGE.indexOf('async function loadDetail')));
    const early = body.slice(0, body.indexOf('loading.value = true'));
    expect(early).toContain('contentId.value <= 0');
    expect(early).toContain('loading.value = false');
    // 反向：不许出现「先 return、后面才放 loading」的旧形态
    expect(early.indexOf('loading.value = false')).toBeLessThan(early.indexOf('return'));
  });

  it('source 只作来源名称：双形态判别件与「查看原文」入口都不得复活', () => {
    expect(fs.existsSync(path.join(ROOT, 'utils', 'featuredSource.uts'))).toBe(false);
    const code = stripComments(PAGE);
    expect(code).not.toContain('isExternalSourceUrl');
    expect(code).not.toContain('sourceDomainOf');
    expect(code).not.toContain('hasSourceLink');
    expect(code).not.toContain('onOpenSource');
    expect(code).not.toContain('查看原文');
    // 元信息那一行直出名称列本身，不经任何投影（模板文本不走抹注释那条）
    expect(PAGE).toContain('<text v-if="detail!.source.length > 0" class="meta-text">{{ detail!.source }}</text>');
  });

  it('外链入口只经正文链接：页面不判协议、不自建确认弹窗', () => {
    const code = stripComments(PAGE);
    expect(code).toContain("from '../../utils/featuredLinks'");
    expect(code).toContain('collectFeaturedLinks(detail.value!.content)');
    // 无链接的正文整段不渲染（不是摆一个空标题糊过去）
    expect(code).toContain('v-if="sourceLinks.length > 0"');
    // 点击一律委托既有编排（移动端 ADR-0025 ⑩-8/⑩-9）：确认弹窗与按端分叉都在那一个模块里
    expect(code).toContain("from '../../utils/externalLinkDispatch'");
    expect(code).toContain('dispatchExternalLinkTap(url)');
    expect(code).not.toContain('uni.showModal');
    // 协议闸只许有分词器那一处：页面里既不出现协议字面量，也不直引闸门函数
    expect(code).not.toContain('inlineLinkSafe');
    expect(code).not.toContain('https://');
    expect(code).not.toContain('http://');
  });

  it('featuredLinks 模块不立第二把闸：不引 inlineLinkSafe、无坏协议字面量', () => {
    const code = stripComments(LINKS);
    expect(code).toContain("from './markdownInline'");
    expect(code).not.toContain('inlineLinkSafe');
    expect(code).not.toMatch(/javascript:|ftp:/);
  });
});
