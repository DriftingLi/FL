/**
 * #2161 —— ④c 出口的「证据自查」（`scripts/lib/evidence-selfcheck.ps1`）· **行为级**守护。
 *
 * ## 新增守护四问（docs/agents/guards.md，逐条作答）
 *
 * 1. **哪一类**：行为守护 —— 真起 pwsh 子进程、真走 `Invoke-Process` 采集链、真跑 #2139 判据 CLI；
 *    `node scripts/classify-guards.mjs` 会因 `execFileSync` + 仓内 `.ps1`/`.mjs` 载体把它归进行为腿。
 * 2. **成对断言在哪**：腿 1「照 lib 传参 ⇒ 必不红」（fffd=0 / status=clean）与腿 2「拔掉那条 JVM 参数 ⇒
 *    必红」（fffd>0 / status=corrupt）。判别力落在**字节形态由 lib 的返回值决定**上：谁把
 *    `-Dfile.encoding=UTF-8` 从 `Get-KotlinJvmUtf8Args` 里删掉，腿 1 当场从 clean 掉成 corrupt ——
 *    票面 AC「拔掉设置即红」由机器举证，不靠人读 diff。
 * 3. **接线由谁兜底**：门是否真的钉了三性、是否真的在 `finally` 里调用本 lib，由
 *    `utils/kotlinAllGateContract.test.js` 的 C11 钉接线；行为（这套东西真能区分好坏证据）由本文件兜底。
 * 4. **吃不吃得到环境**：只吃 `node` + `pwsh`（CI 的 ubuntu runner 两者都有），**不吃 HBuilderX/Corretto**
 *    ⇒ 不会长成 #2078 那种「本地绿、CI 红」。反过来「环境缺失」两支（判据 CLI 找不到 / node 找不到）
 *    都在本机造得出来 ⇒ 腿 3，且断言的是 fail-closed 的 `unknown`，不是 `clean`。
 *
 * ## 已知边界（照票面「待确认」裁定，不掩盖）
 *
 * JVM 侧的真实字节只有本机 ④c 现跑才拿得到（现测 2026-11-02：中文工作树 + Corretto 17 + kotlinc，
 * 加 `-Dfile.encoding=UTF-8` 后 kotlin-all.log 的 fffd 由 18 → 0）；那条读数行原样进 PR 正文。
 * CI 上没有 HBuilderX/Corretto，所以腿 1/2 用 node 子进程当「会按指定码页写 stdout 的替身子进程」，
 * 量的仍是同一条采集链（`StandardOutputEncoding=UTF8` → `Add-Content -Encoding utf8` → 判据）——
 * 缺陷正是这条链把字节烤成不可逆的 U+FFFD，替身子进程不削弱这一环。
 *
 * ## 夹具为什么全用字节数组
 *
 * 同 `utils/evidenceCaptureEncodingBehavior.test.js`：U+FFFD 字面量在本仓写入链实测会被整字吞掉，
 * 故被测字节与被测字符一律用 `[byte[]]` 数字数组 / 码位数组表达，不依赖非 ASCII 字面量活着到达磁盘。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。不可用时 `drive()` 抛错 ⇒ 本文件红，**不 skip**
 * （静默跳过等于假绿，与 `utils/kotlinAllProcessCaptureBehavior.test.js` 同一条纪律）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { encodeCommand } = require('./probeSpawn.js');

const ROOT = path.join(__dirname, '..');
const LIB_CAPTURE = path.join(ROOT, 'scripts', 'lib', 'process-capture.ps1');
const LIB_SELFCHECK = path.join(ROOT, 'scripts', 'lib', 'evidence-selfcheck.ps1');
const CLI_PATH = path.join(ROOT, '..', '..', '.github', 'scripts', 'check-evidence-encoding.mjs');
const DRIVE_TIMEOUT_MS = 120000;

jest.setTimeout(300000);


// 「叉车维修培训学员端跨端应用」= ④c 工作树路径里那段中文，正是缺陷的触发面。
// UTF8_BYTES：同一串的 UTF-8 字节 —— JVM 拿到 -Dfile.encoding=UTF-8 时该写出来的东西。
const UTF8_BYTES = [229,143,137,232,189,166,231,187,180,228,191,174,229,159,185,232,174,173,229,173,166,229,145,152,231,171,175,232,183,168,231,171,175,229,186,148,231,148,168];
// GBK_BYTES：同一串的 GBK(cp936) 字节（本机现测 OEM 码页 = 936，且这 26 个字节按 UTF-8 fatal
// 解码必失败）—— JVM 不设该参数、按码页写 stdout 时真正落到管道里的东西。
const GBK_BYTES = [178,230,179,181,206,172,208,222,197,224,209,181,209,167,212,177,182,203,191,231,182,203,211,166,211,195];
// 干净腿的期望码位（逐字来自上面那串的 codePointAt，不是凭记忆抄的）。
const TARGET_CODEPOINTS = [21449,36710,32500,20462,22521,35757,23398,21592,31471,36328,31471,24212,29992];
const FLAG = '-Dfile.encoding=UTF-8';

// JS 数组字面量 <-> PowerShell 数组主体：只摘方括号，别把 [...] 直接拼到 @ 后面。
function ARRAY_BODY(list) {
  return list.join(',');
}

function q(value) {
  return "'" + String(value).replace(/'/g, "''") + "'";
}

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') {
    args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  }
  return args.concat(extra);
}

// 真起一个 pwsh：dot-source 采集层 + 本次新 lib，然后执行 statements。
// 与先例同一手法（-EncodedCommand + UTF-16LE base64），免引号地狱，也免落盘临时脚本。
function drive(statements) {
  const prelude = [
    '$ErrorActionPreference = "Stop"',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    '. ' + q(LIB_CAPTURE),
    '. ' + q(LIB_SELFCHECK),
    '$Node = ' + q(process.execPath),
    '$Cli = ' + q(CLI_PATH),
    '$Proj = ' + q(ROOT),
    // PowerShell 里 [byte[]]@[…] 会被当成 splatting（现测 ParserError），必须写成 [byte[]]@(…)
    '$Utf8Bytes = [byte[]]@(' + ARRAY_BODY(UTF8_BYTES) + ')',
    '$GbkBytes = [byte[]]@(' + ARRAY_BODY(GBK_BYTES) + ')',
  ];
  const script = prelude.concat(statements).join('; ');
  const encoded = encodeCommand(script);
  return execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
    encoding: 'utf8',
    timeout: DRIVE_TIMEOUT_MS,
    windowsHide: true,
    maxBuffer: 8 * 1024 * 1024,
  });
}

function pick(stdout, key) {
  const m = stdout.match(new RegExp('^' + key + '=(.*)$', 'm'));
  if (!m) {
    throw new Error('驱动脚本没有输出 ' + key + '= 读数行：' + '\n' + + stdout);
  }
  return m[1].trim();
}

const TMP_DECL = [
  "$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('fl2161-' + [guid]::NewGuid().ToString('N'))",
  "New-Item -ItemType Directory -Force -Path (Join-Path $tmp '.ci-verify') | Out-Null",
].join('; ');

// 采集腿：按 lib 给的 JVM 参数决定子进程写哪种字节 —— 参数被删掉，字节就跟着变，产物就坏。
function captureStatements(stripFlag) {
  const argsFromLib = '$jvmArgs = @(Get-KotlinJvmUtf8Args)';
  const strip = stripFlag
    ? "$jvmArgs = @($jvmArgs | Where-Object { $_ -ne '" + FLAG + "' })"
    : '';
  return [
    TMP_DECL,
    "$log = Join-Path (Join-Path $tmp '.ci-verify') 'kotlin-all.log'",
    '$gateStart = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()',
    argsFromLib,
    strip,
    "$hasFlag = @($jvmArgs) -contains '" + FLAG + "'",
    '$bytes = if ($hasFlag) { $Utf8Bytes } else { $GbkBytes }',
    "$js = Join-Path $tmp 'emit.js'",
    "Set-Content -LiteralPath $js -Encoding ascii -Value ('process.stdout.write(Buffer.from([' + ($bytes -join ',') + ']))')",
    "$r = Invoke-Process -FilePath $Node -Arguments @($js) -TimeoutSeconds 60 -Tag 'jvm-standin'",
    // 门的落盘写法：采集层拿到的字符串按 UTF-8 写盘（烤进去就不可逆，正是 #2161 的现场）
    'Add-Content -LiteralPath $log -Value $r.Output -Encoding utf8',
    '$txt = [System.IO.File]::ReadAllText($log, [System.Text.Encoding]::UTF8)',
    "'FLAG=' + $hasFlag",
    "'RAWN=' + $bytes.Count",
    "'FFFD=' + ([regex]::Matches($txt, [string][char]0xFFFD)).Count",
    // 码位只取可见字符（[int]$_ -gt 32 挡掉行尾 CRLF），免在脚本里写转义序列
    "'CP=' + ((@($txt.ToCharArray() | Where-Object { [int]$_ -gt 32 }) | ForEach-Object { [int]$_ }) -join ',')",
    '$v = Get-EvidenceLogVerdict -Root $tmp -SinceMs $gateStart -CliPath $Cli -NodePath $Node',
    "'STATUS=' + $v.Status",
    "'OK=' + $v.Ok",
    "'VFFFD=' + $v.Fffd",
    "'BADC=' + @($v.Bad).Count",
    "'BADFIRST=' + (@($v.Bad) | Select-Object -First 1)",
    "'SUMMARY=' + $v.Summary",
    'Remove-Item -Recurse -Force $tmp',
  ].filter(Boolean);
}

describe('#2161 ④c 证据自查（真采集链 + 真 #2139 判据）', () => {
  afterAll(() => {
    // 临时件按前缀清掉（同先例纪律）
    const base = os.tmpdir();
    let names = [];
    try {
      names = fs.readdirSync(base).filter((n) => n.startsWith('fl2161-'));
    } catch (e) {
      names = [];
    }
    for (const n of names) {
      try {
        fs.rmSync(path.join(base, n), { recursive: true, force: true });
      } catch (e) {
        /* 清不干净不影响判据 */
      }
    }
  });

  it('真件：门照 lib 传参 ⇒ 产物零 U+FFFD、判据 clean（必不红）', () => {
    const out = drive(captureStatements(false));
    // 这条断言是「拔掉设置即红」的支点：字节形态由 lib 的返回值选，lib 没了参数就走坏字节支
    expect(pick(out, 'FLAG')).toBe('True');
    expect(pick(out, 'RAWN')).toBe(String(UTF8_BYTES.length));
    expect(pick(out, 'FFFD')).toBe('0');
    expect(pick(out, 'CP').split(',').map(Number)).toEqual(TARGET_CODEPOINTS);
    expect(pick(out, 'STATUS')).toBe('clean');
    expect(pick(out, 'OK')).toBe('True');
    expect(pick(out, 'VFFFD')).toBe('0');
    expect(pick(out, 'BADC')).toBe('0');
    expect(pick(out, 'SUMMARY')).toContain('KOTLIN_ALL_READABLE status=clean');
  });

  it('变异正控：拔掉 -Dfile.encoding=UTF-8 ⇒ 同一链路烤出 U+FFFD、判据 corrupt 且点名文件（必红）', () => {
    const out = drive(captureStatements(true));
    expect(pick(out, 'FLAG')).toBe('False');
    expect(pick(out, 'RAWN')).toBe(String(GBK_BYTES.length));
    expect(Number(pick(out, 'FFFD'))).toBeGreaterThan(0);
    expect(pick(out, 'STATUS')).toBe('corrupt');
    expect(pick(out, 'OK')).toBe('False');
    // 门自己点名：坏件在输出里有名有姓，不再等 preflight 报成「缺该行」
    expect(Number(pick(out, 'BADC'))).toBeGreaterThanOrEqual(1);
    expect(pick(out, 'BADFIRST')).toContain('kotlin-all.log');
    expect(pick(out, 'BADFIRST')).toContain('B2-replacement-char');
    expect(pick(out, 'SUMMARY')).toContain('KOTLIN_ALL_READABLE status=corrupt');
  });

  it('fail-closed：判据 CLI / node 不可用、或树根不存在 ⇒ unknown，绝不读成 clean', () => {
    const statements = [
      TMP_DECL,
      "$log = Join-Path (Join-Path $tmp '.ci-verify') 'kotlin-all.log'",
      "Set-Content -LiteralPath $log -Encoding utf8 -Value 'KOTLIN_ALL_RESULT errors=0'",
      '$a = Get-EvidenceLogVerdict -Root $tmp -CliPath (Join-Path $tmp nope.mjs) -NodePath $Node',
      "'A=' + $a.Status + '/' + $a.Ok",
      '$b = Get-EvidenceLogVerdict -Root $tmp -CliPath $Cli -NodePath (Join-Path $tmp nope-node)',
      "'B=' + $b.Status + '/' + $b.Ok",
      '$c = Get-EvidenceLogVerdict -Root (Join-Path $tmp nonexistent-tree) -CliPath $Cli -NodePath $Node',
      "'C=' + $c.Status + '/' + $c.Ok",
      'Remove-Item -Recurse -Force $tmp',
    ];
    const out = drive(statements);
    expect(pick(out, 'A')).toBe('unknown/False');
    expect(pick(out, 'B')).toBe('unknown/False');
    // 树根不存在：#2139 判据本体就是抛错（拒绝把「扫不到」读成「全干净」），自查层收成 unknown
    expect(pick(out, 'C')).toBe('unknown/False');
  });

  it('跨平台定位：从中文项目目录真走上去能拿到判据 CLI（Linux 路径拼接不留反斜杠字面量）', () => {
    const out = drive([
      '$cli = Resolve-EvidenceEncodingCli -StartDir $Proj',
      "'RESOLVED=' + $cli",
      "'EXISTS=' + [bool](Test-Path -LiteralPath $cli -PathType Leaf)",
    ]);
    const resolved = pick(out, 'RESOLVED');
    expect(resolved).not.toBe('');
    expect(pick(out, 'EXISTS')).toBe('True');
    expect(path.basename(resolved)).toBe('check-evidence-encoding.mjs');
    expect(fs.existsSync(CLI_PATH)).toBe(true);
  });
});
