/**
 * 仿真机前置冒烟契约守护（scripts/emulator-smoke.ps1）
 *
 * 背景（#883 spike / O2，2026-09-11）：维护者选定「装 SDK + 仿真机做**前置自动冒烟**」，
 * ① 真机门语义**不放宽**、仍由人在真机执行。这个脚本的全部风险在于**被当成门用**，
 * 或被后续会话"顺手优化"掉它赖以成立的几件事，所以用本测试把这几条锁死：
 *   C1 非门标注必须三处齐：脚本头部 / 日志首行 / 可选 PR 评论
 *   C2 判成败只看输出与 logcat，**不得**出现 $LASTEXITCODE 作判据（沿用本仓口径）
 *   C3 finally 路径里必须 adb emu kill（异常/断言失败都不许留下跑着的模拟器）
 *   C4 PR 评论标记必须是 `prefilter:emulator-smoke`，且**不得**出现 `gate-evidence:`
 *      —— 否则会被 .github/workflows/pr-evidence.yml 当成验收门证据
 *   C5 页面清单参数与截图落在 .ci-verify/
 *   C6 ADR-0008 已记录「非门辅助：仿真机前置冒烟」，且不得写成「已通过」/代填执行人
 *   C7 装机脚本存在、SDK 路径默认在 D 盘（非系统盘、不含空格）且可配置
 *   C8 截图入库纪律（非门证据）：路径 / 上限 / 编码器探测 / 逃生开关 / 提交前提
 *   C9 【#1562，2026-10-07 票面现测】每张**截图**调用都必须有**单次超时**，且残帧不进证据名：
 *      截图复用 `lib/auto-screenshot.ps1` 的有界执行核、预算来自 `param()`、先判 TimedOut 再看帧在不在、
 *      `.part` 成功才归位、超时**计入该页判定**（否则挂死被读成「跑过了」）、机检行点名哪一次调用。
 *      与 `device-capture.ps1` 逐字同形的那一处由 D12 钉（同一族，两个消费点各钉各的调用点）。
 *      行为面由 `emulatorSmokeBoundedShotBehavior.test.js`（ESD1–ESD5）另钉。
 *   C10 【#1568，2026-10-08 票面 AC 第 1 条】**文本**收口点 `Get-AdbOutput` 同样接上那件唯一执行核：
 *      它自己只有 1 行调用、却压着 **12 个调用方**（`sys.boot_completed` 轮询在最前面），判据五组——
 *      委托在位、预算是带默认值的**参数**且透传、stderr 合并（老形状是 `2>&1`，不合并就是静默换判据）、
 *      直启按平台判（恒经 cmd ⇒ CI 的 ubuntu 上那一格造不出来）、超时回空串且点名。
 *      AC 第 2 条把委托链改成两段（`Get-AdbOutput` → `Invoke-AdbTierCall` → 执行核），窗口随之取**这两段**：
 *      执行核仍只调一次、超时点名仍只有一处 ⇒ 「等待 + 杀树只有一份」这条不变。
 *   C11 【#1568，2026-10-08 票面 AC 第 2 条】本文件**那 13 处直调全部收口**，并按调用形态分档：
 *      ① 清零锁 —— 代码面（文档块与整行注释都掩掉之后）`& $AdbExe` 命中数必须为 **0**；
 *      ② 五个长等档参数在 `param()` 里且默认值**严格大于文本档**（把 `install`/`push`/`wait-for-device`
 *         套进 15 秒 = 把正常当挂死，票面 AC 第 3 条明令禁止；现测：冷起 wait-for-device 29.2/32.0 秒、
 *         install 8.5/12.3 秒、logcat -d 5.9/6.2 秒，见 `docs/verification/tooling/1568/emulator-tier-readings*.txt`）；
 *      ③ 每个点位与自己的档**在同一行**接上（档写了却没接 = 仍是无界）；`adb version` 那一格必须 `-NoSerial`
 *         （server 级命令恒拼 `-s` 就是换判据）；
 *      ④ `logcat -d` 的挂死必须成为**该页失败**（`if ($logcat.TimedOut) { $pageFail +=` 与 C9 给截图定的同形），
 *         并在汇总里落 `LOGCAT_INCONCLUSIVE` —— 空日志被读成「无崩溃」是本票最险的一格；
 *      ⑤ 收尾两格（`emu kill` / `wait-for-disconnect`）仍在**主 finally 段**里且接 teardown 档；
 *      ⑥ 汇总行 `ADB_TIER_BUDGET` 把六档现值打出来（事后能从日志核对「当时生效的是哪一档」）。
 *      行为面由 `emulatorSmokeTierBehavior.test.js`（EMT1–EMT9）另钉。
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/kotlinAllGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const SMOKE_REL = 'scripts/emulator-smoke.ps1';
const SETUP_REL = 'scripts/android-sdk-setup.ps1';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';

const PR_MARKER = 'prefilter:emulator-smoke';
const GATE_MARKER = 'gate-evidence:';
const NON_GATE_BANNER = '非门（不替代 ① 真机门）';

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/** 掩掉 <# ... #> 文档块（块内字符替换成空格，行号不变）——头部标注本来就允许写在文档块里 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 纯函数：源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const smoke = sources.smoke;
  const adr = sources.adr;
  // 掩码后的正文＝代码部分；「非门标注」允许出现在文档块里，其余规则查正文
  const smokeCode = maskDocBlocks(smoke);
  const bannerCount = (smoke.match(new RegExp(NON_GATE_BANNER.replace(/[()]/g, '\\$&'), 'g')) || []).length;

  // C1 非门标注三处：① 脚本头部（文档块）② 日志首行 ③ PR 评论正文
  if (bannerCount < 3) {
    violations.push('C1 非门标注不足三处（实得 ' + bannerCount + ' 处；需 脚本头部 / 日志首行 / PR 评论）');
  }
  if (!/\$NonGateBanner\s*=/.test(smokeCode)) {
    violations.push('C1 缺少 $NonGateBanner 常量（非门标注应集中一处、三处引用）');
  }
  if (!/Set-Content[^\n]*\$LogPath[^\n]*\$NonGateBanner/.test(smokeCode)) {
    violations.push('C1 日志首行未写非门标注（须 Set-Content $LogPath 写入 $NonGateBanner）');
  }
  if (!/\$bodyLines = @\([\s\S]{0,200}?\$NonGateBanner/.test(smokeCode)) {
    violations.push('C1 PR 评论正文未写非门标注');
  }

  // C2 不得用退出码判成败
  if (/\$LASTEXITCODE/.test(smokeCode)) {
    violations.push('C2 出现 $LASTEXITCODE：判成败只看输出与 logcat，不得依赖退出码');
  }
  ['EMULATOR_SMOKE_RESULT', 'logcat', 'FATAL EXCEPTION', 'ANR'].forEach((token) => {
    if (!smokeCode.includes(token)) {
      violations.push('C2 缺少判据要素：' + token);
    }
  });

  // C3 「冒烟主 try 的 finally」里必须有 adb emu kill。
  // 注意：脚本里还有别的 finally（如压缩工具的 bitmap 释放），所以判据锚在**主 catch**
  // （`脚本异常`）之后的那个 finally，而不是第一个 —— 否则改错地方也能"通过"。
  const mainCatchIdx = smokeCode.indexOf('脚本异常');
  if (mainCatchIdx < 0) {
    violations.push('C3 缺主 try/catch 的异常日志锚点（脚本异常）');
  }
  const teardownFinallyIdx = mainCatchIdx < 0 ? -1 : smokeCode.indexOf('} finally {', mainCatchIdx);
  if (teardownFinallyIdx < 0) {
    violations.push('C3 主 try 缺 finally 块（收尾必须走无条件路径）');
  }
  const killIdx = teardownFinallyIdx < 0 ? -1 : smokeCode.indexOf('emu kill', teardownFinallyIdx);
  if (killIdx < 0) {
    violations.push('C3 主 try 的 finally 里缺 adb emu kill 收尾');
  }

  // C4 PR 标记
  if (!smokeCode.includes(PR_MARKER)) {
    violations.push('C4 缺 ' + PR_MARKER + ' 标记（PR 评论认不出这是非门前置冒烟）');
  }
  if (smokeCode.includes(GATE_MARKER)) {
    violations.push('C4 出现 ' + GATE_MARKER + '：会被 pr-evidence 校验器误当成验收门证据');
  }
  if (!/\$PostToPr/.test(smokeCode)) {
    violations.push('C4 缺 -PostToPr 参数');
  }

  // C5 页面清单 + 截图落 .ci-verify/
  if (!/\$Pages\s*=/m.test(smokeCode) && !/\[string\]\$Pages/.test(smokeCode)) {
    violations.push('C5 缺 -Pages 页面清单参数');
  }
  if (!/\.ci-verify/.test(smokeCode)) {
    violations.push('C5 截图/日志未落 .ci-verify/');
  }
  if (!/emulator-\$?\(?.*\.png|Convert-PageToFileName|exec-out screencap/.test(smokeCode)) {
    violations.push('C5 缺 adb exec-out screencap -p 截图步骤');
  }

  // C6 ADR 记录
  if (!adr.includes('非门辅助：仿真机前置冒烟')) {
    violations.push('C6 ADR-0008 未记录「非门辅助：仿真机前置冒烟」');
  }
  if (!adr.includes('不替代 ①')) {
    violations.push('C6 ADR 未写明不替代 ① 真机门');
  }
  if (/仿真机[^\n]{0,20}已通过|生物识别[^\n]{0,10}已通过/.test(adr)) {
    violations.push('C6 ADR 不得声称仿真机/生物识别「已通过」（① 仍是人工门）');
  }
  if (/执行人\s*[:：]\s*\S/.test(adr)) {
    violations.push('C6 ADR 不得代填「执行人」');
  }

  // C7 装机脚本
  if (!sources.setup.includes('platform-tools')) {
    violations.push('C7 装机脚本未装 platform-tools');
  }
  if (!sources.setup.includes('emulator')) {
    violations.push('C7 装机脚本未装 emulator');
  }
  if (!/D:\\android-sdk/.test(sources.setup)) {
    violations.push('C7 装机脚本默认 SDK 路径不是 D:\\android-sdk（非系统盘、不含空格）');
  }
  if (!/\$SdkRoot/.test(sources.setup)) {
    violations.push('C7 装机脚本的 SDK 路径不可配置');
  }
  if (!/licenses/.test(sources.setup)) {
    violations.push('C7 装机脚本未交代 licenses');
  }
  // C7b 复用既有 AVD：必须修掉指向已删 SDK 的 skin.path（实测不修则 emulator 直接 exit）
  if (!/skin\.path/.test(sources.setup)) {
    violations.push('C7 装机脚本未处理复用 AVD 的 skin.path（emulator 会 unknown skin name 退出）');
  }
  if (!/unknown skin name/.test(sources.setup)) {
    violations.push('C7 装机脚本未记录「不改 skin.path 就 unknown skin name」这一实测坑位');
  }

  // C8 截图入库纪律（非门证据）：路径 / 上限 / 编码器探测 / 逃生开关 / 提交前提
  if (!/docs\\verification|\$Module/.test(smokeCode)) {
    violations.push('C8 缺 docs/verification/<模块>/ 入库路径（-Module）');
  }
  if (!/\$ArchiveMaxWidth\s*=\s*720/.test(smokeCode)) {
    violations.push('C8 缺宽度上限 720px');
  }
  if (!/\$ArchiveMaxBytes\s*=\s*150\s*\*\s*1024/.test(smokeCode)) {
    violations.push('C8 缺单张上限 150 KB');
  }
  if (!/\$ArchiveMaxCount\s*=\s*10/.test(smokeCode)) {
    violations.push('C8 缺每 PR 张数上限 10');
  }
  if (!/\$ArchiveMaxTotalBytes\s*=\s*1536\s*\*\s*1024/.test(smokeCode)) {
    violations.push('C8 缺合计上限 1.5 MB');
  }
  ['cwebp', 'ffmpeg', 'magick'].forEach((token) => {
    if (!smokeCode.includes(token)) {
      violations.push('C8 缺 WebP 编码器探测：' + token);
    }
  });
  if (!smokeCode.includes('NoArchive')) {
    violations.push('C8 缺 -NoArchive 逃生开关');
  }
  if (!/skippedPages|SkippedPages/.test(smokeCode)) {
    violations.push('C8 减图后必须在评论/日志写明省了哪几页');
  }

  // C9 单次 adb 调用必须有超时，且残帧不进证据名（#1562，2026-10-07 票面现测）。
  //   症状：`Export-Screenshot` 与 `device-capture.ps1` **逐字同形** —— `& cmd.exe /c "… exec-out screencap -p > file"`，
  //   前台同步等待、没有单次超时 ⇒ 一次不返回就既不产帧也不报错、后面的页也跑不到（同族现测见 #1560）。
  //   ⚠️ 判据必须落在**剥掉整行注释**的正文上：本文件为 C9 写下的坑位说明本身就以 `#` 注释形态出现，
  //   直接拿 smokeCode 判「不得出现 cmd.exe /c」会在真文件上误报（本仓血账：注释会命中断言）。
  //   行为面（真挂死桩 / 残帧留盘仍不进证据名 / 超时后下一页照旧跑）由
  //   `emulatorSmokeBoundedShotBehavior.test.js`（ESD1–ESD5）另钉。
  const smokeBare = smokeCode.replace(/^[ \t]*#[^\n]*$/gm, '');
  if (!/\. \(Join-Path \$PSScriptRoot 'lib\\auto-screenshot\.ps1'\)/.test(smokeBare)) {
    violations.push('C9 未 dot-source 有界执行器唯一真源（lib/auto-screenshot.ps1）—— 截图调用又变成无界等待');
  }
  if (!/Invoke-BoundedAdbShot -AdbExe \$AdbExe -Serial \$Serial -OutFile \$partPath -TimeoutSeconds \$AdbCallTimeoutSeconds/.test(smokeBare)) {
    violations.push('C9 截图未走有界执行器，或单次预算没从参数透传');
  }
  if (!/\[int\]\$AdbCallTimeoutSeconds\s*=\s*15/.test(smokeBare)) {
    violations.push('C9 缺 -AdbCallTimeoutSeconds 单次预算参数（默认 15 秒；理由写在 param 注释里）');
  }
  if (/cmd\.exe\s+\/c/.test(smokeBare)) {
    violations.push('C9 正文回写 cmd.exe /c 直调截图（无单次超时 ⇒ 一次不返回整条冒烟就地停住）');
  }
  if (/exec-out screencap -p\s*>/.test(smokeBare)) {
    violations.push('C9 截图重定向回到 cmd 的内层 `>`（现由执行器 -RedirectStandardOutput 落盘）');
  }
  if (!/\$partPath = "\$path\.part"/.test(smokeBare)) {
    violations.push('C9 缺 `.part` 暂存名 —— 被杀调用的半张图会直接落在出图判据看的那个名字上');
  }
  if (!/Move-Item -LiteralPath \$partPath -Destination \$path -Force/.test(smokeBare)) {
    violations.push('C9 缺「调用返回 + 帧非空才归位」那一步（归位靠命名，不靠删除成功）');
  }
  const emuTmoIdx = smokeBare.indexOf('if ($shot.TimedOut) {');
  if (emuTmoIdx < 0 || smokeBare.indexOf('if (-not (Test-Path -LiteralPath $partPath)', emuTmoIdx) < 0) {
    violations.push('C9 超时判据缺失或顺序错：必须先判 `$shot.TimedOut`，再看帧文件在不在');
  }
  if (!/SHOT_CALL_TIMEOUT/.test(smokeBare)) {
    violations.push('C9 缺 SHOT_CALL_TIMEOUT 机检行（哪一次调用、多大预算、有没有留残帧必须点名）');
  }
  // 超时必须**落到该页的判定**上（票面：该页记失败并继续跑后面的页），否则「不返回」是一种静默结局
  if (!/if \(\$shot\.TimedOut\) \{\s*\$pageFail \+=/.test(smokeBare)) {
    violations.push('C9 截图超时未计入该页失败（该页仍会被判 PASS ⇒ 挂死被读成「跑过了」）');
  }
  if (!/\$script:ShotTimeouts/.test(smokeBare)) {
    violations.push('C9 缺超时调用清单（汇总里点不出「本次挂过几次」）');
  }

  // C10（#1568，2026-10-08 票面 AC 第 1 条）：**文本**收口点 `Get-AdbOutput` 也接上了同一件唯一执行核。
  //   症状：它是 `& $AdbExe -s $Serial @AdbArgs 2>&1` + `Out-String` —— 前台同步等一次 adb、没有单次超时，
  //   自己只有 1 行调用却压着 **12 个调用方**（`sys.boot_completed` 轮询排在最前面，挂在那里连「起没起机」
  //   都判不出来）。等待/杀树在本仓只许有一份（移动端 ADR-0008「有界单次调用的三个消费点」）。
  //   ⚠️ 与 C9 同一套剥注释判据（文档块 + 整行注释都掩掉），且**只判函数体** —— 本脚本另有 13 处直调
  //   仍无界（#1568 AC 第 2 条的后续 PR），在这里判全局 `Out-String` 禁令会把「还没做完」误判成「做错了」。
  //   行为面（真挂死桩到点 / 对照腿不误判 / stderr 原料不丢 / 预算是参数）由
  //   `emulatorSmokeBoundedTextBehavior.test.js`（ETD1–ETD5）另钉。
  const gaStart = smokeBare.indexOf('function Invoke-AdbTierCall');
  const gaEnd = smokeBare.indexOf('function Assert-Environment');
  const gaBody = gaStart < 0 ? '' : smokeBare.slice(gaStart, gaEnd > gaStart ? gaEnd : smokeBare.length);
  if (gaStart < 0) {
    violations.push('C10 找不到 Invoke-AdbTierCall（AC 第 2 条后收口链的第一段；判据取不到即红，不在空集合上判绿）');
  } else {
    if (!/Invoke-BoundedAdbText -AdbExe \$AdbExe -Serial \$adbSerial/.test(gaBody)) {
      violations.push('C10 取文本没委托到唯一执行核（调用方又回到无界等待）');
    }
    if (!/\$adbSerial = \$\(if \(\$NoSerial\) \{ '' \} else \{ \$Serial \}\)/.test(gaBody)) {
      violations.push('C10 serial 不是按 -NoSerial 现算的（server 级命令恒拼 -s = 换判据）');
    }
    if (!/-NoSerial:\$NoSerial/.test(gaBody)) {
      violations.push('C10 Get-AdbOutput 没把 -NoSerial 透传给 Invoke-AdbTierCall（server 那一格会退回带 -s）');
    }
    if (!/\[int\]\$BudgetSeconds\s*=\s*\$AdbTextCallTimeoutSeconds/.test(gaBody)) {
      violations.push('C10 单次预算没从 param() 的档位取默认值（写死在函数体里就等于没有档）');
    }
    if (!/-TimeoutSeconds\s+\$BudgetSeconds/.test(gaBody)) violations.push('C10 预算没透传给执行核');
    if (!/-MergeStdErr/.test(gaBody)) {
      violations.push('C10 没合并 stderr ⇒ adb 的失败原文进不了调用方（老形状是 2>&1，不合并就是静默换判据）');
    }
    if (!/-DirectExec:\(-not \$IsWindows\)/.test(gaBody)) {
      violations.push('C10 直启没按平台判（恒经 cmd ⇒ 本族的运行期腿在 CI 的 ubuntu 上造不出来，只能静默假绿）');
    }
    if (/&\s*\$AdbExe/.test(gaBody)) violations.push('C10 收口链里回写了 `& $AdbExe` 直调（无单次超时）');
    if (/Out-String/.test(gaBody)) violations.push('C10 收口链里回写了 Out-String 取文本（父进程读子进程输出段 = #1285 那条挂死路径）');
    if (!/if \(\$r\.TimedOut\) \{ return '' \}/.test(gaBody)) violations.push('C10 超时没回空串（既有语义「失败一律返回空串」不许改，调用方各按自己的分支判）');
    if (!/tier=\$Tier/.test(gaBody)) violations.push('C10 超时点名行没写 tier=（分不清挂的是哪一档、该找谁改预算）');
    if (!/ADB_TEXT_TIMEOUT call=adb /.test(gaBody)) violations.push('C10 缺超时点名行（哪一次调用、多大预算必须写出来）');
    if (!/\[int\]\$AdbTextCallTimeoutSeconds\s*=\s*15/.test(smokeBare)) violations.push('C10 缺 -AdbTextCallTimeoutSeconds 默认 15 秒的档位参数');
    if (!/\$script:AdbTextTimeouts\s*=\s*@\(\$script:AdbTextTimeouts\)/.test(gaBody)) violations.push('C10 缺超时清单累计（汇总点不出「本次挂过几次」）');
    if (!/ADB_TEXT_CALL_BUDGET calls=/.test(smokeBare)) violations.push('C10 缺取文本那档的汇总读数行（分不清 adb 通道挂了与设备答了个空）');
  }

  // C11（#1568，2026-10-08 票面 AC 第 2 条）：本文件 **13 处直调清零** + 按调用形态分档。
  //   这一格是 C10 当年「只判函数体」的反面：文件做完了，禁令才撑得住整文件。
  //   数字全部来自现测入库件（`docs/verification/tooling/1568/emulator-tier-readings.txt` 与 `-run2.txt`）：
  //   wait-for-device 29.2/32.0 s、install 8.5/12.3 s、push 69.7/111.1 ms 每 MB、logcat -d 5.9/6.2 s、
  //   emu kill 0.36/0.41 s、wait-for-disconnect 2.1/2.3 s、其余读文本 ≤ 2.5 s。
  //   ⚠️ 判据同样只落在剥注释后的正文（本文件为 C9/C10/C11 写的坑位说明都带 `#`，见 C9 那段血账）。
  const adbDirect = (smokeBare.match(/&\s*\$AdbExe/g) || []).length;
  if (adbDirect !== 0) {
    violations.push('C11 代码面还剩 ' + adbDirect + ' 处 `& $AdbExe` 直调（无单次超时 ⇒ 一次不返回整条冒烟就地停住）');
  }
  // ② 长等档必须**大于**文本档：等值或更小都说明有人把长等塞进了 15 秒（或把 15 秒改成了 0）
  const TEXT_TIER_DEFAULT = 15;
  const LONG_TIERS = [
    'AdbInstallTimeoutSeconds',
    'AdbPushTimeoutSeconds',
    'AdbBootWaitTimeoutSeconds',
    'AdbLogcatTimeoutSeconds',
    'AdbTeardownTimeoutSeconds'
  ];
  LONG_TIERS.forEach((name) => {
    const m = smokeBare.match(new RegExp('\\[int\\]\\$' + name + '\\s*=\\s*(\\d+)'));
    if (!m) {
      violations.push('C11 缺档位参数 -' + name + '（分档写死在函数体里就等于没有档）');
      return;
    }
    if (Number(m[1]) <= TEXT_TIER_DEFAULT) {
      violations.push('C11 -' + name + ' 默认 ' + m[1] + ' 秒没超过文本档 ' + TEXT_TIER_DEFAULT + ' 秒 ⇒ 长等被当挂死');
    }
  });
  // ③ 点位 ↔ 档位 接线（同一行；档写了却没接上 = 那处仍是无界）
  const SITE_WIRING = [
    ['install', /@\('install', '-r', '-t', \$BaseApk\)[^\n]*-BudgetSeconds \$AdbInstallTimeoutSeconds[^\n]*-Tier 'install'/],
    ['push', /@\('push',[^\n]*-BudgetSeconds \$AdbPushTimeoutSeconds[^\n]*-Tier 'push'/],
    ['logcat -d', /@\('logcat', '-d', '-v', 'brief'\)[^\n]*-BudgetSeconds \$AdbLogcatTimeoutSeconds[^\n]*-Tier 'logcat'/],
    ['emu kill', /@\('emu', 'kill'\)[^\n]*-BudgetSeconds \$AdbTeardownTimeoutSeconds[^\n]*-Tier 'teardown'/],
    ['wait-for-disconnect', /@\('wait-for-disconnect'\)[^\n]*-BudgetSeconds \$AdbTeardownTimeoutSeconds[^\n]*-Tier 'teardown'/],
    ['adb version（server 级）', /@\('version'\)[^\n]*-NoSerial/]
  ];
  SITE_WIRING.forEach(([label, re]) => {
    if (!re.test(smokeBare)) violations.push('C11 点位「' + label + '」没在自己的那一档上（或 -NoSerial 丢了）');
  });
  // wait-for-device 被抽成函数才谈得上「运行期腿真跑到它」；两处：定义 + Start-Emulator 里那一处调用
  if (!/function Wait-ForDeviceBounded/.test(smokeBare)) {
    violations.push('C11 wait-for-device 没收进可直调的函数（行为腿打不到它，那一格的有界就只剩注释）');
  }
  if ((smokeBare.match(/Wait-ForDeviceBounded/g) || []).length < 2) {
    violations.push('C11 Wait-ForDeviceBounded 定义了却没被 Start-Emulator 调用');
  }
  // ④ logcat 挂死 ⇒ 该页失败 + 汇总不可判（空日志被读成「无崩溃」是本票最险的一格）
  if (!/if \(\$logcat\.TimedOut\) \{\s*\$pageFail \+=/.test(smokeBare)) {
    violations.push('C11 logcat 读取超时未计入该页失败（该页仍会被判 PASS ⇒ 挂死被读成「无崩溃」）');
  }
  if (!/TimedOut\s*=\s*(\[bool\])?\$r\.TimedOut/.test(smokeBare)) {
    violations.push('C11 Get-LogcatSummary 没把 TimedOut 带出来（调用方无从区分「日志干净」与「日志没读到」）');
  }
  if (!/LOGCAT_INCONCLUSIVE=/.test(smokeBare)) {
    violations.push('C11 汇总缺 LOGCAT_INCONCLUSIVE（崩溃计数不可判必须在结论里点名）');
  }
  // ⑤ 收尾两格仍在主 finally 段里（挪出去 = 断言失败那条路不再收尾，留下跑着的模拟器）
  const mainFinallyIdx = smokeBare.indexOf('} finally {', smokeBare.indexOf('$script:FinalStatus = \'UNUSABLE\''));
  const mainFinallySeg = mainFinallyIdx < 0 ? '' : smokeBare.slice(mainFinallyIdx, mainFinallyIdx + 2000);
  if (mainFinallyIdx < 0) {
    violations.push('C11 找不到主 try 的 finally 段（判据取不到即红，不在空集合上判绿）');
  } else {
    if (!/@\('emu', 'kill'\)/.test(mainFinallySeg)) violations.push('C11 收尾的 emu kill 被挪出了主 finally 段');
    if (!/@\('wait-for-disconnect'\)/.test(mainFinallySeg)) violations.push('C11 收尾的 wait-for-disconnect 被挪出了主 finally 段');
  }
  // ⑥ 六档现值打进汇总：事后能从日志核对「当时生效的是哪一档」
  if (!/ADB_TIER_BUDGET[^\n]*install=\$AdbInstallTimeoutSeconds/.test(smokeBare) ||
      !/ADB_TIER_BUDGET[^\n]*logcat=\$AdbLogcatTimeoutSeconds/.test(smokeBare)) {
    violations.push('C11 汇总缺 ADB_TIER_BUDGET 六档读数（分不清「预算给小了」与「设备真挂了」）');
  }

  return violations;
}

describe('仿真机前置冒烟契约（#883 / O2，非门辅助）', () => {
  const real = {
    smoke: readSource(SMOKE_REL),
    setup: readSource(SETUP_REL),
    adr: readSource(ADR_REL)
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    /**
     * 存在型注入必须**先断言替换真的发生**：锚点会随代码措辞漂移，锚点一漂，用例就变成恒绿的空跑
     * （本仓口径见 utils/wirelessDebugContract.test.js 的 REMOVALS_EXTRA 与本文件 C3/C9 的 anchor 检查）。
     */
    const cut = (s, from, to, rule) => {
      if (!s.smoke.includes(from)) { throw new Error(rule + ' 注入锚点已失效，请同步更新本用例 :: ' + from); }
      const m = s.smoke.replace(from, to);
      if (m === s.smoke) { throw new Error(rule + ' 注入未改动文本 :: ' + from); }
      return { ...s, smoke: m };
    };
    /** 同一形状出现在委托链两段时，只摘一段另一段还接得住 ⇒ 必须全摘；minCount 锁「锚点确实出现这么多次」。 */
    const cutAll = (s, from, to, minCount, rule) => {
      const n = s.smoke.split(from).length - 1;
      if (n < minCount) { throw new Error(rule + ' 注入锚点出现 ' + n + ' 次（要求 ≥' + minCount + '）：' + from); }
      return { ...s, smoke: s.smoke.split(from).join(to) };
    };
    const cases = [
      ['C1', '头部标注被删', (s) => ({ ...s, smoke: s.smoke.replace(/非门（不替代 ① 真机门）[^\n]*/g, '') })],
      ['C1', '日志首行未写标注', (s) => ({ ...s, smoke: s.smoke.replace('Set-Content -LiteralPath $LogPath -Value ("[emulator-smoke] " + $NonGateBanner)', '# x') })],
      ['C2', '拿退出码当判据', (s) => ({ ...s, smoke: s.smoke + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' })],
      ['C3', '主 try 的 finally 被注掉', (s) => {
        const anchor = "    $script:FinalStatus = 'UNUSABLE'\n} finally {";
        const replacement = "    $script:FinalStatus = 'UNUSABLE'\n} # finally {";
        if (!s.smoke.includes(anchor)) { throw new Error('C3 注入点已失效，请同步更新本用例'); }
        return { ...s, smoke: s.smoke.replace(anchor, replacement) };
      }],
      ['C4', '混入门证据标记', (s) => ({ ...s, smoke: s.smoke + '\n# <!-- gate-evidence:④ -->\n' })],
      ['C4', '-PostToPr 被删', (s) => ({ ...s, smoke: s.smoke.replace(/PostToPr/g, 'PostIt') })],
      ['C5', '截图目录被改走', (s) => ({ ...s, smoke: s.smoke.replace(/\.ci-verify/g, '.tmp') })],
      ['C6', 'ADR 未记录非门辅助', (s) => ({ ...s, adr: s.adr.replace(/非门辅助：仿真机前置冒烟/g, '仿真机说明') })],
      ['C6', 'ADR 代填执行人', (s) => ({ ...s, adr: s.adr + '\n执行人：alice\n' })],
      ['C7', 'SDK 路径挪回带空格的系统盘', (s) => ({ ...s, setup: s.setup.replace(/D:\\android-sdk/g, 'C:\\android sdk') })],
      // C9（#1562）：每条都对应一种「把单次超时又拆掉」的真实改法
      ['C9', '有界执行器的 dot-source 被删（回到本地无界调用）', (s) => ({ ...s, smoke: s.smoke.replace(/.*lib\\auto-screenshot\.ps1'\).*/m, '# x') })],
      ['C9', '单次预算参数失去默认值', (s) => ({ ...s, smoke: s.smoke.replace('[int]$AdbCallTimeoutSeconds = 15', '[int]$AdbCallTimeoutSeconds') })],
      ['C9', '旧无界形状被回写（cmd.exe /c + 内层 >）', (s) => ({ ...s, smoke: s.smoke + '\n$cmd = \'x exec-out screencap -p > y\'; & cmd.exe /c $cmd 2>&1 | Out-Null\n' })],
      ['C9', '「成功才归位」被删（半张图直接落在出图判据看的名字上）', (s) => ({ ...s, smoke: s.smoke.replace('Move-Item -LiteralPath $partPath -Destination $path -Force', '# 归位被删') })],
      ['C9', '超时判据被降级成「看文件在不在」', (s) => ({ ...s, smoke: s.smoke.replace('if ($shot.TimedOut) {', 'if ($false) {') })],
      ['C9', '超时不再计入该页失败（挂死被读成「跑过了」）', (s) => ({ ...s, smoke: s.smoke.replace('if ($shot.TimedOut) { $pageFail +=', 'if ($shot.TimedOut) { $skips +=') })],
      ['C9', 'SHOT_CALL_TIMEOUT 机检行被删', (s) => ({ ...s, smoke: s.smoke.replace(/SHOT_CALL_TIMEOUT/g, 'SHOT_TIMEOUT') })],
      // C10（#1568，AC 第 1 条；AC 第 2 条后委托链是两段，锚点随点位改过一次 —— cut 会先证明锚点还在）
      ['C10', '委托被删、回写成 `& $AdbExe … | Out-String`', (s) => cut(s, 'Invoke-BoundedAdbText -AdbExe $AdbExe -Serial $adbSerial -AdbArguments $AdbArgs -AdbArgv $AdbArgs', '& $AdbExe -s $Serial @AdbArgs | Out-String', 'C10')],
      ['C10', 'serial 恒带（server 级命令被换判据）', (s) => cut(s, "$adbSerial = $(if ($NoSerial) { '' } else { $Serial })", '$adbSerial = $Serial', 'C10')],
      ['C10', 'NoSerial 透传断了', (s) => cut(s, ' -NoSerial:$NoSerial', '', 'C10')],
      ['C10', '档位默认值丢失（预算不再是参数；两段都要摘，只摘一段另一段还接得住）', (s) => cutAll(s, '[int]$BudgetSeconds = $AdbTextCallTimeoutSeconds', '[int]$BudgetSeconds', 2, 'C10')],
      ['C10', '预算写死在执行处（透传断了）', (s) => cut(s, '-TimeoutSeconds $BudgetSeconds', '-TimeoutSeconds 15', 'C10')],
      ['C10', 'stderr 合并被摘掉（判据原料静默换掉）', (s) => cut(s, ' -MergeStdErr ', ' ', 'C10')],
      ['C10', '直启恒开（Windows 那条已验证载体被换掉）', (s) => cut(s, '-DirectExec:(-not $IsWindows)', '-DirectExec:$true', 'C10')],
      ['C10', '超时点名行被改名', (s) => ({ ...s, smoke: s.smoke.replace(/ADB_TEXT_TIMEOUT/g, 'ADB_TEXT_TMO') })],
      ['C10', '超时点名行丢了档位', (s) => cut(s, ' seconds=$($r.Seconds) tier=$Tier ', ' seconds=$($r.Seconds) ', 'C10')],
      ['C10', '取文本档失去默认值', (s) => cut(s, '[int]$AdbTextCallTimeoutSeconds = 15', '[int]$AdbTextCallTimeoutSeconds', 'C10')],
      ['C10', '超时不再回空串（改成回一句假数据）', (s) => cut(s, "if ($r.TimedOut) { return '' }", "if ($r.TimedOut) { return 'unknown' }", 'C10')],
      ['C10', '超时清单不再累计', (s) => cut(s, '$script:AdbTextTimeouts = @($script:AdbTextTimeouts)', '$script:AdbTextTimeouts = @()', 'C10')],
      ['C10', '汇总读数行被删', (s) => ({ ...s, smoke: s.smoke.replace(/ADB_TEXT_CALL_BUDGET/g, 'ADB_TEXT_BUDGET_X') })],
      // C11（#1568，AC 第 2 条）：每条都对应一种「分档又被拆回去」的真实改法
      ['C11', '直调形状被回写（清零锁破）', (s) => ({ ...s, smoke: s.smoke + '\n    $z = & $AdbExe -s $Serial shell wm size 2>&1 | Out-String\n' })],
      ['C11', 'install 档被塞进文本档', (s) => cut(s, '[int]$AdbInstallTimeoutSeconds = 180', '[int]$AdbInstallTimeoutSeconds = 15', 'C11')],
      ['C11', 'push 档参数被改名', (s) => cut(s, '[int]$AdbPushTimeoutSeconds = 120,', '[int]$AdbPushBudgetX = 120,', 'C11')],
      ['C11', 'install 点位没接自己的档', (s) => cut(s, "@('install', '-r', '-t', $BaseApk) -BudgetSeconds $AdbInstallTimeoutSeconds -Tier 'install'", "@('install', '-r', '-t', $BaseApk) -Tier 'install'", 'C11')],
      ['C11', 'version 的 -NoSerial 丢了', (s) => cut(s, "@('version') -NoSerial -Tier 'server'", "@('version') -Tier 'server'", 'C11')],
      ['C11', 'Wait-ForDeviceBounded 的定义被拿掉', (s) => cut(s, 'function Wait-ForDeviceBounded {', '# function Wait-ForDeviceBounded {', 'C11')],
      ['C11', 'Wait-ForDeviceBounded 定义了却没被调', (s) => cut(s, '\n    Wait-ForDeviceBounded\n', '\n', 'C11')],
      ['C11', 'logcat 超时不再计入该页失败', (s) => cut(s, 'if ($logcat.TimedOut) { $pageFail +=', 'if ($logcat.TimedOut) { $skips +=', 'C11')],
      ['C11', 'Get-LogcatSummary 不带出 TimedOut', (s) => cut(s, 'TimedOut        = [bool]$r.TimedOut', 'TimedOut        = $false', 'C11')],
      ['C11', 'LOGCAT_INCONCLUSIVE 被删', (s) => ({ ...s, smoke: s.smoke.replace(/LOGCAT_INCONCLUSIVE/g, 'LOGCAT_X') })],
      ['C11', 'emu kill 没接 teardown 档', (s) => cut(s, "@('emu', 'kill') -BudgetSeconds $AdbTeardownTimeoutSeconds -Tier 'teardown'", "@('emu', 'kill') -Tier 'teardown'", 'C11')],
      ['C11', 'wait-for-disconnect 被挪出主 finally', (s) => cut(s, "try { $null = Get-AdbOutput @('wait-for-disconnect') -BudgetSeconds $AdbTeardownTimeoutSeconds -Tier 'teardown' } catch { }", 'try { } catch { }', 'C11')],
      ['C11', 'ADB_TIER_BUDGET 汇总被删', (s) => ({ ...s, smoke: s.smoke.replace(/ADB_TIER_BUDGET/g, 'ADB_X_TIER') })]
    ];
    cases.forEach(([rule, label, mutate]) => {
      const found = scanContract(mutate(real));
      expect({ label, hit: found.some((v) => v.startsWith(rule)) }).toEqual({ label, hit: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('非门标注在脚本头部、日志首行、PR 评论三处都在', () => {
    const occurrences = (real.smoke.match(/非门（不替代 ① 真机门）/g) || []).length;
    expect(occurrences).toBeGreaterThanOrEqual(3);
    expect(real.smoke).toContain('$NonGateBanner');
  });

  it('收尾在 finally 里，且用 adb emu kill', () => {
    // 判据只取**代码段**（掩掉 <# ... #> 文档块），并锚在主 catch 之后的 finally
    const code = maskDocBlocks(real.smoke);
    const mainCatch = code.indexOf('脚本异常');
    const at = mainCatch < 0 ? -1 : code.indexOf('} finally {', mainCatch);
    expect(mainCatch).toBeGreaterThan(-1);
    expect(at).toBeGreaterThan(-1);
    expect(code.indexOf('emu kill', at)).toBeGreaterThan(at);
  });

  it('PR 标记是 prefilter:emulator-smoke，且全文无 gate-evidence:', () => {
    const code = maskDocBlocks(real.smoke);
    expect(code).toContain('prefilter:emulator-smoke');
    expect(code.includes('gate-evidence:')).toBe(false);
  });

  it('ADR-0008 记明「不是门/不替代 ①/抓得到/抓不到」与装机成本', () => {
    expect(real.adr).toContain('非门辅助：仿真机前置冒烟');
    expect(real.adr).toContain('不替代 ①');
    expect(real.adr).toContain('抓得到');
    expect(real.adr).toContain('抓不到');
    expect(real.adr).toContain('android-sdk-setup.ps1');
  });
});


