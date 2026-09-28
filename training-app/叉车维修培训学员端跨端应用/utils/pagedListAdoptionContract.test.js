/**
 * 分页列表协议「手写存量」棘轮（移动端 ADR-0027）—— **接线守护**，不构成 ③ 门证据。
 *
 * 它守的不是某个算法对不对（那由 `utils/pagedListBehavior.test.js` 真执行模块来证），
 * 而是一条**只减不增**的接线事实：还有几处列表页在自己写装载协议。
 *
 * 判据（两条同时成立算一处手写协议，与协议形态无关）：
 *   ① **页码游标**：出现 `…page = ref`（`page` / `replyPage` / `faultPage` … 一律算）；
 *   ② **取数函数**：出现 `function loadXxx / fetchXxx / reloadXxx`。
 * 为什么不用「有没有 `noMore` 标志」当判据：终止条件在本仓有三种算法（`len >= total` /
 * `items.length < pageSize` / `page < pages`，见 `pages/search/search.uvue:200-202`），
 * 拿其中一种当门槛会给出「把标志改名就豁免」的后门。
 *
 * 口径是**两向对账**（同 `utils/modules.js` 的 `oversized` / `pending`、`utils/guardClassification.test.js`
 * 的 H4）：计数必须**恰好等于** `HAND_ROLLED_BASELINE` —— 新增一处判红（棘轮不许回潮），
 * 迁走一处而不下调常量也判红（登记位不许过期）。**不设 allowlist**：
 * 豁免机制已由 ADR-0023 决策⑧ / #654 整体退役，别再带回来。
 *
 * 唯一的排除项是**判据的定义处**：`composables/usePagedList.uts` 自己就持页码游标与取数逻辑 ——
 * 它是协议本体，不是手写存量。先例：`internal/credentialscope` 的 `DefinitionFile`
 * （「判据的定义处不是消费者」）。
 *
 * 读者用 `utils/utsHarness.js` 的 `readText`（ADR-0019：全仓唯一的读取层归一真源）。
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');

/** 判据的定义处：模块本体（协议就是它），不是手写存量 */
const MODULE_FILE = 'composables/usePagedList.uts';

/**
 * 手写载体的实测基线（2026-09-27，ADR-0027 立项时现测）。
 * 迁移一页 ⇒ **同一 PR 下调本数**；新增一页 ⇒ 判红。
 */
const HAND_ROLLED_BASELINE = 22;

/** 页码游标：`page = ref` / `replyPage = ref` / `faultPage = ref<number>(1)` … */
const CURSOR_RE = /\b\w*[Pp]age\s*=\s*ref/;

/** 取数函数：`function loadXxx(` / `function fetchXxx(` / `function reloadXxx(` */
const LOADER_RE = /function\s+\w*(load|fetch|reload)\w*\s*\(/;

/** 收 `.uvue`（页面）与 `.uts`（页面私有 composable / 全局 composable）—— 协议可以藏进两者 */
const SUBJECT_RE = /\.(uvue|uts)$/;

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
    if (rel.includes('.test.')) continue;
    if (rel === MODULE_FILE) continue;
    out.push(rel);
  }
  return out;
}

/** 扫出的手写载体（相对仓根的路径，排序稳定） */
function handRolledCarriers() {
  const files = [...walk(path.join(ROOT, 'pages'), []), ...walk(path.join(ROOT, 'composables'), [])];
  return files
    .filter((rel) => {
      const src = readText(path.join(ROOT, rel));
      return CURSOR_RE.test(src) && LOADER_RE.test(src);
    })
    .sort();
}

describe('分页列表协议：手写存量只减不增（ADR-0027）', () => {
  test('扫描面非空（在空集合上判绿 = 本仓防的那件事）', () => {
    // fail-closed：目录改名 / 扫描面被改窄时先红，而不是「0 处手写」报绿
    expect(walk(path.join(ROOT, 'pages'), []).length).toBeGreaterThan(50);
    expect(walk(path.join(ROOT, 'composables'), []).length).toBeGreaterThan(5);
  });

  test(`手写载体计数必须恰好等于 ${HAND_ROLLED_BASELINE}（新增判红；迁走一页须同步下调本数）`, () => {
    const carriers = handRolledCarriers();
    // 失败信息即普查表：差几个、差在哪个文件，一眼可读
    expect({ count: carriers.length, carriers }).toEqual({
      count: HAND_ROLLED_BASELINE,
      carriers,
    });
    expect(carriers.length).toBe(HAND_ROLLED_BASELINE);
  });

  test('模块本体不被算作手写存量（判据的定义处不是消费者）', () => {
    expect(handRolledCarriers()).not.toContain(MODULE_FILE);
    // 反证：模块本体**确实**持页码游标与取数逻辑 —— 排除它是有意的，不是因为它不匹配
    const src = readText(path.join(ROOT, MODULE_FILE));
    expect([CURSOR_RE.test(src), LOADER_RE.test(src)]).toEqual([true, true]);
  });
});
