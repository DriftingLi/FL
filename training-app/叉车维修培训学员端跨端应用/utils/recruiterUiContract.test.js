/**
 * 招聘端 UI 改造契约（#1604 批1：招聘端 9 页样板批）
 *
 * 钉住 ui-spec §9 机检五硬指标在本批 9 页上的落点——本批是样板批，
 * 断言形态就是后续批 2–6 铺开时的模板：
 *
 * 1. 声明位裸 hex ∈ token 值 ∪ §1.6 白名单（utils/uiHexWhitelist.js，逐值理由）
 * 2. `<style lang="scss">` 100%（uvue CSS 引擎不吃未预处理的 SCSS 变量，移动端 AGENTS.md 判据 1）
 * 3. 手写 `.status-bar` = 0 且页头件引用 100%（login 页 N05 自绘头豁免已于 #1608 批5 清零，见 N05_SELF_DRAWN）
 * 4. app-* 组件引用（渐进：本批钉 app-nav-bar 引用）
 * 5. 主色背景走 token（#239EDD 字面值不豁免，ui-spec §9）
 *
 * 判据来源：docs/ui-spec.md §5.1/§7.1/§9；N×G 落位真源 = #1592 终审
 * （N02×6 / N03×2 / N05×1，见本文件末 describe 的落位表断言）。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** 批1 九页（N×G 落位真源 #1592：N02×6 / N03×2 / N05×1） */
const PAGES = [
  'pages/recruiter/home.uvue',
  'pages/recruiter/jobs.uvue',
  'pages/recruiter/resumes.uvue',
  'pages/recruiter/contacts.uvue',
  'pages/recruiter/applications.uvue',
  'pages/recruiter/me.uvue',
  'pages/recruiter/job-edit.uvue',
  'pages/recruiter/resume-detail.uvue',
  'pages/recruiter/login.uvue',
];

/** N05 自绘头豁免页——#1608 批5 已清零：login 页头件无文字槽的旧限制（#1604）由 #1602
    归一消解（返回语义走 leftClick 事件、图标形态全站统一），九页全部引用页头件。 */
const N05_SELF_DRAWN = [];
/** N02/N03/N05 必须引用页头件的页 */
const NAV_BAR_PAGES = PAGES.filter((p) => !N05_SELF_DRAWN.includes(p));

/** token 值集合（uni.scss 的 $-变量值，归一成大写 6 位）。机检判据 = 裸 hex ∉ (token值 ∪ 白名单) */
const UNI_SCSS = read('uni.scss');
const tokenValues = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)].map((m) =>
    m[1].toUpperCase(),
  ),
);

/** 归一：3 位 hex 折成 6 位、大写（ui-spec §9 口径） */
function normalizeHex(hex) {
  const h6 = hex.length === 4
    ? hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3]
    : hex.slice(1, 7);
  return `#${h6}`.toUpperCase();
}

/** 取 <style …> 块全文（含属性行） */
function styleBlock(src) {
  const start = src.indexOf('<style');
  if (start < 0) return '';
  return src.slice(start, src.indexOf('</style>'));
}

/** 声明位裸 hex 违规清单（样式块内、值 ∉ token ∪ 白名单）。只出事实。 */
function hexViolations(src) {
  // 先剥注释（/* */ 与 // 行）——issue 引用如 #947/#1608 会伪装成 3–4 位 hex（ui-spec §9 只判声明位；
  // 剥注释口径与 batch4UiContract.test.js 对齐，批1 早于该教训，#1608 批5 补上）
  const style = styleBlock(src)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^\s*\/\/.*$/gm, '');
  const hexes = style.match(/#[0-9a-fA-F]{3,8}\b/g) || [];
  return hexes.filter((raw) => {
    const v = normalizeHex(raw);
    return !tokenValues.has(v) && !WHITELIST[v];
  });
}

for (const p of PAGES) {
  const src = read(p);

  describe(`${p}（ui-spec §9 机检）`, () => {
    it('H1 样式块声明位裸 hex ∈ token 值 ∪ §1.6 白名单', () => {
      expect(hexViolations(src)).toEqual([]);
    });

    it('H2 样式块声明 lang="scss"', () => {
      expect(styleBlock(src)).toMatch(/<style lang="scss">/);
    });

    it('H5 主色不出现字面值（#239EDD 必须写 $primary-color）', () => {
      expect(styleBlock(src)).not.toMatch(/#239EDD/i);
    });
  });
}

describe('批1 九页 · 页头归位（ui-spec §5.1 / §7.1）', () => {
  for (const p of NAV_BAR_PAGES) {
    it(`${p} 引用 app-nav-bar 且不再手写 .status-bar / 手写页头`, () => {
      const src = read(p);
      expect(src).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
      expect(src).toContain('<app-nav-bar');
      expect(src).not.toMatch(/class="status-bar"/);
    });
  }

  it('N05 页 login.uvue 已随 #1608 批5 迁移页头件（transparent 自垫 $bg-header，不再自绘 .status-bar/.nav-back）', () => {
    const src = read('pages/recruiter/login.uvue');
    expect(src).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
    expect(src).toMatch(/<app-nav-bar[^>]*transparent/);
    expect(src).toMatch(/<app-nav-bar[^>]*@left-click="leaveRecruiterLogin"/);
    expect(src).not.toMatch(/class="status-bar"/);
    expect(src).not.toContain('useSafeArea');
    expect(src).toMatch(/\.recruiter-login-page\s*\{[^}]*background-color:\s*\$bg-header/);
  });

  it('N03 两页（job-edit / resume-detail）走 divider 分隔线', () => {
    expect(read('pages/recruiter/job-edit.uvue')).toMatch(/<app-nav-bar[^>]*divider/);
    expect(read('pages/recruiter/resume-detail.uvue')).toMatch(/<app-nav-bar[^>]*divider/);
  });

  it('批1 九页均不再 import useStatusBarHeight（页头件内化后的旧接口，#1602 起被 useSafeArea 取代）', () => {
    for (const p of PAGES) {
      expect(read(p)).not.toContain('useStatusBarHeight');
    }
  });
});

describe('§1.6 白名单文件自检', () => {
  it('白名单每个值都有理由且格式合法（逐值一行，不按次数豁免）', () => {
    for (const [value, reason] of Object.entries(WHITELIST)) {
      expect(value).toMatch(/^#[0-9A-F]{6}$/);
      expect(reason.length).toBeGreaterThan(10);
    }
  });

  it('白名单值不得与 token 值重叠（重叠即该值应直接走 token；例外：#F5F5F5=$gradient-end 渐变尾色，语义不是中性底）', () => {
    const SEMANTIC_MISMATCH = new Set(['#F5F5F5']);
    for (const value of Object.keys(WHITELIST)) {
      if (SEMANTIC_MISMATCH.has(value)) continue;
      expect(tokenValues.has(value)).toBe(false);
    }
  });

  it('白名单值必须在某个受管页面里真实出现（防死条目；受管页 = 批1 ∪ 批2（#1605 四页 + #1606 十四页）∪ 批4（#1607 十九页）∪ 批5（#1608 七页）∪ 批6（#1609 七页），白名单是全仓册不是批内册）', () => {
    // 批2 页清单与 utils/batch2UiContract.test.js 的 PAGES 保持一致（不 require 测试模块，防 describe 嵌套）；#1606 十四页与 batch2UiContract 十四页同册
    const BATCH2_PAGES = [
      'pages/exam-info/exam-info.uvue',
      'pages/guide/choose-cert.uvue',
      'pages/jobs/job-list.uvue',
      'pages/profile/personal-activity.uvue',
      'pages/profile/settings.uvue', 'pages/profile/personal-info.uvue', 'pages/profile/wrong-questions.uvue',
      'pages/points/points-detail.uvue', 'pages/points/task-center.uvue', 'pages/ai-assistant/ai-feature.uvue',
      'pages/ai-assistant/ai-settings.uvue', 'pages/ai-assistant/custom-models.uvue', 'pages/practice/practice.uvue',
      'pages/practice/practice-report.uvue', 'pages/jobs/job-detail.uvue', 'pages/resume/resume-edit.uvue',
      'pages/resume/resume-attach.uvue', 'pages/practice/practice-do.uvue',
    ];
    // #1607 批4（N01 蓝底套 19 页）条目落在批4页面，纳入并集扫描面；清单与 learnerUiContract.test.js 的 batch4 保持一致
    const BATCH4_PAGES = [
      'pages/courses/course-detail.uvue', 'pages/courses/chapter-view.uvue',
      'pages/resources/materials.uvue', 'pages/exam/mock-exam.uvue', 'pages/exam/mock-exam-result.uvue',
      'pages/profile/records.uvue', 'pages/profile/practice-records.uvue', 'pages/profile/mock-exam-records.uvue',
      'pages/profile/favorites.uvue', 'pages/profile/notebook.uvue', 'pages/profile/help-center.uvue',
      'pages/forum/forum-detail.uvue', 'pages/forum/forum-create.uvue', 'pages/forum/check-in.uvue',
      'pages/notifications/notifications.uvue', 'pages/resources/my-purchases.uvue', 'pages/resources/my-uploads.uvue',
      'pages/featured/featured-list.uvue', 'pages/featured/featured-detail.uvue',
    ];
    // #1608 批5（N05 透明页头 7 页）白名单条目落在批5页面，纳入并集扫描面；清单与 learnerUiContract/batch4UiContract 保持一致
    const BATCH5_PAGES = [
      'pages/dashboard/dashboard.uvue', 'pages/forum/forum.uvue', 'pages/ai-assistant/ai-assistant.uvue',
      'pages/courses/courses.uvue', 'pages/profile/profile.uvue', 'pages/recruiter/login.uvue',
      'pages/resume/resume.uvue',
    ];
    // #1609 批6（N06 无页头白名单 6 页 + mall N03 补漏）白名单条目落在批6页面，纳入并集扫描面；清单与 learnerUiContract/batch4UiContract 保持一致
    const BATCH6_PAGES = [
      'pages/index/index.uvue', 'pages/login/login.uvue', 'pages/register/register.uvue',
      'pages/forgot-password/forgot-password.uvue', 'pages/profile-setup/profile-setup.uvue',
      'pages/search/search.uvue', 'pages/mall/mall.uvue',
    ];
    const all = PAGES.concat(BATCH2_PAGES, BATCH4_PAGES, BATCH5_PAGES, BATCH6_PAGES)
      .map((p) => styleBlock(read(p)).toUpperCase())
      .join('\n');
    for (const value of Object.keys(WHITELIST)) {
      expect(all).toContain(value);
    }
  });
});
