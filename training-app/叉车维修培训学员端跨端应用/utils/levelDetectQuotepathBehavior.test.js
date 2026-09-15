/**
 * git 引号/八进制转义路径的**归一化**行为级守护（运行期）—— 2026-09-15
 *
 * 为什么需要：
 *   `core.quotepath=true`（git 默认）下，**非 ASCII 路径**会被整条加引号并做八进制转义，
 *   例如 `"training-app/\345\217\211.../AGENTS.md"` —— 末尾是 `md"` 而**不是** `md`
 *   ⇒ `level-detect.ps1` 里所有 `-match '\.md$' / '\.uvue$' / '\.uts$'` **一律静默失配**。
 *
 *   实测（2026-09-15）：本会话改了 4 个中文目录下的 `.md`/`.ps1`/`.js`，`Get-DetectLevel`
 *   却报「纯样式/文案改动」—— 新加的工具链分支从未命中。若只看**文本断言**，这一条根本发现不了：
 *   `levelDetectContract` 的 L3 只检查源码里出现过某些中文字面量，
 *   `levelDetectBehavior` 的临时仓库用的是**纯 ASCII 文件名**（不受 quotepath 影响）⇒ 两者都会绿。
 *
 *   两个子坑位：
 *     ① 引号：路径首尾包着 `"` ⇒ `$` 锚定的扩展名判据失配；
 *     ② 转义：`\ooo` 是**字节**，必须攒成字节数组再按 **UTF-8** 解码。
 *        直接 `[char][Convert]::ToInt32(...,8)` 会把每个 UTF-8 字节当成一个 Latin-1 字符
 *        （`叉` = E5 8F 89 ⇒ 长度 3 而非 1），路径变乱码（长度 62 vs 36）。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip**。
 */
const { execFileSync } = require('child_process');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'level-detect.ps1');

/** git 在 core.quotepath 下对 `training-app/叉车维修培训学员端跨端应用/AGENTS.md` 的真实输出 */
const QUOTED = '"training-app/\\345\\217\\211\\350\\275\\246\\347\\273\\264\\344\\277\\256\\345\\237\\271\\350\\256\\255\\345\\255\\246\\345\\221\\230\\347\\253\\257\\350\\267\\250\\347\\253\\257\\345\\272\\224\\347\\224\\250/AGENTS.md"';
const EXPECTED = 'training-app/叉车维修培训学员端跨端应用/AGENTS.md';

function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** dot-source lib、调用纯函数、回读结果（fail-closed） */
function probeUnquote(input) {
  const q = (s) => `'${String(s).replace(/'/g, "''")}'`;
  const script = [
    '$ErrorActionPreference = "Stop"',
    'Set-StrictMode -Version Latest',
    // 输出编码钉成 UTF-8，否则中文断言会因 OEM 代码页得到乱码而**假通过**
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    `. ${q(path.join(ROOT, LIB_REL))}`,
    `$out = ConvertFrom-GitQuotedPath ${q(input)}`,
    'Write-Output ("GOT=" + $out)',
    'Write-Output ("LEN=" + $out.Length)',
  ].join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: 120000,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    });
    const got = String(stdout).match(/^GOT=(.*)$/m);
    const len = String(stdout).match(/^LEN=(\d+)$/m);
    return { ok: true, got: got ? got[1] : null, len: len ? Number(len[1]) : null, stdout: String(stdout) };
  } catch (e) {
    return { ok: false, status: e.status, stdout: String(e.stdout || ''), stderr: String(e.stderr || e.message || '') };
  }
}

function mustRun(r) {
  if (!r.ok) {
    throw new Error(
      `探针执行失败（pwsh 不可用或 lib 抛错，fail-closed 不跳过）：\n`
      + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
    );
  }
}

describe('git 引号路径归一化（运行期，2026-09-15）', () => {
  test('Q1: 引号 + 八进制转义 ⇒ 还原为真实 UTF-8 路径（逐字相同）', () => {
    const r = probeUnquote(QUOTED);
    mustRun(r);
    expect(r.got).toBe(EXPECTED);
    expect(r.len).toBe(EXPECTED.length); // 36
  });

  test('Q2: 非转义（纯 ASCII）路径原样返回', () => {
    const plain = 'pages/index/index.uvue';
    const r = probeUnquote(plain);
    mustRun(r);
    expect(r.got).toBe(plain);
  });

  test('Q3: 归一化后扩展名判据可用（这正是本次缺陷的判据点）', () => {
    const r = probeUnquote(QUOTED);
    mustRun(r);
    expect(r.got.endsWith('.md')).toBe(true);
    expect(r.got.includes('"')).toBe(false);
  });
});
