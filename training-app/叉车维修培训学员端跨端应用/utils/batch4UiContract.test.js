/**
 * 批4 UI 改造契约（#1607：N01 蓝底页头套 19 页收口 + check-in §8③ 去 mock）
 *
 * 形态沿用批1/批2/批3 的五硬指标机检，落位真源 = #1595 台账 + #1607 票面：
 *
 * 1. 声明位裸 hex ∈ token 值 ∪ §1.6 白名单（utils/uiHexWhitelist.js）
 * 2. `<style lang="scss">` 100%
 * 3. 手写 `.status-bar` = 0 且 app-nav-bar 引用 100%（本批全部 inverse 蓝底套）
 * 4. app-nav-bar 带 variant="inverse"（N01：$primary-color 底、状态栏同色、反白标题）
 * 5. 主色 #239EDD 字面值 = 0
 *
 * 随批收口（§8③）：check-in 页 mock 降级链整条清除——
 * 无 loadMockCalendar / usingMock，加载失败置 loadError 显式失败态（可重试），
 * 自动打卡在失败态下绝不触发（打卡静默失败同堵，失败 toast）。
 *
 * 判据来源：docs/ui-spec.md §7.1/§9；#1607 票面验收标准。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** 批4 十九页（N01 蓝底套，S01 簇 20 − my-forum，#1595 台账） */
const PAGES = [
  'pages/courses/course-detail.uvue',
  'pages/courses/chapter-view.uvue',
  'pages/resources/materials.uvue',
  'pages/exam/mock-exam.uvue',
  'pages/exam/mock-exam-result.uvue',
  'pages/profile/records.uvue',
  'pages/profile/practice-records.uvue',
  'pages/profile/mock-exam-records.uvue',
  'pages/profile/favorites.uvue',
  'pages/profile/notebook.uvue',
  'pages/profile/help-center.uvue',
  'pages/forum/forum-detail.uvue',
  'pages/forum/forum-create.uvue',
  'pages/forum/check-in.uvue',
  'pages/notifications/notifications.uvue',
  'pages/resources/my-purchases.uvue',
  'pages/resources/my-uploads.uvue',
  'pages/featured/featured-list.uvue',
  'pages/featured/featured-detail.uvue',
];

/** token 值集合（uni.scss 的 $-变量值，归一成大写 6 位） */
const UNI_SCSS = read('uni.scss');
const tokenValues = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)].map((m) =>
    m[1].toUpperCase(),
  ),
);

/** 归一：3 位 hex 折成 6 位、大写 */
function normalizeHex(hex) {
  const h6 = hex.length === 4
    ? hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3]
    : hex.slice(1, 7);
  return `#${h6}`.toUpperCase();
}

function styleBlock(src) {
  const start = src.indexOf('<style');
  if (start < 0) return '';
  return src.slice(start, src.indexOf('</style>'));
}

function hexViolations(src) {
  // 先剥注释（/* */ 与 // 行）——issue 引用如 #947/#1140 会伪装成 3 位 hex（ui-spec §9 只判声明位）
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

  describe(`${p}（页头归位 §7.1 N01 蓝底套）`, () => {
    it('引用 app-nav-bar（variant=inverse）且不再手写 .status-bar / 手写页头', () => {
      const s = read(p);
      expect(s).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
      expect(s).toMatch(/<app-nav-bar[^>]*variant="inverse"/);
      expect(s).not.toMatch(/class="status-bar"/);
      expect(s).not.toContain('useStatusBarHeight');
    });
  });
}

describe('批4 特殊页右槽形态（app-nav-bar 槽位承载）', () => {
  it('course-detail/chapter-view 右槽保留 toggleFavorite 收藏入口', () => {
    for (const p of ['pages/courses/course-detail.uvue', 'pages/courses/chapter-view.uvue']) {
      expect(read(p)).toMatch(/<template #right>[\s\S]*?toggleFavorite[\s\S]*?<\/template>/);
    }
    // ♡ 字形不能空串（#1140 教训，承重注释随行保留）
    expect(read('pages/courses/chapter-view.uvue')).toContain('#1140');
  });

  it('mock-exam 右槽保留倒计时（warning/danger 分级样式不动）', () => {
    const src = read('pages/exam/mock-exam.uvue');
    expect(src).toMatch(/<template #right>[\s\S]*?timerDisplay[\s\S]*?<\/template>/);
    expect(src).toContain('timer-text');
  });

  it('notebook 右槽保留新建入口；notifications 右槽保留全部已读', () => {
    expect(read('pages/profile/notebook.uvue')).toMatch(/<template #right>[\s\S]*?onCreate[\s\S]*?<\/template>/);
    expect(read('pages/notifications/notifications.uvue')).toMatch(/<template #right>[\s\S]*?onMarkAllRead[\s\S]*?<\/template>/);
  });

  it('forum-detail 右槽三态保留（删除/编辑/举报互斥 v-if 全搬入槽）', () => {
    const src = read('pages/forum/forum-detail.uvue');
    expect(src).toMatch(/<template #right>[\s\S]*?canDelete[\s\S]*?onEditTopic[\s\S]*?onReportTopicClick[\s\S]*?<\/template>/);
  });

  it('featured-detail 右槽保留收藏 ⭐/☆ 切换', () => {
    expect(read('pages/featured/featured-detail.uvue')).toMatch(/<template #right>[\s\S]*?onToggleFavorite[\s\S]*?<\/template>/);
  });
});

describe('check-in §8③ 去 mock（#1607 随批收口）', () => {
  it('mock 降级链整条清除：无 loadMockCalendar / usingMock 残留', () => {
    const src = read('pages/forum/check-in.uvue');
    expect(src).not.toContain('loadMockCalendar');
    expect(src).not.toContain('usingMock');
  });

  it('加载失败显式化：loadError 置位 + 失败态 UI + onRetryLoad 重试入口', () => {
    const src = read('pages/forum/check-in.uvue');
    expect(src).toContain('loadError');
    expect(src).toContain('onRetryLoad');
    expect(src).toMatch(/<view v-if="loadError" class="error-state">/);
    // 四块数据 UI 在失败态下隐藏（banner/月条/星期头/日历格）
    expect(src).toMatch(/<view v-if="!loadError" class="checkin-banner">/);
    expect(src).toMatch(/<view v-if="!loadError" class="month-bar">/);
    expect(src).toMatch(/<view v-if="!loadError" class="calendar-grid">/);
  });

  it('自动打卡洞已堵：失败态下绝不自动打卡（!loadError 守卫）', () => {
    const src = read('pages/forum/check-in.uvue');
    expect(src).toMatch(/if \(!loadError\.value && !todayChecked\.value && !autoCheckInTried\.value\)/);
    expect(src).not.toMatch(/!usingMock\.value/);
  });

  it('打卡失败不再静默：catch 带 toast 显式报错', () => {
    const src = read('pages/forum/check-in.uvue');
    expect(src).toMatch(/checkIn failed[\s\S]*?uni\.showToast/);
  });
});

describe('§1.6 白名单文件自检（批4 扫描面）', () => {
  it('白名单每个值都有理由且格式合法', () => {
    for (const [value, reason] of Object.entries(WHITELIST)) {
      expect(value).toMatch(/^#[0-9A-F]{6}$/);
      expect(reason.length).toBeGreaterThan(10);
    }
  });

  it('白名单值不得与 token 值重叠（重叠即该值应直接走 token；例外：#F5F5F5 渐变尾色语义不符）', () => {
    const SEMANTIC_MISMATCH = new Set(['#F5F5F5']);
    for (const value of Object.keys(WHITELIST)) {
      if (SEMANTIC_MISMATCH.has(value)) continue;
      expect(tokenValues.has(value)).toBe(false);
    }
  });

  it('批1+批1605+批3+批4+批5+批6 受管页面并集内每个白名单值真实出现（防死条目）', () => {
    const batch1 = [
      'pages/recruiter/home.uvue', 'pages/recruiter/jobs.uvue', 'pages/recruiter/resumes.uvue',
      'pages/recruiter/contacts.uvue', 'pages/recruiter/applications.uvue', 'pages/recruiter/me.uvue',
      'pages/recruiter/job-edit.uvue', 'pages/recruiter/resume-detail.uvue', 'pages/recruiter/login.uvue',
    ];
    const batch1605 = [
      'pages/exam-info/exam-info.uvue', 'pages/guide/choose-cert.uvue',
      'pages/jobs/job-list.uvue', 'pages/profile/personal-activity.uvue',
    ];
    const batch3 = [
      'pages/profile/settings.uvue', 'pages/profile/personal-info.uvue', 'pages/profile/wrong-questions.uvue',
      'pages/points/points-detail.uvue', 'pages/points/task-center.uvue', 'pages/ai-assistant/ai-feature.uvue',
      'pages/ai-assistant/ai-settings.uvue', 'pages/ai-assistant/custom-models.uvue', 'pages/practice/practice.uvue',
      'pages/practice/practice-report.uvue', 'pages/jobs/job-detail.uvue', 'pages/resume/resume-edit.uvue',
      'pages/resume/resume-attach.uvue', 'pages/practice/practice-do.uvue',
    ];
    // #1608 批5（N05 透明页头 7 页）白名单条目落在批5页面，纳入并集扫描面；清单与 recruiterUiContract/learnerUiContract 保持一致
    const batch5 = [
      'pages/dashboard/dashboard.uvue', 'pages/forum/forum.uvue', 'pages/ai-assistant/ai-assistant.uvue',
      'pages/courses/courses.uvue', 'pages/profile/profile.uvue', 'pages/recruiter/login.uvue',
      'pages/resume/resume.uvue',
    ];
    // #1609 批6（N06 无页头白名单 6 页 + mall N03 补漏）白名单条目落在批6页面，纳入并集扫描面；清单与 learnerUiContract/recruiterUiContract 保持一致
    const batch6 = [
      'pages/index/index.uvue', 'pages/login/login.uvue', 'pages/register/register.uvue',
      'pages/forgot-password/forgot-password.uvue', 'pages/profile-setup/profile-setup.uvue',
      'pages/search/search.uvue', 'pages/mall/mall.uvue',
    ];
    const all = PAGES.concat(batch1, batch1605, batch3, batch5, batch6).map((p) => styleBlock(read(p)).toUpperCase()).join('\n');
    for (const value of Object.keys(WHITELIST)) {
      expect(all).toContain(value);
    }
  });
});
