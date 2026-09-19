/**
 * 投稿状态词表单点契约（issue #1108；移动端决策 docs/adr/0018）
 *
 * 背景：投稿状态五态（后端 service.ContributionStatus*）的文案与配色原先内联在
 * pages/resources/my-uploads.uvue，且与 Web 端漂移两条——pending「待审核」vs「审核中」、
 * rejected「未通过」vs「已驳回」。处置：判定收进 utils/contributionStatus.uts 一份，
 * 消费面只做投影（守护规则 S：模板不可直调 import 函数）。
 *
 * 缝：.uts / .uvue 无法在 jest 中 import，沿用源码契约测试缝（先例 utils/resourcesContract.test.js）。
 *
 * 钉住的契约：
 * 1) 单点——五个取值各一行 descriptor，判定只有这一处；
 * 2) 对账锁①——取值集合与后端 service.ContributionStatus* 常量表**两向相等**（直读 Go 源）；
 * 3) 对账锁②——五个 label 与 Web frontend/src/pages/student/ContributionTab.vue 的 STATUS_LABEL
 *    逐字相等（两端同源；Web 侧尚未收编成 descriptor，故本锁直读那张 Record）；
 * 4) 消费面——my-uploads 只做投影、模板不直调 import 函数；
 * 5) 零内联——pages/components/utils/composables/api 的 .uvue/.uts 不再出现投稿状态文案裸串
 *    （允许表只收「唯一判定处」与另一个域的「审核中」资料待审徽标）；
 * 6) 锁自检——把「漏一个取值」「文案漂移」「重新内联」的合成源喂给同一解析器/扫描器，
 *    必须报出差异（防假绿：锁坏了要红，不能静默放行）。
 *
 * @note descriptor 的行形态（单行五字段 + `as ContributionStatusDescriptor`）是锁的一部分：
 *       改形态要同步改本锁，不要把 rowRe 放宽成模糊匹配（放宽 = 漏值不再被看见）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const REPO = path.join(ROOT, '..', '..');

/**
 * 工作树文本一律**经读者归一 EOL** 后再进断言面（#1143）。
 *
 * 读者真源在 `utils/utsHarness.js` 的 `readText` / `normalizeEol`（#1176 收成一份，别在本文件再抄）。
 * 行为守护（喂合成 CRLF）见 `utils/contractReaderEolContract.test.js` —— 那条判据与检出平台无关。
 *
 * 为什么归一放在读者处：本仓的接线守护用**多行锚点**匹配源码文本，锚点里含 `\n`。Windows 检出
 * （`core.autocrlf=true`）落到工作树的是 **CRLF** ⇒ `.*\n` 这类锚点里 `.` **不匹配 `\r`**、
 * `\n` 又必须紧跟其后，于是在 CRLF 行上**匹配 0 次**：变异根本没生效，断言却照跑 —— 守护**静默失效**
 * （实测：`archived` 行删不掉，18 例红 1 例）。归一摆在这一处，对**任何原因**导致的 CRLF 都成立；
 * 逐处把锚点放宽成 `\r?\n` 只治好当前那条。决策见 `docs/adr/0019`。
 */
const { readText } = require('./utsHarness');
const read = (rel) => readText(path.join(ROOT, rel));
const readRepo = (rel) => readText(path.join(REPO, rel));

const DESCRIPTOR_REL = 'utils/contributionStatus.uts';
const CONSUMER_REL = 'pages/resources/my-uploads.uvue';
const GO_REL = 'backend/internal/service/contribution_service.go';
const WEB_REL = 'frontend/src/pages/student/ContributionTab.vue';

/** 投稿状态文案（唯一判定处之外不得出现；注释不算） */
const STATUS_LABELS = ['审核中', '已上架', '已驳回', '已撤回', '已下架'];

/** 允许持有这些文案的文件（唯一判定处 + 另一个域的同名取值） */
const INLINE_ALLOWLIST = new Map([
  ['utils/contributionStatus.uts', '唯一判定处（本锁的判据源）'],
  ['pages/profile/personal-info.uvue', '资料修改待审徽标（pending_profile_change）——另一个域的取值，与投稿无关'],
]);

const stripComments = (src) =>
  src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');

/** 命中文件里出现的投稿状态文案裸串（注释已剥除） */
const scanInlineLabels = (text) => STATUS_LABELS.filter((label) => stripComments(text).includes(label));

/** 取出 descriptor 行表本体（只认 CONTRIBUTION_STATUS_DESCRIPTORS 数组字面量内的行） */
function descriptorTableBody(src) {
  const clean = stripComments(src);
  const head = clean.indexOf('CONTRIBUTION_STATUS_DESCRIPTORS');
  if (head === -1) return null;
  // 从 `=` 之后找数组字面量：类型标注 `ContributionStatusDescriptor[]` 里的空方括号在 `=` 之前，
  // 直接 indexOf('[') 会取到它、得到一个空表体（锁静默失效）。
  const eq = clean.indexOf('=', head);
  if (eq === -1) return null;
  const open = clean.indexOf('[', eq);
  if (open === -1) return null;
  let depth = 0;
  for (let i = open; i < clean.length; i++) {
    const c = clean[i];
    if (c === '[') depth++;
    else if (c === ']') {
      depth--;
      if (depth === 0) return clean.slice(open + 1, i);
    }
  }
  return null;
}

/** 解析 descriptor 行表（只收表内行；表外的兜底常量不算取值） */
function parseDescriptorRows(src) {
  const body = descriptorTableBody(src);
  if (body === null) return [];
  const rows = [];
  const rowRe =
    /\{\s*status:\s*'([^']*)',\s*label:\s*'([^']*)',\s*icon:\s*'([^']*)',\s*color:\s*'([^']*)',\s*bg:\s*'([^']*)'\s*\}\s*as\s+ContributionStatusDescriptor/g;
  let m;
  while ((m = rowRe.exec(body)) !== null) {
    rows.push({ status: m[1], label: m[2], icon: m[3], color: m[4], bg: m[5] });
  }
  return rows;
}

/** 解析后端 Go 常量表里的投稿状态取值（行首赋值形态，避开 `==` 比较） */
function parseGoStatuses(src) {
  const out = [];
  const re = /^\s*ContributionStatus[A-Za-z]*\s*=\s*"([a-z_]+)"/gm;
  let m;
  while ((m = re.exec(stripComments(src))) !== null) out.push(m[1]);
  return out;
}

/** 解析 Web 端 STATUS_LABEL 这张 Record（两端同源的对账源） */
function parseWebLabels(src) {
  const clean = stripComments(src);
  const head = clean.indexOf('const STATUS_LABEL');
  if (head === -1) return null;
  const open = clean.indexOf('{', head);
  if (open === -1) return null;
  const close = clean.indexOf('}', open);
  if (close === -1) return null;
  const labels = new Map();
  const re = /([A-Za-z_$][\w$]*)\s*:\s*'([^']*)'/g;
  let m;
  while ((m = re.exec(clean.slice(open + 1, close))) !== null) labels.set(m[1], m[2]);
  return labels;
}

const rows = parseDescriptorRows(read(DESCRIPTOR_REL));

describe('单点：utils/contributionStatus.uts 持有唯一判定', () => {
  const src = read(DESCRIPTOR_REL);

  it('五个取值各一行 descriptor，status 无重复', () => {
    expect(rows.map((r) => r.status)).toEqual(['pending', 'approved', 'rejected', 'withdrawn', 'archived']);
  });

  it('每行四要素齐备（label/icon/color/bg 非空）', () => {
    const blank = rows.filter((row) =>
      ['label', 'icon', 'color', 'bg'].some((field) => row[field].length === 0)
    );
    expect(blank).toEqual([]);
    expect(rows.map((r) => r.color)).toEqual(['#FF9800', '#22A45D', '#E5484D', '#999999', '#999999']);
  });

  it('唯一入口 describeContributionStatus 导出，且判定不散成 if 链', () => {
    expect(src).toMatch(/export function describeContributionStatus\(/);
    expect(stripComments(src)).not.toMatch(/if \(status ==/);
  });
});

describe('对账锁①：取值集合与后端 Go 常量表两向相等', () => {
  it('后端常量表当前五态（加态须回来加行并同步本锁）', () => {
    expect(parseGoStatuses(readRepo(GO_REL))).toEqual([
      'pending',
      'approved',
      'rejected',
      'withdrawn',
      'archived'
    ]);
  });

  it('descriptor 取值集合 == Go 常量值集合（漏一个取值即红）', () => {
    const go = [...new Set(parseGoStatuses(readRepo(GO_REL)))].sort();
    const mine = [...new Set(rows.map((r) => r.status))].sort();
    expect(mine).toEqual(go);
  });
});

describe('对账锁②：label 与 Web STATUS_LABEL 逐字相等（两端同源）', () => {
  it('Web 侧词表可解析（fail-closed：解析不到 = 锁失效，不是通过）', () => {
    const webPath = path.join(REPO, WEB_REL);
    if (!fs.existsSync(webPath)) {
      throw new Error(
        `两端同源锁：找不到 Web 投稿状态词表 ${WEB_REL}——若 Web 侧已把投稿域收编成 descriptor，` +
          '请把本锁指到那个新单点（根仓 ADR-0056 §8 的相邻域条款），不要删锁'
      );
    }
    expect(parseWebLabels(readRepo(WEB_REL)).size).toBe(5);
  });

  it('五个 label 两端逐字相同', () => {
    const labels = parseWebLabels(readRepo(WEB_REL));
    expect(rows.map((r) => `${r.status}:${r.label}`)).toEqual(
      rows.map((r) => `${r.status}:${labels.get(r.status)}`)
    );
  });
});

describe('消费面：my-uploads 只做投影（判定不回流页面）', () => {
  const src = read(CONSUMER_REL);

  it('import describeContributionStatus 自 utils/contributionStatus', () => {
    expect(src).toMatch(
      /import\s*\{[^}]*\bdescribeContributionStatus\b[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/contributionStatus'/
    );
  });

  it('页面不再持有判定（无 if (status == …) 链）', () => {
    expect(stripComments(src)).not.toMatch(/if \(status ==/);
  });

  it('模板经本地薄包装消费，不直调 import 函数（守护规则 S）', () => {
    const tpl = src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>'));
    expect(tpl).not.toMatch(/describeContributionStatus\s*\(/);
    expect(tpl).toContain('statusLabel(item.status)');
    expect(tpl).toContain('statusBg(item.status)');
  });

  it('驳回提示条仍按 rejected 判定（本次只统一文案，不改行为）', () => {
    expect(src).toContain("item.status == 'rejected' && item.reject_reason.length > 0");
  });
});

describe('零内联：投稿状态文案不再出现在任何消费面', () => {
  it('pages/components/utils/composables/api 的 .uvue/.uts 无裸串（注释不计）', () => {
    const offenders = [];
    const walk = (dir) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) {
          walk(p);
          continue;
        }
        if (!/\.(uts|uvue)$/.test(e.name) || /\.test\./.test(e.name)) continue;
        const rel = path.relative(ROOT, p).replace(/\\/g, '/');
        if (INLINE_ALLOWLIST.has(rel)) continue;
        const hits = scanInlineLabels(readText(p));
        if (hits.length > 0) offenders.push(`${rel} → ${hits.join('/')}`);
      }
    };
    for (const d of ['pages', 'components', 'utils', 'composables', 'api']) {
      walk(path.join(ROOT, d));
    }
    expect(offenders).toEqual([]);
  });
});

describe('锁自检：合成违规必须报红（防假绿）', () => {
  const src = read(DESCRIPTOR_REL);

  it('漏一个取值：合成少一行的源，集合不再与 Go 常量表相等', () => {
    const missing = src.replace(/^ *\{ status: 'archived'.*\n/m, '');
    const broken = parseDescriptorRows(missing);
    expect(broken.map((r) => r.status)).not.toContain('archived');
    expect([...new Set(broken.map((r) => r.status))].sort()).not.toEqual(
      [...new Set(parseGoStatuses(readRepo(GO_REL)))].sort()
    );
  });

  it('文案漂移：合成改一个字的源，两端同源对账必须不等', () => {
    const drifted = parseDescriptorRows(src.replace("label: '审核中'", "label: '待审核'"));
    const pending = drifted.find((r) => r.status === 'pending');
    expect(pending.label).toBe('待审核');
    expect(pending.label).not.toBe(parseWebLabels(readRepo(WEB_REL)).get('pending'));
  });

  it('重新内联：扫描器认得出裸串，且注释不算', () => {
    expect(scanInlineLabels("function statusLabel() : string { return '已上架' }")).toEqual(['已上架']);
    expect(scanInlineLabels('// pending 待审核 / rejected 未通过')).toEqual([]);
    expect(scanInlineLabels("const s = ['已驳回', '已撤回']")).toEqual(['已驳回', '已撤回']);
  });

  it('表体定位：类型标注里的空方括号不算表体（取错 = 空表 = 锁静默失效）', () => {
    const body = descriptorTableBody(src);
    expect(body).not.toBeNull();
    expect(body).toContain("status: 'pending'");
    expect(body).not.toContain('ContributionStatusDescriptor[]');
  });

  it('表外兜底常量不算取值（UNKNOWN_DESCRIPTOR 的空 status 不入集合）', () => {
    expect(src).toContain("status: '', label: '审核中'");
    expect(rows.map((r) => r.status)).not.toContain('');
    expect(rows.filter((r) => r.status.length === 0)).toEqual([]);
  });

  it('解析器形态锁：改散行形态后解析为零（本锁 fail-closed，不放行模糊匹配）', () => {
    const reformatted = src
      .replace(/\{ status: 'pending',/, '{\n    status: "pending",');
    expect(parseDescriptorRows(reformatted).map((r) => r.status)).not.toContain('pending');
  });
});

describe('读者自检：工作树 EOL 归一（Windows 检出恒红根因，#1143）', () => {
  // 合成 CRLF 的**行为**断言已收进共享读者的守护 `utils/contractReaderEolContract.test.js`（#1176）——
  // 那里喂合成源、与检出平台无关，且覆盖任何读者调用方。本文件只留「本契约的真源读出来没有 \r」。

  it('真源经读者后不含 \\r（本仓 .uts / .uvue 由 .gitattributes 钉 LF，此处是纵深防御）', () => {
    expect(read(DESCRIPTOR_REL)).not.toContain('\r');
    expect(read(CONSUMER_REL)).not.toContain('\r');
    // 仓外两个对账源由 .gitattributes 钉了 LF（`*.go` / `*.vue`），今天恒真；
    // 留着是为了「半改一读」也被看见 —— readRepo 绕过归一就少一处出口。
    expect(readRepo(GO_REL)).not.toContain('\r');
  });
});
