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
 *      ⚠️ 只判**函数体**：本脚本另有 13 处直调仍无界（同票 AC 第 2 条的后续 PR），
 *      在这里下全局禁令会把「还没做完」读成「做错了」——那正是 #1568 开票时要防的那类漂移。
 *      行为面由 `emulatorSmokeBoundedTextBehavior.test.js`（ETD1–ETD5）另钉。
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
  const gaStart = smokeBare.indexOf('function Get-AdbOutput');
  const gaEnd = smokeBare.indexOf('function Assert-Environment');
  const gaBody = gaStart < 0 ? '' : smokeBare.slice(gaStart, gaEnd > gaStart ? gaEnd : smokeBare.length);
  if (gaStart < 0) {
    violations.push('C10 找不到 Get-AdbOutput 函数体（判据取不到即红，不在空集合上判绿）');
  } else {
    if (!/Invoke-BoundedAdbText -AdbExe \$AdbExe -Serial \$Serial/.test(gaBody)) {
      violations.push('C10 取文本没委托到唯一执行核（12 个调用方又回到无界等待）');
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
    if (/&\s*\$AdbExe/.test(gaBody)) violations.push('C10 函数体回写了 `& $AdbExe` 直调（无单次超时）');
    if (/Out-String/.test(gaBody)) violations.push('C10 函数体回写了 Out-String 取文本（父进程读子进程输出段 = #1285 那条挂死路径）');
    if (!/return\s*''/.test(gaBody)) violations.push('C10 超时没回空串（既有语义「失败一律返回空串」不许改，调用方各按自己的分支判）');
    if (!/ADB_TEXT_TIMEOUT call=/.test(smokeBare)) violations.push('C10 缺超时点名行（哪一次调用、多大预算必须写出来）');
    if (!/\[int\]\$AdbTextCallTimeoutSeconds\s*=\s*15/.test(smokeBare)) violations.push('C10 缺 -AdbTextCallTimeoutSeconds 默认 15 秒的档位参数');
    if (!/\$script:AdbTextTimeouts\s*=\s*@\(\$script:AdbTextTimeouts\)/.test(smokeBare)) violations.push('C10 缺超时清单累计（汇总点不出「本次挂过几次」）');
    if (!/ADB_TEXT_CALL_BUDGET calls=/.test(smokeBare)) violations.push('C10 缺取文本那档的汇总读数行（分不清 adb 通道挂了与设备答了个空）');
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
      // C10（#1568）：每条都对应一种「把文本收口点的单次超时又拆掉」的真实改法
      ['C10', '委托被删、回写成 `& $AdbExe … | Out-String`', (s) => ({ ...s, smoke: s.smoke.replace('Invoke-BoundedAdbText -AdbExe $AdbExe -Serial $Serial -AdbArguments $AdbArgs -AdbArgv $AdbArgs', '& $AdbExe -s $Serial @AdbArgs | Out-String') })],
      ['C10', '档位默认值丢失（预算不再是参数）', (s) => ({ ...s, smoke: s.smoke.replace('[int]$BudgetSeconds = $AdbTextCallTimeoutSeconds', '[int]$BudgetSeconds') })],
      ['C10', '预算写死在执行处（透传断了）', (s) => ({ ...s, smoke: s.smoke.replace('-TimeoutSeconds $BudgetSeconds', '-TimeoutSeconds 15') })],
      ['C10', 'stderr 合并被摘掉（判据原料静默换掉）', (s) => ({ ...s, smoke: s.smoke.replace(' -MergeStdErr ', ' ') })],
      ['C10', '直启恒开（Windows 那条已验证载体被换掉）', (s) => ({ ...s, smoke: s.smoke.replace('-DirectExec:(-not $IsWindows)', '-DirectExec:$true') })],
      ['C10', '超时点名行被改名', (s) => ({ ...s, smoke: s.smoke.replace(/ADB_TEXT_TIMEOUT/g, 'ADB_TEXT_TMO') })],
      ['C10', '取文本档失去默认值', (s) => ({ ...s, smoke: s.smoke.replace('[int]$AdbTextCallTimeoutSeconds = 15', '[int]$AdbTextCallTimeoutSeconds') })],
      ['C10', '超时不再回空串（改成回一句假数据）', (s) => ({ ...s, smoke: s.smoke.replace(/return ''(\r?\n    \}\r?\n    return \(\[string\]\$r\.Text\))/, "return 'unknown'$1") })],
      ['C10', '超时清单不再累计', (s) => ({ ...s, smoke: s.smoke.replace('$script:AdbTextTimeouts = @($script:AdbTextTimeouts)', '$script:AdbTextTimeouts = @()') })],
      ['C10', '汇总读数行被删', (s) => ({ ...s, smoke: s.smoke.replace(/ADB_TEXT_CALL_BUDGET/g, 'ADB_TEXT_BUDGET_X') })]
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


