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
 *   S23 【2026-10-06，#1560 票面现测】**每次 adb 调用必须有单次超时**：S16 那条 420 秒上限原先**不可达** ——
 *       每轮的 `& cmd.exe /c "adb … > probe.png"` 没有单次超时，一次不返回就永远回不到 `$elapsed -ge $TimeoutSeconds`
 *       （现测：32 分钟 `.ci-verify` 零写入，而同时刻手工 `screencap` 4.8 秒返回 ⇒ 卡点不是 adb 坏，是调用没被约束）。
 *       必须走 `Invoke-BoundedAdbShot`（文件重定向 + `WaitForExit(ms)` + `Kill($true)`），且**两个调用点都覆盖**
 *       （导航采样 + 页面截图；只修前者等于把挂死从「等落定」挪到「等截图」）。
 *       配套三条不变式：超时轮**逻辑上不计入采样**（`-not $shot.TimedOut` 先看轮次再看文件——真链路现测证明
 *       残帧**可能删不掉**，删除只是卫生、不能当判据）、页面截图先落 `.part` 成功才归位（半张图不得进步骤 7
 *       按 `*.png` 扫的证据目录）、`Settled` 两个出口都带 `CallTimeouts` 计数。
 *       行为面（真挂死桩 / 影子夹具 / 预算内收口）由 `autoScreenshotStabilityBehavior.test.js`（B6–B8）另钉。
 *   S24 【2026-10-07，#1562 票面现测】**第三处无界 adb 调用（灭屏前置）也已收口，且「一次调用」只有一份实现**：
 *       `Test-ScreenAwake` 原先是 `& $AdbExe -s $Serial shell dumpsys power | Out-String` —— 前台同步等待、
 *       没有单次超时，而它在**每页之前**跑 ⇒ 挂住的时机比 S23 那两处**更早**，卡的是「能不能开始截屏」。
 *       现在它走 `Invoke-BoundedAdbText`（预算从 `-AdbCallTimeoutSeconds` 透传），超时给的是**既有的那条
 *       fail-closed 出口**（`Ok=false` + `state=unknown`：拿不到唤醒状态 ⇒ 不截），不开第三种后果。
 *       另钉两件事：① 等待+杀树+残帧作废**只许一份**（`Invoke-BoundedAdbCall` 一个实现核，Shot/Text 两个
 *       薄封装都委托它）—— 复制而不是复用正是 ADR-0008「adb 解析的唯一真源」记的那类分叉；
 *       ② `AWAKE_PROBE` 机检行三条出口都点名单次预算。
 *       行为面（真桩在预算内给结论 / 正常返回不被误判 / 半份 stdout 不留盘）由
 *       `autoScreenshotStabilityBehavior.test.js`（B9–B11）在运行期另钉。
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

  // S23（2026-10-06，#1560 **票面现测**）：每次 adb 调用必须有**单次超时**，且**两个调用点**都要覆盖。
  //   症状：S16 那条 420 秒上限**不可达** —— `$elapsed -ge $TimeoutSeconds` 只在两轮之间检查一次，
  //   而真正可能不返回的是那一轮的 `& cmd.exe /c "adb … > probe.png"`（无单次超时）⇒ 一次挂住就永远
  //   回不到上限：既不落定、也不按设计记 Skipped，整条链就地停住（现测 32 分钟 `.ci-verify` 零写入，
  //   而**同一时刻**手工 `adb exec-out screencap` 4.8 秒返回 ⇒ 设备与 adb 都健康，缺的是「约束」）。
  //   判据：走 `Invoke-BoundedAdbShot`（OS 直接把 stdout 写进文件 + `WaitForExit(ms)` + `Kill($true)`）；
  //   不复用 `process-capture.ps1` 的 `Invoke-Process` —— 它把 stdout 按 UTF-8 **读成字符串**会改坏 PNG 字节，
  //   而它「等异步读段排空」那段（#1285）在 adb **常驻 server** 下会造出第二个无界等待点。
  //   行为面（真挂死桩在预算内返回并杀树 / 残帧不在盘上 / 影子夹具驱动循环到点 Skipped）由
  //   `autoScreenshotStabilityBehavior.test.js`（B6–B8）在运行期另钉 —— 这里只钉形状与「不得回写成无界」。
  test('S23: 【2026-10-06，#1560】every adb shot call goes through a bounded single-call executor', () => {
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');

    // ① 有界执行器存在，且两个调用点都走它（定义 1 处 + 调用 ≥2 处）
    expect(code).toContain('function Invoke-BoundedAdbShot');
    expect((code.match(/Invoke-BoundedAdbShot/g) || []).length).toBeGreaterThanOrEqual(3);

    // ② 旧的无界形状**不得**回写：`cmd.exe /c` 直调、以及 cmd 的内层 `>` 重定向（现由 -RedirectStandardOutput 落盘）
    expect(code).not.toMatch(/cmd\.exe\s+\/c/);
    expect(code).not.toMatch(/exec-out screencap -p\s*>/);
    expect(code).toMatch(/-RedirectStandardOutput\s+\$OutFile/);

    // ③ 单次上限必须是**可调参数 + 有默认值**，并从外层透传到两个调用点（不许写死在函数体里）
    expect(code).toMatch(/\[int\]\$CallTimeoutSeconds\s*=\s*15/);
    expect(code).toMatch(/\[int\]\$AdbCallTimeoutSeconds\s*=\s*15/);
    expect(code).toMatch(/-CallTimeoutSeconds\s+\$AdbCallTimeoutSeconds/);
    expect(code).toMatch(/-OutFile\s+\$ProbeFile\s+-TimeoutSeconds\s+\$CallTimeoutSeconds/);
    expect(code).toMatch(/-OutFile\s+\$partFile\s+-TimeoutSeconds\s+\$AdbCallTimeoutSeconds/);

    // ④ 超时轮的不变式：计数、**逻辑上不计入采样**、残帧尽力删（但删除**不是判据**，见下面的 `.part`）
    expect(code).toMatch(/\$callTimeouts\+\+/);
    expect(code).toMatch(/if\s*\(\$shot\.TimedOut\)\s*\{[\s\S]{0,200}?\$callTimeouts\+\+/);
    // 采样判据必须先看「这一轮是不是超时轮」，再看文件在不在 —— 顺序反了就把被杀的半帧当采样
    expect(code).toMatch(/-not\s+\$shot\.TimedOut\s+-and\s+\(Test-Path\s+-LiteralPath\s+\$ProbeFile\)/);
    expect(code).toMatch(/Remove-Item\s+-LiteralPath\s+\$OutFile/);
    expect(code).toMatch(/Samples\s*=\s*\$sampled;\s*CallTimeouts\s*=\s*\$callTimeouts/);

    // ④b 证据目录里半张图不许出现（真链路现测：Kill 之后删除与子进程句柄有竞争，删不掉是真发生过的）
    //     ⇒ 页面截图先落 `.part`，只有「调用返回 + 帧非空」才改名归位
    expect(code).toMatch(/\$partFile\s*=\s*"\$outputFile\.part"/);
    expect(code).toMatch(/Move-Item\s+-LiteralPath\s+\$partFile\s+-Destination\s+\$outputFile\s+-Force/);

    // ⑤ 用时必须在**调用之后**才算，否则「到上限 N 秒」那句文案撒谎（`Seconds` 少算整次调用耗时）
    const shotAt = code.indexOf('Invoke-BoundedAdbShot -AdbExe $AdbExe -Serial $Serial -OutFile $ProbeFile');
    const elapsedAt = code.indexOf('$elapsed = [int]((Get-Date) - $start).TotalSeconds', shotAt);
    expect(shotAt).toBeGreaterThan(-1);
    expect(elapsedAt).toBeGreaterThan(shotAt);

    // ⑥ 页面截图那个调用点的 fail-closed 出口：超时 ⇒ 记 Skipped（调用方语义不变，只多点名）
    expect(code).toMatch(/SHOT_CALL_TIMEOUT/);
    expect(code).toMatch(/if\s*\(\$shot\.TimedOut\)\s*\{[\s\S]{0,300}?\$skipped\s*\+=\s*\$pageName/);

    // ⑦ 机检行：单点格式串 + 两个出口都打；它自我声明为**读数不是判据**（先例 capability-surface.ps1）
    expect(code).toMatch(/\$navLineFmt\s*=\s*'NAV_SAMPLE settled=/);
    expect((code.match(/Write-Host \(\$navLineFmt/g) || []).length).toBe(2);
    expect(src).toMatch(/它\*\*不是判据\*\*/);
  });

  // S24（2026-10-07，#1562 **票面现测**）：第三处无界 adb 调用（灭屏前置）已收口，且「一次调用」只有一份实现。
  //   症状：`Test-ScreenAwake` 是 `& $AdbExe -s $Serial shell dumpsys power | Out-String` —— 前台同步等待、
  //   **没有单次超时**，而它在**每页之前**跑（S19 那条灭屏 fail-closed）⇒ 挂住的时机比 S23 修掉的两处**更早**，
  //   卡的还是「能不能开始截屏」。#1560 收口时现测出这一处，票面刻意没顺手修（扩面会让「只修一处等于把挂死
  //   挪个位置」那条论证失去对照物），另立 #1562。
  //   另钉一件事：等待/杀树/残帧作废**只许一份**（`Invoke-BoundedAdbCall`）—— 三个消费点各抄一份正是
  //   ADR-0008「adb 解析的唯一真源」记的那类「复制而不是复用 ⇒ 两份悄悄漂移」。
  //   行为面由 `autoScreenshotStabilityBehavior.test.js`（B9–B11）另钉 —— 这里只钉形状与「不得回写成无界」。
  test('S24: 【2026-10-07，#1562】the awake pre-check is bounded too, and one bounded-call implementation only', () => {
    const code = src.replace(/<#[\s\S]*?#>/g, '').replace(/^\s*#.*$/gm, '');

    // ① 灭屏前置走有界取文本封装，预算是**参数**而不是写死在函数体里
    const awakeAt = code.indexOf('function Test-ScreenAwake');
    expect(awakeAt).toBeGreaterThan(-1);
    const awakeBody = code.slice(awakeAt, code.indexOf('function Get-NavEnteredPage'));
    expect(awakeBody).toMatch(/Invoke-BoundedAdbText/);
    expect(awakeBody).toMatch(/\[int\]\$TimeoutSeconds\s*=\s*15/);
    expect(awakeBody).toMatch(/-TimeoutSeconds\s+\$TimeoutSeconds/);
    // 超时是一种**结论**（不是 catch 掉就当没发生）
    expect(awakeBody).toMatch(/if\s*\(\$r\.TimedOut\)\s*\{/);
    // 旧形状不得回写：整条 dumpsys 前台同步等待（票面点名的第三处无界调用）
    expect(awakeBody).not.toMatch(/&\s*\$AdbExe\s+-s\s+\$Serial\s+shell\s+dumpsys/);
    expect(awakeBody).not.toMatch(/Out-String/);
    // 超时出口仍是 S19 那条 fail-closed：Ok=false + state=unknown —— 不开「第三种后果」，判定仍只有截/不截
    expect(awakeBody).toMatch(/Ok\s*=\s*\$false;\s*State\s*=\s*'unknown';\s*TimedOut\s*=\s*\$true/);
    // 调用方把单次预算透传给前置检查（缺了它，`-AdbCallTimeoutSeconds` 对这一处不起作用）
    expect(code).toMatch(/\$awake = Test-ScreenAwake[^\n]*-TimeoutSeconds\s+\$AdbCallTimeoutSeconds/);
    // 「通道不返回」与「设备没亮屏」的文案得分开（处置人不同：前者查 adb，后者是人去亮屏）
    expect(code).toMatch(/adb 通道问题，不是没亮屏/);

    // ② 一次 adb 调用只有一份「有界」实现：三段各出现一次，两个封装都委托执行核
    expect((code.match(/WaitForExit\(\$TimeoutSeconds \* 1000\)/g) || []).length).toBe(1);
    expect((code.match(/\.Kill\(\$true\)/g) || []).length).toBe(1);
    expect((code.match(/Start-Process -FilePath 'cmd\.exe'/g) || []).length).toBe(1);
    expect((code.match(/Invoke-BoundedAdbCall -AdbExe/g) || []).length).toBe(2);
    expect(code).toMatch(/function Invoke-BoundedAdbShot[\s\S]{0,600}?Invoke-BoundedAdbCall -AdbExe/);
    expect(code).toMatch(/function Invoke-BoundedAdbText[\s\S]{0,900}?Invoke-BoundedAdbCall -AdbExe/);
    // 二进制通道那条子命令绑在截屏封装上（不靠每个调用方各自记住 PNG 不能经 PowerShell 的 `>`）
    expect(code).toMatch(/-AdbSubCommand 'exec-out screencap -p'/);
    // 取文本也**先落盘再读回**：读段那条挂死路径（#1285）不能因为「这次是文本」就换回 `| Out-String`
    expect(code).toMatch(/function Invoke-BoundedAdbText[\s\S]*?Get-Content -LiteralPath \$outFile/);

    // ②b 读回判据 = `Exited`（进程真的退出），**不是**「没被判超时」（#1562 自审时补的窄缝）：
    //    stdout 按 PID 命名，而 `-RedirectStandardOutput` 只在这次调用起得来时才截断文件 ⇒
    //    起不来那一格（执行核 catch：TimedOut=false + Exited=false）若只判 TimedOut，就会把**上一次调用**
    //    留在同一个路径上的文本读成本次结论。两层各自独立钉住，缺一层就红（行为腿见 B12 / 变异 M6）。
    const textAt2 = code.indexOf('function Invoke-BoundedAdbText');
    expect(textAt2).toBeGreaterThan(-1);
    const textBody = code.slice(textAt2, code.indexOf('function Wait-NavSettled', textAt2));
    expect(textBody).toMatch(/if\s*\(\$call\.Exited\s*-\s*and\s*\(Test-Path -LiteralPath \$outFile\)\)\s*\{/);
    expect(textBody).not.toMatch(/if\s*\(\s*-not\s+\$call\.TimedOut\s+-and\s+\(Test-Path -LiteralPath \$outFile\)\)/);
    // 起调用之前先尽力清旧档（卫生；判据仍是上面那条 Exited —— 顺序钉住，防「清档写在起调用之后」这种空转写法）
    const preDelete = textBody.indexOf('Remove-Item -LiteralPath $outFile');
    const coreCall = textBody.indexOf('Invoke-BoundedAdbCall -AdbExe');
    expect(preDelete).toBeGreaterThan(-1);
    expect(coreCall).toBeGreaterThan(-1);
    expect(preDelete).toBeLessThan(coreCall);

    // ③ 机检行：三条出口（判亮 / 判灭 / 拿不到）都点名单次预算，且自我声明为**读数不是判据**
    expect((code.match(/AWAKE_PROBE ok=/g) || []).length).toBe(3);
    expect(code).toMatch(/callBudgetSeconds=/);
    expect(src).toMatch(/是读数不是判据/);
  });
});
