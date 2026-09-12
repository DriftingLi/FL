/**
 * ④ 本地编译门（scripts/kotlin-all-check.ps1 + scripts/compile-check.ps1）契约守护
 *
 * 背景（#859 spike / #870 T2）：④a 是**增量**编译（只编译变更 .kt + 复用陈旧 class），
 * ④b 是云打包（消耗云资源、需账号）。④c 用 appResource 产物 + HBuilderX 自带 kotlinc 做
 * 「整模块」编译，作为本地加固。2026-09-11 修订后 ④a/④c 合并为「④ 本地编译门」
 * （④c 为默认载体，dev 专属面追加 ④a）。它的价值全押在三条不变量上，本测试就是守这几条：
 *   C1 输入必须是 appResource 产物（unpackage/resources/app-android/**）
 *   C2 绝不把 dev 编译缓存（unpackage/cache/.app-android）当整模块输入
 *      —— 那是增量产物，被引用类型可能只存在于陈旧 class/*.class 里（#859 实测必然失败）
 *   C3/C4 判成败解析输出，绝不看退出码（HBuilderX CLI 退出码恒为 0）
 *   C5 工具链必须是 HBuilderX 自带的 kotlinc + Corretto + UTS 编译器插件 + lib2 classpath
 *   C6 必须写明「④c ≠ compileReleaseKotlin」的非等价声明
 *   C7 受限会话兜底：-SkipPublish + 「与主程序的连接已中断」判据
 *   C8/C9 npm 脚本已注册、ADR-0008 已记录
 *   C10 编译门结果免手抄：两个脚本都必须支持 -PostToPr 贴「sha 绑定」的 PR 评论
 *      （标记 `gate-evidence:④` + `commit: <sha>`），pr-evidence 才认得出这条证据
 *
 * 设计沿用本仓既有守护测试的形态（见 utils/utsAndroidCompile.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/kotlin-all-check.ps1';
const COMPILE_SCRIPT_REL = 'scripts/compile-check.ps1';
const PKG_REL = 'package.json';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

/** 纯函数：四份源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const script = sources.script;
  const pkg = sources.pkg;
  const adr = sources.adr;

  if (!script.includes('unpackage\\resources\\app-android')) {
    violations.push('C1 脚本未引用 appResource 产物路径（unpackage\\resources\\app-android）');
  }

  // dev 缓存路径只允许出现在注释里：行注释 # 或文档块 <# ... #>（块内先掩码为空格，行号不变）
  const maskedScript = script.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
  // 允许的三种出现位置：注释行 / 常量定义行 / 报错提示文案行；其余一律视为「把它当输入」
  const devCacheAllowed = (line) =>
    /^\s*#/.test(line) ||
    /^\s*\$DevCacheRelative\s*=/.test(line) ||
    /Write-Log|Write-Host/.test(line);
  maskedScript.split(/\r?\n/).forEach((line, idx) => {
    if (line.includes('unpackage\\cache\\.app-android') && !devCacheAllowed(line)) {
      violations.push('C2 第 ' + (idx + 1) + ' 行把 dev 缓存路径当输入（禁止；dev 缓存不是自洽编译单元）');
    }
  });

  if (!script.includes('KOTLIN_ALL_RESULT')) {
    violations.push('C3 缺少 KOTLIN_ALL_RESULT 结果行（无法机检成败）');
  }
  if (!/ErrorLinePattern\s*=\s*'/.test(script) || !/\[regex\]::Matches\(\$result\.Output, \$ErrorLinePattern\)/.test(script)) {
    violations.push('C3 未用 ErrorLinePattern 解析输出判定成败');
  }
  if (/\$LASTEXITCODE/.test(script)) {
    violations.push('C4 出现 $LASTEXITCODE：CLI 退出码恒为 0，判成败不得依赖退出码');
  }

  ['uts-kotlin-compiler-plugin.jar', 'kotlinc', 'lib2', 'amazon-corretto'].forEach((token) => {
    if (!script.includes(token)) {
      violations.push('C5 缺少自带工具链要素：' + token);
    }
  });

  if (!script.includes('compileReleaseKotlin')) {
    violations.push('C6 缺少与 compileReleaseKotlin 的对照说明');
  }
  if (!script.includes('非等价')) {
    violations.push('C6 缺少「非等价声明」（④c ≠ compileReleaseKotlin）');
  }

  if (!script.includes('SkipPublish')) {
    violations.push('C7 缺少 -SkipPublish（受限会话只跑 kotlinc 部分的兜底）');
  }
  if (!script.includes('与主程序的连接已中断')) {
    violations.push('C7 缺少 CLI↔主程序 IPC 阻断的判据');
  }

  if (!/"build:kotlin-all"\s*:\s*"[^"]*scripts\/kotlin-all-check\.ps1"/.test(pkg)) {
    violations.push('C8 package.json 未注册 build:kotlin-all → scripts/kotlin-all-check.ps1');
  }

  if (!adr.includes('④c')) {
    violations.push('C9 ADR-0008 未记录 ④c');
  }

  // C10：P1 编译门结果免手抄 —— 两个脚本都必须能把结果贴成「sha 绑定」的 PR 评论。
  // 判据三段：标记（校验器靠它认评论）、commit 行（sha 绑定）、-PostToPr 开关（仅在门通过时贴）。
  [
    { name: 'kotlin-all-check.ps1', text: script },
    { name: 'compile-check.ps1', text: sources.compileScript }
  ].forEach(({ name, text }) => {
    if (typeof text !== 'string' || !text) {
      violations.push('C10 缺 ' + name + ' 源码');
      return;
    }
    if (!text.includes('gate-evidence:④')) {
      violations.push('C10 ' + name + ' 未输出 gate-evidence:④ 标记（pr-evidence 认不出这条评论证据）');
    }
    if (!text.includes('commit:')) {
      violations.push('C10 ' + name + ' 的评论缺 commit 行（无法与 PR head sha 绑定）');
    }
    if (!/\$PostToPr/.test(text)) {
      violations.push('C10 ' + name + ' 缺 -PostToPr 开关（门结果免手抄的入口）');
    }
  });

  return violations;
}

describe('④ 本地编译门契约（#870 T2 / 2026-09-11 修订）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    compileScript: readSource(COMPILE_SCRIPT_REL),
    pkg: readSource(PKG_REL),
    adr: readSource(ADR_REL)
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['C1', { ...real, script: real.script.replace('unpackage\\resources\\app-android', 'unpackage\\resources\\x') }],
      ['C2', { ...real, script: real.script + '\n$dev = "unpackage\\cache\\.app-android\\src"\n' }],
      ['C2', { ...real, script: real.script + '\n$kt = Get-ChildItem (Join-Path $Project "unpackage\\cache\\.app-android\\src") -Recurse -Filter *.kt\n' }],
      ['C3', { ...real, script: real.script.replace(/KOTLIN_ALL_RESULT/g, 'RESULT_X') }],
      ['C4', { ...real, script: real.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' }],
      ['C5', { ...real, script: real.script.replace(/uts-kotlin-compiler-plugin\.jar/g, 'plugin.jar') }],
      ['C6', { ...real, script: real.script.replace(/非等价/g, '等价性待定') }],
      ['C7', { ...real, script: real.script.replace(/SkipPublish/g, 'SkipAll') }],
      ['C8', { ...real, pkg: real.pkg.replace('build:kotlin-all', 'build:kotlin-everything') }],
      ['C9', { ...real, adr: real.adr.replace(/④c/g, '门 C') }],
      ['C10', { ...real, script: real.script.replace(/gate-evidence:④/g, 'gate-evidence:x') }],
      ['C10', { ...real, compileScript: real.compileScript.replace(/commit:/g, 'sha=') }],
      ['C10', { ...real, compileScript: real.compileScript.replace(/PostToPr/g, 'PostIt') }]
    ];
    cases.forEach(([rule, sources]) => {
      const found = scanContract(sources);
      expect(found.some((v) => v.startsWith(rule))).toBe(true);
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('输入口径写死为 appResource 产物，并显式禁止 dev 缓存', () => {
    expect(real.script).toContain("$AppResourceRelative = 'unpackage\\resources\\app-android'");
    expect(real.script).toContain("$DevCacheRelative = 'unpackage\\cache\\.app-android'");
  });

  it('整模块编译走 java + kotlin-preloader + K2JVMCompiler（不经 cmd.exe，规避 8191 字符上限）', () => {
    expect(real.script).toContain('kotlin-preloader.jar');
    expect(real.script).toContain('org.jetbrains.kotlin.cli.jvm.K2JVMCompiler');
    expect(real.script).toContain('plugin:io.dcloud.uts.kotlin:tag=UTS');
  });

  it('C10：④ 门结果免手抄——两脚本都能贴 sha 绑定的 PR 评论（-PostToPr）', () => {
    [real.script, real.compileScript].forEach((text) => {
      expect(text).toContain('gate-evidence:④');
      expect(text).toContain('- commit: $sha');
      expect(text).toContain('[int]$PostToPr = 0');
      // 只在门通过分支调用：失败分支（exit 1 / exit 2）之前不得出现调用
      const callAt = text.indexOf('Publish-GateComment -PrNumber');
      expect(callAt).toBeGreaterThan(-1);
      expect(text.indexOf('exit 1')).toBeLessThan(callAt);
    });
  });
});


