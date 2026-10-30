/**
 * #2139 证据采集层（`scripts/lib/process-capture.ps1` 的 `Get-Utf8CompletePrefixLength` /
 * `Read-Utf8Chunk`）· **行为级**守护。
 *
 * ## 为什么是行为守护（不是接线守护）
 *
 * 缺陷只在「块边界落在多字节序列中间」时现形，读源码断言拦不住它：`dev-finish.ps1` 旧写法
 * `$utf8.GetString($buf, 0, $read)` 字面上完全正确（显式 UTF-8、显式长度），但 `Encoding.GetString`
 * 是**无状态一次性**解码 ⇒ 切断处当场烤进 U+FFFD，而证据日志一旦烤进就**不可逆**。
 * 现测（2026-10-30，本机）：③ 门行为腿的 `test-behavior.log` 与 `2045-behavior.log` 各有 56 个
 * U+FFFD、位置成簇（2376/2378/2379/2380…）⇒ 与票面原判「原始 GBK 字节直接落盘」不是同一个根因。
 * 判别力（guards.md 判据①）由腿 2 承担：同一份字节流按旧写法跑**必须真的**产出 U+FFFD。
 *
 * ## 四条腿（成对取证，判据③）
 *
 * 1. **真件·跨块续读**：先写「中 + 半个文」→ 读到「中」且只推进 3 字节 → 补齐 → 再读到「文」
 *    ⇒ 合起来逐字相等、零 U+FFFD。
 * 2. **旧写法变异**：同一份字节按块无状态解码 ⇒ U+FFFD 必为 2（这条是「判据有牙」的正控）。
 * 3. **前缀纯函数**：1/2/3 字节尾巴、纯 ASCII、4 字节序列（U+1F600）各返回正确的可解码前缀长度；
 *    `-Flush` 一律返回全长。
 * 4. **`-Flush` 不丢字**：真截断的尾巴（只落 2 个字节）在非 Flush 下整段留尾（Text 空、不推进），
 *    在 Flush 下必须把字节**交出来**并留下可见的替换字符 ⇒ 坏要看得见，不能变成「这段没发生过」。
 *
 * ## 为什么不用 `System.Text.Decoder`
 *
 * 本来 `Decoder` 就是这个 bug 的正解，但本会话实测 **PowerShell 绑不到**：`GetDecoder()` 返回运行时
 * 类型 `DecoderNLS`（internal），PS 的方法解析在该类型上取不到任何 `GetString` 重载（一/三/四参
 * 全部报「不包含名为 'GetString' 的方法」）。所以这里是等价的纯字节做法 —— 本文件的腿 3 就是把
 * 那段字节算术单独量出来，免得它退化成没人验的黑盒。
 *
 * ## 夹具为什么全用码位
 *
 * U+FFFD 字面量在本会话实测会被**写入链整字吞掉**（落盘成空串），所以被测字节与被测字符全部用
 * `[byte[]]` / `[char]0xFFFD` 表达，不依赖任何非 ASCII 字面量活着到达磁盘。
 *
 * 运行前提：需要 `pwsh`（PowerShell 7）。不可用时 `drive()` 抛错 ⇒ 本文件红，**不 skip**
 * （静默跳过等于假绿，与 `utils/kotlinAllProcessCaptureBehavior.test.js` 同一条纪律）。
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const LIB = path.join(ROOT, 'scripts', 'lib', 'process-capture.ps1');
const DRIVE_TIMEOUT_MS = 120000;

jest.setTimeout(300000);

function psArgs(extra) {
  const args = ['-NoProfile', '-NonInteractive'];
  if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass', '-OutputFormat', 'Text');
  return args.concat(extra);
}

/** 单引号字面量转义（PowerShell）。 */
function q(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

/** dot-source 真库并执行语句，回读 stdout。 */
function drive(statements) {
  const prelude = [
    '$ErrorActionPreference = "Stop"',
    '[Console]::OutputEncoding = [System.Text.Encoding]::UTF8',
    '$OutputEncoding = [System.Text.Encoding]::UTF8',
    'Set-StrictMode -Version Latest',
    `. ${q(LIB)}`,
  ];
  const script = prelude.concat(statements).join('; ');
  const encoded = Buffer.from(script, 'utf16le').toString('base64');
  try {
    return String(execFileSync('pwsh', psArgs(['-EncodedCommand', encoded]), {
      encoding: 'utf8',
      timeout: DRIVE_TIMEOUT_MS,
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024,
    }));
  } catch (e) {
    throw new Error(
      '驱动采集层失败（pwsh 不可用 / 库抛错 / 命中驱动上限）：\n'
      + `exit=${e.status}\nstdout=${e.stdout}\nstderr=${e.stderr || e.message}`,
    );
  }
}

function pick(stdout, key) {
  const m = String(stdout).match(new RegExp(`^${key}=(.*)$`, 'm'));
  if (m === null) throw new Error(`输出里没有 ${key}：\n${stdout}`);
  return m[1].trim();
}

/** PS 侧数 U+FFFD —— 不把非 ASCII 文本再穿一次管道，只回传计数与码位表。 */
const CODES_OF_OUT = '$codes = (($out.ToCharArray() | ForEach-Object { [int]$_ }) -join \',\')';

// 「中文」的 UTF-8 字节：E4 B8 AD / E6 96 87 ⇒ 在 index 3 处切开正好落在第二个字中间
const BYTES = '0xE4,0xB8,0xAD,0xE6,0x96,0x87';
const EXPECT_ZHONG_WEN = String.fromCharCode(0x4e2d, 0x6587);
const FFFD_CODE = 0xFFFD;

const TMP_DECL = "$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('fl2139-' + [guid]::NewGuid().ToString('N') + '.log')";

afterAll(() => {
  // 各腿自带 Remove-Item；这里只兜「腿中途抛错」留下的临时文件
  let names = [];
  try { names = fs.readdirSync(os.tmpdir()); } catch { return; }
  for (const n of names.filter((x) => x.startsWith('fl2139-'))) {
    try { fs.rmSync(path.join(os.tmpdir(), n), { force: true }); } catch { /* 尽力清 */ }
  }
});

test('腿1 真件：半截尾巴留在文件里，补齐后逐字相等且零 U+FFFD', () => {
  const out = drive([
    TMP_DECL,
    `$b = [byte[]](${BYTES})`,
    '$pos = [long]0',
    // 第一段只写到「中 + E6」：第二个字缺两个尾字节
    '[System.IO.File]::WriteAllBytes($tmp, $b[0..3])',
    '$p1 = Read-Utf8Chunk -Path $tmp -Offset $pos -Flush:$false',
    '$pos += $p1.BytesRead',
    '"T1=$($p1.Text)"',
    '"N1=$($p1.BytesRead)"',
    // 补齐后从新偏移再读：E6 96 87 凑齐 ⇒ 读到「文」
    '[System.IO.File]::WriteAllBytes($tmp, $b)',
    '$p2 = Read-Utf8Chunk -Path $tmp -Offset $pos -Flush:$false',
    '$pos += $p2.BytesRead',
    '"T2=$($p2.Text)"',
    '"N2=$($p2.BytesRead)"',
    '$out = $p1.Text + $p2.Text',
    CODES_OF_OUT,
    '"CODES=$codes"',
    '"POS=$pos"',
    'Remove-Item $tmp -Force',
  ]);
  expect(pick(out, 'T1')).toBe(String.fromCharCode(0x4e2d));
  expect(pick(out, 'N1')).toBe('3');
  expect(pick(out, 'T2')).toBe(String.fromCharCode(0x6587));
  expect(pick(out, 'N2')).toBe('3');
  expect(pick(out, 'POS')).toBe('6');
  expect(pick(out, 'CODES').split(',').map(Number)).toEqual(
    EXPECT_ZHONG_WEN.split('').map((c) => c.charCodeAt(0)),
  );
  expect(pick(out, 'CODES').split(',').map(Number)).not.toContain(FFFD_CODE);
});

test('腿2 变异（旧写法）：同一份字节按块无状态解码，必须真的烤进两个 U+FFFD', () => {
  const out = drive([
    `$b = [byte[]](${BYTES})`,
    '$e = [System.Text.Encoding]::UTF8',
    '$t1 = $e.GetString($b, 0, 2)',
    '$t2 = $e.GetString($b, 2, 4)',
    '$out = $t1 + $t2',
    CODES_OF_OUT,
    '"CODES=$codes"',
  ]);
  const codes = pick(out, 'CODES').split(',').map(Number);
  expect(codes.filter((c) => c === FFFD_CODE).length).toBe(2);
  // 旧写法不是「读得少」，是「读错了还看不出来」：字节数照样全消耗完
  expect(codes.length).toBeGreaterThan(0);
});

test('腿3 前缀纯函数：1/2/3 字节尾巴、ASCII、四字节序列各退回到正确边界；Flush 一律全长', () => {
  const cases = [
    { name: 'tail1', bytes: '0x41,0xF0', want: 1, len: 2 },      // A + 半个 4 字节序列
    { name: 'tail2', bytes: '0xE4,0xB8', want: 0, len: 2 },      // 只有半个 3 字节序列
    { name: 'tail3', bytes: '0xE4,0xB8,0xAD,0xE6,0x96', want: 3, len: 5 }, // 中 + 半个文
    { name: 'ascii', bytes: '0x41,0x42,0x43', want: 3, len: 3 }, // 纯 ASCII 不留尾
    { name: 'full4', bytes: '0xF0,0x9F,0x98,0x80', want: 4, len: 4 }, // U+1F600 凑齐
    { name: 'empty', bytes: '', want: 0, len: 0 },
  ];
  const statements = [];
  for (const c of cases) {
    statements.push(`$x = [byte[]]@(${c.bytes})`);
    statements.push(`"${c.name}=$(Get-Utf8CompletePrefixLength -Bytes $x)"`);
    statements.push(`"${c.name}F=$(Get-Utf8CompletePrefixLength -Bytes $x -Flush)"`);
  }
  const out = drive(statements);
  for (const c of cases) {
    expect(pick(out, c.name)).toBe(String(c.want), `${c.name} 的可解码前缀`);
    expect(pick(out, `${c.name}F`)).toBe(String(c.len), `${c.name} Flush 必须全交出去`);
  }
});

test('腿4 Flush 不丢字：真截断的尾巴在非 Flush 下留尾、在 Flush 下必须可见地坏', () => {
  const out = drive([
    TMP_DECL,
    '[System.IO.File]::WriteAllBytes($tmp, [byte[]](0xE4,0xB8))',
    '$p = Read-Utf8Chunk -Path $tmp -Offset 0 -Flush:$false',
    '"KEEP_T=$($p.Text)"',
    '"KEEP_N=$($p.BytesRead)"',
    '$f = Read-Utf8Chunk -Path $tmp -Offset 0 -Flush',
    '$out = $f.Text',
    CODES_OF_OUT,
    '"FLUSH_CODES=$codes"',
    '"FLUSH_N=$($f.BytesRead)"',
    'Remove-Item $tmp -Force',
  ]);
  expect(pick(out, 'KEEP_N')).toBe('0', '没凑齐就不推进偏移，字节留在文件里');
  expect(pick(out, 'KEEP_T')).toBe('');
  expect(pick(out, 'FLUSH_N')).toBe('2', 'Flush 必须把这两个字节交出来，不能当它们不存在');
  expect(pick(out, 'FLUSH_CODES').split(',').map(Number)).toEqual([FFFD_CODE]);
});
