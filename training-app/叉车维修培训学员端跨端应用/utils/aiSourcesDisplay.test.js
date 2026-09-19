/**
 * 诊断来源展示纯函数单元测试（#988）
 *
 * `utils/aiSourcesDisplay.uts` 是 UTS，Node 无法直接 import ⇒ 按仓库惯例
 * （`pointsDisplay.test.js` / `checkinCalendar.test.js`）以**镜像实现**验证算法行为，
 * 文末的「镜像同步」用例把 .uts 源码与镜像逐点对齐，防止两份实现悄悄分叉。
 *
 * 为什么这份测试是这套改动里最承重的一块：抽成 `utils/` 之前，这些纯函数住在
 * `components/ai-chat/ai-chat-sources.uvue` 里，守护只能写成
 * `expect(SOURCES).toContain("const IMAGE_OPEN = '<<IMAGE:'")` —— 断言的是源码**文本**，
 * 改坏行为而文本仍在就照样绿。现在算法本身被真的执行。
 *
 * 用例里的样本取自 **2026-09-16 直连生产**的 `sources` 事件原文（`BMS` 一问，13 条来源），
 * 不是编造的格式。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const SRC_REL = 'utils/aiSourcesDisplay.uts';
const SRC = read(SRC_REL);

// ===== 镜像实现（与 utils/aiSourcesDisplay.uts 保持一致）=====

const IMAGE_OPEN = '<<IMAGE:';
const IMAGE_CLOSE = '>>';
const ASSISTANT_STATIC_PREFIX = 'assistant/static/';
const MANUAL_PREFIX = 'manual/';

function stripAssistantPrefix(p) {
  let s = p;
  if (s.startsWith('/')) s = s.substring(1);
  if (s.startsWith(ASSISTANT_STATIC_PREFIX)) s = s.substring(ASSISTANT_STATIC_PREFIX.length);
  if (s.startsWith(MANUAL_PREFIX)) s = s.substring(MANUAL_PREFIX.length);
  return s;
}

function extractImagePaths(text) {
  const out = [];
  let from = 0;
  while (true) {
    const start = text.indexOf(IMAGE_OPEN, from);
    if (start < 0) break;
    const end = text.indexOf(IMAGE_CLOSE, start + IMAGE_OPEN.length);
    if (end < 0) break;
    const p = stripAssistantPrefix(text.substring(start + IMAGE_OPEN.length, end).trim());
    if (p.length > 0) out.push(p);
    from = end + IMAGE_CLOSE.length;
  }
  return out;
}

function stripImageMarkers(text) {
  let out = '';
  let from = 0;
  while (true) {
    const start = text.indexOf(IMAGE_OPEN, from);
    if (start < 0) {
      out += text.substring(from);
      break;
    }
    const end = text.indexOf(IMAGE_CLOSE, start + IMAGE_OPEN.length);
    if (end < 0) {
      out += text.substring(from);
      break;
    }
    out += text.substring(from, start);
    from = end + IMAGE_CLOSE.length;
  }
  return out.trim();
}

function pageLabel(src) {
  const ps = src.page_start == null ? 0 : src.page_start;
  if (ps <= 0) return '';
  const pe = src.page_end == null ? 0 : src.page_end;
  if (pe > ps) return '第 ' + ps.toString() + '-' + pe.toString() + ' 页';
  return '第 ' + ps.toString() + ' 页';
}

// ===== 线上原文样本 =====

const REAL_MARKER =
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_643.png>>';
const REAL_PATH = 'ep_byd_pmw20_service_manual_en/page_85_643.png';
/** 同一页三张图（实测 source#6 / page 85 就是这样） */
const THREE_FIGURES =
  '见图：' + REAL_MARKER +
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_644.png>>' +
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_645.png>>' +
  ' 以上为接线示意。';

describe('aiSourcesDisplay：标记解析（真行为，不是源码文本）', () => {
  describe('stripAssistantPrefix：剥掉助手内网前缀', () => {
    it('线上原样（带前导 / 与 assistant/static/ 与 manual/）', () => {
      expect(stripAssistantPrefix('/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_643.png'))
        .toBe(REAL_PATH);
    });
    it('缺前导 / 也成立', () => {
      expect(stripAssistantPrefix('assistant/static/manual/doc/page_1.png')).toBe('doc/page_1.png');
    });
    it('只有 manual/（后端已代理过的形态）', () => {
      expect(stripAssistantPrefix('manual/doc/page_1.png')).toBe('doc/page_1.png');
    });
    it('没有前缀时原样返回', () => {
      expect(stripAssistantPrefix('doc/page_1.png')).toBe('doc/page_1.png');
    });
    it('manual 是**目录名的一部分**时不被误剥（不得吃掉文档名）', () => {
      // 反例防线：若剥离写成「全局替换 manual/」，这个文档名会被啃掉一段。
      expect(stripAssistantPrefix('/assistant/static/manual/manual_extra/page_1.png'))
        .toBe('manual_extra/page_1.png');
    });
  });

  describe('extractImagePaths：抽出子路径列表', () => {
    it('单标记 → 单路径', () => {
      expect(extractImagePaths(REAL_MARKER)).toEqual([REAL_PATH]);
    });
    it('同页三张图 → 三条，且保持出现顺序', () => {
      expect(extractImagePaths(THREE_FIGURES)).toEqual([
        REAL_PATH,
        'ep_byd_pmw20_service_manual_en/page_85_644.png',
        'ep_byd_pmw20_service_manual_en/page_85_645.png',
      ]);
    });
    it('正文里的标记加前后夹带文字也能抽出来', () => {
      expect(extractImagePaths('见下图 ' + REAL_MARKER + ' 完毕')).toEqual([REAL_PATH]);
    });
    it('没有标记 → 空数组（不得抛错）', () => {
      expect(extractImagePaths('这是一段没有图的来源正文。')).toEqual([]);
    });
    it('空路径的标记被丢弃', () => {
      expect(extractImagePaths('a<<IMAGE:>>b')).toEqual([]);
    });
    it('只有左侧定界符（残缺）时安全退出，且不吞掉已抽出的部分', () => {
      expect(extractImagePaths(REAL_MARKER + ' 残缺 <<IMAGE:/assistant/static/manual/x/')).toEqual([REAL_PATH]);
    });
  });

  describe('stripImageMarkers：标记不进正文', () => {
    it('标记被剥掉、正文保留、两端空白被裁', () => {
      expect(stripImageMarkers('  ' + REAL_MARKER + '  ')).toBe('');
    });
    it('夹在正文里的标记剥掉后正文连起来', () => {
      expect(stripImageMarkers('前' + REAL_MARKER + '后')).toBe('前后');
    });
    it('多标记全剥', () => {
      expect(stripImageMarkers(THREE_FIGURES)).toBe('见图： 以上为接线示意。');
    });
    it('无标记时等于 trim', () => {
      expect(stripImageMarkers('  纯文本  ')).toBe('纯文本');
    });
    it('**不得**把 `<<IMAGE:`/`>>` 之类的残字留在正文里', () => {
      const out = stripImageMarkers(THREE_FIGURES);
      expect(out).not.toContain('<<IMAGE:');
      expect(out).not.toContain('>>');
      expect(out).not.toContain('/assistant/static/');
    });
  });

  describe('pageLabel：页码标签', () => {
    it('同页（page_start == page_end）→ 第 N 页', () => {
      expect(pageLabel({ page_start: 529, page_end: 529 })).toBe('第 529 页');
    });
    it('跨页 → 第 N-M 页', () => {
      expect(pageLabel({ page_start: 4, page_end: 12 })).toBe('第 4-12 页');
    });
    it('只有起始页 → 第 N 页', () => {
      expect(pageLabel({ page_start: 90 })).toBe('第 90 页');
    });
    it('起始页缺失 → 不显示（空串）', () => {
      expect(pageLabel({})).toBe('');
    });
    it('起始页为 0 → 不显示（0 不是合法页号）', () => {
      expect(pageLabel({ page_start: 0, page_end: 0 })).toBe('');
    });
    it('page_end 小于 page_start（脏数据）→ 退回单页，不产出「第 5-3 页」', () => {
      expect(pageLabel({ page_start: 5, page_end: 3 })).toBe('第 5 页');
    });
  });
});

describe('aiSourcesDisplay：镜像同步（防两份实现分叉）', () => {
  it('源码在 utils/ 下，且不依赖 api（utils 不反向依赖 api 层）', () => {
    expect(SRC).toContain('export function stripAssistantPrefix');
    expect(SRC).toContain('export function extractImagePaths');
    expect(SRC).toContain('export function stripImageMarkers');
    expect(SRC).toContain('export function pageLabel');
    expect(SRC).not.toContain("from '../api/");
    // 只禁**依赖**（import 行），不禁「注释里提到这个名字」—— 本文件第一版写成
    // `not.toContain('aiManualUrl')`，被自己文档注释里那句「URL 由 … 的 `aiManualUrl()` 负责」
    // 误判成违规（假阳性）。这是本会话第三次「判据撞上注释」，故锚点收窄到 import 行。
    expect(SRC).not.toMatch(/^import .*aiManualUrl/m);
  });

  it('四个定界符/前缀字面量与镜像一致', () => {
    expect(SRC).toContain("export const IMAGE_OPEN = '<<IMAGE:'");
    expect(SRC).toContain("export const IMAGE_CLOSE = '>>'");
    expect(SRC).toContain("const ASSISTANT_STATIC_PREFIX = 'assistant/static/'");
    expect(SRC).toContain("const MANUAL_PREFIX = 'manual/'");
  });

  it('源码里没有正则字面量（全仓 .uts 无 RegExp 先例，Kotlin 目标下不可靠）', () => {
    expect(SRC).not.toContain('new RegExp(');
    expect(SRC).not.toMatch(/=\s*\/[^/\n]*<<IMAGE/);
  });

  it('源码用的就是 indexOf/substring 扫描（镜像照着它写）', () => {
    expect(SRC).toContain('text.indexOf(IMAGE_OPEN');
    expect(SRC).toContain('text.substring(');
  });

  it('页码标签的三种形态在源码里都在（第 N 页 / 第 N-M 页 / 空串）', () => {
    expect(SRC).toContain("' 页'");
    expect(SRC).toContain("'-'");
    expect(SRC).toContain("if (ps <= 0) return ''");
  });
});
