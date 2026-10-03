/**
 * 串行轮转外层 `scripts/frontier-run.ps1` 的**计划面**守护（#1435 ④ 步骤 2-3）
 *
 * 为什么这样切（设计要点，不是装饰）：
 *   外层真正会伤人的三件事都发生在**组命令**的那一刻：
 *     ① 建错树 / 嵌树 —— `new-worktree.ps1` 必须在**主树 cwd** 下跑（在树内调它会算出
 *        那个树自己，实测建出 `D:\FL\wt-1185\wt-(wip)1185`）；
 *     ② 把门交给 agent —— agent 一旦能调 `hx-run` / `dev-finish`，无人值守时它就会在
 *        「编译卡住」的路上顺手清理占用进程（kill 红线），而本机 agent CLI **没有权限档位**；
 *     ③ `-DryRun` 偷偷做了事 —— 一个自称 dry-run 的入口如果真去 push，比没有 dry-run 更糟。
 *   所以这三条钉成**数据断言**（计划是数组，可直接比对），而不是文案断言。
 *
 * ⚠️ 平台中立是这份文件的硬要求（第一轮 CI 实测教训）：③ 门跑在 **ubuntu**，
 *   而 `Join-Path $Root 'a\b\c'` 在 Windows 能过、在 Linux **直接抛**（子路径含反斜杠＝非法字符），
 *   本地全绿 CI 判红就是这个形态。⇒ 本文件的仓库根**用真实 ROOT**（两平台各自成立），
 *   断言一律走 `norm()` 比正斜杠形态，不写死任何 `D:\` / `C:\` 字面量。
 *
 * 刻意不测的（防越界断言）：
 *   建树、起 agent、跑门、真机取证 —— 那些的验收判据是「真跑一张票跑通」（#1435 ⑥ 第 3 条），
 *   在本文件里假称「已验证」正是 release.md:25 记的那类假绿。本文件只 dot-source 纯函数，
 *   **入口段（需要 -Ticket 才跑）根本不触达**。
 *
 * 运行前提：需要 pwsh（PowerShell 7）。先例同 wtBootstrapBehavior / frontierBehavior。
 */
const { execFileSync } = require('child_process');
const os = require('os');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'frontier.ps1');
const ENTRY_REL = path.join('scripts', 'frontier-run.ps1');

// 仓库根与纪律路径都取**当前平台**的真实值：纪律文件必须落在仓外（TEMP）。
const REPO = ROOT.replace(/\\/g, '/');
const DISCIPLINE = (path.join(os.tmpdir(), 'fl-frontier-1499-discipline.txt')).replace(/\\/g, '/');

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

function runPs(body) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    'try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }',
    `. "${path.join(ROOT, LIB_REL)}"`,
    body,
  ].join('\n');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  return String(execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
    encoding: 'utf8', timeout: 120000, windowsHide: true, maxBuffer: 8 * 1024 * 1024,
  })).split(/\r?\n/).map((l) => l.trim());
}

const norm = (p) => String(p).replace(/\\/g, '/');
const kv = (lines, key) => lines.filter((l) => l.startsWith(key + '=')).map((l) => l.slice(key.length + 1));

/**
 * 一次取回整份计划，按步切块。
 * ⚠️ 空值一律打哨兵 `~`：runPs 会 trim，`stop=` 这种空尾会被读成「没有这一步」——
 *    第一版就是这样把「步骤缺失」误读成「步骤不阻塞」的（假绿方向正好相反）。
 */
function plan(device) {
  const devLit = (device === null || device === undefined) ? '$null' : `'${device}'`;
  const lines = runPs([
    `$p = Get-FrontierRunPlan -RepoRoot '${REPO}' -TicketNumber 1499 -Level standard -Device ${devLit} -AgentExe 'pi' -DisciplineFile '${DISCIPLINE}'`,
    'foreach ($x in $p) {',
    '  Write-Output ("STEP=" + $x.Name)',
    '  Write-Output ("file=" + $(if ($x.File) { $x.File } else { "~" }))',
    '  Write-Output ("cwd=" + $(if ($x.Cwd) { $x.Cwd } else { "~" }))',
    '  Write-Output ("stop=" + $(if ($x.StopOn) { $x.StopOn } else { "~" }))',
    '  foreach ($a in $x.Args) { Write-Output ("arg=" + $a) }',
    '}',
  ].join('\n'));
  const steps = {};
  let cur = null;
  for (const raw of lines) {
    const l = raw.trim();
    if (!l) continue;
    if (l.startsWith('STEP=')) {
      cur = l.slice(5);
      steps[cur] = { file: '', cwd: '', stop: '', args: [] };
      continue;
    }
    if (!cur) throw new Error(`计划输出里有不属于任何步骤的行：${l}`);
    if (l.startsWith('file=')) steps[cur].file = l.slice(5);
    else if (l.startsWith('cwd=')) steps[cur].cwd = l.slice(4);
    else if (l.startsWith('stop=')) steps[cur].stop = l.slice(5);
    else if (l.startsWith('arg=')) steps[cur].args.push(l.slice(4));
  }
  return { steps, names: Object.keys(steps) };
}

const ALL = plan('10.0.0.2:5555');
const joined = (s) => [s.file, ...s.args].join(' ');

describe('串行轮转外层的计划面（运行期）', () => {
  test('FP1: 计划是固定的四步，顺序不可换（先建树 → 再写码 → 再自检 → 最后上机）', () => {
    expect(ALL.names).toEqual(['worktree', 'agent', 'self-test', 'gate']);
  });

  test('FP2: 建树步必须在主树 cwd 下跑（在树内调它会算出那个树自己 ⇒ 嵌树）', () => {
    const s = ALL.steps.worktree;
    expect(s.file).toMatch(/new-worktree\.ps1$/);
    // 判据真源：new-worktree.ps1 自己从 git 的公共目录取仓库根，但 cwd 落在别的树里时
    // 它仍会把新树建进那棵树 —— 故计划必须钉死 cwd = 主树。
    expect(norm(s.cwd)).toBe(REPO);
    expect(s.args.join(' ')).toContain('-Task 1499');
    // 反向：建树步里不得出现 agent 参数（防把两步糊成一步）
    expect(s.args.join(' ')).not.toMatch(/--print|--append-system-prompt/);
  });

  test('FP2b: 真源与入口的**代码行**里不得有「子路径含反斜杠」的 Join-Path（Windows 能过、ubuntu 直接抛）', () => {
    // 这条是**静态**守卫而非输出守卫：把断言写成「输出里没有 D:\」会变成平台依赖测试
    // （本机 Windows 一定写出反斜杠、CI ubuntu 不会）—— 那正是「本地绿 CI 红」的镜像错误，
    // 第一版就犯过一次。真正的不变量在源码里：Join-Path 的**子段**不许带分隔符。
    // ⚠️ 只扫代码行：注释里留着 `'a\b\c'` 这种反面示例是有教学价值的，第一版把注释也扫
    //    进去，守卫抓到的是自己的注释（假阳性），不是真缺陷。
    for (const rel of [LIB_REL, ENTRY_REL]) {
      const codeLines = readText(path.join(ROOT, rel))
        .split(/\r?\n/)
        .map((l) => l.trim())
        .filter((l) => l.length > 0 && !l.startsWith('#'));
      const bad = [];
      for (const l of codeLines) {
        const m = l.match(/Join-Path[^\r\n]*['"][^'"\r\n]*\\[^'"\r\n]*['"]/g);
        if (m) bad.push(...m);
      }
      expect({ file: rel, offenders: bad }).toEqual({ file: rel, offenders: [] });
    }
  });

  test('FP3: agent 步**不得**含任何 HBuilderX 调用，且必须带上纪律与票号', () => {
    const s = ALL.steps.agent;
    const a = s.args.join(' ');
    expect(a).toContain('--print');                       // 非交互形态
    expect(a).toContain('--append-system-prompt');        // 纪律必须随会话注入
    expect(a).toContain('1499');
    expect(norm(s.cwd)).toContain('wt-1499');              // 写码发生在自己的树里，不是主树
    // 纪律参数必须指向**纪律文件**，而且不能是本仓真源 .ps1（把源码路径塞进去，
    // agent 读到的是函数定义、不是禁令；这一条是 DryRun 实测抓出来的，「参数非空」判不住它）。
    const discIdx = s.args.indexOf('--append-system-prompt');
    expect(discIdx).toBeGreaterThanOrEqual(0);
    const discArg = norm(s.args[discIdx + 1]);
    expect(discArg).toMatch(/discipline/i);
    expect(discArg).not.toMatch(/\.ps1$/);
    expect(discArg.startsWith(REPO)).toBe(false);          // 落仓外，防被 git add 进树
    for (const forbidden of ['hx-run', 'dev-finish', 'kotlin-all', 'compile-check', 'mp-weixin-check', 'cli.exe']) {
      expect(joined(s)).not.toContain(forbidden);
    }
  });

  test('FP3b: 默认档（AgentExe=none）的写码步是**暂停步**：File 为空、cwd 仍在本票树里', () => {
    // none 档的存在理由：本机唯一可用的 agent CLI 凭证已失效（实调 401），
    // 而外层价值在取票/建树/自检/门/收尾那几格 —— 写码换成人不影响它们。这一档不可被删：
    // 删了整条链在凭证坏掉时就一行都跑不了（本轮真实发生过）。
    const lines = runPs([
      `$p = Get-FrontierRunPlan -RepoRoot '${REPO}' -TicketNumber 1499 -Level standard -Device $null -DisciplineFile ''`,
      "$a = $p | Where-Object { $_.Name -eq 'agent' }",
      'Write-Output ("file=" + $(if ($a.File) { $a.File } else { "~" }))',
      'Write-Output ("cwd=" + $(if ($a.Cwd) { $a.Cwd } else { "~" }))',
      'Write-Output ("stop=" + $(if ($a.StopOn) { $a.StopOn } else { "~" }))',
      'Write-Output ("args=" + ($a.Args -join " "))',
    ].join('\n'));
    // 逐行键值取回，不用分隔符拼一行（cwd 本身可能含 | 或 \，拼行会把字段切错 —— 第一版就这么红过）
    expect(kv(lines, 'file')[0]).toBe('~');              // File 空 = 无可执行体 = 暂停
    expect(norm(kv(lines, 'cwd')[0])).toContain('wt-1499'); // 但仍钉在本票的树里
    expect(kv(lines, 'stop')[0]).toBe('~');               // 暂停步不配 StopOn
    expect(kv(lines, 'args')[0]).toMatch(/Enter/);        // 提示人完成后继续
  });

  test('FP4: 只有 gate 一步可以调 dev-finish，且只透传 Level / Device（不夹带别的开关）', () => {
    const g = ALL.steps.gate;
    expect(g.file).toMatch(/dev-finish\.ps1$/);
    expect(g.args.join(' ')).toMatch(/-Level standard/);
    expect(g.args.join(' ')).toMatch(/-Device 10\.0\.0\.2:5555/);
    // 反向：不得把 -Distribute / -UpdateBaseline 这类会动发行或动基线默认的开关塞进去
    for (const forbidden of ['-Distribute', '-UpdateBaseline', '-DryRun']) {
      expect(g.args.some((a) => a === forbidden)).toBe(false);
    }
    // 其余三步都不得指向 dev-finish（「只有 gate 能调门」是这句的另一半）
    for (const n of ['worktree', 'agent', 'self-test']) {
      expect(joined(ALL.steps[n])).not.toContain('dev-finish');
    }
    // 自检步必须是纯 ③（不占设备、不取锁）
    const t = ALL.steps['self-test'];
    expect(t.args.join(' ')).toContain('test:unit');
    expect(joined(t)).not.toMatch(/hx-run|dev-finish/);
  });

  test('FP5: 退出码分工 —— 门与环境步骤 2 必停，写码/自检步骤 1 可续', () => {
    // dev-finish 的 exit 2 = 「环境不可用」（忙 / 超时 / 无设备）⇒ 继续轮转只会连续撞同一个环境，
    // 还会把上一棵树的常驻真运行会话留在原地。这条是 #1435 ⑤ 第三条的落点。
    expect(ALL.steps.gate.stop).toBe('2');
    expect(ALL.steps.worktree.stop).toBe('2');
    expect(ALL.steps.agent.stop).toBe('~');
    expect(ALL.steps['self-test'].stop).toBe('~');
  });

  test('FP6: 无设备时 gate 步不带 -Device（不得凭空造一个设备串，也不得留空的 -Device 尾巴）', () => {
    const noDev = plan(null);
    const g = noDev.steps.gate;
    expect(g.args.join(' ')).toMatch(/-Level standard/);
    expect(g.args.join(' ')).not.toMatch(/-Device/);
    // 反向锁：有设备时必须带 —— 两半都钉住，删掉任一半的实现都会红
    expect(ALL.steps.gate.args.join(' ')).toMatch(/-Device 10\.0\.0\.2:5555/);
  });

  test('FP7: 取票只取判据认可的候选，且按票号升序取第一张', () => {
    const lines = runPs([
      '$c = @(',
      '  [pscustomobject]@{ number = 1424; state = "OPEN"; labels = @("ready-for-mobile-agent"); assignees = @(); blockedBy = @() }',
      '  [pscustomobject]@{ number = 1422; state = "OPEN"; labels = @("ready-for-mobile-agent"); assignees = @(); blockedBy = @() }',
      '  [pscustomobject]@{ number = 1423; state = "OPEN"; labels = @("ready-for-mobile-agent"); assignees = @(); blockedBy = @(@{ number = 1421; state = "OPEN" }) }',
      '  [pscustomobject]@{ number = 1421; state = "OPEN"; labels = @("ready-for-mobile-agent"); assignees = @(@{ login = "other" }); blockedBy = @() }',
      ')',
      '$r = Select-FrontierTicket -Candidates $c',
      'Write-Output ("picked=" + $(if ($null -ne $r.Number) { $r.Number } else { "~" }))',
      // ⚠️ 逐条打行，**不做字符串 join**：`("x=" + @(...)) -join ","` 的 -join 绑在拼接结果上，
      //    会把整个数组折成一行（本仓记过的「转发流尾部丢行」同一族形态），断言看着有内容、实则不可分。
      'foreach ($x in $r.Rejected) { Write-Output ("rej=" + $x.Number + ":" + $x.Reason) }',
    ].join('\n'));
    expect(kv(lines, 'picked')[0]).toBe('1422');   // 升序里第一张**合格**的（不是数组第一个）
    const rejected = kv(lines, 'rej');
    expect(rejected).toEqual(expect.arrayContaining(['1423:blocked', '1421:assigned', '1424:superseded']));
    expect(rejected.length).toBe(3);
  });

  test('FP8: 空候选不猜票（取不到票 ⇒ 停，不退回「随便挑一张」）', () => {
    const lines = runPs([
      '$r = Select-FrontierTicket -Candidates @()',
      'Write-Output ("picked=" + $(if ($null -ne $r.Number) { $r.Number } else { "~" }))',
      'Write-Output ("rej=" + @($r.Rejected).Count)',
    ].join('\n'));
    expect(kv(lines, 'picked')[0]).toBe('~');      // 无票可取时不得凭空造一个票号
    expect(kv(lines, 'rej')[0]).toBe('0');
  });

  test('FP9: 整份计划里不得出现任何交付动作或进程操作（AFK 终点 = 本地提交）', () => {
    const all = Object.values(ALL.steps).map(joined).join(' | ');
    for (const forbidden of ['git push', 'gh pr', 'gh issue', 'Stop-Process', 'taskkill', 'cli.exe']) {
      expect(all).not.toContain(forbidden);
    }
    // 入口段必须存在且被守卫（DryRun 要在取票**之前**就能返回，否则「看一眼计划」也要联网）
    const src = readText(path.join(ROOT, ENTRY_REL));
    expect(src).toMatch(/DryRun/);
    expect(src).toMatch(/Get-FrontierRunPlan/);
    expect(src).toMatch(/Select-FrontierTicket/);
    expect(src).toMatch(/Get-AfkAgentDiscipline/);   // 纪律文件必须由入口落盘，不是手写常量
    expect(src).toMatch(/AUTO_PICK_CONFIRM/);       // 母票不可判 ⇒ 自动取票必须过人这一关
  });

  test('FP10: CLI 档没有纪律文件就组不出计划；none 档不要求（否则默认档自己先跑不起来）', () => {
    let failed = false;
    try {
      runPs([
        `Get-FrontierRunPlan -RepoRoot '${REPO}' -TicketNumber 1499 -Level standard -Device $null -AgentExe 'pi' -DisciplineFile ''`,
      ].join('\n'));
    }
    catch (e) {
      failed = true;
      expect(String(e.message)).toMatch(/DisciplineFile/);
    }
    expect(failed).toBe(true);   // 反向锁：删掉这条守卫，本用例即红
    // 另一半：默认档（不传 AgentExe）不带纪律也必须能组图 ——
    // 否则FP3b 那一档在真实调用里会先被 FP10 的守卫抵掉。
    const okLines = runPs([
      `$p = Get-FrontierRunPlan -RepoRoot '${REPO}' -TicketNumber 1499 -Level standard -Device $null`,
      'foreach ($x in $p) { Write-Output ("n=" + $x.Name) }',
    ].join('\n'));
    expect(kv(okLines, 'n')).toEqual(['worktree', 'agent', 'self-test', 'gate']);
  });

  test('FP11: 执行器对 .ps1 步必须经子 `pwsh -File` 分派（#1467 首跑实测：`& $s.File @($s.Args)` 把参数数组当成一个实参绑给脚本 ⇒ -Task 收到数组、转 System.String 失败，整轮崩在建树步）', () => {
    const src = readText(path.join(ROOT, ENTRY_REL));
    // 正锁：入口必须按 .ps1 分派到子 `pwsh -File`（worktree / gate 两步都是 .ps1）。
    expect(src).toMatch(/-like\s+'\*\.ps1'/);
    expect(src).toMatch(/-File\s+\$s\.File/);
    // 反锁：旧的「直接 & 脚本传数组」写法不得复活（改回这行，本用例即红）。
    expect(src).not.toMatch(/&\s+\$s\.File\s+@\(\$s\.Args\)/);
    // 外部命令（npm / agent CLI）仍走直接调用 —— 数组对 exe 天然拆成 argv，不能被 -File 化。
    expect(src).toMatch(/&\s+\$s\.File\s+\$s\.Args/);
  });
});
