/**
 * 门脚本阶段时间戳录制器 —— 接线守护（#2001）。
 *
 * 不变量清单（文本级判据；分支行为判据见 gateStagesBehavior.test.js）：
 *  C0 lib/gate-stages.ps1 导出 Initialize-GateStageRecorder / Start-GateStage / Stop-GateStage。
 *  C1 三份门脚本都 dot-source gate-stages.ps1，且 dot-source 行先于 Initialize 调用（顺序）。
 *  C2 三份门脚本都调 Initialize-GateStageRecorder 且传 -LogPath。
 *  C3 ④a compile-check.ps1 五段齐全且有序：hx-wait < hx-prepare < project-open < compile-launch < audit；
 *      hx-wait 先于 Wait-HxFree、audit 先于读日志判读（Get-Content -Raw）。
 *  C4 ④c kotlin-all-check.ps1 五段齐全：hx-wait/publish/collect/toolchain/kotlinc；
 *      hx-wait 与 publish 都落在 -SkipPublish 不生效的分支内（顺序判据：publish 在
 *      if (-not $SkipPublish) { 之后、hx-wait 在其 else 分支且先于 Wait-HxFree）。
 *  C5 ② mp-weixin-check.ps1 四段齐全：wait-hx/publish/automation/auto-probe；
 *      wait-hx 在第一个 if (-not $SkipBuild) { 内且先于 Wait-HxFree、publish 在第二个该分支之后。
 *  C6 三份门脚本都恰好一次 Stop-GateStage（STAGE_SUMMARY 只出一行）。
 *
 * 读源码一律走 utsHarness.readText（ADR-0019；裸 readFileSync 会被 check-contract-read 判违规）。
 */
const { readText } = require('./utsHarness');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/gate-stages.ps1';
const SCRIPTS = {
  a: 'scripts/compile-check.ps1',
  c: 'scripts/kotlin-all-check.ps1',
  w: 'scripts/mp-weixin-check.ps1',
};

function readSource(rel) { return readText(path.join(ROOT, rel)); }

function startIdx(src, name) {
  const m = new RegExp("Start-GateStage\\b[^\\r\\n]*['\"]" + name + "['\"]").exec(src);
  return m ? m.index : -1;
}

function scanContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  const lib = sources.lib;
  must(/function\s+Initialize-GateStageRecorder/.test(lib), 'C0', 'lib 缺 Initialize-GateStageRecorder');
  must(/function\s+Start-GateStage/.test(lib), 'C0', 'lib 缺 Start-GateStage');
  must(/function\s+Stop-GateStage/.test(lib), 'C0', 'lib 缺 Stop-GateStage');

  for (const [key, rel] of Object.entries(SCRIPTS)) {
    const src = sources.scripts[key];
    const idxDs = src.search(/lib[\\/]gate-stages\.ps1/);
    const idxInit = src.indexOf('Initialize-GateStageRecorder');
    must(idxDs !== -1, 'C1', rel + ' 未 dot-source gate-stages.ps1');
    must(idxDs !== -1 && idxInit !== -1 && idxDs < idxInit, 'C1', rel + ' dot-source 未先于 Initialize');
    must(idxInit !== -1, 'C2', rel + ' 未初始化录制器');
    must(/-LogPath/.test(src), 'C2', rel + ' 初始化缺 -LogPath');
  }

  // C3 ④a：五段齐全 + 相对顺序
  const a = sources.scripts.a;
  const order4a = ['hx-wait', 'hx-prepare', 'project-open', 'compile-launch', 'audit'];
  for (const name of order4a) {
    must(startIdx(a, name) !== -1, 'C3', '④a 缺 ' + name + ' 段');
  }
  must(order4a.map((n) => startIdx(a, n)).every((v, i, arr) => v !== -1 && (i === 0 || v > arr[i - 1])),
    'C3', '④a 段顺序错乱');
  must(startIdx(a, 'hx-wait') !== -1 && startIdx(a, 'hx-wait') < a.indexOf('$hx = Wait-HxFree'),
    'C3', '④a hx-wait 未先于 Wait-HxFree');
  must(startIdx(a, 'audit') !== -1 && startIdx(a, 'audit') < a.indexOf('Get-Content -LiteralPath $logPath -Raw'),
    'C3', '④a audit 未先于日志判读');

  // C4 ④c：五段齐全 + 分支归属（顺序判据）
  const c = sources.scripts.c;
  for (const name of ['hx-wait', 'publish', 'collect', 'toolchain', 'kotlinc']) {
    must(startIdx(c, name) !== -1, 'C4', '④c 缺 ' + name + ' 段');
  }
  const cIfPub = c.indexOf('if (-not $SkipPublish) {');
  const cWait = c.indexOf('$hx = Wait-HxFree');
  const cElse = c.lastIndexOf('} else {', cWait);
  must(cIfPub !== -1 && startIdx(c, 'publish') > cIfPub, 'C4', '④c publish 段不在 -not SkipPublish 分支内');
  must(cElse !== -1 && startIdx(c, 'hx-wait') > cElse && startIdx(c, 'hx-wait') < cWait,
    'C4', '④c hx-wait 不在 Wait-HxFree 所在分支内');

  // C5 ②：四段齐全 + 分支归属（顺序判据）
  const w = sources.scripts.w;
  for (const name of ['wait-hx', 'publish', 'automation', 'auto-probe']) {
    must(startIdx(w, name) !== -1, 'C5', '② 缺 ' + name + ' 段');
  }
  const wIfs = [...w.matchAll(/if \(-not \$SkipBuild\) \{/g)].map((m) => m.index);
  const wWait = w.indexOf('$hx = Wait-HxFree -CliExe $cliExe');
  const wIfNear = wWait !== -1 ? Math.max(...wIfs.filter((i) => i < wWait)) : -1;
  must(wIfs.length >= 2, 'C5', '② 缺两个 if (-not $SkipBuild) 分支');
  must(wIfNear !== -1 && startIdx(w, 'wait-hx') > wIfNear && startIdx(w, 'wait-hx') < wWait,
    'C5', '② wait-hx 不在 Wait-HxFree 所在分支内');
  must(wIfs.length >= 2 && startIdx(w, 'publish') > wIfs[1], 'C5', '② publish 段不在 -not SkipBuild 分支内');

  // C6：每份脚本恰好一次 Stop-GateStage
  for (const [key, rel] of Object.entries(SCRIPTS)) {
    const n = (sources.scripts[key].match(/Stop-GateStage\b/g) || []).length;
    must(n === 1, 'C6', rel + ' Stop-GateStage 出现 ' + n + ' 次（应恰 1 次）');
  }
  return violations;
}

function realSources() {
  return {
    lib: readSource(LIB_REL),
    scripts: {
      a: readSource(SCRIPTS.a),
      c: readSource(SCRIPTS.c),
      w: readSource(SCRIPTS.w),
    },
  };
}

describe('gate-stages 接线守护（#2001）', () => {
  it('自检：注入违规必须被检出（防空跑假绿；注入一律全局替换）', () => {
    const real = realSources();
    const cases = [
      ['C0', { ...real, lib: real.lib.replace(/function\s+Stop-GateStage/g, 'function RemovedStop') }],
      ['C1', { ...real, scripts: { ...real.scripts, a: real.scripts.a.replace(/lib[\\/]gate-stages\.ps1/g, 'lib/removed.ps1') } }],
      ['C2', { ...real, scripts: { ...real.scripts, a: real.scripts.a.replace(/Initialize-GateStageRecorder/g, 'Removed-Recorder') } }],
      ['C3', { ...real, scripts: { ...real.scripts, a: real.scripts.a.replace(/['\"]compile-launch['\"]/g, "'removed-seg'") } }],
      ['C4', { ...real, scripts: { ...real.scripts, c: real.scripts.c.replace(/['\"]kotlinc['\"]/g, "'removed-seg'") } }],
      ['C5', { ...real, scripts: { ...real.scripts, w: real.scripts.w.replace(/['\"]auto-probe['\"]/g, "'removed-seg'") } }],
      ['C6', { ...real, scripts: { ...real.scripts, a: real.scripts.a.replace(/Stop-GateStage/g, 'RemovedStageStop') } }],
    ];
    for (const [rule, injected] of cases) {
      expect(scanContract(injected)).toEqual(expect.arrayContaining([expect.stringMatching('^' + rule + ' ')]));
    }
  });

  it('真实文件：三脚本 + lib 全部满足不变量（零违规）', () => {
    expect(scanContract(realSources())).toEqual([]);
  });
});
