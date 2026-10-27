/**
 * ④ 错误行判据的**接线契约**守护（#2045）—— 接线守护（读源码文本，不起子进程）。
 *
 * 分工（两层各防各的瞎，谁都不许替谁）：
 *   · `utils/gateErrorLinesBehavior.test.js`（行为腿）= 真执行判据，防「判据认不出历史错题」；
 *   · 本文件（接线腿）= 读源码文本，防「门绕开了判据」——判据住在 lib 里但门不再调它，
 *     题库照样全绿，而门已经瞎了。这一层只有文本断言能做，也只有它该做（抄正则内容就会
 *     制造第二份判据，票面明令禁止）。
 *
 * 守住的五条：
 *   W1 两份门脚本都 dot-source `scripts/lib/gate-error-lines.ps1`
 *   W2 调用点在**真路径**上：判据调用先于门自己的结论行与判红分支（不是只活在某个不走的分支里）
 *   W3 lib 里三个判据函数各**恰好定义一次**，且返回 `@()` 包裹（StrictMode 下裸管道退化成 $null）
 *   W4 脚本内不得再留判据字面量（第二真源；`lib/hx-errors.ps1` 那条第三形态除外，不在本票射程）
 *   W5 lib 必须**零副作用**（不读文件、不起进程、不碰 HBuilderX/设备）—— 这是「可 dot-source 直测」的前提，
 *      也是票面「零副作用 lib 纯函数」这条硬约束的机检落点
 *
 * 设计沿用本仓契约守护形态（见 utils/kotlinAllGateContract.test.js）：先对**注入违规的变形样本**
 * 断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/gate-error-lines.ps1';
const COMPILE_REL = 'scripts/compile-check.ps1';
const KOTLIN_REL = 'scripts/kotlin-all-check.ps1';

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/**
 * 剥掉 PowerShell 的两类注释（文档块与整行注释）。
 * 为什么必须先剥：`kotlin-all-check.ps1` 的头注释里有一句「整模块编译必然报大量 unresolved reference」——
 * 那是散文不是判据，不剥就会把 W4 误判成违规；反过来，注释里提「零副作用」也骗不过 W5。
 */
function stripPsComments(text) {
  return String(text)
    .replace(/<#[\s\S]*?#>/g, '')
    .split(/\r?\n/)
    .filter((line) => !/^\s*#/.test(line))
    .join('\n');
}

/** 判据字面量的强特征（任一模子出现在**代码**里就说明脚本还留着一份判据）。 */
const CRITERIA_LITERALS = [
  'unresolved reference',
  'cannot infer type',
  '找不到名称',
  '类型不匹配',
  'Could not resolve',
  'ErrorLinePattern\\s*=',
];

/** 有副作用的 cmdlet 前缀：判据库一旦碰这些，就无法在任意机器上 dot-source 断言。 */
const SIDE_EFFECT_RE = /\b(Get-Content|Set-Content|Add-Content|Get-ChildItem|New-Item|Remove-Item|Test-Path|Invoke-|Start-|Stop-|Out-File|Write-Host|adb|cli\.exe)\b/;

/** 纯函数：三份源码文本（注释已剥）→ 违规清单 */
function scanContract(sources) {
  const violations = [];
  const lib = stripPsComments(sources.lib || '');
  const compile = stripPsComments(sources.compile || '');
  const kotlin = stripPsComments(sources.kotlin || '');
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  // W1 两份门脚本都接上了判据库
  [[COMPILE_REL, compile], [KOTLIN_REL, kotlin]].forEach(([rel, code]) => {
    must(/lib[\\/]gate-error-lines\.ps1/.test(code), 'W1', rel + ' 未 dot-source scripts/lib/gate-error-lines.ps1');
  });

  // W2 调用点在真路径上：判据 → 机检结论行的**数字** → 判红分支，次序不许倒过来。
  // ⚠️ 锚点选的是 `errors=$($errorLines.Count)` 这一枚（结论行里那个数出自判据结果），
  //    不是裸的 `COMPILE_RESULT` / `KOTLIN_ALL_RESULT` 字样 —— 后者在**环境早退分支**里
  //    先于调用出现（`errors=env`），拿它排序会把真接线判成违规（写本守护时实测撞过）。
  const COUNT_ANCHOR = 'errors=$($errorLines.Count)';
  [[COMPILE_REL, compile, 'Get-CompileGateErrorLines', '$errorLines = @(Get-CompileGateErrorLines'], [KOTLIN_REL, kotlin, 'Get-KotlincGateErrorMatches', '$errorLines = @(Get-KotlincGateErrorMatches']].forEach(
    ([rel, code, fn, decl]) => {
      const callAt = code.indexOf(decl);
      must(callAt > -1, 'W2', rel + ' 没有把 $errorLines 交给 ' + fn + '（门可能仍在自判）');
      if (callAt > -1) {
        const resultAt = code.indexOf(COUNT_ANCHOR);
        const gateAt = code.search(/if\s*\(\s*\$errorLines\.Count\s+-gt\s+0\s*\)/);
        must(resultAt > callAt, 'W2', rel + ' 的机检结论行数字不出自判据调用（结论行未排在调用之后）');
        must(gateAt > callAt, 'W2', rel + ' 的判红分支没用判据调用的结果（$errorLines.Count 不来自它）');
      }
    }
  );
  must(/Get-KotlincGateErrorMatches\b/.test(kotlin), 'W2', KOTLIN_REL + ' 未调用计数判据');
  must(/Select-KotlincGateErrorLines\b/.test(kotlin), 'W2', KOTLIN_REL + ' 未调用打印判据（失败日志的行应与计数同源）');
  const kotlinGateAt = kotlin.search(/if\s*\(\s*\$errorLines\.Count\s+-gt\s+0\s*\)/);
  if (kotlinGateAt > -1) {
    must(kotlin.indexOf('Select-KotlincGateErrorLines') > kotlinGateAt,
      'W2', KOTLIN_REL + ' 的打印判据不在判红分支里（失败日志可能与计数不同源）');
  }

  // W3 lib：三个判据函数各恰好一次，且都返回 @() 包裹
  ['Get-CompileGateErrorLines', 'Get-KotlincGateErrorMatches', 'Select-KotlincGateErrorLines'].forEach((fn) => {
    const defs = (lib.match(new RegExp('function ' + fn + '\\b', 'g')) || []).length;
    must(defs === 1, 'W3', LIB_REL + ' 里 ' + fn + ' 定义了 ' + defs + ' 次（应恰好 1 次）');
    const body = lib.slice(lib.indexOf('function ' + fn));
    const nextDef = body.slice(11).search(/function\s+[A-Za-z]/);
    const isolated = nextDef > -1 ? body.slice(0, nextDef + 11) : body;
    must(/return\s+@\(/.test(isolated), 'W3', fn + ' 未用 @() 包裹返回值（空命中时 StrictMode 下 $null.Count 会抛）');
  });

  // W4 两份门脚本不得再内联判据字面量（第二真源）
  [[COMPILE_REL, compile], [KOTLIN_REL, kotlin]].forEach(([rel, code]) => {
    CRITERIA_LITERALS.forEach((token) => {
      const hit = token.includes('\\') ? new RegExp(token).test(code) : code.includes(token);
      must(!hit, 'W4', rel + ' 的代码里仍留有判据字面量：' + token + '（判据唯一住 ' + LIB_REL + '）');
    });
  });

  // W5 lib 零副作用
  const sideEffect = lib.match(SIDE_EFFECT_RE);
  must(!sideEffect, 'W5', LIB_REL + ' 含副作用调用（' + (sideEffect ? sideEffect[0] : '') + '）——判据必须能在任何机器上 dot-source 断言');

  return violations;
}

describe('④ 错误行判据的接线契约（#2045，防「门绕开判据」）', () => {
  const real = {
    lib: readSource(LIB_REL),
    compile: readSource(COMPILE_REL),
    kotlin: readSource(KOTLIN_REL),
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      // W1：拆掉 dot-source
      ['W1', { ...real, compile: real.compile.replace(/lib[\\/]gate-error-lines\.ps1/g, 'lib/gate-other.ps1') }],
      ['W1', { ...real, kotlin: real.kotlin.replace(/lib[\\/]gate-error-lines\.ps1/g, 'lib/gate-other.ps1') }],
      // W2：调用点改名（门自己另判一套）/ 结论行倒到调用之前 / 打印判据没接上
      ['W2', { ...real, compile: real.compile.replace(/\$errorLines\s*=\s*@\(\s*Get-CompileGateErrorLines/, '$whatever = @(Get-CompileGateErrorLines') }],
      ['W2', { ...real, compile: real.compile.replace('errors=$($errorLines.Count)', 'errors=fixed-number') }],
      ['W2', { ...real, kotlin: real.kotlin.replace('errors=$($errorLines.Count)', 'errors=fixed-number') }],
      ['W2', { ...real, kotlin: real.kotlin.replace('Select-KotlincGateErrorLines -Output $result.Output | ', '') }],
      ['W2', { ...real, kotlin: real.kotlin.replace(/Get-KotlincGateErrorMatches/g, 'Get-WhateverMatches') }],
      ['W2', { ...real, kotlin: real.kotlin.replace(/Select-KotlincGateErrorLines/g, 'Write-WhateverLines') }],
      // W3：判据函数被复制 / 去掉 @() 包裹
      ['W3', { ...real, lib: real.lib + '\nfunction Get-CompileGateErrorLines { param([string]$Output) return @() }\n' }],
      ['W3', { ...real, lib: real.lib.replace('return @([regex]::Matches("$Output", $KotlincGateErrorLinePattern)', 'return ([regex]::Matches("$Output", $KotlincGateErrorLinePattern)') }],
      // W4：把判据字面量抄回脚本（第二真源的诞生形状）
      ['W4', { ...real, kotlin: real.kotlin + "\n$ErrorLinePattern = '(?m)(^\\s*e: )'\n" }],
      ['W4', { ...real, compile: real.compile + "\n$x = 'unresolved reference|cannot infer type'\n" }],
      // W5：判据库开始读文件
      ['W5', { ...real, lib: real.lib + '\n$log = Get-Content -LiteralPath $env:HAXBUILD -Raw\n' }],
    ];
    cases.forEach(([rule, sources], caseIndex) => {
      const found = scanContract(sources);
      expect({ caseIndex, rule, detected: found.some((v) => v.startsWith(rule)) })
        .toEqual({ caseIndex, rule, detected: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('接线写的是函数名、不是判据内容（本守护不成为第二份判据）', () => {
    // 反向自证：本文件里出现不了任何一条判据 token 的**完整模子**，
    // 否则「判据改措辞」与「判据被拆接线」两件事就会混在同一份文本里各漂各的。
    const self = readText(path.join(__dirname, 'gateErrorLinesWiringContract.test.js'));
    ['(?i)\\berror\\b|unresolved reference', '(:\\d+:\\d+: *error:)'].forEach((pattern) => {
      expect(self).not.toContain(pattern);
    });
  });
});
