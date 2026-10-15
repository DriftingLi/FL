/**
 * 分页信封「手写存量」棘轮（移动端 ADR-0037）—— **接线守护**，不构成 ③ 门证据。
 *
 * 它守的不是解码算法对不对（那由 `utils/envelopeBehavior.test.js` 真执行模块来证），
 * 而是一条**只减不增**的接线事实：api 层还有几处在绕过信封出口、自己索引读分页字段。
 *
 * 判据：api/*.uts 里出现 `['total']` / `['page_size']` / `['pages']` 索引读，每处算一个载体。
 * 三个键**合并计数、不分彼此**——`page_size` 与 `pages` 换着写拿不到后门（反向锁）。
 * `pages→page_size` 误读家族（api/forum.uts:343 病根）的消亡路径就是本棘轮的终态：基线 0。
 *
 * 口径说明（与 pagedListAdoptionContract 的逐行 strip 注释**有意不同**）：
 * 这里用 **raw 全文 regex 计数（含注释）**。理由：注释里的分页字段（如 course.uts 文档注释
 * `{ courses: [...], total, page, pages }`）描述的正是该文件的信封形态，是迁移施工图的合法组成；
 * strip 注释会让「改个注释」与「迁一处载体」在计数上不可区分，口径反而失真。raw 计数只会**多算**
 * 不会漏算——对只减不增的棘轮而言，多算方向是安全的。基线与实现必须同口径现测（见 BASELINE 注）。
 *
 * 口径是**两向对账**（先例 `utils/pagedListAdoptionContract.test.js`）：计数必须**恰好等于**
 * `ENVELOPE_BASELINE` —— 新增一处判红（棘轮不许回潮），迁走一处而不下调常量也判红（登记位不许过期）。
 * **不设 allowlist**：豁免机制已由 ADR-0023 决策⑧ / #654 整体退役，别再带回来。
 *
 * 唯一的排除项是**判据的定义处**：`api/envelope.uts` 自己就索引读 `data['total']` 与
 * `params['page_size']` —— 它是归一出口本体，不是手写存量（先例：判据的定义处不是消费者）。
 *
 * 读者用 `utils/utsHarness.js` 的 `readText`（ADR-0019：全仓唯一的读取层归一真源）。
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');

/** 判据的定义处：信封归一出口本体（它读 total/page_size 是职责本身），不是手写存量 */
const MODULE_FILE = 'api/envelope.uts';

/**
 * 载体的实测基线（PR-A #1661 迁移后现测）：迁移前 raw 50 处（forum.uts 占 2，含 :343 误读站点）；
 * forum 首刀清零后 raw 50，其中定义文件 envelope.uts 自身 2 处经排除不计 ⇒ **48**。
 * 迁移一处 ⇒ **同一 PR 下调本数**；新增一处 ⇒ 判红。终态 = 0（ADR-0037 ⑥）。
 */
const ENVELOPE_BASELINE = 48;

/** 载体判据：三键合并、不分彼此（`page_size` 与 `pages` 换着写拿不到后门） */
const CARRIER_RE = /\['(total|page_size|pages)'\]/g;

/** 收 api 层的 .uts（信封收编面就是 api 层，页面侧归 pagedListAdoptionContract 管辖） */
const SUBJECT_RE = /\.uts$/;

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

/** 逐文件逐匹配收集载体 {file, line, content}（行号从 1 起，排序稳定） */
function envelopeCarriers() {
  const carriers = [];
  for (const rel of walk(path.join(ROOT, 'api'), []).sort()) {
    const lines = readText(path.join(ROOT, rel)).split('\n');
    for (let i = 0; i < lines.length; i++) {
      CARRIER_RE.lastIndex = 0;
      while (CARRIER_RE.exec(lines[i]) !== null) {
        carriers.push({ file: rel, line: i + 1, content: lines[i].trim() });
      }
    }
  }
  return carriers;
}

describe('分页信封：手写存量只减不增（ADR-0037）', () => {
  test('扫描面非空（在空集合上判绿 = 本仓防的那件事）', () => {
    // fail-closed：api/ 目录改名 / 扫描面被改窄时先红，而不是「0 处载体」报绿
    expect(walk(path.join(ROOT, 'api'), []).length).toBeGreaterThan(15);
  });

  test(`载体计数必须恰好等于 ${ENVELOPE_BASELINE}（新增判红；迁走一处须同步下调本数，终态 0）`, () => {
    const carriers = envelopeCarriers();
    // 失败信息即普查表：差几处、差在哪个文件哪一行，一眼可读
    expect({ count: carriers.length, carriers }).toEqual({
      count: ENVELOPE_BASELINE,
      carriers,
    });
    expect(carriers.length).toBe(ENVELOPE_BASELINE);
  });

  test('模块本体不被算作手写存量（判据的定义处不是消费者）', () => {
    expect(envelopeCarriers().map((c) => c.file)).not.toContain(MODULE_FILE);
    // 反证：模块本体**确实**索引读分页字段 —— 排除它是有意的，不是因为它不匹配
    const src = readText(path.join(ROOT, MODULE_FILE));
    expect(CARRIER_RE.test(src)).toBe(true);
    CARRIER_RE.lastIndex = 0;
  });
});
