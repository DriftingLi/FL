/**
 * 证据生成模块契约守护（scripts/lib/evidence-gen.ps1）
 *
 * 为什么重写（2026-09-29，issue #1403）：旧版是**纯文本 `toContain`**（断言源码里出现过
 * `④ 本地编译门` 之类）—— 正是本仓 `docs/spec-永绿整改.md §①` 点名的「永绿」族：把生成器改坏
 * 成多行块（校验器判「缺该行」的那一版）它照样绿，因为它从不真跑生成器、也不喂校验器。
 * 于是「dev:finish 步骤 8 产物一开 PR 即红」这个真缺陷一路漏到运行时才被发现。
 *
 * 新版按 spec §⑥「判别力」口径做**成对断言**：dot-source 真模块 → 真跑 `New-Evidence` 出文本 →
 * 从 `pr-evidence.yml` 抽出真校验器 `validatePrEvidence` → 把产物喂进去。
 *   ① 必不红对照组：补全真产物（CI run 链接 / 仓库内截图 / 编译日志）⇒ 校验器判绿。
 *   ② 必红·结构漂移：退回旧「多行块」⇒ 校验器判「缺该行」（证明这锁抓得住形状漂移）。
 *   ③ 必红·等证据：开 PR 前的默认产物（①③④ 结论留「待补」）⇒ 判红，但**只因缺产物、不再「缺该行」**
 *      （锁住「结构正确、只剩填值」这个修复态；一旦有人把行结构改回多行，② 会红）。
 *   ④ (c) 永绿销账：空编译结果不得自称 `COMPILE_RESULT errors=0`（源码不含该硬编码，且 ④ 结论如实判红）。
 *
 * ⚠️ 本测试依赖 pwsh（GitHub ubuntu-latest 镜像与本机均预装 PowerShell 7）。**取不到 pwsh 即 fail-loud**
 *   （绝不静默 skip —— 触发条件永假的守护等于没有守护，spec §④ 更正 3）。
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const { spawnSync } = require('child_process');
const { readText } = require('./utsHarness');

jest.setTimeout(120000);

const ROOT = path.join(__dirname, '..');
const GEN_REL = 'scripts/lib/evidence-gen.ps1';
const WORKFLOW = path.resolve(ROOT, '..', '..', '.github', 'workflows', 'pr-evidence.yml');

/** 从 workflow 抽出内联校验器（与 .github/scripts/pr-evidence-check.test.mjs 同一抽取式）。 */
function loadValidator() {
  const src = fs.readFileSync(WORKFLOW, 'utf8');
  const m = src.match(
    /\/\/ ==== PR-EVIDENCE-VALIDATOR-START ====([\s\S]*?)\/\/ ==== PR-EVIDENCE-VALIDATOR-END ====/,
  );
  if (!m) throw new Error('在 pr-evidence.yml 里找不到 PR-EVIDENCE-VALIDATOR 标记块');
  return new Function(`${m[1]}\nreturn validatePrEvidence;`)();
}

function resolveShell() {
  for (const exe of ['pwsh', 'powershell']) {
    const r = spawnSync(exe, ['-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()'], {
      encoding: 'utf8',
    });
    if (!r.error && r.status === 0 && /\d/.test(r.stdout || '')) return exe;
  }
  throw new Error('未找到 pwsh/powershell —— 本契约测试须跑在 PowerShell 可用的环境（不静默 skip）');
}

// 命中运行时面的移动端 .uvue（与校验器 isRuntimeFile 同判据）；未命中 MP-WEIXIN 面 ⇒ 免 ②
const RUNTIME_FILES = [
  {
    filename: 'training-app/叉车维修培训学员端跨端应用/pages/exam/exam.uvue',
    status: 'modified',
    patch: '',
  },
];
const VALIDATE_OPTS = { author: 'zhengcookie', headSha: 'a'.repeat(40), fetchCompare: async () => null };

// 旧版「多行块」产物（门号单独一行、字段拆成缩进 bullet）—— 校验器按「以门号开头的行」取门，抓不到 ⇒ 缺该行
const OLD_MULTILINE = [
  '## 改了什么',
  '拆页。',
  '',
  '## 验收证据',
  '',
  '④ 本地编译门（④c）：',
  '- 执行人：agent 执行',
  '- 日期：2026-09-29',
  '- 复测对象：npm run build:kotlin-all',
  '- 结论：KOTLIN_ALL_RESULT errors=0 日志 .ci-verify/kotlin-all.log',
].join('\n');

describe('evidence-gen.ps1 与 pr-evidence 校验器的对齐（真跑 + 真校验，成对断言）', () => {
  const validate = loadValidator();
  let greenBody = '';
  let todoBody = '';

  beforeAll(() => {
    const shell = resolveShell();
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'evidence-gen-'));
    const genOut = (n) => path.join(tmp, n).replace(/\\/g, '/');
    const greenPath = genOut('green.txt');
    const todoPath = genOut('todo.txt');

    // pwsh 侧：dot-source 真模块 → 跑两次 New-Evidence → 落 UTF-8 文本供 node 读取。
    // 每条命令写成整行（不用 PowerShell 续行反引号），动态值经 JSON.stringify 生成带引号字面量。
    const enc = 'New-Object System.Text.UTF8Encoding($false)';
    const ps = [
      '$ErrorActionPreference = "Stop"',
      `$gen = ${JSON.stringify(path.join(ROOT, GEN_REL).replace(/\\/g, '/'))}`,
      `$root = ${JSON.stringify(ROOT.replace(/\\/g, '/'))}`,
      '. "$gen"',
      `$c = New-Evidence -Level full -CompileResult "KOTLIN_ALL_RESULT errors=0 classes=1261，日志 .ci-verify/kotlin-all.log" -ScreenshotDiff @("exam.uvue:有变化","home.uvue:无变化") -ScreenshotChangedCount 1 -ProjectDir $root -OutputPath ${JSON.stringify(genOut('u1.md'))} -CiRunUrl "https://github.com/DriftingLi/FL/actions/runs/123456789" -ScreenshotRelPath "docs/verification/exam/624/exam-home-after.jpg"`,
      `[System.IO.File]::WriteAllText(${JSON.stringify(greenPath)}, $c.Content, (${enc}))`,
      `$t = New-Evidence -Level full -CompileResult "" -ScreenshotDiff @("exam.uvue:有变化") -ScreenshotChangedCount 1 -ProjectDir $root -OutputPath ${JSON.stringify(genOut('u2.md'))}`,
      `[System.IO.File]::WriteAllText(${JSON.stringify(todoPath)}, $t.Content, (${enc}))`,
    ].join('\n');

    const ps1 = path.join(tmp, 'run.ps1');
    // 带 BOM：PowerShell 对含 CJK 的 .ps1 按 UTF-8 解析（无 BOM 时 Windows 端可能按本地码页乱码）
    fs.writeFileSync(ps1, Buffer.concat([Buffer.from([0xef, 0xbb, 0xbf]), Buffer.from(ps, 'utf8')]));

    const r = spawnSync(shell, ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ps1], {
      encoding: 'utf8',
    });
    if (r.status !== 0) {
      throw new Error(`pwsh 生成证据失败(status=${r.status}):\n${r.stdout || ''}\n${r.stderr || ''}`);
    }
    greenBody = fs.readFileSync(greenPath, 'utf8');
    todoBody = fs.readFileSync(todoPath, 'utf8');
  });

  test('① 必不红：补全真产物（CI run + 仓库截图 + 编译日志）⇒ 校验器判绿', async () => {
    const res = await validate({ files: RUNTIME_FILES, body: `## 改了什么\n拆页。\n\n${greenBody}\n`, ...VALIDATE_OPTS });
    expect(res.errors).toEqual([]);
    expect(res.ok).toBe(true);
  });

  test('② 必红·结构漂移有牙齿：退回旧「多行块」⇒ 校验器判「缺该行」', async () => {
    const res = await validate({ files: RUNTIME_FILES, body: `${OLD_MULTILINE}\n`, ...VALIDATE_OPTS });
    expect(res.ok).toBe(false);
    expect(res.errors.some((e) => /缺该行/.test(e))).toBe(true);
  });

  test('③ 修复态：新产物结构完整（无「缺该行」），只剩「等证据」', async () => {
    const res = await validate({ files: RUNTIME_FILES, body: `## 改了什么\n拆页。\n\n${todoBody}\n`, ...VALIDATE_OPTS });
    // 开 PR 前产物本就该红（①③④ 结论是「待补」）—— 但错误必须全是「须引用产物」，不得再有结构级「缺该行」
    expect(res.ok).toBe(false);
    expect(res.errors.some((e) => /缺该行/.test(e))).toBe(false);
    // 三条都在（③ 行确实被生成器补齐了：旧版根本不发 ③）
    expect(res.errors.some((e) => e.startsWith('①'))).toBe(true);
    expect(res.errors.some((e) => e.startsWith('③'))).toBe(true);
    expect(res.errors.some((e) => e.startsWith('④'))).toBe(true);
  });

  test('④ (c) 永绿销账：空编译结果不自称 errors=0，源码不含该硬编码', async () => {
    const src = readText(path.join(ROOT, GEN_REL));
    expect(src).not.toMatch(/COMPILE_RESULT errors=0/);
    // 空 CompileResult 的 ④ 结论落「待补」——不含 kotlin-all.log/build.log 字面，故不得假绿
    const line4 = todoBody.split('\n').find((l) => /^\s*-\s*④/.test(l));
    expect(line4).toBeTruthy();
    expect(line4).not.toMatch(/kotlin-all\.log/);
    expect(line4).not.toMatch(/build\.log/);
  });
});
