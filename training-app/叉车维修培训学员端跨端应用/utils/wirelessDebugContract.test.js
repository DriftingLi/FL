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
 *
 * 形态沿用本仓既有 .ps1 守护：**先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中**。
 * 变异分两类，用错方法就等于没注入：
 *   · **禁止型**（W1/W2/W4/W6/W7 的 gate-evidence）——出现即违规，用「整行追加」验，不依赖原文锚点；
 *   · **存在型**（W3 的两处 sleep 与时间窗、W5 的 ASCII 过滤、W6 的取名、W7 的非门声明、W8 的两处早退）
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
  ];
  test.each(INJECTIONS)('%s：注入违规必须被抓到', (tag, line) => {
    expect(hitsOf(injected(line), tag).length).toBeGreaterThan(0);
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
  test('真文件零违规（八条判据同时成立）', () => {
    expect(scanContract(real)).toEqual([]);
  });

  // ── ④ 判据本身不能空转：两个面都取得到内容，函数体也取到
  test('取样面非空：文档块、代码面、两个函数体都真取到了', () => {
    expect(docBlock(real).length).toBeGreaterThan(800);
    expect(codeFace(real).length).toBeGreaterThan(4000);
    expect(functionBody(codeFace(real), 'Do-Keep')).not.toBeNull();
    expect(functionBody(codeFace(real), 'Do-Watch')).not.toBeNull();
  });

  test('W4 的正向读数：状态目录走机器级路径（不是宿主树）', () => {
    expect(real).toMatch('ADB_WIRELESS_STATE_DIR');
    expect(real).toMatch('LOCALAPPDATA');
  });
});
