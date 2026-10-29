/**
 * `-FullRepo` 全仓逐页载体的行为守护（运行期）—— 2026-10-28，#2078
 *
 * 为什么必须是运行期断言（接线层钉不住的那一半）：
 *   S25-S31 钉的是「代码里有这些形状」。但本票 ⑥ 最硬那条 ——「60 页清单与产物计数相符」——
 *   靠形状**证明不了**：它取决于*真读仓内 pages.json 之后，产物文件名会不会互相覆盖*。
 *   现测事实：主树 60 条页路径按末段只有 **59** 个唯一值（`pages/login/login` 与 `pages/recruiter/login`）
 *   ⇒ 沿用末段命名的话「截 60 页、落盘 59 张」，而任何文本守护都看不出来（那行源码完全正确）。
 *   FR1 因此**真读真文件**算两个唯一数，并把「末段更少」本身也钉住：坑被重新引入时它会红。
 *
 * 断言：
 *   FR1 真实 pages.json ⇒ 计划页数 == 拍平后的唯一名数；且末段唯一数**严格更少**（重名坑的现场证据）
 *   FR2 `subPackages` 的 path 相对各自 root ⇒ 清单必须拼成 `root/path`（现数 0 项，
 *       所以这一支不能用真文件证明它不是死代码，只能用夹具真跑）
 *   FR3 票号非纯数字 ⇒ 当场红且**一个目录都不建**（授权位先于任何副作用的运行期对应）
 *   FR4 `-FullRepo` 与 `-Pages` 同给 ⇒ 当场红、同样不建目录（不静默取其一）
 *   FR5 manifest 真落盘 + 真读回：本轮对账与目录累计对账是**两个不同结论**，都写在文件里
 *
 * ⚠️ 判据一律在 pwsh 侧算成 ASCII token（True/False/数字），JS 侧不匹配中文文本 ——
 *   同源血账见 `autoScreenshotAdbAgreementBehavior.test.js` 文件头与 `autoScreenshotContract.test.js` S20。
 * ⚠️ 路径经**环境变量**传入，不在 JS 里拼 PowerShell 字面量：本仓中文目录名 + 反斜杠混排时，
 *   字符串插值一处写错就是「探针跑的是假路径」那种静默失效（#2078 会话实测踩过，改成 env 后消停）。
 *
 * 安全性：FR3/FR4 走授权门，在 adb / cli / 设备之前就返回 ⇒ 不派发 launch、不碰 HBuilderX、不碰真机；
 *   FR1/FR2/FR5 只读文件与写临时目录。整个用例不需要设备。
 * 运行前提：需要 `pwsh`（PowerShell 7）。不可用时 fail-closed 抛错，不 skip（仓库先例）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const AUTO_SHOT = path.join(ROOT, 'scripts', 'lib', 'auto-screenshot.ps1');
const PAGES_JSON = path.join(ROOT, 'pages.json');

/** 一次 pwsh 往返跑完 FR1-FR5；路径全走 env，脚本体内只用 $env: 引用。 */
function probe(ctx) {
  const s = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    ". \"$env:FR_AUTO_SHOT\"",
    // ---- FR1：真读主树 pages.json ----
    "$pj = Get-Content -LiteralPath $env:FR_PAGES_JSON -Raw | ConvertFrom-Json",
    "$planned = @(Get-FullRepoPageList -PagesJson $pj)",
    "$flat = @($planned | ForEach-Object { Get-FullRepoShotName -Page $_ } | Sort-Object -Unique)",
    "$leaf = @($planned | ForEach-Object { ($_ -split '/')[-1] } | Sort-Object -Unique)",
    "Write-Output ('FR1_PLANNED=' + @($planned).Count)",
    "Write-Output ('FR1_FLAT=' + @($flat).Count)",
    "Write-Output ('FR1_LEAF=' + @($leaf).Count)",
    // ---- FR2：subPackages 必须拼 root ----
    "$sub = '{\"pages\":[{\"path\":\"pages/a/a\"}],\"subPackages\":[{\"root\":\"subB\",\"pages\":[{\"path\":\"pages/c/c\"}]}]}' | ConvertFrom-Json",
    "$l2 = @(Get-FullRepoPageList -PagesJson $sub)",
    "Write-Output ('FR2_COUNT=' + @($l2).Count)",
    "Write-Output ('FR2_JOINED=' + [bool]($l2 -contains 'subB/pages/c/c'))",
    // ---- FR3 / FR4：授权门（必须在建任何目录之前返回）----
    "$r3 = Invoke-AutoScreenshot -FullRepo 'abc' -ProjectDir $env:FR_PROJ",
    "Write-Output ('FR3_OK=' + $r3.Ok)",
    "Write-Output ('FR3_NODIR=' + [bool](-not (Test-Path -LiteralPath (Join-Path $env:FR_PROJ 'docs'))))",
    "$r4 = Invoke-AutoScreenshot -FullRepo '1864' -Pages 'pages/a/a' -ProjectDir $env:FR_PROJ",
    "Write-Output ('FR4_OK=' + $r4.Ok)",
    "Write-Output ('FR4_NODIR=' + [bool](-not (Test-Path -LiteralPath (Join-Path $env:FR_PROJ 'docs'))))",
    // ---- FR5：manifest 真落盘真读回（本轮 vs 目录累计，两个结论）----
    "[System.IO.File]::WriteAllBytes((Join-Path $env:FR_OUT 'pages_a_a.png'), [byte[]](1,2,3))",
    "$plan = @('pages/a/a','pages/b/b','pages/c/c')",
    "$mf = Get-FullRepoManifestData -Ticket '1864' -PlannedPages $plan -Screenshots @('pages_a_a') -OutputDir $env:FR_OUT",
    "$w = Write-FullRepoManifest -Path (Join-Path $env:FR_OUT 'fullrepo-manifest.json') -Manifest $mf",
    "Write-Output ('FR5_WRITE=' + $w.Ok)",
    "$back = Get-Content -LiteralPath (Join-Path $env:FR_OUT 'fullrepo-manifest.json') -Raw | ConvertFrom-Json",
    "Write-Output ('FR5_SCHEMA=' + $back.Schema)",
    "Write-Output ('FR5_ROUNDTRIP=' + [bool]([int]$back.PlannedCount -eq 3))",
    "Write-Output ('FR5_THISRUN_MATCH=' + [bool]$back.CountsMatch)",
    "Write-Output ('FR5_DIR_MATCH=' + [bool]$back.DirCountsMatch)",
    "Write-Output ('FR5_THISRUN_MISSING=' + @($back.Missing).Count)",
    "Write-Output ('FR5_DIR_MISSING=' + @($back.DirMissing).Count)",
    // ---- FR6 / FR7：起跑档（本轮裁、全仓不裁）----
    "$allReal = @(Get-FullRepoPageList -PagesJson $pj)",
    "$one = @(Select-FullRepoAttempted -PlannedPages $allReal -RampPages 1)",
    "$none = @(Select-FullRepoAttempted -PlannedPages $allReal -RampPages 0)",
    "$over = @(Select-FullRepoAttempted -PlannedPages $allReal -RampPages 9999)",
    "Write-Output ('FR6_ONE=' + @($one).Count)",
    "Write-Output ('FR6_FIRST=' + $one[0])",
    "Write-Output ('FR6_NONE=' + @($none).Count)",
    "Write-Output ('FR6_OVER=' + @($over).Count)",
    // 起跑轮的真实形态：本轮那一张就落在**同一个归档目录**里，累计对账才该说「还差 59 页」
    "$rampPng = Join-Path $env:FR_OUT2 'pages_index_index.png'",
    "[System.IO.File]::WriteAllBytes($rampPng, [byte[]](1,2,3))",
    "$mfRamp = Get-FullRepoManifestData -Ticket '1864' -PlannedPages $allReal -AttemptedPages $one -Screenshots @('pages_index_index') -OutputDir $env:FR_OUT2",
    "Write-Output ('FR7_PLANNED=' + $mfRamp.PlannedCount)",
    "Write-Output ('FR7_ATTEMPTED=' + $mfRamp.AttemptedCount)",
    "Write-Output ('FR7_RAMP=' + [bool]$mfRamp.RampActive)",
    "Write-Output ('FR7_THISRUN_MATCH=' + [bool]$mfRamp.CountsMatch)",
    "Write-Output ('FR7_REPO_MATCH=' + [bool]$mfRamp.DirCountsMatch)",
    "Write-Output ('FR7_REPO_MISSING=' + @($mfRamp.DirMissing).Count)",
    // ---- FR8：断路腿（设备不可用）必须报得出来，且零产物不冒充绿 ----
    // ⚠️ 这里**只断言与环境无关**的收口事实。原先还断言 FR8_PLANNED>0 / FR8_ATTEMPTED=1 / FR8_MF=True：
    //    那三条在本机（有 adb ⇒ 走到灭屏那一支，分母已算过）为真，在 CI（ubuntu 无 adb ⇒ 走「找不到 adb.exe」
    //    那一支，环境检查在计划之前）恒为空 —— 是「本地永远绿、CI 永远红」的断言，#2078 三次连红就红在这。
    //    分母/分批评的纯逻辑由 FR1/FR6/FR7/FR10/FR11 直接调纯函数钉（不吃环境）；无 adb 那一支由 FR8N 钉。
    "$outB = Join-Path $env:FR_OUT3 'brk'",
    "$rb = Invoke-AutoScreenshot -FullRepo '1864' -RampPages 1 -Device 'nosuchserial0000' -OutputDir $outB -AdbCallTimeoutSeconds 5 6>$null",
    "Write-Output ('FR8_OK=' + $rb.Ok)",
    "Write-Output ('FR8_VERDICT=' + [string]$rb.Verdict)",
    "Write-Output ('FR8_SHOTS=' + @($rb.Screenshots).Count)",
    "Write-Output ('FR8_PNG=' + @(Get-ChildItem -Path $outB -Filter '*.png' -File -ErrorAction SilentlyContinue).Count)",
    // 「不冒充绿」的机器形态：本轮若真落了 manifest，它也不得自称已对齐。
    // 空计划时 Get-FullRepoCounts 恒判相符 ⇒ 在这里「顺手写一份 manifest」正是本票要挡的假绿。
    "$mfGreen = $false",
    "if ($rb.ManifestPath -and (Test-Path -LiteralPath $rb.ManifestPath)) {",
    "    $mj = Get-Content -LiteralPath $rb.ManifestPath -Raw | ConvertFrom-Json",
    "    $mfGreen = [bool](([bool]$mj.CountsMatch) -or ([bool]$mj.DirCountsMatch))",
    "}",
    "Write-Output ('FR8_MF_GREEN=' + $mfGreen)",
    // ---- FR9：中转页白名单（真读仓内 json，不写死页名）----
    "$map = Get-RedirectMap -Path (Join-Path $env:FR_LIB 'auto-screenshot-redirects.json')",
    "Write-Output ('FR9_HAS_INDEX=' + [bool]$map.ContainsKey('pages/index/index'))",
    "Write-Output ('FR9_LANDING=' + (@($map['pages/index/index']) -join ','))",
    "$cntRedir = Get-FullRepoCounts -PlannedPages @('pages/index/index','pages/b/b') -CapturedNames @('pages_b_b') -RedirectedNames @('pages_index_index')",
    "Write-Output ('FR9_MISSING=' + @($cntRedir.Missing).Count)",
    "Write-Output ('FR9_MATCH=' + [bool]$cntRedir.CountsMatch)",
    "$mfRedir = Get-FullRepoManifestData -Ticket '1864' -PlannedPages @('pages/index/index','pages/b/b') -Screenshots @('pages_b_b') -Redirected @([pscustomobject]@{ Page='pages/index/index'; Name='pages_index_index'; To='pages/dashboard/dashboard'; Seconds=12 }) -OutputDir $env:FR_OUT4",
    "Write-Output ('FR9_MF_REDIR=' + @($mfRedir.Redirected).Count)",
    "Write-Output ('FR9_MF_REDIRCOUNT=' + $mfRedir.RedirectedCount)",
    "$mfRedir2 = Get-FullRepoManifestData -Ticket '1864' -PlannedPages @('pages/index/index','pages/b/b') -Screenshots @('pages_b_b') -OutputDir $env:FR_OUT4",
    "Write-Output ('FR9_NO_REDIR_MISSING=' + @($mfRedir2.Missing).Count)",
    // ---- FR10 / FR11：按前缀分批（学员 51 / 招聘者 9）----
    // 直接调**纯函数**。原先穿过 Invoke-AutoScreenshot 去断言页数，而没有 adb 的 CI 在环境检查处就早退
    // ⇒ AttemptedPages / PlannedPages / Excluded 恒空 ⇒「60 页里排掉 9 页」结构性不可能通过。
    // 接线面（参数确实被传进这个纯函数、全仓计划不被排除污染）由接线腿 S34/S35/S36 钉；纯逻辑的证据在这里。
    "$sel10 = Select-FullRepoByPrefix -Pages $allReal -ExcludePrefix 'pages/recruiter/'",
    "Write-Output ('FR10_PLANNED=' + @($allReal).Count)",
    "Write-Output ('FR10_ATTEMPTED=' + @($sel10.Attempted).Count)",
    "Write-Output ('FR10_EXCLUDED=' + @($sel10.Excluded).Count)",
    "$bad = @($sel10.Excluded | Where-Object { -not $_.StartsWith('pages/recruiter/') }).Count",
    "Write-Output ('FR10_EXCLUDED_OFFPREFIX=' + $bad)",
    // 「只裁本轮」的分区证据：本轮 + 被排除 == 全仓 ⇒ 一页都没被吞掉
    "Write-Output ('FR10_PARTITION=' + [bool]((@($sel10.Attempted).Count + @($sel10.Excluded).Count) -eq @($allReal).Count))",
    // 先 Only 再 Exclude 的叠加语义（三批口径靠它确定）
    "$selCombo = Select-FullRepoByPrefix -Pages $allReal -OnlyPrefix 'pages/' -ExcludePrefix 'pages/recruiter/'",
    "Write-Output ('FR10_COMBO_ATTEMPTED=' + @($selCombo.Attempted).Count)",
    "Write-Output ('FR10_COMBO_EXCLUDED=' + @($selCombo.Excluded).Count)",
    "$re2 = Invoke-AutoScreenshot -ExcludePrefix 'pages/recruiter/' -Pages 'pages/login/login' 6>$null",
    "Write-Output ('FR10_GATE_OK=' + $re2.Ok)",
    // ---- FR11：只选前缀（招聘方 9 页那一批的入口）----
    "$sel11 = Select-FullRepoByPrefix -Pages $allReal -OnlyPrefix 'pages/recruiter/'",
    "Write-Output ('FR11_PLANNED=' + @($allReal).Count)",
    "Write-Output ('FR11_ATTEMPTED=' + @($sel11.Attempted).Count)",
    "$off = @($sel11.Attempted | Where-Object { -not $_.StartsWith('pages/recruiter/') }).Count",
    "Write-Output ('FR11_OFFPREFIX=' + $off)",
    "Write-Output ('FR11_EXCLUDED=' + @($sel11.Excluded).Count)",
    // ---- FR8N：无 adb 那一支 ----
    // 本机有 adb ⇒ 那一支只在上游显形（#2078 三次连红的现场）。把解析器 stub 成 \$null，每台机器都真跑它。
    // 必须放最后：stub 会改变本进程内后续任何 Invoke-AutoScreenshot 的环境分支。
    "function Resolve-AdbExeLocal { return $null }",
    "$outN = Join-Path $env:FR_OUT3 'noadb'",
    "$rn = Invoke-AutoScreenshot -FullRepo '1864' -Device 'x' -OutputDir $outN",
    "Write-Output ('FR8N_OK=' + $rn.Ok)",
    "Write-Output ('FR8N_VERDICT=' + [string]$rn.Verdict)",
    "Write-Output ('FR8N_ERR_ADB=' + [bool](([string]$rn.Error) -match 'adb'))",
    "Write-Output ('FR8N_SHOTS=' + @($rn.Screenshots).Count)",
    "Write-Output ('FR8N_PLANNED=' + @($rn.PlannedPages).Count)",
    "Write-Output ('FR8N_ATTEMPTED=' + @($rn.AttemptedPages).Count)",
    "Write-Output ('FR8N_MF_PATH=' + [string]$rn.ManifestPath)",
    "Write-Output ('FR8N_MF_EXISTS=' + [bool]($rn.ManifestPath -and (Test-Path -LiteralPath $rn.ManifestPath)))",
    "Write-Output ('FR8N_PNG=' + @(Get-ChildItem -Path $outN -Filter '*.png' -File -ErrorAction SilentlyContinue).Count)",
    'Write-Output "PROBE_DONE=1"',
  ].join('\n');
  const encoded = Buffer.from(s, 'utf16le').toString('base64');
  const env = Object.assign({}, process.env, {
    FR_AUTO_SHOT: AUTO_SHOT,
    FR_PAGES_JSON: PAGES_JSON,
    FR_PROJ: ctx.proj,
    FR_OUT: ctx.out,
    FR_OUT2: ctx.out2,
    FR_OUT3: ctx.out3,
    FR_OUT4: ctx.out4,
    FR_OUT5: ctx.out5,
    FR_OUT6: ctx.out6,
    FR_LIB: path.join(ROOT, 'scripts', 'lib'),
  });
  try {
    const stdout = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass',
      '-OutputFormat', 'Text', '-EncodedCommand', encoded], {
      encoding: 'utf8', timeout: 180000, windowsHide: true, maxBuffer: 8 * 1024 * 1024, env,
    });
    return { ok: true, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return { ok: false, stdout: String(e.stdout || ''), stderr: String(e.stderr || e.message || '') };
  }
}

function field(stdout, key) {
  const m = String(stdout).match(new RegExp('^' + key + '=(.*)$', 'm'));
  return m ? m[1].trim() : null;
}

describe('-FullRepo 全仓逐页载体（运行期，#2078）', () => {
  let ctx;
  let probeRun;

  beforeAll(() => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'fullrepo-'));
    ctx = { tmp, proj: path.join(tmp, 'proj'), out: path.join(tmp, 'arch'), out2: path.join(tmp, 'ramp'), out3: path.join(tmp, 'broken'), out4: path.join(tmp, 'redir'), out5: path.join(tmp, 'excl'), out6: path.join(tmp, 'only') };
    fs.mkdirSync(ctx.proj, { recursive: true });
    fs.mkdirSync(ctx.out, { recursive: true });
    fs.mkdirSync(ctx.out2, { recursive: true });
    fs.mkdirSync(ctx.out3, { recursive: true });
    fs.mkdirSync(ctx.out4, { recursive: true });
    fs.mkdirSync(ctx.out5, { recursive: true });
    fs.mkdirSync(ctx.out6, { recursive: true });
    probeRun = probe(ctx);
    if (!probeRun.ok || !field(probeRun.stdout, 'PROBE_DONE')) {
      throw new Error('pwsh 探针未跑通（不 skip）：' + probeRun.stderr.slice(0, 400) + ' | ' + probeRun.stdout.slice(0, 400));
    }
  });

  afterAll(() => {
    if (ctx) { try { fs.rmSync(ctx.tmp, { recursive: true, force: true }); } catch { /* 清理失败不判红 */ } }
  });

  test('FR1: 真实 pages.json ⇒ 拍平后一页一名；末段命名会吃掉页（⑥ 的运行期依据）', () => {
    const planned = Number(field(probeRun.stdout, 'FR1_PLANNED'));
    const flat = Number(field(probeRun.stdout, 'FR1_FLAT'));
    const leaf = Number(field(probeRun.stdout, 'FR1_LEAF'));
    expect(planned).toBeGreaterThan(0);
    expect(flat).toBe(planned);
    // 把坑本身钉住：末段唯一数严格少于页数。哪天 pages.json 改到末段不再重名，这条会红，
    // 提醒人那是**侥幸**而不是「末段命名安全了」（写死 60/59 会漂，所以判相对关系不判常数）。
    expect(leaf).toBeLessThan(planned);
  });

  test('FR2: subPackages 的 path 相对各自 root ⇒ 清单拼成 root/path', () => {
    expect(Number(field(probeRun.stdout, 'FR2_COUNT'))).toBe(2);
    expect(field(probeRun.stdout, 'FR2_JOINED')).toBe('True');
  });

  test('FR3: 票号非纯数字 ⇒ 当场红且不建任何目录', () => {
    expect(field(probeRun.stdout, 'FR3_OK')).toBe('False');
    expect(field(probeRun.stdout, 'FR3_NODIR')).toBe('True');
  });

  test('FR4: -FullRepo 与 -Pages 同给 ⇒ 当场红、同样不建目录', () => {
    expect(field(probeRun.stdout, 'FR4_OK')).toBe('False');
    expect(field(probeRun.stdout, 'FR4_NODIR')).toBe('True');
  });

  test('FR6: 起跑档只裁本轮，全仓清单原样不动（⑥「先一页起跑通」的入口）', () => {
    const planned = Number(field(probeRun.stdout, 'FR1_PLANNED'));
    expect(Number(field(probeRun.stdout, 'FR6_ONE'))).toBe(1);
    expect(field(probeRun.stdout, 'FR6_FIRST')).toBe('pages/index/index');
    // RampPages=0 与超出总数都不裁（0 是默认值，不是「跑 0 页」）
    expect(Number(field(probeRun.stdout, 'FR6_NONE'))).toBe(planned);
    expect(Number(field(probeRun.stdout, 'FR6_OVER'))).toBe(planned);
  });

  test('FR7: 起跑一轮后，本轮 PASS 与「全仓差 59 页」必须同时可见', () => {
    const planned = Number(field(probeRun.stdout, 'FR1_PLANNED'));
    expect(Number(field(probeRun.stdout, 'FR7_PLANNED'))).toBe(planned);   // 全仓分母没被裁小
    expect(Number(field(probeRun.stdout, 'FR7_ATTEMPTED'))).toBe(1);
    expect(field(probeRun.stdout, 'FR7_RAMP')).toBe('True');
    // 本轮该产的 1 页产出了 => 本轮相符；但全仓远未齐 => 目录对账必为 False
    expect(field(probeRun.stdout, 'FR7_THISRUN_MATCH')).toBe('True');
    expect(field(probeRun.stdout, 'FR7_REPO_MATCH')).toBe('False');
    expect(Number(field(probeRun.stdout, 'FR7_REPO_MISSING'))).toBe(planned - 1);
  });

  test('FR8: 断路腿（设备不可用）报得出 Verdict，且零产物不冒充绿', () => {
    // ⑥ 断路腿：「拔设备 ⇒ 明报、已产页不丢、零产物不冒充绿」。
    // 这里用**不存在的序列号**模拟断连（同一支代码路径：Test-ScreenAwake 拿不到设备状态即 fail-closed），
    // 不要求人肉拔线也能把判据钉住；真机拔线的形态由发版冒烟一次性覆盖。
    // 本用例只收「两支都成立」的事实：分母与分批评另见 FR1/FR6/FR7/FR10/FR11（纯函数）与 FR8N（无 adb 支）。
    expect(field(probeRun.stdout, 'FR8_OK')).toBe('False');
    expect(field(probeRun.stdout, 'FR8_VERDICT')).toBe('FAIL');      // 零产物 ⇒ FAIL，不是 INCOMPLETE 也不是绿
    expect(Number(field(probeRun.stdout, 'FR8_SHOTS'))).toBe(0);
    expect(Number(field(probeRun.stdout, 'FR8_PNG'))).toBe(0);       // 没产图就是没产图
    // 「不冒充绿」的机器形态：真落了 manifest，它也不得自称已对齐（CountsMatch / DirCountsMatch 任一为真即冒充）
    expect(field(probeRun.stdout, 'FR8_MF_GREEN')).toBe('False');
  });

  test('FR8N: 无 adb 那一支——早退也自报 FAIL，且不谎报 manifest 落点', () => {
    // 现场：CI 是 ubuntu、没有 adb ⇒ 探针走「找不到 adb.exe」那一支；本机有 adb ⇒ 那一支只在上游显形。
    // 探针把 Resolve-AdbExeLocal stub 成 $null，让**每台机器**都真跑它 —— 这正是「本地永远绿、CI 永远红」
    // 那条验尸结论的机器形态（#2078 CI 三次连红）。
    expect(field(probeRun.stdout, 'FR8N_ERR_ADB')).toBe('True');     // 自检：stub 真生效，否则本组与 FR8 同支
    expect(field(probeRun.stdout, 'FR8N_OK')).toBe('False');
    expect(field(probeRun.stdout, 'FR8N_VERDICT')).toBe('FAIL');
    expect(Number(field(probeRun.stdout, 'FR8N_SHOTS'))).toBe(0);
    expect(Number(field(probeRun.stdout, 'FR8N_PNG'))).toBe(0);
    // 环境缺失发生在**计划之前** ⇒ 分母如实为空（不是 60，也不假装算过）
    expect(Number(field(probeRun.stdout, 'FR8N_PLANNED'))).toBe(0);
    expect(Number(field(probeRun.stdout, 'FR8N_ATTEMPTED'))).toBe(0);
    // 没写过 manifest 就不许回带路径：假路径让 Test-Path 恒假、让 #2077 续跑读空；
    // 而「在这里顺手写一份空计划 manifest」会得到 CountsMatch=True 的假绿（空计划恒判相符）。
    expect(field(probeRun.stdout, 'FR8N_MF_PATH')).toBe('');
    expect(field(probeRun.stdout, 'FR8N_MF_EXISTS')).toBe('False');
  });

  test('FR11: 只选前缀——招聘方 9 页可单独成批，全仓计划仍是 60', () => {
    const planned = Number(field(probeRun.stdout, 'FR1_PLANNED'));
    expect(Number(field(probeRun.stdout, 'FR11_PLANNED'))).toBe(planned);
    expect(Number(field(probeRun.stdout, 'FR11_ATTEMPTED'))).toBe(9);
    expect(Number(field(probeRun.stdout, 'FR11_OFFPREFIX'))).toBe(0);
    // 只选不是「顺手排除」：没给 -ExcludePrefix ⇒ 被排除名单必须为空
    expect(Number(field(probeRun.stdout, 'FR11_EXCLUDED'))).toBe(0);
  });

  test('FR10: 按前缀分批——排除只裁本轮，全仓计划与被排除名单都可见', () => {
    const planned = Number(field(probeRun.stdout, 'FR1_PLANNED'));
    // 真读数：60 页里 9 页是招聘者侧 ⇒ 排除后本轮 51 页，全仓计划仍是 60
    expect(Number(field(probeRun.stdout, 'FR10_PLANNED'))).toBe(planned);
    expect(Number(field(probeRun.stdout, 'FR10_ATTEMPTED'))).toBe(planned - 9);
    expect(Number(field(probeRun.stdout, 'FR10_EXCLUDED'))).toBe(9);
    // 被排除的页必须全部命中该前缀（别把无关页顺手排掉）
    expect(Number(field(probeRun.stdout, 'FR10_EXCLUDED_OFFPREFIX'))).toBe(0);
    // 「只裁本轮」的分区证据：本轮 + 被排除 == 全仓 ⇒ 排除没让任何页凭空消失
    expect(field(probeRun.stdout, 'FR10_PARTITION')).toBe('True');
    // 先 Only 再 Exclude 的叠加语义（分角色三批口径靠它确定）
    expect(Number(field(probeRun.stdout, 'FR10_COMBO_ATTEMPTED'))).toBe(planned - 9);
    expect(Number(field(probeRun.stdout, 'FR10_COMBO_EXCLUDED'))).toBe(9);
    // 不带 -FullRepo 时单独给 -ExcludePrefix ⇒ 拒
    expect(field(probeRun.stdout, 'FR10_GATE_OK')).toBe('False');
  });

  test('FR9: 中转页算「已交代」但不产图（白名单真读仓内数据文件）', () => {
    // 白名单本身：仓内 json 里 index 的允许落点是 dashboard（这条不写死在代码里，S33 钉住）
    expect(field(probeRun.stdout, 'FR9_HAS_INDEX')).toBe('True');
    expect(field(probeRun.stdout, 'FR9_LANDING')).toBe('pages/dashboard/dashboard');
    // 计数：中转页不算缺失 ⇒ 本轮对账可为真（否则 60/60 永远不可达）
    expect(Number(field(probeRun.stdout, 'FR9_MISSING'))).toBe(0);
    expect(field(probeRun.stdout, 'FR9_MATCH')).toBe('True');
    // manifest 里中转页是**独立字段**，不与 Captured 混为一谈
    expect(Number(field(probeRun.stdout, 'FR9_MF_REDIR'))).toBe(1);
    expect(Number(field(probeRun.stdout, 'FR9_MF_REDIRCOUNT'))).toBe(1);
    // 对照：不传中转记录时，同一页必须算缺失 —— 证明「算已交代」是白名单的效果，不是默认放宽
    expect(Number(field(probeRun.stdout, 'FR9_NO_REDIR_MISSING'))).toBe(1);
  });

  test('FR5: manifest 真落盘真读回，本轮对账与目录累计对账是两个结论', () => {
    expect(field(probeRun.stdout, 'FR5_WRITE')).toBe('True');
    expect(field(probeRun.stdout, 'FR5_SCHEMA')).toBe('fullrepo-manifest/1');
    expect(field(probeRun.stdout, 'FR5_ROUNDTRIP')).toBe('True');
    // 计划 3、本轮截 1 ⇒ 本轮不相符、Missing 点名 2 页（不得假装成 0）
    expect(field(probeRun.stdout, 'FR5_THISRUN_MATCH')).toBe('False');
    expect(Number(field(probeRun.stdout, 'FR5_THISRUN_MISSING'))).toBe(2);
    // 目录累计也只有 1 张 ⇒ DirMissing 同样 2 页
    expect(field(probeRun.stdout, 'FR5_DIR_MATCH')).toBe('False');
    expect(Number(field(probeRun.stdout, 'FR5_DIR_MISSING'))).toBe(2);
  });
});
