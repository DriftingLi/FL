#!/usr/bin/env node
/**
 * UI 形态盘点（#1957 的证据生成器，只读）。
 *
 * 三块输出，全部可复跑：
 *   1) 封装面：components/ui 下 35 个组件的使用面 + 模板里每个 el-* 控件的裸用量；
 *   2) 容器面：页面本地的卡片类（容器 vs 卡片内部件）+ UiCard / el-card 的用法；
 *   3) 视觉档位：scoped 里 border-radius / box-shadow / padding / border 的取值分布，
 *      以及模板里 rounded-* / shadow-* 工具类的精确 token 计数。
 *
 * 用法：node scripts/inventory-ui-forms.mjs [--json <path>]
 * 退出码：0 正常；2 读不到 frontend/src。
 */
import { readdirSync, readFileSync, statSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';

const ROOT = process.cwd();
const SRC = join(ROOT, 'frontend', 'src');
if (!existsSync(SRC)) {
  console.error('找不到 ' + SRC + '（请在仓库根目录运行）');
  process.exit(2);
}

const EXCLUDE = new Set(['__tests__', 'node_modules', 'dist']);
const WATCHED = new Set([
  'border-radius', 'border-top-left-radius', 'border-top-right-radius',
  'border-bottom-left-radius', 'border-bottom-right-radius',
  'box-shadow', 'padding', 'border'
]);
/** 卡片内部件（标题 / 头 / 列表 …）不是容器，别计入「本地卡片类」。 */
const CARD_PART = /(title|header|body|footer|list|item|more|empty|desc|name|grid|icon|content|inner|wrap|toolbar|tag|badge|count|meta|num|stat-?s)s?$/;

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (EXCLUDE.has(name)) continue;
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (name.endsWith('.vue')) out.push(full);
  }
  return out;
}

const rel = (p) => relative(ROOT, p).replace(/\\/g, '/');
const relSrc = (p) => relative(SRC, p).replace(/\\/g, '/');
const norm = (v) => v.replace(/\s+/g, ' ').trim();
const nl = (s) => (s.match(/\n/g) || []).length;

function templateBlock(src) {
  const start = src.indexOf('<template>');
  const end = src.lastIndexOf('</template>');
  if (start < 0 || end <= start) return { tpl: '', base: 1 };
  return { tpl: src.slice(start, end), base: nl(src.slice(0, start)) + 1 };
}

function tags(tpl, base) {
  const out = [];
  const re = /<([a-zA-Z][a-zA-Z0-9-]*)((?:"[^"]*"|'[^']*'|[^>"'])*)>/g;
  let m;
  while ((m = re.exec(tpl))) out.push({ name: m[1], attrs: m[2], line: base + nl(tpl.slice(0, m.index)) });
  return out;
}

function styleDecls(src) {
  const out = [];
  const re = /<style([^>]*)>([\s\S]*?)<\/style>/g;
  let m;
  while ((m = re.exec(src))) {
    const baseLine = nl(src.slice(0, m.index + m[0].indexOf('>') + 1)) + 1;
    const css = m[2].replace(/\/\*[\s\S]*?\*\//g, (s) => s.replace(/[^\n]/g, ' '));
    const ruleRe = /([^{}]+)\{([^{}]*)\}/g;
    let r;
    while ((r = ruleRe.exec(css))) {
      const selector = norm(r[1]);
      const bodyStart = r.index + r[1].length + 1;
      const declRe = /([-a-zA-Z]+)\s*:\s*([^;]+)/g;
      let d;
      while ((d = declRe.exec(r[2]))) {
        out.push({
          scoped: /\bscoped\b/.test(m[1] || ''),
          selector,
          prop: d[1].toLowerCase(),
          value: norm(d[2]),
          line: baseLine + nl(css.slice(0, bodyStart + d.index))
        });
      }
    }
  }
  return out;
}

/** class / :class 里的 token（含对象语法里的键）。 */
function classTokens(tpl) {
  const out = [];
  const re = /(?:^|\s):?class="([^"]*)"/g;
  let m;
  while ((m = re.exec(tpl))) {
    for (const raw of m[1].split(/[\s,{}'"]+/)) {
      const t = raw.replace(/:$/, '');
      if (t) out.push(t);
    }
  }
  return out;
}

const files = walk(SRC);
const elCensus = new Map();
const wrapperUse = new Map();
const decls = [];
const tokens = new Map();
const localCards = [];
const cardUsage = [];
const nativeButtons = [];

for (const file of files) {
  const src = readFileSync(file, 'utf8');
  const r = relSrc(file);
  const { tpl, base } = templateBlock(src);
  const tagList = tags(tpl, base);

  for (const t of tagList) {
    if (t.name.startsWith('el-')) {
      const cur = elCensus.get(t.name) || { total: 0, outside: [] };
      cur.total += 1;
      if (!r.startsWith('components/ui/')) cur.outside.push(r + ':' + t.line);
      elCensus.set(t.name, cur);
    }
    if (/^Ui[A-Z]/.test(t.name) || t.name === 'ThemeToggle') {
      const cur = wrapperUse.get(t.name) || { uses: 0, files: new Set() };
      cur.uses += 1;
      cur.files.add(r);
      wrapperUse.set(t.name, cur);
    }
    if (t.name === 'button') nativeButtons.push({ file: r, line: t.line, attrs: norm(t.attrs).slice(0, 120) });
    if (t.name === 'UiCard' || t.name === 'el-card') cardUsage.push({ tag: t.name, file: r, line: t.line, attrs: norm(t.attrs).slice(0, 160) });
  }

  for (const tok of classTokens(tpl)) {
    if (/^(rounded|shadow)/.test(tok)) tokens.set(tok, (tokens.get(tok) || 0) + 1);
  }

  const local = new Set();
  for (const d of styleDecls(src)) {
    if (WATCHED.has(d.prop)) decls.push({ ...d, file: r });
    if (d.scoped) {
      const cm = /\.([a-zA-Z0-9_-]*(?:card|panel|tile)[a-zA-Z0-9_-]*)/.exec(d.selector);
      if (cm && !CARD_PART.test(cm[1]) && !local.has(cm[1])) local.add(cm[1]);
    }
  }
  for (const cls of local) {
    const hits = tagList.filter((t) => {
      const cm = /(?:^|\s):?class="([^"]*)"/.exec(t.attrs);
      return cm ? cm[1].split(/\s+/).includes(cls) : false;
    });
    if (!hits.length) continue;
    const own = {};
    for (const d of decls) {
      if (d.file !== r || !WATCHED.has(d.prop)) continue;
      if (new RegExp('^\\.' + cls + '($|[.:\\s>])').test(d.selector)) own[d.prop] = d.value;
    }
    localCards.push({
      file: r,
      cls: '.' + cls,
      hosts: hits.map((h) => ({ name: h.name, line: h.line, clickable: /(^|\s)(@click|v-on:click)/.test(h.attrs) })),
      style: own
    });
  }
}

const group = (rows, key) => {
  const map = new Map();
  for (const row of rows) {
    const k = row[key];
    if (!map.has(k)) map.set(k, []);
    map.get(k).push(row);
  }
  return map;
};

const style = {};
for (const [prop, rows] of group(decls, 'prop')) {
  const byValue = {};
  for (const [value, list] of group(rows, 'value')) {
    byValue[value] = list.map((x) => ({ file: x.file, line: x.line, selector: x.selector, scoped: x.scoped }));
  }
  style[prop] = byValue;
}

const result = {
  scanned: files.length,
  wrappers: [...wrapperUse.entries()]
    .map(([name, v]) => ({ name, uses: v.uses, files: [...v.files].sort() }))
    .sort((a, b) => b.uses - a.uses || a.name.localeCompare(b.name)),
  elementPlus: [...elCensus.entries()]
    .map(([tag, v]) => ({ tag, total: v.total, outside: v.outside.length, where: v.outside }))
    .sort((a, b) => b.total - a.total || a.tag.localeCompare(b.tag)),
  nativeButtons,
  cards: { usage: cardUsage, local: localCards },
  utilities: Object.fromEntries([...tokens.entries()].sort((a, b) => b[1] - a[1])),
  style
};

const jsonIdx = process.argv.indexOf('--json');
if (jsonIdx > 0 && process.argv[jsonIdx + 1]) {
  const target = process.argv[jsonIdx + 1];
  mkdirSync(dirname(target), { recursive: true });
  writeFileSync(target, JSON.stringify(result, null, 2) + '\n', 'utf8');
  console.log('json → ' + target);
}

const u = (prefix) => Object.entries(result.utilities).filter(([k]) => k.startsWith(prefix));
const radius = style['border-radius'] || {};
const shadow = style['box-shadow'] || {};
const containers = localCards;
console.log('扫描 ' + files.length + ' 个 .vue');
console.log('封装层在用组件 ' + result.wrappers.length + ' / 35；本地卡片容器 ' + containers.length +
  ' 个（可点击 ' + containers.filter((c) => c.hosts.some((h) => h.clickable)).length + '）');
console.log('UiCard ' + cardUsage.filter((c) => c.tag === 'UiCard').length + ' 处 / el-card ' +
  cardUsage.filter((c) => c.tag === 'el-card').length + ' 处；原生 <button> ' + nativeButtons.length + ' 处');
console.log('scoped 取值：border-radius ' + Object.keys(radius).length + ' 种、box-shadow ' +
  Object.keys(shadow).length + ' 种；工具类 rounded-* ' + u('rounded').length + ' 种 / ' +
  u('rounded').reduce((s, [, n]) => s + n, 0) + ' 处');
console.log('裸用 el-*（封装层之外）：' + result.elementPlus
  .filter((e) => e.outside > 0)
  .slice(0, 8)
  .map((e) => e.tag + '=' + e.outside)
  .join(' ') + ' …');
/** --md <path>：把四项属性的逐条取值清单写成 markdown 附录（供 docs/design 引用）。 */
if (process.argv.includes('--md')) {
  const target = process.argv[process.argv.indexOf('--md') + 1];
  const ORDER = ['border-radius', 'box-shadow', 'padding', 'border'];
  const lines = [];
  lines.push('# 1957 附录 · scoped 取值逐条清单');
  lines.push('');
  lines.push('> 由 scripts/inventory-ui-forms.mjs --md 生成，**不要手改**。');
  lines.push('> 口径：<style scoped>（含 @media 内）的声明；括号里是出现次数。');
  lines.push('');
  for (const prop of ORDER) {
    const values = style[prop] || {};
    const total = Object.values(values).reduce((s, l) => s + l.length, 0);
    lines.push('## ' + prop + '（' + Object.keys(values).length + ' 种取值 / ' + total + ' 处）');
    lines.push('');
    lines.push('| 取值 | 次数 | 落点（文件:行） |');
    lines.push('| --- | --- | --- |');
    for (const [value, list] of Object.entries(values).sort((a, b) => b[1].length - a[1].length)) {
      const where = list.map((x) => x.file.replace('frontend/src/', '') + ':' + x.line).join('<br>');
      lines.push('| ' + value + ' | ' + list.length + ' | ' + where + ' |');
    }
    lines.push('');
  }
  writeFileSync(target, lines.join('\n'), 'utf8');
  console.log('md → ' + target);
}
