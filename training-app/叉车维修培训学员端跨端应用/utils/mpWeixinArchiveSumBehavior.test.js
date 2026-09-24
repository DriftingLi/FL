/**
 * ② 门「入库图合计字节」行为守护（2026-09-23）。
 *
 * 病根：`Publish-ScreenshotArchive` 直接取 `($made | Measure-Object -Property Bytes -Sum).Sum` ——
 * `Measure-Object` 对**空管道不产出对象**，于是 `Set-StrictMode -Version Latest` 下抛
 * 「在此对象上找不到属性"Sum"」。实测场景：失败路径（探针没拿到截图 ⇒ `$shots` 为空）必现，被调用点
 * try/catch 兜成 `[warn] 截图入库失败（不影响门结论）`：不影响门结论，但把整段入库打成了异常。
 *
 * 为什么判据是**真执行库**而不是断言门脚本源码文本：文本断言在「判据被摘掉、只留一句字面量」时照样绿
 * （本仓 C20 的教训）。这里 `dot-source` 真库、真调 `Get-MadeTotalBytes`，并用**成对取证**钉住语义：
 *   - 必不红：空集合 / `$null` ⇒ 返回 0（不是抛）；
 *   - 必红：**朴素写法**在同一 StrictMode 下**必须抛** —— 这条证明「本用例真能区分修好与没修」，
 *     而不是一条恒真的断言（若哪天 PowerShell 改了空管道语义，这条会红，提醒我们重新校准判据）。
 *
 * 判据面单点真源：`scripts/lib/mp-weixin-archive.ps1`（门脚本 dot-source 同一份）。
 */
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（判据面 + ③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/mp-weixin-archive.ps1';

/** 起 pwsh：dot-source 判据库 → 调 `Get-MadeTotalBytes` 与**朴素写法**，回传机读 JSON。 */
function probeLib() {
  // 用自己的绝对路径 dot-source（`-Command` 下没有 `$PSScriptRoot`，别用它）
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const script = [
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    '$empty = @()',
    '$one = @([pscustomobject]@{ Path = "a"; Bytes = 10 })',
    '$two = @([pscustomobject]@{ Path = "a"; Bytes = 10 }, [pscustomobject]@{ Path = "b"; Bytes = 32 })',
    'function Try-Naive($Items) { try { $null = ($Items | Measure-Object -Property Bytes -Sum).Sum; return "no-throw" } catch { return "throw" } }',
    '$out = [ordered]@{',
    '  empty = (Get-MadeTotalBytes $empty)',
    '  nullArg = (Get-MadeTotalBytes $null)',
    '  one = (Get-MadeTotalBytes $one)',
    '  two = (Get-MadeTotalBytes $two)',
    '  naiveEmpty = (Try-Naive $empty)',
    '  naiveOne = (Try-Naive $one)',
    '}',
    '$out | ConvertTo-Json -Compress'
  ].join('\n');
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
    cwd: ROOT,
    encoding: 'utf8',
    timeout: 120000,
    windowsHide: true
  });
  return JSON.parse(out.trim().split(/\r?\n/).filter(Boolean).pop());
}

describe('② 入库图合计字节（空安全：Measure-Object 对空管道不产出对象）', () => {
  let r = null;

  beforeAll(() => { r = probeLib(); }, 180000);

  it('必不红：空集合 / $null / 非空集合都给出正确合计（空集合为 0，不抛）', () => {
    expect(r.empty).toBe(0);
    expect(r.nullArg).toBe(0);
    expect(r.one).toBe(10);
    expect(r.two).toBe(42);
  });

  it('必红：朴素的 `.Sum` 写法在空集合上**必须抛**（证明本判据能区分修好与没修）', () => {
    // 空管道 ⇒ Measure-Object 不产出对象 ⇒ `.Sum` 在 StrictMode 下抛「找不到属性 Sum」
    expect(r.naiveEmpty).toBe('throw');
    // 非空管道 ⇒ 有对象 ⇒ 朴素写法不抛（说明上面那条红的是**空集合**这一支，不是 StrictMode 配置问题）
    expect(r.naiveOne).toBe('no-throw');
  });
});
