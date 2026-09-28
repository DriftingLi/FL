/**
 * #1340 契约：招聘者简历库必须能从「返回 / 前台恢复」路径刷新（取数走 onShow，且下拉可刷新）。
 *
 * ## 为什么需要这条守护
 *
 * 症状：候选人更新简历后，招聘者停留在简历库或 navigateBack 回到本页，列表仍是旧快照。
 * 根因：本页只在 onLoad 取数，而 onLoad 一生只触发一次；页面实例存活时返回不会重取。
 * （切 tab 走 redirectTo 会重建页，恰好掩盖了这个洞，只有 navigateBack / App 前台恢复才暴露。）
 *
 * 沿用 #1126（course-detail）的「onShow 驱动刷新」口径；但本页受 recruitWorkspaceContract H 约束
 * —— onLoad 开头必须是 guardRecruiter()，故保留 onLoad 的首取，onShow 只负责「再次显示时刷新」，
 * 靠 loading.value 天然避免首屏「onLoad + onShow」双取。
 *
 * ## 这是接线/契约守护（读源码文本），非行为守护
 * .uvue 页面生命周期 utsHarness 无法真执行（见 K2 / docs/agents/guards.md），故本文件属「防回潮」，
 * 不构成 ③ 门承重证据；真效果由 ①a 真机取证（切走切回看到更新 + 下拉刷新）。
 */
const path = require('path');

const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const PAGE = 'pages/recruiter/resumes.uvue';

/** 取 marker 之后的函数/回调体，花括号配平判收尾 */
function bodyAfter(src, marker) {
  const start = src.indexOf(marker);
  if (start < 0) return '';
  const open = src.indexOf('{', start);
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    const ch = src[i];
    if (ch === '{') depth++;
    else if (ch === '}') {
      depth--;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  return '';
}

describe('#1340 简历库刷新：onShow 驱动 + 下拉刷新 + 不清空列表', () => {
  const src = read(PAGE);

  it('从 uni-app 引入了 onShow', () => {
    expect(src).toMatch(/import\s*\{[^}]*\bonShow\b[^}]*\}\s*from\s*'@dcloudio\/uni-app'/);
  });

  it('存在 onShow 钩子，且其中真的调用刷新函数 refreshResumes', () => {
    const body = bodyAfter(src, 'onShow(');
    expect(body).not.toBe('');
    expect(body).toContain('refreshResumes()');
  });

  it('refreshResumes 拉第一页、成功后整体替换 items', () => {
    const body = bodyAfter(src, 'function refreshResumes');
    expect(body).not.toBe('');
    expect(body).toContain('getRecruitResumesApi(filters.value, 1)');
    expect(body).toContain('items.value = result.items');
  });

  it('refreshResumes 不在取数前清空列表（避免闪烁 + 失败保留旧数据）', () => {
    const body = bodyAfter(src, 'function refreshResumes');
    const fetchAt = body.indexOf('getRecruitResumesApi');
    const clearAt = body.indexOf('items.value = []');
    expect(clearAt).toBe(-1);
    expect(fetchAt).toBeGreaterThan(-1);
  });

  it('refreshResumes 以 loading 收口，避免并发（也挡住首屏 onLoad+onShow 双取）', () => {
    const body = bodyAfter(src, 'function refreshResumes');
    expect(body).toMatch(/if\s*\(\s*loading\.value\s*\)/);
  });

  it('scroll-view 已挂下拉刷新（refresher-enabled + @refresherrefresh），且保留 @scrolltolower', () => {
    expect(src).toMatch(/:refresher-enabled="true"/);
    expect(src).toMatch(/@refresherrefresh="onRefresh"/);
    expect(src).toContain('@scrolltolower="onLoadMore"');
  });

  it('onLoad 仍先守卫再首取（受 recruitWorkspaceContract H 约束，不得删）', () => {
    const body = bodyAfter(src, 'onLoad((options');
    expect(body).toContain('guardRecruiter()');
    expect(body).toContain('loadResumes(true)');
    expect(body.indexOf('guardRecruiter()')).toBeLessThan(body.indexOf('loadResumes(true)'));
  });
});
