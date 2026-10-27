/**
 * 证据生成模块契约守护（scripts/lib/evidence-gen.ps1）
 *
 * 为什么重写（2026-09-29，issue #1403）：旧版是**纯文本 `toContain`**（断言源码里出现过
 * `④ 本地编译门` 之类）—— 正是本仓 `docs/spec-永绿整改.md §①` 点名的「永绿」族：把生成器改坏
 * 成多行块（校验器判「缺该行」的那一版）它照样绿，因为它从不真跑生成器、也不喂校验器。
 * 于是「dev:finish 步骤 8 产物一开 PR 即红」这个真缺陷一路漏到运行时才被发现。
 *
 * 新版按 spec §⑥「判别力」口径做**成对断言**：dot-source 真模块 → 真跑 `New-Evidence` 出文本 →
 * 取真校验器 `validatePrEvidence`（#1770 起的单一来源 `.github/scripts/lib/gate-validator.mjs`）→ 把产物喂进去。
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
// #1770 起校验器是单一来源模块；两个工作流都用 repos.getContent 取它执行。
const VALIDATOR = path.resolve(ROOT, '..', '..', '.github', 'scripts', 'lib', 'gate-validator.mjs');
const WORKFLOW = path.resolve(ROOT, '..', '..', '.github', 'workflows', 'pr-evidence.yml');

/**
 * 取真校验器（判据同源）。本文件是 CJS、模块是 ESM，故读源码剥 `export ` 后 new Function ——
 * 与工作流里的 evalValidator 同一手法（不用抽块副本，避免第二份判据）。取不到模块才退回
 * 抽 workflow 的标注块，兜底迁移期（模块是 #1770 新加的，老分支的 base 上没有）。
 */
function loadValidator() {
  if (fs.existsSync(VALIDATOR)) {
    const src = fs.readFileSync(VALIDATOR, 'utf8');
    return new Function(`${src.replace(/^export /gm, '')}\nreturn validatePrEvidence;`)();
  }
  const src = fs.readFileSync(WORKFLOW, 'utf8');
  const m = src.match(
    /\/\/ ==== PR-EVIDENCE-VALIDATOR-START ====([\s\S]*?)\/\/ ==== PR-EVIDENCE-VALIDATOR-END ====/,
  );
  if (!m) throw new Error('取不到 .github/scripts/lib/gate-validator.mjs，pr-evidence.yml 里也没有标注块');
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

// -------------------- #1920：描述符是唯一真源（pwsh 侧与 node 侧读同一段字节） --------------------

const EVIDENCE_LIB = path.join(ROOT, 'scripts', 'lib', 'gate-evidence.ps1');
const REPO_ROOT = path.resolve(ROOT, '..', '..');
const PR_TEMPLATE = path.resolve(REPO_ROOT, '.github', 'PULL_REQUEST_TEMPLATE.md');

describe('#1920 证据描述符（GATE_EVIDENCE）单一真源', () => {
  const shell = resolveShell();
  const validatorSrc = fs.readFileSync(VALIDATOR, 'utf8');
  const descriptor = new Function(`${validatorSrc.replace(/^export /gm, '')}\nreturn GATE_EVIDENCE;`)();
  const q = (p) => `'${p.replace(/\\/g, '/')}'`;

  // #2013：无控制台形态（stdout 被重定向 / dev-finish 逐套件驱动）下，子 pwsh 的
  // `[Console]::OutputEncoding` 退回本机 ANSI 码页（本机 = gb2312）⇒ 中文描述符按 GBK 写出，
  // Node 按 utf8 解码成「## ����֤��」形态。修法与 scripts/dev-finish.ps1 顶部同款：
  // 让子进程自己把输出码页钉成 UTF-8（非控制台环境赋值可能抛，try/catch 兜住），
  // 而不是把断言降级成 ASCII token —— #1920 验的就是「中文描述符两侧同源」。
  const UTF8_OUTPUT_PREAMBLE = 'try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }';

  function runPs(statements) {
    const r = spawnSync(shell, ['-NoProfile', '-Command', [UTF8_OUTPUT_PREAMBLE].concat(statements).join('; ')], {
      encoding: 'utf8',
    });
    if (r.status !== 0) throw new Error(`pwsh 退出码 ${r.status}：${(r.stderr || '').trim()}`);
    return (r.stdout || '').trim();
  }

  // 把同一份校验器文本挪到临时仓库根：改描述符 = 改那一份字节，绝不碰真仓库文件
  function tempRootWith(mutate) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'gate-desc-'));
    const dir = path.join(root, '.github', 'scripts', 'lib');
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, 'gate-validator.mjs'), mutate(validatorSrc), 'utf8');
    return root;
  }

  test('pwsh 读到的描述符与 node import 的是同一份（门/字段/免行/行形态）', () => {
    const root = tempRootWith((s) => s);
    const out = runPs([
      `. ${q(EVIDENCE_LIB)}`,
      `$d = Get-GateEvidenceDescriptor -RepoRoot ${q(root)}`,
      [
        'ConvertTo-Json -Compress -InputObject ([pscustomobject]@{',
        'heading = $d.heading;',
        'conclusionLabel = $d.conclusionLabel;',
        'separator = $d.fieldSeparator.display;',
        'prefix = $d.listItem.prefix;',
        'gates = @($d.gates | ForEach-Object { "$($_.key)|$($_.label)|$($_.fields -join \',\')|$($_.artifact)" });',
        'ex1a = $d.exemptions.' + "'①'" + '[0].text',
        '})',
      ].join(' '),
    ]);
    const got = JSON.parse(out);
    expect(got.heading).toBe(descriptor.heading);
    expect(got.conclusionLabel).toBe(descriptor.conclusionLabel);
    expect(got.separator).toBe(descriptor.fieldSeparator.display);
    expect(got.prefix).toBe(descriptor.listItem.prefix);
    expect(got.gates).toEqual(
      descriptor.gates.map((g) => `${g.key}|${g.label}|${g.fields.join(',')}|${g.artifact}`),
    );
    expect(got.ex1a).toBe(descriptor.exemptions['①'][0].text);
  });

  test('改描述符 ⇒ 生成的行跟着变（生成侧没有第二份字面量）', () => {
    const root = tempRootWith((s) =>
      s
        .replace('"display": " · "', '"display": " / "')
        .replace('"label": "npm run test:unit / test:behavior 两腿全绿"', '"label": "TEMP-LABEL"'),
    );
    const out = runPs([
      `. ${q(EVIDENCE_LIB)}`,
      `$d = Get-GateEvidenceDescriptor -RepoRoot ${q(root)}`,
      `Format-GateEvidenceRow -Descriptor $d -Gate (Get-GateEvidenceGate -Key '①' -Descriptor $d) -Values @{ '执行人'='agent 执行'; '日期'='2026-10-25'; '复测对象'='无'; '结论'='免（①a 降级发版前一次，ADR-0043）' }`,
      `Format-GateEvidenceRow -Descriptor $d -Gate (Get-GateEvidenceGate -Key '③' -Descriptor $d) -Values @{ '结论'='x' }`,
    ]);
    const [row1, row3] = out.split(/\r?\n/);
    expect(row1).toContain('Android 真机逐页截图对比');
    expect(row1).toContain(' / '); // 分隔符来自描述符，而不是生成器里写死
    expect(row1).not.toContain(' · ');
    expect(row3).toBe('- ③ TEMP-LABEL — 结论（含产物）：x');
  });

  test('生成器与预检侧不留第二份门形状字面量', () => {
    const gen = readText(path.join(ROOT, GEN_REL));
    expect(gen).not.toMatch(/- ① /);
    expect(gen).not.toMatch(/Android 真机逐页截图对比/);
    const pre = readText(path.resolve(REPO_ROOT, '.github', 'scripts', 'preflight-pr.mjs'));
    expect(pre).not.toMatch(/\['①', '②', '③'/);
    expect(pre).not.toMatch(/\\u6838\\u5fc3/); // 占位词表已不再内联（此断言只为锚住改法）
  });

  test('PR 模板的五门行与描述符同序、同结论栏名', () => {
    const tpl = readText(PR_TEMPLATE);
    const rows = tpl.split(/\r?\n/).filter((l) => /^- [①②③④]/.test(l));
    expect(rows.map((l) => l.match(/^- ([①②③④]b?)/)[1])).toEqual(descriptor.gates.map((g) => g.key));
    // 模板行允许在结论栏里加括号说明（如 ② 的「引用 MP_WEIXIN_RESULT…」），故只钉「含结论栏词干」
    for (const l of rows) {
      expect(l).toContain(descriptor.conclusionField);
      expect(l).toContain('含产物');
    }
  });
});
