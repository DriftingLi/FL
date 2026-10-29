/**
 * ② 门的两处「静默失效」守护（2026-10-29 立，来路 #2099 盘点 / #2107 量尺实测）。
 *
 * 为什么不写成「源码里有没有那行字」：#1030 的教训 —— 文本判据会把「改了措辞」误判成「改了行为」，
 * 也会把「注释留着、行为删了」放过去。这两个洞的后果都不是红，而是**门静默失效**，所以这里真执行：
 *
 *   F3（噪音白名单）：把探针的 `platformNoiseOf` 切出来真跑 —— 平台文案放行、真错误照旧计数、
 *      非 error 不进白名单、白名单外的 appLaunch 文案也不放行。
 *   F4（有界收流）：把 check.ps1 的 `Invoke-Process` 切出来真跑 —— 子进程继承 stdout 句柄时，
 *      命令**必须仍然返回**（旧实现对已退出的主进程 `.Result` 无界等待 ⇒ tee 管道挂死，
 *      现场读起来像「门还在跑」）。
 *
 * 两条都刻意用「真实进程」而不是 mock：F4 的病根就是真句柄继承，mock 掉就什么也没测到。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.join(__dirname, '..');
const PROBE_REL = 'scripts/mp-weixin-probe.mjs';
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';

// 读取层归一（ADR-0019 / #1177）：必须走共享读者，不自己写 readFileSync。
// 裸读在 Windows 检出（core.autocrlf=true）上会让多行锚点里的 `.` 撞上 `\r` 而匹配 0 次
// ⇒ 切片为空 ⇒ 守护静默失效；而 CI 跑 ubuntu 默认关 autocrlf，这一类**结构性不可见**
// （本行由 scripts/check-contract-read.mjs 守，本次 CI 就是这么抓到的）。
const { readText } = require('./utsHarness');

/** 取一个 PowerShell 函数的函数体（花括号配对）。注释已剥离不影响配对，但**不要用注释当锚点**（#1209）。 */
function psFunctionBody(code, decl) {
  const at = code.indexOf(decl);
  if (at === -1) return '';
  const open = code.indexOf('{', at);
  if (open === -1) return '';
  let depth = 0;
  for (let i = open; i < code.length; i++) {
    if (code[i] === '{') depth++;
    else if (code[i] === '}') { depth--; if (depth === 0) return code.slice(open, i + 1); }
  }
  return '';
}

describe('F3：平台层噪音白名单（② 在 cold-start-direct 下不再恒假红）', () => {
  // 把常量声明与分类函数从探针源码里切出来重建，**真跑** —— 不 import 整个脚本（那会启动探针主流程）
  const src = readText(path.join(ROOT, PROBE_REL));
  const constDecl = (src.match(/const PLATFORM_NOISE_PATTERNS = \[[\s\S]*?\n\];/) || [''])[0];
  const fnDecl = (src.match(/function platformNoiseOf\(entry\) \{[\s\S]*?\n\}/) || [''])[0];

  let platformNoiseOf;
  beforeAll(() => {
    expect(constDecl).not.toBe('');
    expect(fnDecl).not.toBe('');
    // eslint-disable-next-line no-new-func
    platformNoiseOf = new Function(`${constDecl}\n${fnDecl}\nreturn platformNoiseOf;`)();
  });

  const entry = (type, text) => ({ type, args: [text] });

  it('H1 放行：开发者工具「栈非空又 appLaunch」这条平台文案不计入 errors', () => {
    expect(platformNoiseOf(entry('error', 'appLaunch with non-empty page stack'))).toBeTruthy();
    // 实测形态里前后可能夹带上下文，白名单按「appLaunch + non-empty page stack」两段同时出现判
    expect(platformNoiseOf(entry('error', '[system] appLaunch failed: page stack is non-empty (3)'))).toBeTruthy();
  });

  it('H2 不放行：真业务错误照旧计数（白名单必须窄，宽了就是静默吞真错）', () => {
    expect(platformNoiseOf(entry('error', "Cannot read properties of undefined (reading 'split')"))).toBeNull();
    expect(platformNoiseOf(entry('error', 'request:fail timeout'))).toBeNull();
    // 只出现 appLaunch、不带 non-empty page stack ⇒ 不属这一族，必须照旧报错
    expect(platformNoiseOf(entry('error', 'appLaunch failed: something else entirely'))).toBeNull();
  });

  it('H3 不越界：非 error 级别不进白名单（warn/log 本来就不计数，放宽只会掩盖分级错误）', () => {
    expect(platformNoiseOf(entry('warn', 'appLaunch with non-empty page stack'))).toBeNull();
    expect(platformNoiseOf(entry('log', 'appLaunch with non-empty page stack'))).toBeNull();
  });

  it('H4 过滤可见：探针结果里必须同时有 noiseTotal 与 noiseTail（不是静默吞掉）', () => {
    // 行为面已由 H1–H3 覆盖；这里只钉「过滤的读数能不能被外部看到」——删掉字段等于重新引入
    // 「覆盖为零的绿」：门绿了，但没人知道它放过了什么。
    expect(src).toMatch(/noiseTotal:\s*0/);
    expect(src).toMatch(/noiseTail:\s*\[\]/);
    // 命中时既要记数、也要把原文留进尾部日志
    expect(src).toMatch(/result\.noiseTotal \+= 1/);
    expect(src).toMatch(/result\.noiseTail\.push/);
  });
});

describe('F4：Invoke-Process 的收流必须有界（主进程退出后不许被句柄挂死）', () => {
  const src = readText(path.join(ROOT, SCRIPT_REL));
  const body = psFunctionBody(src, 'function Invoke-Process');
  let scriptPath = '';

  beforeAll(() => {
    expect(body).not.toBe('');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mp-weixin-drain-'));
    scriptPath = path.join(dir, 'invoke-process.ps1');
    // 切出来的函数单独成脚本：只喂给它一个可跑的最小上下文（$script:workDir + node 路径）
    const harness = [
      "$ErrorActionPreference = 'Stop'",
      '$script:workDir = (Get-Location).Path',
      // psFunctionBody 只回花括号体，函数头要自己补回来
      'function Invoke-Process ' + body,
      // case1：正常命令 —— 必须拿到输出、DrainAbandoned 为 false
      "$t0 = Get-Date",
      "$r1 = Invoke-Process -FilePath $args[0] -Arguments @('-e','process.stdout.write(\"hello-out\")') -TimeoutSeconds 10 -Tag f4-normal",
      "$ms1 = [int]((Get-Date) - $t0).TotalMilliseconds",
      // case2：主进程退出但子进程继承 stdout 句柄（IDE 常驻的同族形态）——
      // 旧实现会在 .Result 上无界等待 ⇒ 本用例挂到 holder 自然退出（30s）
      '$holder = @\'\n'
        + "const { spawn } = require('child_process');\n"
        + "spawn(process.execPath, ['-e', 'setTimeout(()=>{}, 15000)'], { stdio: 'inherit', detached: true }).unref();\n"
        + "process.stdout.write('parent-done\\n');\n"
        + 'process.exit(0);\n'
        + "'@",
      '[System.IO.File]::WriteAllText((Join-Path $PSScriptRoot "holder.js"), $holder)',
      '$t1 = Get-Date',
      '$r2 = Invoke-Process -FilePath $args[0] -Arguments @((Join-Path $PSScriptRoot "holder.js")) -TimeoutSeconds 10 -Tag f4-handle-holder',
      '$ms2 = [int]((Get-Date) - $t1).TotalMilliseconds',
      '$out = [ordered]@{',
      "  normalOutput = $r1.Output",
      '  normalDrainAbandoned = [bool]$r1.DrainAbandoned',
      '  normalMs = $ms1',
      '  holderDrainAbandoned = [bool]$r2.DrainAbandoned',
      '  holderMs = $ms2',
      '  holderExit = $r2.ExitCode',
      '}',
      "Write-Output ('F4RESULT ' + ($out | ConvertTo-Json -Compress))"
    ].join('\n');
    fs.writeFileSync(scriptPath, harness, 'utf8');
  });

  function runHarness() {
    const args = ['-NoProfile', '-NonInteractive'];
    if (process.platform === 'win32') args.push('-ExecutionPolicy', 'Bypass');
    args.push('-File', scriptPath, process.execPath);
    let raw = '';
    try {
      raw = execFileSync('pwsh', args, { encoding: 'utf8', timeout: 90000, windowsHide: true, maxBuffer: 8 * 1024 * 1024 });
    } catch (e) {
      throw new Error(`F4 harness 执行失败：exit=${e.status} signal=${e.signal}\nstdout=${e.stdout}\nstderr=${e.stderr}`);
    }
    const line = raw.split(/\r?\n/).find((l) => l.startsWith('F4RESULT '));
    if (!line) throw new Error(`F4 harness 没有输出结果行：\n${raw}`);
    return JSON.parse(line.slice('F4RESULT '.length));
  }

  // 两次断言看的是**同一次**执行：harness 里两个 case 串行跑，且 holder 会拖住 pwsh 的退出；
  // 每个 it 各跑一遍等于白付一倍机时（实测 31 s × 2）。
  let result;
  beforeAll(() => { result = runHarness(); }, 120000);

  it('H5 正常命令：输出完整、不走兜底（有界收流不能把正常读取也截断）', () => {
    expect(result.normalOutput).toContain('hello-out');
    expect(result.normalDrainAbandoned).toBe(false);
  }, 120000);

  it('H6 句柄被继承：命令仍然返回，且明确标记 DrainAbandoned（旧实现会挂到 holder 自然退出）', () => {
    // 主进程已退出 ⇒ 退出码是真值，不被"仍在跑"污染
    expect(result.holderExit).toBe(0);
    // 关键：必须有界返回。drain 窗口 5 s；一旦回到 holder 的存活量级（15 s），
    // 就说明 `.Result` 又被无界等待了（旧实现的形态）
    expect(result.holderMs).toBeLessThan(10000);
    expect(result.holderDrainAbandoned).toBe(true);
  }, 120000);
});
