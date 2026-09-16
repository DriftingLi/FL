/**
 * level-detect.ps1 行为级守护（运行期，非源码文本断言）
 *
 * 为什么需要这个文件：
 *   `levelDetectContract.test.js`（L1–L8）全是**源码文本**断言 —— L8「空 diff → quick」
 *   只断言文件里出现过 `无改动文件（空 diff）` 这个**字符串字面量**，
 *   分支本身**从未被执行**。于是 `Set-StrictMode -Latest` 下
 *   `$null | Select-Object -Unique` ⇒ `$allFiles = $null` ⇒ `$allFiles.Count` 抛
 *   「The property 'Count' cannot be found on this object」的崩溃逃过了全部 8 条断言，
 *   直到冷启动会话按交接文档 §8 跑第一条命令（干净工作树 + `npm run dev:finish`）才暴露。
 *   ⇒ 结论：**「出现某字面量」不等于「该分支可执行」**，关键分支必须有运行期断言。
 *
 *   G1  干净工作树（本仓真实调用，本次崩溃的原始场景）：不抛错
 *   G1b 空 diff（临时干净仓库）⇒ Level=quick 且不抛错 —— 崩溃分支的确定性复现
 *   G2  单个改动文件 ⇒ 不崩，且 ChangedFiles 保持数组语义（未包 @() 会退化成标量）
 *   G3  -ForceLevel 覆盖自动判定，且 ChangedFiles 仍是数组（**且带上真实改动集**，2026-09-16）
 *   G4  只改 .ps1 ⇒ quick 且原因点名「工具链」（2026-09-15）
 *   G5  只改 .uvue ⇒ quick，但原因必须点名「验收门口径 / ①③④」，**不得**声称「未命中运行时面」（2026-09-16，#1037）
 *   G6  只改 .ts（非 .uvue 非工具链）⇒ quick 且原因写「非运行时面改动，未命中运行时面」
 *   G7  Get-QuickEvidenceHint 的行为分流：含 .uvue ⇒ 明说「不要写『免』」；否则 ⇒ 「免（未命中运行时面）」
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与仓库先例 `mpWeixinGateContract.test.js:143` 一致；静默跳过等于假绿。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'level-detect.ps1');

/** PowerShell 宿主：本仓 npm 脚本与门脚本都用 pwsh（Windows 本机与 CI runner 都自带）。 */
function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/**
 * 在 Set-StrictMode -Latest 下 dot-source 目标模块并调用 Get-DetectLevel。
 * 脚本体通过 -EncodedCommand 传入，避免与 PowerShell 的引号 / `$` 转义打架。
 * 返回 { ok, status, stdout, stderr }；失败不吞异常（fail-closed）。
 */
/** 单引号转义（PowerShell 字面量）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

/** dot-source 目标模块的前导语句（输出编码钉法见注释，别删）。 */
function psPrelude() {
  return [
    '$ErrorActionPreference = "Stop"',
    // ⚠️ 必须先把输出编码钉成 UTF-8：脚本打的是中文 `Reason`，而 pwsh 在被管道捕获时
    //    默认用 **OEM 代码页**（本机 GBK）输出 ⇒ Node 按 UTF-8 解码会得到乱码，
    //    于是「不含某中文子串」这类断言**恒真**（假通过）。ASCII-only 的断言察觉不到这个坑。
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    `. ${q(path.join(ROOT, LIB_REL))}`,
  ];
}

/**
 * 在 Set-StrictMode -Latest 下 dot-source 目标模块并执行若干语句。
 * 脚本体通过 -EncodedCommand 传入，避免与 PowerShell 的引号 / `$` 转义打架。
 * 返回 { ok, status, stdout, stderr }；失败不吞异常（fail-closed）。
 */
function runInModule(statements) {
  const script = psPrelude().concat(statements).join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  const exe = powershellExe();
  const args = psArgs(['-EncodedCommand', encoded]);
  try {
    const stdout = execFileSync(exe, args, {
      encoding: 'utf8',
      timeout: 60000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    return { ok: true, status: 0, stdout: String(stdout), stderr: '' };
  } catch (e) {
    return {
      ok: false,
      status: e.status,
      stdout: String(e.stdout || ''),
      stderr: String(e.stderr || e.message || ''),
    };
  }
}

function invokeDetectLevel(projectDir, forceLevel) {
  const force = forceLevel ? `-ForceLevel ${q(forceLevel)}` : '';
  return runInModule([
    `$r = Get-DetectLevel -ProjectDir ${q(projectDir)} ${force}`.trim(),
    'Write-Output ("LEVEL=" + $r.Level)',
    'Write-Output ("REASON=" + $r.Reason)',
    'Write-Output ("CF_ARRAY=" + ($r.ChangedFiles -is [array]))',
  ]);
}

/** 调 Get-QuickEvidenceHint（🟢 收尾建议句的单点真源，#1037）。 */
function invokeQuickHint(files) {
  const list = '@(' + files.map(q).join(', ') + ')';
  return runInModule([
    `$h = Get-QuickEvidenceHint -ChangedFiles ${list}`,
    'Write-Output ("HINT=" + $h)',
  ]);
}

function parseResult(stdout) {
  const pick = (key) => {
    const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
    return m ? m[1].trim() : null;
  };
  return { level: pick('LEVEL'), reason: pick('REASON'), cfArray: pick('CF_ARRAY') };
}

/** 建一个干净工作树的临时仓库（空 diff 场景的确定性复现）。 */
function withCleanRepo(files, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'leveldetect-'));
  const git = (...args) => {
    try {
      return execFileSync('git', args, { cwd: tmp, encoding: 'utf8', timeout: 60000 });
    } catch (e) {
      throw new Error(`临时仓库 git ${args.join(' ')} 失败：${e.stderr || e.message}`);
    }
  };
  try {
    git('init', '-q');
    git('config', 'user.email', 'test@example.com');
    git('config', 'user.name', 'test');
    files.forEach((f) => {
      const full = path.join(tmp, f);
      fs.mkdirSync(path.dirname(full), { recursive: true });
      fs.writeFileSync(full, 'x\n');
    });
    files.forEach((f) => git('add', f));
    git('commit', '-qm', 'init');
    return fn(tmp, git);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

describe('level-detect.ps1 行为级守护（运行期）', () => {
  test('G1: 干净/常规工作树下调用不抛错', () => {
    const r = invokeDetectLevel(ROOT, '');
    if (!r.ok) {
      throw new Error(
        `Get-DetectLevel 执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
        + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
      );
    }
    expect(r.stderr).toBe('');
  });

  test('G1b: 空 diff（临时干净仓库）⇒ Level=quick 且不抛错', () => {
    withCleanRepo(['a.txt'], (tmp) => {
      const r = invokeDetectLevel(tmp, '');
      if (!r.ok) {
        throw new Error(
          `空 diff 场景崩溃 —— 这正是 §8 冷启动第一条命令的故障：\n`
          + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
        );
      }
      expect(parseResult(r.stdout).level).toBe('quick');
    });
  });

  test('G2: 单个改动文件 ⇒ 不崩且 ChangedFiles 为数组', () => {
    withCleanRepo(['a.uvue'], (tmp) => {
      fs.appendFileSync(path.join(tmp, 'a.uvue'), '\n');
      const r = invokeDetectLevel(tmp, '');
      if (!r.ok) {
        throw new Error(`单文件改动场景崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
      }
      const parsed = parseResult(r.stdout);
      expect(parsed.level).toBe('quick');
      expect(parsed.cfArray).toBe('True');
    });
  });

  test('G3: -ForceLevel 覆盖自动判定且 ChangedFiles 为数组', () => {
    const r = invokeDetectLevel(ROOT, 'full');
    if (!r.ok) {
      throw new Error(`-ForceLevel 场景崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
    }
    const parsed = parseResult(r.stdout);
    expect(parsed.level).toBe('full');
    expect(parsed.cfArray).toBe('True');
  });

  // G4/G5（2026-09-15）：🟢 的 `Reason` 必须**区分**「工具链/测试改动」与「纯样式/文案改动」。
  // 文本断言只能证明「源码里出现过那两句话」，证明不了「哪一类改动真的走哪个分支」——
  // 这里用临时仓库真跑一遍（G4 只改 .ps1 / G5 只改 .uvue），钉住分支选择。
  test('G4: 只改 .ps1（未命中运行时面）⇒ quick 且原因点名「工具链」', () => {
    withCleanRepo(['run.ps1'], (tmp) => {
      fs.appendFileSync(path.join(tmp, 'run.ps1'), '\n# tweak\n');
      const r = invokeDetectLevel(tmp, '');
      if (!r.ok) throw new Error(`工具链改动场景崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
      const parsed = parseResult(r.stdout);
      expect(parsed.level).toBe('quick');
      expect(parsed.reason).toContain('工具链/测试改动');
      // 关键：**不得**把它说成样式改动（那正是本次要修的误导）
      expect(parsed.reason).not.toContain('纯样式');
    });
  });

  // G5 于 2026-09-16 改写（#1037）：**旧断言恰好锁着这个 bug** —— 它要求 `.uvue` 的 Reason 里出现
  // 「未命中运行时面」，而 `.uvue` 在验收门口径（`pr-evidence` 的 `isRuntimeFile`）里就是运行时面。
  // 旧文案于是被两条断言（文本 L3 + 行为 G5）一起钉死，改文案必须先改断言 —— 这正是本次要做的。
  test('G5: 只改 .uvue ⇒ quick，但原因必须点名验收门口径，且不得声称「未命中运行时面」', () => {
    withCleanRepo(['pages/home/home.uvue'], (tmp) => {
      fs.appendFileSync(path.join(tmp, 'pages/home/home.uvue'), '\n<!-- tweak -->\n');
      const r = invokeDetectLevel(tmp, '');
      if (!r.ok) throw new Error(`样式改动场景崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
      const parsed = parseResult(r.stdout);
      expect(parsed.level).toBe('quick');
      expect(parsed.reason).toContain('验收门口径');
      expect(parsed.reason).toContain('①③④');
      // 关键：**不得**再说「未命中运行时面」（那正是本次要修的误导）
      expect(parsed.reason).not.toContain('未命中运行时面');
      expect(parsed.reason).not.toContain('工具链');
    });
  });

  test('G6: 只改 .ts（非 .uvue、非工具链）⇒ quick 且原因写「非运行时面改动，未命中运行时面」', () => {
    withCleanRepo(['utils/a.ts'], (tmp) => {
      fs.appendFileSync(path.join(tmp, 'utils/a.ts'), '\n// tweak\n');
      const r = invokeDetectLevel(tmp, '');
      if (!r.ok) throw new Error(`非运行时面场景崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
      const parsed = parseResult(r.stdout);
      expect(parsed.level).toBe('quick');
      expect(parsed.reason).toContain('非运行时面改动');
      expect(parsed.reason).toContain('未命中运行时面');
      expect(parsed.reason).not.toContain('工具链');
    });
  });

  // G7（2026-09-16，#1037）：🟢 收尾建议句的**行为**决定人往 PR 正文里写什么 ——
  // 写错就让一个本该绿的 PR 判红（「免（低风险运行时面：仅 .uvue 样式/文案改动）」就是这么来的）。
  // 纯函数，不需要临时仓库。
  test('G7: Get-QuickEvidenceHint 按改动集分流', () => {
    const hintOf = (files) => {
      const r = invokeQuickHint(files);
      if (!r.ok) throw new Error(`QuickHint(${files.join(',')}) 崩溃：\nexit=${r.status}\nstderr=${r.stderr}`);
      const m = String(r.stdout).match(/^HINT=(.*)$/m);
      if (!m) throw new Error(`QuickHint(${files.join(',')}) 没打出 HINT 行：${r.stdout}`);
      return m[1];
    };

    const uvueHint = hintOf(['pages/home/home.uvue']);
    expect(uvueHint).toContain('不要写「免」');
    expect(uvueHint).toContain('运行时面');
    expect(uvueHint).not.toContain('免（未命中运行时面）');

    expect(hintOf(['utils/a.ts'])).toContain('免（未命中运行时面）');
    expect(hintOf([])).toContain('免（未命中运行时面）');
  });
});
