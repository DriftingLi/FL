/**
 * ④c「publish 未成立 / 导出陈旧」判据 · **行为级**守护（票 #1272，2026-09-22）
 *
 * ## 为什么是行为守护
 *
 * #1272 的假绿由两条判据的缺少造成，两条都**只有跑起来才看得见**：
 *   ① publish 步**没有成立**（旧判据只认「与主程序的连接已中断|启动超时」两条文案，
 *      于是 `-1:cli:命令'publish app-android'不存在或缺少参数` 被放过）；
 *   ② 产物**陈旧**（旧判据只看「有没有 .kt」，磁盘上留着一份旧导出就满足）。
 * 断言「源码里出现过某个字面量」对这两条毫无判别力（`docs/agents/guards.md`：接线守护不构成 ③ 证据）。
 *
 * ## 怎么真执行
 *
 * 两条判据住在 `scripts/lib/publish-freshness.ps1`（零副作用），本文件用 pwsh dot-source 它之后
 * **直接驱动**（先例 `utils/capabilitySurfaceBehavior.test.js` 驱动 `scripts/lib/capability-surface.ps1`）。
 * 被测物是**仓内真件**，不是手抄镜像。
 *
 * ## 判据（③ 门三问）
 *
 * - **判别力（①「我故意弄坏被测物，它会不会红？」）**：H1 的「必红」半用**本票实测抓到的真实失败串**
 *   （不是编一个像模像样的样本）；末组 H4 把库里的判据注入变异，证明 H1/H2 的断言**真的有牙**。
 * - **测的是该测的那一支（②）**：驱动的是**真实产物的 mtime**（临时目录里真造 `.kt` 再改 mtime），
 *   不是常量比较。
 * - **成对取证（③「只跑通过的那一次不算验收」）**：同一份产物给两个基准 —— 一个必判 stale、一个必判 fresh；
 *   publish 判据同样给真实失败串与真实成功串**各一条**。
 *
 * ## 已知边界（写实，勿读成「已覆盖」）
 *
 * 本守护驱动的是**判据函数**；**门脚本端到端**的失败路径（真跑 publish、让它失败）需要 HBuilderX 且会取
 * `scripts/lib/hx-busy.ps1` 的**全局互斥锁** —— 测试**不得**碰那把锁（多会话共享资源，ADR-0008），
 * 故端到端一侧由票面 #1272 的实测记录承担：本机实测 `publish app-android --type appResource --project <p>`
 * **成功**导出 119 个 `.kt`（工作树未改时再跑一次，119/119 个 `.kt` mtime 照样前进 ⇒ 导出是全量重写，
 * 这正是「最新 .kt mtime 晚于 publish 基准」这条判据**不会误杀合法运行**的前提）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（与仓库先例一致；静默跳过等于假绿）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'publish-freshness.ps1');
const GATE_REL = 'scripts/kotlin-all-check.ps1';

/**
 * **真实证据串**（勿改成「像那么回事」的样本）：
 * - 失败串 = #1272 实施记录里本机抓到的那一行原文（旧判据正是在它上面放行）；
 * - 成功串 = 同机 publish 成功时的输出尾部原文。
 */
const REAL_FAILURE_OUTPUT = "-1:cli:命令'publish app-android'不存在或缺少参数 当前命令执行错误";
const REAL_SUCCESS_OUTPUT = [
  '16:57:37.501 项目 叉车维修培训学员端跨端应用 正在导出...',
  '16:57:40.223 项目 叉车维修培训学员端跨端应用 导出 android 成功，路径为：D:\\FL\\wt-1272\\training-app\\叉车维修培训学员端跨端应用\\unpackage\\resources\\app-android',
].join('\n');

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 单引号转义（PowerShell 字面量）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

/**
 * dot-source 指定副本的判据库并执行语句，回读 stdout。
 * @param {string} libAbs 要 dot-source 的库路径（真实库或**变异副本**）
 */
function drive(libAbs, statements, timeout = 60000) {
  const prelude = [
    '$ErrorActionPreference = "Stop"',
    // 必须先把编码钉成 UTF-8：脚本与断言里都有中文，被管道捕获时可能落成 OEM 代码页 ⇒ 恒假
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    `. ${q(libAbs)}`,
  ];
  const script = prelude.concat(statements).join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return String(stdout);
  } catch (e) {
    throw new Error(
      '驱动判据库失败（pwsh 不可用或库抛错，fail-closed 不跳过）：\n'
      + `exit=${e.status}\nstdout=${e.stdout}\nstderr=${e.stderr || e.message}`
    );
  }
}

/** 取 `KEY=value` 行 */
function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  if (m === null) throw new Error(`输出里没有 ${key}：\n${stdout}`);
  return m[1].trim();
}

/** 造一份「陈旧」的导出目录：里面有一个 3 小时前写入的 .kt */
function makeStaleExport() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-'));
  fs.mkdirSync(path.join(dir, 'a'));
  const kt = path.join(dir, 'a', 'X.kt');
  fs.writeFileSync(kt, 'class X');
  const old = new Date(Date.now() - 3 * 3600 * 1000);
  fs.utimesSync(kt, old, old);
  return dir;
}

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function mutatedLib(replacements) {
  let src = readText(path.join(ROOT, LIB_REL));
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-mut-')), 'publish-freshness.ps1');
  fs.writeFileSync(file, src);
  return file;
}

const REAL_LIB = path.join(ROOT, LIB_REL);

// ===== ① 新鲜度判据：同一份产物、两个基准（成对取证）=====

describe('新鲜度判据：产物是否覆盖当前树（同一份产物给两个基准，必 stale / 必 fresh 各一条）', () => {
  it('必红半：基准晚于产物 ⇒ stale 且 Fresh=false；必不红半：基准早于产物 ⇒ fresh 且 Fresh=true', () => {
    const dir = makeStaleExport();
    const out = drive(REAL_LIB, [
      `$r1 = Test-AppResourceFreshness -ExportDir ${q(dir)} -Since (Get-Date)`,
      'Write-Output ("STALE=" + $r1.Fresh + "|" + $r1.Reason + "|" + $r1.KtCount)',
      `$r2 = Test-AppResourceFreshness -ExportDir ${q(dir)} -Since (Get-Date).AddHours(-4)`,
      'Write-Output ("FRESH=" + $r2.Fresh + "|" + $r2.Reason + "|" + $r2.KtCount)',
    ]);
    // 必红半：这份产物是 3 小时前写的，基准取「现在」⇒ 必须判不新鲜（旧写法在这里会判 ✅）
    expect(pick(out, 'STALE')).toBe('False|stale|1');
    // 必不红半：同目录、基准退到 4 小时前 ⇒ 必须判新鲜（防「判据恒红」这种反向假绿）
    expect(pick(out, 'FRESH')).toBe('True|fresh|1');
  });

  it('导出目录里没有 .kt ⇒ 独立 reason（no-kt）而不是 stale —— 调用方要按它给不同处置', () => {
    const empty = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-empty-'));
    const out = drive(REAL_LIB, [
      `$r = Test-AppResourceFreshness -ExportDir ${q(empty)} -Since (Get-Date)`,
      'Write-Output ("EMPTY=" + $r.Fresh + "|" + $r.Reason + "|" + $r.KtCount)',
    ]);
    expect(pick(out, 'EMPTY')).toBe('False|no-kt|0');
  });

  it('`www` 段下的 .kt 不算产物 —— 且这条口径**与路径分隔符无关**（CI 在 ubuntu 上跑）', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-www-'));
    fs.mkdirSync(path.join(dir, 'www'), { recursive: true });
    const kt = path.join(dir, 'www', 'Y.kt');
    fs.writeFileSync(kt, 'class Y');
    const out = drive(REAL_LIB, [
      `$r = Test-AppResourceFreshness -ExportDir ${q(dir)} -Since (Get-Date).AddHours(-1)`,
      'Write-Output ("WWW=" + $r.Fresh + "|" + $r.Reason + "|" + $r.KtCount)',
    ]);
    expect(pick(out, 'WWW')).toBe('False|no-kt|0');
  });
});

// ===== ② publish 判据：用**实测抓到的真实串**（不是编样本）=====

describe('publish 判据：这一步是否成立（真实失败串必红 / 真实成功串必不红）', () => {
  it('必红半：本票实测的失败串 ⇒ Ok=false 且 reason=cli-command-failed', () => {
    const out = drive(REAL_LIB, [
      `$v = Get-PublishVerdict -Output ${q(REAL_FAILURE_OUTPUT)} -TimedOut $false`,
      'Write-Output ("FAIL=" + $v.Ok + "|" + $v.Reason)',
    ]);
    expect(pick(out, 'FAIL')).toBe('False|cli-command-failed');
  });

  it('必不红半：同机实测的成功输出 ⇒ Ok=true 且 reason=exported', () => {
    const out = drive(REAL_LIB, [
      `$v = Get-PublishVerdict -Output ${q(REAL_SUCCESS_OUTPUT)} -TimedOut $false`,
      'Write-Output ("OK=" + $v.Ok + "|" + $v.Reason)',
    ]);
    expect(pick(out, 'OK')).toBe('True|exported');
  });

  it('fail-closed 兜底：既无成功标记、也不命中失败签名的输出 ⇒ 照样判「未成立」', () => {
    const out = drive(REAL_LIB, [
      '$v = Get-PublishVerdict -Output "正在编译中..." -TimedOut $false',
      'Write-Output ("BARE=" + $v.Ok + "|" + $v.Reason)',
    ]);
    expect(pick(out, 'BARE')).toBe('False|no-success-marker');
  });

  it('两种环境原因各自可分辨（调用方按 reason 给不同处置）', () => {
    const out = drive(REAL_LIB, [
      '$a = Get-PublishVerdict -Output "" -TimedOut $true',
      'Write-Output ("TIMEOUT=" + $a.Ok + "|" + $a.Reason)',
      '$b = Get-PublishVerdict -Output "与主程序的连接已中断" -TimedOut $false',
      'Write-Output ("IPC=" + $b.Ok + "|" + $b.Reason)',
    ]);
    expect(pick(out, 'TIMEOUT')).toBe('False|timeout');
    expect(pick(out, 'IPC')).toBe('False|cli-ipc-blocked');
  });
});

// ===== ③ 整段裁决：门的**决策序列**也要被真执行 =====
//
// 评审发现（2026-09-22）：只抽两个叶子判据时，门的「陈旧 ⇒ exit 1」那条**决策分支**没有任何用例跑过 ——
// 那等于把新加的失败分支写成没有证据的代码。故裁决本身也抽成 `Get-PublishStageVerdict`，门与守护**共用**它，
// 下面把四个退出路径逐个真跑一遍。

describe('整段裁决（`Get-PublishStageVerdict`）：四个退出路径都真跑过（含「陈旧 ⇒ exit 1」那一支）', () => {
  it('必红 · 产物陈旧 ⇒ ExitCode=1 / reason=stale-export / freshness=stale', () => {
    const stale = makeStaleExport();
    const out = drive(REAL_LIB, [
      `$e = Get-PublishStageVerdict -PublishOutput ${q(REAL_SUCCESS_OUTPUT)} -PublishTimedOut $false -ExportDir ${q(stale)} -Since (Get-Date)`,
      'Write-Output ("STALE=" + $e.ExitCode + "|" + $e.Reason + "|" + $e.Freshness + "|" + $e.KtCount)',
    ]);
    expect(pick(out, 'STALE')).toBe('1|stale-export|stale|1');
  });

  it('必不红 · 刚导出 + 成功输出 ⇒ ExitCode=0 / reason=ok / freshness=fresh', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-ok-'));
    fs.mkdirSync(path.join(dir, 'a'));
    fs.writeFileSync(path.join(dir, 'a', 'X.kt'), 'class X');
    const out = drive(REAL_LIB, [
      `$e = Get-PublishStageVerdict -PublishOutput ${q(REAL_SUCCESS_OUTPUT)} -PublishTimedOut $false -ExportDir ${q(dir)} -Since (Get-Date).AddHours(-1)`,
      'Write-Output ("OK=" + $e.ExitCode + "|" + $e.Reason + "|" + $e.Freshness)',
    ]);
    expect(pick(out, 'OK')).toBe('0|ok|fresh');
  });

  it('必红 · publish 未成立 ⇒ ExitCode=2 / freshness=**实测量**（#1285：不短路成 not-measured），且仍带出 mtime/数量供失败日志', () => {
    const stale = makeStaleExport();
    const out = drive(REAL_LIB, [
      `$e = Get-PublishStageVerdict -PublishOutput ${q(REAL_FAILURE_OUTPUT)} -PublishTimedOut $false -ExportDir ${q(stale)} -Since (Get-Date)`,
      'Write-Output ("NOSUCCESS=" + $e.ExitCode + "|" + $e.Reason + "|" + $e.Freshness + "|" + $e.KtCount + "|" + ($e.Newest -ne $null))',
    ]);
    // #1285：freshness 是**实测量**（这份产物 3 小时前导出 ⇒ stale）—— publish 未成立只改 ExitCode/Reason，
    // 不把新鲜度糊成 not-measured：否则「导出已刷新但文案没读到」与「根本没导出」共用同一条红
    expect(pick(out, 'NOSUCCESS')).toBe('2|publish-cli-command-failed|stale|1|True');
  });

  it('必红 · 成功输出但导出目录没有 .kt ⇒ ExitCode=1 / reason=no-artifact / freshness=no-kt（保住既有口径）', () => {
    const empty = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-none-'));
    const out = drive(REAL_LIB, [
      `$e = Get-PublishStageVerdict -PublishOutput ${q(REAL_SUCCESS_OUTPUT)} -PublishTimedOut $false -ExportDir ${q(empty)} -Since (Get-Date)`,
      'Write-Output ("NOART=" + $e.ExitCode + "|" + $e.Reason + "|" + $e.Freshness)',
    ]);
    expect(pick(out, 'NOART')).toBe('1|no-artifact|no-kt');
  });

  it('超时 ⇒ ExitCode=2 / reason=publish-timeout（原因可分辨，处置不同；freshness 照实报）', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-to-'));
    const out = drive(REAL_LIB, [
      `$e = Get-PublishStageVerdict -PublishOutput "" -PublishTimedOut $true -ExportDir ${q(dir)} -Since (Get-Date)`,
      'Write-Output ("TO=" + $e.ExitCode + "|" + $e.Reason + "|" + $e.Freshness)',
    ]);
    expect(pick(out, 'TO')).toBe('2|publish-timeout|no-kt');
  });

  it('期望(b) 区分对：同是「无成功标记」，fresh 导出 ⇒ 文案没读到（采集方向）/ stale、no-kt ⇒ 根本没导出（环境方向）', () => {
    // #1285 的字段症状：导出目录其实已刷新（freshness=fresh），只是成功文案没读到 ——
    // 旧写法这里记 not-measured，与「压根没导出」糊成同一条红，处置完全不同却分不开。
    const fresh = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-b-diff-'));
    fs.mkdirSync(path.join(fresh, 'a'));
    fs.writeFileSync(path.join(fresh, 'a', 'X.kt'), 'class X');
    const stale = makeStaleExport();
    const empty = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-b-none-'));
    const out = drive(REAL_LIB, [
      `$a = Get-PublishStageVerdict -PublishOutput '正在编译中...' -PublishTimedOut $false -ExportDir ${q(fresh)} -Since (Get-Date).AddHours(-1)`,
      'Write-Output ("FRESHCASE=" + $a.ExitCode + "|" + $a.Reason + "|" + $a.Freshness)',
      `$b = Get-PublishStageVerdict -PublishOutput '正在编译中...' -PublishTimedOut $false -ExportDir ${q(stale)} -Since (Get-Date)`,
      'Write-Output ("STALECASE=" + $b.ExitCode + "|" + $b.Reason + "|" + $b.Freshness)',
      `$c = Get-PublishStageVerdict -PublishOutput '正在编译中...' -PublishTimedOut $false -ExportDir ${q(empty)} -Since (Get-Date)`,
      'Write-Output ("NONECASE=" + $c.ExitCode + "|" + $c.Reason + "|" + $c.Freshness)',
    ]);
    expect(pick(out, 'FRESHCASE')).toBe('2|publish-no-success-marker|fresh');
    expect(pick(out, 'STALECASE')).toBe('2|publish-no-success-marker|stale');
    expect(pick(out, 'NONECASE')).toBe('2|publish-no-success-marker|no-kt');
  });
});

// ===== ④ 成对取证：判据被改坏时，上面的断言必须判红 =====

describe('成对取证（必红）：把判据本身注入变异，证明上面的断言有牙', () => {
  it('变异「成功标记」⇒ 真实成功串会被判成未成立（H2 的必不红断言随之判红）', () => {
    const broken = mutatedLib([["$PublishSuccessMarker = '导出 android 成功'", "$PublishSuccessMarker = '导出 android 成功X'"]]);
    const out = drive(broken, [
      `$v = Get-PublishVerdict -Output ${q(REAL_SUCCESS_OUTPUT)} -TimedOut $false`,
      'Write-Output ("MUT=" + $v.Ok + "|" + $v.Reason)',
    ]);
    expect(pick(out, 'MUT')).toBe('False|no-success-marker');
  });

  it('变异「新鲜度比较」⇒ 陈旧产物会被判成新鲜（H1 的必红断言随之判红）', () => {
    const stale = makeStaleExport();
    const broken = mutatedLib([
      ['if ($newest -gt $Since) { return @{ Fresh = $true;', 'if ($true) { return @{ Fresh = $true;'],
    ]);
    const out = drive(broken, [
      `$r = Test-AppResourceFreshness -ExportDir ${q(stale)} -Since (Get-Date)`,
      'Write-Output ("MUT=" + $r.Fresh + "|" + $r.Reason)',
    ]);
    expect(pick(out, 'MUT')).toBe('True|fresh');
  });

  it('变异「未成立 ⇒ freshness 照实报」⇒ 期望(b) 的区分对与实测钉随之判红（断言有牙）', () => {
    const broken = mutatedLib([[
      'ExitCode = 2; Reason = "publish-$($v.Reason)"; Freshness = $f.Reason',
      'ExitCode = 2; Reason = "publish-$($v.Reason)"; Freshness = \'not-measured\'',
    ]]);
    const fresh = fs.mkdtempSync(path.join(os.tmpdir(), 'kotlin-fresh-b-mut-'));
    fs.mkdirSync(path.join(fresh, 'a'));
    fs.writeFileSync(path.join(fresh, 'a', 'X.kt'), 'class X');
    const out = drive(broken, [
      `$a = Get-PublishStageVerdict -PublishOutput '正在编译中...' -PublishTimedOut $false -ExportDir ${q(fresh)} -Since (Get-Date).AddHours(-1)`,
      'Write-Output ("MUT=" + $a.ExitCode + "|" + $a.Reason + "|" + $a.Freshness)',
    ]);
    expect(pick(out, 'MUT')).toBe('2|publish-no-success-marker|not-measured');
  });
});

// ===== ④ 接线：库被门脚本真的用上（**不构成 ③ 证据**，行为面由上面三组承重）=====

describe('接线（本组是接线守护，不构成 ③ 证据）：门脚本确实消费这个库', () => {
  const gate = readText(path.join(ROOT, GATE_REL));

  it('门脚本 dot-source 判据库，且三个判据都真被调用（含整段裁决）', () => {
    expect(gate).toContain("lib\\publish-freshness.ps1");
    expect(gate).toContain('Get-PublishStageVerdict -PublishOutput');
    expect(gate).toContain('Test-AppResourceFreshness -ExportDir');
    // 产物口径的唯一定义也被门脚本消费（评审发现：原先这里抄了一份没守护的收集）
    expect(gate).toContain('Get-AppResourceKtFiles -ExportDir');
  });

  it('结果行带上新鲜度结论；`-SkipPublish` 显式记「跳过」而不是假装判过', () => {
    expect(gate).toContain('freshness=$freshnessVerdict');
    expect(gate).toContain("skipped(skip-publish)");
    // 预置值是响亮的 not-judged（不是无害的 skipped）：绕过裁决的路径不许读成「无害」
    expect(gate).toContain("'not-judged'");
  });

  it('失败路径的 errors 栏写 gate/env 而不是数字（旧写法 errors=1 像「1 条编译错误」）', () => {
    expect(gate).toContain("'gate'");
    expect(gate).not.toContain('errors=1 stage=publish');
  });

  it('两个失败 reason 都由门脚本分支处理（陈旧 ⇒ stale-export；无产物 ⇒ 既有的 no-artifact）', () => {
    // 裁决把 reason 交回来，门按它分流处置文案 —— 故判据是「门里出现这两个 reason 的分支」，
    // 而不是结果行里的字面量（结果行由 `reason=$($publishEval.Reason)` 插值）。
    expect(gate).toContain("'stale-export'");
    expect(gate).toContain("'no-artifact'");
  });

  it('失败提示按 freshness 分流两种处置（#1285 期望 b：文案没读到 ≠ 没导出）', () => {
    expect(gate).toContain('freshness=fresh');
    expect(gate).toContain('根本没导出（环境方向）');
  });
});
