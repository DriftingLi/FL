/**
 * 串行轮转外层的两块判据真源 `scripts/lib/frontier.ps1` 的行为守护（运行期）
 *
 * 为什么需要这个文件（#1435）：
 *   「挂机跑一张移动端票」的那条链，仓里已经有了（`dev-finish.ps1` 的 ④c → 真运行 →
 *   只截改动页 → 证据 → 还原 json）。缺的只有外层：取票、起一次 headless agent、调 `dev-finish`。
 *   外层里最贵的两个判断恰好都是**纯逻辑**：
 *     ① 取票判错 ⇒ 跑了一张**被阻塞**的票（改中间态，返工整批）或抢了别人认领的票；
 *     ② 纪律清单写漏一条 ⇒ 无人值守的 agent 自己去碰 HBuilderX / 提交三份 json（kill 红线、
 *        凭空命中 ④b 云打包门）。
 *   本机可用的 agent CLI **没有任何权限档位** ⇒ 「agent 会跑哪些命令」不能靠约束它，
 *   只能靠**不交给它**（见 issue #1435 ②）。这份清单因此必须是判据、而不是提醒。
 *
 * 射程（刻意收窄，防越界断言）：
 *   本文件只 dot-source 纯函数，**不取票、不建树、不起 agent、不跑任何门**。
 *   副作用那一步的验收判据是「真跑一张票跑通」——它不能在这里假称，
 *   必须由带真机证据的那个 PR 给（#1435 ⑤「工具票的验收判据是它要达成的行为在真实链路上成立」）。
 *
 * 运行前提：需要 pwsh（PowerShell 7）。该前提不是假设 —— 同 CI job 里的
 * contractTestPatternBehavior.test.js 已在 ubuntu runner 上 dot-source pwsh 跑绿（先例 wtBootstrapBehavior）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'frontier.ps1');

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** dot-source 真源并执行一段脚本，stdout 按非空行返回。 */
function runPs(body) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    // 控制台输出编码必须强制 UTF-8：Windows 下默认跟随 OEM 代码页，CJK 会在 stdout 往返中被改写。
    'try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }',
    `. "${path.join(ROOT, LIB_REL)}"`,
    body,
  ].join('\n');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  const stdout = execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
    encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 8 * 1024 * 1024,
  });
  return String(stdout).split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
}

const kv = (lines, key) => lines.filter((l) => l.startsWith(key + '=')).map((l) => l.slice(key.length + 1));

/** 把一条候选票喂给判据，取回 verdict / reason。 */
function verdict(payload) {
  const json = JSON.stringify(payload).replace(/'/g, "''");
  const lines = runPs([
    `$j = '${json}' | ConvertFrom-Json`,
    '$v = Get-FrontierVerdict -Ticket $j',
    'Write-Output ("verdict=" + $v.Verdict)',
    'Write-Output ("reason=" + $v.Reason)',
  ].join('\n'));
  return { verdict: kv(lines, 'verdict')[0], reason: kv(lines, 'reason')[0] };
}

const base = {
  number: 1400,
  state: 'OPEN',
  labels: ['ready-for-mobile-agent'],
  assignees: [],
  blockedBy: [],
};

describe('串行轮转外层的判据真源（运行期）', () => {
  test('FR1: 取票判据 —— 正例与每一条反向', () => {
    expect(verdict(base).verdict).toBe('eligible');
    // 闭票不取（frontier 口径：open）
    expect(verdict({ ...base, state: 'CLOSED' }).reason).toBe('closed');
    // 没挂 `ready-for-mobile-agent` 不取（挂了 needs-triage 也不够 —— 那是前置步骤）
    expect(verdict({ ...base, labels: ['needs-triage'] }).reason).toBe('not_labelled');
    expect(verdict({ ...base, labels: [] }).reason).toBe('not_labelled');
    // 移动端面必须用 `ready-for-mobile-agent`；`ready-for-agent` 是后端/前端票的 label，
    // 用它取票会把外层的 agent 派到非移动端承载面上去。
    expect(verdict({ ...base, labels: ['ready-for-agent'] }).reason).toBe('not_labelled');
    // 已认领不取（一会话一票；抢别人在飞的票是 multi-agent-git 记的互踩形态）
    expect(verdict({ ...base, assignees: [{ login: 'someone' }] }).reason).toBe('assigned');
    // 阻塞未清不取
    expect(verdict({ ...base, blockedBy: [{ number: 1421, state: 'OPEN' }] }).reason).toBe('blocked');
    // 阻塞已解（CLOSED / MERGED）不挡路
    expect(verdict({ ...base, blockedBy: [{ number: 1421, state: 'CLOSED' }] }).verdict).toBe('eligible');
    expect(verdict({ ...base, blockedBy: [{ number: 1421, state: 'MERGED' }] }).verdict).toBe('eligible');
  });

  test('FR2: fail-closed —— blocked_by 读不出来一律不取，不得静默当「没阻塞」', () => {
    // 「读数失败 ⇒ 当作没阻塞」的代价是跑一张中间态票，比「取不到票」严重得多。
    expect(verdict({ ...base, blockedBy: null }).reason).toBe('blocked_unknown');
    expect(verdict({ ...base, blockedBy: [] }).verdict).toBe('eligible'); // 显式空数组才算「真的没阻塞」
    expect(verdict({ ...base, state: null }).reason).toBe('state_unknown');
    // 合成违规：判据表里不得出现「缺字段即放行」的分支
    const src = readText(path.join(ROOT, LIB_REL));
    expect(src).not.toMatch(/\$null\s*\|\|\s*@\(\)/);
  });

  test('FR3: 纪律清单逐字含被禁脚本，且被点名的脚本真实存在（防指到不存在的判据）', () => {
    const lines = runPs([
      '$d = Get-AfkAgentDiscipline',
      'foreach ($x in $d.Deny) { Write-Output ("deny=" + $x) }',
      'foreach ($x in $d.Allow) { Write-Output ("allow=" + $x) }',
      // 成品文本必须**折成一行**回传：runPs 按换行拆行，多行的 Text 会被拆散、
      // 只留首行 —— 首行恰好什么都不含，于是三条断言会「全绿但什么都没读到」（本轮实测红过）。
      'Write-Output ("text=" + ($d.Text -replace "\\r?\\n", " ⏎ "))',
    ].join('\n'));
    const deny = kv(lines, 'deny');
    const text = kv(lines, 'text').join('\n');
    expect(text.split('⏎').length).toBeGreaterThan(3); // 哨兵：拿到的确实整份清单，不是被拆残的首行
    // 被禁的每一个脚本都必须在仓里存在 —— 指到不存在的名字，等于那条禁令是空的。
    for (const name of ['hx-run.ps1', 'dev-finish.ps1', 'kotlin-all-check.ps1', 'compile-check.ps1',
                        'mp-weixin-check.ps1', 'auto-screenshot.ps1', 'device-capture.ps1']) {
      expect(deny.some((d) => d.includes(name))).toBe(true);
      expect(fs.existsSync(path.join(ROOT, 'scripts', name)) ||
              fs.existsSync(path.join(ROOT, 'scripts', 'lib', name))).toBe(true);
    }
    for (const name of ['Stop-Process', 'taskkill', 'cli.exe', 'git push', 'manifest.json', 'pages.json', 'platformConfig.json']) {
      expect(deny.some((d) => d.includes(name))).toBe(true);
    }
    expect(text).toContain('exit 2');            // 撞忙的正确反应必须写明，不能只说「别乱跑」
    expect(text).toContain('npm run test:unit'); // 允许面只有一条门
    expect(text).toContain('hx-run.ps1');         // 成品文本里要点名真实脚本，不能只写抽象禁令
  });

  test('FR4: 允许面与禁止面不得重叠（重叠的那条使整份清单失效）', () => {
    const lines = runPs([
      '$d = Get-AfkAgentDiscipline',
      'foreach ($x in $d.Allow) { Write-Output ("allow=" + $x) }',
      'foreach ($x in $d.Deny) { Write-Output ("deny=" + $x) }',
    ].join('\n'));
    const allow = kv(lines, 'allow');
    const deny = kv(lines, 'deny');
    // 按**子串**判重叠，不只看全等：「git add <本票文件>」与禁止项「add」这种半截重叠
    // 会让两条规则互相抵消（实测第一版就踩了这个 —— 禁令写得越短越容易反噬允许面）。
    const clashes = [];
    for (const a of allow) {
      for (const d of deny) {
        for (const tok of d.split(/[\s/、，（）()]+/).filter((t) => t.length >= 4)) {
          if (a.includes(tok)) clashes.push(`allow「${a}」含禁止词条「${tok}」`);
        }
      }
    }
    expect(clashes).toEqual([]);
    // 允许面必须极窄：写码 + 那一条门 + 本票文件的 add/commit，不多不少
    expect(allow.length).toBeLessThanOrEqual(4);
    expect(allow.some((a) => a.includes('npm run test:unit'))).toBe(true);
  });

  test('FR5: 结构锁 —— 本守护的 token 确实登记进 Q-A 契约面（否则这是一份永不执行的守护）', () => {
    const src = readText(path.join(ROOT, 'scripts', 'lib', 'contract-tests.ps1'));
    expect(src).toContain('frontier');
    // 反向断言：pattern 里那个 token 只能命中本文件（不得靠别的套件顺带绿）
    const pattern = src.match(/return\s*'([^']*frontier[^']*)'/)[1];
    expect(pattern.split('|').filter((t) => t === 'frontier').length).toBe(1);
    expect(pattern.split('|').filter((t) => /^frontier/.test(t)).length).toBe(1);
  });
});
