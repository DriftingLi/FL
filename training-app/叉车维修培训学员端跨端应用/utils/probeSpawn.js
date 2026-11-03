/**
 * #2252 共享探针工具：起 PowerShell 探针时，**脚本体**与**解码选项**只有这一个来源。
 *
 * ## 它解决什么
 *
 * 本机 `[Console]::OutputEncoding` 默认是 gb2312。测试里 `execFileSync('pwsh', …)` 起的子进程
 * 若没有前置 UTF-8 自律行，中文写回父侧时当场被按 GBK 解成 U+FFFD 烤进证据日志 —— 不可逆，
 * 正是 spec C（#2234）立项的病根。此前这个自律行靠 **73 处手抄**（#2248 实测：30 `-EncodedCommand`
 * + 17 `-Command` + 26 `-File`，落 53 个文件），抄漏一处就坏一处。
 *
 * ## 职责恰好两件（#2234 原文）
 *
 *   1. 内联探针脚本体**统一前置** UTF-8 自律行（对齐门脚本顶部那条既有自律）；
 *   2. `execFileSync` 选项统一收口（encoding / timeout / maxBuffer / windowsHide）。
 *
 * 它**不改任何测试的断言语义**：自律行只有那两行编码设置，不夹带 `$ErrorActionPreference`
 * 或 `Set-StrictMode` —— 后者会把「未初始化变量读到 $null」变成抛错，迁移时等于悄悄改了别人的断言。
 *
 * ## 它刻意不做的事
 *
 * - **不持有 `execFileSync`**。③ 门判据按**文件自身**有无代码级执行调用判行为腿/接线腿
 *   （先例 `utils/mpWeixinGateHarness.js` 头注）。调用搬进这里 ⇒ 用它的测试退化成接线守护，
 *   `utils/guardClassification.test.js` 当场转红。
 * - **不收 `-File` 形态**（#2234 原文：「`-File` 形态不收进工具（脚本体在 scripts/ 侧）」）：
 *   那 26 处的处置是逐脚本顶部补自律行，不是走这个工具。
 *
 * ## 与仓内权威行的关系
 *
 * `UTF8_DISCIPLINE` 与 `scripts/kotlin-all-check.ps1:85-86` 那两行**逐字同款**
 * （守卫测试里有断言钉住），出处与病根分析见 `scripts/dev-finish.ps1:132-137`。
 * 改这里之前先读那两处。
 */

/**
 * 子进程 UTF-8 自律行 —— 与 `scripts/kotlin-all-check.ps1:85-86` 逐字同款。
 *
 * 第一行的 try/catch 不是防御性编程：`[Console]::OutputEncoding` 在非控制台环境会抛，
 * 而这条自律行必须能安全地贴进任何探针（包括无控制台的 CI 子进程）。
 */
const UTF8_DISCIPLINE = [
  'try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }',
  '$OutputEncoding = [System.Text.Encoding]::UTF8',
];

/**
 * 脚本体前置自律行。两种走法共用它：
 *   - `-EncodedCommand`：把返回值再交给 `encodeCommand`；
 *   - `-Command`：返回值本身就是参数（自律行与正文用 `; ` 串起来）。
 *
 * @param {string|string[]} body 探针正文（可传语句数组）
 * @returns {string} 自律行 + 正文
 */
function withUtf8Discipline(body) {
  const tail = Array.isArray(body)
    ? body.filter((x) => x != null && String(x).trim() !== '')
    : [String(body == null ? '' : body).trim()].filter(Boolean);
  return UTF8_DISCIPLINE.concat(tail).join('; ');
}

/**
 * 变成 `-EncodedCommand` 可用的参数值：UTF-16LE + base64。
 * 不走命令行转义 —— 探针正文里常有引号、中文与多行，编码形态是唯一稳的传递方式。
 */
function encodeCommand(body) {
  return Buffer.from(withUtf8Discipline(body), 'utf16le').toString('base64');
}

/**
 * `execFileSync` 解码选项的唯一来源。`maxBuffer` 与 #2139 同值：
 * 证据日志单条观测到过 MB 级，默认 200KB 会把大输出截成 `ENOBUFS`。
 */
const EXEC_OPTIONS = {
  encoding: 'utf8',
  timeout: 120000,
  maxBuffer: 8 * 1024 * 1024,
  windowsHide: true,
};

module.exports = {
  UTF8_DISCIPLINE,
  withUtf8Discipline,
  encodeCommand,
  EXEC_OPTIONS,
};
