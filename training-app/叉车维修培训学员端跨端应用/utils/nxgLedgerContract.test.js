/**
 * N×G 台账契约守护（#2834；判据面 = #2829 Q6 裁定「台账入仓」+ #2715「不立判红门」的边界）
 *
 * 钉住的是「台账与路由清单彼此不漂移」——台账（docs/design/20261125-移动端61页N×G归属台账.md）
 * 是本仓 N×G 全表的唯一真源，docs/ui-spec.md §7.3 只留指针。三组断言：
 * - A 集合相等（双向）：台账的页面路径集合 === pages.json 的路由集合，且行数相等；
 *   台账多一行（凭空写的页）与少一行（漏登记的页）都判红，并点名是哪条。
 * - B 归属合法 + 计数↔规范一致：N 列只认 N01/N02/N03/N05/N06（N04 已并入 N03，编号留洞），
 *   G 列只认 G1–G9；逐页数出来的成员数必须等于 ui-spec §7.1 / §7.2 表里登记的成员数——
 *   这是防「台账改了、规范没改」的第三处漂移面（前两处是 §7.3 指针与 §0 文档地图）。
 * - C 指针在场：ui-spec 的 §7.3 段必须指向本台账文件；指针被摘掉（退回「本文不复制 #1595」）
 *   即判红，否则规范与台账会各自成说。
 *
 * 分类：接线守护（读两份真源文本做一致性断言，无代码级执行调用，分类器落 wiring）；
 * guards.md 末节三问在 PR 正文作答。
 *
 * 边界（#2715）：本套件**不是判红门**——不进 CI 必检、不挂 ruleset，属「文档与配置一致性」
 * 的接线守护。它管的是「两份文本对不对得上」，不是样式判据（间距/颜色那类机检门另议）。
 *
 * 更新指引：新增 / 下线页面 → 同一次提交改 pages.json + 台账逐页表 + ui-spec §7.1/§7.2 成员数；
 * 只改一处的样本在下方「注入必红」组里就是反例。改扫描函数后三组必须保持全绿。
 */
const fs = require('fs');
const path = require('path');

const h = require('./contractHarness');
const read = h.read;

const ROOT = path.join(__dirname, '..');

/** 仓根靠 .git 上溯找，不写死层数：工作树里 ../../ 与主树里 ../../ 不是同一个地方。 */
function findRepoRoot(from) {
  let d = from;
  for (let i = 0; i < 6; i += 1) {
    if (fs.existsSync(path.join(d, '.git'))) return d;
    d = path.dirname(d);
  }
  throw new Error('找不到 git 根（台账在根仓库 docs/design/ 下，不在移动端工程内）');
}

const REPO = findRepoRoot(ROOT);
const LEDGER_REL = 'docs/design/20261125-移动端61页N×G归属台账.md';

const PAGES_JSON = read('pages.json');
const UI_SPEC = read('docs/ui-spec.md');
/** 台账读文本同样经共享读者归一 EOL（ADR-0019：不得新写第二份 normalizeEol） */
const { readText } = require('./utsHarness');
const LEDGER = readText(path.join(REPO, LEDGER_REL));

const N_SETS = ['N01', 'N02', 'N03', 'N05', 'N06'];
const G_SETS = ['G1', 'G2', 'G3', 'G4', 'G5', 'G6', 'G7', 'G8', 'G9'];
/** 台账 §3 表头关键字：限定只解析逐页表，不误吃 §1 的套定义表 */
const LEDGER_TABLE_HEADER = '| # | 页面路径 | N 套 | G 套 |';

/** 表行拆格：以 | 开头的行 → 去掉首尾空段后的单元格数组；非表行回 null */
function tableCells(line) {
  if (!line.trim().startsWith('|')) return null;
  const parts = line.split('|');
  return parts.slice(1, -1).map((c) => c.trim());
}

/** 剥 markdown 修饰（反引号 / 粗体星号） */
function stripMd(cell) {
  return cell.replace(/[`*]/g, '').trim();
}

/** pages.json 的路由全集：pages[] 直挂 + subPackages[].root + path（口径同 dashboardPage/examContract）。 */
function routesFromPages(pagesText) {
  const conf = JSON.parse(pagesText);
  const main = (conf.pages || []).map((p) => p.path);
  const subs = (conf.subPackages || []).flatMap((sp) => (sp.pages || []).map((p) => sp.root + '/' + p.path));
  return main.concat(subs);
}

/** 从台账 §3 逐页表抽出 {path, N, G, lines} 行；行号留作报错坐标。 */
function parseLedgerRows(ledgerText) {
  const rows = [];
  const lines = ledgerText.split('\n');
  let inTable = false;
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    if (!inTable) {
      if (line.trim().startsWith(LEDGER_TABLE_HEADER)) inTable = true;
      continue;
    }
    const cells = tableCells(line);
    if (!cells) {
      if (line.trim() === '') continue;
      break; // 表结束
    }
    if (cells.length < 4) continue;
    if (/^-+$/.test(cells[0].replace(/:/g, ''))) continue; // 分隔线
    if (!/^\d+$/.test(cells[0])) continue; // 非数据行
    rows.push({
      line: i + 1,
      path: stripMd(cells[1]),
      N: stripMd(cells[2]),
      G: stripMd(cells[3]),
      lines: stripMd(cells[5] || ''),
    });
  }
  return rows;
}

/** A + B（合法性部分）：集合双向相等、行数相等、路径唯一、N/G 取值合法。 */
function scanLedger(routes, ledgerText) {
  const rows = parseLedgerRows(ledgerText);
  const violations = [];
  const routeSet = new Set(routes);
  const seen = new Set();

  for (const r of rows) {
    if (!r.path.startsWith('pages/')) {
      violations.push('台账第 ' + r.line + ' 行路径不是 pages/ 相对形式：' + r.path);
      continue;
    }
    if (seen.has(r.path)) violations.push('台账重复登记：' + r.path + '（第 ' + r.line + ' 行）');
    seen.add(r.path);
    if (!routeSet.has(r.path)) {
      violations.push('台账登记了 pages.json 里不存在的页：' + r.path + '（第 ' + r.line + ' 行；新增页要同批写进 pages.json）');
    }
    if (!N_SETS.includes(r.N)) {
      violations.push('台账第 ' + r.line + ' 行 N 列越枚举：' + r.N + '（只认 ' + N_SETS.join('/') + '；N04 已并入 N03，编号留洞不重排）');
    }
    if (!G_SETS.includes(r.G)) {
      violations.push('台账第 ' + r.line + ' 行 G 列越枚举：' + r.G + '（只认 ' + G_SETS.join('/') + '）');
    }
  }
  for (const p of routes) {
    if (!seen.has(p)) violations.push('pages.json 的路由未在台账登记：' + p + '（下线页要同批从台账删行）');
  }
  if (rows.length !== routes.length) {
    violations.push('行数不等：台账 ' + rows.length + ' 行 vs pages.json ' + routes.length + ' 条路由');
  }
  return { rows, violations };
}

/** B（计数部分）：逐页数出的成员数 === ui-spec §7.1 / §7.2 表内登记的成员数。 */
function parseSpecCounts(uiSpecText, headerText) {
  const lines = uiSpecText.split('\n');
  const out = new Map();
  let inTable = false;
  for (const line of lines) {
    if (!inTable) {
      if (line.trim().startsWith(headerText)) inTable = true;
      continue;
    }
    const cells = tableCells(line);
    if (!cells) {
      if (line.trim() === '') continue;
      break;
    }
    const key = stripMd(cells[0]);
    if (!/^[NG]\d+$/.test(key)) continue;
    // §7.1 成员数在第 4 格、§7.2 在第 2 格：取「第一个以数字开头的格」（N06 那格是
    // 「5：`index` / `login` …」连带名单，故用前缀匹配而不是全等）
    const numCell = cells.slice(1).map(stripMd).find((c) => /^\d+/.test(c));
    if (numCell === undefined) continue;
    out.set(key, Number(numCell.match(/^\d+/)[0]));
  }
  return out;
}

function countBy(rows, field) {
  const out = new Map();
  for (const r of rows) out.set(r[field], (out.get(r[field]) || 0) + 1);
  return out;
}

function scanSpecCounts(uiSpecText, rows) {
  const violations = [];
  const specs = [
    ['## 7.1', '| 套 | 定义 | 底色 |', 'N'],
    ['## 7.2', '| 套 | 成员数（', 'G'],
  ];
  for (const [label, header, field] of specs) {
    const declared = parseSpecCounts(uiSpecText, header);
    if (declared.size === 0) {
      violations.push('ui-spec ' + label + ' 成员数表解析为空：表头「' + header + '」未命中（改表头要同批改本守护）');
      continue;
    }
    const actual = countBy(rows, field);
    for (const [set, n] of actual) {
      if (!declared.has(set)) {
        violations.push('台账有 ' + set + '（' + n + ' 页），ui-spec ' + label + ' 表未登记该套');
      } else if (declared.get(set) !== n) {
        violations.push(set + ' 成员数不一致：台账逐页数出 ' + n + ' 页，ui-spec ' + label + ' 写 ' + declared.get(set));
      }
    }
    for (const [set, n] of declared) {
      if (!actual.has(set) && n !== 0) {
        violations.push('ui-spec ' + label + ' 登记 ' + set + ' ' + n + ' 页，台账里该套 0 页');
      }
    }
  }
  return violations;
}

/** C：ui-spec §7.3 段必须指向台账文件（摘掉指针即判红）。 */
function scanSpecPointer(uiSpecText) {
  const lines = uiSpecText.split('\n');
  const start = lines.findIndex((l) => l.trim().startsWith('### 7.3'));
  if (start < 0) return ['ui-spec 找不到 §7.3 段（标题「### 7.3」缺失，指针面失效）'];
  let body = '';
  for (let i = start + 1; i < lines.length; i += 1) {
    if (/^#{2,3} /.test(lines[i])) break;
    body += lines[i] + '\n';
  }
  if (!body.includes('20261125-移动端61页N×G归属台账.md')) {
    return ['ui-spec §7.3 未指向台账 docs/design/20261125-移动端61页N×G归属台账.md（唯一真源必须留指针）'];
  }
  return [];
}

/** 在台账逐页表表头后插一行（注入样本用，不动真文件） */
function insertLedgerRow(ledgerText, row) {
  const lines = ledgerText.split('\n');
  const at = lines.findIndex((l) => l.trim().startsWith(LEDGER_TABLE_HEADER));
  if (at < 0) throw new Error('注入失败：台账找不到逐页表表头');
  // 表头 + 分隔线之后插入
  lines.splice(at + 2, 0, row);
  return lines.join('\n');
}

describe('nxgLedgerContract（#2834）：台账 61 行 ↔ pages.json 路由 · N/G 归属 ↔ ui-spec §7.1/§7.2', () => {
  const ROUTES = routesFromPages(PAGES_JSON);
  const real = scanLedger(ROUTES, LEDGER);

  describe('真库必绿 + 锚点（防解析面空跑假绿）', () => {
    it('台账零违规（集合双向相等 + 行数相等 + N/G 取值合法）', () => {
      expect(real.violations).toEqual([]);
    });

    it('锚点：台账解析出 ≥40 行（防空跑；真值由上一组断言钉死）', () => {
      expect(real.rows.length).toBeGreaterThanOrEqual(40);
    });

    it('逐页数出的成员数 === ui-spec §7.1 / §7.2 登记值', () => {
      expect(scanSpecCounts(UI_SPEC, real.rows)).toEqual([]);
    });

    it('ui-spec §7.3 指向台账（唯一真源指针在场）', () => {
      expect(scanSpecPointer(UI_SPEC)).toEqual([]);
    });
  });

  describe('注入必红（检测器自检：改坏样本必须判红，防空跑假绿）', () => {
    it('台账漏登记一条路由 → 判红且点名该页', () => {
      const victim = real.rows[0].path;
      const doc = LEDGER.replace(/^\| 1 \|.*$/m, '');
      const v = scanLedger(ROUTES, doc).violations;
      expect(v.some((x) => x.includes('未在台账登记') && x.includes(victim))).toBe(true);
    });

    it('台账凭空多一行（pages.json 无此路由）→ 判红', () => {
      const doc = insertLedgerRow(LEDGER, '| 999 | `pages/ghost/ghost` | N02 | G3 | — | 100 | 注入样本 |');
      const v = scanLedger(ROUTES, doc).violations;
      expect(v.some((x) => x.includes('不存在的页') && x.includes('pages/ghost/ghost'))).toBe(true);
    });

    it('N 列越枚举 → 判红（N04 是留洞编号，不得回流）', () => {
      // 只改逐页表里的归属格（§1 的套定义表也有「| N03 |」，不能误伤）
      const doc = LEDGER.replace('| N03 | G', '| N04 | G');
      expect(scanLedger(ROUTES, doc).violations.some((x) => x.includes('N 列越枚举') && x.includes('N04'))).toBe(true);
    });

    it('台账改了 N 归属但没同批改 §7.1 → 判红', () => {
      const rows = real.rows.map((r, i) => (i === 0 ? { ...r, N: r.N === 'N01' ? 'N02' : 'N01' } : r));
      expect(scanSpecCounts(UI_SPEC, rows).some((x) => x.includes('成员数不一致'))).toBe(true);
    });

    it('§7.3 指针被摘掉 → 判红', () => {
      const doc = UI_SPEC.replace(
        '**全表（61 页 × N×G 归属）真源 = `docs/design/20261125-移动端61页N×G归属台账.md`**',
        '**全表（61 页 × N×G 归属）真源 = #1595 六十页台账**'
      );
      expect(scanSpecPointer(doc).some((x) => x.includes('未指向台账'))).toBe(true);
    });
  });

  describe('对称注入必绿（两侧一致新增必须放行，防「永远红」假守护）', () => {
    it('pages.json 与台账同批各加一页 → 零违规', () => {
      const conf = JSON.parse(PAGES_JSON);
      conf.pages.push({ path: 'pages/ghost/ghost', style: { navigationBarTitleText: '注入样本' } });
      const pagesText = JSON.stringify(conf, null, 2);
      const ledgerText = insertLedgerRow(LEDGER, '| 999 | `pages/ghost/ghost` | N02 | G3 | — | 100 | 对称注入样本 |');
      expect(scanLedger(routesFromPages(pagesText), ledgerText).violations).toEqual([]);
    });
  });
});
