/**
 * 诊断来源展示纯函数单元测试（#988 立模块 · #1279 补第二根与描述后缀）
 *
 * **本文件真执行 `utils/aiSourcesDisplay.uts`**（缝 = `utils/utsHarness.js` 的 `loadUts`，
 * 把 `.uts` 去类型后当 JS 跑），断言的是**跑起来的产出物**。先例：`#1237`（收藏落点守护切到
 * utsHarness，同口径「补 ③ 承重证据的缝」）、`recruitDisplayBehavior` / `forumBodyBehavior`。
 *
 * 为什么这次把它从**手抄镜像**迁到真执行（不是顺手重构，是验收口径）：`docs/agents/guards.md`
 * 明写「手抄镜像只是**接线守护** —— 算法被改坏而字面量还在时**不会红**，**不构成 ③ 门证据**」，
 * 而 `docs/spec-永绿整改.md` 的 S6a 正是把本模块列为八个迁移对象之首。#1279 要落的三条
 * （第二根留给后端、`| 描述:` 截断、正文展开）全是**算法行为**，用镜像守等于没守。
 * 迁移台账见 `utils/guardClassification.test.js` 的 `KNOWN_MIRROR_MODULES`（H5 会因本模块
 * 不再是接线守护而红 —— 那正是要的，迁移 PR 必须把它划掉）。
 *
 * ⚠️ 判据纪律（spec §④ 更正 2）：「改成真跑了」本身不是验收，**注入坏实现必红**才是 ⇒
 * 文末「成对取证」四条各对应一种**写得出来**的坏实现，另配真源对照组（必不红）。
 *
 * 用例里的样本取自**一手实测**，不是编造的格式：
 *   ① `manual` 那串 = 2026-09-16 直连生产 `sources` 事件原文（`BMS` 一问，13 条来源）；
 *   ② `fault_images` 那串 = 2026-09-24 直连生产代理实测 HTTP 200 / image/png 的那张图，
 *      左串同时是后端 `canonicalizeDiagnosisSources` 换形后的来源正文原文
 *      （`backend/internal/service/ai_diagnosis_wire_fixture_test.go:156`）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { loadUts, exportedNames, readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SRC_REL = 'utils/aiSourcesDisplay.uts';
const UTS = path.join(ROOT, SRC_REL);
const SRC = readText(UTS);

/** 每次取一个**全新模块实例**（本模块零运行期依赖：只有一条 `import type` ⇒ 注入空绑定即可跑） */
const display = () => loadUts(UTS, {});

// ===== 线上原文样本 =====

const REAL_MARKER =
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_643.png>>';
const REAL_PATH = 'ep_byd_pmw20_service_manual_en/page_85_643.png';
/**
 * #1279 的第二根（助手 `20260921` 图文案例）：中文系统目录 + 非 manual 根。
 * 后端 `resolveStaticSubpath` 认**带根**形状（`staticRoots` = manual / fault_images），
 * 且绝对形状一律拒 ⇒ 「strip 唯一在前端、且恰好停在根名」是硬不变式。
 */
const FAULT_MARKER =
  '<<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png>>';
const FAULT_PATH = 'fault_images/制动系统/1721219449286.png';
/** 交付方入库形状可带 `| 描述:xxx` 后缀（其 postprocess 按第一个 `|` 截断取 URL） */
const CAPTIONED_MARKER =
  '<<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png | 描述:蓄能器接口>>';
/** 同一页三张图（实测 source#6 / page 85 就是这样） */
const THREE_FIGURES =
  '见图：' + REAL_MARKER +
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_644.png>>' +
  '<<IMAGE:/assistant/static/manual/ep_byd_pmw20_service_manual_en/page_85_645.png>>' +
  ' 以上为接线示意。';

// ===== ① 执行器自检：模块真的被 loadUts 跑起来了（防空跑假绿）=====

describe('aiSourcesDisplay：执行器自检', () => {
  it('注入空绑定即可跑（本模块只有一条 `import type`，零运行期依赖）', () => {
    expect(() => loadUts(UTS, {})).not.toThrow();
  });

  it('七个导出全部回读得到（少一个就是 `loadUts` 静默丢名字）', () => {
    const m = display();
    expect(exportedNames(SRC).sort()).toEqual([
      'IMAGE_CLOSE', 'IMAGE_OPEN', 'expandImageMarkers', 'extractImagePaths',
      'pageLabel', 'stripAssistantPrefix', 'stripImageMarkers',
    ]);
    expect(Object.keys(m).sort()).toEqual(exportedNames(SRC).sort());
  });

  it('定界符由**真执行**读出（不再是「镜像与源码两处字面量互抄」）', () => {
    const m = display();
    expect(m.IMAGE_OPEN).toBe('<<IMAGE:');
    expect(m.IMAGE_CLOSE).toBe('>>');
  });
});

// ===== ② 行为面：标记解析（真跑，不是源码文本）=====

describe('aiSourcesDisplay：标记解析（真行为）', () => {
  describe('stripAssistantPrefix：剥掉助手内网前缀', () => {
    const { stripAssistantPrefix } = display();

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
    const { extractImagePaths } = display();

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

    // ── #1279 自测点 1：20260921 的第二根（中文案例目录 + 非 manual 根）──────────
    it('fault_images 中文根：只剥 `assistant/static/`，**整段 `fault_images/…` 留给后端代理**', () => {
      expect(extractImagePaths(FAULT_MARKER)).toEqual([FAULT_PATH]);
    });
    it('两根并存时各归各的（历史行的无根旧形状不得被新根带偏）', () => {
      expect(extractImagePaths(REAL_MARKER + FAULT_MARKER)).toEqual([REAL_PATH, FAULT_PATH]);
    });

    // ── #1279 自测点 2：`| 描述:xxx` 后缀 ────────────────────────────────────
    it('带 `| 描述:` 后缀 → 后缀不得进路径（Web 同式，见 DiagnosisSources.vue）', () => {
      // 不截断的失效形态已实测：后缀整段进 URL ⇒ 后端逐段白名单判非法 ⇒
      // `HTTP 404 {"message":"无效的手册资源路径"}` ⇒ 图**静默消失**。
      expect(extractImagePaths('排气操作 ' + CAPTIONED_MARKER)).toEqual([FAULT_PATH]);
    });
    it('半角/全角标签与无标签的裸 `|` 都按第一个 `|` 截断', () => {
      expect(extractImagePaths('<<IMAGE:manual/d/p.png| 描述：接线图>>')).toEqual(['d/p.png']);
      expect(extractImagePaths('<<IMAGE:manual/d/p.png| caption: 接线图 >>')).toEqual(['d/p.png']);
      expect(extractImagePaths('<<IMAGE:manual/d/p.png| 接线图>>')).toEqual(['d/p.png']);
      expect(extractImagePaths('<<IMAGE:manual/d/p.png|a|b>>')).toEqual(['d/p.png']);
    });
    it('路径段自身含冒号时不误剥（冒号只在 `|` 之后才是标签）', () => {
      // 反例防线：截断若写成「按冒号切」，这个文档名会被啃掉一段。
      expect(extractImagePaths('<<IMAGE:manual/d/p:.png>>')).toEqual(['d/p:.png']);
    });
  });

  describe('stripImageMarkers：标记不进正文（来源面）', () => {
    const { stripImageMarkers } = display();

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

  // ── #1279 自测点 3：正文标记展开（气泡是纯文本渲染，展开为 alt = root ADR-0046「仍可读降级」）──
  describe('expandImageMarkers：正文标记展开为图片说明', () => {
    const { expandImageMarkers, stripImageMarkers } = display();

    it('有描述 → 留下描述，标记与路径都不进正文', () => {
      expect(expandImageMarkers('先排空。' + CAPTIONED_MARKER + '再目视。')).toBe(
        '先排空。蓄能器接口再目视。');
    });
    it('无描述（来源面常态）→ 退回 `诊断配图`，**绝不**把内网路径露出来', () => {
      const out = expandImageMarkers('见' + FAULT_MARKER);
      expect(out).toBe('见诊断配图');
      expect(out).not.toContain('/assistant/static/');
      expect(out).not.toContain('fault_images');
      expect(out).not.toContain('1721219449286');
      expect(out).not.toContain('<<IMAGE:');
    });
    it('占位词与后端正文归一的 alt 缺省同词（三端一致，不各造一个占位词）', () => {
      // 判据源：`backend/internal/service/ai_diagnosis_adapter.go` 的 `normalizeDiagnosisImages`
      // 在 alt 为空时写「诊断配图」。这里断言**产出物**，不是模块内私有常量的字面量。
      expect(expandImageMarkers(FAULT_MARKER)).toBe('诊断配图');
    });
    it('多标记各自展开，正文其余部分不动', () => {
      expect(expandImageMarkers('A' + FAULT_MARKER + 'B<<IMAGE:manual/d/p.png | 描述:接线图>>C')).toBe(
        'A诊断配图B接线图C');
    });
    it('残缺标记（只有左定界符）原样保留，不吞后半段', () => {
      expect(expandImageMarkers('a<<IMAGE:/assistant/static/manual/x/')).toBe(
        'a<<IMAGE:/assistant/static/manual/x/');
    });
    it('**无标记时逐字节不变**（含首尾空白）⇒ 每条用户消息零 diff', () => {
      // 这条是「未命中调用方零 diff」的行为面：展开函数若顺手 trim，
      // 用户消息的尾部空格与换行会在气泡里悄悄变样。
      expect(expandImageMarkers('  纯文本  \n')).toBe('  纯文本  \n');
      expect(expandImageMarkers('')).toBe('');
    });
    it('展开 ≠ 删除：同一份文本两个函数结果**必须**不同（防把来源面口径抄进正文）', () => {
      // `stripImageMarkers` 服务来源卡（路径另走代理图，正文留字符串会重复）；
      // 正文面没有代理图，删掉就等于「一张图都没有且不知道讲了什么」。
      expect(stripImageMarkers(CAPTIONED_MARKER)).toBe('');
      expect(expandImageMarkers(CAPTIONED_MARKER)).toBe('蓄能器接口');
    });
  });

  describe('pageLabel：页码标签', () => {
    const { pageLabel } = display();

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

// ===== ③ 结构面（接线）：行为已由 ② 兜底，这里只钉「形状没长歪」=====

describe('aiSourcesDisplay：结构约束（接线面，行为由上面各组真执行兜底）', () => {
  it('源码在 utils/ 下，且不依赖 api（utils 不反向依赖 api 层）', () => {
    expect(SRC).toContain('export function stripAssistantPrefix');
    expect(SRC).toContain('export function extractImagePaths');
    expect(SRC).toContain('export function stripImageMarkers');
    expect(SRC).toContain('export function expandImageMarkers');
    expect(SRC).toContain('export function pageLabel');
    expect(SRC).not.toContain("from '../api/");
    // 只禁**依赖**（import 行），不禁「注释里提到这个名字」—— 本文件第一版写成
    // `not.toContain('aiManualUrl')`，被自己文档注释里那句「URL 由 … 的 `aiManualUrl()` 负责」
    // 误判成违规（假阳性）。这是当时第三次「判据撞上注释」，故锚点收窄到 import 行。
    expect(SRC).not.toMatch(/^import .*aiManualUrl/m);
  });

  it('正文展开的实现只在 utils 单点（组件/页面不得长出第二实现）', () => {
    expect(SRC).toContain('const caption = captionSegmentOf(inner)');
    // 接线面（气泡有没有真的调用它）归 `aiDiagnosisSourcesContract.test.js` ⑥：
    // 那是契约缝的活，放这儿会变成「单测替接线背书」——而接线守护不算 ③ 证据。
  });

  it('源码里没有正则字面量（全仓 .uts 无 RegExp 先例，Kotlin 目标下不可靠）', () => {
    expect(SRC).not.toContain('new RegExp(');
    expect(SRC).not.toMatch(/=\s*\/[^/\n]*<<IMAGE/);
  });

  it('扫描用 indexOf/substring 而不是正则', () => {
    expect(SRC).toContain('text.indexOf(IMAGE_OPEN');
    expect(SRC).toContain('text.substring(');
  });
});

// ===== ④ 成对取证（必红）：四种**写得出来**的坏实现，逐条注入真执行 =====

/**
 * 读真源 → 注入变异 → 落到临时目录 → 返回变异副本并真执行（工作树不动）。
 * 锚点失效即红：变异**必须真的进去了**，否则这条取证是空跑（spec §④ 更正 2 的验收口径）。
 */
function loadMutated(replacements) {
  let src = SRC;
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-sources-display-'));
  const file = path.join(dir, path.basename(UTS));
  fs.writeFileSync(file, src);
  return loadUts(file, {});
}

describe('aiSourcesDisplay：成对取证（本套件的判据在坏实现上确实会红）', () => {
  it('必不红（对照组）：真源上四条判据各自成立', () => {
    const m = display();
    expect(m.extractImagePaths(CAPTIONED_MARKER)).toEqual([FAULT_PATH]);
    expect(m.extractImagePaths(FAULT_MARKER)).toEqual([FAULT_PATH]);
    expect(m.expandImageMarkers(CAPTIONED_MARKER)).toBe('蓄能器接口');
    expect(m.expandImageMarkers('  纯文本  \n')).toBe('  纯文本  \n');
  });

  it('必红 · 截断没接上（按第一个 `|` 截断退化为不截断）⇒ 后缀整段进路径', () => {
    // 坏实现 = #1279 实测的那个失效形态（后端判非法 ⇒ 图静默消失）。
    // 锚点带函数头，只改 `pathSegmentOf` 那一处 —— `captionSegmentOf` 里有同形一行，
    // 一并改掉会把两种坏混成一条取证。
    const broken = loadMutated([[
      'function pathSegmentOf(inner : string) : string {\n\tconst bar = inner.indexOf(MARKER_CAPTION_SEP)',
      'function pathSegmentOf(inner : string) : string {\n\tconst bar = -1',
    ]]);
    expect(broken.extractImagePaths(CAPTIONED_MARKER))
      .toEqual([FAULT_PATH + ' | 描述:蓄能器接口']);
  });

  it('必红 · 「顺手把新根也剥掉」⇒ 第二根不再留给后端代理（后端只认带根形状）', () => {
    // 坏实现 = 看到 20260921 新增根后最自然的错误写法：往可剥前缀里再加一条。
    // 后果是产物变成 `制动系统/….png`（无根）⇒ 后端归一到 `manual/` 根下 ⇒ 404。
    const broken = loadMutated([[
      "\tif (p.startsWith(MANUAL_PREFIX)) p = p.substring(MANUAL_PREFIX.length)\n",
      "\tif (p.startsWith(MANUAL_PREFIX)) p = p.substring(MANUAL_PREFIX.length)\n"
      + "\tif (p.startsWith('fault_images/')) p = p.substring('fault_images/'.length)\n",
    ]]);
    expect(broken.extractImagePaths(FAULT_MARKER)).toEqual(['制动系统/1721219449286.png']);
  });

  it('必红 · 展开写成删除（把来源面口径抄进正文）⇒ 图片说明整段丢失', () => {
    const broken = loadMutated([[
      "\t\tout += caption.length > 0 ? caption : IMAGE_ALT_FALLBACK\n",
      '',
    ]]);
    expect(broken.expandImageMarkers('先排空。' + CAPTIONED_MARKER + '再目视。')).toBe('先排空。再目视。');
    expect(broken.expandImageMarkers(FAULT_MARKER)).toBe('');
  });

  it('必红 · 展开顺手 trim ⇒ 「无标记逐字节不变」的零 diff 判据落空', () => {
    // 锚点带展开那一行的上下文：`\treturn out\n}` 在 `extractImagePaths` 里同形出现，
    // 只写它会同时改掉两个函数（Array 上 `.trim()` 直接抛，取证就混进了别的坏）。
    const ANCHOR = 'out += caption.length > 0 ? caption : IMAGE_ALT_FALLBACK\n'
      + '\t\tfrom = end + IMAGE_CLOSE.length\n\t}\n\treturn out\n}';
    const broken = loadMutated([[ANCHOR, ANCHOR.replace('return out\n}', 'return out.trim()\n}')]]);
    expect(broken.expandImageMarkers('  纯文本  \n')).toBe('纯文本');
  });
});
