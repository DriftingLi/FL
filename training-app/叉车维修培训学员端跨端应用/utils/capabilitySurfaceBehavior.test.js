/**
 * 能力面（ADR-0016 ①b）行为级守护（运行期，非源码文本断言）
 *
 * 为什么需要这个文件：
 *   `scripts/lib/capability-surface.ps1` 是「能力面」的唯一真源，它决定 `dev:finish` 收口时打
 *   「本批触及能力面：是 / 否」—— 而这行字决定人要不要做 ①b 真机人签（ADR-0016 ⑥）。
 *   判错「是」= 白花一次人工真机冒烟；判错「否」= 人工门漏掉。
 *   所以断言的是**行为**（真跑 pwsh、造两类改动集、看返回），不是「源码里出现过某个字面量」——
 *   后者正是 ADR-0008 收束过、#1030 又踩了一次的反模式（文本存在 ≠ 行为成立）。
 *
 *   H1  白名单：指纹 / 运行时权限弹窗 / 真机上传三类各造一个真实路径 ⇒ 是，且类别正确
 *      （第一个用例用**仓库根相对**路径，钉住 `git diff --name-only` 的真实形态）
 *   H2  普通运行时面（.uvue / .uts / 文档 / 脚本 / 测试）⇒ 否
 *   H3  「否」的提示行**不含**任何「必做」类新增义务（验收标准第 3 条：不制造新的假红）
 *   H4  「是」的提示行点明 ①b 必做 + 由**人**给出原文（与 PR 模板自报框语义一致）
 *   H5  调用点扫描按**调用语法**匹配：真调用命中；仅注释提及**不**命中（#1030 的坑位）
 *   H6  四个类别名与 `.github/PULL_REQUEST_TEMPLATE.md` 的自报框**逐字**对齐（两份产物的契约）
 *   H7  `dev-finish.ps1 -DryRun` 真的打出能力面提示行（接线行为，不靠源码文本断言）
 *   H8  空改动集 / 不存在的文件不崩（Set-StrictMode -Latest 下的数组语义）
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。**不可用时 fail-closed 抛错，不 skip** ——
 * 与仓库先例 `levelDetectBehavior.test.js` / `mpWeixinGateContract.test.js:143` 一致；静默跳过等于假绿。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB_REL = path.join('scripts', 'lib', 'capability-surface.ps1');
const DEV_FINISH_REL = path.join('scripts', 'dev-finish.ps1');
const TEMPLATE_REL = path.join('..', '..', '.github', 'PULL_REQUEST_TEMPLATE.md');

/** 仓库根相对形态（本仓项目位于 `training-app/<中文目录名>/` 下，git 输出带这层前缀）。 */
const REPO_PREFIX = 'training-app/叉车维修培训学员端跨端应用/';

/** PowerShell 宿主：本仓 npm 脚本与门脚本都用 pwsh。 */
function powershellExe() {
  return 'pwsh';
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  // -OutputFormat Text：否则 stderr 被序列化成 `#< CLIXML`，失败信息不可读
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 单引号转义（PowerShell 字面量）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

function psPrelude() {
  return [
    '$ErrorActionPreference = "Stop"',
    // ⚠️ 必须先把输出编码钉成 UTF-8：脚本打的是中文，而 pwsh 被管道捕获时可能用 OEM 代码页
    //    （本机 GBK）⇒ Node 按 UTF-8 解码得到乱码，于是「含某中文子串」的断言**恒假**（假红/假绿）。
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    `. ${q(path.join(ROOT, LIB_REL))}`,
  ];
}

function runInModule(statements, timeout = 60000) {
  const script = psPrelude().concat(statements).join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    const stdout = execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout,
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

/** 运行失败即抛（fail-closed，不 skip）。 */
function mustRun(what, r) {
  if (!r.ok) {
    throw new Error(
      `${what} 执行失败（pwsh 不可用或脚本抛错，fail-closed 不跳过）：\n`
      + `exit=${r.status}\nstdout=${r.stdout}\nstderr=${r.stderr}`
    );
  }
  return r.stdout;
}

function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  return m ? m[1].trim() : null;
}

function invokeVerdict(files) {
  const list = '@(' + files.map(q).join(', ') + ')';
  return runInModule([
    `$v = Get-CapabilitySurfaceVerdict -ChangedFiles ${list}`,
    'Write-Output ("TOUCHED=" + $v.Touched)',
    'Write-Output ("HITS=" + @($v.Hits).Count)',
    'Write-Output ("CATEGORIES=" + (@($v.Categories) -join ","))',
  ]);
}

function invokeHint(files, callSitesExpr) {
  const list = '@(' + files.map(q).join(', ') + ')';
  const sites = callSitesExpr ? ` -CallSites @(${callSitesExpr})` : '';
  const out = mustRun('Format-CapabilityHint', runInModule([
    `$lines = @(Format-CapabilityHint -ChangedFiles ${list} -ProjectDir ${q(ROOT)}${sites})`,
    '$lines | ForEach-Object { Write-Output ("HINT=" + $_) }',
  ]));
  return String(out).split(/\r?\n/).filter((l) => l.startsWith('HINT=')).map((l) => l.slice(5));
}

function invokeScan(projectDir, files) {
  const list = '@(' + files.map(q).join(', ') + ')';
  const out = mustRun('Get-CapabilityCallSites', runInModule([
    `$s = @(Get-CapabilityCallSites -ChangedFiles ${list} -ProjectDir ${q(projectDir)})`,
    '$s | ForEach-Object { Write-Output ("SITE=" + $_.Path + ":" + $_.Line + ":" + $_.Api) }',
    'Write-Output ("SITECOUNT=" + @($s).Count)',
  ]));
  return {
    sites: String(out).split(/\r?\n/).filter((l) => l.startsWith('SITE=')).map((l) => l.slice(5)),
    count: Number(pick(out, 'SITECOUNT')),
  };
}

function invokeCategories() {
  const out = mustRun('Get-CapabilityCategories', runInModule([
    'Write-Output ("CATS=" + ((Get-CapabilityCategories) -join "|"))',
  ]));
  return String(pick(out, 'CATS') || '').split('|').filter(Boolean);
}

describe('capability-surface.ps1 行为级守护（运行期）', () => {
  test('H1: 白名单三类各命中一个真实路径 ⇒ 是，且类别正确', () => {
    const r = invokeVerdict([
      REPO_PREFIX + 'composables/useBiometric.uts', // 指纹（仓库根相对形态）
      'pages/profile/profile.uvue',                 // 运行时权限弹窗（chooseImage）
      'api/request.uts',                            // 真机上传（uni.uploadFile + content://）
    ]);
    const out = mustRun('Get-CapabilitySurfaceVerdict', r);
    expect(pick(out, 'TOUCHED')).toBe('True');
    expect(pick(out, 'HITS')).toBe('3');
    expect(pick(out, 'CATEGORIES')).toBe('指纹,运行时权限弹窗,真机上传');
  });

  test('H2: 普通运行时面（.uvue / .uts / 文档 / 脚本 / 测试）⇒ 否', () => {
    const r = invokeVerdict([
      'pages/home/home.uvue',
      'utils/exam-session.uts',
      'docs/adr/0016-真机门的人工性收缩与按批取证.md',
      'scripts/dev-finish.ps1',
      'utils/someFeature.test.js',
    ]);
    const out = mustRun('Get-CapabilitySurfaceVerdict', r);
    expect(pick(out, 'TOUCHED')).toBe('False');
    expect(pick(out, 'HITS')).toBe('0');
  });

  // H3 是**验收标准第 3 条**的机检：未触及能力面的普通运行时面改动，不能被这行提示凭空加上义务
  // （「不引入新的必做项」）。所以断言的是「否」的分支里**没有**任何新增义务类措辞。
  test('H3: 「否」的提示行不引入新的必做项（不制造假红）', () => {
    const lines = invokeHint(['pages/home/home.uvue', 'utils/exam-session.uts']);
    const text = lines.join('\n');
    expect(text).toContain('本批触及能力面：否');
    expect(text).toContain('无需 ①b');
    expect(text).not.toContain('必做');
    // 白名单未命中时也要把「扫描也没命中」说清，避免读者以为扫描没跑
    expect(text).toContain('原生能力调用点扫描');
    expect(text).toContain('未命中');
    // 机检行：ASCII，供守护/取证 grep（不是判据）
    expect(text).toContain('CAPABILITY_SURFACE touched=no');
  });

  // 「白名单未命中但扫描有命中」这一支尤其不能变成义务 —— 它只提醒人去自报（自报是声明，不是判据）
  test('H3b: 「否 + 扫描有命中」也只提醒自报，不新增必做项', () => {
    const lines = invokeHint(
      ['pages/home/home.uvue'],
      "@{ Path = 'pages/home/home.uvue'; Line = 12; Api = 'chooseImage' }"
    );
    const text = lines.join('\n');
    expect(text).toContain('本批触及能力面：否');
    expect(text).toContain('自报');
    expect(text).not.toContain('必做');
    expect(text).toContain('CAPABILITY_SURFACE touched=no');
  });

  test('H4: 「是」的提示行点明 ①b 必做且由人给出原文', () => {
    const lines = invokeHint(['composables/useBiometric.uts']);
    const text = lines.join('\n');
    expect(text).toContain('本批触及能力面：是');
    expect(text).toContain('①b 必做');
    expect(text).toContain('人');
    expect(text).toContain('执行人');
    expect(text).toContain('指纹');
    // 扫描**真的读了文件**：这一行若退化成「未命中」，提示行就只是装饰（收口的人看不到调用点）
    expect(text).toContain('startSoterAuthentication');
    expect(text).toContain('CAPABILITY_SURFACE touched=yes');
  });

  // H5 钉的是 #1030 的坑位：判「文件里出现过这个字符串」会把注释/文档里的**提及**也算命中。
  // 扫描器只认**调用语法**，所以「真调用」与「注释里提一句」必须分流。
  test('H5: 扫描按调用语法匹配 —— 真调用命中、注释提及不命中', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'capsurface-'));
    try {
      // ⚠️ 夹具里的「真调用」**拆两段拼**：本测试文件若原样写出 `uni` + 点 + API + 左括号，
      //    它自己就会成为扫描器的一个命中（扫描只认语法、不认语义）。这正是本用例要钉住的形态。
      const callOpen = ['uni', 'chooseImage'].join('.') + '(';
      fs.writeFileSync(path.join(tmp, 'real.uvue'), `${callOpen}{\n  count: 1\n})\n`, 'utf8');
      // 仅注释提及：本仓真实形态（api/auth.uts 等 5 个文件都长这样）
      fs.writeFileSync(path.join(tmp, 'comment.uvue'), '// uni.chooseImage 返回的临时路径\n', 'utf8');
      fs.writeFileSync(path.join(tmp, 'notes.md'), '文档里提一句 uni.chooseImage 不算调用点\n', 'utf8');

      const r = invokeScan(tmp, ['real.uvue', 'comment.uvue', 'notes.md']);
      expect(r.sites).toEqual(['real.uvue:1:chooseImage']);
      expect(r.count).toBe(1);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  // H5b 复现的是**真实收口路径**上的一次静默零命中（2026-09-16 实测）：改动集来自
  // `git diff --name-only` ⇒ **仓库根相对**路径，而扫描若只按「项目目录 + 该路径」拼接就会拼出
  // 一个不存在的路径 ⇒ `call_sites=0`，提示行看起来「跑过了」其实什么都没读。
  // 造一棵临时树（仓库根 / 项目目录两级）钉住第 ③ 个候选分支。
  test('H5b: 仓库根相对路径也要能解析（真实 git diff 形态）', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'capsurface-repo-'));
    const projRel = ['training-app', '叉车维修培训学员端跨端应用'].join('/');
    try {
      const proj = path.join(tmp, ...projRel.split('/'));
      fs.mkdirSync(path.join(proj, 'composables'), { recursive: true });
      const callOpen = ['uni', 'startSoterAuthentication'].join('.') + '(';
      fs.writeFileSync(path.join(proj, 'composables', 'useBiometric.uts'), `${callOpen}{\n})\n`, 'utf8');

      const r = invokeScan(proj, [`${projRel}/composables/useBiometric.uts`]);
      expect(r.sites).toEqual([`${projRel}/composables/useBiometric.uts:1:startSoterAuthentication`]);
      expect(r.count).toBe(1);
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });

  // H6：白名单与「自报字段」的对齐无法用执行行为证明（模板没有可执行行为），故断言的是
  // **两份产物之间的契约**：四个类别名在真源里定义一次、模板逐字包含。
  test('H6: 四个类别名与 PR 模板自报框逐字对齐', () => {
    const cats = invokeCategories();
    expect(cats).toEqual(['指纹', '运行时权限弹窗', '真机上传', '厂商 ROM 交互']);

    const template = fs.readFileSync(path.join(ROOT, TEMPLATE_REL), 'utf8');
    cats.forEach((c) => expect(template).toContain(c));
    // 模板必须指向机检口径真源，否则「自报」与「白名单」各自漂移、互相解释不通
    expect(template).toContain('capability-surface.ps1');
    expect(template).toContain('①b');
  });

  // H7：接线行为 —— `dev:finish -DryRun` 不取锁、不占设备、不跑 jest，但必须真把提示行打出来。
  // 用 -DryRun 而不是文本断言：脚本里写了调用、却把打印放进了走不到的分支，是本仓反复踩过的形态。
  test('H7: dev-finish.ps1 -DryRun 真的打出能力面提示行', () => {
    const script = [
      '$ErrorActionPreference = "Stop"',
      '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
      '$OutputEncoding = [System.Text.Encoding]::UTF8',
      `& ${q(path.join(ROOT, DEV_FINISH_REL))} -DryRun`,
    ].join('\n');
    const encoded = Buffer.from(script, 'utf16le').toString('base64');
    let out;
    try {
      out = String(execFileSync(powershellExe(), psArgs(['-EncodedCommand', encoded]), {
        cwd: ROOT,
        encoding: 'utf8',
        timeout: 120000,
        windowsHide: true,
        maxBuffer: 8 * 1024 * 1024,
      }));
    } catch (e) {
      throw new Error(`dev-finish -DryRun 失败（fail-closed 不跳过）：\n${e.stdout || ''}\n${e.stderr || e.message}`);
    }
    expect(out).toContain('不执行任何操作'); // DryRun 本体仍在
    expect(out).toContain('本批触及能力面：');
    expect(out).toMatch(/CAPABILITY_SURFACE touched=(yes|no)/);
  }, 180000);

  test('H8: 空改动集 / 不存在的文件不崩', () => {
    const r = invokeVerdict([]);
    const out = mustRun('Get-CapabilitySurfaceVerdict(空)', r);
    expect(pick(out, 'TOUCHED')).toBe('False');
    expect(pick(out, 'HITS')).toBe('0');

    // 白名单命中但文件不存在（删除的改动集）⇒ 判定仍成立（白名单只看路径），扫描跳过
    const gone = invokeHint(['composables/useBiometric.uts', 'api/request.uts']);
    expect(gone.join('\n')).toContain('本批触及能力面：是');
  });
});
