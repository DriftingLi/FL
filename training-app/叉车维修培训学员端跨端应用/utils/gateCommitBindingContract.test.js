/**
 * 接线守护（#1695）：贴 sha 绑定评论的门脚本必须统一走 `scripts/lib/gate-commit.ps1` 的口径 ——
 * 「评论里的 `- commit:` 只绑 GitHub 点得开的提交，绑不上就如实标注」。
 *
 * ⚠️ 本文件是**接线层**（只读源码文本断言）：它钉「接线没被摘掉 / 没有回到各写一份」这件事，
 *    **不构成 ③ 门的行为证据**（见 docs/agents/guards.md）。行为证据在
 *    `utils/gateCommitBindingBehavior.test.js`：那里真建 bare 远端 + monorepo 形状工作树，真跑
 *    `Test-ShaReachable` / `Push-ArchiveCommit` / `Resolve-CommitBinding`，成对取证（必不红 + 必红）。
 *
 * 为什么要有这一层：口径的承重点不在某一个脚本里，而在「**所有**产出证据块的脚本都走同一处」——
 * 只加行为守护的话，将来有人新增一个门、或在某个脚本里自己算一遍 sha（第二真源），行为守护照样全绿。
 *
 * #1841 追加（K8–K11）→ **A1（#2100）起大半已退役**：
 *   - 404 降级判据（`Get-CommitCommentKind`）随投递一起删；「六处必须消费降级三字段」的要求也已撤（见 K9 注释）——
 *     投递退役后那三支**恒假**，还要求调用方处理一条永不发生的分支只会制造误导。
 *   - 调用方的「降级 / 失败 / 成功」三支**已于 2026-10-29 删除**，只留一行 Notes 回声。
 *   - 仍然有效的：K8（库内不得再出现任何评论写请求）、K10（返回形状一个都不能少）、
 *     K11（库与调用方都不得因投递失败 exit —— #1209 血账）。
 */
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/gate-commit.ps1';

/** 凡贴 sha 绑定评论（`- commit: <sha>`）的脚本 —— 门与同族取证脚本。新增此类脚本必须进本清单。 */
const BINDING_CALLERS = [
  'scripts/mp-weixin-check.ps1', // ② 微信开发者工具门（-PostToPr）
  'scripts/device-capture.ps1', // ①a 真机逐页取证（prefilter 评论）
  'scripts/dev-finish.ps1', // ③ 全量单测门
  'scripts/compile-check.ps1', // ④a 编译门
  'scripts/kotlin-all-check.ps1', // ④c kotlin all 门
  'scripts/emulator-smoke.ps1', // 仿真机冒烟（非门，同族形态）
];

/** 会产生「本地归档提交」的脚本：必须有真推这一步（推不动由 lib 如实回报，不许假装成功）。 */
const ARCHIVE_CALLERS = [
  'scripts/mp-weixin-check.ps1',
  'scripts/device-capture.ps1',
  'scripts/emulator-smoke.ps1',
];

const LIB_FUNCS = ['Test-ShaReachable', 'Push-ArchiveCommit', 'Resolve-CommitBinding'];

const countDefs = (text, name) =>
  (text.match(new RegExp('function\\s+' + name + '\\s*\\{', 'g')) || []).length;

function must(violations, ok, code, message) {
  if (!ok) violations.push(`${code}: ${message}`);
}

/**
 * 去注释后的代码文本：注释里允许解释判据，判据本身只准在实现里出现一次。
 * （只处理 PS 的块注释与整行注释；调用方注释风格稳定，行为测试另钉真实输出。）
 */
function stripComments(text) {
  return text
    .replace(/<#[\s\S]*?#>/g, '')
    .split(/\r?\n/)
    .filter((line) => !/^\s*#/.test(line))
    .join('\n');
}

/** 判据：给定各文件文本，返回违规清单（空数组 = 符合口径）。抽出成纯函数，下面用它跑注入自检。 */
function scanContract(sources) {
  const violations = [];
  const lib = sources.lib;

  // K1：口径只有一份 —— lib 里三个成员各自恰好定义一次，且**不**自定义 sha 取法（那是 gate-common 的活）
  LIB_FUNCS.forEach((name) => {
    must(violations, countDefs(lib, name) === 1, 'K1', `${LIB_REL} 必须恰好定义一次 function ${name}`);
  });
  must(
    violations,
    !/function\s+Get-HeadSha\s*\{/.test(lib),
    'K1',
    `${LIB_REL} 不得自定义 Get-HeadSha（sha 取法单一真源在 lib/gate-common.ps1）`,
  );

  sources.callers.forEach(({ rel, text }) => {
    // K2：接线 —— dot-source + 调 Resolve-CommitBinding，并把评论里的 sha 换成 binding 给的那个
    must(
      violations,
      /lib[\\/]gate-commit\.ps1/.test(text),
      'K2',
      `${rel} 必须 dot-source ${LIB_REL}（否则口径没接上）`,
    );
    must(
      violations,
      /Resolve-CommitBinding\s+-ProjectDir\s+\S+\s+-HeadSha\s+\S+/.test(text),
      'K2',
      `${rel} 必须调 Resolve-CommitBinding -ProjectDir <dir> -HeadSha <sha>`,
    );
    must(
      violations,
      /\$sha\s*=\s*\$binding\.Sha/.test(text),
      'K2',
      `${rel} 必须把评论里的 $sha 换成 $binding.Sha（否则算出来的绑定没用上）`,
    );
    // K3：pr-evidence 的绑定位仍在（改这一行会让校验器整段读不到 commit）
    must(
      violations,
      text.includes('- commit: $sha'),
      'K3',
      `${rel} 评论正文必须保留字面量 \`- commit: $sha\`（pr-evidence 靠它取 sha）`,
    );
    // K5：不许出现第二份 sha 取法（#1695 的「各写一份」正是缺陷来源）
    must(
      violations,
      !/function\s+Get-HeadSha\s*\{/.test(text),
      'K5',
      `${rel} 不得再定义 Get-HeadSha（sha 取法单一真源）`,
    );
  });

  ARCHIVE_CALLERS.forEach((rel) => {
    const text = (sources.callers.find((c) => c.rel === rel) || {}).text || '';
    // K4：归档提交必须真推（推得动就推；推不动由 lib 回报，由调用方如实标注）
    must(
      violations,
      /Push-ArchiveCommit\s+-ProjectDir\s+\S+\s+-Branch\s+\S+/.test(text),
      'K4',
      `${rel} 的归档提交必须调 Push-ArchiveCommit -ProjectDir <dir> -Branch <branch>`,
    );
  });

  // K6（#1770）：-PostToIssue 是正式名，-PostToPr 保留为别名（同值转发）——
  // 删掉别名 = 旧调用点（draft 里的 pwsh 命令、外部会话）当场静默丢证据，而门脚本仍报绿。
  sources.callers.forEach(({ rel, text }) => {
    must(
      violations,
      /\[int\]\$PostToIssue\s*=\s*0/.test(text),
      'K6',
      `${rel} 缺少正式参数 [int]$PostToIssue`,
    );
    must(
      violations,
      /if\s*\(\$PostToIssue\s*-gt\s*0\)\s*\{\s*\$PostToPr\s*=\s*\$PostToIssue\s*\}/.test(text),
      'K6',
      `${rel} 必须把 -PostToIssue 转给 -PostToPr（旧名别名）`,
    );
  });

  // K7（#1770）：判据评论必须贴 commit（Send-GateComment），不许再回到 `gh pr comment`
  // （PR 号做键 ⇒ 证据只能出现在开 PR 之后 ⇒ 首推必红）。
  must(violations, countDefs(lib, 'Send-GateComment') === 1, 'K7', `${LIB_REL} 必须恰好定义一次 function Send-GateComment`);
  must(violations, countDefs(lib, 'Get-GateRepoSlug') === 1, 'K7', `${LIB_REL} 必须恰好定义一次 function Get-GateRepoSlug`);
  sources.callers.forEach(({ rel, text }) => {
    must(
      violations,
      /Send-GateComment\s+-ProjectDir\s+\S+\s+-Sha\s+\S+\s+-Reachable/.test(text),
      'K7',
      `${rel} 的门评论必须走 Send-GateComment -ProjectDir <dir> -Sha <sha> -Reachable <bool>`,
    );
    must(violations, !/gh pr comment/.test(text), 'K7', `${rel} 又出现了 gh pr comment（判据评论必须贴 commit）`);
  });

  // K8（A1，#2100 **反回流**）：投递已退役，库内不得再出现任何评论写请求。
  // 这是本票的主判据：退役之后谁把「贴评论」加回来，必须先让这条红 ——
  // 否则会出现「证在评论上、判据只看正文」的半退役形态（比全旧或全新都难查）。
  // `-GhCli` 签名保留（调用方零改动），默认仍须是生产路径的 'gh'。
  const libCode = stripComments(lib);
  must(violations, !/api\s+--method\s+POST/.test(libCode), 'K8', `${LIB_REL} 又出现了评论投递（A1 起证据只走 PR 正文一行）`);
  must(violations, !/\/comments/.test(libCode), 'K8', `${LIB_REL} 又出现了 /comments 端点（投递已退役，#2100）`);
  must(
    violations,
    /\[string\]\$GhCli\s*=\s*'gh'/.test(libCode),
    'K8',
    `${LIB_REL} 的 -GhCli 必须默认 'gh'（签名兼容保留，别让注桩改掉默认）`,
  );

  // K9（A1，#2100）：**原 K9「六个调用方必须消费降级三字段」已删除**。
  // 降级三字段（CommitUnavailable / EvidenceUrl / CommitKind=failed）随投递一起退役，
  // 继续要求调用方消费它们等于逼着代码去处理一条永不发生的分支。
  // 调用方里那些分支暂留一版（改动面收在库内，五个脚本零改动 ⇒ 不拿日常 dev:finish 冒险），
  // 它们收到的永远是 CommitUnavailable=$false / CommitKind='retired'，不会误报。
  // 清理调用方的死分支是独立小票，不塞进本次落库。

  // K10（A1）：返回值形状一个都不能少 —— 调用方靠这些字段分流；少一个 = 旧调用点取到 $null。
  // A1 后 CommitKind 恒 'retired'、CommitOk/IssueOk 恒 false、Errors 恒空、EvidenceUrl 恒空
  // （退役不是失败，别让调用方打红字）；新增 Printed / Body 供「块已打印」可用性判断。
  const RETURN_FIELDS = [
    ['CommitOk', /CommitOk\s*=\s*\$false/],
    ['CommitUrl', /CommitUrl\s*=\s*''/],
    ['IssueOk', /IssueOk\s*=\s*\$false/],
    ['IssueUrl', /IssueUrl\s*=\s*''/],
    ['Errors', /Errors\s*=\s*@\(\)/],
    ['CommitKind', /CommitKind\s*=\s*'retired'/],
    ['CommitUnavailable', /CommitUnavailable\s*=\s*\$false/],
    ['EvidenceUrl', /EvidenceUrl\s*=\s*''/],
    ['Notes', /Notes\s*=\s*@\(\$notes\)/],
    ['Printed', /Printed\s*=\s*\$printed/],
    ['Body', /Body\s*=\s*\$Body/],
  ];
  // 只在**返回值块**里判形状：`.OUTPUTS` 的文档注释里也写着同样的 `EvidenceUrl = ''`，
  // 对整个文件做正则会把注释当成代码（注入用例因此漏判过一次）。
  const sendAt = libCode.indexOf('function Send-GateComment');
  const returnAt = sendAt >= 0 ? libCode.indexOf('return [pscustomobject]@{', sendAt) : -1;
  const returnBlock = returnAt >= 0 ? libCode.slice(returnAt) : '';
  RETURN_FIELDS.forEach(([name, re]) => {
    must(violations, re.test(returnBlock), 'K10', `${LIB_REL} 的 Send-GateComment 返回值缺少 ${name}`);
  });

  // K11（#1209 血账）：库与调用方都不得把「评论投递失败」变成门退出码 ——
  // 库不许 exit；调用方从 Send-GateComment 到 catch 之间不许 exit。
  must(violations, !/\bexit\s+[1-9]/.test(libCode), 'K11', `${LIB_REL} 是库，不得 exit（投递失败不改门结论）`);
  sources.callers.forEach(({ rel, text }) => {
    const callAt = text.indexOf('$sent = Send-GateComment');
    const catchAt = callAt >= 0 ? text.indexOf('} catch {', callAt) : -1;
    must(
      violations,
      callAt >= 0 && catchAt > callAt && !/\bexit\s+[1-9]/.test(text.slice(callAt, catchAt)),
      'K11',
      `${rel} 的评论投递区间不得 exit（#1209：报告层崩了不许把门通过改成失败）`,
    );
  });

  return violations;
}

function readAll() {
  return {
    lib: readText(path.join(ROOT, LIB_REL)),
    callers: BINDING_CALLERS.map((rel) => ({ rel, text: readText(path.join(ROOT, rel)) })),
  };
}

describe('门评论 sha 绑定口径接线（#1695）', () => {
  test('K0: 真实文件符合口径（违规清单为空）', () => {
    expect(scanContract(readAll())).toEqual([]);
  });

  test('K1–K11 注入违规必须被检出（防「恒绿」假守护）', () => {
    const real = readAll();
    const patchCaller = (rel, fn) => ({
      ...real,
      callers: real.callers.map((c) => (c.rel === rel ? { ...c, text: fn(c.text) } : c)),
    });
    const cases = [
      ['K1', { ...real, lib: real.lib.replace(/function\s+Resolve-CommitBinding\s*\{/, 'function ResolveBindingX {') }],
      ['K1', { ...real, lib: real.lib + "\nfunction Get-HeadSha { param([string]$ProjectDir) return '' }\n" }],
      ['K2', patchCaller('scripts/compile-check.ps1', (t) => t.replace(/lib[\\/]gate-commit\.ps1/g, 'lib\\gate-other.ps1'))],
      ['K2', patchCaller('scripts/dev-finish.ps1', (t) => t.replace(/Resolve-CommitBinding/g, 'ResolveBindingX'))],
      ['K2', patchCaller('scripts/kotlin-all-check.ps1', (t) => t.replace(/\$sha\s*=\s*\$binding\.Sha/, '$sha = $sha') )],
      ['K3', patchCaller('scripts/mp-weixin-check.ps1', (t) => t.replace(/- commit: \$sha/g, '- sha: $sha'))],
      ['K4', patchCaller('scripts/device-capture.ps1', (t) => t.replace(/Push-ArchiveCommit/g, 'PushArchiveX'))],
      ['K5', patchCaller('scripts/emulator-smoke.ps1', (t) => t + "\nfunction Get-HeadSha { param([string]$ProjectDir) return '' }\n")],
      // #1770：旧名别名被删 / 门评论退回 gh pr comment / lib 的口径函数被改名，都必须被抓
      ['K6', patchCaller('scripts/dev-finish.ps1', (t) => t.replace(/if \(\$PostToIssue -gt 0\) \{ \$PostToPr = \$PostToIssue \}/, ''))],
      ['K7', patchCaller('scripts/compile-check.ps1', (t) => t.replace(/Send-GateComment/, 'gh pr comment -x'))],
      ['K7', { ...real, lib: real.lib.replace(/function Send-GateComment \{/, 'function SendGateX {') }],
      // A1（#2100）：把「贴评论」加回来 / -GhCli 默认被改 → 必须抓
      ['K8', { ...real, lib: real.lib + "\n$null = & gh api --method POST repos/o/r/commits/x/comments --input t.json\n" }],
      ['K8', { ...real, lib: real.lib.replace(/\[string\]\$GhCli\s*=\s*'gh'/, '[string]$GhCli') }],
      // 注入要打**代码里那一处**：`.OUTPUTS` 文档注释里也有同样的 `EvidenceUrl = ''`，
      // 改第一处只会改到注释（这条用例因此漏判过一次）。取最后一处 = return 块内的赋值。
      ['K10', (() => {
        const re = /EvidenceUrl\s*=\s*''/g;
        const hits = [...real.lib.matchAll(re)];
        const last = hits[hits.length - 1];
        return {
          ...real,
          lib: real.lib.slice(0, last.index) + 'EvidenceUrlX = $evidenceUrl' + real.lib.slice(last.index + last[0].length),
        };
      })()],
      ['K10', { ...real, lib: real.lib.replace(/Notes\s*=\s*@\(\$notes\)/, 'NotesX = @($notes)') }],
      ['K11', { ...real, lib: real.lib + '\nexit 1\n' }],
      ['K11', patchCaller('scripts/emulator-smoke.ps1', (t) => t.replace(
        // A1（#2100）：调用方三支恒假分支已删，锚点迁到 Send-GateComment 之后仍在的那一行（Notes 回声）。
        // 锚点若哪天又被删掉，`replace` 不命中 ⇒ 本 case 落进 missed ⇒ 下面那句 expect 直接红（不会静默失效）。
        'foreach ($note in @($sent.Notes)) { Write-Log $note }',
        'foreach ($note in @($sent.Notes)) { Write-Log $note }; exit 1',
      ))],
    ];
    // 一次报出**所有**没被抓到的注入（只报第一个的话，改一条又冒出下一条，来回跑很多轮）
    const missed = cases
      .filter(([code, patched]) => !scanContract(patched).some((m) => m.startsWith(code + ':')))
      .map(([code], i) => `${code}#${i}`);
    expect(missed).toEqual([]);
  });
});
