/**
 * 学员端 UI 改造契约（#1606 批2：学员端 14 页 N03 收口 + settings §8④）
 *
 * 形态沿用批1（utils/recruiterUiContract.test.js）的五硬指标机检，落位真源
 * = #1592 终审（本批 14 页全部 N03：白底带分隔线页头）：
 *
 * 1. 声明位裸 hex ∈ token 值 ∪ §1.6 白名单（utils/uiHexWhitelist.js，逐值理由）
 * 2. `<style lang="scss">` 100%
 * 3. 手写 `.status-bar` = 0 且 app-nav-bar 引用 100%（本批无 N05 自绘豁免页）
 * 4. app-nav-bar 带 divider（N03 分隔线，ui-spec §7.1）
 * 5. 主色 #239EDD 字面值 = 0
 *
 * 判据来源：docs/ui-spec.md §5.1/§7.1/§8④/§9；#1606 票面验收标准。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** 批2 十四页（N×G 落位真源 #1592：全部 N03） */
const PAGES = [
  'pages/profile/settings.uvue',
  'pages/profile/personal-info.uvue',
  'pages/profile/wrong-questions.uvue',
  'pages/points/points-detail.uvue',
  'pages/points/task-center.uvue',
  'pages/ai-assistant/ai-feature.uvue',
  'pages/ai-assistant/ai-settings.uvue',
  'pages/ai-assistant/custom-models.uvue',
  'pages/practice/practice.uvue',
  'pages/practice/practice-report.uvue',
  'pages/jobs/job-detail.uvue',
  'pages/resume/resume-edit.uvue',
  'pages/resume/resume-attach.uvue',
  'pages/practice/practice-do.uvue',
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
  // 先剥注释（/* */ 与 // 行）——issue 引用如 #947/#1124 会伪装成 3 位 hex（ui-spec §9 只判声明位）
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

  describe(`${p}（页头归位 §7.1 N03）`, () => {
    it('引用 app-nav-bar（带 divider）且不再手写 .status-bar / 手写页头', () => {
      const s = read(p);
      expect(s).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
      expect(s).toMatch(/<app-nav-bar[^>]*divider/);
      expect(s).not.toMatch(/class="status-bar"/);
      expect(s).not.toContain('useStatusBarHeight');
    });
  });
}

describe('settings §8④ 清缓存入口回归 #1606', () => {
  it('settings.uvue 清缓存 cell 挂回 onClearCache（不再被注释）', () => {
    const src = read('pages/profile/settings.uvue');
    expect(src).toMatch(/<view class="cell" @click="onClearCache">/);
    expect(src).not.toMatch(/<!--\s*<view class="cell" @click="onClearCache">/);
    // 清缓存不得削掉续期能力（#1124 根因 5）：refresh_token 保全链路仍在
    expect(src).toContain('STORAGE_KEY_REFRESH_TOKEN');
    expect(src).toContain('uni.clearStorageSync()');
    // 恢复顺序：clear 之后必须把 refreshToken 写回
    const clearAt = src.indexOf('uni.clearStorageSync()');
    const restoreAt = src.indexOf("uni.setStorageSync(STORAGE_KEY_REFRESH_TOKEN, refreshToken as any)", clearAt);
    expect(restoreAt).toBeGreaterThan(clearAt);
  });
});

describe('批2 特殊页头形态（app-nav-bar 槽位承载）', () => {
  it('practice 右槽保留 🔍 搜索入口', () => {
    const src = read('pages/practice/practice.uvue');
    expect(src).toMatch(/<template #right>[\s\S]*?onSearch[\s\S]*?<\/template>/);
  });

  it('practice-do 计时+进度下沉固定底条（右槽腾空给胶囊宽度预算，isSingle 隐藏语义保留）', () => {
    const src = read('pages/practice/practice-do.uvue');
    // 右槽不再放两段动态文字：⏱计时+题号进度最长合计约 270rpx，超典型可用槽宽约 164rpx（顶部布局调研 §4.2；票 1847）
    expect(src).not.toMatch(/<template #right>/);
    // 两段挂到 scroll 结束后的固定底条（scroll 外 = 答题全程可见，锚定 scroll-view 关闭标签防回搬进滚动区），
    // 且仍在单题承载下隐藏
    const bar = src.match(/<\/scroll-view>[\s\S]*?class="bottom-bar"[\s\S]*?class="bottom-actions"/);
    expect(bar).not.toBeNull();
    expect(bar[0]).toContain('v-if="!isSingle"');
    expect(bar[0]).toContain('timerText');
    expect(bar[0]).toContain('currentIndex.value + 1');
  });

  it('ai-feature 右槽保留 panel-icon 抽屉入口（aiChatDrawerContract 同源交互）', () => {
    const src = read('pages/ai-assistant/ai-feature.uvue');
    expect(src).toMatch(/<template #right>[\s\S]*?toggleSidebarLeft[\s\S]*?<\/template>/);
    // statusBarHeight 是 useAiChat 的 ref，抽屉组件仍消费它——不得误删
    expect(src).toContain(':status-bar-height="statusBarHeight"');
  });

  it('job-detail 右槽保留举报入口；resume-edit 右槽保留保存；custom-models 右槽保留添加', () => {
    expect(read('pages/jobs/job-detail.uvue')).toMatch(/<template #right>[\s\S]*?onOpenReport[\s\S]*?<\/template>/);
    expect(read('pages/resume/resume-edit.uvue')).toMatch(/<template #right>[\s\S]*?onSave[\s\S]*?<\/template>/);
    expect(read('pages/ai-assistant/custom-models.uvue')).toMatch(/<template #right>[\s\S]*?startCreate[\s\S]*?<\/template>/);
  });
});

describe('§1.6 白名单文件自检（批2 扫描面）', () => {
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

  it('批1+批2+批4+批5+批6 受管页面并集内每个白名单值真实出现（防死条目）', () => {
    const batch1 = [
      'pages/recruiter/home.uvue', 'pages/recruiter/jobs.uvue', 'pages/recruiter/resumes.uvue',
      'pages/recruiter/contacts.uvue', 'pages/recruiter/applications.uvue', 'pages/recruiter/me.uvue',
      'pages/recruiter/job-edit.uvue', 'pages/recruiter/resume-detail.uvue', 'pages/recruiter/login.uvue',
    ];
    // 白名单是全仓册：#1605 批2（choose-cert/job-list 等）的条目落在 #1605 页面，纳入并集扫描面
    const batch1605 = [
      'pages/exam-info/exam-info.uvue', 'pages/guide/choose-cert.uvue',
      'pages/jobs/job-list.uvue', 'pages/profile/personal-activity.uvue',
    ];
    // #1607 批4（N01 蓝底套 19 页）条目落在批4页面，纳入并集扫描面
    const batch4 = [
      'pages/courses/course-detail.uvue', 'pages/courses/chapter-view.uvue',
      'pages/resources/materials.uvue', 'pages/exam/mock-exam.uvue', 'pages/exam/mock-exam-result.uvue',
      'pages/profile/records.uvue', 'pages/profile/practice-records.uvue', 'pages/profile/mock-exam-records.uvue',
      'pages/profile/favorites.uvue', 'pages/profile/notebook.uvue', 'pages/profile/help-center.uvue',
      'pages/forum/forum-detail.uvue', 'pages/forum/forum-create.uvue', 'pages/forum/check-in.uvue',
      'pages/notifications/notifications.uvue', 'pages/resources/my-purchases.uvue', 'pages/resources/my-uploads.uvue',
      'pages/featured/featured-list.uvue', 'pages/featured/featured-detail.uvue',
    ];
    // #1608 批5（N05 透明页头 7 页）白名单条目落在批5页面，纳入并集扫描面；清单与 recruiterUiContract/batch4UiContract 保持一致
    const batch5 = [
      'pages/dashboard/dashboard.uvue', 'pages/forum/forum.uvue', 'pages/ai-assistant/ai-assistant.uvue',
      'pages/courses/courses.uvue', 'pages/profile/profile.uvue', 'pages/recruiter/login.uvue',
      'pages/resume/resume.uvue',
    ];
    // #1609 批6（N06 无页头白名单 6 页 + mall N03 补漏）白名单条目落在批6页面，纳入并集扫描面；清单与 recruiterUiContract/batch4UiContract 保持一致
    const batch6 = [
      'pages/index/index.uvue', 'pages/login/login.uvue', 'pages/register/register.uvue',
      'pages/forgot-password/forgot-password.uvue', 'pages/profile-setup/profile-setup.uvue',
      'pages/search/search.uvue', 'pages/mall/mall.uvue',
    ];
    const all = PAGES.concat(batch1, batch1605, batch4, batch5, batch6).map((p) => styleBlock(read(p)).toUpperCase()).join('\n');
    for (const value of Object.keys(WHITELIST)) {
      expect(all).toContain(value);
    }
  });
});
