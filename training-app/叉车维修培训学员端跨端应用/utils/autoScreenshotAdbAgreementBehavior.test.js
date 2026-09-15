/**
 * 步骤 6 与步骤 2/5 的 **adb 解析一致性** 行为级守护（运行期）—— 2026-09-15，#1027
 *
 * 缺陷（本机实测，机制级复现）：
 *   adb 解析在本仓被写了三遍，其中 `scripts/lib/auto-screenshot.ps1`（步骤 6）的**内联副本**
 *   比 `scripts/lib/env-check.ps1` 的 `Resolve-AdbExeLocal`（步骤 2/5）**少了 D 盘兜底** ⇒
 *   本机（`$env:ANDROID_SDK_ROOT` / `$env:ANDROID_HOME` 都未设、`adb` 不在 PATH、adb 只在
 *   `D:\android-sdk\platform-tools\adb.exe`）**步骤 2/5 能过、步骤 6 恒报「找不到 adb.exe」**；
 *   而步骤 4/5 已经真跑完并产生副作用（编译 + 部署），步骤 7–9（对比 / 证据 / 还原）永远到不了。
 *
 * 为什么必须是**运行期**断言：这个缺陷只在「共享解析器能命中、内联副本命中不了」时显形。
 *   纯文本断言（`expect(src).toContain('Resolve-AdbExeLocal')`）在**把内联副本留在原地**时照样绿 ——
 *   它证明不了「步骤 6 用的是那份解析器」。本用例真跑一次 `Invoke-AutoScreenshot`，把
 *   「步骤 6 到底有没有解析到 adb」变成可观测结论，再与 `Resolve-AdbExeLocal` 的结论逐项比对。
 *   （结构性那一面另有 `autoScreenshotContract.test.js` S21 钉住「不得再出现第二份候选集」。）
 *
 * 断言：
 *   A1 探针真跑完（pwsh 可用、两条调用形态都走通、判据 token 齐备）——fail-closed，不 skip
 *   A2 **独立调用形态**（只 dot-source `auto-screenshot.ps1`）下，步骤 6 的 adb 结论与
 *      `Resolve-AdbExeLocal` 一致 —— 这条同时证明「懒加载真的接上了同一份解析器」
 *   A3 **dev:finish 形态**（env-check.ps1 已先被 dot-source）下，结论同样一致
 *
 * ⚠️ **判据一律在 pwsh 侧算成 ASCII token，JS 侧绝不匹配中文错误文本**（2026-09-15 实测踩到，务必保留）：
 *   本用例第一版是在 JS 里 `!/找不到 adb\.exe/.test(error)` —— pwsh 默认按 **OEM 码页（GBK）** 写 stdout，
 *   中文到 Node 侧成了乱码（实测拿到 `\uFFFD\u0492\u04B2\u04B2 adb.exe` 一类字节），正则**恒不匹配**
 *   ⇒ 任何错误都被判成「解析到了」⇒ 两侧恒「一致」⇒ **假绿**：把修复前的文件塞回去，这个用例照样全绿。
 *   同源坑位见 `autoScreenshotContract.test.js` S20（导航包装脚本必须钉 UTF-8）。故：
 *   `_ADB_MISSING` / `SHARED_FOUND` 这类**布尔判据全部在 PowerShell 里算好**（PowerShell 经
 *   `-EncodedCommand` 收到的脚本文本是 UTF-16，脚本内中文匹配是准的），JS 只比 `True` / `False`。
 *
 * ⚠️ 为什么不直接断言「调用方可见 `Resolve-AdbExeLocal`」：PowerShell 里函数体内的 dot-source
 *   落在**函数作用域**（实测：独立调用后顶层 `Get-Command Resolve-AdbExeLocal` 仍为 False，
 *   而函数内部已能解析），故「可见性」不是可靠的判据面；可靠的是**解析结论**本身（A2/A3）。
 *
 * **写实的边界（不许假称它有判别力）**：若本机**没有任何** adb 候选可命中（共享解析器返回 `$null`），
 *   A2/A3 两侧同为「未命中」⇒ 用例**自然成立**（没有可分歧的候选）。CI（ubuntu，无 `D:\android-sdk`）
 *   即此情形；判别力只在「候选集恰好分叉」的机器上兑现 —— 那正是本 bug 的发生条件。
 *
 * 安全性：探针用**不存在的假 CliPath** 挡在设备之前（`Invoke-AutoScreenshot` 的顺序是
 *   adb → cli → pages.json → 目标页 → 设备）⇒ 无论 adb 是否命中，都**不派发 launch、
 *   不碰 HBuilderX、不碰真机**；项目目录也是临时目录（不是 git 仓库 ⇒ 推导恒 0 页）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**（仓库先例：
 *   `hxLaunchDetachBehavior.test.js` / `contractTestPatternBehavior.test.js`）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIBS = path.join(ROOT, 'scripts', 'lib');
const ENV_CHECK = path.join(LIBS, 'env-check.ps1');
const AUTO_SHOT = path.join(LIBS, 'auto-screenshot.ps1');

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** PowerShell 单引号字面量转义（`'` ⇒ `''`）。路径在本仓不含单引号，属防御性写法。 */
function psQuote(s) {
  return String(s).replace(/'/g, "''");
}

/** 隔离沙箱：临时项目（空 pages.json）+ 不存在的假 cli + 独立输出目录。 */
function makeSandbox() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'adbagree-'));
  const proj = path.join(tmp, 'proj');
  fs.mkdirSync(proj, { recursive: true });
  fs.writeFileSync(path.join(proj, 'pages.json'), JSON.stringify({ pages: [] }), 'utf8');
  return {
    tmp,
    proj,
    cli: path.join(tmp, 'no-such-cli.exe'),
    out: path.join(tmp, 'shots'),
  };
}

/**
 * 在**同一个** pwsh 进程里跑两种调用形态，回读：
 *   STANDALONE_*     —— 只 dot-source 步骤 6 的模块（懒加载形态）后的步骤 6 结论
 *   SHARED_FOUND     —— `Resolve-AdbExeLocal` 是否解析到 adb（步骤 2/5 的结论）
 *   AFTER_ENVCHECK_* —— 先 dot-source env-check.ps1（dev:finish 的真实形态）后的步骤 6 结论
 *
 * ⚠️ 判据 token 全是 ASCII（`True`/`False`），中文错误文本只作诊断回带 —— 见文件头「判据一律在 pwsh 侧算成 ASCII」。
 */
function probeAdbResolution(ctx) {
  const invoke = (tag) =>
    [
      `$r = Invoke-AutoScreenshot -Device 'FAKE-SERIAL' -CliPath '${psQuote(ctx.cli)}' `
        + `-ProjectDir '${psQuote(ctx.proj)}' -OutputDir '${psQuote(ctx.out)}'`,
      `Write-Output ("${tag}_OK=" + $r.Ok)`,
      // 判据：唯一「未解析到 adb」的出口就是那句错误文案（在 PowerShell 里匹配，不经 JS）
      `$adbMissing = ($null -eq $r.Error) -or [bool]($r.Error -match '找不到 adb\\.exe')`,
      `Write-Output ("${tag}_ADB_MISSING=" + [bool]$adbMissing)`,
      `Write-Output ("${tag}_ERROR=" + $r.Error)`,
    ].join('\n');

  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    // 诊断可读性（S20 同源坑位）：不钉的话中文错误文本回带到 Node 侧是乱码
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    // 形态 1：独立调用 —— 只加载步骤 6 的模块，唯一真源必须由它自己懒加载带进来
    `. "${AUTO_SHOT}"`,
    invoke('STANDALONE'),
    // 形态 2：dev:finish 的真实形态 —— 步骤 2 已 dot-source env-check.ps1，步骤 6 复用同一函数
    `. "${ENV_CHECK}"`,
    '$shared = Resolve-AdbExeLocal',
    'Write-Output ("SHARED_FOUND=" + [bool]$shared)',
    '$sharedText = if ($shared) { $shared } else { "<null>" }',
    'Write-Output ("SHARED=" + $sharedText)',
    invoke('AFTER_ENVCHECK'),
    'Write-Output "PROBE_DONE=1"',
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

function field(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

describe('auto-screenshot 步骤 6 的 adb 解析与唯一真源一致（运行期，#1027）', () => {
  let ctx;
  let probe;
  let standaloneMissing;
  let afterEnvCheckMissing;
  let sharedFound;

  beforeAll(() => {
    ctx = makeSandbox();
    probe = probeAdbResolution(ctx);
    if (probe.ok) {
      standaloneMissing = field(probe.stdout, 'STANDALONE_ADB_MISSING');
      afterEnvCheckMissing = field(probe.stdout, 'AFTER_ENVCHECK_ADB_MISSING');
      sharedFound = field(probe.stdout, 'SHARED_FOUND');
    }
  });

  afterAll(() => {
    if (ctx) {
      try {
        fs.rmSync(ctx.tmp, { recursive: true, force: true });
      } catch {
        // 临时目录清理失败不该把用例判红（真判据是下面的 A1–A3）
      }
    }
  });

  // A1：探针必须真跑完，且三条判据 token 齐备（缺一个就说明探针形态变了 ⇒ fail-closed 抛错，**不 skip**）。
  test('A1: 探针跑完，判据 token 齐备（pwsh 可用，两条调用形态都走通）', () => {
    if (!probe.ok) {
      throw new Error(
        'adb 解析一致性探针失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n'
          + `exit=${probe.status}\nstdout=${probe.stdout}\nstderr=${probe.stderr}`
      );
    }
    expect(field(probe.stdout, 'PROBE_DONE')).toBe('1');
    // 判据 token 必须是 pwsh 侧算好的 ASCII 布尔量 —— 为 null 说明回带形态变了（曾经正是这里靠中文匹配 ⇒ 假绿）
    expect(['True', 'False']).toContain(standaloneMissing);
    expect(['True', 'False']).toContain(afterEnvCheckMissing);
    expect(['True', 'False']).toContain(sharedFound);
  });

  // A2：**独立调用**形态（只 dot-source 步骤 6 的模块）。此时唯一真源不在调用方作用域，
  //     必须靠模块自己的懒加载接上 —— 判据是「步骤 6 的解析结论与共享解析器一致」。
  //     ⚠️ 不要改成断言「调用方可见 `Resolve-AdbExeLocal`」：函数体内 dot-source 落在**函数作用域**，
  //        顶层 `Get-Command` 恒为 False（实测），那会是个恒红的假判据。
  test('A2: 独立调用形态下，步骤 6 的 adb 结论与 Resolve-AdbExeLocal 一致（懒加载真的接上了）', () => {
    const sharedResolved = sharedFound === 'True';
    expect(standaloneMissing === 'False').toBe(sharedResolved);
  });

  // A3：env-check 先加载（dev:finish 的真实形态：步骤 2 已 dot-source）⇒ 结论同样必须一致。
  //     这是本 bug 的直接回归钉：内联副本缺 D 盘兜底时，共享解析器命中而步骤 6 报「找不到 adb.exe」⇒ 红
  //     （修复前文件塞回去实测：A3 红；第一版中文判据下它曾假绿）。
  test('A3: 步骤 6 的 adb 结论与 Resolve-AdbExeLocal（步骤 2/5）一致', () => {
    const sharedResolved = sharedFound === 'True';
    expect(afterEnvCheckMissing === 'False').toBe(sharedResolved);
  });
});
