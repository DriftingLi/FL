/**
 * 测试+编译编排模块契约守护（scripts/lib/test-compile.ps1）
 *
 * 背景（2026-09-14，Q-2 口径）：quick 模式**不再是「跳过编译门」** —— 那样等于不做任何编译验证。
 * 改为调 `hx-run.ps1 -CompileOnly`（官方语义的「仅编译」：**不推送 / 不启动 / 不轮询 / 不需要设备**，
 * 且会自己收口、不留常驻会话），拿**编译期诊断**（类型、uvue 样式规则、模板编译错误）。
 *
 * 守护的不变量：
 *   T1  Invoke-TestAndCompile 函数存在
 *   T2  返回对象含 Ok / Step / Error / TestOutput / CompileResult / Duration
 *   T3  【文本层】quick 模式**真实调用** `hx-run.ps1 -CompileOnly`（不是跳过）
 *   T4  【文本层】standard/full 模式**真实调用** `kotlin-all-check.ps1`（④c 整模块编译门）
 *   T5  测试失败时 Step=test
 *   T6  编译失败时 Step=compile
 *   T7  quick 的结论取自 hx-run 的机检行（HX_RUN …），不靠猜
 *   T8  【结构层】编译期诊断非空 ⇒ Ok=false（fail-closed）
 *   T9  SkipTests 参数存在（dev-finish 已在步骤 3 跑过测试，步骤 4 不重复跑）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/test-compile.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('test-compile.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('T1: Invoke-TestAndCompile function exists', () => {
    expect(src).toContain('function Invoke-TestAndCompile');
  });

  test('T2: returns Ok/Step/Error/TestOutput/CompileResult/Duration', () => {
    ['Ok', 'Step', 'Error', 'TestOutput', 'CompileResult', 'Duration'].forEach((f) => {
      expect(src).toContain(f);
    });
  });

  test('T3: 【文本层】quick mode really calls hx-run.ps1 -CompileOnly', () => {
    // 必须真的当脚本调用：先解析路径变量，再交给 -File（只断言出现字符串会被注释满足）
    expect(src).toMatch(/\$hxRun\s*=\s*Join-Path/);
    expect(src).toMatch(/-File\s+\$hxRun/);
    expect(src).toMatch(/-CompileOnly/);
  });

  test('T4: 【文本层】standard/full mode really calls kotlin-all-check.ps1', () => {
    expect(src).toMatch(/\$kotlinScript\s*=\s*Join-Path/);
    expect(src).toMatch(/-File\s+\$kotlinScript/);
    expect(src).toMatch(/kotlin-all-check\.ps1/);
  });

  test('T5: test failure sets Step=test', () => {
    expect(src).toMatch(/Step\s*=\s*'test'/);
  });

  test('T6: compile failure sets Step=compile', () => {
    expect(src).toMatch(/Step\s*=\s*'compile'/);
  });

  test('T7: quick verdict comes from hx-run machine line (HX_RUN …)', () => {
    expect(src).toContain('HX_RUN');
  });

  test('T8: 【结构层】compile diagnostics non-empty ⇒ Ok=false (fail-closed)', () => {
    expect(src).toMatch(/\$ok\s*=\s*\$false/);
    expect(src).toMatch(/编译期诊断/);
  });

  test('T9: SkipTests parameter exists', () => {
    expect(src).toContain('SkipTests');
  });

  test('T10: quick is NOT treated as "skip compile"', () => {
    // Q-2 修正：跳过编译门等于不做验证，已被废止
    expect(src).not.toContain('跳过编译门');
  });

  test('T11: 【文本层】forwards -TimeoutSeconds to hx-run (cold compile can exceed 900s)', () => {
    // 2026-09-14 实测：本项目冷缓存下 compile-only 跑到 901 秒仍在编译，
    // hx-run 自身默认 900 秒会**过早判环境不可用** ⇒ 必须显式透传更长的超时。
    expect(src).toMatch(/-TimeoutSeconds/);
    expect(src).toContain('$HxRunTimeoutSeconds');
  });

  // T12（2026-09-14 Q-2 修正）：quick **默认不编译**（Q-A 静态守护），只有显式 -QuickCompile 才编译（Q-B）。
  // 依据：实测 compile-only 在冷/失效缓存下 >901 秒，比真运行（4–5 分钟）还慢 ⇒
  // 把编译塞进默认 quick 路径等于谎称「快速」。
  test('T12: quick defaults to static-only (Q-A); compile only when -QuickCompile given', () => {
    expect(src).toContain('$QuickCompile');
    expect(src).toMatch(/\$Level -eq 'quick' -and -not \$QuickCompile/);
    // Q-A 必须**显式声明未做编译诊断**（不是静默跳过）
    expect(src).toContain('未做编译诊断');
    expect(src).toContain('quick_static_only');
  });
});
