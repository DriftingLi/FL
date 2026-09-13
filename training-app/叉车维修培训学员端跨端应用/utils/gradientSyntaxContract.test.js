/**
 * uvue 渐变语法契约守护（#937）—— 源码契约测试缝（读源文本、断言语法形态）。
 *
 * 为什么需要它：2026-09-13 在真机上跑了 10 种写法的探针页（小米 2510DRK44C，`screencap` 无损 PNG
 * + 逐块精确取色），实测结论是 —— uvue 原生端的 `linear-gradient` **只接受「恰好 2 个颜色值、
 * 且不带百分比停靠位」**：
 *
 *   可用：`linear-gradient(135deg, #e3f0ff, #cfe4ff)`        （角度、2 值、无 %）
 *   可用：`linear-gradient(to bottom, #FF0000, #0000FF)`      （关键字、2 值、无 %）
 *   丢弃：`linear-gradient(135deg, #FF1493 0%, #FF1493 100%)` （带 %）
 *   丢弃：`linear-gradient(to bottom, #F00, #FF0, #0F0, #00F)`（4 值）
 *
 * 两条容易搞反的事实，别再复述错：
 *   ① **角度合法** —— 实测 `135deg` 画得出来 ⇒ **不要**把「不得出现 `deg`」写成守护规则；
 *   ② **失败是整条声明被丢弃** —— 连同一处的 `background-color` 兜底也不生效（实测该显示兜底色
 *      的几块全是纯白）⇒ 兜底是好实践，但**不是**静默失败的保护网，只能靠语法本身合规。
 *
 * 本测试把「语法形态」锁死，让后续会话不会再写回不可用的写法（#937 之前全项目 22 处里 15 处中招，
 * 而且**不报错不警告**地静默失败）。
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/deviceCaptureContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');

/** 颜色值个数上限；uvue 只接受 color-start / color-stop 两个 */
const MAX_COLOR_STOPS = 2;

/** 抽取 <style> 块内容（渐变只可能出现在样式里；模板/脚本里出现这个词基本都是注释） */
function styleTextOf(source) {
  const blocks = source.match(/<style[^>]*>[\s\S]*?<\/style>/g) || [];
  return blocks.join('\n');
}

/** 去掉 HTML 注释与 CSS 注释 —— 注释里写着 `linear-gradient` 不算实现（假阳性来源） */
function stripComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/\/\*[\s\S]*?\*\//g, ' ');
}

/**
 * 取一段 CSS 里所有 `linear-gradient(...)` 调用的「顶层」参数。
 * **不能**用 `[^)]*` 抓参数 —— 遇到 `rgba(0, 0, 0, .5)` 会在它的 `)` 处截断，
 * 把一个合法的两值写法误判成「4 个颜色值」（实测过的假阳性）。
 */
function gradientCalls(css) {
  const calls = [];
  const marker = 'linear-gradient(';
  let idx = css.indexOf(marker);
  while (idx !== -1) {
    const start = idx + marker.length;
    let depth = 1;
    let i = start;
    for (; i < css.length && depth > 0; i++) {
      const ch = css[i];
      if (ch === '(') depth += 1;
      else if (ch === ')') depth -= 1;
    }
    if (depth !== 0) break; // 括号不配平：不猜，停止解析
    const args = css.slice(start, i - 1).trim();
    // 按**顶层**逗号切分（括号内的逗号不算，如 rgba 的通道分隔）
    const parts = [];
    let buf = '';
    let d = 0;
    for (const ch of args) {
      if (ch === '(') d += 1;
      else if (ch === ')') d -= 1;
      if (ch === ',' && d === 0) { parts.push(buf.trim()); buf = ''; } else { buf += ch; }
    }
    if (buf.trim()) parts.push(buf.trim());
    calls.push({ args, parts });
    idx = css.indexOf(marker, i);
  }
  return calls;
}

/** 纯函数：单个文件源码 → 违规清单 */
function scanGradients(source) {
  const violations = [];
  const css = stripComments(styleTextOf(source));
  for (const { args, parts } of gradientCalls(css)) {
    // 方向有两种写法 —— 关键字（`to bottom` / `to bottom right`，含空格）与**角度**（`135deg`）。
    // 两者都实测可用（#937 探针 A4 就是角度），都不计入颜色值个数。
    const DIRECTION = /^(to\s+(right|left|top|bottom)(\s+(right|left))?|\d+(\.\d+)?deg)$/;
    const hasDirection = parts.length > 0 && DIRECTION.test(parts[0]);
    const stops = hasDirection ? parts.slice(1) : parts;

    if (/%/.test(stops.join(','))) {
      violations.push(
        '带百分比停靠位（实测整条声明被丢弃）：linear-gradient(' + args + ')'
      );
    }
    if (stops.length !== MAX_COLOR_STOPS) {
      violations.push(
        '颜色值个数 ' + stops.length + '（uvue 只接受恰好 ' + MAX_COLOR_STOPS + ' 个）：linear-gradient(' + args + ')'
      );
    }
  }
  // 背景图只支持 linear-gradient，且统一走简写 background（长写形态在全项目已收敛为 0 处）
  if (/background-image\s*:\s*linear-gradient/.test(css)) {
    violations.push('用 background-image 长写声明渐变（本仓统一走简写 background）');
  }
  // 兜底必须写在简写**之后**：CSS 简写 `background` 会把未列出的 `background-color`
  // 重置为初始值，写在它前面的兜底等于没有（#937 复查发现）。
  // 注意 `[^}]*` —— 必须限制在**同一条规则块内**，否则会把前一条规则的 background-color
  // 与后一条规则的 background 误判成一对（实测过的假阳性）。
  if (/background-color\s*:\s*#[0-9A-Fa-f]{3,8}\s*;[^}]*?background\s*:\s*linear-gradient/.test(css)) {
    violations.push('实色兜底写在 background 简写之前 —— 会被简写重置，等于没有兜底');
  }
  return violations;
}

/** 递归收集目标 .uvue（跳过构建产物与依赖） */
function collectUvue(dir, acc = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return acc;
  }
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name === 'unpackage' || e.name === 'node_modules') continue;
      collectUvue(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

describe('uvue 渐变语法契约（#937 真机实测口径）', () => {
  // ---- 注入自检：检测器必须能抓住违规，否则本测试是空跑假绿 ----
  describe('① 检测器自检（注入违规样本必须被判红）', () => {
    const cases = [
      ['三值 + 百分比（本仓原页底写法）',
        '<style>.a { background: linear-gradient(180deg, #CFE9FB 1%, #D0EBFD 16%, #F5F5F5 100%); }</style>',
        /百分比停靠位[\s\S]*颜色值个数 3/],
      ['两值但带百分比（探针 A1 写法）',
        '<style>.a { background: linear-gradient(135deg, #FF1493 0%, #FF1493 100%); }</style>',
        /百分比停靠位/],
      ['四值无百分比（探针 B3 写法，实测同样被丢弃）',
        '<style>.a { background: linear-gradient(to bottom, #FF0000, #FFFF00, #00FF00, #0000FF); }</style>',
        /颜色值个数 4/],
      ['background-image 长写',
        '<style>.a { background-image: linear-gradient(135deg, #e3f0ff, #cfe4ff); }</style>',
        /长写声明渐变/],
      ['兜底写在简写之前（会被 background 重置，等于没有兜底）',
        '<style>.a { background-color: #1B5E20;\n  background: linear-gradient(135deg, #e3f0ff, #cfe4ff); }</style>',
        /兜底写在 background 简写之前/],
    ];
    cases.forEach(([name, src, pattern]) => {
      it(name, () => {
        const v = scanGradients(src);
        expect(v.length).toBeGreaterThan(0);
        expect(v.join('\n')).toMatch(pattern);
      });
    });
  });

  // ---- 自检的另一半：合规写法与「注释里出现该词」不得误报 ----
  describe('② 检测器自检（合规样本必须判绿，防误报）', () => {
    const ok = [
      ['角度 + 两值 + 无百分比（探针 A4 实测可用）',
        '<style>.a { background: linear-gradient(135deg, #e3f0ff, #cfe4ff); }</style>'],
      ['关键字 + 两值 + 无百分比（探针 B1 实测可用）',
        '<style>.a { background: linear-gradient(to bottom, #FF0000, #0000FF); }</style>'],
      ['两词方向（to bottom right）',
        '<style>.a { background: linear-gradient(to bottom right, #CFE9FB, #F5F5F5); }</style>'],
      ['只有本行兜底、没有渐变',
        '<style>.a { background-color: #F5F5F5; }</style>'],
      ['注释里出现该词不算实现',
        '<style>.a { background-color: #fff; /* 实色：本机型不绘制 linear-gradient(...) */ }</style>'],
      ['含 rgba() 的两值写法（括号内逗号不得被当成颜色值分隔）',
        '<style>.a { background: linear-gradient(135deg, rgba(0,0,0,.5), rgba(255,255,255,1)); }</style>'],
    ];
    ok.forEach(([name, src]) => {
      it(name, () => {
        expect(scanGradients(src)).toEqual([]);
      });
    });
  });

  // ---- 对真实文件断言零命中 ----
  it('③ 全项目 .uvue 的渐变声明均为「恰好 2 个颜色值 + 无百分比 + 简写 background」', () => {
    const files = [
      ...collectUvue(path.join(ROOT, 'pages')),
      ...collectUvue(path.join(ROOT, 'components')),
      path.join(ROOT, 'App.uvue'),
    ].filter((p) => { try { return fs.statSync(p).isFile(); } catch { return false; } });

    expect(files.length).toBeGreaterThan(50);   // 锚点自检：扫描面必须真的覆盖到，不许退化成空跑

    const offenders = [];
    for (const f of files) {
      const v = scanGradients(fs.readFileSync(f, 'utf8'));
      if (v.length) offenders.push(path.relative(ROOT, f) + '\n    ' + v.join('\n    '));
    }
    expect(offenders.join('\n')).toBe('');
  });
});
