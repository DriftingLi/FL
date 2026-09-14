/**
 * 证据生成模块契约守护（scripts/lib/evidence-gen.ps1）
 *
 * 守护的不变量：
 *   G1 New-Evidence 函数存在
 *   G2 返回对象包含 Generated / Path / Content / Message
 *   G3 quick 模式 Generated=false
 *   G4 standard 模式包含编译结果和截图结论
 *   G5 full 模式包含全部四门
 *   G6 输出格式包含 ## 验收证据 段
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/lib/evidence-gen.ps1';

function readSource() {
  return fs.readFileSync(path.join(ROOT, SCRIPT_REL), 'utf8');
}

describe('evidence-gen.ps1 contract', () => {
  let src;

  beforeAll(() => {
    src = readSource();
  });

  test('G1: New-Evidence function exists', () => {
    expect(src).toContain('function New-Evidence');
  });

  test('G2: returns object with Generated, Path, Content, Message', () => {
    expect(src).toContain('Generated');
    expect(src).toContain('Path');
    expect(src).toContain('Content');
    expect(src).toContain('Message');
  });

  test('G3: quick mode returns Generated=false', () => {
    expect(src).toContain("Generated = $false");
    expect(src).toContain('免（低风险运行时面');
  });

  test('G4: standard mode includes compile and screenshot info', () => {
    expect(src).toContain('④ 本地编译门');
    expect(src).toContain('CompileResult');
  });

  test('G5: full mode includes all gates', () => {
    expect(src).toContain('① Android 真机逐页截图');
    expect(src).toContain('④ 本地编译门');
  });

  test('G6: output contains ## 验收证据', () => {
    expect(src).toContain('## 验收证据');
  });
});
