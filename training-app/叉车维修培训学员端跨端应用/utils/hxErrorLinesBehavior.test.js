/**
 * 错误行判定的**行为级守护**（运行期，非源码文本断言）—— ADR-0012
 *
 * 为什么需要这个文件：
 *   `hxRunContract.test.js` 的 C13 只断言 `Get-HxErrorLines` 的**调用点包了 `@()`** 与**数量 ≥2**，
 *   它**不断言这条判据匹配什么**。于是下面这个缺陷逃过了全部契约断言：
 *
 *   2026-09-14 🟡 端到端首跑实测（当前代码）：HBuilderX 把 App 的运行 console 日志混进同一条 stdout，
 *   其中的 `[Error]` 是**应用业务错误**（设备登录态过期）：
 *       21:49:13.833 [dashboard] loadMyCourses failed: [Error] {"message": "登录已过期，请重新登录"} …
 *   旧判据把它当编译期诊断 ⇒ 真运行路径在**打印 `HX_RUN_DEPLOY` 之前**就 `exit 1`
 *   ⇒ ①「到底有没有到设备」的证据行**不可得**；②一次**成功部署**被判成失败（假失败）。
 *
 *   ⇒ 与 ADR-0011 ⑥ / ADR-0012 的纪律一致：**「出现某字面量」≠「该判据判得对」**，
 *     关键判据除文本断言外**必须有运行期断言**（先例 hxTimingBehavior.test.js、levelDetectBehavior.test.js）。
 *
 *   本文件的断言全部是**运行期**的（dot-source 真正的 lib 函数，喂真实/典型日志行）：
 *     E1 本轮**真实捕获**的 4 行设备日志（含 `[Error]`）⇒ **一条都不判失败**
 *     E2 设备栈帧行 ⇒ 不判失败
 *     E3 典型编译期诊断（unresolved reference / cannot infer type / 找不到名称 / 类型不匹配）⇒ **仍判失败**
 *     E4 带文件行号的 `error:` 行 ⇒ **仍判失败**（不能把编译诊断一刀切掉）
 *     E5 设备日志 + 编译诊断混在一起 ⇒ **只报编译诊断行**
 *     E6 「0 error」否证行 ⇒ 不判失败
 *     E7 空输入 ⇒ 返回**数组**（不是 $null；StrictMode 下 `$null.Count` 会抛）
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与仓库先例 mpWeixinGateContract.test.js:143 一致；静默跳过等于假绿。
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const ERRORS_LIB = path.join(ROOT, 'scripts', 'lib', 'hx-errors.ps1');

/** 本轮真机实测捕获的 4 行（时间戳与内容原样，来自 .ci-verify/hx-run.log 21:49:13–14） */
const REAL_DEVICE_ERROR_LOGS = [
  '21:49:13.833 [dashboard] loadMyCourses failed: [Error] {"cause": null, "message": "登录已过期，请重新登录", "name": "Error"} at pages/dashboard/composables/use-dashboard-feeds.uts:66',
  '21:49:13.868 [getCurrentCredentialApi] failed: [Error] {"cause": null, "message": "登录已过期，请重新登录", "name": "Error"} at api/credential.uts:140',
  '21:49:14.530 [getCurrentCredentialApi] failed: [Error] {"cause": null, "message": "登录已过期，请重新登录", "name": "Error"} at api/credential.uts:140',
  '21:49:14.550 [dashboard] loadMyCourses failed: [Error] {"cause": null, "message": "登录已过期，请重新登录", "name": "Error"} at pages/dashboard/composables/use-dashboard-feeds.uts:66',
];

const DEVICE_STACK_FRAMES = [
  '21:49:14.333 \tat android.os.Handler.handleCallback(Handler.java:1029)',
  '21:49:14.497 \tat java.lang.reflect.Method.invoke(Native Method)',
];

const COMPILE_DIAGNOSTICS = [
  'unresolved reference: foo at utils/secureStorage.uts:12',
  'Cannot infer type for this parameter. Please specify it explicitly.',
  '找不到名称 undefined',
  '类型不匹配: Any? vs Any',
];

const COMPILE_ERROR_WITH_FILELINE = 'utils/secureStorage.uts:134:8 error: identity equality is deprecated';

const ZERO_ERROR_LINES = ['编译完成: 0 error, 0 warning', 'errors: 0'];

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 把一组行喂给真正的 lib 函数，回读它判出的行数与被判行（fail-closed）。 */
function probe(lines) {
  // ⚠️ 必须显式用 "`n" 拼成**一个多行字符串**再传：函数形参是 [string]，
  //    传数组会被 PowerShell 用**空格**拼接（实测会把多行压成一行、判据整体失效）。
  const psLines = lines.length
    ? lines.map((l) => `'${String(l).replace(/'/g, "''")}'`).join(',\n')
    : '';
  const joinArgs = lines.length ? ' -join "`n"' : '';
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    `. "${ERRORS_LIB}"`,
    `$in = @(\n${psLines})${joinArgs}`,
    '$hits = @(Get-HxErrorLines -Output $in)',
    'Write-Output ("COUNT=" + $hits.Count)',
    'Write-Output ("ISARRAY=" + ($hits -is [array]))',
    'foreach ($h in $hits) { Write-Output ("HIT=" + $h) }',
  ].join('\n');

  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

function mustRun(r) {
  if (!r.ok) {
    throw new Error(
      `探针执行失败（pwsh 不可用或 lib 抛错，fail-closed 不跳过）：\n`
      + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
    );
  }
}

function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

function hits(stdout) {
  return String(stdout)
    .split(/\r?\n/)
    .filter((l) => l.startsWith('HIT='))
    .map((l) => l.slice(4));
}

describe('错误行判定行为级守护（ADR-0012，运行期）', () => {
  test('E1: 本轮真实捕获的 4 行设备日志（含 [Error]）⇒ 一条都不判失败', () => {
    const r = probe(REAL_DEVICE_ERROR_LOGS);
    mustRun(r);
    expect({ count: pick(r.stdout, 'COUNT'), hits: hits(r.stdout) }).toEqual({
      count: '0',
      hits: [],
    });
  });

  test('E2: 设备栈帧行 ⇒ 不判失败', () => {
    const r = probe(DEVICE_STACK_FRAMES);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe('0');
  });

  test('E3: 典型编译期诊断 ⇒ 仍判失败（不能把编译诊断一刀切掉）', () => {
    const r = probe(COMPILE_DIAGNOSTICS);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe(String(COMPILE_DIAGNOSTICS.length));
  });

  test('E4: 带文件行号的 error: 行（无时间戳）⇒ 仍判失败', () => {
    const r = probe([COMPILE_ERROR_WITH_FILELINE]);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe('1');
    expect(hits(r.stdout)[0]).toContain('identity equality is deprecated');
  });

  test('E5: 设备日志 + 编译诊断混合 ⇒ 只报编译诊断行', () => {
    const mixed = [...REAL_DEVICE_ERROR_LOGS, ...COMPILE_DIAGNOSTICS, ...DEVICE_STACK_FRAMES];
    const r = probe(mixed);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe(String(COMPILE_DIAGNOSTICS.length));
    const reported = hits(r.stdout);
    // 任何一条被判的行都不得是设备日志（不得带时间戳前缀）
    reported.forEach((line) => {
      expect(line).not.toMatch(/^\s*\d{2}:\d{2}:\d{2}\.\d{3}/);
    });
  });

  test('E6: 「0 error」否证行 ⇒ 不判失败', () => {
    const r = probe(ZERO_ERROR_LINES);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe('0');
  });

  test('E7: 空输入 ⇒ 返回数组而非 $null（StrictMode 下 $null.Count 会抛）', () => {
    const r = probe([]);
    mustRun(r);
    expect(pick(r.stdout, 'COUNT')).toBe('0');
    expect(pick(r.stdout, 'ISARRAY')).toBe('True');
  });
});
