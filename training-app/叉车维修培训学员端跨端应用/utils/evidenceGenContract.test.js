/**
 * 证据生成模块契约守护（scripts/lib/evidence-gen.ps1）—— A1 形态（2026-10-29，#2100）
 *
 * 做法沿用 #1403 的成对断言（**判别力**口径）：dot-source 真模块 → 真跑 `New-Evidence` 出文本 →
 * 取真校验器 `validatePrEvidence`（单一来源 `.github/scripts/lib/gate-validator.mjs`）→ 把产物喂进去。
 *
 * A1 后要锁的四件事：
 *   ① 有读数 ⇒ 生成的行能被校验器认成「已绑 sha、有读数」⇒ 绿（必不红对照组）。
 *   ② 没读数 ⇒ 仍然给出**对的行形状**（行首 + `跑于`），红因是「缺读数」而**不是「缺该行」**
 *      —— 锁住「形状对、只差填值」这个态。一旦有人把行改回多行块，③ 会红。
 *   ③ 多行块漂移 ⇒ 校验器判「缺该行」（#1403 的真实回归形状）。
 *   ④ 没 sha7 且取不到 git ⇒ 抛错，不编造 sha（编一个假 sha 就是假绿）。
 *
 * ⚠️ 依赖 pwsh（本机与 ubuntu-latest 镜像均预装 PowerShell 7）。**取不到即 fail-loud**，
 *    绝不静默 skip —— 触发条件永假的守护等于没有守护。
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const { spawnSync } = require('child_process');

jest.setTimeout(120000);

const ROOT = path.join(__dirname, '..');
const GEN = path.join(ROOT, 'scripts', 'lib', 'evidence-gen.ps1');
const VALIDATOR = path.resolve(ROOT, '..', '..', '.github', 'scripts', 'lib', 'gate-validator.mjs');

/** 取真校验器（判据同源）：CJS 读 ESM 源码剥 `export ` 后 new Function，与工作流同一手法。 */
function loadValidator() {
  const src = fs.readFileSync(VALIDATOR, 'utf8');
  return new Function(`${src.replace(/^export /gm, '')}\nreturn validatePrEvidence;`)();
}

function resolveShell() {
  for (const exe of ['pwsh', 'powershell']) {
    const r = spawnSync(exe, ['-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()'], { encoding: 'utf8' });
    if (!r.error && r.status === 0 && /\d/.test(r.stdout || '')) return exe;
  }
  throw new Error('未找到 pwsh/powershell —— 本契约测试须跑在 PowerShell 可用的环境（不静默 skip）');
}

const EXE = resolveShell();
const validate = loadValidator();

const SHA7 = 'abc1234';
const HEAD = `${SHA7}${'0'.repeat(33)}`; // 40 位；sha7 是它的前缀 ⇒ 绑定成立且不打 compare
const MP_LINE = 'MP_WEIXIN_RESULT appid=wx1 pageStack=2 entry=pages/index/index errorsTotal=0 exceptionsTotal=0 shots=2 navigation=ok';
const KOTLIN_LINE = 'KOTLIN_ALL_RESULT errors=0 classes=1261 files=88 freshness=fresh';

const RUNTIME_FILES = [
  { filename: 'training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue', status: 'modified', patch: '' },
];
const MP_FILES = [
  ...RUNTIME_FILES,
  { filename: 'training-app/叉车维修培训学员端跨端应用/manifest.json', status: 'modified', patch: '' },
];

function runGen(level, extraArgs = []) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'evgen-'));
  const out = path.join(tmp, 'evidence.md');
  const q = (s) => `'${String(s).replace(/'/g, "''")}'`;
  const script = [
    "$ErrorActionPreference = 'Stop'",
    `. ${q(GEN)}`,
    `$r = New-Evidence -Level ${q(level)} -OutputPath ${q(out)} -ProjectDir ${q(ROOT)} ${extraArgs.join(' ')}`,
    'if ($r.Generated) { Get-Content -LiteralPath $r.Path -Raw } else { "GENERATED=false`n" + $r.Message }',
  ].join('\n');
  const r = spawnSync(EXE, ['-NoProfile', '-NonInteractive', '-Command', script], {
    encoding: 'utf8',
    cwd: ROOT,
  });
  return {
    status: r.status,
    stdout: r.stdout || '',
    stderr: r.stderr || '',
    text: fs.existsSync(out) ? fs.readFileSync(out, 'utf8') : '',
  };
}

const validateWith = (files, body) => validate({ files, body, headSha: HEAD, fetchCompare: async () => null });

test('① 有读数 ⇒ 生成两行（②④），校验器判绿', async () => {
  const r = runGen('standard', [`-MpResult ${JSON.stringify(MP_LINE).replace(/"/g, "'")}`, `-CompileResult ${JSON.stringify(KOTLIN_LINE).replace(/"/g, "'")}`, `-Sha7 ${SHA7}`]);
  expect(r.status).toBe(0);
  expect(r.text).toContain('## 验收证据');
  expect(r.text).toContain(`跑于 \`${SHA7}\``);
  expect(r.text).toContain('MP_WEIXIN_RESULT');
  expect(r.text).toContain('KOTLIN_ALL_RESULT');

  const v = await validateWith(MP_FILES, r.text);
  expect(v.errors).toEqual([]);
  expect(v.ok).toBe(true);
  expect(v.gates['②'].commit).toBe(SHA7);
  expect(v.gates['④'].commit).toBe(SHA7);
});

test('①b 只给 ④ 读数且未命中 MP-WEIXIN 面 ⇒ 一行即绿（② 免）', async () => {
  const r = runGen('standard', [`-CompileResult ${JSON.stringify(KOTLIN_LINE).replace(/"/g, "'")}`, `-Sha7 ${SHA7}`]);
  expect(r.status).toBe(0);
  expect(r.text).not.toContain('MP_WEIXIN_RESULT');
  const v = await validateWith(RUNTIME_FILES, r.text);
  expect(v.ok).toBe(true);
  expect(v.gates['②'].required).toBe(false);
});

test('② 没读数 ⇒ 形状仍对：红因是「缺读数」，不是「缺该行」', async () => {
  const r = runGen('standard', [`-Sha7 ${SHA7}`]);
  expect(r.status).toBe(0);
  const v = await validateWith(RUNTIME_FILES, r.text);
  expect(v.ok).toBe(false);
  const errs = v.errors.join('\n');
  expect(errs).toContain('KOTLIN_ALL_RESULT'); // 缺读数
  expect(errs).not.toContain('缺该行'); // 行本身在
});

test('③ 读数行必须与门号**同一行**：拆成多行 ⇒ 判「没有读数」（A1 的单行是硬约束）', async () => {
  const r = runGen('standard', [`-CompileResult ${JSON.stringify(KOTLIN_LINE).replace(/"/g, "'")}`, `-Sha7 ${SHA7}`]);
  const line = r.text.split('\n').find((l) => l.startsWith('- ④'));
  expect(line).toBeTruthy();
  // 把单行拆成「门号一行 + 缩进子项」——#1403 里生成器改坏后的形态。
  // A1 下门号那行仍会被认出来（只写 `- ④` 也算该行存在），但**读数行不在同一行** ⇒ 判红。
  const drifted = r.text.replace(line, `- ④\n  - ${line.replace(/^-\s*④\s*/, '')}`);
  const v = await validateWith(RUNTIME_FILES, drifted);
  expect(v.ok).toBe(false);
  expect(v.errors.join('\n')).toContain('KOTLIN_ALL_RESULT');
});

test('④ 拿不到 sha7 且无法从 git 取 ⇒ 抛错，不编造 sha（编一个就是假绿）', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'evgen-nogit-'));
  const q = (s) => `'${String(s).replace(/'/g, "''")}'`;
  const script = [
    "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8",
    "$ErrorActionPreference = 'Stop'",
    `. ${q(GEN)}`,
    `New-Evidence -Level 'standard' -OutputPath ${q(path.join(tmp, 'e.md'))} -ProjectDir ${q(tmp)} -CompileResult ${q(KOTLIN_LINE)}`,
  ].join('\n');
  const r = spawnSync(EXE, ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', cwd: tmp });
  expect(r.status).not.toBe(0);
  // 只断言「是 New-Evidence 抛的」+ 指明了要 -Sha7：GitHub 的 pwsh 与 Windows 控制台编码不同，
  // 对中文报错整句做匹配会在 CI 上因编码差异假红（改断言，不改产品）。
  expect(`${r.stdout}${r.stderr}`).toMatch(/New-Evidence[\s\S]*-Sha7|New-Evidence[\s\S]*Sha7/);
});
