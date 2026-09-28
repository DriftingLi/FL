/**
 * worktree 初始化真源 `scripts/lib/wt-bootstrap.ps1` 的行为守护（运行期）
 *
 * 为什么需要这个文件（2026-09-28）：
 *   「新树怎么拿到 node_modules」原本每条链路各写一份：Qoder 的启动配置框里一份（依赖宿主注入的
 *   私有环境变量）、人手工建树时靠 docs/agents/multi-agent-git.md 里那条 mklink 配方、闸门
 *   new-worktree.ps1 一份都没有。后果已记过账：worktree 里跑 npm test 报
 *   `'jest' 不是内部或外部命令`。
 *   ⇒ 收成 scripts/lib/wt-bootstrap.ps1 一个真源，本文件**运行期**钉住五条判据：
 *     ① 子工程由「有 jest.config.unit.js」反推，真源里不写死移动端项目名；
 *     ② 真源不读任何编辑器专有环境变量（否则别的工具那条链路又断）；
 *     ③ 点开头/元字符开头的目录段必须判为不安全（E2/E3 的机制），Qoder 的真实落点必须在判红一侧；
 *     ④ 状态归属正确（ok / broken / refuse / link / skip）：主树缺依赖时是 skip 而**不是抛错**；
 *     树里有空壳（无 jest 入口）时是 broken 而不是 ok —— 只看目录存在会把一次失败的 npm ci 判成已装好；
 *     ⑤ 本树非 ③ 可信树时，移动端那份依赖判 **refuse**（故意不链：链上就是装配假绿）。
 *     射程只到移动端：`Get-WtNodeProjects` 的唯一判据就是「有 `jest.config.unit.js`」，
 *     不枚举 `frontend`（Web/后端不由本计划管）；
 *     ⑥ 退出码分工（Q5-C）：非闸门树不可信只**告知**（exit 0），只有 `-ExpectEligible`（闸门树）
 *     才判失败（exit 3）—— 宿主树不可信是常态，不是故障。（⑥ 由 W8 钉住，Task 2 追加。）
 *
 * 运行前提：需要 pwsh（PowerShell 7）。该前提**不是假设**：同一 CI job 里的
 * contractTestPatternBehavior.test.js 已经在 ubuntu runner 上 dot-source pwsh 并跑绿，
 * 而它与本文件一样 fail-closed 不 skip ⇒ 镜像没有 pwsh 的话它早就红了。
 * 本文件只 dot-source 纯函数，**不建 junction**（那是 Windows-only，且 CI 是 ubuntu）：
 * 副作用那一步的验收判据是「真建一棵树跑通」，见 PR 的验收证据（Task 4）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'wt-bootstrap.ps1');
const JEST_ENTRY = require.resolve('jest/bin/jest');   // ENTRY_REL 到 Task 2 追加 W8 时才声明

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读（先例同）
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** dot-source 真源并执行一段脚本，stdout 按非空行返回。 */
function runPs(body) {
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    // 控制台输出编码必须强制 UTF-8：Windows 下默认跟随 OEM 代码页，枚举结果里的 CJK 目录段
    // 会在 stdout 往返中被改写（本机实测：无此行时 W1 红，加后绿；ubuntu 上是空操作）。
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
const norm = (p) => String(p).replace(/\\/g, '/').replace(/\/+$/, '');

describe('worktree 初始化真源（运行期）', () => {
  test('W1: 子工程枚举 —— 每一项都真的指到它的 jest.config.unit.js（不写死项目名、不枚举 frontend）', () => {
    const repoRoot = path.join(ROOT, '..', '..');   // ⚠️ Base 必须是**仓库根**：判据扫的是 <Base>/training-app/
    const items = kv(runPs(`foreach ($p in (Get-WtNodeProjects -Base "${repoRoot}")) { Write-Output ("P=" + $p) }`), 'P');
    expect(items.length).toBeGreaterThanOrEqual(1);
    for (const one of items) {
      expect(one.startsWith('training-app/')).toBe(true);
      expect(fs.existsSync(path.join(repoRoot, ...one.split('/'), 'jest.config.unit.js'))).toBe(true);
    }
  });

  test('W2: 真源不写死移动端项目名、不读编辑器专有变量', () => {
    const projName = path.basename(ROOT);
    for (const rel of [LIB_REL]) {                       // ⚠️ 只查 lib —— 入口在 Task 2 才存在（W2 在那里扩到入口）
      const src = readText(path.join(ROOT, rel));
      expect(src).not.toContain(projName);
      expect(src).not.toContain('ROOT_WORKTREE_PATH');
    }
  });

  test('W3: 点/元字符开头的目录段判不安全，段中含点判安全', () => {
    const cases = [
      ['D:/FL/wt-1368', 'True'],
      ['D:/FL/wt-(wip)1185', 'True'],
      ['D:/FL/training-app/x', 'True'],
      ['/home/runner/work/FL/FL/wt-1', 'True'],
      ['C:/Users/dev/.qoder/worktree/FL-1a2b3c4d', 'False'],
      ['D:/FL/.wt-1144probe/training-app/x', 'False'],
      ['D:/FL/(wip)1185/training-app/x', 'False'],
    ];
    const got = kv(runPs(cases.map(([p]) => `Write-Output ("S=" + (Test-WtPathSafe -Path '${p}'))`).join('\n')), 'S');
    expect(got.length).toBe(cases.length);
    cases.forEach(([, want], i) => expect(got[i]).toBe(want));
  });

  test('W4: 五种状态归属正确（③ 可信树下）—— 含 skip 不抛错、空壳判 broken', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'wtb-'));
    try {
      const main = path.join(tmp, 'main');
      const tree = path.join(tmp, 'wt-1');
      const proj = 'training-app/FakeProj';        // 主树有 → link
      const bare = 'training-app/FakeBare';        // 两边都没 → skip（且不抛）
      const shell = 'training-app/FakeShell';      // 树里有空壳、无 jest 入口 → broken
      const ready = 'training-app/FakeReady';      // 树里有 jest 入口 → ok
      for (const root of [main, tree]) {
        for (const p of [proj, bare, shell, ready]) {
          fs.mkdirSync(path.join(root, p), { recursive: true });
          fs.writeFileSync(path.join(root, p, 'jest.config.unit.js'), 'module.exports = {};\n');
        }
        // 诱饵：把同名的 jest 配置放到 training-app **外面**。
        // 真源只扫 <Base>/training-app/* ⇒ 它不得入列；扫描根一旦被放宽，下面的行数断言就红。
        fs.mkdirSync(path.join(root, 'frontend', 'decoy'), { recursive: true });
        fs.writeFileSync(path.join(root, 'frontend', 'decoy', 'jest.config.unit.js'), 'module.exports = {};\n');
      }
      fs.mkdirSync(path.join(main, proj, 'node_modules'), { recursive: true });
      fs.mkdirSync(path.join(tree, shell, 'node_modules'), { recursive: true });                  // 空壳
      fs.mkdirSync(path.join(tree, ready, 'node_modules', 'jest', 'bin'), { recursive: true });   // 可用
      fs.writeFileSync(path.join(tree, ready, 'node_modules', 'jest', 'bin', 'jest.js'), '');
      const body = [
        `$plan = Get-WtNodeModulesPlan -MainRoot "${main}" -WtRoot "${tree}"`,
        'foreach ($i in $plan) { Write-Output ("R=" + $i.Project + "|" + $i.State + "|" + ([bool]$i.Target) + "|" + ([bool]$i.Source)) }',
      ].join('\n');
      const rows = kv(runPs(body), 'R');
      const line = (name) => rows.find((r) => r.startsWith(name + '|')) || '';
      const state = (name) => line(name).split('|')[1];
      expect(state(proj)).toBe('link');
      expect(state(bare)).toBe('skip');       // 主树缺依赖：skip，**不抛错**
      expect(state(shell)).toBe('broken');    // 空壳不再被当成已装好
      expect(state(ready)).toBe('ok');
      expect(rows.length).toBe(4);            // 射程机检：诱饵不得入列
      expect(line(proj)).toContain('|True|True');   // Target / Source 恒在（Task 2 依赖）
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('W5: ③ 不可信的树里，移动端那份依赖判 refuse（故意不链，把假绿换成硬报错）', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'wtb-'));
    try {
      const main = path.join(tmp, 'main');
      // 模拟宿主落点：父段段首带点 ⇒ 本树非 ③ 可信树
      const tree = path.join(tmp, '.qoder', 'worktree', 'FL-1a2b3c4d');
      const proj = 'training-app/FakeProj';
      for (const root of [main, tree]) {
        fs.mkdirSync(path.join(root, proj), { recursive: true });
        fs.writeFileSync(path.join(root, proj, 'jest.config.unit.js'), 'module.exports = {};\n');
      }
      // 诱饵：training-app **外面**的同名配置。真源只扫 training-app/ ⇒ 它不得入列。
      for (const root of [main, tree]) {
        fs.mkdirSync(path.join(root, 'frontend', 'decoy'), { recursive: true });
        fs.writeFileSync(path.join(root, 'frontend', 'decoy', 'jest.config.unit.js'), 'module.exports = {};\n');
      }
      fs.mkdirSync(path.join(main, proj, 'node_modules'), { recursive: true });
      const body = [
        `$plan = Get-WtNodeModulesPlan -MainRoot "${main}" -WtRoot "${tree}"`,
        'foreach ($i in $plan) { Write-Output ("R=" + $i.Project + "|" + $i.State) }',
      ].join('\n');
      const rows = kv(runPs(body), 'R');
      // 唯一一项且判 refuse（链上就会 0 套件假绿 ⇒ 故意不链）；整行相等，连带守住「诱饵未入列」。
      // ⚠️ kv() 已剥掉 `R=` 前缀 ⇒ 期望值不带它（brief 原文带 R= 是笔误，本机实测必红；语义不变）。
      expect(rows).toEqual([`${proj}|refuse`]);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  test('W6: 主树位置由 git 元数据反推 —— 等于**共享主树**（linked worktree 下不是本树根）', () => {
    // 独立判据：`git worktree list --porcelain` 的第一条恒为主工作树。
    // ⚠️ 不能拿「本树根」当期望值：linked worktree 里 --git-common-dir 返回的是**主树**的 .git
    //    ⇒ 主树根 ≠ 本树根 —— 而那正是本函数要的语义（要借的就是主树那份 node_modules）。
    const porcelain = String(execFileSync('git', ['-C', ROOT, 'worktree', 'list', '--porcelain'], { encoding: 'utf8' }));
    const mainWorktree = porcelain.match(/^worktree (.+)$/m);
    expect(mainWorktree).toBeTruthy();
    const [got] = kv(runPs(`Write-Output ("M=" + (Get-WtMainRoot -Base "${ROOT}"))`), 'M');
    expect(got).toBeTruthy();
    expect(norm(got).toLowerCase()).toBe(norm(mainWorktree[1].trim()).toLowerCase());   // ⚠️ trim：git 输出可能带 \r
  });

  test('W7: 自指 —— wtBootstrap token 真的能选中本文件（否则它是永不执行的守护）', () => {
    // ⚠️ 本条只证明 --listTests 能选中本文件；token 三处连锁（contract-tests.ps1 字面量 /
    //    EXPECTED_PATTERN / TOKENS_REQUIRED_TO_SELECT）的真正闭合在 contractTestPatternBehavior 的 P1/P2/P3。
    const out = execFileSync(process.execPath,
      [JEST_ENTRY, '--config', 'jest.config.unit.js', '--listTests', '--testPathPattern', 'wtBootstrap'],
      { cwd: ROOT, encoding: 'utf8', timeout: 180000, windowsHide: true, maxBuffer: 16 * 1024 * 1024 });
    const suites = String(out).split(/\r?\n/).map((l) => l.trim()).filter((l) => l.endsWith('.test.js'));
    expect(suites.some((l) => norm(l).endsWith('utils/wtBootstrapBehavior.test.js'))).toBe(true);
  });
  // ⚠️ W8（跑入口脚本的那条）**不在本任务** —— 入口到 Task 2 才存在，它由 Task 2 Step 2 追加。
  //    同理：`ENTRY_REL` 常量也在 Task 2 追加 W8 时一并加回（本任务不引用它，留它就是死变量）。
});
