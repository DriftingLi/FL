/**
 * AI 助手「会话重命名 API」源码契约测试（T3 / 规格 #552）
 *
 * aiAssistant.uts 无法在 jest import，沿用源码契约测试缝（读源文本、提取函数体、断言语义）。
 * 先例：utils/secureStorage.test.js。
 *
 * 钉住的行为契约（为后续「手动改名」功能预留的备用通道）：
 * 1) 重命名走 PATCH /ai-assistant/sessions/:id/title（方法 + 路径）
 * 2) 请求体携带新标题
 * 3) 响应按 ADR-0003 手动 JSON 映射构造会话对象（buildSession 范式）
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const API_PATH = path.join(__dirname, '..', 'api', 'aiAssistant.uts');
const src = readText(API_PATH);

/**
 * 提取重命名函数体：兼容 `export function 名(` 声明。
 * 从声明处到该函数顶层缩进闭合的 `\n}`（API 层为 tab 缩进）。
 */
function fnBody(name) {
  let start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('会话重命名 API 契约（renameAiSessionApi）', () => {
  const body = fnBody('renameAiSessionApi');

  it('走 PATCH .../sessions/:id/title 的路径（patch 调用 + /title 端），请求体带 title', () => {
    expect(body).toContain("patch('/ai-assistant/sessions/' + id + '/title'");
    expect(body).toContain('{ title: title }');
  });

  it('响应按手动 JSON 映射构造会话对象（buildSession 范式，ADR-0003）', () => {
    expect(body).toContain('buildSession');
  });
});
