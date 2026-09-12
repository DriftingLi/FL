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
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/kotlinAllGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SMOKE_REL = 'scripts/emulator-smoke.ps1';
const SETUP_REL = 'scripts/android-sdk-setup.ps1';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';

const PR_MARKER = 'prefilter:emulator-smoke';
const GATE_MARKER = 'gate-evidence:';
const NON_GATE_BANNER = '非门（不替代 ① 真机门）';

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
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
      ['C7', 'SDK 路径挪回带空格的系统盘', (s) => ({ ...s, setup: s.setup.replace(/D:\\android-sdk/g, 'C:\\android sdk') })]
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


