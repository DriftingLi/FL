/**
 * 真机无线调试入口契约守护（scripts/wireless-debug.ps1，#1564）
 *
 * 背景：这台机器上「无线调试每次要人点开关、端口要读屏抄」的处置此前只活在会话记忆里，而那份记忆是错的
 * （它记着「本机拿不到 mDNS」，而仓内 `docs/adr/0007-渐进式重构手册.md:563-564` 与
 * `docs/verification/forgot-password/1263/README.md:109` 一直用 `adb mdns services` 现测端口）。
 * 入口固化进仓的同时，要把它赖以成立的几条红线钉死。下面每条都对应 2026-10-07 的一次实测或一次踩坑：
 *
 *   W1 只读红线：代码面不得出现 kill-server / install / force-stop / logcat -c / push / reboot /
 *      input tap|swipe|keyevent / screencap。adb server 与 HBuilderX 共享（kill-server 会打断门），
 *      `input` 会抢 ①a 取证的焦点判据（同 scripts/device-capture.ps1 的只读禁令）。
 *   W2 开关只写「开」方向、且只写这一个 key：`settings put global adb_wifi_enabled` 的实参恒为 '1'。
 *      这是「防被关闭」那一手的边界——工具能重声明开关，不能顺手改设备上的其它设置。
 *   W3 循环必睡 + 放弃按时间窗：Do-Keep 两条分支各需一次 Start-Sleep；放弃判据必须比 TotalMinutes。
 *      血账：上一版心跳收了 `-IntervalSeconds 15` 却漏了 sleep，日志看着跑了 25 分钟、实际 68 秒就 GIVEUP；
 *      而「失败次数」当上限是同一个坑的另一半（快速失败的调用会把窗口压成几十秒）。
 *   W4 不得写死宿主树绝对路径：整个文件（含注释）都不得出现 `D:\FL` 或 `.scratch`——这工具要在任意
 *      worktree / 任意检出里用，写死宿主树等于换个树就失效。
 *   W5 机读行只用 ASCII：结论行前缀 WIRELESS_DEBUG 且非 ASCII 过滤 `[^\x20-\x7E]` 在位。
 *      实测：pwsh 子进程 stdout 被上层按 GBK 解码时中文必乱码，拿控制台输出当 grep 目标会判错。
 *   W6 进程归属判据不得写死脚本名：必须取 `$script:ScriptFile` 的 basename。
 *      血账（本次迁移实测）：改名成 wireless-debug.ps1 后写死的旧名再也不命中 ⇒
 *      watch 恒报 spawn_unconfirmed、stop 恒报 not_running，而 keep 进程还在后台跑（留孤儿）。
 *   W7 非门：头部文档必须自称「非门（不替代 ① 验收门）」，且**代码面**不得出现 gate-evidence:
 *      （文档块里那句「不得出现 gate-evidence:」是合法的禁令说明——判据只看代码面，
 *      口径同 utils/deviceCaptureContract.test.js 的 D3 与 utils/emulatorSmokeContract.test.js 的 C4）。
 *   W8 不叠第二个心跳循环：Do-Watch 与 Do-Keep 都必须先查 Get-WatcherPid 再动手。
 *   W9 每一次 adb 调用都要有**单次超时**（#1568，2026-10-08）：`Invoke-Adb` 必须委托
 *      `scripts/lib/auto-screenshot.ps1` 的那件唯一有界执行核（16 个调用方全压在这一个点上，而本工具是
 *      **设备掉线时才用的自愈工具** —— 最容易撞上「adb 不返回」的那条路）。判据五组：① 真源 dot-source 在位，
 *      且路径用「自身扩展名」构造（写死 `.ps1` 字面量会撞 W6，写反斜杠在 Linux 上解析不了）；
 *      ② 两档预算都是带默认值的参数（读数 15 秒 / 端点 30 秒，票面 AC 第 3 条禁止把长等套进 15 秒）；
 *      ③ 旧无界形状不得回写（代码面 `& $AdbExe` 与 `Out-String` 各零处）；
 *      ④ 非 Windows 走直启（`-DirectExec:(-not $IsWindows)`：本工具的行为守护在 CI 的 ubuntu 上真跑，
 *      恒经 cmd 那一层就只剩「起不来」一条出口，「有界」会退化成「永远拿不到结果」）；
 *      ⑤ 「不返回」是可见结论（`ADB_CALL_TIMEOUT` 点名哪一次调用与多大预算 + `ADB_CALL_BUDGET` 读数行），
 *      且点名件只进 UTF-8 日志、不写 stdout —— `-Quiet` 取 serial 那条路径的全部价值就是 stdout 只有 serial。
 *      另钉 `-MergeStdErr`：本工具三条判据的原料在 stderr 上（pair 分类的 protocol fault、connect 的 10061）。
 *
 * 形态沿用本仓既有 .ps1 守护：**先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中**。
 * 变异分两类，用错方法就等于没注入：
 *   · **禁止型**（W1/W2/W4/W6/W7 的 gate-evidence）——出现即违规，用「整行追加」验，不依赖原文锚点；
 *   · **存在型**（W3 的两处 sleep 与时间窗、W5 的 ASCII 过滤、W6 的取名、W7 的非门声明、W8 的两处早退、
 *     W9 的委托与两档预算与点名——#1568 一次十条）
 *     ——追加一行永远打不中，只能「把该在的拿掉」验；这类每条都先断言替换真的发生
 *     （`expect(m).not.toBe(real)`），因为替换锚点会随措辞漂移，锚点一漂用例就变成恒绿的空跑。
 */
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/wireless-debug.ps1';

/** 掩掉 <# ... #> 文档块（字符换成空格、行号不变）——红线管的是代码面，不是注释与文档。 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 去掉整行 `#` 注释（保留行号）。 */
function stripLineComments(text) {
  return text
    .split('\n')
    .map((l) => (/^\s*#/.test(l) ? '' : l))
    .join('\n');
}

const codeFace = (text) => stripLineComments(maskDocBlocks(text));

function docBlock(text) {
  const m = text.match(/^<#[\s\S]*?#>/);
  return m ? m[0] : '';
}

/** 取某个 function 的函数体（到下一个顶层 `function ` 或文件尾）。 */
function functionBody(code, name) {
  const start = code.indexOf('function ' + name);
  if (start < 0) return null;
  const next = code.slice(start + 1).search(/^function /m);
  return next < 0 ? code.slice(start) : code.slice(start, start + 1 + next);
}

const FORBIDDEN = [
  ['kill-server', /kill-server/],
  ['install', /\binstall\b/],
  ['force-stop', /force-stop/],
  ['logcat -c', /logcat[^\n]*\s-c\b/],
  ['push', /\bpush\b/],
  ['reboot', /\breboot\b/],
  ['input tap|swipe|keyevent', /\binput\s+(?:tap|swipe|keyevent)\b/],
  ['screencap', /screencap/],
];

/** 纯函数：源文本 → 违规清单。判据全在这里，用例只负责「注入必须抓到」与「真文件必须零」。 */
function scanContract(src) {
  const v = [];
  const code = codeFace(src);
  const doc = docBlock(src);

  // W1 只读红线
  for (const [label, re] of FORBIDDEN) {
    const hits = code.match(new RegExp(re.source, 'g'));
    if (hits) v.push(`W1 出现禁用 adb 动作 ${label} ×${hits.length}`);
  }

  // W2 开关只写「开」方向，且不许写别的 key
  const putLines = code.match(/'settings',\s*'put',[^\n]*/g) || [];
  const offKey = putLines.filter((s) => !/'adb_wifi_enabled'/.test(s));
  if (offKey.length) v.push(`W2 有 ${offKey.length} 处 settings put 不在 adb_wifi_enabled 上`);
  const badVals = putLines
    .map((s) => /'adb_wifi_enabled',\s*'([^']*)'/.exec(s))
    .filter(Boolean)
    .map((m) => m[1])
    .filter((x) => x !== '1');
  if (badVals.length) v.push(`W2 adb_wifi_enabled 被写成非「开」值：${badVals.join(',')}`);

  // W3 循环必睡 + 放弃按时间窗
  const keep = functionBody(code, 'Do-Keep');
  if (!keep) {
    v.push('W3 找不到 Do-Keep 函数体（判据取不到即红，不在空集合上判绿）');
  } else {
    for (const need of ['Start-Sleep -Seconds $IntervalSeconds', 'Start-Sleep -Seconds $ProbeSeconds']) {
      if (!keep.includes(need)) v.push(`W3 Do-Keep 缺 ${need}（连接态与掉线态各需一处，漏一处就是空转）`);
    }
    if (!/TotalMinutes/.test(keep)) v.push('W3 放弃判据没按时间窗（TotalMinutes）');
    if (/\$fail(?:Count|Times)\s*-ge/.test(keep)) v.push('W3 出现「按失败次数放弃」的形态');
  }

  // W4 不得写死宿主树绝对路径（整份文件都算，注释也不例外）
  if (/D:\\FL/.test(src)) v.push('W4 出现宿主树绝对路径 D:\\FL');
  if (/\.scratch/.test(src)) v.push('W4 出现 .scratch 路径（入仓件不得依赖未入库目录）');

  // W5 机读行 ASCII 过滤在位
  if (!/WIRELESS_DEBUG/.test(code)) v.push('W5 代码面没有 WIRELESS_DEBUG 结论行前缀');
  if (!/\[\^\\x20-\\x7E\]/.test(code)) v.push('W5 结论行缺非 ASCII 过滤');

  // W6 进程归属判据取当前脚本名，不得写死字面量。
  // 判据取「代码面一个 .ps1 字面量都不许有」而不是「只盯 Get-WatcherPid 那几行」：
  // 这个工具在代码面上根本不需要自己的文件名（文档块与注释已被掩掉），出现任何一条都是写死。
  if (/'[^'\n]*\.ps1'/.test(code)) v.push('W6 代码面出现 .ps1 文件名字面量（改名即静默失效、留孤儿进程）');
  const owner = functionBody(code, 'Get-WatcherPid') || '';
  if (!owner) v.push('W6 找不到 Get-WatcherPid 函数体（判据取不到即红）');
  else if (!/GetFileName\(\s*\$script:ScriptFile\s*\)/.test(owner)) v.push('W6 未从 $script:ScriptFile 取自己的文件名');

  // W7 非门声明在文档面；gate-evidence: 只禁代码面
  if (!doc.includes('非门（不替代 ① 验收门')) v.push('W7 头部文档缺「非门（不替代 ① 验收门）」声明');
  if (/gate-evidence:/.test(code)) v.push('W7 代码面出现 gate-evidence: —— 会被 pr-evidence 当成验收门证据');

  // W8 不叠第二个心跳循环：早退必须既查进程、又留下可见的结论
  const watch = functionBody(code, 'Do-Watch');
  if (!watch || !/Get-WatcherPid/.test(watch) || !/already_running/.test(watch)) {
    v.push('W8 Do-Watch 缺「已查到活循环 ⇒ already_running 早退」这一段');
  }
  if (keep && (!/Get-WatcherPid/.test(keep) || !/REFUSE_STACK/.test(keep))) {
    v.push('W8 Do-Keep 缺「已查到活循环 ⇒ REFUSE_STACK 早退」这一段（两条心跳对同一设备只会互抢）');
  }

  // W9 每一次 adb 调用都必须**有单次超时**（#1568，2026-10-08 票面 AC 第 1 条点名本文件那个收口点）。
  //   症状：`Invoke-Adb` 是 `& $AdbExe @AdbArgs 2>&1 | Out-String` —— 前台同步等一次 adb、没有单次超时，
  //   而 **16 个调用方全压在这一个点上**，本工具又是**设备掉线时才用的自愈工具**（最容易撞上「adb 不返回」
  //   的那条路）。一次不返回就既不产结论也不报错：keep 循环静默停住、`ensure` 永远不返回端口。
  //   裁定：复用 `scripts/lib/` 那件**唯一**的有界执行核，不写第二份「等待 + 杀树」
  //   （移动端 ADR-0008「有界单次调用的三个消费点，与「不上收」的载体裁定」）。
  //   行为面（真挂死桩到点给结论 / 正常返回不误判 / stderr 判据原料不丢 / 两档预算各自点名）由
  //   `utils/wirelessDebugBoundedTextBehavior.test.js`（WBD1–WBD5）在运行期另钉 —— 这里只钉形状与「不得回写成无界」。
  const invokeAdb = functionBody(code, 'Invoke-Adb') || '';
  if (!invokeAdb) v.push('W9 找不到 Invoke-Adb 函数体（判据取不到即红，不在空集合上判绿）');
  if (!/function Note-AdbTimeout/.test(code)) v.push('W9 缺超时点名件（「不返回」必须是一种可见结论）');
  if (!/Join-Path \(Join-Path \$PSScriptRoot 'lib'\)/.test(code) || !/auto-screenshot/.test(code)) {
    v.push('W9 未 dot-source 有界执行器的唯一真源（lib/auto-screenshot.ps1）—— adb 调用又变成无界等待');
  }
  // 路径**不得**写成 `.ps1` 字面量（那会撞 W6），而扩展名从当前脚本自己取；顺带才是跨平台的
  if (!/\(Get-Item -LiteralPath \$PSCommandPath\)\.Extension/.test(code)) {
    v.push('W9 未用「自身扩展名」构造 lib 路径（写死 `.ps1` 字面量会撞 W6，写反斜杠在 Linux 上解析不了）');
  }
  if (invokeAdb && !/Invoke-BoundedAdbText -AdbExe \$AdbExe -Serial ''/.test(invokeAdb)) {
    v.push('W9 Invoke-Adb 没委托到唯一执行核（本工具的候选阶段没有 serial，`-Serial \'\'` 是刻意的）');
  }
  // 两档预算都得是**带默认值的参数**（票面 AC 第 3 条：读数和端点不是一档，且默认值指得到入库产物）
  if (!/\[int\]\$AdbCallTimeoutSeconds\s*=\s*15/.test(code)) v.push('W9 缺 -AdbCallTimeoutSeconds 默认 15 秒的读数档参数');
  if (!/\[int\]\$AdbEndpointTimeoutSeconds\s*=\s*30/.test(code)) v.push('W9 缺 -AdbEndpointTimeoutSeconds 默认 30 秒的端点档参数');
  if (!/-TimeoutSeconds\s+\$BudgetSeconds/.test(invokeAdb)) v.push('W9 单次预算没从参数透传给执行核');
  const endpointWired = (code.match(/-BudgetSeconds\s+\$AdbEndpointTimeoutSeconds/g) || []).length;
  if (endpointWired < 5) v.push(`W9 端点类调用只有 ${endpointWired} 处接上端点档（connect / disconnect×2 / pair / mdns×2 = 6 处，掉一处就静默回落读数档）`);
  // 非 Windows 不许经 cmd 那一层（本工具的行为守护在 CI 的 ubuntu 上真跑，经 cmd 就只剩「起不来」一条出口）
  if (!/-DirectExec:\(-not \$IsWindows\)/.test(invokeAdb)) {
    v.push('W9 直启开关没按平台判（无条件直启会换掉 Windows 那条已验证载体；恒经 cmd 则 Linux 上永远拿不到文本）');
  }
  // stderr 是本工具三条判据的原料（pair 分类的 protocol fault、connect 失败的 10061 ⇒ 决定要不要 disconnect）
  if (!/-MergeStdErr/.test(invokeAdb)) v.push('W9 没合并 stderr ⇒ adb 的失败原文进不了调用方的判据（静默换了判据）');
  // 旧形状不得回写：前台同步等一次 adb
  if (/&\s*\$AdbExe/.test(code)) v.push('W9 代码面回写了 `& $AdbExe` 直调（无单次超时 ⇒ 一次不返回整条自愈链就地停住）');
  if (/Out-String/.test(code)) v.push('W9 代码面回写了 Out-String 取文本（父进程读子进程输出段 = #1285 那条挂死路径）');
  // 「不返回」是可见结论：点名哪一次调用、多大预算，并有汇总读数
  if (!/ADB_CALL_TIMEOUT args=/.test(code)) v.push('W9 缺 ADB_CALL_TIMEOUT 点名行（哪一次调用、多大预算必须写出来）');
  if (!/callBudgetSeconds=/.test(code)) v.push('W9 超时结论没带预算大小');
  if (!/ADB_CALL_BUDGET calls=/.test(code)) v.push('W9 缺每次结论的调用/超时读数行');
  if (!/\$script:AdbTimeouts\s*=\s*@\(\$script:AdbTimeouts\)/.test(code)) v.push('W9 缺超时清单累计（汇总里点不出「本次挂过几次」）');
  // 超时结论只进日志文件，不许污染 stdout：`-Quiet` 那条路径的全部价值就是 stdout 只有 serial
  const noteBody = functionBody(code, 'Note-AdbTimeout') || '';
  if (!noteBody) v.push('W9 找不到 Note-AdbTimeout 函数体（判据取不到即红）');
  else if (/Write-Host|Say\s/.test(noteBody)) v.push('W9 超时点名件往 stdout 写了东西（`-Quiet` 取 serial 会被这一行污染）');

  return v;
}

const real = readText(path.join(ROOT, SCRIPT_REL));

/** 整行追加式变异：不依赖原文锚点，措辞怎么改都照样注入。 */
const injected = (line) => scanContract(real + '\n' + line + '\n');
/** 只取某一类违规，避免一条变异撞上多条规则时读不出「谁在管」。 */
const hitsOf = (list, tag) => list.filter((x) => x.startsWith(tag));

describe('wireless-debug.ps1 契约守护（#1564）', () => {
  // ── ① 检测器不是空跑：**禁止型**判据用整行追加来验（不依赖原文锚点，措辞怎么改都照样注入）
  const INJECTIONS = [
    ['W1', "& $AdbExe kill-server"],
    ['W1', "& $AdbExe -s $ep install app.apk"],
    ['W1', "& $AdbExe -s $ep shell force-stop io.dcloud.uniappx"],
    ['W1', "& $AdbExe -s $ep logcat -c"],
    ['W1', "& $AdbExe -s $ep push local /sdcard/x"],
    ['W1', "& $AdbExe -s $ep reboot"],
    ['W1', "& $AdbExe -s $ep shell input tap 540 926"],
    ['W1', "& $AdbExe -s $ep exec-out screencap -p"],
    ['W2', "Invoke-Adb @('-s', $ep, 'shell', 'settings', 'put', 'global', 'adb_wifi_enabled', '0')"],
    ['W2', "Invoke-Adb @('-s', $ep, 'shell', 'settings', 'put', 'global', 'stay_on_while_plugged_in', '7')"],
    ['W4', "$StateDir = 'D:\\FL\\.scratch\\adb-wireless'"],
    ['W6', "$mine = 'adb-wireless.ps1'"],
    ['W7', "Write-Host 'gate-evidence: 这条会被校验器当门证据'"],
    // W9（#1568）：无界形状一回写就红 —— 这两条是「不得回写」型，追加一行即可命中
    ['W9', "& $AdbExe -s $ep shell echo ok | Out-String"],
    ['W9', "$hb = Invoke-Adb @('-s', $ep, 'shell', 'echo', 'hb') 2>&1 | Out-String"],
  ];
  const REMOVALS_EXTRA = [
    ['W9', '委托唯一执行核', "Invoke-BoundedAdbText -AdbExe $AdbExe -Serial ''", "& $AdbExe @AdbArgs | Out-String"],
    ['W9', '读数档默认值', '[int]$AdbCallTimeoutSeconds = 15', '[int]$AdbCallTimeoutSeconds'],
    ['W9', '端点档默认值', '[int]$AdbEndpointTimeoutSeconds = 30', '[int]$AdbEndpointTimeoutSeconds'],
    ['W9', '端点档接线', '-BudgetSeconds $AdbEndpointTimeoutSeconds', '-BudgetSeconds $AdbCallTimeoutSeconds'],
    ['W9', 'stderr 合并（判据原料）', '-MergeStdErr', ''],
    ['W9', '直启按平台判', '-DirectExec:(-not $IsWindows)', '-DirectExec:$true'],
    ['W9', '超时点名行', 'ADB_CALL_TIMEOUT args=', 'TIMEOUT args='],
    ['W9', '自身扩展名构造 lib 路径', '(Get-Item -LiteralPath $PSCommandPath).Extension', "'ps' + '1'"],
    ['W9', 'dot-source 唯一真源', 'auto-screenshot', 'adb-local'],
    ['W9', '点名件只进日志不进 stdout', "try { Add-Content -LiteralPath $ActLogPath -Value ('{0} {1}' -f $t, $Line) -Encoding utf8 } catch { }", 'Write-Host $Line'],
  ];
  test.each(INJECTIONS)('%s：注入违规必须被抓到', (tag, line) => {
    expect(hitsOf(injected(line), tag).length).toBeGreaterThan(0);
  });

  // W9 的十条都是**存在型**（该在的东西被拿掉/换形）——追加一行打不中它们，只能就地拆。
  test.each(REMOVALS_EXTRA)('W9（#1568）：%s 被拿掉或换形 ⇒ 必须判红', (tag, label, from, to) => {
    const m = real.split(from).join(to);
    expect(m).not.toBe(real); // 锚点一漂就等于什么都没注入，用例就假绿
    expect(hitsOf(scanContract(m), tag).length).toBeGreaterThan(0);
  });

  // ── ② **存在型**判据只能靠「把该在的东西拿掉」来验：追加一行永远打不中它们。
  //    每条都先断言替换真的发生了——锚点一漂就等于什么都没注入，用例就假绿。
  const REMOVALS = [
    ['W3', '连接态睡眠', 'Start-Sleep -Seconds $IntervalSeconds', '$null = $null'],
    ['W3', '掉线态重发现间隔', 'Start-Sleep -Seconds $ProbeSeconds', '$null = $null'],
    ['W5', '非 ASCII 过滤', "[^\\x20-\\x7E]", '[[:print:]]'],
    ['W6', '从 $script:ScriptFile 取名', '$mine = [System.IO.Path]::GetFileName($script:ScriptFile)', "$mine = 'adb-wireless.ps1'"],
    ['W7', '非门声明', '非门（不替代 ① 验收门', '门（已替代 ① 验收门'],
    ['W8', 'Do-Keep 的叠循环早退', /if \(\$alive -and \$alive -ne \$PID\) \{[\s\S]*?\r?\n {4}\}/, '$null = $null'],
    ['W8', 'Do-Watch 的叠循环早退', 'already_running', 'already_gone'],
  ];
  test.each(REMOVALS)('%s：%s 被拿掉 ⇒ 必须判红', (tag, label, from, to) => {
    const m = typeof from === 'string' ? real.split(from).join(to) : real.replace(from, to);
    expect(m).not.toBe(real);
    expect(hitsOf(scanContract(m), tag).length).toBeGreaterThan(0);
  });

  test('W3：时间窗改成失败次数 ⇒ 两头都要判红（既没按窗，又按了次数）', () => {
    const m = real.replace('((Get-Date) - $absentSince).TotalMinutes -ge $WaitMinutes', '$failCount -ge 20');
    expect(m).not.toBe(real);
    const hits = hitsOf(scanContract(m), 'W3');
    expect(hits.join('|')).toMatch('时间窗');
    expect(hits.join('|')).toMatch('失败次数');
  });

  // ── ③ 真文件必须干净
  test('真文件零违规（九条判据同时成立）', () => {
    expect(scanContract(real)).toEqual([]);
  });

  // ── ④ 判据本身不能空转：两个面都取得到内容，函数体也取到
  test('取样面非空：文档块、代码面、四个函数体都真取到了', () => {
    expect(docBlock(real).length).toBeGreaterThan(800);
    expect(codeFace(real).length).toBeGreaterThan(4000);
    expect(functionBody(codeFace(real), 'Do-Keep')).not.toBeNull();
    expect(functionBody(codeFace(real), 'Do-Watch')).not.toBeNull();
    // W9 的两件（#1568）：判据落在函数体上，函数体取不到就必须在 scanContract 里当场判红
    expect(functionBody(codeFace(real), 'Invoke-Adb')).not.toBeNull();
    expect(functionBody(codeFace(real), 'Note-AdbTimeout')).not.toBeNull();
  });

  test('W4 的正向读数：状态目录走机器级路径（不是宿主树）', () => {
    expect(real).toMatch('ADB_WIRELESS_STATE_DIR');
    expect(real).toMatch('LOCALAPPDATA');
  });
});
