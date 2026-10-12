/**
 * UI 文档地图契约守护（#1611；判据面 = #1597 S3 裁定，地图表见 docs/ui-spec.md §0）
 *
 * 钉住的是「UI 文档彼此不漂移」——#1597 全仓对账发现 UI 文档各说各话（token 名对不上
 * uni.scss、状态口径不一致、快照被当成真源引用），裁定唯一能防复发的手段是让守护读文档本身。
 * 三组断言：
 * - A 地图齐全：§0 表六份 UI 文档各登记一次（缺行 / 重复行 / 未登记杂项都判红）。
 * - B 状态枚举：每行「状态」以四枚举类之一开头——现行真源 / 现行 / 实施计划 / 历史快照
 *   （最长优先：现行真源 必须先于 现行 判；括注如（值）（产品层）（Phase 1–3 已废止）是
 *   补充说明不参与判定，#1597 S1 建议值与 S3 草表两种写法都兼容）。
 * - C token 双向一致：文档全文里表行首格为 $token 的行（§1.1–§4，共 39 行）与 uni.scss
 *   业务段（「业务主题变量」标记行起，含「扩展 Token」段）双向对账——文档登记但 uni 未定义、
 *   值不一致、uni 定义但文档未登记，都判红。$uni-* 内建变量在标记行之前，天然不进判据；
 *   :18 $uni-color-primary 与 :81 $primary-color 的双点同步单列一组（ui-spec §1.1 换色执行口径）。
 *
 * 分类：接线守护（读两份真源文本做一致性断言，无代码级执行调用，分类器落 wiring）；
 * guards.md 末节三问在 PR 正文作答。
 *
 * 更新指引：新增 UI 文档 → 先改 §0 表、再把它加进 REQUIRED_DOCS；新增 token → uni.scss
 * 业务段与文档 token 表**同一次提交**成对登记。改坏样本注入（必红）与对称注入（必绿）
 * 自检组在下方，动扫描函数后必须保持双绿。
 */
const h = require('./contractHarness');
const read = h.read;

const UI_SPEC = read('docs/ui-spec.md');
const UNI_SCSS = read('uni.scss');

const MAP_HEADER = '## 0. 本仓 UI 文档地图';
const BUSINESS_MARKER = '业务主题变量';
/** #1597 S3 六份；顺序即 §0 表顺序 */
const REQUIRED_DOCS = [
  'docs/ui-spec.md',
  'uni.scss',
  'docs/ui-spec-implementation-plan.md',
  'docs/product-design.md',
  'docs/对齐成果-AI助手底栏与专业版形态.md',
  'docs/ui-design-diff-analysis.md',
];
/** 最长优先手写序：现行真源（4 字）先于 现行（2 字）判 */
const STATUS_CLASSES = ['现行真源', '实施计划', '历史快照', '现行'];

/** 值归一口径：trim + 小写 + 3 位 hex 折 6 位（同 batch2UiContract / recruiterUiContract） */
function normValue(raw) {
  let v = raw.trim().toLowerCase();
  const m = v.match(/^#([0-9a-f]{3})$/);
  if (m) v = '#' + m[1].split('').map((c) => c + c).join('');
  return v;
}

/** 剥 markdown 修饰（反引号 / 粗体星号） */
function stripMd(cell) {
  return cell.replace(/[`*]/g, '').trim();
}

/** 表行拆格：以 | 开头的行 → 去掉首尾空段后的单元格数组；非表行回 null */
function tableCells(line) {
  if (!line.trim().startsWith('|')) return null;
  const parts = line.split('|');
  return parts.slice(1, -1).map((c) => c.trim());
}

function statusClassOf(status) {
  for (const cls of STATUS_CLASSES) {
    if (status.startsWith(cls)) return cls;
  }
  return null;
}

/** A+B：§0 文档地图提取与判红。纯函数——注入自检喂改坏样本。 */
function scanDocMap(docText) {
  const violations = [];
  const rows = [];
  const lines = docText.split('\n');
  const start = lines.findIndex((l) => l.trim().startsWith(MAP_HEADER));
  if (start < 0) {
    return { rows, violations: ['文档地图缺失：找不到「' + MAP_HEADER + '」标题（地图被移动或删除？）'] };
  }
  for (let i = start + 1; i < lines.length; i++) {
    const t = lines[i].trim();
    if (t === '---' || /^## /.test(t)) break; // 地图表到节尾为止
    const cells = tableCells(lines[i]);
    if (!cells) continue; // 标题下的说明 blockquote
    if (cells.every((c) => /^[-: ]*$/.test(c))) continue; // 表头分隔行 |---|---|
    if (cells[0] === '文档' && cells[2] === '状态') continue; // 表头行
    if (cells.length < 3) {
      violations.push('文档地图行格式异常（应为 | 文档 | 定位 | 状态 |）：' + lines[i]);
      continue;
    }
    rows.push({ path: stripMd(cells[0]), status: stripMd(cells[2]) });
  }
  const counts = new Map();
  for (const r of rows) counts.set(r.path, (counts.get(r.path) || 0) + 1);
  for (const p of REQUIRED_DOCS) {
    if (!counts.get(p)) violations.push('文档地图缺行：' + p + '（#1597 S3 六份必须各登记一次）');
  }
  for (const [p, n] of counts) {
    if (n > 1) violations.push('文档地图重复行：' + p + '（登记 ' + n + ' 次）');
    if (!REQUIRED_DOCS.includes(p)) {
      violations.push('文档地图出现未登记文档：' + p + '（确属 UI 文档就把它加进本守护 REQUIRED_DOCS，否则移出地图）');
    }
  }
  for (const r of rows) {
    if (!statusClassOf(r.status)) {
      violations.push('状态越枚举：' + r.path + ' 状态「' + r.status + '」（只允许以 现行真源/现行/实施计划/历史快照 之一开头）');
    }
  }
  return { rows, violations };
}

/** C：文档 token 表 ↔ uni.scss 业务段双向对账。纯函数。 */
function scanTokenConsistency(docText, uniText) {
  const violations = [];
  const docTokens = new Map();
  for (const line of docText.split('\n')) {
    const cells = tableCells(line);
    if (!cells || cells.length < 2) continue;
    const name = stripMd(cells[0]);
    if (!/^\$[a-z][a-z0-9-]*$/.test(name)) continue; // 表头 / 非token行
    if (docTokens.has(name)) violations.push('文档 token 表重复行：' + name);
    else docTokens.set(name, stripMd(cells[1]));
  }
  const uniTokens = new Map();
  const lines = uniText.split('\n');
  const bizStart = lines.findIndex((l) => l.includes(BUSINESS_MARKER));
  if (bizStart < 0) {
    violations.push('uni.scss 结构锚点丢失：找不到「' + BUSINESS_MARKER + '」标记行（业务段定位失效，守护拒判）');
    return { docTokens, uniTokens, violations };
  }
  const defRe = /^\$([a-z][a-z0-9-]*)\s*:\s*([^;]+);/;
  for (let i = bizStart; i < lines.length; i++) {
    const m = lines[i].match(defRe);
    if (!m) continue; // 注释行 / 空行 / 段标（占位注释以 /* 开头，天然不中）
    const name = '$' + m[1];
    if (uniTokens.has(name)) violations.push('uni.scss 业务段重复定义：' + name);
    else uniTokens.set(name, m[2]);
  }
  for (const [name, docVal] of docTokens) {
    const uniVal = uniTokens.get(name);
    if (uniVal === undefined) {
      violations.push('文档登记但 uni.scss 未定义：' + name + '（文档表值 ' + docVal + '）');
      continue;
    }
    if (normValue(docVal) !== normValue(uniVal)) {
      violations.push('token 值漂移：' + name + ' 文档=' + docVal + ' uni.scss=' + uniVal);
    }
  }
  for (const [name, uniVal] of uniTokens) {
    if (!docTokens.has(name)) {
      violations.push('uni.scss 定义但文档表未登记：' + name + '（uni.scss=' + uniVal + '）');
    }
  }
  return { docTokens, uniTokens, violations };
}

/** C+：ui-spec §1.1 换色执行口径——$uni-color-primary 与 $primary-color 双点同值 */
function scanPrimaryPair(uniText) {
  const builtin = uniText.match(/^\$uni-color-primary\s*:\s*([^;]+);/m);
  const primary = uniText.match(/^\$primary-color\s*:\s*([^;]+);/m);
  if (!builtin || !primary) {
    return ['主色双点锚丢失：uni.scss 找不到 $uni-color-primary 或 $primary-color 定义（换色执行口径失效）'];
  }
  if (normValue(builtin[1]) !== normValue(primary[1])) {
    return ['主色双点漂移：$uni-color-primary=' + builtin[1] + ' vs $primary-color=' + primary[1] + '（ui-spec §1.1：换色只改两处，两处必须同值）'];
  }
  return [];
}

describe('uiDocDocMapContract（#1611）：文档地图 6 份 + 状态枚举 + token↔uni.scss 双向一致', () => {
  const realMap = scanDocMap(UI_SPEC);
  const realTok = scanTokenConsistency(UI_SPEC, UNI_SCSS);

  describe('真库必绿 + 锚点（防解析面空跑假绿）', () => {
    it('§0 文档地图零违规', () => {
      expect(realMap.violations).toEqual([]);
    });

    it('锚点：地图恰好六份、各一次、路径与 #1597 S3 一致', () => {
      expect(realMap.rows.map((r) => r.path).sort()).toEqual([...REQUIRED_DOCS].sort());
    });

    it('锚点：文档 token 表与 uni.scss 业务段都解析出 ≥30 条（防两侧同烂的对称空跑）', () => {
      expect(realTok.docTokens.size).toBeGreaterThanOrEqual(30);
      expect(realTok.uniTokens.size).toBeGreaterThanOrEqual(30);
    });

    it('token 表↔uni.scss 双向一致（存在 + 值 + 登记）', () => {
      expect(realTok.violations).toEqual([]);
    });

    it('主色双点同步（ui-spec §1.1 换色执行口径）', () => {
      expect(scanPrimaryPair(UNI_SCSS)).toEqual([]);
    });
  });

  describe('注入必红（检测器自检：改坏样本必须判红，防空跑假绿）', () => {
    it('地图缺行 → 判红且点名缺失文档', () => {
      const doc = UI_SPEC.replace('| `docs/ui-design-diff-analysis.md` | 设计稿差异快照 | 历史快照 |\n', '');
      const v = scanDocMap(doc).violations;
      expect(v.some((x) => x.includes('缺行') && x.includes('ui-design-diff-analysis'))).toBe(true);
    });

    it('地图重复行 → 判红', () => {
      const row = '| `uni.scss` | 值面 | 现行真源（值） |';
      const doc = UI_SPEC.replace(row, row + '\n' + row);
      expect(scanDocMap(doc).violations.some((x) => x.includes('重复行'))).toBe(true);
    });

    it('地图杂项行 → 判红（地图 = 全部 UI 文档的唯一索引）', () => {
      const row = '| `uni.scss` | 值面 | 现行真源（值） |';
      const doc = UI_SPEC.replace(row, row + '\n| `docs/杂项-并非UI文档.md` | 杂项 | 现行 |');
      expect(scanDocMap(doc).violations.some((x) => x.includes('未登记文档'))).toBe(true);
    });

    it('状态越枚举 → 判红（不以四枚举类开头）', () => {
      const doc = UI_SPEC.replace('设计稿差异快照 | 历史快照', '设计稿差异快照 | 已废止残影');
      expect(scanDocMap(doc).violations.some((x) => x.includes('状态越枚举'))).toBe(true);
    });

    it('文档登记但 uni.scss 未定义 → 判红', () => {
      const doc = UI_SPEC.replace(
        '| `$input-border` | `#e5e5e5` | 输入框边框 |',
        '$&\n| `$ghost-token` | `#123456` | 注入自检样本 |'
      );
      expect(scanTokenConsistency(doc, UNI_SCSS).violations.some((x) => x.includes('$ghost-token'))).toBe(true);
    });

    it('token 值漂移 → 判红且点名两侧值', () => {
      const doc = UI_SPEC.replace('**`#239EDD`**', '**`#2979ff`**');
      const v = scanTokenConsistency(doc, UNI_SCSS).violations;
      expect(v.some((x) => x.includes('值漂移') && x.includes('$primary-color') && x.includes('#2979ff') && x.includes('#239EDD'))).toBe(true);
    });

    it('uni.scss 定义但文档表未登记 → 判红', () => {
      const doc = UI_SPEC.replace('| `$input-border` | `#e5e5e5` | 输入框边框 |\n', '');
      const v = scanTokenConsistency(doc, UNI_SCSS).violations;
      expect(v.some((x) => x.includes('未登记') && x.includes('$input-border'))).toBe(true);
    });

    it('主色双点漂移 → 判红', () => {
      const uni = UNI_SCSS.replace('$uni-color-primary: #239EDD;', '$uni-color-primary: #2979ff;');
      expect(scanPrimaryPair(uni).some((x) => x.includes('$uni-color-primary'))).toBe(true);
    });
  });

  describe('对称注入必绿（两侧一致新增必须放行，防「永远红」假守护）', () => {
    it('同一次提交成对新增 token（含大写 hex 归一）→ 零违规', () => {
      const doc = UI_SPEC.replace(
        '| `$input-border` | `#e5e5e5` | 输入框边框 |',
        '$&\n| `$ghost-ok` | `#ABCDEF` | 对称注入样本（大写 hex 考归一） |'
      );
      const uni = UNI_SCSS + '\n$ghost-ok: #abcdef;';
      expect(scanTokenConsistency(doc, uni).violations).toEqual([]);
    });
  });
});
