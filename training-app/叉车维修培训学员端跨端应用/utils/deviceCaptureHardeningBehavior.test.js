/**
 * 加固四项的行为腿（#2077，2026-10-28）
 *
 * 为什么需要这一族：#2077 加的是一条可信收敛规则 ——「要么出可核验产物，要么明报 INCOMPLETE」。
 * 它的风险全在两格上：① 把「没测上」写成「测出问题」（读结论的人会去查 App，该查的是工具或那根线）；
 * ② 空转冒充绿（0/20 那条病根，0043 决策 6）。两格都是**纯逻辑判定**，不需要设备也必须有守护 ——
 * 否则票面 ⑥ 的成对取证就只能靠真机跑一次来证明，而 #1864 的设备窗口是稀缺资源。
 *
 *   HB1 通道成因（重试用尽仍无产物 / 取不到前台）单独出现 ⇒ INCOMPLETE，不得写成 FAIL
 *   HB2 产品成因与缺口同时出现 ⇒ FAIL 优先（更强的信号不许被 INCOMPLETE 吞掉）
 *   HB3 零失败零缺口 ⇒ PASS（证明 HB1 不是「逢跑必红」）
 *   HB4 熔断已触发、本次零产物 ⇒ INCOMPLETE（零产物不得冒充绿）
 *   HB5 连续缺口计数：拿到产物必须**归零**（写成「有失败就 +1」会让一次产品缺陷把整链熔断）
 *   HB6 台账 round-trip：写进去的页读得回来；GAP 行不算已产（上轮没产物的页这轮仍要重采）
 *   HB7 台账不存在 ⇒ 空集合，不是「全部已产」（把「没台账」读成「都跑过了」就是整批假绿）
 *   HB8 反空跑自检：通道成因与产品成因必须给出**不同**结论（把优先级写反也逃不掉）
 *
 * 形态沿用本仓既有行为腿（utils/deviceCaptureTierBehavior.test.js）：AST 抽出产品函数真执行，不写影子实现。
 * 判据 token 全 ASCII（pwsh 的中文经 OEM 码页到 Node 侧会变乱码 —— 本仓血账）；需要中文样本的格子
 * 把样本放 fixtures.json 并显式 -Encoding UTF8 读，在 PS 侧判完再回带 ASCII 结论。
 *
 * ⚠️ 这族与脚本文案**有意的耦合**：Resolve-StatusWord 认的是 #1562 / #1568 定下的失败文案形状
 *   （「未在 N 秒内返回」「0 字节」「取不到前台 Activity」）。改了那几句原话，HB1 / HB2 / HB8 会红 ——
 *   那是**应当**红：成因分类就建在那几行字面上，不许改了文案而没人发现判据已失配。
 *
 * 运行前提：需要 pwsh；不可用时红，不 skip（HB0 是这一格的显式见证）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

jest.setTimeout(180000);

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(ROOT, 'scripts', 'device-capture.ps1');

/** 中文失败样本：与 scripts/device-capture.ps1 里的原话逐字同形（见上「有意的耦合」）。 */
const FIXTURES = {
  channelTimeout: '截图调用未在 15 秒内返回（pages-login-login-current.png）—— 是 adb 通道不返回，不是设备没亮屏',
  channelZeroBytes: '截图未落盘或为 0 字节（pages-login-login-current.png）',
  channelNoForeground: '取不到前台 Activity（dumpsys 无 mResumedActivity/topResumedActivity）',
  productFatal: '窗口内 FATAL EXCEPTION x 2',
  productWrongPackage: 'pages/a/b -> 前台不是目标包（com.other/com.other.MainActivity）',
  productIdentity: 'pages/a/b -> 切页未生效：截图哈希与 pages/c/d 相同（sha256=abc）',
};

const fwd = (s) => String(s).replace(/\\/g, '/');

/** 一条腿 = 一次 pwsh 子进程：AST 抽出 device-capture.ps1 的函数定义并真执行，再跑给定的 body。 */
function leg(body, opts) {
  const o = opts || {};
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dc-harden-'));
  const fixturePath = path.join(dir, 'fixtures.json');
  fs.writeFileSync(fixturePath, JSON.stringify(Object.assign({}, FIXTURES, o.fixtures || {})), 'utf8');
  const script = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$fxPath = "' + fwd(fixturePath) + '"',
    '$FX = Get-Content -LiteralPath $fxPath -Raw -Encoding UTF8 | ConvertFrom-Json',
    '$src = Get-Content -LiteralPath "' + fwd(SCRIPT) + '" -Raw',
    '$ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$null, [ref]$null)',
    '$fns = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $true))',
    'foreach ($f in $fns) { Invoke-Expression $f.Extent.Text }',
    // 抽出了几个函数必须打出来：HB0 判的就是「产品函数真被抽出」这一格 —— 少了它，整族可以在
    // 「一个函数都没定义成功」的假绿上空跑（DCT 同族先例把这一行当第一道门）。
    'Write-Output ("EXTRACTED=" + $fns.Count)',
    '$dir = "' + fwd(dir) + '"',
    body,
    '',
  ].join('\n');
  const psPath = path.join(dir, 'leg.ps1');
  fs.writeFileSync(psPath, '\uFEFF' + script, 'utf8');
  try {
    const out = execFileSync('pwsh', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', psPath], {
      encoding: 'utf8', timeout: o.timeoutMs || 150000, maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, stdout: out, code: 0 };
  } catch (e) {
    return { ok: false, stdout: String(e.stdout || ''), stderr: String(e.stderr || ''), code: e.status };
  }
}

function field(text, name) {
  const m = new RegExp('^' + name + '=(.*)$', 'm').exec(String(text));
  return m ? m[1].trim() : '<missing>';
}

describe('加固四项行为腿（#2077 · device-capture.ps1）', () => {
  it('HB0 前置：pwsh 真可用且产品函数真被抽出来（不可用即红，不许静默 skip）', () => {
    const r = leg('Write-Output "HB0=1"');
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB0')).toBe('1');
    expect(Number(field(r.stdout, 'EXTRACTED'))).toBeGreaterThan(10);
  });

  it('HB1 通道成因单独出现 ⇒ INCOMPLETE，不得写成 FAIL（设备掉线不该被读成产品缺陷）', () => {
    const body = [
      '$f = @($FX.channelTimeout, $FX.channelZeroBytes, $FX.channelNoForeground)',
      '$g = @("只读取证未拿到产物（x.png，重试 2 次）")',
      '$s = Resolve-StatusWord -Failures $f -CaptureGaps $g -Broken $false',
      'Write-Output ("HB1=" + $s)',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB1')).toBe('INCOMPLETE');
  });

  it('HB2 产品成因与缺口同时出现 ⇒ FAIL 优先（更强的信号不许被 INCOMPLETE 吞掉）', () => {
    const body = [
      '$f = @($FX.channelTimeout, $FX.productFatal, $FX.productWrongPackage)',
      '$g = @("pages/a/b -> 取不到产物")',
      '$s = Resolve-StatusWord -Failures $f -CaptureGaps $g -Broken $true',
      'Write-Output ("HB2=" + $s)',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB2')).toBe('FAIL');
  });

  it('HB3 零失败零缺口 ⇒ PASS（对照腿：证明 HB1 不是「逢跑必红」）', () => {
    const r = leg('Write-Output ("HB3=" + (Resolve-StatusWord -Failures @() -CaptureGaps @() -Broken $false))');
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB3')).toBe('PASS');
  });

  it('HB4 熔断已触发且零产物 ⇒ INCOMPLETE；单页缺口也算（零产物不得冒充绿）', () => {
    const body = [
      'Write-Output ("HB4=" + (Resolve-StatusWord -Failures @() -CaptureGaps @() -Broken $true))',
      'Write-Output ("HB4b=" + (Resolve-StatusWord -Failures @() -CaptureGaps @("p1 -> 取不到产物") -Broken $false))',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB4')).toBe('INCOMPLETE');
    expect(field(r.stdout, 'HB4b')).toBe('INCOMPLETE');
  });

  it('HB5 连续缺口计数：没拿到才 +1，拿到产物必须归零', () => {
    const body = [
      'Write-Output ("HB5_gap=" + (Update-GapStreak -Streak 2 -NoProduct $true))',
      'Write-Output ("HB5_clear=" + (Update-GapStreak -Streak 3 -NoProduct $false))',
      'Write-Output ("HB5_zero=" + (Update-GapStreak -Streak 0 -NoProduct $true))',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB5_gap')).toBe('3');
    expect(field(r.stdout, 'HB5_clear')).toBe('0');
    expect(field(r.stdout, 'HB5_zero')).toBe('1');
  });

  it('HB6 台账 round-trip：写进去的页读得回来，GAP 行不算已产', () => {
    const body = [
      '$led = Join-Path $dir "captured.tsv"',
      'if (Test-Path -LiteralPath $led) { Remove-Item -LiteralPath $led -Force }',
      'Add-CaptureLedger -Path $led -Page "pages/a/b" -Shot "device-pages-a-b-after.png"',
      'Add-CaptureLedger -Path $led -Page "pages/c/d" -Shot "device-pages-c-d-after.png"',
      'Add-Content -LiteralPath $led -Value "pages/e/f`tGAP" -Encoding UTF8',
      '$done = Read-CaptureLedger -Path $led',
      'Write-Output ("HB6_count=" + $done.Count)',
      'Write-Output ("HB6_ab=" + $done["pages/a/b"])',
      'Write-Output ("HB6_ef_absent=" + [bool](-not $done.ContainsKey("pages/e/f")))',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB6_count')).toBe('2');
    expect(field(r.stdout, 'HB6_ab')).toBe('device-pages-a-b-after.png');
    expect(field(r.stdout, 'HB6_ef_absent')).toBe('True');
  });

  it('HB7 台账不存在 / 路径为空 ⇒ 空集合（把「没台账」读成「都跑过了」就是整批假绿）', () => {
    const body = [
      '$a = Read-CaptureLedger -Path (Join-Path $dir "no-such-ledger.tsv")',
      'Write-Output ("HB7_missing=" + $a.Count)',
      '$b = Read-CaptureLedger -Path ""',
      'Write-Output ("HB7_empty=" + $b.Count)',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB7_missing')).toBe('0');
    expect(field(r.stdout, 'HB7_empty')).toBe('0');
  });

  it('HB8 反空跑自检：通道成因与产品成因必须给出不同结论（优先级写反也逃不掉）', () => {
    const body = [
      '$chan = Resolve-StatusWord -Failures @($FX.channelTimeout) -CaptureGaps @("g") -Broken $false',
      '$prod = Resolve-StatusWord -Failures @($FX.productIdentity) -CaptureGaps @("g") -Broken $false',
      'Write-Output ("HB8_chan=" + $chan)',
      'Write-Output ("HB8_prod=" + $prod)',
      'Write-Output ("HB8_differ=" + [bool]($chan -ne $prod))',
    ].join('\n');
    const r = leg(body);
    expect(r.ok).toBe(true);
    expect(field(r.stdout, 'HB8_differ')).toBe('True');
    expect(field(r.stdout, 'HB8_chan')).toBe('INCOMPLETE');
    expect(field(r.stdout, 'HB8_prod')).toBe('FAIL');
  });
});
