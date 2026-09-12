/**
 * 门脚本共享库契约守护（scripts/lib/gate-common.ps1）
 *
 * 背景（2026-09-13 实测，PR #913 记录）：门脚本家族存在成规模重复 —— `Test-PeHeader` 4 份、
 * `Write-Log` 4 份、`Get-HeadSha` / `Save-Jpeg` / `Get-WebpEncoder` / `Publish-GateComment` 各 3 份。
 * 但**只有 `Get-HeadSha` 被上移**，因为它是唯一满足共享库准入门槛的：
 *   G1 各副本**逐字节相同**（412 B / 14 行，实测三份完全一致）；
 *   G2 **没有被任何 utils/*Contract.test.js 作为字面量锚点 pin 住**（`Get-HeadSha` 在守护里 0 命中）。
 * 门槛存在的原因是 G3：本仓守护断言**源码文本**，而 `mpWeixinGateContract` 正好 pin 住
 * `Invoke-Process -FilePath $devTools -Arguments @('auto'` 这类**调用点**原文 ⇒ 把被 pin 的代码
 * 搬进共享库会让守护失配，唯一的「修法」是放宽守护（等于拆掉防护）。故本守护同时守住
 * 「共享库有准入门槛、且不得被悄悄绕过」这一条，防止后续会话顺手把 `Test-PeHeader` 也搬进去。
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/hxBusyGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/gate-common.ps1';
// 三份曾各自定义 Get-HeadSha 的门脚本（compile-check / kotlin-all-check / mp-weixin-check）
const CALLERS = [
  'scripts/compile-check.ps1',
  'scripts/kotlin-all-check.ps1',
  'scripts/mp-weixin-check.ps1',
];

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

/** 纯函数：共享库 + 调用方源码 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };
  const lib = sources.lib || '';
  const callers = sources.callers || {};

  // G1 共享库必须真的定义了这个函数，且是**唯一**一处定义
  must(lib.includes('function Get-HeadSha'), 'G1', LIB_REL + ' 未定义 Get-HeadSha');
  const defs = (lib.match(/function Get-HeadSha/g) || []).length;
  must(defs === 1, 'G1', LIB_REL + ' 里 Get-HeadSha 定义了 ' + defs + ' 次（应恰好 1 次）');

  // G2 每个调用方都必须 dot-source 共享库，且**不得**再自己定义（防重复长回来）
  Object.entries(callers).forEach(([rel, text]) => {
    const t = text || '';
    must(/lib[\\/]gate-common\.ps1/.test(t), 'G2', rel + ' 未 dot-source scripts/lib/gate-common.ps1');
    must(!/function Get-HeadSha/.test(t), 'G2', rel + ' 又自定义了 Get-HeadSha（重复长回来了，应只用共享库那份）');
  });

  // G3 准入门槛必须写在共享库头部：没有它，后续会话会顺手把被守护 pin 住的函数也搬进来
  must(lib.includes('准入门槛'), 'G3', LIB_REL + ' 头部未写明「准入门槛」');
  must(lib.includes('逐字节相同'), 'G3', LIB_REL + ' 未写明准入条件①（逐字节相同）');
  must(lib.includes('pin'), 'G3', LIB_REL + ' 未写明准入条件②（未被守护作为字面量锚点 pin 住）');
  must(lib.includes('Test-PeHeader'), 'G3', LIB_REL + ' 未列出「明确未入库」的反例（Test-PeHeader）');

  return violations;
}

describe('门脚本共享库契约（scripts/lib/gate-common.ps1）', () => {
  const real = {
    lib: readSource(LIB_REL),
    callers: Object.fromEntries(CALLERS.map((rel) => [rel, readSource(rel)])),
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['G1', { ...real, lib: real.lib.replace('function Get-HeadSha', 'function GetHeadShaX') }],
      ['G1', { ...real, lib: real.lib + '\nfunction Get-HeadSha { return "dup" }\n' }],
      ['G2', {
        ...real,
        callers: { ...real.callers, 'scripts/x.ps1': 'no dot-source here' },
      }],
      ['G2', {
        ...real,
        callers: {
          ...real.callers,
          'scripts/mp-weixin-check.ps1': real.callers['scripts/mp-weixin-check.ps1']
            + '\nfunction Get-HeadSha { return "" }\n',
        },
      }],
      ['G3', { ...real, lib: real.lib.replace(/准入门槛/g, '说明') }],
      ['G3', { ...real, lib: real.lib.replace(/逐字节相同/g, '大致相同') }],
      ['G3', { ...real, lib: real.lib.replace(/Test-PeHeader/g, 'X') }],
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

  it('G2：三个调用方都只用共享库那一份，各自不再定义', () => {
    for (const rel of CALLERS) {
      const t = real.callers[rel];
      expect(t).toMatch(/lib[\\/]gate-common\.ps1/);
      expect(t).not.toMatch(/function Get-HeadSha/);
    }
  });

  it('G3：共享库头部的准入门槛写明了两条准入条件与未入库反例', () => {
    expect(real.lib).toContain('准入门槛');
    expect(real.lib).toContain('逐字节相同');
    expect(real.lib).toContain('pin');
    expect(real.lib).toContain('Test-PeHeader');
  });
});
