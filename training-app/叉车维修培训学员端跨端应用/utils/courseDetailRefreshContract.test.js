/**
 * #1126 契约：课程详情页必须能从「返回」路径刷新进度（取数走 onShow，不是 onLoad）。
 *
 * ## 为什么需要这条守护
 *
 * 症状：标记完成某章后 `navigateBack` 返回课程详情，进度条**仍是陈旧的 0%**；
 * 只有退到首页重新进入（页面重建）才显示正确值。
 *
 * 根因：课程详情页只在 `onLoad` 取数，而 `onLoad` 在页面生命周期**只触发一次**；
 * `navigateBack` 返回时页面实例仍存活 ⇒ `onLoad` 不再执行 ⇒ 界面停留在进入时的值。
 *
 * 真机实证（设备 b32d8398）：章节点「标记本章完成」后后端立即为
 * `progress=14.29, completed=1/7`，但返回课程详情实拍仍为 `0% / 已完成 0/7 章`。
 *
 * ## 断言的产物
 *
 * 1. `loadDetail` 由 **onShow** 驱动（返回时会重新取数）；
 * 2. `onLoad` **不再**自己取数 —— 因为 `onShow` 首次进入同样触发，
 *    两个钩子都取数会让首个请求**翻倍**（单一取数入口）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
/** 归一 CRLF（工作树里的 .uvue 是 CRLF，不归一则按行锚定的断言会失效） */
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');

const PAGE = 'pages/courses/course-detail.uvue';

/** 取 `<marker>` 之后的函数体，以花括号配平判收尾（见 #1111 契约测试里记的三个坑） */
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

describe('#1126 取数入口：onShow 驱动刷新，onLoad 不重复取数', () => {
  const src = read(PAGE);

  it('从 uni-app 引入了 onShow', () => {
    expect(src).toMatch(/import\s*\{[^}]*\bonShow\b[^}]*\}\s*from\s*'@dcloudio\/uni-app'/);
  });

  it('存在 onShow 钩子，且其中真的调用 loadDetail', () => {
    const body = bodyAfter(src, 'onShow(');
    expect(body).not.toBe('');
    expect(body).toContain('loadDetail()');
  });

  it('onShow 里对 courseId 做了守卫（未拿到 id 时不发请求）', () => {
    const body = bodyAfter(src, 'onShow(');
    expect(body).toMatch(/courseId\.value\s*>\s*0/);
  });

  it('onLoad 只记 courseId、**不再**自己取数（否则首个请求翻倍）', () => {
    const body = bodyAfter(src, 'onLoad(');
    expect(body).not.toBe('');
    expect(body).toContain('courseId.value');
    expect(body).not.toContain('loadDetail()');
  });

  it('loadDetail 仍在（取数函数本身未被删）', () => {
    expect(src).toMatch(/async\s+function\s+loadDetail\s*\(/);
  });
});
