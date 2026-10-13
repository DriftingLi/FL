/**
 * ② 门「环境性失败三分」行为守护（#1655 / ADR-0036 处置二）。
 *
 * 三分：通过（metric 行本身——无 errors= token，errorsTotal=0 承载通过语义，exit 0）/ 门未过（errors=<N> failures=…，exit 1）/
 * 环境受阻（errors=env-blocked reason=<slug>，exit 2）。分类判据真源 =
 * `scripts/lib/mp-weixin-env.ps1`（门脚本 dot-source 同一份，:1535 一带的分类下沉到它）。
 *
 * 为什么判据是**真执行库**而不是断言门脚本源码文本：文本断言在「判据被摘掉、只留一句字面量」时
 * 照样绿（本仓 C20 的教训）。这里 dot-source 真库、真调 `Get-MpWeixinEnvClassification` /
 * 两个结果行构造器，用**成对取证**钉住语义：
 *   - 必不红：三类夹具给出正确的 kind / reason 派生 / 结果行形状；
 *   - 必红：**裸 Contains 写法**会把「引用『环境不可用：』这句话的普通断言失败」误判成环境受阻
 *     —— 这条证明本判据的 StartsWith 锚真的在起作用（能区分判得对与判得错），不是恒真断言。
 *
 * 对抗式自查（判据会不会放过坏实现？）：断言的是**产物**（分类结论 + 结果行形状），
 * 不是源码片段；把判据改坏（改裸 Contains / 摘掉前缀锚）时，必红夹具会当场翻红。
 */

const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
// ⚠️ 载体路径写成**本文件内的字面量**（判据面 + ③ 门分类器按文件自身的「执行调用 + 仓内载体引用」判类）
const LIB_REL = 'scripts/lib/mp-weixin-env.ps1';

/** 起 pwsh：dot-source 判据库 → 跑分类矩阵 + 行构造器 + 裸 Contains 反例，回传机读 JSON。 */
function probeLib() {
  // 用自己的绝对路径 dot-source（`-Command` 下没有 `$PSScriptRoot`，别用它）
  const libPath = path.join(ROOT, LIB_REL.split('/').join(path.sep));
  const script = [
    // 本机控制台代码页是 GBK：探针输出含中文（夹具与结果行），先钉死 UTF-8 再跑
    "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)",
    "$ErrorActionPreference = 'Stop'",
    'Set-StrictMode -Version Latest',
    `. '${libPath}'`,
    // —— 分类矩阵（#1655 实测枚举；每类配探针原文形状的夹具）——
    // 空集合 ⇒ pass；纯断言失败 ⇒ gate-failed；带「环境不可用：」前缀 ⇒ env-blocked + reason 派生。
    // 反例：**引用**这句话（前缀不在开头）的失败条目必须仍归 gate-failed（StartsWith 锚）。
    '$class = [ordered]@{',
    '  empty = (Get-MpWeixinEnvClassification -Failures @())',
    "  gateOnly = (Get-MpWeixinEnvClassification -Failures @('断言未过：entry（期望 login）'))",
    "  quoteMention = (Get-MpWeixinEnvClassification -Failures @('断言未过：先看「环境不可用：」说明'))",
    "  notReady = (Get-MpWeixinEnvClassification -Failures @('环境不可用：自动化会话未就绪：Tool.getInfo 未返回 SDKVersion（口径 connectError.kind=not-ready；原始：raw）'))",
    "  unreachable = (Get-MpWeixinEnvClassification -Failures @('环境不可用：自动化端口连不上（ws 层）（口径 connectError.kind=unreachable；原始：raw）'))",
    "  autoTimeout = (Get-MpWeixinEnvClassification -Failures @('环境不可用：x（口径 connectError.kind=timeout；原始：raw）'))",
    "  autoUnknown = (Get-MpWeixinEnvClassification -Failures @('环境不可用：x（口径 connectError.kind=weird；原始：raw）'))",
    "  autoMissing = (Get-MpWeixinEnvClassification -Failures @('环境不可用：找不到 miniprogram-automator（npm install 失败）'))",
    "  stackEmpty = (Get-MpWeixinEnvClassification -Failures @('环境不可用：pageStack 为空（自动化会话没有打开页面）。处置 = 先 `cli.bat close` 再 `cli.bat auto`（#883 坑位 1）。'))",
    "  probeEnv = (Get-MpWeixinEnvClassification -Failures @('环境不可用：别的环境故障'))",
    "  mixed = (Get-MpWeixinEnvClassification -Failures @('断言未过：entry', '环境不可用：pageStack 为空（自动化会话没有打开页面）。'))",
    '}',
    // —— 结果行构造器（三分互斥由构造器保证：门未过行永不写 env-blocked）——
    '$envLine = New-MpWeixinEnvResultLine -Prefix MP_WEIXIN_RESULT -Reason automation-not-ready -Failures @("环境不可用：a", "断言未过：b") -MetricLine "appid=wx1 pageStack=1"',
    '$gateLine = New-MpWeixinGateResultLine -Prefix MP_WEIXIN_RESULT -Count 2 -FailuresJoined "a / b" -MetricLine "appid=wx1"',
    '$gateNoFailures = New-MpWeixinGateResultLine -Prefix MP_WEIXIN_RESULT -Count 1 -FailuresJoined "" -MetricLine "appid=wx1"',
    '$envNoFailures = New-MpWeixinEnvResultLine -Prefix MP_WEIXIN_RESULT -Reason probe-env -Failures @() -MetricLine ""',
    // 裸 Contains 反例：这个写法会把 quoteMention 误判成 env —— 证明判据锚真在起作用。
    "$naiveEnv = @('断言未过：先看「环境不可用：」说明') | Where-Object { $_ -match '环境不可用' }",
    // reason 非法形状必须抛（行构造器的 slug 纪律）
    '$reasonThrow = "no-throw"',
    'try { $null = New-MpWeixinEnvResultLine -Prefix MP_WEIXIN_RESULT -Reason "BAD Reason" } catch { $reasonThrow = "throw" }',
    // reason 目录（唯一真源枚举）
    '$catalogue = (Get-MpWeixinEnvReasonCatalogue) -join ","',
    '$out = [ordered]@{ class = $class; envLine = $envLine; gateLine = $gateLine; gateNoFailures = $gateNoFailures; envNoFailures = $envNoFailures; naiveEnvCount = @($naiveEnv).Count; reasonThrow = $reasonThrow; catalogue = $catalogue }',
    '$out | ConvertTo-Json -Depth 6 -Compress'
  ].join('\n');
  const out = execFileSync('pwsh', ['-NoProfile', '-NonInteractive', '-Command', script], {
    cwd: ROOT,
    encoding: 'utf8',
    timeout: 120000,
    windowsHide: true
  });
  return JSON.parse(out.trim().split(/\r?\n/).filter(Boolean).pop());
}

describe('② 环境性失败三分（分类真执行 + 前缀锚成对取证）', () => {
  let r = null;

  beforeAll(() => { r = probeLib(); }, 180000);

  it('必不红：空集合 ⇒ pass；纯断言失败与「引用」前缀的失败都归 gate-failed', () => {
    expect(r.class.empty.kind).toBe('pass');
    expect(r.class.empty.isEnv).toBe(false);
    expect(r.class.gateOnly.kind).toBe('gate-failed');
    expect(r.class.gateOnly.isEnv).toBe(false);
    // 裸 Contains 会把这条误判成 env —— StartsWith 锚的判据价值所在
    expect(r.class.quoteMention.kind).toBe('gate-failed');
    expect(r.class.quoteMention.isEnv).toBe(false);
  });

  it('必不红：带前缀的环境性失败 ⇒ env-blocked，reason 派生逐类正确', () => {
    expect(r.class.notReady.kind).toBe('env-blocked');
    expect(r.class.notReady.envReasons).toEqual(['automation-not-ready']);
    expect(r.class.unreachable.envReasons).toEqual(['automation-unreachable']);
    expect(r.class.autoTimeout.envReasons).toEqual(['automation-timeout']);
    expect(r.class.autoUnknown.envReasons).toEqual(['automation-unknown']);
    expect(r.class.autoMissing.envReasons).toEqual(['automator-missing']);
    expect(r.class.stackEmpty.envReasons).toEqual(['page-stack-empty']);
    expect(r.class.probeEnv.envReasons).toEqual(['probe-env']);
  });

  it('必不红：混合失败（断言 + 环境）⇒ env-blocked 胜出且保留两类信息', () => {
    expect(r.class.mixed.kind).toBe('env-blocked');
    expect(r.class.mixed.isEnv).toBe(true);
    expect(r.class.mixed.failureCount).toBe(2);
    expect(r.class.mixed.envReasons).toEqual(['page-stack-empty']);
  });

  it('必不红：结果行构造器形状（env 行带 reason=；门未过行永不写 env-blocked）', () => {
    expect(r.envLine).toBe('MP_WEIXIN_RESULT errors=env-blocked reason=automation-not-ready failures=环境不可用：a / 断言未过：b appid=wx1 pageStack=1');
    expect(r.gateLine).toBe('MP_WEIXIN_RESULT errors=2 failures=a / b appid=wx1');
    expect(r.gateLine).not.toMatch(/env-blocked/);
    // 空 failures / 空 metric 的省略分支
    expect(r.gateNoFailures).toBe('MP_WEIXIN_RESULT errors=1 appid=wx1');
    expect(r.envNoFailures).toBe('MP_WEIXIN_RESULT errors=env-blocked reason=probe-env');
  });

  it('必红：裸 Contains 写法会把「引用前缀句」误判成 env（证明 StartsWith 锚真在起作用）', () => {
    expect(r.naiveEnvCount).toBe(1);
  });

  it('必红：reason 非小写连字符 slug 时构造器必须抛（形状纪律）', () => {
    expect(r.reasonThrow).toBe('throw');
  });

  it('reason 目录 = 探针派生 7 枚举（唯一真源）', () => {
    expect(r.catalogue.split(',')).toEqual([
      'automation-not-ready', 'automation-unreachable', 'automation-timeout', 'automation-unknown',
      'automator-missing', 'page-stack-empty', 'probe-env'
    ]);
  });
});
