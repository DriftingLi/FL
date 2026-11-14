/**
 * 网络图必须开 webp 属性 —— 源码契约守护（#2543）。
 *
 * 为什么必须机检：这条故障在**开发者工具里永远看不见** —— 工具跑的是 Chromium，包内 webp 与
 * 网络 webp 都正常渲染；只有真机 / 体验版的小程序运行时不解析。
 * 2026-11-14 实测：体验版整屏空白，而工具一切正常，只能靠真机二分才定位。
 *
 * 口径（`docs/adr/0066-…md` ⑦）：
 *   ① 包内 webp 一律不解析（微信 image 的 `webp` 属性只对**网络**资源生效）⇒ 静态资源用 PNG；
 *   ② 网络图默认**不解析** webp ⇒ 喂动态 `:src` 的 `<image>` 必须显式 `:webp="true"`。
 *      后端 `filestore` 把 jpg/png/bmp 一律转 `image/webp` 存，所以封面 / 头像 / 论坛图 / 验证码
 *      这些服务端图默认全是网络 webp —— 漏一个就是一处真机空白。
 *
 * 本守护守的是 ② 的**接线**面：动态 src 的 `<image>` 是否带 `:webp="true"`。
 * 行为面无从机检（渲染发生在小程序运行时），这是这类平台差异的唯一廉价防线 —— 如实声明：
 * 它不证明「真机上真的显示出来了」，只证明「喂网络图的那 38 处都声明了」。
 *
 * 设计沿用本仓既有守护形态（`utils/iconGlyphContract.test.js`、`utils/uvueEntityLiteralContract.test.js`）：
 * 先对「注入违规」的样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');

/** 源码目录（构建产物与依赖不算源码） */
const SOURCE_DIRS = ['pages', 'components', 'utils', 'composables', 'constants', 'api', 'stores', 'types'];

/** 递归收集 `.uvue`（模板就在里面；跳过构建产物、依赖、工作树留档） */
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
      if (SKIP_DIRS.has(e.name)) continue;
      collectUvue(full, acc);
    } else if (e.name.endsWith('.uvue')) {
      acc.push(full);
    }
  }
  return acc;
}

const SKIP_DIRS = new Set(['unpackage', 'node_modules', '.git', '.ci-verify', '.scratch']);

function relOf(abs) {
  return path.relative(ROOT, abs).split(path.sep).join('/');
}

/**
 * 逐个扫 `<image …>` 开标签，返回属性对象。
 *
 * 不能用「到第一个 `>` 就停」的正则：`v-if="avatar.length > 0"` 里的 `>` 会把标签切成两半
 * （#2543 实测：第一版脚本因此只命中 20/38）。这里跟踪引号状态，只在**引号外**的 `>` 收尾。
 */
function findImageTags(src) {
  const out = [];
  const open = /<image(?=[\s/>])/g;
  let m;
  while ((m = open.exec(src)) !== null) {
    let i = m.index + m[0].length;
    let quote = null;
    while (i < src.length) {
      const ch = src[i];
      if (quote) {
        if (ch === quote) quote = null;
      } else if (ch === '"' || ch === "'") {
        quote = ch;
      } else if (ch === '>') {
        break;
      }
      i += 1;
    }
    out.push({ text: src.slice(m.index, i + 1), line: src.slice(0, m.index).split('\n').length });
  }
  return out;
}

/** 从开标签文本里取属性表；`:` 前缀原样保留（`:src` 与 `src` 是两回事） */
function parseAttrs(tagText) {
  const attrs = {};
  const re = /([:@\w.-]+)\s*=\s*("[^"]*"|'[^']*')/g;
  let m;
  while ((m = re.exec(tagText)) !== null) {
    const name = m[1];
    const raw = m[2];
    attrs[name] = raw.slice(1, -1);
  }
  return attrs;
}

/** 静态包内路径（`/static/...`）—— 不需要 webp 属性，因为包内 webp 本来就不解析、一律用 PNG */
function refersToStaticPath(expr) {
  return /(\/static\/)/.test(expr);
}

/**
 * 违规清单：喂**动态** src、且不指向包内 `/static/`、却没有 `:webp="true"` 的 `<image>`。
 * 纯静态 `src="/static/xxx.png"` 的标签天然豁免（包内图不进这条口径）。
 */
function scanMissingWebp(src) {
  const offenders = [];
  for (const tag of findImageTags(src)) {
    const attrs = parseAttrs(tag.text);
    const dynamicSrc = attrs[':src'];
    if (dynamicSrc === undefined) continue;
    if (attrs.src !== undefined) continue;
    if (refersToStaticPath(dynamicSrc)) continue;
    if (attrs[':webp'] === 'true' || attrs.webp === 'true') continue;
    offenders.push(`L${tag.line}  ${tag.text.replace(/\s+/g, ' ').trim()}`);
  }
  return offenders;
}

function allUvueFiles() {
  const out = [];
  for (const d of SOURCE_DIRS) out.push(...collectUvue(path.join(ROOT, d)));
  for (const f of ['App.uvue']) {
    const p = path.join(ROOT, f);
    if (fs.existsSync(p)) out.push(p);
  }
  return out;
}

// ---------------------------------------------------------------- 测试

describe('image 网络图 webp 属性契约（#2543）', () => {
  // ---- 先证检测器有效：注入违规必须被抓到（否则「零命中」可能只是没在扫） ----
  describe('检测器自证（防空跑假绿）', () => {
    it('动态 src 缺 :webp ⇒ 抓到', () => {
      expect(scanMissingWebp('<image class="a" :src="item.cover_url" mode="aspectFill" />')).toHaveLength(1);
    });

    it('动态 src 带 :webp="true" ⇒ 放过', () => {
      expect(
        scanMissingWebp('<image class="a" :src="item.cover_url" mode="aspectFill" :webp="true" />'),
      ).toHaveLength(0);
    });

    it('属性里的 `>` 不会把标签截断（v-if 含比较运算）', () => {
      const src = '<image v-if="avatar.length > 0" class="b" :src="avatar" mode="aspectFill" />';
      expect(scanMissingWebp(src)).toHaveLength(1); // 截断的旧正则会漏掉这一处
      expect(
        scanMissingWebp('<image v-if="avatar.length > 0" :src="avatar" :webp="true" />'),
      ).toHaveLength(0);
    });

    it('静态包内路径豁免（包内走 PNG，不吃 webp 属性）', () => {
      expect(scanMissingWebp('<image class="c" src="/static/logo.png" mode="aspectFit" />')).toHaveLength(0);
      expect(scanMissingWebp('<image class="c" :src="ok ? \'/static/icons/star-filled.png\' : \'/static/icons/star.png\'" />')).toHaveLength(0);
    });

    it(':src 与 src 不是同一个属性（`src` 的子串不能被当成动态 src）', () => {
      expect(scanMissingWebp('<image class="d" src="/static/logo.png" />')).toHaveLength(0);
    });
  });

  // ---- 再对真实文件断言：本票交付判据 ----
  describe('全仓网络图零漏配', () => {
    const files = allUvueFiles();

    it('扫描面有效（防 walker 静默失效）', () => {
      expect(files.length).toBeGreaterThan(80);
      // 本票实际改动的承载文件必须在扫描面内
      for (const rel of [
        'pages/forum/components/forum-topic-card.uvue',
        'pages/mall/mall.uvue',
        'pages/login/login.uvue',
        'components/app-list-item/app-list-item.uvue',
      ]) {
        expect(files.map(relOf)).toContain(rel);
      }
    });

    it('没有一处喂网络图的 <image> 漏写 :webp="true"', () => {
      const offenders = [];
      for (const f of files) {
        const v = scanMissingWebp(readText(f));
        if (v.length) offenders.push(relOf(f) + '\n    ' + v.join('\n    '));
      }
      expect(offenders.join('\n')).toBe('');
    });

    it('至少扫到 30 处已开 webp 的网络图（防「注释掉整片模板」式假绿）', () => {
      let n = 0;
      for (const f of files) {
        for (const tag of findImageTags(readText(f))) {
          const attrs = parseAttrs(tag.text);
          if (attrs[':src'] !== undefined && attrs[':webp'] === 'true') n += 1;
        }
      }
      expect(n).toBeGreaterThanOrEqual(30);
    });
  });
});
