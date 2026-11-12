/**
 * 全局件可用性契约（#2444）
 *
 * 背景（#2436 取证 → #2438 裁定）：8 个 `app-*` 全局件里有 6 个全仓 0 引用，
 * 其中 5 个 `<style>` 缺 `lang="scss"`、4 个引用了 uni.scss 里根本不存在的变量短名
 * （`$text-inverse` / `$text-secondary` / `$text-placeholder`）——
 * 即便有人开始用它们，样式也只会静默失效。裁定是「复用现有 + 补 easycom，不引入第三方库」，
 * 所以本文件钉的是「它们真的能用」，不是它们的视觉。
 *
 * 判据来源：四条红线（docs/agents/uvue-css.md）、ADR-0007「easycom 仅全局件」、#2438 裁定。
 */
const h = require('./contractHarness');

/** easycom autoscan 的目录约定是 `components/<name>/<name>.uvue`；这里是 `app-*` 家族 8 件。 */
const APP_COMPONENTS = [
  'app-badge',
  'app-button',
  'app-card',
  'app-chip',
  'app-empty-state',
  'app-list-item',
  'app-nav-bar',
  'app-tabs',
];

/**
 * 缺 `lang` 的 style 开标签：`<style>` / `<style scoped>` 都算，`<style lang="scss">` 不算。
 * 必须锚行首 —— 行内注释里出现「`// … <style> …`」这类说明会被误判（app-nav-bar 就是这种）。
 */
function styleTagViolations(src) {
  return /^\s*<style(?![^>]*\blang\s*=)[^>]*>/m.test(src);
}

/** uni.scss 的真名是 `$text-color-*`；这三条短名全仓不存在，写上去只是编译期空值。 */
const WRONG_VARS = /\$(?:text-inverse|text-secondary|text-placeholder)\b/;

describe('全局件可用性（#2444）', () => {
  it('A: 8 个 app-* 全局件都在 components/<name>/<name>.uvue（autoscan 的目录约定）', () => {
    for (const name of APP_COMPONENTS) {
      expect(h.exists(`components/${name}/${name}.uvue`)).toBe(true);
    }
  });

  it('B: 8 个 app-* 全局件的 <style> 全部带 lang="scss"（四条红线之一）', () => {
    const bad = APP_COMPONENTS.filter((name) =>
      styleTagViolations(h.read(`components/${name}/${name}.uvue`)));
    expect(bad).toEqual([]);
  });

  it('C: components/ 与 pages/ 下没有引用 uni.scss 里不存在的短名变量', () => {
    const hits = [];
    for (const dir of ['components', 'pages']) {
      for (const rel of h.sourceFilesIn(dir)) {
        if (WRONG_VARS.test(h.read(rel))) hits.push(rel);
      }
    }
    expect(hits).toEqual([]);
  });

  it('D: pages.json 显式声明 easycom.autoscan（ADR-0007：easycom 仅全局件）', () => {
    const pages = JSON.parse(h.read('pages.json'));
    expect(pages.easycom).toBeTruthy();
    expect(pages.easycom.autoscan).toBe(true);
  });

  // 判别力：判据不能恒真——真塞一个坏样本必须被抓出来
  it('E: 判别力 —— 裸 <style> 与错名变量都会被 B/C 的判据抓到', () => {
    expect(styleTagViolations('<style>\n.a { color: red; }\n</style>')).toBe(true);
    expect(styleTagViolations('<style scoped>\n.a {}\n</style>')).toBe(true);
    expect(styleTagViolations('<style lang="scss">\n.a {}\n</style>')).toBe(false);

    expect(WRONG_VARS.test('.a { color: $text-inverse; }')).toBe(true);
    expect(WRONG_VARS.test('.a { color: $text-secondary; }')).toBe(true);
    expect(WRONG_VARS.test('.a { color: $text-placeholder; }')).toBe(true);

    // 正名不得误伤
    expect(WRONG_VARS.test('.a { color: $text-color-inverse; }')).toBe(false);
    expect(WRONG_VARS.test('.a { color: $text-color-secondary; }')).toBe(false);
    expect(WRONG_VARS.test('.a { color: $text-color-placeholder; }')).toBe(false);
  });
});
