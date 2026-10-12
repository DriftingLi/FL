/**
 * 批2 UI 改造契约（#1605：N02 白底页头套 4 页收口）
 *
 * 与批1（recruiterUiContract.test.js）同构，钉住 ui-spec §9 机检五硬指标
 * 在本批 4 页上的落点。recruiter/login 已由 #1592 终审判 N05、随批1实施，
 * 不在本批（#1605 票面列它属票面笔误，用户已裁定维持 N05）。
 *
 * 1. 声明位裸 hex ∈ token 值 ∪ §1.6 白名单（utils/uiHexWhitelist.js）
 * 2. `<style lang="scss">` 100%
 * 3. 手写 `.status-bar` = 0 且页头件引用 100%
 * 4. app-nav-bar 引用 + personal-activity 右槽形态（⚙ 设置钮走 slot=right）
 * 5. 主色背景走 token（#239EDD 字面值不豁免）
 *
 * 判据来源：docs/ui-spec.md §5.1/§7.1/§9；N×G 落位真源 = #1592 终审。
 */
const h = require('./contractHarness');
const read = h.read;

const WHITELIST = require('./uiHexWhitelist');

/** 导出给批1「防死条目」自检并集使用（白名单是全仓册，不是批内册） */
const PAGES = [
  'pages/exam-info/exam-info.uvue',
  'pages/guide/choose-cert.uvue',
  'pages/jobs/job-list.uvue',
  'pages/profile/personal-activity.uvue',
];

/** token 值集合（uni.scss 的 $-变量值，归一成大写 6 位） */
const UNI_SCSS = read('uni.scss');
const tokenValues = new Set(
  [...UNI_SCSS.matchAll(/^\s*\$[\w-]+\s*:\s*(#[0-9a-fA-F]{3,8})\s*(?:!default)?\s*;/gm)].map((m) =>
    m[1].toUpperCase(),
  ),
);

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
  const style = styleBlock(src);
  const hexes = style.match(/#[0-9a-fA-F]{3,8}\b/g) || [];
  return hexes.filter((raw) => {
    const v = normalizeHex(raw);
    return !tokenValues.has(v) && !WHITELIST[v];
  });
}

for (const p of PAGES) {
  const src = read(p);

  describe(`${p}（ui-spec §9 机检 · #1605 批2）`, () => {
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

describe('批2 四页 · 页头归位（ui-spec §5.1 / §7.1 · N02 白底套）', () => {
  it('四页均引用 app-nav-bar 且不再手写 .status-bar / 手写页头', () => {
    for (const p of PAGES) {
      const src = read(p);
      expect(src).toContain("from '../../components/app-nav-bar/app-nav-bar.uvue'");
      expect(src).toContain('<app-nav-bar');
      expect(src).not.toMatch(/class="status-bar"/);
    }
  });

  it('四页均不再 import useStatusBarHeight（页头件内化后的旧接口）', () => {
    for (const p of PAGES) {
      expect(read(p)).not.toContain('useStatusBarHeight');
    }
  });

  it('personal-activity 右槽承载 ⚙ 设置钮（无返回键的标题页，slot=right 形态）', () => {
    const src = read('pages/profile/personal-activity.uvue');
    expect(src).toMatch(/<template #right>/);
    expect(src).toContain('onSettings');
    // 右槽页不再有返回钮绑定
    expect(src).not.toMatch(/<app-nav-bar[^>]*@left-click/);
  });

  it('三个有返回键的页绑定 @left-click', () => {
    for (const p of ['pages/exam-info/exam-info.uvue', 'pages/guide/choose-cert.uvue', 'pages/jobs/job-list.uvue']) {
      expect(read(p)).toMatch(/<app-nav-bar[^>]*@left-click/);
    }
  });

  it('choose-cert 渐变深端 #1565C0 保留在白名单（渐变在 uvue 支持有限，points-detail 先例）', () => {
    const src = read('pages/guide/choose-cert.uvue');
    expect(src).toMatch(/linear-gradient\(to bottom right, \$primary-color, #1565C0\)/);
    expect(WHITELIST['#1565C0']).toBeTruthy();
  });
});
