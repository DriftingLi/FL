/**
 * 考情资讯页接线契约（#1940）—— 接线腿
 *
 * 钉住「页面取数走公开接口 + 口径单点在 utils/examInfoTabs + 详情复用 featured-detail」这几条接线，
 * 防止下一次重构把页面改回 mock、把类别映射抄回模板、或把详情落点改成别的页。
 * 它**不**证明映射本身对（那是行为腿 examInfoTabs.test.js 的事）。
 */
const fs = require('fs');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const PAGE = path.join(ROOT, 'pages/exam-info/exam-info.uvue');
const API = path.join(ROOT, 'api/featured.uts');

function read(p) {
  return readText(p);
}

/** 判据本体：页面层「接线面」违规清单（真源与注入自检共用同一条） */
function wiringViolations(src) {
  const hits = [];
  if (/mockItems|mockItems\s*=|详情开发中/.test(src)) hits.push('退回 mock（mockItems / 详情开发中）');
  if (!/getFeaturedContentsApi\(/.test(src)) hits.push('未接公开列表接口 getFeaturedContentsApi');
  if (!/getFeaturedContentsApi\([^)]*query\.sort/.test(src)) hits.push('未把 chips 的 sort 传给接口');
  if (!/EXAM_INFO_TABS/.test(src)) hits.push('tab 表未走 utils/examInfoTabs 单点');
  if (!/appEmptyState|app-empty-state/.test(src)) hits.push('未复用 app-empty-state（空态/失败态）');
  if (!/\/pages\/featured\/featured-detail\?id=/.test(src)) hits.push('详情落点不是 featured-detail');
  if (/uni\.request\s*\(/.test(src)) hits.push('页面直发请求（uni.request）');
  for (const lit of ["'industry'", "'news'", "'exam'"]) {
    if (src.includes(lit)) hits.push('类别字面量回到页面层：' + lit);
  }
  return hits;
}

describe('考情资讯页接线契约（#1940）', () => {
  it('页面取数走公开接口、口径单点、详情落点正确', () => {
    expect(wiringViolations(read(PAGE))).toEqual([]);
  });

  it('判据具备判别力：把页面改回 mock / 抄回类别字面量，必须抓到', () => {
    const src = read(PAGE);
    const injected = src + "\n    const mockItems = []\n    const c = 'exam'\n";
    const hits = wiringViolations(injected);
    expect(hits).toContain('退回 mock（mockItems / 详情开发中）');
    expect(hits.join(' ')).toContain("类别字面量回到页面层：'exam'");
  });

  it('公开列表接口透传 sort（后端 handler.go 早已支持 latest|hot）', () => {
    const src = read(API);
    expect(src).toMatch(/sort\s*:\s*string\s*=\s*''/);
    expect(src).toMatch(/params\['sort'\]\s*=\s*sort/);
  });

  it('页面不再出现旧的七个 tab 表（收敛为四 tab 的另一半判据）', () => {
    const src = read(PAGE);
    for (const gone of ['招聘信息', '相关问题', '岗位表']) {
      expect(src).not.toContain(gone);
    }
  });
});
