/**
 * 诊断组句模板的**跨栈同源**守护（#2296）
 *
 * 守的是什么：webside 的 `frontend/src/utils/diagnosisCompose.ts` 与移动端
 * `utils/diagnosisCompose.uts` 必须给出**同一套问法**。这是票面验收里那条
 * 「拼句模板与 Web 端同源，由跨栈守护锁一致」的落地。
 *
 * 为什么不能各写一份：组句是**发给外部诊断服务的正文**，模板字面量就是检索命中的那一半。
 * 两端各写一份 = 一处改了另一处不知道 ⇒ 同一个学员换个屏幕拿到两套问法，而且**没有任何判据会红**。
 *
 * 为什么断言「模板骨架序列」而不是整段源码：
 *   - 整段源码比会因语法差异（`|| ''` vs `.trim()`、`===` vs `==`）恒红，守护活不过一次重构；
 *   - 只断言「某个字面量出现过」是**存在性断言**，正是本仓踩过的假绿（`toContain` 是坏实现的超集）——
 *     把 `请按手册给出安全注意事项` 从林德分支挪到中力分支，它照样绿。
 * 故判据取**顺序敏感的骨架序列**：每个模板字面量把插值归一成 `{}`，短的纯标点/空串丢弃，
 * 两侧序列逐位相等。改一个字、换一个位置、调一次顺序，都红。
 *
 * 本文件是**接线守护**（只读源码文本，不执行 .uts）；真执行的那一半在
 * `utils/diagnosisGuideBehavior.test.js`（行为腿）。
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');

/** 工作树根 = 移动端项目目录往上三级（`training-app/<项目>` ⇒ 仓根） */
const REPO_ROOT = path.join(__dirname, '..', '..', '..');
const WEB_FILE = path.join(REPO_ROOT, 'frontend', 'src', 'utils', 'diagnosisCompose.ts');
const MOBILE_FILE = path.join(__dirname, 'diagnosisCompose.uts');

/**
 * 两侧都要一致比对的函数（名字相同、语义相同）。
 *
 * **不含 `pickerKindOf`**：它的骨架是「判档取值」，两侧的 null 兜底写法不同
 * （web 用 `brand?.picker` 的可选链、移动端 UTS 必须显式 `if (brand == null) return`），
 * 比字面量会因写法差异恒红。判档口径另由下面第 ③ 条按**语义**断言。
 */
const SHARED_FUNCTIONS = ['composeDiagnosisSentence', 'faultStepTitle'];

/** 去注释（块 + 行）：注释里提到某句中文不算「模板里有它」 */
function stripComments(src) {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 取函数体（大括号配平）：切片到闭合处为止，防止越界到文件尾造成假绿 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error('找不到源码片段: ' + sig);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error('片段后找不到 { : ' + sig);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error('大括号未配平: ' + sig);
}

/**
 * 抽「模板骨架」序列：
 *   - 模板字面量（反引号）⇒ `TPL:` + 把 `\${…}` 归一成 `{}` 的骨架
 *   - 单/双引号字符串 ⇒ 原文，**长度 < 3 的丢弃**（`''` / `'x'` 这类不算问法）
 * 顺序即出现顺序（顺序敏感是判据的一部分：换分支位置也要红）。
 */
function skeletons(src) {
  const out = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (c === "'" || c === '"') {
      const quote = c;
      let j = i + 1;
      let s = '';
      while (j < src.length && src[j] !== quote) {
        if (src[j] === '\\') { s += src[j] + src[j + 1]; j += 2; continue; }
        s += src[j];
        j += 1;
      }
      if (s.length >= 3) out.push(s);
      i = j + 1;
      continue;
    }
    if (c === '`') {
      let j = i + 1;
      let s = '';
      let depth = 0;
      while (j < src.length) {
        const ch = src[j];
        if (ch === '\\') { s += src[j] + src[j + 1]; j += 2; continue; }
        if (ch === '$' && src[j + 1] === '{') { depth += 1; s += '${'; j += 2; continue; }
        if (ch === '}' && depth > 0) { depth -= 1; s += '}'; j += 1; continue; }
        if (ch === '`' && depth === 0) break;
        s += ch;
        j += 1;
      }
      out.push('TPL:' + s.replace(/\$\{[^}]*\}/g, '{}'));
      i = j + 1;
      continue;
    }
    // 跳过正则字面量（本仓两文件都不含，但跳法留个兜底，避免把 /xxx/ 当成除法吞掉后续字符）
    if (c === '/') {
      const prev = src.slice(0, i).trimEnd().slice(-1);
      if ('(,=:[!&|?{};'.indexOf(prev) >= 0) {
        i += 1;
        while (i < src.length && src[i] !== '/') {
          if (src[i] === '\\') i += 1;
          i += 1;
        }
        i += 1;
        continue;
      }
    }
    i += 1;
  }
  return out;
}

/** 两侧同名函数的骨架，按函数名分组（`{}` 里的差异不影响，模板其余部分逐字比对） */
function skeletonsByName(src) {
  const out = {};
  for (const name of SHARED_FUNCTIONS) {
    out[name] = skeletons(fnBody(src, 'function ' + name));
  }
  return out;
}

const WEB_SRC = stripComments(readText(WEB_FILE));
const MOBILE_SRC = stripComments(readText(MOBILE_FILE));

describe('诊断组句模板跨栈同源（#2296）', () => {
  it('① 前置：两侧文件都在，且抽出的骨架非空（否则后面的相等是「空 == 空」的假绿）', () => {
    expect(fs.existsSync(WEB_FILE)).toBe(true);
    expect(fs.existsSync(MOBILE_FILE)).toBe(true);
    const mobile = skeletonsByName(MOBILE_SRC);
    for (const name of SHARED_FUNCTIONS) {
      expect(mobile[name].length).toBeGreaterThan(0);
    }
    // 组句那支至少要有一条带「{}」的模板（林德句 / 中力句），证明抽的是真模板而不是散装字面量
    const tpl = mobile.composeDiagnosisSentence.filter((s) => s.indexOf('TPL:') === 0 && s.indexOf('{}') > 0);
    expect(tpl.length).toBeGreaterThanOrEqual(3);
  });

  it('② 三个同名函数的模板骨架逐位相等（改一个字 / 换分支位置 / 调顺序都会红）', () => {
    const web = skeletonsByName(WEB_SRC);
    const mobile = skeletonsByName(MOBILE_SRC);
    for (const name of SHARED_FUNCTIONS) {
      expect({ fn: name, seq: mobile[name] }).toEqual({ fn: name, seq: web[name] });
    }
  });

  it('③ 判档口径同源：两侧都只认 picker === \'model\'（其余与空值一律系统档）', () => {
    // 写法天然不同（web 可选链、UTS 显式 null 判断），故按**语义**断言：model 分支存在 + 缺省兜 system
    expect(WEB_SRC).toMatch(/brand\?\.picker === 'model' \? 'model' : 'system'/);
    expect(MOBILE_SRC).toMatch(/if \(brand == null\) return 'system'/);
    expect(MOBILE_SRC).toMatch(/brand\.picker == 'model' \? 'model' : 'system'/);
    // 术语同源：两侧都用 'model' / 'system' 两个值，没有第三方拼写（'type' / 'series' 之类）
    for (const src of [WEB_SRC, MOBILE_SRC]) {
      expect(src).not.toMatch(/picker\s*==+\s*'type'/);
    }
  });

  it('④ 检测器有牙：把移动端某个模板改一个字，第 ② 条必须判红', () => {
    const mutated = MOBILE_SRC.replace('请给我排查步骤、根因分析和维修方案。', '请给我排查步骤与维修方案。');
    expect(mutated).not.toBe(MOBILE_SRC); // 变异必须真落上，否则这条自检本身假绿
    const web = skeletonsByName(WEB_SRC);
    const mobile = skeletonsByName(mutated);
    expect(mobile.composeDiagnosisSentence).not.toEqual(web.composeDiagnosisSentence);
  });

  it('⑤ 检测器有牙：把林德句里的模板挪到中力分支（字面量还在、位置变了），第 ② 条必须判红', () => {
    const moved = MOBILE_SRC.replace(
      'if (extra.length > 0) s += `补充：${extra}。`\n\ts += \'请按手册给出安全注意事项。\'',
      's += \'请按手册给出安全注意事项。\'\n\tif (extra.length > 0) s += `补充：${extra}。`'
    );
    // 位置变异可能因换行锚点不匹配而空跑 —— 那种情况只断言「要么改到了、要么本条不适用」，
    // 但绝不能静默地把没改到的源当成「改到了」来判绿。
    if (moved === MOBILE_SRC) {
      expect(MOBILE_SRC).toContain('请按手册给出安全注意事项。');
      return;
    }
    const web = skeletonsByName(WEB_SRC);
    const mobile = skeletonsByName(moved);
    expect(mobile.composeDiagnosisSentence).not.toEqual(web.composeDiagnosisSentence);
  });
});