/**
 * 自动截图模块契约守护（scripts/lib/auto-screenshot.ps1）
 *
 * 背景（2026-09-14）：
 *   · 第一版只截图不翻页（已由 #989 修：改用 `--pagePath` + SHA256 反假绿）。
 *   · 第二版仍**遍历 pages.json 全部页面**（本仓 50 页），而每页一次 `cli launch --pagePath` 要过一遍
 *     HBuilderX（编译 + 推送）⇒ 全量截图的成本不可接受。`device-capture.ps1` 早已把「逐页 launch 成本高」
 *     标为待裁定，本次裁定：**只截本次改动涉及的页面**（用户 2026-09-14 Q3）。
 *
 * 守护的不变量：
 *   S1  Invoke-AutoScreenshot 存在
 *   S2  返回对象含 Ok / Screenshots / Skipped / HashConflicts / Error
 *   S3  读取 pages.json 取页面清单
 *   S4  跳过的页面记在 Skipped
 *   S5  【文本层】用 HBuilderX CLI --pagePath 导航（不得回退到 am start 深链 —— 实测不生效）
 *   S6  用 SHA256 hash 做反假绿判据（连续相同 hash ⇒ 切页未生效）
 *   S7  -CliPath 参数存在
 *   S8  【Q3】-Pages 参数：允许显式指定要截的页面
 *   S9  【Q3】-ChangedOnly：从 git diff 推导改动页面
 *   S10 【Q3+结构层】推导出 0 页 ⇒ **明报「无改动页面」并返回**，绝不回退到全量截图
 *   S11 【Q3】-MaxPages 上限兜底
 *   S12 【Q3】遍历的是**目标页集合**，不是 pages.json 的全量清单
 *   S13 【2026-09-15】**陈旧截图判据**：文件时间戳必须晚于本次运行起点
 *       （否则上次运行的同名残留 PNG 会被当成本次证据）
 *   S14 【2026-09-15】**改动页推导的路径形状**：两条 git 调用都必须 `--relative`，输出必须过
 *       `ConvertFrom-GitQuotedPath`（否则仓库根相对 + quotepath 转义 ⇒ `^pages/` 恒失配 ⇒ 推导恒 0 页，
 *       步骤 6 必然报「无改动页面」⇒ HashConflicts / TargetPages 永远取不到真值）
 *   S15 【2026-09-15】**切页派发必须分离**：不得再出现前台同步调用那个永不收口的真运行 cli
 *       （实测 18 分钟挂住），必须走 `UseShellExecute` 新进程树 + 有界落定判据 + fail-closed 出口
 *   S16 【2026-09-15】`Wait-NavSettled` 必须**有界**：同时有「等满 MinSeconds 且连续采样一致 ⇒ 落定」
 *       与「到 TimeoutSeconds ⇒ 未落定」两条出口，并以 `Settled` 布尔回报
 *   S17 【2026-09-15】**落定必须有页身份**：日志里出现**目标页**的页面进入行才算落定；
 *       进的是别的页 ⇒ 未落定且原因点名「请求页 X，实际进入 Y」
 *   S18 【2026-09-15】**全黑帧不是证据**：截图整帧无内容 ⇒ 记 Skipped（灭屏时 screencap 只给黑帧）
 *   S19 【2026-09-15】**灭屏前置**：`mWakefulness` 非 Awake ⇒ 直接拒绝截图（fail-closed，不注入 input）
 *   S20 【2026-09-15】**包装脚本必须钉 UTF-8 解码**：否则 CLI 的 UTF-8 输出被按 OEM 码页解码后再写盘
 *       ⇒ 页面进入行变双重编码乱码 ⇒ S17 的页身份判据永远匹配不到（本地假 cli 已实证）
 *   S21 【2026-09-15，#1027】**adb 解析不得再有第二份**：必须复用 `lib/env-check.ps1` 的
 *       `Resolve-AdbExeLocal`（代码里不得再出现候选集原文；独立调用时懒加载补唯一真源）。
 *       两份漂移的后果实测：步骤 2/5 能过而步骤 6 恒报「找不到 adb.exe」，步骤 7–9 永远到不了。
 *       行为面的一致性（结论必须相同）由 `autoScreenshotAdbAgreementBehavior.test.js`（A1–A3）另钉。
 *   S22 【2026-09-15，#1027 收尾真机实测】**「画面稳定」不得用全帧 hash 相等**：系统状态栏的实时读数
 *       （MIUI 实时网速）会让整帧 hash 恒不同 ⇒ 判据永远不可能满足、步骤 7–9 永远跑不到；
 *       必须走 `Compare-ScreenFrames` 的宽容比较（差异像素占比 ≤ `-StableMaxDiffPercent`，默认 0.5%）。
 *       宽容比较本身的语义由 `autoScreenshotStabilityBehavior.test.js`（B1–B4）在运行期另钉。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/auto-screenshot.ps1';

function readSource() {
  return readText(path.join(ROOT, SCRIPT_REL));
}

describe('auto-screenshot.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('S1: Invoke-AutoScreenshot function exists', () => {
    expect(src).toContain('function Invoke-AutoScreenshot');
  });

  test('S2: returns Ok/Screenshots/Skipped/HashConflicts/StaleShots/Error', () => {
    ['Ok', 'Screenshots', 'Skipped', 'HashConflicts', 'StaleShots', 'Error'].forEach((f) => {
      expect(src).toContain(f);
    });
  });

  test('S3: reads pages.json for the page list', () => {
    expect(src).toContain('pages.json');
  });

  test('S4: skipped pages tracked', () => {
    expect(src).toContain('skipped');
  });

  test('S5: 【文本层】navigates via HBuilderX CLI --pagePath (not am start deep link)', () => {
    expect(src).toContain('--pagePath');
    // 2026-09-15：导航改**分离派发**（Start-NavLaunchDetached）后，正向判据**不能再吃注释里的 `& $CliPath`**
    //   （旧写法只会被自己文档注释满足 —— 假通过）。改成钉两件事：
    //     ① 派发调用真的把 `$CliPath` 传进去；② 仍不允许退回 `am start` 深链（实测不生效，见 S5 原意）。
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');
    expect(code).toMatch(/Start-NavLaunchDetached[^\r\n]*-CliExe\s+\$CliPath/);
    expect(code).not.toMatch(/am\s+start/);
  });

  test('S6: SHA256 anti-false-green (identical consecutive hashes ⇒ navigation failed)', () => {
    expect(src).toContain('SHA256');
    expect(src).toContain('seenHashes');
  });

  test('S7: -CliPath parameter exists', () => {
    expect(src).toContain('$CliPath');
  });

  test('S8: 【Q3】-Pages parameter (explicit page list)', () => {
    expect(src).toContain('$Pages');
    expect(src).toMatch(/Pages\s*-split|\[string\]\$Pages/);
  });

  test('S9: 【Q3】-ChangedOnly derives changed pages from git diff', () => {
    expect(src).toContain('ChangedOnly');
    expect(src).toMatch(/git[^\r\n]*diff/);
    expect(src).toMatch(/--name-only/);
  });

  test('S10: 【结构层】0 derived pages ⇒ explicitly reports and returns (never falls back to all)', () => {
    // 注意：该短语在**文档注释里也会出现**，故不能只看第一处 —— 必须检查「有一处在代码里紧邻 return」。
    const hits = [...src.matchAll(/无改动页面/g)].map((m) => m.index);
    expect(hits.length).toBeGreaterThan(0);
    const hasReturnNear = hits.some((i) =>
      /return\s+\[pscustomobject\]/.test(src.slice(Math.max(0, i - 500), i + 500))
    );
    expect(hasReturnNear).toBe(true);
  });

  test('S11: 【Q3】-MaxPages cap', () => {
    expect(src).toContain('MaxPages');
  });

  test('S12: 【Q3】iterates the target page set, not the full pages.json list', () => {
    expect(src).toContain('$targetPages');
    expect(src).toMatch(/foreach\s*\(\s*\$page\s+in\s+\$targetPages\s*\)/);
    // 不得再直接遍历全量 $pages
    expect(src).not.toMatch(/foreach\s*\(\s*\$page\s+in\s+\$pages\s*\)/);
  });

  // S13（2026-09-15）：截图**陈旧文件**判据。`$OutputDir` 不清理、文件名按页名固定
  // ⇒ adb 静默失败时上一次运行的同名残留 PNG 会让「存在且非空」（S4 那条）照样通过，
  //    把陈旧截图当本次证据。必须有一条「文件时间戳晚于本次运行起点」的判据。
  test('S13: rejects stale screenshots (file mtime must be newer than this run)', () => {
    // 必须有运行起点，且**在循环之前**取（循环内各取一次会让判据退化成恒真）
    const runStartAt = src.indexOf('$runStarted');
    const loopAt = src.indexOf('foreach ($page in $targetPages)');
    expect(runStartAt).toBeGreaterThan(-1);
    expect(loopAt).toBeGreaterThan(-1);
    expect(runStartAt).toBeLessThan(loopAt);
    // 必须有「时间戳早于起点 ⇒ 判陈旧」的比较与归类
    expect(src).toMatch(/\$shotWritten\s+-lt\s+\$runStarted/);
    expect(src).toMatch(/\$staleShots\s*\+=/);
    // 陈旧截图必须计入 Skipped（否则不会让 Ok=false ⇒ 仍会假绿）
    const staleAt = src.indexOf('$staleShots += $pageName');
    expect(staleAt).toBeGreaterThan(-1);
    expect(src.slice(staleAt, staleAt + 300)).toMatch(/\$skipped\s*\+=/);
  });

  // S14（2026-09-15）：S9 只断言「源码里出现了 git … diff」（文本层），**推导坏掉它照样绿**。
  //   实际症状：`git diff --name-only` 在本仓输出的是**仓库根相对**路径（本项目位于
  //   training-app/<中文目录>/ 之下），且非 ASCII 段被 core.quotepath 转义成引号 + \ooo；
  //   而下面的映射按 `^pages/` 锚定 ⇒ 一条也匹配不上 ⇒ 恒推导出 0 页 ⇒ dev:finish 步骤 6
  //   必然报「无改动页面」⇒ HashConflicts / TargetPages 永远取不到真值（长跑不出来的验证债）。
  //   本条是**回归钉**，钉死「必须 --relative + 必须解码 + 解码器来自唯一真源」三点，旧代码必红。
  test('S14: 【2026-09-15】derivation uses --relative and decodes git-quoted paths', () => {
    const from = src.indexOf('确定目标页集合');
    // ⚠️ 右边界必须**从 from 之后再找**：文件头说明里就有「…本仓 `pages.json` 有 50 页 ⇒…」，
    //    直接 indexOf 那个词会命中它 ⇒ 切片为空 ⇒ 整条守护退化成恒真的空断言（正是要防的弱守护）。
    const to = src.indexOf('3) 0 页', from);
    expect(from).toBeGreaterThan(-1);
    expect(to).toBeGreaterThan(from);
    const block = src.slice(from, to);

    // 反例：不得再出现**不带走相对**的那两种调用形态
    expect(block).not.toMatch(/'--name-only',\s*'HEAD'/);
    expect(block).not.toMatch(/'--name-only',\s*'origin\/master\.\.\.HEAD'/);
    expect(block).not.toMatch(/'--name-only',\s*'--cached'/);

    // 正例：两条调用都带 --relative（锚定 '^pages/' 只有在路径相对项目根时才可能命中）
    expect(block).toMatch(/'--name-only',\s*'--relative',\s*'HEAD'/);
    expect(block).toMatch(/'--name-only',\s*'--relative',\s*'origin\/master\.\.\.HEAD'/);

    // 正例：输出必须过解码器，且解码器来自唯一真源（不在此另抄一份实现）
    expect(block).toMatch(/ConvertFrom-GitQuotedPath/);
    expect(block).toMatch(/level-detect\.ps1/);
  });

  // S15（2026-09-15）：切页派发**必须分离**。
  //   症状：先前是 `& $CliPath @launchArgs 2>&1 | Out-String` —— 前台同步等待一个**永不自己收口**的
  //   真运行会话（实测存活 18 分钟、CPU 0.08 秒）⇒ 步骤 6 永久挂住，截图不落盘，
  //   HashConflicts / TargetPages 永远算不出来。
  //   ⚠️ 反向断言必须**先剥注释**：脚本与本文件的注释里都会引用旧形态（本仓血账：注释会命中断言）。
  test('S15: 【2026-09-15】navigation dispatch is detached (no foreground sync call)', () => {
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');
    expect(code).not.toMatch(/&\s*\$CliPath\s*@launchArgs/);
    expect(code).not.toMatch(/\$CliPath\s*@launchArgs[^\r\n]*\|\s*Out-String/);

    // 必须走「新进程树」派发（与 hx-run.ps1 同一套）
    expect(code).toMatch(/Start-NavLaunchDetached/);
    expect(src).toContain('UseShellExecute');

    // 必须有有界的落定判据，且未落定时 fail-closed（记 Skipped 且不产截图）
    expect(src).toMatch(/Wait-NavSettled/);
    expect(src).toMatch(/NavigateTimeoutSeconds/);
    const navAt = src.indexOf('if (-not $settle.Settled)');
    expect(navAt).toBeGreaterThan(-1);
    const navBlock = src.slice(navAt, navAt + 400);
    expect(navBlock).toMatch(/\$skipped\s*\+=/);
    expect(navBlock).toMatch(/continue/);
  });

  // S16（2026-09-15）：落定判据必须**有界**，且有明确的布尔回报（否则调用方又会退化成「无限等」）。
  test('S16: 【2026-09-15】Wait-NavSettled is bounded and reports a Settled flag', () => {
    const fnAt = src.indexOf('function Wait-NavSettled');
    expect(fnAt).toBeGreaterThan(-1);
    expect(src).toMatch(/Settled\s*=\s*\$true/);
    expect(src).toMatch(/Settled\s*=\s*\$false/);
    // 两条出口：等满 MinSeconds 且连续采样一致 / 到 TimeoutSeconds
    expect(src).toMatch(/\$elapsed\s*-ge\s*\$MinSeconds/);
    expect(src).toMatch(/\$elapsed\s*-ge\s*\$TimeoutSeconds/);
    expect(src).toMatch(/\$stable\s*-ge\s*1/);
  });

  // S17（2026-09-15）：**页身份**判据 —— ADR-0008 记的那条缺口（「四项断言覆盖不到『目标页是否真的加载』」）
  //   在 dev:finish 载体上的落锁。旧判据只要求「画面稳定」，已被全黑帧骗过一次；现在必须由**应用自己
  //   打的页面进入行**证明进的是目标页，进错了要在原因里点名。
  test('S17: 【2026-09-15】settle requires the app-side page-identity line for the TARGET page', () => {
    expect(src).toContain('function Get-NavEnteredPage');
    // 页面进入行标记（中文；脚本文件是 UTF-8 ⇒ 字面量可直接写）
    expect(src).toMatch(/进入页面/);
    expect(src).toMatch(/\$entered\s*-eq\s*\$ExpectedPage/);
    expect(src).toMatch(/EnteredPage\s*=/);
    // 未落定的原因必须能点名「请求页 X，实际进入 Y」
    expect(src).toMatch(/请求页 \$ExpectedPage，日志里最后进入的是 \$entered/);
    // 且该判据必须真的被调用方接上（参数名不能只写在函数签名里）
    expect(src).toMatch(/-NavOutFile\s+\$nav\.OutFile\s+-ExpectedPage\s+\$page/);
  });

  // S18（2026-09-15）：全黑帧**不是证据**。实测 17208 字节的全黑 PNG 曾一路通过
  //   「存在且非空 + 时间戳新 + hash 不变」三条判据 ⇒ 必须有一条「整帧无内容」的判据把它挡掉。
  test('S18: 【2026-09-15】all-black frames are rejected as evidence', () => {
    expect(src).toContain('function Test-ScreenBlank');
    const at = src.indexOf('$blankInfo = Test-ScreenBlank -Path $outputFile');
    expect(at).toBeGreaterThan(-1);
    const block = src.slice(at, at + 400);
    expect(block).toMatch(/\$blankInfo\.Blank/);
    expect(block).toMatch(/\$skipped\s*\+=/);
    expect(block).toMatch(/continue/);
  });

  // S19（2026-09-15）：灭屏前置 —— 灭屏时 screencap 只给全黑帧（且天然「稳定」）⇒ 必须在循环**之前**
  //   fail-closed 拒绝，并写明「唤醒设备是人来做」（脚本不注入 input）。
  test('S19: 【2026-09-15】screen must be awake before screenshotting (fail-closed, no input injection)', () => {
    expect(src).toContain('function Test-ScreenAwake');
    expect(src).toMatch(/mWakefulness/);
    expect(src).toMatch(/设备屏幕未唤醒/);
    const at = src.indexOf('$awake = Test-ScreenAwake');
    expect(at).toBeGreaterThan(-1);
    expect(src.slice(at, at + 700)).toMatch(/return\s+\[pscustomobject\]/);
    // 不得用 input 注入去「自动唤醒设备」
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');
    expect(code).not.toMatch(/keyevent\s+KEYCODE_WAKEUP/i);
  });

  // S20（2026-09-15）：包装脚本必须**钉死 UTF-8 解码**。
  //   实测（本机假 cli）：不钉的话，DCloud CLI 的 UTF-8 输出被 pwsh 按 OEM 码页(GBK)解码、再以 UTF-8
  //   写盘 ⇒ 双重编码乱码，`进入页面:` 永远匹配不到 ⇒ S17 的页身份判据恒不可用。
  test('S20: 【2026-09-15】nav wrapper pins UTF-8 output decoding', () => {
    expect(src).toMatch(/\[Console\]::OutputEncoding\s*=\s*\[System\.Text\.Encoding\]::UTF8/);
    const at = src.indexOf('$wrapLines = @(');
    expect(at).toBeGreaterThan(-1);
    const block = src.slice(at, at + 800);
    expect(block).toMatch(/\[Console\]::OutputEncoding/);
    expect(block).toMatch(/\*>\s*'\$outFile'/);
  });

  // S21（2026-09-15，#1027）：adb 解析**不得再有第二份**。
  //   症状：本文件曾内联自己的候选集（只查 `$env:ANDROID_SDK_ROOT` / `$env:ANDROID_HOME` / `Get-Command adb`），
  //   比 `env-check.ps1` 的 `Resolve-AdbExeLocal` 少了 `D:\android-sdk\platform-tools\adb.exe` 兜底 ⇒
  //   本机（两个 env 未设、adb 不在 PATH、adb 在 D 盘）步骤 2/5 能过、**步骤 6 恒报「找不到 adb.exe」**，
  //   而步骤 4/5 已真跑完并产生副作用（编译 + 部署），步骤 7–9 永远到不了 ——
  //   这正是 ADR-0008「门脚本共享载体」记的「解析器被复制而不是复用」那类分叉。
  //   ⚠️ 断言前**必须剥注释**：本文件、脚本头与调用点的注释里都会引用旧候选集原文（本仓血账：注释会命中断言）。
  test('S21: 【2026-09-15，#1027】adb resolution delegates to the single source (no second candidate list)', () => {
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');

    // 反例：代码里不得再出现第二份候选集
    expect(code).not.toMatch(/ANDROID_SDK_ROOT|ANDROID_HOME/);
    expect(code).not.toMatch(/platform-tools/);
    expect(code).not.toMatch(/Get-Command\s+adb/);

    // 正例：复用唯一真源 `env-check.ps1` 的解析器，且**懒加载**（本模块被单独调用时也能补上）
    expect(code).toMatch(/\.\s*\(Join-Path\s+\$PSScriptRoot\s+'env-check\.ps1'\)/);
    expect(code).toMatch(/Get-Command\s+Resolve-AdbExeLocal\s+-ErrorAction\s+SilentlyContinue/);

    // 解析不到时仍必须 fail-closed 返回（不得静默继续跑到后面拿 $null 当路径用）
    const assignAt = code.indexOf('$adbExe = Resolve-AdbExeLocal');
    expect(assignAt).toBeGreaterThan(-1);
    const after = code.slice(assignAt, assignAt + 400);
    expect(after).toMatch(/if\s*\(-not\s+\$adbExe\)\s*\{/);
    expect(after).toMatch(/找不到 adb\.exe/);
    expect(after).toMatch(/return\s+\[pscustomobject\]/);
  });

  // S22（2026-09-15，#1027 收尾**真机实测**）：「画面稳定」**不得**用全帧 hash 相等。
  //   症状：系统状态栏里有**应用控制不了**的实时读数（MIUI「显示实时网速」的 KB/s，每 1–2 秒就变）⇒
  //   连拍三帧的整帧 sha256 **两两不同**（实测差异**只在顶部 0–99px 状态栏**、MaxDiff 188，
  //   其余整幅逐字节相同）⇒ 「连续两次采样一致」在该设备上**永远不可能满足** ⇒ 步骤 6 必然 420 秒超时、
  //   步骤 7–9 永远跑不到（而 app 画面早就落定了）。
  //   判据：必须走 `Compare-ScreenFrames` 的**宽容**比较（网格差异像素占比 ≤ 阈值），旧 hash 写法不得回写。
  //   实测标定（同一台设备，1080×2400）：状态栏量级的 churn 在 24/48/96 网格上 = 0%、192 网格 = 0.011%；
  //   真切页（对照全黑帧）= ~100% ⇒ 默认阈值 0.5% 卡在噪声上方 ~45×、真变化下方 ~200×。
  test('S22: 【2026-09-15，#1027】screen stability uses tolerant frame comparison, not full-frame hash equality', () => {
    expect(src).toContain('function Compare-ScreenFrames');

    // 反例（旧写法）：不得再出现「取探针帧的全帧 hash 再与上一帧比相等」这三行
    expect(src).not.toMatch(/\$h\s*=\s*\(Get-FileHash[^\r\n]*ProbeFile/);
    expect(src).not.toMatch(/\$h\s*-eq\s*\$prev/);
    expect(src).not.toMatch(/\$prev\s*=\s*\$h/);

    // 正例：落定循环里必须真的调用宽容比较，并把上一帧留在盘上（否则没有可比对象）
    expect(src).toMatch(/Compare-ScreenFrames\s+-Path\s+\$ProbeFile\s+-PrevPath\s+\$prevFile/);
    expect(src).toMatch(/nav-probe-prev\.png/);

    // 阈值必须是**可调参数 + 有默认值**（不许写死在函数体里，也不许悄悄调成 0）
    expect(src).toMatch(/\$StableMaxDiffPercent\s*=\s*0\.5/);
    expect(src).toMatch(/-StableMaxDiffPercent\s+\$StableMaxDiffPercent/);

    // 「连续两次采样一致」的语义没变（仍是 stable ≥ 1），fail-closed 出口照旧
    expect(src).toMatch(/\$stable\s*-ge\s*1/);
    expect(src).toMatch(/Settled\s*=\s*\$false/);
  });
});
