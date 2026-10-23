/**
 * 接线守护（#1695）：贴 sha 绑定评论的门脚本必须统一走 `scripts/lib/gate-commit.ps1` 的口径 ——
 * 「评论里的 `- commit:` 只绑 GitHub 点得开的提交，绑不上就如实标注」。
 *
 * ⚠️ 本文件是**接线层**（只读源码文本断言）：它钉「接线没被摘掉 / 没有回到各写一份」这件事，
 *    **不构成 ③ 门的行为证据**（见 docs/agents/guards.md）。行为证据在
 *    `utils/gateCommitBindingBehavior.test.js`：那里真建 bare 远端 + monorepo 形状工作树，真跑
 *    `Test-ShaReachable` / `Push-ArchiveCommit` / `Resolve-CommitBinding`，成对取证（必不红 + 必红）。
 *
 * 为什么要有这一层：口径的承重点不在某一个脚本里，而在「**所有**贴 sha 绑定评论的脚本都走同一处」——
 * 只加行为守护的话，将来有人新增一个门、或在某个脚本里自己算一遍 sha（第二真源），行为守护照样全绿。
 *
 * #1841 追加（K8–K11）：commit 评论端点 404 的降级必须**一处判定、六处消费** ——
 *   404 判据只准在 `Get-CommitCommentKind`；六个调用方必须消费 CommitKind / CommitUnavailable /
 *   EvidenceUrl / Notes 并打印 `GATE_COMMENT_DEGRADED`；返回字段一个都不能少；库与调用方都不得
 *   因投递失败 exit（#1209 血账）。
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

  // K8（#1841）：404 判据只有一处真源 —— `Get-CommitCommentKind`；Send-GateComment 只调它，
  // 调用方不解析 gh 的错误文本。`-GhCli` 只为测试注桩，默认必须还是生产路径的 'gh'。
  const libCode = stripComments(lib);
  const kindFnAt = libCode.indexOf('function Get-CommitCommentKind');
  const sendFnAt = libCode.indexOf('function Send-GateComment', Math.max(kindFnAt, 0));
  // 判据的**代码形态**是 `HTTP 404\)`（正则字面量里右括号被转义）；Notes 里的人读文案是
  // 全角「（HTTP 404）」，不算判据 —— 所以这里数的是转义形态，不是裸文本。
  const kindHits = (libCode.match(/HTTP 404\\\)/g) || []).length;
  must(violations, countDefs(lib, 'Get-CommitCommentKind') === 1, 'K8', `${LIB_REL} 必须恰好定义一次 function Get-CommitCommentKind`);
  must(
    violations,
    kindFnAt >= 0 && sendFnAt > kindFnAt && kindHits === 1
      && libCode.indexOf('HTTP 404\\)') > kindFnAt && libCode.indexOf('HTTP 404\\)') < sendFnAt,
    'K8',
    `${LIB_REL} 的 404 判据必须只出现在 Get-CommitCommentKind 内（调用方与 Send-GateComment 不得自己看 gh 错误文本）`,
  );
  must(
    violations,
    /Get-CommitCommentKind\s+-Output\s+\$out1\s+-ExitCode\s+\$LASTEXITCODE/.test(libCode),
    'K8',
    `${LIB_REL} 的 Send-GateComment 必须用 Get-CommitCommentKind -Output $out1 -ExitCode $LASTEXITCODE 分类`,
  );
  must(
    violations,
    /\[string\]\$GhCli\s*=\s*'gh'/.test(libCode),
    'K8',
    `${LIB_REL} 的 -GhCli 必须默认 'gh'（测试注桩不得改生产默认路径）`,
  );

  // K9（#1841）：六个调用方必须消费降级三字段 + Notes，并打出机检行 / 人读行 / 硬失败行；
  // 404 文本不许在调用方二次解析（判据单一真源）。
  sources.callers.forEach(({ rel, text }) => {
    must(
      violations,
      /\$sent\.CommitUnavailable\s+-and\s+\$sent\.EvidenceUrl/.test(text),
      'K9',
      `${rel} 必须消费 $sent.CommitUnavailable + $sent.EvidenceUrl（降级分支）`,
    );
    must(
      violations,
      /GATE_COMMENT_DEGRADED tier=issue url=\$\(\$sent\.EvidenceUrl\)/.test(text),
      'K9',
      `${rel} 降级时必须打印机检行 GATE_COMMENT_DEGRADED tier=issue url=<EvidenceUrl>`,
    );
    must(
      violations,
      /证据主键降级：commit 评论不可用（HTTP 404）→ 落点 \$\(\$sent\.EvidenceUrl\)/.test(text),
      'K9',
      `${rel} 降级时必须打印人读落点行（URL 来自 $sent.EvidenceUrl）`,
    );
    must(
      violations,
      /\$sent\.CommitKind\s+-eq\s+'failed'/.test(text),
      'K9',
      `${rel} 必须区分硬失败（$sent.CommitKind -eq 'failed'）`,
    );
    must(violations, /GATE_COMMENT_FAILED/.test(text), 'K9', `${rel} 硬失败必须打印 GATE_COMMENT_FAILED`);
    must(
      violations,
      !/HTTP 404\\\)/.test(text),
      'K9',
      `${rel} 不得自己解析 gh 的 404 错误文本（判据只准在 Get-CommitCommentKind）`,
    );
    must(
      violations,
      /foreach\s*\(\$note\s+in\s+@\(\$sent\.Notes\)\)\s*\{\s*(Write-Host|Write-Log)\s+\$note\s*\}/.test(text),
      'K9',
      `${rel} 必须逐行输出 $sent.Notes（降级落点写进可见输出）`,
    );
  });

  // K10（#1841）：返回值字段一个都不能少（向后兼容 5 个 + 降级 4 个）——
  // 调用方靠这些字段分流；少一个 = 降级静默，或旧调用点取到 $null。
  const RETURN_FIELDS = [
    ['CommitOk', /CommitOk\s*=\s*\$commitOk/],
    ['CommitUrl', /CommitUrl\s*=\s*\$commitUrl/],
    ['IssueOk', /IssueOk\s*=\s*\$issueOk/],
    ['IssueUrl', /IssueUrl\s*=\s*\$issueUrl/],
    ['Errors', /Errors\s*=\s*@\(\$errors\)/],
    ['CommitKind', /CommitKind\s*=\s*\$commitKind/],
    ['CommitUnavailable', /CommitUnavailable\s*=\s*\$commitUnavailable/],
    ['EvidenceUrl', /EvidenceUrl\s*=\s*\$evidenceUrl/],
    ['Notes', /Notes\s*=\s*@\(\$notes\)/],
  ];
  RETURN_FIELDS.forEach(([name, re]) => {
    must(violations, re.test(libCode), 'K10', `${LIB_REL} 的 Send-GateComment 返回值缺少 ${name}`);
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
      // #1841：降级接线退回（第二真源 / 调用方不消费 / 字段删掉 / 库 exit）都必须抓
      ['K8', {
        ...real,
        lib: real.lib.replace(
          '$commitKind = Get-CommitCommentKind -Output $out1 -ExitCode $LASTEXITCODE',
          "$commitKind = if ($out1 -match 'HTTP 404\\)') { 'commit-unavailable' } else { Get-CommitCommentKind -Output $out1 -ExitCode $LASTEXITCODE }",
        ),
      }],
      ['K8', { ...real, lib: real.lib.replace(/\[string\]\$GhCli\s*=\s*'gh'/, '[string]$GhCli') }],
      ['K9', patchCaller('scripts/compile-check.ps1', (t) => t.replace(/\$sent\.CommitUnavailable\s+-and\s+\$sent\.EvidenceUrl/, '$sent.CommitOk'))],
      ['K9', patchCaller('scripts/mp-weixin-check.ps1', (t) => t.replace(/GATE_COMMENT_DEGRADED tier=issue url=\$\(\$sent\.EvidenceUrl\)/, 'GATE_COMMENT_DEGRADED'))],
      ['K9', patchCaller('scripts/dev-finish.ps1', (t) => t.replace(/证据主键降级：commit 评论不可用（HTTP 404）→ 落点 \$\(\$sent\.EvidenceUrl\)/, '降级'))],
      ['K9', patchCaller('scripts/kotlin-all-check.ps1', (t) => t.replace(/\$sent\.CommitKind\s+-eq\s+'failed'/, "$sent.CommitKind -eq 'broken'"))],
      ['K9', patchCaller('scripts/device-capture.ps1', (t) => t + "\n$null = 'HTTP 404\\)'\n")],
      ['K10', { ...real, lib: real.lib.replace(/EvidenceUrl\s*=\s*\$evidenceUrl/, 'EvidenceUrlX = $evidenceUrl') }],
      ['K10', { ...real, lib: real.lib.replace(/Notes\s*=\s*@\(\$notes\)/, 'NotesX = @($notes)') }],
      ['K11', { ...real, lib: real.lib + '\nexit 1\n' }],
      ['K11', patchCaller('scripts/emulator-smoke.ps1', (t) => t.replace(
        'Write-Log "GATE_COMMENT_FAILED reason=$gateCommentFailReason"',
        'Write-Log "GATE_COMMENT_FAILED reason=$gateCommentFailReason"; exit 1',
      ))],
    ];
    cases.forEach(([code, patched]) => {
      const found = scanContract(patched);
      expect(found.some((m) => m.startsWith(code + ':'))).toBe(true);
    });
  });
});
