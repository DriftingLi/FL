/**
 * 真机只读取证契约守护（scripts/device-capture.ps1）
 *
 * 背景（2026-09-12，为 PR #624「生物识别快捷登录」真机验证备料）：
 * ① 真机门已决定拆成 ①a（agent 用 adb 自动逐页截图 + logcat 断言）与 ①b（**人**走关键交互，如指纹）。
 * 本脚本只做 ①a 的取证，**不替代 ①b**。它的全部风险在于**被当成门用**，或者被后续会话
 * 「顺手优化」时破掉它赖以成立的几条红线，所以用本测试把这几条锁死：
 *
 *   D1 只读禁令：正文（去掉 <# #> 文档块与整行注释后）**不得**出现 kill-server / install /
 *      force-stop / logcat -c —— 这几条会改设备状态或打断维护者的调试会话
 *   D2 切页默认关闭：闸门定义逐字锁定，且唯一的 am start 调用点必须在「非只读」分支里
 *   D3 PR 评论标记必须是 `prefilter:device-capture`，**不得**出现 gate-evidence:
 *      —— 否则会被 .github/workflows/pr-evidence.yml 当成验收门证据（本次刻意不改校验器）
 *   D4 截图入库上限常量（宽 720 / 单张 150KB / 每 PR 10 张 / 合计 1.5MB）与编码器探测、-NoArchive 逃生
 *   D5 adb devices 多设备 ⇒ 必须显式 -Device，否则 exit 2（绝不自动猜）
 *   D6 收尾走 finally：不留后台进程
 *   D7 判成败只看输出（DEVICE_CAPTURE_RESULT），**不得**出现 $LASTEXITCODE
 *   D8 ADR-0008 已记录「①a」，且写明只替代取证、不替代 ①b，且不得代填「执行人」
 *   D9 脚本必须是 LF 行尾（.gitattributes 钉了 *.ps1；Windows 上 CRLF 会让本测试的字面量锚点全部失配）
 *   D10 切页模式必须有「页身份」fail-closed 断言：截图记 SHA256，多页哈希相同即判相关页 FAIL
 *      —— 2026-09-12 PR #898 实测 `am start -d uniapp://<page>` 未让 App 换页、五图同哈希却各判 PASS
 *      （原有四项断言覆盖不到「目标页是否真的加载」），故把这条补成硬规
 *   D11 头部文档块必须记有「adb 无法滚动 / 切页」的**机械根因**（INJECT_EVENTS 拒注入 /
 *      UniPortraitPageActivity 未导出 / uniapp:// scheme 未注册）与**可靠替代**（临时把目标页置为
 *      pages.json 首项 + 人点一次运行），且不得把已按实测订正的旧说法写回
 *      —— 2026-09-13 真机实测（#937 取证会话）。只记「不生效」不记「为什么」，后续会话会把它读成
 *      「写法不对、换个 intent 就好」，反复重试一条**机械上不可能**的路（#970）
 *   D12 【#1562，2026-10-07 票面现测】每张**截图**调用都必须有**单次超时**，且残帧不进证据名：
 *      截图复用 `lib/auto-screenshot.ps1` 的有界执行核（唯一真源，不复制第二份）、预算来自 `param()`、
 *      先判 `$shot.TimedOut` 再看帧在不在、`.part` 成功才归位、超时机检行点名「哪一次/多大预算/有无残帧」
 *      —— 旧写法 `& cmd.exe /c "… > file"` 无单次超时，一次不返回就**既不产帧也不报错**，后面的页也跑不到
 *      （同族现测见 #1560：32 分钟零写入，而同时刻手工 `screencap` 4.8 秒返回）。
 *      行为面由 `deviceCaptureBoundedShotBehavior.test.js`（BSD1–BSD5）另钉 —— 这里只钉形状与「不得回写成无界」。
 *   D13 【#1568 AC 第 2 条，2026-10-08 票面现测】本文件另外 6 处**文本 / 管理类**直调（`Get-DeviceList` /
 *      每页都跑的 `Get-ForegroundInfo` / `Get-LogcatBaseline` / `Get-LogcatWindow` / `Resolve-LauncherComponent` /
 *      `Start-AppPage`）也全部接上同一件唯一执行核，并**按调用形态分档**（单行读 15 秒、全量 logcat 60 秒）。
 *      判据四格：委托层是档位与点名的唯一落点、`& $AdbExe` 在本文件**归零**（棘轮，只减不增）、
 *      logcat 挂死的 `TimedOut` 必须落到主失败清单（空日志会被读成「无崩溃」）、文档块那句「射程就到截图为止」
 *      已过期且**不许**被写成全仓声明（剩余面只许指向复算尺 `docs/verification/tooling/1568/adb-bounded-count.mjs`）。
 *      行为面（真挂死桩 / 对照腿不误判 / server 级不带 -s / 预算是参数 / 挂死后流程继续）由
 *      `deviceCaptureTierBehavior.test.js`（DCT1–DCT13）另钉。
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/emulatorSmokeContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/device-capture.ps1';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';

const PR_MARKER = 'prefilter:device-capture';
const GATE_MARKER = 'gate-evidence:';
const NON_GATE_BANNER = '非门（不替代 ①b：指纹等关键交互仍由人走）';
const PAGE_SWITCH_OFF_NOTE = '切页动作默认关闭：会打断维护者的调试会话';
const GATE_FORBID = '$ForbidStart = ($NoStartApp -or $SkipAppStart)';
const GATE_CAN = '$CanStart = ($AllowAppStart -and -not $ForbidStart)';
const TEARDOWN_ANCHOR = "    Stop-LeftoverChildren\n    Write-Log '收尾完成";

// D11（2026-09-14 补，#970）——「切页为何不可用」的机械根因与可靠替代必须留在文档块里。
// 判据取自 2026-09-13 真机实测（#937 取证会话）：三条路全部堵死，而其中两条是**系统级拒绝**，
// 不是拼写问题 ⇒ 这段「为什么」被删掉，后人就会去重试一条不可能的路。
const ADB_INJECT_DENIED = 'INJECT_EVENTS';
const ADB_ACTIVITY_NOT_EXPORTED = 'UniPortraitPageActivity';
const ADB_NOT_EXPORTED_NOTE = '未导出';
const ADB_SCHEME_UNREGISTERED = '未注册';
const ADB_FALLBACK_PAGES_JSON = 'pages.json';
const ADB_FALLBACK_FIRST_PAGE = '首项';
const ADB_CLI_CHANNEL_PREREQ = 'cli open';
const ADB_LAUNCH_STALE_CLAIM = '已知可用的替代切页机制是';

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/** 掩掉 <# ... #> 文档块（块内字符替换成空格，行号不变）——头部标注/禁令本来就允许写在文档块里 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 取 <# ... #> 文档块的**原文**（未掩码）：禁令说明允许（且要求）写在这里 */
function docTextOf(text) {
  return (text.match(/<#[\s\S]*?#>/g) || []).join('\n');
}

/** 掩掉整行注释（只掩以 # 开头的整行；行内注释与字符串一律保留，避免掩盖真正的代码） */
function stripLineComments(text) {
  return text.replace(/^[ \t]*#[^\n]*$/gm, '');
}

/** 正文＝去掉文档块与整行注释之后的代码。禁令只对正文生效。 */
function codeOf(text) {
  return stripLineComments(maskDocBlocks(text));
}

/** 纯函数：源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const raw = sources.script;
  const code = codeOf(raw);
  const doc = docTextOf(raw);
  const adr = sources.adr;

  // D1 只读禁令（正文里一个都不许有；禁令说明本身只许待在 <# #> 文档块里）
  const FORBIDDEN = ['kill-server', 'force-stop', 'logcat -c', 'install', 'uninstall'];
  FORBIDDEN.forEach((token) => {
    if (code.includes(token)) {
      violations.push('D1 正文出现只读禁令词：' + token + '（会改设备状态或打断维护者调试会话）');
    }
  });
  // D1b 文档块必须**写明**这些禁令（否则后人不知道红线在哪）
  FORBIDDEN.forEach((token) => {
    if (!doc.includes(token)) {
      violations.push('D1b 文档块未写明禁令：' + token);
    }
  });

  // D2 切页默认关闭
  if (!/\[switch\]\$AllowAppStart/.test(code)) {
    violations.push('D2 缺 -AllowAppStart 开关（切页只允许显式开启）');
  }
  if (!/\[switch\]\$NoStartApp/.test(code) || !/\[switch\]\$SkipAppStart/.test(code)) {
    violations.push('D2 缺 -NoStartApp / -SkipAppStart（显式禁止切页的开关）');
  }
  if (!code.includes(GATE_FORBID)) {
    violations.push('D2 切页禁止闸门定义被改：期望 ' + GATE_FORBID);
  }
  if (!code.includes(GATE_CAN)) {
    violations.push('D2 切页放行闸门定义被改：期望 ' + GATE_CAN);
  }
  if (!code.includes(PAGE_SWITCH_OFF_NOTE)) {
    violations.push('D2 缺「' + PAGE_SWITCH_OFF_NOTE + '」提示语（默认关闭必须写在输出里）');
  }
  // am start 只允许出现在 Start-AppPage 里，且该函数唯一的调用点必须在「非只读」分支内
  const amIdx = code.indexOf("'am', 'start'");
  if (amIdx < 0) {
    violations.push('D2 找不到 am start 调用（本测试的锚点已失效，请同步更新）');
  } else {
    const fnIdx = code.lastIndexOf('function Start-AppPage', amIdx);
    if (fnIdx < 0 || fnIdx > amIdx) {
      violations.push('D2 am start 不在 Start-AppPage 函数里');
    }
  }
  const callCount = (code.match(/Start-AppPage -Page/g) || []).length;
  if (callCount !== 1) {
    violations.push('D2 Start-AppPage 调用点必须恰好 1 处（实得 ' + callCount + '）');
  }
  const readonlyIdx = code.indexOf('if (-not $CanStart) {');
  if (readonlyIdx < 0) {
    violations.push('D2 缺「if (-not $CanStart) {」只读分支锚点');
  }
  const elseIdx = readonlyIdx < 0 ? -1 : code.indexOf('} else {', readonlyIdx);
  const callIdx = code.indexOf('Start-AppPage -Page');
  if (elseIdx < 0) {
    violations.push('D2 只读分支后缺 else（切页调用点必须落在非只读分支里）');
  } else if (callIdx < elseIdx) {
    violations.push('D2 切页调用点在只读分支之前/之内（默认只读会被绕过）');
  }

  // D3 PR 标记
  if (!code.includes(PR_MARKER)) {
    violations.push('D3 缺 ' + PR_MARKER + ' 标记（PR 评论认不出这是非门前置取证）');
  }
  if (code.includes(GATE_MARKER)) {
    violations.push('D3 出现 ' + GATE_MARKER + '：会被 pr-evidence 校验器误当成验收门证据');
  }
  if (!/\$PostToPr/.test(code)) {
    violations.push('D3 缺 -PostToPr 参数');
  }

  // D4 截图入库纪律
  if (!/docs\\verification|\$Module/.test(code)) {
    violations.push('D4 缺 docs/verification/<模块>/ 入库路径（-Module / -ArchiveModule）');
  }
  if (!/\$ArchiveMaxWidth\s*=\s*720/.test(code)) {
    violations.push('D4 缺宽度上限 720px');
  }
  if (!/\$ArchiveMaxBytes\s*=\s*150\s*\*\s*1024/.test(code)) {
    violations.push('D4 缺单张上限 150 KB');
  }
  if (!/\$ArchiveMaxCount\s*=\s*10/.test(code)) {
    violations.push('D4 缺每 PR 张数上限 10');
  }
  if (!/\$ArchiveMaxTotalBytes\s*=\s*1536\s*\*\s*1024/.test(code)) {
    violations.push('D4 缺合计上限 1.5 MB');
  }
  ['cwebp', 'ffmpeg', 'magick'].forEach((token) => {
    if (!code.includes(token)) {
      violations.push('D4 缺 WebP 编码器探测：' + token);
    }
  });
  if (!code.includes('NoArchive')) {
    violations.push('D4 缺 -NoArchive 逃生开关');
  }
  if (!/skippedPages|SkippedPages/.test(code)) {
    violations.push('D4 减图后必须写明省了哪几页');
  }

  // D5 多设备 ⇒ 显式 -Device，否则 exit 2
  if (!/function Fail-Environment \{[\s\S]*?exit 2/.test(code)) {
    violations.push('D5 Fail-Environment 未用 exit 2（环境不可用）');
  }
  const multiIdx = code.indexOf('$online.Count -gt 1');
  if (multiIdx < 0) {
    violations.push('D5 缺「多设备」分支（$online.Count -gt 1）');
  } else if (!/Fail-Environment/.test(code.slice(multiIdx, multiIdx + 800))) {
    violations.push('D5 多设备分支未走 Fail-Environment（未 exit 2）');
  }
  const autoPickIdx = code.indexOf('$online[0].Serial');
  if (autoPickIdx < 0) {
    violations.push('D5 缺「唯一在线设备」自动取用（$online[0].Serial）');
  } else if (multiIdx >= 0 && autoPickIdx < multiIdx) {
    violations.push('D5 自动取用在多设备判定之前（多设备会被静默取第一个）');
  }
  if (!/\$Device/.test(code)) {
    violations.push('D5 缺 -Device 参数');
  }

  // D6 收尾在 finally，且不留后台进程
  const mainCatchIdx = code.indexOf('脚本异常');
  if (mainCatchIdx < 0) {
    violations.push('D6 缺主 try/catch 的异常日志锚点（脚本异常）');
  }
  const teardownFinallyIdx = mainCatchIdx < 0 ? -1 : code.indexOf('} finally {', mainCatchIdx);
  if (teardownFinallyIdx < 0) {
    violations.push('D6 主 try 缺 finally 块（收尾必须走无条件路径）');
  } else if (code.indexOf('Stop-LeftoverChildren', teardownFinallyIdx) < 0) {
    violations.push('D6 主 try 的 finally 里缺 Stop-LeftoverChildren 收尾');
  }
  if (!/function Stop-LeftoverChildren \{[\s\S]*?\.Kill\(\)/.test(code)) {
    violations.push('D6 Stop-LeftoverChildren 未真正停止子进程（缺 .Kill()）');
  }

  // D7 只看输出
  if (/\$LASTEXITCODE/.test(code)) {
    violations.push('D7 出现 $LASTEXITCODE：判成败只看输出（DEVICE_CAPTURE_RESULT），不得依赖退出码');
  }
  if (!code.includes('DEVICE_CAPTURE_RESULT')) {
    violations.push('D7 缺 DEVICE_CAPTURE_RESULT 判据输出');
  }

  // D9 LF 行尾（Windows 上 CRLF 会让本测试的字面量锚点全部失配）
  if (raw.includes('\r')) {
    violations.push('D9 脚本含 CRLF 行尾（.gitattributes 已钉 *.ps1 为 LF）');
  }

  // D8 ADR 记录
  if (!adr.includes('①a')) {
    violations.push('D8 ADR-0008 未记录 ①a');
  }
  if (!adr.includes('不替代 ①b')) {
    violations.push('D8 ADR 未写明 ①a 不替代 ①b（人工关键交互）');
  }
  if (!adr.includes('device-capture.ps1')) {
    violations.push('D8 ADR 未点名载体 scripts/device-capture.ps1');
  }
  if (!adr.includes('#883')) {
    violations.push('D8 ADR 未引用 #883 的 devtools console 证据');
  }
  if (/生物识别[^\n]{0,10}已通过/.test(adr)) {
    violations.push('D8 ADR 不得声称生物识别「已通过」（①b 仍是人工门）');
  }
  if (/执行人\s*[:：]\s*\S/.test(adr)) {
    violations.push('D8 ADR 不得代填「执行人」');
  }

  // D10 页身份 fail-closed：切页静默失效必须被判出，不得各判 PASS
  // （2026-09-12 PR #898 实测：`am start -d uniapp://<page>` 未让 App 换页，五张 *-after.png
  //   SHA256 完全相同，却因「前台/字节数/FATAL/ANR」四项都过而各判 PASS —— 那四项覆盖不到
  //   「目标页是否真的加载」。故切页模式必须自带哈希撞车判据。）
  if (!/function Export-Screenshot \{[\s\S]*?Get-FileHash[\s\S]*?SHA256/.test(code)) {
    violations.push('D10 截图未记 SHA256（页身份断言缺输入）');
  }
  if (!code.includes('$script:ShotRecords | Where-Object { $_.Name -eq $r.Shot }')) {
    violations.push('D10 缺「按截图名回查哈希」的关联步骤');
  }
  if (!code.includes('切页未生效')) {
    violations.push('D10 缺「切页未生效」失败文案（切页失效须被判出而非静默 PASS）');
  }
  if (!code.includes("$hit.Status = 'FAIL'")) {
    violations.push('D10 哈希撞车未把相关页判 FAIL');
  }
  if (!code.includes('页身份断言：')) {
    violations.push('D10 汇总未打出页身份断言状态（人无法一眼核）');
  }

  // D11 adb 机械根因与其可靠替代（#970）——「为什么不可用」这段不许被删。
  //      判据落在**文档块**（docTextOf）而非正文：这是文档纪律，正文里本来就不该出现这些机制。
  if (!doc.includes(ADB_INJECT_DENIED)) {
    violations.push('D11 文档块缺「输入注入被系统硬拒」的根因（' + ADB_INJECT_DENIED + '）');
  }
  if (!doc.includes(ADB_ACTIVITY_NOT_EXPORTED)) {
    violations.push('D11 文档块缺未导出 Activity 名（' + ADB_ACTIVITY_NOT_EXPORTED + '）——它是「意图从未送达」的直接证据');
  }
  if (!doc.includes(ADB_NOT_EXPORTED_NOTE)) {
    violations.push('D11 文档块缺「' + ADB_NOT_EXPORTED_NOTE + '」判据（只写活动名不足以说明 intent 送不到）');
  }
  if (!doc.includes(ADB_SCHEME_UNREGISTERED)) {
    violations.push('D11 文档块缺「uniapp:// scheme ' + ADB_SCHEME_UNREGISTERED + '」（这条说明不存在换写法绕过的余地）');
  }
  if (!doc.includes(ADB_FALLBACK_PAGES_JSON) || !doc.includes(ADB_FALLBACK_FIRST_PAGE)) {
    violations.push('D11 文档块缺可靠替代：把目标页临时置为 ' + ADB_FALLBACK_PAGES_JSON + ' 的 ' + ADB_FALLBACK_FIRST_PAGE + '（启动页）');
  }
  if (!doc.includes(ADB_CLI_CHANNEL_PREREQ)) {
    violations.push('D11 文档块缺 cli launch 路径的前提「' + ADB_CLI_CHANNEL_PREREQ + '」（GUI 已运行时该路径不可用，不得写成随手可用）');
  }
  if (doc.includes(ADB_LAUNCH_STALE_CLAIM)) {
    violations.push('D11 已按实测订正的旧说法被写回：' + ADB_LAUNCH_STALE_CLAIM + '（该路径需先建 CLI 通道，非随手可用）');
  }

  // D12 单次 adb 调用必须有超时，且残帧不进证据名（#1562，2026-10-07 票面现测）。
  //   症状：`Export-Screenshot` 原先是 `& cmd.exe /c "adb … exec-out screencap -p > file" 2>&1 | Out-Null`
  //   —— 前台同步等待、**没有单次超时**。#1560 已证明这形状偶发但真实（同机同时刻手工 `screencap`
  //   4.8 秒返回，而链里那一次 32 分钟没回来）⇒ 一次不返回就既不产帧也不报错，**后面的页也跑不到**，
  //   而 ①a 的全部产物就是「逐页的图 + 一行机检结论」。
  //   判据落点：复用 `lib/auto-screenshot.ps1` 那份有界执行核（唯一真源，**不**在此复制第二份），
  //   预算是 `param()` 里的参数，超时先判、再看文件在不在（删除不是判据 —— #1560 真链路现测证明
  //   被杀调用的残帧**可能删不掉**，而本链的出图判据正是「存在且非空」⇒ 半张图会伪装成本次证据）。
  //   行为面（真挂死桩 / 残帧留盘仍不进证据名 / 超时后下一页照旧跑）由
  //   `deviceCaptureBoundedShotBehavior.test.js`（BSD1–BSD5）在运行期另钉。
  if (!/\. \(Join-Path \$PSScriptRoot 'lib\\auto-screenshot\.ps1'\)/.test(code)) {
    violations.push('D12 未 dot-source 有界执行器唯一真源（lib/auto-screenshot.ps1）—— 截图调用又变成无界等待');
  }
  if (!/Invoke-BoundedAdbShot -AdbExe \$AdbExe -Serial \$script:Serial -OutFile \$partPath -TimeoutSeconds \$AdbCallTimeoutSeconds/.test(code)) {
    violations.push('D12 截图未走有界执行器，或单次预算没从参数透传');
  }
  if (!/\[int\]\$AdbCallTimeoutSeconds\s*=\s*15/.test(code)) {
    violations.push('D12 缺 -AdbCallTimeoutSeconds 单次预算参数（默认 15 秒；理由写在 param 注释里）');
  }
  if (/cmd\.exe\s+\/c/.test(code)) {
    violations.push('D12 正文回写 cmd.exe /c 直调截图（无单次超时 ⇒ 一次不返回整条取证链就地停住）');
  }
  if (/exec-out screencap -p\s*>/.test(code)) {
    violations.push('D12 截图重定向回到 cmd 的内层 `>`（现由执行器 -RedirectStandardOutput 落盘）');
  }
  if (!/\$partPath = "\$path\.part"/.test(code)) {
    violations.push('D12 缺 `.part` 暂存名 —— 被杀调用的半张图会直接落在证据名上');
  }
  if (!/Move-Item -LiteralPath \$partPath -Destination \$path -Force/.test(code)) {
    violations.push('D12 缺「调用返回 + 帧非空才归位」那一步（归位靠命名，不靠删除成功）');
  }
  const tmoIdx = code.indexOf('if ($shot.TimedOut) {');
  const partExistsIdx = code.indexOf('if (-not (Test-Path -LiteralPath $partPath)', tmoIdx < 0 ? 0 : tmoIdx);
  if (tmoIdx < 0 || partExistsIdx < 0) {
    violations.push('D12 超时判据缺失或顺序错：必须先判 `$shot.TimedOut`，再看帧文件在不在');
  } else if (code.indexOf('if (-not (Test-Path -LiteralPath $partPath)') < tmoIdx) {
    violations.push('D12 出图判据排在超时之前（会把被杀的半帧当成一次成功截图）');
  }
  if (!/SHOT_CALL_TIMEOUT/.test(code)) {
    violations.push('D12 缺 SHOT_CALL_TIMEOUT 机检行（哪一次调用、多大预算、有没有留残帧必须点名）');
  }
  if (!/\$script:ShotTimeouts/.test(code)) {
    violations.push('D12 缺超时调用清单（汇总里点不出「本次挂过几次」）');
  }
  if (!/截图调用未在 \$AdbCallTimeoutSeconds 秒内返回/.test(code)) {
    violations.push('D12 缺「截图调用未在 N 秒内返回」的失败文案（通道问题不得被读成设备没亮屏）');
  }

  // D13（#1568 AC 第 2 条，2026-10-08）：本文件 **6 处文本/管理类直调清零** + 按调用形态分档。
  //   症状：这 6 处原先都是 `& $AdbExe … 2>&1 | Out-String` —— 前台同步等一次 adb、**没有单次超时**。
  //   最险的一格是 `Get-LogcatWindow`：挂死的下场是**空日志**⇒ 崩溃计数读成 0 = 把挂死读成「无崩溃」，
  //   比原来的就地停住更坏；所以 `TimedOut` 必须一路带到主判据，而不是只留在日志里。
  //   这一格是 emulatorSmoke 的 C10「只判函数体」的反面 —— **文件做完了，整文件禁令才撑得住**（棘轮只减不增）。
  //   档位默认值只引用指得到库内产物的现测（`docs/verification/tooling/1568/`）：
  //     devices 106 ms / version 178 ms（latency-readings.txt）；
  //     dumpsys activity activities 1,168 / 1,662 ms、resolve-activity 457 / 336 ms、logcat -d 5,896 / 6,186 ms
  //     （emulator-tier-readings.txt 与 -run2.txt）⇒ 单行读走文本档 15 秒，全量 dump 走 logcat 档 60 秒。
  const tierStart = code.indexOf('function Invoke-AdbTierCall');
  const tierEnd = code.indexOf('function Get-DeviceList');
  const tierBody = tierStart < 0 ? '' : code.slice(tierStart, tierEnd > tierStart ? tierEnd : code.length);
  if (tierStart < 0) {
    violations.push('D13 找不到 Invoke-AdbTierCall（档位 / 点名 / serial 取舍的唯一落点；判据取不到即红，不在空集合上判绿）');
  } else {
    if (!/Invoke-BoundedAdbText -AdbExe \$AdbExe -Serial \$adbSerial/.test(tierBody)) {
      violations.push('D13 委托层没接到唯一执行核（调用点又回到无界等待）');
    }
    if (!/\$adbSerial = \$\(if \(\$NoSerial\) \{ '' \} else \{ \$script:Serial \}\)/.test(tierBody)) {
      violations.push('D13 serial 不是按 -NoSerial 现算的（server 级命令恒拼 -s = 换判据）');
    }
    if (!/\[int\]\$BudgetSeconds\s*=\s*\$AdbTextCallTimeoutSeconds/.test(tierBody)) {
      violations.push('D13 单次预算没从 param() 的档位取默认值（写死在函数体里就等于没有档）');
    }
    if (!/-TimeoutSeconds \$BudgetSeconds/.test(tierBody)) violations.push('D13 预算没透传给执行核');
    if (!/-MergeStdErr/.test(tierBody)) {
      violations.push('D13 没合并 stderr ⇒ adb 的失败原文进不了调用方（老形状是 2>&1，不合并就是静默换判据）');
    }
    if (!/-DirectExec:\(-not \$IsWindows\)/.test(tierBody)) {
      violations.push('D13 直启没按平台判（恒经 cmd ⇒ 本族的运行期腿在 CI 的 ubuntu 上造不出来，只能静默假绿）');
    }
    if (/Out-String/.test(tierBody)) {
      violations.push('D13 委托层里回写了 Out-String 取文本（父进程读子进程输出段 = #1285 那条挂死路径）');
    }
    if (!/ADB_TEXT_TIMEOUT call=adb /.test(tierBody)) violations.push('D13 缺超时点名行（哪一次调用、多大预算必须写出来）');
    if (!/tier=\$Tier/.test(tierBody)) violations.push('D13 点名行没写 tier=（分不清挂的是哪一档、该找谁改预算）');
    if (!/\$script:AdbTextTimeouts = @\(\$script:AdbTextTimeouts\)/.test(tierBody)) {
      violations.push('D13 缺超时清单累计（汇总点不出「本次挂过几次」）');
    }
    if (!/\$script:AdbTextCalls = \$script:AdbTextCalls \+ 1/.test(tierBody)) {
      violations.push('D13 缺调用计数（汇总 calls= 的分母；没有分母就读不出 timeouts= 那一格）');
    }
  }
  // ② 整文件棘轮：本文件 6 处已全部收口 ⇒ 直调禁令撑得住全文（与 emulator-smoke 的 C11 同律）
  const adbDirect = (code.match(/&\s*\$AdbExe/g) || []).length;
  if (adbDirect !== 0) {
    violations.push('D13 代码面还剩 ' + adbDirect + ' 处 `& $AdbExe` 直调（无单次超时 ⇒ 一次不返回整条取证链就地停住）');
  }
  // ③ 档位参数与默认值（默认值被删 = 调用方拿到 0 秒档；长等档被塞进文本档 = 把正常当挂死）
  if (!/\[int\]\$AdbTextCallTimeoutSeconds\s*=\s*15/.test(code)) {
    violations.push('D13 缺 -AdbTextCallTimeoutSeconds 默认 15 秒（单行读与 server 级同档）');
  }
  if (!/\[int\]\$AdbLogcatTimeoutSeconds\s*=\s*60/.test(code)) {
    violations.push('D13 缺 -AdbLogcatTimeoutSeconds 默认 60 秒（全量 logcat -d 现测 5.9 / 6.2 秒，与文本档不同形）');
  }
  // ④ 点位 ↔ 档位 接线（同一行；档写了却没接上 = 那处仍是无界）
  const SITE_WIRING = [
    ['adb devices -l（server 级）', /@\('devices', '-l'\)[^\n]*-BudgetSeconds \$AdbTextCallTimeoutSeconds[^\n]*-Tier 'server'[^\n]*-NoSerial/],
    ['dumpsys activity activities', /@\('shell', 'dumpsys', 'activity', 'activities'\)[^\n]*-BudgetSeconds \$AdbTextCallTimeoutSeconds[^\n]*-Tier 'read'/],
    ['logcat 基线 -t 1', /@\('shell', 'logcat', '-d', '-v', 'epoch', '-t', '1'\)[^\n]*-BudgetSeconds \$AdbLogcatTimeoutSeconds/],
    ['logcat 全量窗口', /@\('shell', 'logcat', '-d', '-v', 'epoch'\)[^\n]*-BudgetSeconds \$AdbLogcatTimeoutSeconds/],
    ['resolve-activity', /'resolve-activity'[^\n]*-BudgetSeconds \$AdbTextCallTimeoutSeconds[^\n]*-Tier 'resolve'/],
    ['am start', /-AdbArgs \$intentArgs[^\n]*-BudgetSeconds \$AdbTextCallTimeoutSeconds[^\n]*-Tier 'am-start'/]
  ];
  SITE_WIRING.forEach(([label, re]) => {
    if (!re.test(code)) violations.push('D13 点位「' + label + '」没在自己的那一档上（或 -NoSerial 丢了）');
  });
  // ④b 设备探测的两个出口必须分开且**挂死排在前**：一次不返回的 `adb devices` 也给出空列表，
  //     顺序反了就把「通道故障」写成「请先恢复无线连接」（同一个 exit 2，两种成因，读者查的地方不同）
  const devHangIdx = code.indexOf('if ($found.TimedOut) {');
  const devEmptyIdx = code.indexOf("if ($devices.Count -eq 0) {");
  if (devHangIdx < 0) {
    violations.push('D13 设备探测没有自己的挂死出口（不返回会被读成「没有设备」）');
  } else if (devEmptyIdx >= 0 && devEmptyIdx < devHangIdx) {
    violations.push('D13 「没有设备」判定排在挂死之前（通道故障被写成用户该去重连无线）');
  }
  // ⑤ logcat 窗口挂死 ⇒ 落失败 + 汇总不可判（空日志被读成「无崩溃」是本票最险的一格）
  if (!/if \(\$final\.TimedOut\) \{\s*\$failures \+=/.test(code)) {
    violations.push('D13 logcat 窗口读取超时未计入失败（本次仍会被判 PASS ⇒ 挂死被读成「无崩溃」）');
  }
  if (!/TimedOut\s*=\s*\[bool\]\$r\.TimedOut/.test(code)) {
    violations.push('D13 没把执行核的 TimedOut 带进返回值（调用方无从区分「日志干净」与「日志没读到」）');
  }
  if (!/LOGCAT_INCONCLUSIVE=/.test(code)) {
    violations.push('D13 汇总缺 LOGCAT_INCONCLUSIVE（崩溃计数不可判必须在结论里点名）');
  }
  // ⑥ StrictMode 初值：读未声明属性会抛 ⇒ 接线自己把脚本跑炸（本文件 Set-StrictMode -Version Latest）
  if (!/\$script:Foreground = \[pscustomobject\]@\{ Component = ''; Raw = ''; Source = ''; TimedOut = \$false \}/.test(code)) {
    violations.push('D13 $script:Foreground 初值缺 TimedOut（StrictMode 下读未声明属性会抛）');
  }
  if (!/\$script:FinalTotals = \[pscustomobject\]@\{[\s\S]{0,400}?AnrSamples = @\(\); TimedOut = \$false/.test(code)) {
    violations.push('D13 $script:FinalTotals 初值缺 TimedOut（PR 评论段在 try 之外读它 ⇒ 异常路径上会撞 StrictMode）');
  }
  if (!/Lines = 0; Total = 0; WindowLines = 0; WindowStart = 0\.0; WindowKnown = \$false/.test(code)) {
    violations.push('D13 $script:FinalTotals 初值段被改（后续读 .FatalCount 会撞 StrictMode）');
  }
  // ⑦ 汇总读数行：事后能从日志核对「当时生效的是哪一档、挂了几次」
  if (!/ADB_TEXT_CALL_BUDGET calls=/.test(code)) {
    violations.push('D13 缺取文本那档的汇总读数行（分不清 adb 通道挂了与设备答了个空）');
  }
  if (!/ADB_TIER_BUDGET[^\n]*logcat=\$AdbLogcatTimeoutSeconds/.test(code)) {
    violations.push('D13 汇总缺 ADB_TIER_BUDGET 档位读数（分不清「预算给小了」与「设备真挂了」）');
  }
  // ⑧ 文档块口径：既不许留在过期声明里，也不许把本文件的收口写成全仓的收口
  if (doc.includes('射程就到截图为止')) {
    violations.push('D13 文档块仍写「射程就到截图为止」——本文件 6 处直调已收口，该句过期（会让下一会话误判射程）');
  }
  if (!doc.includes('本文件的每次 adb 调用都有单次超时')) {
    violations.push('D13 文档块缺现行口径「本文件的每次 adb 调用都有单次超时」');
  }
  if (doc.includes('全仓每次 adb 调用都有单次超时')) {
    violations.push('D13 把本文件的收口写成了整个仓的收口（声明比代码宽；剩余面只许指向复算尺）');
  }
  if (!doc.includes('adb-bounded-count.mjs')) {
    violations.push('D13 文档块没指复算尺 —— 剩余读数必须可复算，抄数字就是第二真源');
  }

  return violations;
}

describe('真机只读取证契约（①a 预置 · 非门 · 不替代 ①b）', () => {
  const real = { script: readSource(SCRIPT_REL), adr: readSource(ADR_REL) };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['D1', '正文混入 kill-server', (s) => ({ ...s, script: s.script + '\n& $AdbExe kill-server\n' })],
      ['D1', '正文混入 force-stop', (s) => ({ ...s, script: s.script + "\n$out = (& $AdbExe -s $script:Serial shell am force-stop $pkg 2>&1)\n" })],
      ['D1', '正文混入 logcat -c', (s) => ({ ...s, script: s.script + "\n& $AdbExe -s $script:Serial logcat -c 2>&1 | Out-Null\n" })],
      ['D2', '切页禁止闸门被短路', (s) => ({ ...s, script: s.script.replace(GATE_FORBID, '$ForbidStart = $false') })],
      ['D2', '只读分支锚点被改', (s) => ({ ...s, script: s.script.replace('if (-not $CanStart) {', 'if ($CanStart) {') })],
      ['D2', '默认关闭提示语被删', (s) => ({ ...s, script: s.script.replace(/切页动作默认关闭：会打断维护者的调试会话/g, '默认') })],
      ['D3', '混入门证据标记', (s) => ({ ...s, script: s.script.replace(new RegExp(PR_MARKER, 'g'), 'gate-evidence:④') })],
      ['D4', '单张上限被放大', (s) => ({ ...s, script: s.script.replace('$ArchiveMaxBytes = 150 * 1024', '$ArchiveMaxBytes = 500 * 1024') })],
      ['D5', '多设备判定被放宽', (s) => ({ ...s, script: s.script.replace('if ($online.Count -gt 1) {', 'if ($online.Count -gt 99) {') })],
      ['D6', 'finally 里收尾被注掉', (s) => {
        if (!s.script.includes(TEARDOWN_ANCHOR)) { throw new Error('D6 注入点已失效，请同步更新本用例'); }
        return { ...s, script: s.script.replace(TEARDOWN_ANCHOR, "    Write-Log '收尾完成") };
      }],
      ['D7', '拿退出码当判据', (s) => ({ ...s, script: s.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' })],
      ['D8', 'ADR 抹掉「不替代 ①b」', (s) => ({ ...s, adr: s.adr.replace(/不替代 ①b/g, '不替代某物') })],
      ['D8', 'ADR 代填执行人', (s) => ({ ...s, adr: s.adr + '\n执行人：alice\n' })],
      ['D9', '脚本被写出 CRLF', (s) => ({ ...s, script: s.script.replace(/\n/g, '\r\n') })],
      ['D10', '截图不再记 SHA256', (s) => ({ ...s, script: s.script.replace(/Get-FileHash[\s\S]*?SHA256\)\.Hash/, "''") })],
      ['D10', '按截图名回查哈希被删', (s) => ({ ...s, script: s.script.replace('$script:ShotRecords | Where-Object { $_.Name -eq $r.Shot }', '$script:ShotRecords') })],
      ['D10', '哈希撞车不再判 FAIL', (s) => ({ ...s, script: s.script.replace("$hit.Status = 'FAIL'", '$hit.Status = $hit.Status') })],
      ['D10', '「切页未生效」文案被删', (s) => ({ ...s, script: s.script.replace(/切页未生效/g, '页问题') })],
      ['D10', '汇总页身份行被删', (s) => ({ ...s, script: s.script.replace('页身份断言：', '备注：') })],
      ['D11', '输入注入被拒的根因被删', (s) => ({ ...s, script: s.script.replace(/INJECT_EVENTS/g, '注入被系统拒绝') })],
      ['D11', '未导出 Activity 名被删', (s) => ({ ...s, script: s.script.replace(/UniPortraitPageActivity/g, '某 Activity') })],
      ['D11', '「未导出」判据被删', (s) => ({ ...s, script: s.script.replace(/未导出/g, '不可用') })],
      ['D11', 'scheme 未注册判据被删', (s) => ({ ...s, script: s.script.replace(/未注册/g, '不可达') })],
      ['D11', '可靠替代（pages.json 首项）被删', (s) => ({ ...s, script: s.script.replace(/pages\.json/g, '页面配置') })],
      ['D11', 'cli 通道前提被删', (s) => ({ ...s, script: s.script.replace(/cli open/g, 'cli 已连通') })],
      ['D11', '已订正的旧说法被写回', (s) => ({ ...s, script: s.script + '\n<#\n已知可用的替代切页机制是 HBuilderX 的 cli launch app-android --pagePath。\n#>\n' })],
      // D12（#1562）：每条都对应一种「把单次超时又拆掉」的真实改法
      ['D12', '有界执行器的 dot-source 被删（回到本地无界调用）', (s) => ({ ...s, script: s.script.replace(/.*lib\\auto-screenshot\.ps1'\).*/m, '# x') })],
      ['D12', '单次预算参数失去默认值', (s) => ({ ...s, script: s.script.replace('[int]$AdbCallTimeoutSeconds = 15', '[int]$AdbCallTimeoutSeconds') })],
      ['D12', '旧无界形状被回写（cmd.exe /c + 内层 >）', (s) => ({ ...s, script: s.script + '\n$cmd = \'"{}" -s {} exec-out screencap -p > "{}"\'; & cmd.exe /c $cmd 2>&1 | Out-Null\n' })],
      ['D12', '「成功才归位」被删（半张图直接落在证据名上）', (s) => ({ ...s, script: s.script.replace('Move-Item -LiteralPath $partPath -Destination $path -Force', '# 归位被删') })],
      ['D12', '超时判据被降级成「看文件在不在」', (s) => ({ ...s, script: s.script.replace('if ($shot.TimedOut) {', 'if ($false) {') })],
      ['D12', 'SHOT_CALL_TIMEOUT 机检行被删', (s) => ({ ...s, script: s.script.replace(/SHOT_CALL_TIMEOUT/g, 'SHOT_TIMEOUT') })],
      ['D12', '「未在 N 秒内返回」文案被改成通用失败', (s) => ({ ...s, script: s.script.replace(/截图调用未在 \$AdbCallTimeoutSeconds 秒内返回/g, '截图失败') })],
      // D13（#1568 AC 第 2 条）：每条都对应一种「把这 6 处又放回无界 / 把声明写歪」的真实改法
      ['D13', '正文回写 & $AdbExe 直调（本文件的棘轮是整文件级）', (s) => ({ ...s, script: s.script + '\n$raw = (& $AdbExe devices -l 2>&1 | Out-String)\n' })],
      ['D13', '委托层被拆掉（档位与点名各点位抄一份）', (s) => ({ ...s, script: s.script.replace('function Invoke-AdbTierCall', 'function Invoke-TierXyz') })],
      ['D13', '委托层没接到唯一执行核', (s) => ({ ...s, script: s.script.replace('Invoke-BoundedAdbText -AdbExe $AdbExe -Serial $adbSerial', 'Invoke-BoundedAdbTextX -AdbExe $AdbExe -Serial $adbSerial') })],
      ['D13', 'serial 改成恒拼（server 级命令换了判据）', (s) => ({ ...s, script: s.script.replace("$adbSerial = $(if ($NoSerial) { '' } else { $script:Serial })", "$adbSerial = $script:Serial") })],
      ['D13', 'stderr 不再合并（adb 失败原文进不了调用方）', (s) => ({ ...s, script: s.script.replace(' -MergeStdErr', '') })],
      ['D13', '直启不按平台判（恒经 cmd）', (s) => ({ ...s, script: s.script.replace('-DirectExec:(-not $IsWindows)', '# 直启被摘') })],
      ['D13', '预算写死在函数体里（param() 的档不再被引用）', (s) => ({ ...s, script: s.script.replace('[int]$BudgetSeconds = $AdbTextCallTimeoutSeconds', '[int]$BudgetSeconds = 15') })],
      ['D13', 'logcat 档被塞进文本档（把现测 6.2 秒的正常当挂死）', (s) => ({ ...s, script: s.script.replace('[int]$AdbLogcatTimeoutSeconds = 60', '[int]$AdbLogcatTimeoutSeconds = 15') })],
      ['D13', '文本档默认值被删（调用方拿到 0 秒档）', (s) => ({ ...s, script: s.script.replace('[int]$AdbTextCallTimeoutSeconds = 15', '[int]$AdbTextCallTimeoutSeconds') })],
      ['D13', 'server 点位丢了 -NoSerial', (s) => ({ ...s, script: s.script.replace("-Tier 'server' -NoSerial", "-Tier 'server'") })],
      ['D13', 'logcat 点位没接自己的档（写成文本档）', (s) => ({ ...s, script: s.script.replace("-BudgetSeconds $AdbLogcatTimeoutSeconds", '-BudgetSeconds $AdbTextCallTimeoutSeconds') })],
      ['D13', 'logcat 窗口挂死不再落失败（挂死读成「无崩溃」）', (s) => ({ ...s, script: s.script.replace('if ($final.TimedOut) {', 'if ($false) {') })],
      ['D13', '设备探测的挂死出口被并进了「没有设备」', (s) => ({ ...s, script: s.script.replace('if ($found.TimedOut) {', 'if ($false) {') })],
      ['D13', 'FinalTotals 初值缺 TimedOut（异常路径上 PR 评论段会抛）', (s) => ({ ...s, script: s.script.replace('AnrSamples = @(); TimedOut = $false', 'AnrSamples = @()') })],
      ['D13', 'TimedOut 没带进返回值', (s) => ({ ...s, script: s.script.replace(/= \[bool\]\$r\.TimedOut/g, '= $false') })],
      ['D13', 'LOGCAT_INCONCLUSIVE 机检行被删', (s) => ({ ...s, script: s.script.replace(/LOGCAT_INCONCLUSIVE/g, 'LOGCAT_X') })],
      ['D13', 'Foreground 初值缺 TimedOut（StrictMode 下接线自己把脚本跑炸）', (s) => ({ ...s, script: s.script.replace("$script:Foreground = [pscustomobject]@{ Component = ''; Raw = ''; Source = ''; TimedOut = $false }", "$script:Foreground = [pscustomobject]@{ Component = ''; Raw = ''; Source = '' }") })],
      ['D13', '超时点名行被删（读日志的人分不清通道挂了与答了个空）', (s) => ({ ...s, script: s.script.replace('ADB_TEXT_TIMEOUT call=adb ', 'X call=adb ') })],
      ['D13', '汇总缺档位读数行', (s) => ({ ...s, script: s.script.replace('ADB_TIER_BUDGET shot=', 'X_TIER shot=') })],
      ['D13', '过期的「射程就到截图为止」被写回文档块', (s) => ({ ...s, script: s.script + '\n<#\n⚠️ 这句话的射程就到截图为止。\n#>\n' })],
      ['D13', '把本文件的收口写成整个仓的收口（声明比代码宽）', (s) => ({ ...s, script: s.script + '\n<#\n本仓结论：全仓每次 adb 调用都有单次超时。\n#>\n' })],
      ['D13', '复算尺的指向被删（剩余读数不可复算）', (s) => ({ ...s, script: s.script.replace(/adb-bounded-count\.mjs/g, '那把尺') })]
    ];
    cases.forEach(([rule, label, mutate]) => {
      const found = scanContract(mutate(real));
      expect({ label, hit: found.some((v) => v.startsWith(rule)) }).toEqual({ label, hit: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('正文不含只读禁令词，文档块写明了全部禁令', () => {
    const code = codeOf(real.script);
    const doc = docTextOf(real.script);
    ['kill-server', 'force-stop', 'logcat -c', 'install', 'uninstall'].forEach((token) => {
      expect(code.includes(token)).toBe(false);
      expect(doc).toContain(token);
    });
  });

  it('am start 只有一处调用点，且落在「非只读」分支里', () => {
    const code = codeOf(real.script);
    expect((code.match(/Start-AppPage -Page/g) || []).length).toBe(1);
    // 只读分支闸门必须**只有一处**：两处会让「锚点被改」的注入自检打偏（本次实测踩过）
    expect(code.split('if (-not $CanStart) {').length - 1).toBe(1);
    const readonlyIdx = code.indexOf('if (-not $CanStart) {');
    const elseIdx = code.indexOf('} else {', readonlyIdx);
    expect(readonlyIdx).toBeGreaterThan(-1);
    expect(elseIdx).toBeGreaterThan(readonlyIdx);
    expect(code.indexOf('Start-AppPage -Page')).toBeGreaterThan(elseIdx);
  });

  it('切页模式自带页身份 fail-closed 断言（防「五图同哈希却各判 PASS」）', () => {
    const code = codeOf(real.script);
    expect(code).toMatch(/Get-FileHash[\s\S]*?SHA256/);
    expect(code).toContain('切页未生效');
    expect(code).toContain("$hit.Status = 'FAIL'");
    expect(code).toContain('页身份断言：');
    // 断言必须落在切页分支之后（只读模式不适用）
    const startIdx = code.indexOf('Start-AppPage -Page');
    expect(startIdx).toBeGreaterThan(-1);
    expect(code.indexOf('切页未生效')).toBeGreaterThan(startIdx);
  });

  it('文档块记有 adb 机械根因与可靠替代，且已订正的旧说法未回写（D11）', () => {
    const doc = docTextOf(real.script);
    expect(doc).toContain(ADB_INJECT_DENIED);
    expect(doc).toContain(ADB_ACTIVITY_NOT_EXPORTED);
    expect(doc).toContain(ADB_NOT_EXPORTED_NOTE);
    expect(doc).toContain(ADB_SCHEME_UNREGISTERED);
    expect(doc).toContain(ADB_FALLBACK_PAGES_JSON);
    expect(doc).toContain(ADB_FALLBACK_FIRST_PAGE);
    expect(doc).toContain(ADB_CLI_CHANNEL_PREREQ);
    expect(doc.includes(ADB_LAUNCH_STALE_CLAIM)).toBe(false);
  });

  it('默认（不加开关）就是只读：切页闸门与提示语都在', () => {
    const code = codeOf(real.script);
    expect(code).toContain(GATE_FORBID);
    expect(code).toContain(GATE_CAN);
    expect(code).toContain(PAGE_SWITCH_OFF_NOTE);
    expect(code).toContain('只读取证：不切页');
  });

  it('PR 标记是 prefilter:device-capture，且全文无 gate-evidence:', () => {
    const code = codeOf(real.script);
    expect(code).toContain(PR_MARKER);
    expect(code.includes(GATE_MARKER)).toBe(false);
  });

  it('收尾在 finally 里，且不留后台进程', () => {
    const code = codeOf(real.script);
    const mainCatch = code.indexOf('脚本异常');
    const at = mainCatch < 0 ? -1 : code.indexOf('} finally {', mainCatch);
    expect(mainCatch).toBeGreaterThan(-1);
    expect(at).toBeGreaterThan(-1);
    expect(code.indexOf('Stop-LeftoverChildren', at)).toBeGreaterThan(at);
  });

  it('ADR-0008 记明「①a 只替代取证、不替代 ①b」与 #883 证据，且不代填执行人', () => {
    expect(real.adr).toContain('①a');
    expect(real.adr).toContain('不替代 ①b');
    expect(real.adr).toContain('device-capture.ps1');
    expect(real.adr).toContain('不改验收门校验器');
    expect(real.adr).toContain('checkIsSupportSoterAuthentication');
    expect(/执行人\s*[:：]\s*\S/.test(real.adr)).toBe(false);
  });

  it('脚本是 LF 行尾，且非门标注三处齐（头部 / 日志首行 / PR 评论）', () => {
    expect(readSource(SCRIPT_REL).includes('\r')).toBe(false);
    const occurrences = (real.script.match(new RegExp(NON_GATE_BANNER.replace(/[()]/g, '\\$&'), 'g')) || []).length;
    expect(occurrences).toBeGreaterThanOrEqual(3);
    expect(real.script).toContain('$NonGateBanner');
    const code = codeOf(real.script);
    expect(/Set-Content[^\n]*\$LogPath[^\n]*\$NonGateBanner/.test(code)).toBe(true);
    expect(/\$bodyLines = @\([\s\S]{0,200}?\$NonGateBanner/.test(code)).toBe(true);
  });
});