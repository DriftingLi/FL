/**
 * 请求基座 mapper-callback 泛型出口契约测试（TDD: RED → GREEN 钉住行为契约）
 *
 * request.uts 无法在 jest 中 import（.uts 非 JS 模块），沿用项目既有的
 * 「源码契约测试」缝：读取源文本，断言函数体的外部行为语义。
 * 先例：utils/quickLoginContract.test.js、utils/secureStorage.test.js。
 *
 * 钉住的行为契约（对应 Issue #639 / refactor epic #638）：
 * 1) 出口签名：requestMapped<T>(options, map) —— mapper-callback 形态，
 *    域 api 传入自己的 DTO 映射函数（与 ADR-0003 手动 JSON 映射同构）
 * 2) 拆包后映射：mapper 收到的是 ApiResponse 拆包后的 data（非信封）
 * 3) 失败传播：映射函数抛错不被静默吞掉，沿 Promise 链 reject
 * （切片 2/3 的契约用例随后续 commit 追加）
 */
const fs = require('fs');
const path = require('path');

const REQ_PATH = path.join(__dirname, '..', 'api', 'request.uts');
const src = fs.readFileSync(REQ_PATH, 'utf8');

/** 提取函数体：从函数声明到顶层级 "\n}"（先例：quickLoginContract） */
function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('出口签名契约（requestMapped）', () => {
  it('导出泛型出口 requestMapped<T>：RequestOptions + mapper 回调，返回 Promise<T>', () => {
    const sig = /export\s+function\s+requestMapped\s*<\s*T\s*>\s*\(\s*options\s*:\s*RequestOptions\s*,\s*map\s*:\s*\(\s*data\s*:\s*UTSJSONObject\s*\)\s*=>\s*T\s*\)\s*:\s*Promise\s*<\s*T\s*>/;
    expect(sig.test(src)).toBe(true);
  });
});

describe('拆包后映射契约', () => {
  it('出口复用底层 request 的解包结果，mapper 收到 data 而非信封', () => {
    const body = fnBody('requestMapped');
    expect(body).toContain('request<UTSJSONObject>(options)');
    expect(body).toMatch(/\.then\s*\(/);
    expect(body).toMatch(/map\s*\(\s*data\s*\)/);
  });

  it('出口内不二次解包：不得再取信封 data 字段（mapper 收到的已是拆包结果）', () => {
    const body = fnBody('requestMapped');
    expect(body).not.toContain("['data']");
    expect(body).not.toContain('["data"]');
  });
});
