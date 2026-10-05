/**
 * 样式内循环入口（`scripts/style-loop.ps1` + `scripts/lib/style-loop.ps1`，issue #1543）的
 * **行为守护** —— 断言的是**产物形状**，不是脚本源码文本。
 *
 * 为什么这层必须存在（票面第 5 条：「守护断言产物形状而非脚本源码原文」）：
 *   本入口的全部价值就在那一行 —— 红要真的红、要说出哪一页、要装得下 200 字节、日志要真落盘。
 *   `expect(src).toContain('STYLE_LOOP')` 这类接线守护对这四件事**一件都答不了**：
 *   源码文本长得一样、行为可以是恒绿的空转（本仓三个「永远绿」先例的共同形态，见 docs/agents/guards.md）。
 *   所以这里用 pwsh **真跑** lib 的纯函数与入口本身，量字节、读文件、比 sha、看退出码。
 *
 * 判据分组（每组都有**成对**用例，只跑通过的那一次不算验收 —— guards.md 判据 ③）：
 *   L  那一行的形状：≤预算 / 恒 ASCII 单行 / 判红必点名 / 收缩先砍哪一半
 *   V  阶段结论：优先级、判不了⇒红、`gate=none` 恒在
 *   W  预检方向：有活⇒放行、无活⇒红（且无活那条**不许**把有活的形态一起判红）
 *   C  配置字节还原：改脏⇒还原、BOM/行尾漂移⇒也还原、幂等、只报不删
 *   D  基线落盘：首次运行**真的**写基线（否则绿是空的）、非本轮残留不得进参考图
 *   R  包装层：子进程有行⇒原样 relay、无行⇒合成并按红（含「码 0 却没行」那一形）
 *   E  入口真跑：-DryRun / 预检红 / 环境红 三条路径的退出码与「终端恰好一行」
 *
 * 成对断言在哪（guards.md 末节第 2 问）：
 *   · 必不红 —— L1（绿⇒exit 0）、V4（Ok=false 但码 0 ⇒ 仍判红，反向配对见 V3）、C1（未改动⇒Unchanged）
 *   · 必红   —— L2（像素变化⇒exit 1 且点名 settings.png）、V1/V6/V7（判不了⇒红）、
 *              R3（子进程静默却报 0 ⇒ 改判 1）、C3（原本没有的文件出现 ⇒ 只报不删，不静默）
 *
 * 安全性（为什么这些用例可以在任何机器上跑、也不会抢 HBuilderX）：
 *   E 组只走两条**不碰主程序**的路径 —— `-DryRun`（不建子进程）与「预检/环境就收口」。
 *   环境那两条一律带 `-Device <不存在的序列号>`：`lib/env-check.ps1` 在**设备判定**那一步就返回
 *   （它排在 HBuilderX 探测与取锁之前），所以既不会 `Wait-HxFree`、也不会 `cli launch`。
 *   入口用例一律带 `-ProjectDir <临时目录>` —— 日志落在临时目录里，不进工作树。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）与 `git`。**不可用时 fail-closed 抛错，不 skip**
 *   （仓库先例：`screenshotDiffBehavior.test.js` / `hxLaunchDetachBehavior.test.js` /
 *    `contractTestPatternBehavior.test.js`）。
 *
 * ⚠️ 判据 token 全 ASCII：JS 侧绝不匹配中文（本仓血账 —— pwsh 默认按 OEM 码页写 stdout，
 *   中文到 Node 侧成了乱码，正则恒不匹配 ⇒ 什么都能被判成通过 ⇒ 假绿）。布尔与枚举一律
 *   在 pwsh 侧算成 `True`/`green`/`leaf-log` 这类 ASCII 再回读。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB = path.join(ROOT, 'scripts', 'lib', 'style-loop.ps1');
const ENTRY = path.join(ROOT, 'scripts', 'style-loop.ps1');

/** 票面第 3 条的字节预算：机检行不得超过它。守护按**字节**量（字符数不是判据）。 */
const LINE_BUDGET = 200;

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function psQuote(s) {
  return String(s).replace(/'/g, "''");
}

function runPwshCommand(scriptText) {
  const encoded = Buffer.from(scriptText, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 180000,
      windowsHide: true,
      maxBuffer: 16 * 1024 * 1024,
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

/** 真跑入口（父进程包装层）。返回退出码与完整 stdout。 */
function runEntry(args, timeoutMs) {
  const full = psArgs(['-File', ENTRY].concat(args.map(String)));
  try {
    const stdout = execFileSync('pwsh', full, {
      encoding: 'utf8',
      timeout: timeoutMs || 180000,
      cwd: ROOT,
      windowsHide: true,
      maxBuffer: 16 * 1024 * 1024,
    });
    return { ok: true, status: 0, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      // e.status 可能是 null（超时/信号），统一成 -1 让断言落在「非零」而不是「读了个 null」
      status: typeof e.status === 'number' ? e.status : -1,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

function machineLines(stdout) {
  return String(stdout)
    .split(/\r?\n/)
    .filter((l) => /^STYLE_LOOP /.test(l));
}

function fieldOf(line, key) {
  const m = new RegExp('(?:^|\\s)' + key + '=([^\\s]*)').exec(String(line));
  return m ? m[1] : null;
}

function field(stdout, key) {
  const m = new RegExp('^' + key + '=(.*)$', 'm').exec(String(stdout));
  return m ? m[1].trim() : null;
}

/**
 * 纯函数探针：dot-source lib 后把每个场景的**产物**打成 ASCII token。
 * 场景在 pwsh 侧算，JS 只比字符串 —— 与 autoScreenshotAdbAgreementBehavior 的纪律一致。
 */
function probePure() {
  const lines = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. '${psQuote(LIB)}'`,
    // 绿色：走完全程、像素层判无变化
    '$green = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 2 -ShotCount 2 ' +
      '-ChangedCount 0 -DiffVerdict ([pscustomobject]@{Ok=$true;Action="none";ExitCode=0})',
    'Write-Output ("GREEN_V=" + $green.Verdict + "|" + $green.Phase + "|" + $green.ExitCode + "|" + $green.Reason)',
    // L1 那一行（绿）
    '$f1 = Format-StyleLoopLine -Verdict $green -LogPath ".ci-verify/style-loop.log" -Seconds 412',
    'Write-Output ("L1_B=" + $f1.Bytes)',
    'Write-Output ("L1_LADDER=" + $f1.Ladder)',
    'Write-Output ("L1_LINE=" + $f1.Line)',
    // ASCII/单行判据用 \\Z 收尾（不在引号前留裸 `$`，省一类转义歧义）
    'Write-Output ("L1_ASCII=" + ($f1.Line -match \'^[\\x20-\\x7E]+\\Z\'))',
    'Write-Output ("L1_NEWLINES=" + ([regex]::Matches($f1.Line, "[\\r\\n]").Count))',
    // L2 那一行（像素红，三页里两页变了 ⇒ 必须点名两页 + changed=2）
    '$red = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 3 -ShotCount 3 ' +
      '-ChangedCount 2 -ChangedPages @("settings.png","course-detail.png") ' +
      '-DiffVerdict ([pscustomobject]@{Ok=$false;Action="request-decision";ExitCode=1})',
    'Write-Output ("L2_V=" + $red.Verdict + "|" + $red.Phase + "|" + $red.ExitCode + "|" + $red.Reason)',
    '$f2 = Format-StyleLoopLine -Verdict $red -LogPath ".ci-verify/style-loop.log" -Seconds 398',
    'Write-Output ("L2_B=" + $f2.Bytes)',
    'Write-Output ("L2_LINE=" + $f2.Line)',
    'Write-Output ("L2_SHOT=" + (@($f2.ShotNames) -join ","))',
    'Write-Output ("L2_HIDDEN=" + $f2.ShotHidden)',
    // L3 极端：50 个长页名 + 绝对长日志路径 ⇒ 必须仍在预算内、且仍点名 ≥1 页
    '$many = @(); for ($i = 1; $i -le 50; $i++) { $many += ("very-long-page-name-number-" + $i + "-suffix.png") }',
    '$vr = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 50 -ShotCount 50 ' +
      '-ChangedCount 50 -ChangedPages $many -DiffVerdict ([pscustomobject]@{Ok=$false;Action="request-decision";ExitCode=1})',
    '$f3 = Format-StyleLoopLine -Verdict $vr -LogPath "C:\\Users\\someone\\AppData\\Local\\Temp\\x.ci-verify\\deep\\style-loop.log" -Seconds 1234',
    'Write-Output ("L3_B=" + $f3.Bytes)',
    'Write-Output ("L3_LADDER=" + $f3.Ladder)',
    'Write-Output ("L3_NAMES=" + (@($f3.ShotNames).Count))',
    'Write-Output ("L3_HIDDEN=" + $f3.ShotHidden)',
    'Write-Output ("L3_LINE=" + $f3.Line)',
    // L4 预算压到 40：结论字段必须活下来（先砍的是尾部，不是结论）
    '$f4 = Format-StyleLoopLine -Verdict $vr -LogPath ".ci-verify/style-loop.log" -Seconds 1234 -MaxBytes 40',
    'Write-Output ("L4_B=" + $f4.Bytes)',
    'Write-Output ("L4_LADDER=" + $f4.Ladder)',
    'Write-Output ("L4_HEADOK=" + ($f4.Line -match \'^STYLE_LOOP gate=none verdict=red\'))',
    'Write-Output ("L4_TILTAIL=" + ($f4.Line -match \'~\\Z\'))',
    // L5 中文页名 ⇒ 行内必须只剩 ASCII（机检判据不得依赖码页）
    '$vc = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 1 -ShotCount 1 ' +
      '-ChangedCount 1 -ChangedPages @("设置页.png") -DiffVerdict ([pscustomobject]@{Ok=$false;Action="request-decision";ExitCode=1})',
    '$f5 = Format-StyleLoopLine -Verdict $vc -LogPath ".ci-verify/style-loop.log" -Seconds 9',
    'Write-Output ("L5_ASCII=" + ($f5.Line -match \'^[\\x20-\\x7E]+\\Z\'))',
    'Write-Output ("L5_LINE=" + $f5.Line)',
    // V 组：阶段优先级与「判不了⇒红」
    '$v1 = Get-StyleLoopVerdict -HasWork $false',
    'Write-Output ("V1=" + $v1.Verdict + "|" + $v1.Phase + "|" + $v1.ExitCode + "|" + $v1.Reason)',
    '$v2 = Get-StyleLoopVerdict -EnvError "no device"',
    'Write-Output ("V2=" + $v2.Verdict + "|" + $v2.Phase + "|" + $v2.ExitCode + "|" + $v2.Reason)',
    '$v3a = Get-StyleLoopVerdict -Deployed $true -CompileOk $false -CompileExit 1',
    'Write-Output ("V3A=" + $v3a.Verdict + "|" + $v3a.Phase + "|" + $v3a.ExitCode + "|" + $v3a.Reason)',
    '$v3b = Get-StyleLoopVerdict -Deployed $true -CompileOk $false -CompileExit 2',
    'Write-Output ("V3B=" + $v3b.Verdict + "|" + $v3b.Phase + "|" + $v3b.ExitCode + "|" + $v3b.Reason)',
    // V4 是本票新增 ExitCode 字段的存在理由：Ok=false 而码是陈旧的 0 ⇒ **仍判红**，且判成红不是 env
    '$v4 = Get-StyleLoopVerdict -Deployed $true -CompileOk $false -CompileExit 0',
    'Write-Output ("V4=" + $v4.Verdict + "|" + $v4.Phase + "|" + $v4.ExitCode + "|" + $v4.Reason)',
    '$v5 = Get-StyleLoopVerdict -Deployed $false -CompileOk $true -CompileExit 0',
    'Write-Output ("V5=" + $v5.Verdict + "|" + $v5.Phase + "|" + $v5.ExitCode + "|" + $v5.Reason)',
    '$v6 = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -ShotError "整帧全黑"',
    'Write-Output ("V6=" + $v6.Verdict + "|" + $v6.Phase + "|" + $v6.ExitCode + "|" + $v6.Reason)',
    '$v7 = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -ShotCount 1 -PageCount 1',
    'Write-Output ("V7=" + $v7.Verdict + "|" + $v7.Phase + "|" + $v7.ExitCode + "|" + $v7.Reason)',
    '$v8 = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 1 -ShotCount 1 ' +
      '-ChangedCount 1 -DiffVerdict ([pscustomobject]@{Ok=$true;Action="write-baseline";ExitCode=0})',
    'Write-Output ("V8=" + $v8.Verdict + "|" + $v8.Phase + "|" + $v8.ExitCode + "|" + $v8.Reason)',
    '$v9 = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 1 -ShotCount 1 ' +
      '-ChangedCount 1 -DiffVerdict ([pscustomobject]@{Ok=$true;Action="refresh-baseline";ExitCode=0})',
    'Write-Output ("V9=" + $v9.Verdict + "|" + $v9.Phase + "|" + $v9.ExitCode + "|" + $v9.Reason)',
    // V10 判定说不通过却给 0 ⇒ 退出码按红收（fail-closed，不放行）
    '$v10 = Get-StyleLoopVerdict -Deployed $true -CompileOk $true -CompileExit 0 -PageCount 1 -ShotCount 1 ' +
      '-ChangedCount 1 -DiffVerdict ([pscustomobject]@{Ok=$false;Action="request-decision";ExitCode=0})',
    'Write-Output ("V10=" + $v10.Verdict + "|" + $v10.Phase + "|" + $v10.ExitCode + "|" + $v10.Reason)',
    // V11/V12 优先级：前一阶段红就定相，后面的坏消息不会把相改掉
    '$v11 = Get-StyleLoopVerdict -HasWork $false -EnvError "no device" -ShotError "x"',
    'Write-Output ("V11=" + $v11.Phase)',
    '$v12 = Get-StyleLoopVerdict -Deployed $false -CompileOk $false -CompileExit 1 -ShotError "x"',
    'Write-Output ("V12=" + $v12.Phase + "|" + $v12.ExitCode)',
    // V13 恒带 gate=none：三档结论的行都得有它（门归属不随阶段变）
    '$gateOk = $true',
    'foreach ($vv in @($green, $red, $v1, $v2, $v4, $v8)) {',
    '  $l = (Format-StyleLoopLine -Verdict $vv -LogPath ".ci-verify/style-loop.log" -Seconds 1).Line',
    '  if ($l -notmatch "gate=none") { $gateOk = $false }',
    '}',
    'Write-Output ("V13_GATE=" + $gateOk)',
    // W 组：预检方向
    '$w1 = Get-StyleLoopWorkState -ChangedFiles @("pages/profile/settings.uvue","scripts/a.ps1")',
    'Write-Output ("W1=" + $w1.HasWork + "|" + $w1.PageFileCount + "|" + $w1.Source)',
    '$w2 = Get-StyleLoopWorkState -ChangedFiles @("scripts/a.ps1","docs/x.md")',
    'Write-Output ("W2=" + $w2.HasWork + "|" + $w2.PageFileCount)',
    '$w3 = Get-StyleLoopWorkState -PagesExplicit',
    'Write-Output ("W3=" + $w3.HasWork + "|" + $w3.Source)',
    '$w4 = Get-StyleLoopWorkState -ChangedFiles @()',
    'Write-Output ("W4=" + $w4.HasWork)',
    // W5 方向性判据：预检是 auto-screenshot 推导集合的**超集** —— 推导能认出页的文件清单，
    //   预检绝不允许判成「没活」（把人挡在门外就是预检最坏的错法）。
    '$w5 = Get-StyleLoopWorkState -ChangedFiles @("pages/index/index.uvue")',
    'Write-Output ("W5=" + $w5.HasWork)',
    // R 组：包装层
    '$r1 = Resolve-StyleLoopChildOutput -Captured "noise\nSTYLE_LOOP gate=none verdict=green phase=done log=x\nSTYLE_LOOP gate=none verdict=red phase=pixel" -ChildExitCode 0 -Seconds 5',
    'Write-Output ("R1=" + $r1.FromChild + "|" + $r1.ExitCode + "|" + $r1.Line)',
    '$r2 = Resolve-StyleLoopChildOutput -Captured "HX_BUSY wait=5 result=timeout" -ChildExitCode 2 -Seconds 9',
    'Write-Output ("R2=" + $r2.FromChild + "|" + $r2.ExitCode + "|" + $r2.Line)',
    '$r3 = Resolve-StyleLoopChildOutput -Captured "" -ChildExitCode 0 -Seconds 3',
    'Write-Output ("R3=" + $r3.FromChild + "|" + $r3.ExitCode + "|" + $r3.Line)',
    'Write-Output "PROBE_DONE=1"',
  ];
  return runPwshCommand(lines.join('\n'));
}

/** C 组：配置字节还原 —— **真文件真字节**，用临时目录，不碰工作树。 */
function probeConfig() {
  const lines = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    `. '${psQuote(LIB)}'`,
    '$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("styleloop-" + [guid]::NewGuid().ToString("N").Substring(0, 8))',
    '$null = New-Item -ItemType Directory -Force -Path $tmp',
    'try {',
    // manifest.json 原字节：CRLF 结尾 + 带 appid（平台回写会把它置空）
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "manifest.json"), [byte[]](0x7B,0x22,0x61,0x70,0x70,0x69,0x64,0x22,0x3A,0x22,0x5F,0x5F,0x55,0x4E,0x49,0x5F,0x41,0x42,0x22,0x7D,0x0D,0x0A))',
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "pages.json"), [byte[]](0x7B,0x22,0x70,0x61,0x67,0x65,0x73,0x22,0x3A,0x5B,0x5D,0x7D))',
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "platformConfig.json"), [byte[]](0x7B,0x7D,0x0A))',
    '$orig = [System.IO.File]::ReadAllBytes((Join-Path $tmp "manifest.json"))',
    '$fp = Get-StyleLoopConfigFingerprint -ProjectDir $tmp',
    'Write-Output ("C0_PATHS=" + (@($fp.Paths) -join ","))',
    // C1 平台回写：appid 置空 + BOM/CRLF 漂移（**文本相同、字节不同**的那一类，正是比文本的还原会漏掉的）
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "manifest.json"), [byte[]](0x7B,0x7D))',
    '[System.IO.File]::WriteAllText((Join-Path $tmp "platformConfig.json"), "{`r`n}", [System.Text.UTF8Encoding]::new($true))',
    '$r = Restore-StyleLoopConfig -Fingerprint $fp',
    'Write-Output ("C1_RESTORED=" + (@($r.Restored) -join ","))',
    'Write-Output ("C1_UNCHANGED=" + (@($r.Unchanged) -join ","))',
    '$now = [System.IO.File]::ReadAllBytes((Join-Path $tmp "manifest.json"))',
    'Write-Output ("C1_BYTES_BACK=" + ((([BitConverter]::ToString($now)) -replace "-","") -eq (([BitConverter]::ToString($orig)) -replace "-","")))',
    '$pc = [System.IO.File]::ReadAllBytes((Join-Path $tmp "platformConfig.json"))',
    'Write-Output ("C1_PC_BYTES=" + (($pc | ForEach-Object { $_.ToString("X2") }) -join " "))',
    // C2 幂等：再还原一次应当「什么都没还原」，三份都在 Unchanged 里
    '$r2 = Restore-StyleLoopConfig -Fingerprint $fp',
    'Write-Output ("C2_RESTORED=" + (@($r2.Restored).Count))',
    'Write-Output ("C2_UNCHANGED=" + (@($r2.Unchanged).Count))',
    // C3 原本没有的文件出现 ⇒ 只报不删
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "platformConfig.json"), [byte[]](0x7B,0x7D))',
    'Remove-Item -LiteralPath (Join-Path $tmp "platformConfig.json") -Force',
    '$r3 = Restore-StyleLoopConfig -Fingerprint $fp',
    'Write-Output ("C3_APPEARED=" + (@($r3.Appeared) -join ","))',
    'Write-Output ("C3_STILLGONE=" + (-not (Test-Path -LiteralPath (Join-Path $tmp "platformConfig.json"))))',
    // C4 -DryRun 只报「会被还原」而不真写盘 ⇒ 与「真调用会写回」成对。
    //   基准必须**分两次读**：$dirty 在调用之前读、$still 在调用之后读。
    //   同一时刻读两次会恒等 —— 那就是一条「根本没判据却恒绿」的弱守护（本仓反复防的形态）。
    '[System.IO.File]::WriteAllBytes((Join-Path $tmp "pages.json"), [byte[]](0x7B,0x7D))',
    '$dirty = [System.IO.File]::ReadAllBytes((Join-Path $tmp "pages.json"))',
    '$dirtySha = ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash($dirty))) -replace "-",""',
    '$r4 = Restore-StyleLoopConfig -Fingerprint $fp -DryRun',
    '$still = [System.IO.File]::ReadAllBytes((Join-Path $tmp "pages.json"))',
    '$stillSha = ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash($still))) -replace "-",""',
    'Write-Output ("C4_RESTORED=" + (@($r4.Restored) -join ","))',
    // 防空转：脏基准确实与快照不同（若两者相等，下面的 NOTWRITTEN 就什么都没说）
    'Write-Output ("C4_DIRTYNOTSNAP=" + ($dirtySha -ne $fp.Hash["pages.json"]))',
    'Write-Output ("C4_NOTWRITTEN=" + ($stillSha -eq $dirtySha))',
    // 成对的另一半：不带 -DryRun 调一次，同一份文件必须被写回快照原字节
    '$null = Restore-StyleLoopConfig -Fingerprint $fp',
    '$after = [System.IO.File]::ReadAllBytes((Join-Path $tmp "pages.json"))',
    '$afterSha = ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash($after))) -replace "-",""',
    'Write-Output ("C4_REALLYBACK=" + ($afterSha -eq $fp.Hash["pages.json"]))',
    // D 组：首次运行的**基线落盘动作**（判定表 write-baseline 那一支只给结论，动作归调用方）
    //   没有这一步，入口第一轮的绿是空的 —— 基线永远为空、每轮都「首次运行」、判据永不咬（ADR-0008:419）。
    '$dsrc = Join-Path $tmp "shots"; $dbase = Join-Path $tmp "base"',
    '$null = New-Item -ItemType Directory -Force -Path $dsrc, $dbase',
    '[System.IO.File]::WriteAllBytes((Join-Path $dsrc "d1.png"), [byte[]](1,2,3,4))',
    '[System.IO.File]::WriteAllBytes((Join-Path $dsrc "d2.png"), [byte[]](5,6,7,8))',
    '(Get-Item (Join-Path $dsrc "d2.png")).LastWriteTime = (Get-Date).AddHours(-5)',
    '$seedStart = (Get-Date).AddMinutes(-1)',
    '$seed = Copy-StyleLoopBaselineShot -ProjectDir $tmp -RunStartedAt $seedStart -SourceDir $dsrc -BaselineDir $dbase',
    'Write-Output ("D1_COPIED=" + (@($seed.Copied) -join ","))',
    'Write-Output ("D1_SKIPPED=" + (@($seed.Skipped) -join ","))',
    'Write-Output ("D1_BASECOUNT=" + (@(Get-ChildItem $dbase -Filter "*.png" -File).Count))',
    '$b1 = [System.IO.File]::ReadAllBytes((Join-Path $dbase "d1.png"))',
    'Write-Output ("D1_BYTES=" + (($b1 -join ",") -eq "1,2,3,4"))',
    // D2 不传 RunStartedAt ⇒ 全拷（旧形态向后兼容，不静默少拷）
    '$seed2 = Copy-StyleLoopBaselineShot -ProjectDir $tmp -SourceDir $dsrc -BaselineDir $dbase',
    'Write-Output ("D2_COPIED=" + (@($seed2.Copied) -join ","))',
    // D3 源目录不存在 ⇒ 明报 SourceExists=False 且不抛（入口不能因为没图就崩在半路）
    '$seed3 = Copy-StyleLoopBaselineShot -ProjectDir $tmp -SourceDir (Join-Path $tmp "nope") -BaselineDir (Join-Path $tmp "base3")',
    'Write-Output ("D3_EXISTS=" + $seed3.SourceExists + "|" + (@($seed3.Copied).Count))',
    '} finally {',
    '  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue',
    '}',
    'Write-Output "PROBE_DONE=1"',
  ];
  return runPwshCommand(lines.join('\n'));
}

function gitInit(dir) {
  const run = (args) => execFileSync('git', args, { cwd: dir, encoding: 'utf8', windowsHide: true });
  run(['init', '-q']);
  run(['config', 'user.email', 'probe@example.invalid']);
  run(['config', 'user.name', 'probe']);
  fs.writeFileSync(path.join(dir, 'README.md'), 'seed\n');
  run(['add', 'README.md']);
  run(['commit', '-q', '-m', 'seed']);
  return run;
}

describe('样式内循环入口：那一行的形状与阶段结论（lib 纯函数真跑）', () => {
  let out;

  beforeAll(() => {
    const r = probePure();
    if (!r.ok) {
      throw new Error('pwsh 探针失败（fail-closed，不跳过）：\n' + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`);
    }
    out = r.stdout;
    if (field(out, 'PROBE_DONE') !== '1') {
      throw new Error('pwsh 探针没跑完（中途抛错）：\n' + out + '\n' + r.stderr);
    }
  });

  test('L1: 绿 ⇒ verdict=green 且退出码 0，行 ≤200 字节、单行 ASCII、恒带 gate=none', () => {
    expect(field(out, 'GREEN_V')).toBe('green|done|0|clean');
    const line = field(out, 'L1_LINE');
    expect(Number(field(out, 'L1_B'))).toBeLessThanOrEqual(LINE_BUDGET);
    expect(Buffer.byteLength(line, 'utf8')).toBe(Number(field(out, 'L1_B')));
    expect(field(out, 'L1_ASCII')).toBe('True');
    expect(Number(field(out, 'L1_NEWLINES'))).toBe(0);
    expect(line).toMatch(/ gate=none /);
    expect(fieldOf(line, 'verdict')).toBe('green');
    expect(fieldOf(line, 'shot')).toBe('-');
    expect(fieldOf(line, 'log')).toBe('.ci-verify/style-loop.log');
  });

  // 票面第 2 条：改坏一行样式 ⇒ 非零退出，且**机检行指出是哪一页变的**
  test('L2: 像素红 ⇒ 退出码 1、changed=2、行内点名两页（对照 L1 的绿）', () => {
    expect(field(out, 'L2_V')).toBe('red|pixel|1|pixel-changed');
    const line = field(out, 'L2_LINE');
    expect(fieldOf(line, 'verdict')).toBe('red');
    expect(fieldOf(line, 'changed')).toBe('2');
    expect(fieldOf(line, 'shot')).toBe('settings.png,course-detail.png');
    expect(Number(field(out, 'L2_B'))).toBeLessThanOrEqual(LINE_BUDGET);
    // 绿的那一行不带页面名，红的那一行必须带 —— 这条差异就是「指出哪一页」的判据面
    expect(field(out, 'L1_LINE')).not.toMatch(/shot=settings/);
  });

  test('L3: 50 个长页名 + 绝对长日志路径 ⇒ 仍 ≤200 字节，且**仍然点名的那一页还在**', () => {
    const bytes = Number(field(out, 'L3_B'));
    expect(bytes).toBeLessThanOrEqual(LINE_BUDGET);
    expect(bytes).toBeGreaterThan(0);
    // 收缩必须落在「日志取末段」或「页名截断」这两档之一（具体哪档取决于路径长度，不钉死一档）
    expect(['leaf-log', 'clipped-name']).toContain(field(out, 'L3_LADDER'));
    expect(Number(field(out, 'L3_NAMES'))).toBeGreaterThan(0);
    expect(Number(field(out, 'L3_HIDDEN'))).toBeGreaterThanOrEqual(40);
    const line = field(out, 'L3_LINE');
    expect(line).toMatch(/shot=[^ ]+\+\d/);
  });

  test('L4: 预算压到 40 字节 ⇒ 结论字段活下来，尾巴以 ~ 明示「这行不完整」', () => {
    expect(Number(field(out, 'L4_B'))).toBeLessThanOrEqual(40);
    expect(field(out, 'L4_HEADOK')).toBe('True');
    expect(field(out, 'L4_TILTAIL')).toBe('True');
    expect(field(out, 'L4_LADDER')).toBe('hard-trim');
  });

  test('L5: 中文页名被替换成 ASCII（机检判据不得依赖码页；#1542 缺陷 5 的同一坑）', () => {
    expect(field(out, 'L5_ASCII')).toBe('True');
    expect(field(out, 'L5_LINE')).not.toMatch(/[^\x20-\x7E]/);
    // 名字被转写而不是被丢掉：仍然要点名
    expect(fieldOf(field(out, 'L5_LINE'), 'shot')).toMatch(/\.png$/);
  });

  test('V: 阶段结论与退出码逐条对账（0=绿 / 1=红 / 2=环境不可用）', () => {
    expect(field(out, 'V1')).toBe('red|precheck|1|no-work');
    expect(field(out, 'V2')).toBe('env|env|2|env-unavailable');
    expect(field(out, 'V3A')).toBe('red|compile|1|compile-diagnostic');
    expect(field(out, 'V3B')).toBe('env|compile|2|hx-env-unavailable');
    expect(field(out, 'V5')).toBe('env|deploy|2|not-deployed');
    expect(field(out, 'V6')).toBe('red|shot|1|screenshot-failed');
    expect(field(out, 'V8')).toBe('green|baseline-written|0|first-baseline');
    expect(field(out, 'V9')).toBe('green|baseline-refreshed|0|baseline-refreshed');
  });

  // 反「判不了当无变化」——截图或像素层拿不到结论时必须是红
  test('V7/V10: 判不了与「说不通过却给 0」都收在红（不是绿、也不是 env）', () => {
    expect(field(out, 'V7')).toBe('red|pixel|1|no-diff-verdict');
    expect(field(out, 'V10')).toBe('red|pixel|1|pixel-changed');
  });

  // 这条锁住 #1543 给 lib/test-compile.ps1 新增 ExitCode 字段的理由：
  //   判红权在 Ok，退出码**只用来分方向**。若谁把判据写成「ExitCode -ne 0 才算红」，
  //   陈旧 $LASTEXITCODE（子进程没拉起来时它可能还是上一条命令留下的 0）就会让红漏成绿。
  test('V4: CompileOk=false 而退出码是陈旧的 0 ⇒ 仍判红且归到 compile（不是 env、更不是绿）', () => {
    expect(field(out, 'V4')).toBe('red|compile|1|compile-diagnostic');
  });

  test('V11/V12: 优先级即阶段顺序 —— 前一阶段的红定相，后面的坏消息不改相', () => {
    expect(field(out, 'V11')).toBe('precheck');
    expect(field(out, 'V12')).toBe('compile|1');
  });

  // 票面第 4 条：入口不进 CI、不改任何门的触发面与归属 ⇒ 行内恒自报 gate=none
  test('V13: 六档结论的行都带 gate=none（门归属不随阶段变）', () => {
    expect(field(out, 'V13_GATE')).toBe('True');
  });

  test('W: 预检方向 —— 有 pages/ 改动⇒放行、只有脚本/文档⇒红、显式 -Pages⇒放行', () => {
    expect(field(out, 'W1')).toBe('True|1|git-diff');
    expect(field(out, 'W2')).toBe('False|0');
    expect(field(out, 'W3')).toBe('True|explicit-pages');
    expect(field(out, 'W4')).toBe('False');
    // 超集方向：auto-screenshot 认得出的页，预检**不得**说没活
    expect(field(out, 'W5')).toBe('True');
  });

  test('R1: 子进程打出了行 ⇒ 原样 relay 第一行，退出码不被包装层改写', () => {
    expect(field(out, 'R1')).toBe(
      'True|0|STYLE_LOOP gate=none verdict=green phase=done log=x'
    );
  });

  test('R2: 子进程没打行（Wait-HxFree 超时 exit 2 那一形）⇒ 合成 env 且保留码 2', () => {
    const r2 = field(out, 'R2');
    expect(r2.split('|').slice(0, 2).join('|')).toBe('False|2');
    expect(r2).toMatch(/verdict=env phase=child .*reason=child-exited-2/);
  });

  // 反「什么都没做却报成功」：自称成功（码 0）却拿不出结论行 ⇒ 改判红
  test('R3: 子进程静默却退出 0 ⇒ 合成红并把退出码改判 1', () => {
    const r3 = field(out, 'R3');
    expect(r3.split('|').slice(0, 2).join('|')).toBe('False|1');
    expect(r3).toMatch(/verdict=red phase=child .*reason=child-silent/);
  });
});

describe('样式内循环入口：配置字节还原与基线落盘（真文件真字节，临时目录）', () => {
  let out;

  beforeAll(() => {
    const r = probeConfig();
    if (!r.ok) {
      throw new Error('配置还原探针失败（fail-closed，不跳过）：\n' + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`);
    }
    out = r.stdout;
    if (field(out, 'PROBE_DONE') !== '1') {
      throw new Error('配置还原探针没跑完（中途抛错）：\n' + out + '\n' + r.stderr);
    }
  });

  test('C0: 三份平台配置文件都在射程里（platformConfig.json 不能漏 —— 它是打包面判据来源）', () => {
    expect(field(out, 'C0_PATHS')).toBe('manifest.json,pages.json,platformConfig.json');
  });

  // 票面第 6 条。这条同时是「为什么字节还原不是 build-deploy 那层文本还原的重复劳动」的证据：
  // 平台把 appid 置空 + 给 platformConfig 加 BOM/改行尾，文本判据看不见第二类的变化，字节判据看得见。
  test('C1: appid 置空与 BOM/行尾漂移都被还原成原字节', () => {
    expect(field(out, 'C1_RESTORED')).toBe('manifest.json,platformConfig.json');
    expect(field(out, 'C1_BYTES_BACK')).toBe('True');
    expect(field(out, 'C1_PC_BYTES')).toBe('7B 7D 0A');
    expect(field(out, 'C1_UNCHANGED')).toBe('pages.json');
  });

  test('C2: 还原一次就够 —— 第二次 Restored 为空、三份全部 Unchanged（不自我覆盖、不反复写盘）', () => {
    expect(field(out, 'C2_RESTORED')).toBe('0');
    expect(field(out, 'C2_UNCHANGED')).toBe('3');
  });

  test('C3: 原本没有的文件出现 ⇒ 只报不删（入口无权判断它是平台回写还是人刚建的）', () => {
    expect(field(out, 'C3_APPEARED')).toContain('platformConfig.json');
    expect(field(out, 'C3_STILLGONE')).toBe('True');
  });

  test('C4: -DryRun 只报「会被还原」而不写盘；不带它再调一次才真写回（成对）', () => {
    expect(field(out, 'C4_RESTORED')).toBe('pages.json');
    // 防空转：脏基准必须确实不同于快照，否则「没写回」这句话没有内容
    expect(field(out, 'C4_DIRTYNOTSNAP')).toBe('True');
    expect(field(out, 'C4_NOTWRITTEN')).toBe('True');
    expect(field(out, 'C4_REALLYBACK')).toBe('True');
  });

  // 这条是「绿了但什么都没建」那种空转的封口：判定表说 write-baseline，产物里就得真有基线
  test('D1: 首次运行 ⇒ 本轮截图真的写进基线目录，非本轮残留被跳过且不进基线', () => {
    expect(field(out, 'D1_COPIED')).toBe('d1.png');
    expect(field(out, 'D1_SKIPPED')).toBe('d2.png');
    expect(field(out, 'D1_BASECOUNT')).toBe('1');
    expect(field(out, 'D1_BYTES')).toBe('True');
  });

  test('D2: 不传运行起点 ⇒ 两张都拷（旧形态向后兼容，不静默少拷）', () => {
    expect(field(out, 'D2_COPIED')).toBe('d1.png,d2.png');
  });

  test('D3: 截图目录不存在 ⇒ 明报 SourceExists=False 且拷贝数为 0（不抛错、不假装建好）', () => {
    expect(field(out, 'D3_EXISTS')).toBe('False|0');
  });
});

describe('样式内循环入口：真跑三条不碰 HBuilderX 的路径（E 组）', () => {
  let tmpClean;
  let tmpDirty;

  beforeAll(() => {
    tmpClean = fs.mkdtempSync(path.join(os.tmpdir(), 'styleloop-clean-'));
    tmpDirty = fs.mkdtempSync(path.join(os.tmpdir(), 'styleloop-dirty-'));
    const runDirty = gitInit(tmpDirty);
    // 预检与 auto-screenshot 的推导一样**只看 git diff**（未跟踪的新页不在集合里 —— 那是
    // 「新增页面」，本属 🔴 完整档，入口用 -Pages 覆盖）。所以夹具必须是**已入库页被改脏**，
    // 那才是样式内循环的真实形态；写成未跟踪的新文件会让预检正确地判「没活」，测不到放行那支。
    fs.mkdirSync(path.join(tmpDirty, 'pages', 'profile'), { recursive: true });
    fs.writeFileSync(path.join(tmpDirty, 'pages', 'profile', 'settings.uvue'), '<template><view class="a"/></template>\n');
    runDirty(['add', 'pages/profile/settings.uvue']);
    runDirty(['commit', '-q', '-m', 'page']);
    fs.writeFileSync(path.join(tmpDirty, 'pages', 'profile', 'settings.uvue'), '<template><view class="b"/></template>\n');
  });

  afterAll(() => {
    [tmpClean, tmpDirty].forEach((d) => {
      if (d) {
        try { fs.rmSync(d, { recursive: true, force: true }); } catch { /* 清理失败不判红 */ }
      }
    });
  });

  test('E1: -DryRun ⇒ 退出码 0、终端**恰好一行** STYLE_LOOP、带 gate=none、verdict=plan（计划不是判据）', () => {
    const r = runEntry(['-ProjectDir', tmpClean, '-DryRun']);
    const ms = machineLines(r.stdout);
    expect(ms.length).toBe(1);
    expect(r.status).toBe(0);
    expect(fieldOf(ms[0], 'verdict')).toBe('plan');
    expect(fieldOf(ms[0], 'gate')).toBe('none');
    expect(Buffer.byteLength(ms[0], 'utf8')).toBeLessThanOrEqual(LINE_BUDGET);
  });

  // 成对（必红）：干净树 ⇒ 预检就收口。判据用**产物**而不是计时：锁日志没被创建，
  // 才证明「没装机」这件事真的发生了（#1544 刚把墙钟毫秒断言换掉，这里不再引入新的计时判据）。
  test('E2: 无 pages/ 改动 ⇒ 退出码 1、恰好一行、reason=no-work，且没创建 HBuilderX 锁日志', () => {
    const r = runEntry(['-ProjectDir', tmpClean]);
    const ms = machineLines(r.stdout);
    expect(r.status).toBe(1);
    expect(ms.length).toBe(1);
    expect(fieldOf(ms[0], 'verdict')).toBe('red');
    expect(fieldOf(ms[0], 'phase')).toBe('precheck');
    expect(fieldOf(ms[0], 'reason')).toBe('no-work');
    expect(fs.existsSync(path.join(tmpClean, '.ci-verify', 'style-loop-hx.log'))).toBe(false);
    // 日志仍落盘（票面第 3 条：一行以外的事实在磁盘上）
    expect(fs.existsSync(path.join(tmpClean, '.ci-verify', 'style-loop.log'))).toBe(true);
  });

  // 成对（对照 E2）：有 pages/ 改动 ⇒ 预检放行、走到环境，被不存在的设备挡住 ⇒ 退出码 2（不是 1）。
  // 一律带 -Device <不存在>：lib/env-check.ps1 在设备判定就返回，排在 HBuilderX 探测与取锁之前。
  test('E3: 有 pages/ 改动 ⇒ 预检放行、环境阶段判 env（退出码 2），同样只回一行', () => {
    const r = runEntry(['-ProjectDir', tmpDirty, '-Device', 'no-such-device-xyz', '-HxWaitSeconds', '5']);
    const ms = machineLines(r.stdout);
    expect(r.status).toBe(2);
    expect(ms.length).toBe(1);
    expect(fieldOf(ms[0], 'verdict')).toBe('env');
    expect(fieldOf(ms[0], 'phase')).toBe('env');
    expect(fs.existsSync(path.join(tmpDirty, '.ci-verify', 'style-loop-hx.log'))).toBe(false);
    expect(fs.existsSync(path.join(tmpDirty, '.ci-verify', 'style-loop.log'))).toBe(true);
  });

  test('E4: 显式 -Pages 时预检让路（首轮建基线那一支：改动集里还没有 .uvue 也要能跑）', () => {
    const r = runEntry(['-ProjectDir', tmpClean, '-Pages', 'pages/profile/settings', '-Device', 'no-such-device-xyz']);
    const ms = machineLines(r.stdout);
    expect(r.status).toBe(2);
    expect(ms.length).toBe(1);
    expect(fieldOf(ms[0], 'phase')).toBe('env');
  });
});
