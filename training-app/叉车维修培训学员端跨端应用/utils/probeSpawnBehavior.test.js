/**
 * #2252 共享探针工具的行为守护。
 *
 * ## 为什么是行为守护，不是接线守护
 *
 * 判据的真身是「真 pwsh 子进程吐出来的字节，父侧按 utf8 解码后是什么」。只有真起子进程才看得见。
 * `execFileSync` 调用刻意留在这个文件自身：③ 门按**文件自身**有无代码级执行调用判腿
 * （`scripts/classify-guards.mjs`），把执行搬进 `utils/probeSpawn.js` 会让本文件退化成接线守护，
 * `utils/guardClassification.test.js` 的 H4 当场转红。
 *
 * ## 判别力在哪
 *
 * 必绿腿证明「走工具 ⇒ 中文逐字完整」；必红腿证明「不走工具 ⇒ 同一段字节必然烤成 U+FFFD」。
 * 后者不能写成「把自律行删掉再跑」：自律行在 UTF-8 控制台（Linux CI、英文 Windows）上本来就是
 * no-op，删掉也不出 U+FFFD —— 那种写法只有中文 Windows 本机能红，CI 上必红（#2078 的
 * 「本地永远绿、CI 永远红」正是这个形状）。所以必红腿直接把「子进程没按 UTF-8 写」的产物喂给
 * 父侧解码器：cp936 的四个字节（UTF-8 的「中文」是 E4 B8 AD E6 96 87），任何平台都确定反复现。
 *
 * ## 夹具为什么全是码位
 *
 * U+FFFD 等非 ASCII 字面量会在写入链上被整字吞掉、落盘成空串（#2139）。一律用 `String.fromCharCode`
 * 与 `[char]0xXXXX` 现造。
 *
 * ## 不给 skip 留口子
 *
 * `pwsh` 起不来就必须红：探针工具的判据只在真子进程上成立，跳过等于把守护静音。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const { readText } = require('./utsHarness');
const { UTF8_DISCIPLINE, withUtf8Discipline, encodeCommand, EXEC_OPTIONS } = require('./probeSpawn.js');

const ROOT = path.join(__dirname, '..');
const ZHONG_WEN = String.fromCharCode(0x4e2d, 0x6587);
const FFFD = String.fromCharCode(0xfffd);

jest.setTimeout(300000);

/** 探针的 pwsh 参数：与仓内既有探针一致（不做 `-OutputFormat`，那是各测试自己的形状）。 */
function psArgs(extra) {
  const platform = process.platform === 'win32' ? ['-ExecutionPolicy', 'Bypass'] : [];
  return ['-NoProfile', '-NonInteractive'].concat(platform, extra);
}

/**
 * 起一次真子进程。不带任何 UTF-8 自律行的「裸」路径也走这里 ——
 * 必红腿要的正是「工具之外」那条路。
 */
function run(args) {
  try {
    return { status: 0, stdout: String(execFileSync('pwsh', args, EXEC_OPTIONS)), stderr: '' };
  } catch (e) {
    if (e.status === null || e.status === undefined) {
      throw new Error(`pwsh 没能真跑起来（探针守护不许 skip）：${e.message}`);
    }
    return { status: e.status, stdout: String(e.stdout || ''), stderr: String(e.stderr || '') };
  }
}

/** 用工具编码后的脚本起子进程（`-EncodedCommand` 路）。 */
function runViaTool(statements) {
  return run(psArgs(['-EncodedCommand', encodeCommand(statements)]));
}

const WRITE_ZHONG_WEN = `[string]([char]0x4E2D + [char]0x6587)`;
const THROW_ZHONG_WEN = `throw ([string]([char]0x4E2D + [char]0x6587))`;

// cp936 的「中文」= D6 D0 CE C4；按 utf8 解会被切碎成多个 U+FFFD。用字节数组写死，
// 不依赖 .NET 的 CodePages provider（Linux 上未必注册），也不需要任何非 ASCII 字面量。
const GBK_BYTES = '0xD6,0xD0,0xCE,0xC4';
const RAW_GBK_SCRIPT = [
  `[byte[]]$b = @(${GBK_BYTES})`,
  '$s = [Console]::OpenStandardOutput()',
  '$s.Write($b, 0, $b.Length)',
  '$s.Flush()',
].join('; ');

const countFFFD = (s) => String(s).split('').filter((c) => c === FFFD).length;

test('必绿1：工具编码的脚本起真子进程写中文，父侧逐字拿到（stdout 路）', () => {
  const r = runViaTool([WRITE_ZHONG_WEN]);
  expect(r.status).toBe(0);
  expect(r.stdout.trim()).toBe(ZHONG_WEN);
  expect(countFFFD(r.stdout)).toBe(0);
});

test('必绿2：同一路上抛出的中文异常，stderr 同样逐字完整', () => {
  const r = runViaTool([THROW_ZHONG_WEN]);
  expect(r.status).not.toBe(0);
  expect(r.stderr).toContain(ZHONG_WEN);
  expect(countFFFD(r.stderr)).toBe(0);
});

test('必绿3：-Command 路（自律行直接拼在正文前）同样逐字完整', () => {
  // 台账里 -Command 有 17 处，走的是「自律行 + 正文」拼成一个参数字符串，不是 base64。
  const r = run(psArgs(['-Command', withUtf8Discipline([WRITE_ZHONG_WEN])]));
  expect(r.status).toBe(0);
  expect(r.stdout.trim()).toBe(ZHONG_WEN);
  expect(countFFFD(r.stdout)).toBe(0);
});

test('必红·变异腿：不走工具的子进程吐 cp936 字节，父侧按 EXEC_OPTIONS 解码必出 2 个以上 U+FFFD', () => {
  // 裸路径：直接 -Command，不带 UTF-8 自律行 —— 模拟「忘了前置」的那个生产者。
  const r = run(psArgs(['-Command', RAW_GBK_SCRIPT]));
  expect(r.status).toBe(0);
  // 父侧的解码器就是工具收口的那一个：判据落在「同一套选项下，没走 UTF-8 的字节会烂掉」。
  expect(countFFFD(r.stdout)).toBeGreaterThanOrEqual(2);
  expect(r.stdout).not.toContain(ZHONG_WEN);
});

test('自律行与仓内权威门脚本逐字同款，且工具产出的脚本体确实以它开头（防两份各自漂移）', () => {
  // 读源码一律走读取层归一（ADR-0019 / #1177）：裸 fs 读在 Windows 检出上是 CRLF，
  // 多行锚点在 CRLF 行上会匹配 0 次而断言照跑 —— 守护静默失效。
  const gate = readText(path.join(ROOT, 'scripts', 'kotlin-all-check.ps1'));
  for (const line of UTF8_DISCIPLINE) {
    expect(gate).toContain(line);
  }
  // 只钉「编码那两行」，不夹带 $ErrorActionPreference / Set-StrictMode：后者会改断言语义。
  expect(UTF8_DISCIPLINE).toHaveLength(2);
  const body = withUtf8Discipline(['"probe"']);
  expect(body.startsWith(UTF8_DISCIPLINE[0])).toBe(true);
  expect(body.startsWith(UTF8_DISCIPLINE.join('; '))).toBe(true);
  expect(body.slice(-'"probe"'.length)).toBe('"probe"');
});

test('解码选项收口：encoding/timeout/maxBuffer/windowsHide 四件齐全且是工具的唯一来源', () => {
  expect(EXEC_OPTIONS.encoding).toBe('utf8');
  expect(typeof EXEC_OPTIONS.timeout).toBe('number');
  expect(EXEC_OPTIONS.timeout).toBeGreaterThan(0);
  expect(EXEC_OPTIONS.maxBuffer).toBeGreaterThanOrEqual(8 * 1024 * 1024);
  expect(EXEC_OPTIONS.windowsHide).toBe(true);
});

afterAll(() => {
  // 本守护不落盘任何临时文件；这里只兜「将来谁加了写盘却忘了清」的临时目录。
  let names = [];
  try {
    names = fs.readdirSync(os.tmpdir());
  } catch {
    return;
  }
  for (const n of names.filter((x) => x.startsWith('fl2252-'))) {
    try {
      fs.rmSync(path.join(os.tmpdir(), n), { recursive: true, force: true });
    } catch {
      /* 尽力清 */
    }
  }
});
