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
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/emulatorSmokeContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

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

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
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
      ['D10', '汇总页身份行被删', (s) => ({ ...s, script: s.script.replace('页身份断言：', '备注：') })]
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