/**
 * 微信小程序「元素级 API 可用性」探针（scripts/mp-weixin-element-probe.mjs）契约守护
 *
 * 背景：仓库里对同一件事有两份**互相矛盾**的实测记录 —— #883 的探针输出说导航 API 都 ok、
 * 只有 `$("view")` 挂起 15s；而 `docs/adr/0008:134/137` 把「元素级断言」整体判为不可用。
 * #883 测的是**标签选择器**，类选择器（任何真实「点了没跳转」用例都要用的那个）**从未被实测过**。
 * `scripts/mp-weixin-element-probe.mjs` 就是裁决这件事的一次性 spike 工具。
 *
 * 为什么它需要**自己的**守护（而不是并进 mpWeixinGateContract.test.js）：
 *   ② 门的探针受 C3 守护「不得用元素级 API」，理由正是 15s 挂起会让门时好时坏。
 *   本探针**故意**要用元素级 API ⇒ 它必须与门**反向解耦**，且必须满足下面这些不变量，
 *   否则它会变成「第四个永绿」：一行看起来很专业的绿字，实际什么都没证明。
 *
 *   E1 结果行前缀唯一（`MP_WEIXIN_ELEMENT_PROBE`），且**不得**输出门的结果行（`MP_WEIXIN_PROBE`）
 *   E2 **已知必败对照项**（`control.absent`：不存在的 class 必须取不到元素）；对照失败时
 *      `trustworthy=false`，且 `computeVerdict` **必须先**判 `!trust.ok` —— 即**结论不得在对照失败时产出**
 *   E3 每步都走统一超时包装，且**默认步超时 < 15000ms**（#883 的挂起要被判成 TIMEOUT 而不是拖死）
 *   E4 **判据预登记**：结论字符串与对应 Q15 影响逐字在位，且不得出现未登记的结论
 *   E5 **与 ② 门反向解耦**：门脚本 `mp-weixin-check.ps1` **不得**引用本探针
 *   E6 导航 API 的**原始错误串**必须被捕获（裁决 ADR「恒报 Uncaught [object Object]」要用原文，不能用归纳）
 *   E7 显式声明**非门证据**（`isGate: false` + 头部声明「spike 工具，不是门」）
 *   E8 `package.json` 注册入口 `probe:mp-weixin-element`（否则没人跑得到它）
 *
 * 设计沿用本仓既有守护测试的形态：先对「注入违规」的变形样本断言检测有效（防空跑假绿），
 * 再对真实文件断言零命中。
 *
 * 纯文件断言（不跑 pwsh、不连设备）⇒ Windows 本机与 CI 的 ubuntu runner 行为一致。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const PROBE_REL = 'scripts/mp-weixin-element-probe.mjs';
const GATE_SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const PKG_REL = 'package.json';

const RESULT_PREFIX = 'MP_WEIXIN_ELEMENT_PROBE';
/** 门的结果行前缀（本探针**不得**输出它，否则会被 dev:finish / 门脚本误当成门结论）。 */
const GATE_RESULT_PREFIX = 'MP_WEIXIN_PROBE';
const ABSENT_STEP_ID = 'control.absent';
const ABSENT_DEFAULT_SELECTOR = '.zzz-probe-absent-class-9f3a';
const MAX_STEP_TIMEOUT_MS = 15000;

/** 判据预登记表：结论 → 该结论对 Q15 的处置（逐字在位，防止事后挪门槛）。 */
const VERDICT_IMPACT = {
  'class-selector-and-tap-flow-usable': 'Q15：类选择器 + 点击流程可用',
  'class-selector-usable-text-read-failed': 'Q15：类选择器可解析但文本读取失败',
  'class-selector-usable-tap-flow-unproven': 'Q15：类选择器可用、点击流程未证实',
  'class-selector-unusable': 'Q15：类选择器亦不可用',
  'inconclusive-control-failed': 'Q15：**不判**',
  'env-unavailable': 'Q15：**不判**',
};

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

/** 注释行置空（保留行号）：判据只对**代码行**成立，注释里提 API 名不算违规。 */
function codeOnly(text) {
  return String(text)
    .split(/\r?\n/)
    .map((l) => (/^\s*\/\//.test(l) ? '' : l))
    .join('\n');
}

/** 纯函数：源码文本 → 违规清单（便于用注入样本自检检测有效）。 */
function scanContract(sources) {
  const violations = [];
  const probeText = sources.probe || '';
  const probe = codeOnly(probeText);
  const gateScript = sources.gateScript || '';
  const pkg = sources.pkg || '';
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  // E1 结果行前缀唯一
  must(probe.includes("'" + RESULT_PREFIX + " '"), 'E1', '未输出 ' + RESULT_PREFIX + ' 结果行');
  must(!probe.includes("'" + GATE_RESULT_PREFIX + " '"),
    'E1', '输出了门的结果行 ' + GATE_RESULT_PREFIX + '（会被误当成 ② 门的结论）');

  // E2 已知必败对照项 + 对照失败即不作结论
  must(probe.includes("'" + ABSENT_STEP_ID + "'"), 'E2', '缺已知必败对照项 step id ' + ABSENT_STEP_ID);
  must(probe.includes(ABSENT_DEFAULT_SELECTOR), 'E2', '对照项默认选择器不在（对照项可能被改成恒真有元素）');
  must(probe.includes('trustworthy') && probe.includes('trustReason'),
    'E2', '缺 trustworthy / trustReason（对照结论没有落点）');
  const verdictGuardAt = probe.indexOf("if (!trust.ok) return 'inconclusive-control-failed';");
  const firstConclusionAt = probe.indexOf("if (m.classResolve !== 'resolved')");
  must(verdictGuardAt !== -1, 'E2', 'computeVerdict 未先判 !trust.ok —— 对照失败时仍会产出结论（假绿）');
  must(verdictGuardAt !== -1 && firstConclusionAt !== -1 && verdictGuardAt < firstConclusionAt,
    'E2', 'computeVerdict 里 !trust.ok 判据**晚于**结论判据（顺序反了）');

  // E3 统一超时 + 默认步超时 < 15000
  must(probe.includes('withTimeout('), 'E3', '缺统一超时包装 withTimeout（会有步骤无限挂起）');
  const m = probe.match(/stepTimeoutMs:\s*(\d+)/);
  must(Boolean(m), 'E3', '找不到默认步超时 stepTimeoutMs 声明');
  if (m) {
    must(Number(m[1]) < MAX_STEP_TIMEOUT_MS, 'E3',
      `默认步超时 ${m[1]}ms 未小于 ${MAX_STEP_TIMEOUT_MS}ms（#883 的 15s 挂起会拖死探针，且"挂起"与"报错"分不开）`);
  }
  must(probe.includes('TIMEOUT('), 'E3', '缺 TIMEOUT 签名判定（超时与报错分不开）');

  // E4 判据预登记（三层：表里全 → 说明全 → **computeVerdict 只产出表里登记的**）
  // 第三层是关键：只查「表里有这个键」会被 `String.replace` 的第一处命中骗过 ——
  // 必须反过来查「每个 return 字面量都是已登记的键」，才能挡住「新增结论绕过预登记」。
  const verdictKeys = Object.keys(VERDICT_IMPACT);
  // ⚠️ 必须把断言**限定在 VERDICT_IMPACT 对象字面量内部**：整文件 `includes` 会被
  // `computeVerdict` 里的 return 字面量满足，于是「把表里的键删掉」这条漂移根本检不出来
  // （本守护的注入用例 #8 就是踩到这个才发现的）。
  const tableStart = probe.indexOf('const VERDICT_IMPACT');
  const tableEnd = tableStart > -1 ? probe.indexOf('};', tableStart) : -1;
  const table = tableEnd > tableStart ? probe.slice(tableStart, tableEnd) : '';
  must(table !== '', 'E4', '找不到 VERDICT_IMPACT 对象字面量（判据表结构变了？）');
  verdictKeys.forEach((v) => {
    must(table.includes("'" + v + "'"), 'E4', '预登记表 VERDICT_IMPACT 里缺结论：' + v);
  });
  Object.values(VERDICT_IMPACT).forEach((impact) => {
    must(probeText.includes(impact), 'E4', '结论缺对应的 Q15 影响说明：' + impact);
  });
  must(probe.includes('const VERDICT_IMPACT'), 'E4',
    '缺判据表声明 const VERDICT_IMPACT（结论→影响映射表被删/改名 —— 判据散落回分支里就会改一处漏一处）');
  {
    const body = probe.slice(probe.indexOf('function computeVerdict'));
    const cut = body.indexOf('\n}');
    const cv = cut > -1 ? body.slice(0, cut) : body;
    const returns = [...cv.matchAll(/return\s+'([^']+)'/g)].map((mm) => mm[1]);
    must(returns.length > 0, 'E4', 'computeVerdict 里找不到 return 字面量（判据被抽走了？）');
    returns.forEach((r) => {
      must(verdictKeys.includes(r), 'E4', 'computeVerdict 产出了未登记的结论：' + r);
    });
    must(returns.length === new Set(returns).size, 'E4', 'computeVerdict 有重复 return（分支可能被复制粘贴改错）');
  }

  // E5 与 ② 门反向解耦
  must(!gateScript.includes('mp-weixin-element-probe'),
    'E5', '② 门脚本引用了本探针 —— 元素级 API 会被带进门里（C3 守护的理由正是不许）');

  // E6 原始错误串（⚠️ 必须用词边界：`stringifiedX` 仍**包含子串** `stringified`，
  //    纯 `includes` 会被这种改名骗过 —— 注入用例 #12 就是踩到这个才发现断言太弱）
  must(/\brawError\b/.test(probe) && /\bstringified\b/.test(probe),
    'E6', '缺原始错误串捕获（裁决 ADR「恒报 Uncaught [object Object]」需要原文，不能只用归纳）');
  must(probe.includes('navigationContradiction'), 'E6', '缺两份记录矛盾点的裁决字段');

  // E7 显式非门声明
  must(probe.includes('isGate: false'), 'E7', '未声明 isGate: false（会被读成门证据）');
  must(probeText.includes('spike 工具，不是门'), 'E7', '头部未声明「spike 工具，不是门」');

  // E8 入口注册
  must(pkg.includes('"probe:mp-weixin-element"'), 'E8', 'package.json 未注册 probe:mp-weixin-element 入口');
  must(pkg.includes('mp-weixin-element-probe.mjs'), 'E8', 'package.json 入口未指向本探针');

  return violations;
}

const real = {
  probe: readSource(PROBE_REL),
  gateScript: readSource(GATE_SCRIPT_REL),
  pkg: readSource(PKG_REL),
};

describe('微信小程序元素级探针契约守护（E1–E8）', () => {
  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['E1', { ...real, probe: real.probe.replace("'MP_WEIXIN_ELEMENT_PROBE '", "'NOTHING '") }],
      ['E1', { ...real, probe: real.probe + "\nconsole.log('MP_WEIXIN_PROBE ' + JSON.stringify({}));\n" }],
      ['E2', { ...real, probe: real.probe.replace("'control.absent'", "'control.absentX'") }],
      ['E2', { ...real, probe: real.probe.replace(ABSENT_DEFAULT_SELECTOR, '.title') }],
      ['E2', { ...real, probe: real.probe.replace("if (!trust.ok) return 'inconclusive-control-failed';", 'if (false) return null;') }],
      ['E3', { ...real, probe: real.probe.replace('stepTimeoutMs: 8000', 'stepTimeoutMs: 20000') }],
      ['E3', { ...real, probe: real.probe.replace(/withTimeout\(/g, 'noTimeout(') }],
      ['E3', { ...real, probe: real.probe.replace("'TIMEOUT('", "'X('") }],
      ['E4', { ...real, probe: real.probe.replace("'class-selector-unusable'", "'whatever'") }],
      ['E4', { ...real, probe: real.probe.replace("return 'class-selector-and-tap-flow-usable';", "return 'brand-new-verdict';") }],
      // ⚠️ 锚点必须是**声明语句**：`VERDICT_IMPACT` 这个词第一次出现在头部注释里，
      //    直接 replace 那个词只会改到注释（而 codeOnly 把注释置空）⇒ 注入用例空跑、假绿。
      ['E4', { ...real, probe: real.probe.replace('const VERDICT_IMPACT', 'const IMPACT_X') }],
      ['E5', { ...real, gateScript: real.gateScript + "\n# scripts/mp-weixin-element-probe.mjs\n" }],
      ['E6', { ...real, probe: real.probe.replace(/stringified/g, 'stringifiedX') }],
      ['E7', { ...real, probe: real.probe.replace('isGate: false', 'isGate: true') }],
      ['E8', { ...real, pkg: real.pkg.replace('probe:mp-weixin-element', 'probe:mp-x') }],
    ];
    cases.forEach(([rule, sources], caseIndex) => {
      const found = scanContract(sources);
      // 断言里带上序号与规则名：注入用例一旦失效，失败信息要能直接指出是**哪一条**
      expect({ caseIndex, rule, detected: found.some((v) => v.startsWith(rule)) })
        .toEqual({ caseIndex, rule, detected: true });
    });
  });

  it('E2：对照项（control.absent）必须在类选择器测量**之前**执行（对照先于测量）', () => {
    const controlAt = real.probe.indexOf("'control.absent'");
    const measureAt = real.probe.indexOf("'measure.classResolve'");
    expect(controlAt).toBeGreaterThan(-1);
    expect(measureAt).toBeGreaterThan(-1);
    expect(controlAt).toBeLessThan(measureAt);
  });

  it('E5：② 门脚本与本探针反向解耦（双向断言）', () => {
    expect(real.gateScript).not.toContain('mp-weixin-element-probe');
    // 反向：门脚本仍在用**它自己的**探针（解耦不等于把门拆了）
    expect(real.gateScript).toContain('scripts\\mp-weixin-probe.mjs');
  });

  it('E4：结论集恰好是预登记的那 6 个（多一个少一个都算漂移）', () => {
    const found = [...real.probe.matchAll(/'(class-selector-[a-z-]+|inconclusive-control-failed|env-unavailable)'/g)]
      .map((m) => m[1]);
    const uniq = [...new Set(found)].sort();
    expect(uniq).toEqual(Object.keys(VERDICT_IMPACT).sort());
  });
});
