/**
 * 「日常增量运行」契约守护（scripts/hx-run.ps1 + 三层节奏文档）
 *
 * 背景（2026-09-12）：移动端 UI 迭代的痛点不是编译慢，而是**每次微调都走全量那一圈**——
 * 全量编译实测 4.8–8 分钟（`compile-check.ps1` 就是这条门，默认值不动），增量约 1–2 分钟；
 * 资源导出 publish 98 秒–3 分钟；基座 APK 95.7 MB **只需装一次**。于是新增 `scripts/hx-run.ps1`
 * 承担**日常增量**：默认无干净缓存重建、不重装基座、忙就等、绝不 kill。
 * 这个脚本的全部风险在于**被后续会话"顺手优化"成全量**，或**为了"清干净"去 kill 主程序**
 * （违反 ADR-0008 的「HBuilderX 是单实例串行资源」坑位），所以用本测试把这几条锁死：
 *   C1 默认增量：干净缓存重建开关**只允许**出现在文件唯一的 `if ($Full)` 块里；`-Full` 是显式开关
 *   C2 不得出现任何强杀调用（Stop-Process / taskkill / 进程对象 .Kill()）——共享主程序绝不抢占
 *   C3 必须 dot-source `scripts/lib/hx-busy.ps1`、调用 `Wait-HxFree`，且锁在 `finally` 里释放
 *   C4 忙等待上限是 `-WaitSeconds`（默认 600），环境不可用一律 `exit 2`
 *   C5 必须输出机检行 `HX_RUN mode=… compile=… deploy=… total=… exit=…`，且三种结论都有出口
 *   C6 判成败只解析 stdout：不得出现 `$LASTEXITCODE` / `ExitCode`（CLI 退出码恒 0）
 *   C7 `-DryRun` 存在、在任何 HBuilderX 交互之前 exit 0，且该分支内不派发 cli / 不做忙检测 / 不启进程；
 *      真正派发 cli（`Invoke-CliStep -Name`）必须在忙检测（`Wait-HxFree -CliExe`）之后
 *   C8 `-Device` 缺/多 ⇒ exit 2、日志默认 `.ci-verify/hx-run.log`、脚本**不做** adb 装基座
 *   C9 头注释写明与 `compile-check.ps1` 的分工（门 = 全量，本脚本 = 日常增量）与「绝不 kill 主程序」
 *   C10 文档落锁：移动端 `AGENTS.md`「开发内循环（移动端 UI 迭代）」三层节奏 + 反模式、
 *       ADR-0008 非门辅助段的一行指针、`package.json` 注册 `hx:run`
 *
 * 设计沿用本仓既有守护测试形态（见 utils/emulatorSmokeContract.test.js、utils/hxBusyGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 *
 * **CRLF 坑位**：`.gitattributes` 已把 `*.ps1` / `*.js` / `*.md` 钉成 LF；本文件里的锚点按 **LF** 写。
 * 若哪天有人把 `hx-run.ps1` 存成 CRLF，本测试的多行锚点会失配（这正是想要的：先红再改）。
 *
 * **未实测声明（照实记）**：`scripts/hx-run.ps1` 在本 PR 里**只跑到了 `-DryRun`**（维护者正用 HBuilderX 做 #781，
 * 不抢占主程序）。所以「真实 HBuilderX 输出下 `HX_RUN` 分段行与编译/部署段是否准」**待维护者空出 HBuilderX 后首跑校准**
 * （脚本内的 `$script:HxCompileEndMarkers` 标记表就是为此留的校准点）。
 */

const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/hx-run.ps1';
const AGENTS_REL = 'AGENTS.md';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
const PKG_REL = 'package.json';

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

/**
 * 掩掉 `<# ... #>` 文档块（块内字符替换成空格，偏移/行号不变）。
 * 头注释本来就允许出现 `Stop-Process`、`if ($Full)`、`adb install` 这类**叙述性**字面量
 * （它们在解释规则本身），所以涉及「代码里不许有」的规则只查掩码后的正文。
 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 纯函数：源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };
  const raw = sources.script || '';
  const code = maskDocBlocks(raw);
  const agents = sources.agents || '';
  const adr = sources.adr || '';
  const pkg = sources.pkg || '';

  // ---- C1 默认增量：干净缓存重建开关只允许出现在唯一的 if ($Full) 块里 ----
  const blocks = code.match(/if\s*\(\s*\$Full\s*\)\s*\{[\s\S]*?\n\}/g) || [];
  must(blocks.length === 1, 'C1', `应恰有 1 个 if ($Full) 块（实得 ${blocks.length} 个）——全量开关必须集中一处`);
  const totalSw = (code.match(/cleanCache/g) || []).length;
  const inBlockSw = blocks.reduce((n, b) => n + (b.match(/cleanCache/g) || []).length, 0);
  must(totalSw >= 1, 'C1', '未见干净缓存重建开关（-Full 分支必须给出，否则 -Full 是空开关）');
  must(totalSw === inBlockSw, 'C1', `干净缓存重建开关出现在 if ($Full) 块之外（全文 ${totalSw} 处，块内 ${inBlockSw} 处）⇒ 默认会变全量`);
  must(/\[switch\]\$Full/.test(code), 'C1', '缺 -Full 开关（全量必须显式给，不能默认）');
  must(/\$mode = 'incremental'/.test(code), 'C1', '默认 mode 不是 incremental');

  // ---- C2 绝不强杀（共享主程序绝不抢占）----
  ['Stop-Process', 'taskkill', 'pkill', 'kill.exe'].forEach((token) => {
    must(!code.includes(token), 'C2', `正文出现强杀调用字面量：${token}（HBuilderX 是单实例串行资源，忙就等）`);
  });
  must(!/\.Kill\s*\(/i.test(code), 'C2', '正文出现进程对象 .Kill() 调用（不得强杀任何进程）');

  // ---- C3 忙检测 helper + 锁 ----
  must(/lib[\\/]hx-busy\.ps1/.test(code), 'C3', '未 dot-source scripts/lib/hx-busy.ps1');
  must(/Wait-HxFree\s+-CliExe/.test(code), 'C3', '未调用 Wait-HxFree（缺 agent 互斥锁 + 忙探测 + 等待上限）');
  const finallyAt = code.lastIndexOf('} finally {');
  must(finallyAt !== -1, 'C3', '缺 finally 路径（锁必须在那里释放）');
  if (finallyAt !== -1) {
    must(/Release-HxLock/.test(code.slice(finallyAt)), 'C3', 'finally 路径里没有 Release-HxLock');
  }

  // ---- C4 忙/超时 = exit 2 ----
  must(/\[int\]\$WaitSeconds\s*=\s*600/.test(code), 'C4', '-WaitSeconds 默认不是 600（忙等待上限必须显式且默认 600）');
  const exit2 = (code.match(/exit 2/g) || []).length;
  must(exit2 >= 3, 'C4', `环境不可用路径未判 exit 2（应 ≥3 处：cli/adb 缺失、设备缺或多、连接中断或超时；实得 ${exit2} 处）`);

  // ---- C5 机检行 ----
  must(/HX_RUN mode=/.test(code), 'C5', '缺 HX_RUN mode= 机检行');
  must(
    /HX_RUN mode=\{0\} compile=\{1\} deploy=\{2\} total=\{3\} exit=\{4\}/.test(code),
    'C5',
    '机检行格式不符（应为 HX_RUN mode=… compile=… deploy=… total=… exit=…）'
  );
  ["-Exit 'ok'", "-Exit 'fail'", "-Exit 'env'"].forEach((token) => {
    must(code.includes(token), 'C5', `缺结论出口 ${token}（判成败的每条路径都要打机检行）`);
  });

  // ---- C6 不看退出码 ----
  must(!/\$LASTEXITCODE/.test(code), 'C6', '出现 $LASTEXITCODE：CLI 退出码恒 0，判成败只解析 stdout');
  must(!/ExitCode/.test(code), 'C6', '出现 ExitCode：CLI 退出码恒 0，判成败只解析 stdout');
  must(/\$errorLines/.test(code), 'C6', '缺 stdout 判据（error 行扫描）');

  // ---- C7 -DryRun 零副作用 + 派发顺序 ----
  must(/\[switch\]\$DryRun/.test(code), 'C7', '缺 -DryRun 开关（唯一可端到端验证的路径）');
  const dryAt = code.indexOf('if ($DryRun) {');
  must(dryAt !== -1, 'C7', '未找到 `if ($DryRun) {` 分支');
  const dryExitAt = dryAt === -1 ? -1 : code.indexOf('exit 0', dryAt);
  must(dryExitAt > dryAt, 'C7', '-DryRun 分支没有在自己那段里 exit 0');
  if (dryAt !== -1 && dryExitAt > dryAt) {
    const drySlice = code.slice(dryAt, dryExitAt);
    must(!drySlice.includes('Invoke-CliStep'), 'C7', '-DryRun 分支里派发了 cli（Invoke-CliStep）');
    must(!/Wait-HxFree\s+-CliExe/.test(drySlice), 'C7', '-DryRun 分支里做了忙检测（会碰主程序）');
    must(!drySlice.includes('Start-Process'), 'C7', '-DryRun 分支里启动了进程');
    must(!/Resolve-TargetDevice|Resolve-AdbExe/.test(drySlice), 'C7', '-DryRun 分支里探测了设备（会调 adb）');
  }
  const waitCallAt = code.indexOf('Wait-HxFree -CliExe');
  const firstDispatchAt = code.indexOf('Invoke-CliStep -Name');
  must(dryAt !== -1 && waitCallAt !== -1 && dryAt < waitCallAt, 'C7', '-DryRun 的早退不在忙检测之前 ⇒ 计划输出也可能碰到主程序');
  must(waitCallAt !== -1 && firstDispatchAt !== -1 && waitCallAt < firstDispatchAt, 'C7', '派发 cli 早于忙检测（Wait-HxFree 必须在 Invoke-CliStep -Name 之前）');

  // ---- C8 设备前置判定 / 日志 / 不自装基座 ----
  must(/\[string\]\$Device/.test(code), 'C8', '缺 -Device 参数');
  // 「多设备必须显式指定」锚在**守卫代码**上（只查散文会让"把守卫删了但留着提示"蒙混过关）
  must(
    /if\s*\(\s*\$online\.Count\s*-gt\s*1\s*\)\s*\{[\s\S]{0,500}?exit 2/.test(code),
    'C8',
    '缺「多设备未显式指定 ⇒ exit 2」的守卫（$online.Count -gt 1 分支必须 exit 2）'
  );
  must(/\$online\.Count\s*-eq\s*0/.test(code), 'C8', '缺「无在线设备 ⇒ exit 2」的守卫（$online.Count -eq 0 分支）');
  must(/hx-run\.log/.test(code), 'C8', '日志默认路径不是 .ci-verify/hx-run.log');
  must(/\.ci-verify/.test(code), 'C8', '日志未落 .ci-verify/（该目录已在 .gitignore）');
  must(!/install/i.test(code), 'C8', '正文出现 install：脚本不得自装基座（95.7 MB 只装一次，装机交给 HBuilderX）');

  // ---- C9 头注释的分工与纪律（查原始文本：这些声明写在文档块里）----
  must(raw.includes('compile-check.ps1'), 'C9', '头注释未写明与 compile-check.ps1 的分工');
  must(raw.includes('门') && raw.includes('全量'), 'C9', '头注释未写明「门 = 全量」');
  must(raw.includes('日常增量'), 'C9', '头注释未写明本脚本 = 日常增量');
  must(raw.includes('绝不 kill'), 'C9', '头注释未写明「绝不 kill 主程序」');
  must(raw.includes('scripts/lib/hx-busy.ps1'), 'C9', '头注释未指向 scripts/lib/hx-busy.ps1');
  must(raw.includes('不是门'), 'C9', '头注释未声明本脚本不是门（不进验收证据）');

  // ---- C10 文档落锁 ----
  must(agents.includes('## 开发内循环（移动端 UI 迭代）'), 'C10', 'AGENTS.md 缺「开发内循环（移动端 UI 迭代）」小节');
  ['内循环', '中循环', '外循环'].forEach((token) => {
    must(agents.includes(token), 'C10', `AGENTS.md 三层节奏缺「${token}」`);
  });
  must(agents.includes('hx-run.ps1'), 'C10', 'AGENTS.md 未指向 scripts/hx-run.ps1');
  must(agents.includes('npm run hx:run'), 'C10', 'AGENTS.md 未给出 npm run hx:run 入口');
  must(agents.includes('反模式'), 'C10', 'AGENTS.md 缺「反模式」清单');
  must(agents.includes('一个视觉主题一个 commit'), 'C10', 'AGENTS.md 未写「一个视觉主题一个 commit」的提交粒度');
  must(agents.includes('56d0fe3'), 'C10', 'AGENTS.md 未给出混提交的反例（56d0fe3）');
  must(agents.includes('不重装基座') || agents.includes('基座只装一次'), 'C10', 'AGENTS.md 未写「不重装基座」');
  must(adr.includes('开发内循环'), 'C10', 'ADR-0008 非门辅助段缺一行指针（应指向该小节与本脚本）');
  must(adr.includes('hx-run.ps1'), 'C10', 'ADR-0008 指针未指向 scripts/hx-run.ps1');
  must(/"hx:run"\s*:\s*"[^"]*scripts\/hx-run\.ps1"/.test(pkg), 'C10', 'package.json 未注册 hx:run → scripts/hx-run.ps1');

  return violations;
}

describe('日常增量运行契约（scripts/hx-run.ps1，2026-09-12）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    agents: readSource(AGENTS_REL),
    adr: readSource(ADR_REL),
    pkg: readSource(PKG_REL)
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['C1', '干净缓存开关被挪出 -Full 块', (s) => ({
        ...s,
        script: s.script + "\n$launchArgs += @('--cleanCache', 'true')\n"
      })],
      ['C1', '-Full 块被改名（全量变成不可控）', (s) => ({
        ...s,
        script: s.script.replace('if ($Full) {', 'if ($true) {')
      })],
      ['C1', '-Full 开关被删', (s) => ({ ...s, script: s.script.replace('[switch]$Full,', '') })],
      ['C2', '混入强杀主程序', (s) => ({ ...s, script: s.script + '\nStop-Process -Name HBuilderX -Force\n' })],
      ['C2', '混入 taskkill', (s) => ({ ...s, script: s.script + '\ntaskkill /IM HBuilderX.exe /F\n' })],
      ['C2', '混入进程对象 .Kill()', (s) => ({ ...s, script: s.script + '\n$proc.Kill()\n' })],
      ['C3', 'dot-source 被删', (s) => ({ ...s, script: s.script.replace(/lib\\hx-busy\.ps1/g, 'other.ps1') })],
      ['C3', 'Wait-HxFree 被删', (s) => ({ ...s, script: s.script.replace(/Wait-HxFree -CliExe/g, 'WaitNothing -CliExe') })],
      ['C3', '锁没在 finally 里释放', (s) => ({ ...s, script: s.script.replace(/Release-HxLock/g, 'ReleaseNothing') })],
      ['C4', '忙等待上限被改大', (s) => ({ ...s, script: s.script.replace('[int]$WaitSeconds = 600', '[int]$WaitSeconds = 3600') })],
      ['C4', '环境不可用不再 exit 2', (s) => ({ ...s, script: s.script.replace(/exit 2/g, 'exit 9') })],
      ['C5', '机检行格式被改', (s) => ({
        ...s,
        script: s.script.replace('HX_RUN mode={0} compile={1}', 'HX_RUN compile={1}')
      })],
      ['C5', 'fail 出口被删', (s) => ({ ...s, script: s.script.replace("-Exit 'fail'", "-Exit 'bad'") })],
      ['C6', '拿退出码当判据', (s) => ({ ...s, script: s.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' })],
      ['C6', '改用 ExitCode 判成败', (s) => ({ ...s, script: s.script + '\nif ($proc.ExitCode -ne 0) { exit 1 }\n' })],
      ['C7', '-DryRun 分支被改名', (s) => ({ ...s, script: s.script.replace('if ($DryRun) {', 'if ($false) {') })],
      ['C7', '-DryRun 分支里派发 cli', (s) => ({
        ...s,
        script: s.script.replace('if ($DryRun) {', "if ($DryRun) {\n    Invoke-CliStep -Name 'open'\n")
      })],
      ['C8', '多设备守卫被删', (s) => ({ ...s, script: s.script.replace('if ($online.Count -gt 1) {', 'if ($false) {') })],
      ['C8', '无设备守卫被删', (s) => ({ ...s, script: s.script.replace('if ($online.Count -eq 0) {', 'if ($false) {') })],
      ['C8', '日志改名', (s) => ({ ...s, script: s.script.replace(/hx-run\.log/g, 'run.log') })],
      ['C8', '混入自装基座', (s) => ({ ...s, script: s.script + '\n& $adbExe install -r base.apk\n' })],
      ['C9', '头注释不再声明绝不 kill', (s) => ({ ...s, script: s.script.replace(/绝不 kill/g, '尽量避免 kill') })],
      ['C9', '头注释不再提 compile-check 分工', (s) => ({
        ...s,
        script: s.script.replace(/compile-check\.ps1/g, 'some-gate.ps1')
      })],
      ['C10', 'AGENTS.md 小节被删', (s) => ({
        ...s,
        agents: s.agents.replace('## 开发内循环（移动端 UI 迭代）', '## 开发循环')
      })],
      ['C10', 'AGENTS.md 反模式清单被删', (s) => ({ ...s, agents: s.agents.replace(/反模式/g, '注意事项') })],
      ['C10', 'ADR 指针被删', (s) => ({ ...s, adr: s.adr.replace('hx-run.ps1', 'other.ps1') })],
      ['C10', 'package.json 未注册', (s) => ({ ...s, pkg: s.pkg.replace('"hx:run"', '"hxrun"') })]
    ];
    cases.forEach(([rule, label, mutate]) => {
      const found = scanContract(mutate(real));
      expect({ label, hit: found.some((v) => v.startsWith(rule)) }).toEqual({ label, hit: true });
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('C1：默认增量的 launch 参数里没有干净缓存重建开关，-Full 才给', () => {
    const code = maskDocBlocks(real.script);
    const blocks = code.match(/if\s*\(\s*\$Full\s*\)\s*\{[\s\S]*?\n\}/g) || [];
    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toContain('--cleanCache');
    expect(code).toContain("$launchArgs = @('launch', 'app-android', '--project', $Project, '--compile', 'true')");
  });

  it('C2：正文不含任何强杀调用（头注释里的说明性字面量已被掩码排除）', () => {
    const code = maskDocBlocks(real.script);
    expect(code).not.toMatch(/Stop-Process|taskkill|\.Kill\s*\(/i);
    // 头注释本身必须写明这条纪律（否则下个会话看不见为什么不能 kill）
    expect(real.script).toContain('绝不 kill');
  });

  it('C7：-DryRun 在任何 HBuilderX 交互之前 exit 0，且真派发在忙检测之后', () => {
    const code = maskDocBlocks(real.script);
    const dryAt = code.indexOf('if ($DryRun) {');
    const waitAt = code.indexOf('Wait-HxFree -CliExe');
    const dispatchAt = code.indexOf('Invoke-CliStep -Name');
    expect(dryAt).toBeGreaterThan(-1);
    expect(dryAt).toBeLessThan(waitAt);
    expect(waitAt).toBeLessThan(dispatchAt);
  });

  it('C10：AGENTS.md 的三层节奏与 ADR-0008 的一行指针都在', () => {
    expect(real.agents).toContain('## 开发内循环（移动端 UI 迭代）');
    expect(real.agents).toContain('npm run hx:run');
    expect(real.adr).toContain('开发内循环');
    expect(real.adr).toContain('scripts/hx-run.ps1');
  });
});