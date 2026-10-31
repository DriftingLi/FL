/**
 * 分页信封「手写存量」棘轮（移动端 ADR-0037）—— **接线守护**，不构成 ③ 门证据。
 *
 * 它守的不是解码算法对不对（那由 `api/envelope.uts` 的行为用例真执行来证），
 * 而是一条**只减不增**的接线事实：api 层还有几处在绕过信封出口、自己索引读分页字段。
 *
 * ## 口径（PR-F #1666 维护者裁定，2026-10-16）
 *
 * 1. 计数面 = `api/*.uts` 里的**索引读**（`x['total']` / `x['page_size']` / `x['pages']`）。
 *    索引**写位**（`params['page_size'] = …`）是请求侧拼参数，不是「读响应信封」，
 *    ⇒ 不计入棘轮；但写位单列一张登记表（`EXPECTED_WRITES`）并两向对账——
 *    这样「为了过正则把写位改写成对象字面量」会立刻在写位表上判红（不许把写位藏起来）。
 * 2. 终态判据拆成两件独立事实：
 *    - `page_size` / `pages` 的索引读 = **0**（这两个键只有分页信封语义，没有第二种用途）；
 *    - `total` 的索引读 = **逐处登记**的少数几个「域统计」站点（打卡累计天数、积分任务目标值、
 *      练习统计、错题统计）——`total` 是后端通用的「总数量」键，统计端点里它本来就叫 total，
 *      **不设文件级豁免**，也不许含糊地写「允许几个」：登记表与实测必须逐条相等（两向对账），
 *      多一处判红、少一处也判红（登记位不许过期）。
 * 3. 注释仍计入（raw 全文扫描，与 `pagedListAdoptionContract` 的逐行 strip 有意不同）：
 *    注释里写出被计数键字面量同样说明该文件的信封形态还在，多算方向对只减不增是安全的。
 *
 * 唯一的排除项是**判据的定义处**：`api/envelope.uts` 自己就索引读 `data['total']` 与
 * `params['page_size']` —— 它是归一出口本体，不是手写存量（判据的定义处不是消费者）。
 *
 * 读者用 `utils/utsHarness.js` 的 `readText`（ADR-0019：全仓唯一的读取层归一真源）。
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');

/** 判据的定义处：信封归一出口本体（它读 total/page_size 是职责本身），不是手写存量 */
const MODULE_FILE = 'api/envelope.uts';

/** 载体判据：三键合并扫描，读/写再分流（换着写拿不到后门）；方括号内允许空白与双引号，防「加个空格就绕过」 */
const CARRIER_RE = /\[\s*(['"])(total|page_size|pages)\1\s*\]/g;

/** 收 api 层的 .uts（信封收编面就是 api 层，页面侧归 pagedListAdoptionContract 管辖） */
const SUBJECT_RE = /\.uts$/;

/**
 * `total` 索引读的逐处登记（PR-F #1666 终态；实测 5 处）。
 *
 * 每条的 `fn` 是**直接包含该读的函数名**——登记到函数级，不是文件级：
 * 同一文件将来新增一处统计读仍会判红（新 site 不在表里）。
 */
const DOMAIN_TOTAL_READS = [
  {
    file: 'api/checkin.uts',
    fn: 'buildCheckInResult',
    why: '打卡结果 total = 全生命周期累计打卡天数（后端 CheckInResult.Total；不是分页总条数）',
  },
  {
    file: 'api/checkin.uts',
    fn: 'buildCheckInCalendarResult',
    why: '打卡日历 total = 累计打卡天数（后端 CheckInCalendarResult.Total，同上一处语义）',
  },
  {
    file: 'api/points.uts',
    fn: 'buildPointsTaskItem',
    why: '积分任务项 total = 该任务的目标值（与 progress 配成分子/分母），不是分页总条数',
  },
  {
    file: 'api/practice.uts',
    fn: 'buildPracticeStats',
    why: '练习统计 /practice-mode/stats 的真键就叫 total（后端 PracticeStatsDTO.Total），累计练习题数',
  },
  {
    file: 'api/practice.uts',
    fn: 'buildPracticeTagAccuracyList',
    why: 'by_tag 每项的 total = 该考点累计作答次数（后端 PracticeTagStat.Total，正确率的分母），不是分页总条数（#1948）',
  },
  {
    file: 'api/wrongQuestion.uts',
    fn: 'buildWrongQuestionStats',
    why: '错题统计 /wrong-questions/stats 的 total = 错题总数（后端 WrongQuestionStatsDTO.Total）',
  },
];

/**
 * 索引**写位**登记（`x['k'] = v` 形态，不计入载体）。
 *
 * 现存唯一一处：简历库筛选参数拼 `page_size`——请求侧写入，按裁定不属「读响应信封」，
 * 且**不许**为了过正则改写成 `const params = { page_size: … }` 之类（那是把写位藏起来）。
 * 写位消失也必须同步下调本表，防「悄悄删了请求参数」与「悄悄加了写位」两种漂移。
 */
const EXPECTED_WRITES = [
  { file: 'api/recruit.uts', fn: 'buildFilterParams', key: 'page_size' },
];

function walk(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const abs = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === 'unpackage') continue;
      walk(abs, out);
      continue;
    }
    const rel = path.relative(ROOT, abs).split(path.sep).join('/');
    if (!SUBJECT_RE.test(rel)) continue;
    if (rel === MODULE_FILE) continue;
    out.push(rel);
  }
  return out;
}

/** 该匹配是否处在赋值左侧（`x['k'] = v`）：`=` 但排除 `==` / `===` / `=>` */
function isAssignmentLeft(rest) {
  return /^\s*=(?![=>])/.test(rest);
}

/** 逐行向上找最近的具名函数（`function name(` / `export function name(`），找不到返回 '<top>' */
function enclosingFn(lines, fromIndex) {
  const FN_RE = /(?:export\s+)?function\s+([A-Za-z0-9_]+)\s*\(/;
  for (let i = fromIndex; i >= 0; i--) {
    const m = FN_RE.exec(lines[i]);
    if (m !== null) return m[1];
  }
  return '<top>';
}

/** 逐文件逐匹配分流成 { reads, writes }（行号从 1 起，排序稳定） */
function scanCarriers() {
  const reads = [];
  const writes = [];
  for (const rel of walk(path.join(ROOT, 'api'), []).sort()) {
    const lines = readText(path.join(ROOT, rel)).split('\n');
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      CARRIER_RE.lastIndex = 0;
      let m = CARRIER_RE.exec(line);
      while (m !== null) {
        const key = m[2];
        const entry = { file: rel, line: i + 1, key, fn: enclosingFn(lines, i), content: line.trim() };
        if (isAssignmentLeft(line.slice(m.index + m[0].length))) {
          writes.push(entry);
        } else {
          reads.push(entry);
        }
        m = CARRIER_RE.exec(line);
      }
    }
  }
  return { reads, writes };
}

/** 稳定标识：`file#fn.key`（行号会随编辑漂移，不写进登记表） */
function siteKey(entry) {
  return `${entry.file}#${entry.fn}.${entry.key}`;
}

describe('分页信封：手写存量只减不增（ADR-0037 ⑥ 终态）', () => {
  test('扫描面非空（在空集合上判绿 = 本仓防的那件事）', () => {
    // fail-closed：api/ 目录改名 / 扫描面被改窄时先红，而不是「0 处载体」报绿
    expect(walk(path.join(ROOT, 'api'), []).length).toBeGreaterThan(15);
  });

  test('判据本体仍然有效（排除 envelope.uts 不是因为它不匹配）', () => {
    const src = readText(path.join(ROOT, MODULE_FILE));
    expect(CARRIER_RE.test(src)).toBe(true);
    CARRIER_RE.lastIndex = 0;
    // 同时反证：排除是「判据定义处」这一步带来的，不是它不在扫描面里
    expect(walk(path.join(ROOT, 'api'), [])).not.toContain(MODULE_FILE);
    expect(scanCarriers().reads.map((c) => c.file)).not.toContain(MODULE_FILE);
  });

  test('page_size / pages 索引读 = 0（两键只有分页信封语义）', () => {
    const envelopeReads = scanCarriers().reads.filter((c) => c.key !== 'total');
    // 失败信息即普查表：差在哪个文件哪一行，一眼可读
    expect(envelopeReads).toEqual([]);
  });

  test('total 索引读 = 逐处登记的域统计站点（两向对账，多一处或过期一处都判红）', () => {
    const actual = scanCarriers()
      .reads.filter((c) => c.key === 'total')
      .map(siteKey)
      .sort();
    const registered = DOMAIN_TOTAL_READS.map((r) => `${r.file}#${r.fn}.total`).sort();
    expect(actual).toEqual(registered);
    // 登记表自身不许有重复（同一个函数登记两遍会让「少一处」不可见）
    expect(new Set(registered).size).toBe(registered.length);
  });

  test('索引写位与登记表一致（收紧口径不许退化成「把写位藏起来」）', () => {
    const actual = scanCarriers().writes.map(siteKey).sort();
    const registered = EXPECTED_WRITES.map((w) => `${w.file}#${w.fn}.${w.key}`).sort();
    expect(actual).toEqual(registered);
  });
});
