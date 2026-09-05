/**
 * AI 助手「会话标题自动命名」源码契约测试（T1 / 规格 #552）
 *
 * useAiChat.uts 无法在 jest import（.uts 非 JS 模块），沿用项目既有
 * "源码契约测试"缝：读取源文本，提取函数体，断言行为语义。
 * 先例：utils/secureStorage.test.js、utils/pointsBalance.test.js。
 *
 * 钉住的行为契约：
 * 1) 首条用户消息 >20 字时，标题取前 20 字（substring(0, 20)）
 * 2) 首条用户消息 ≤20 字时，标题取全文
 * 3) 空内容兜底「新会话」
 * 4) 该标题在「创建会话时」传入（createAiSessionApi 参数），无事后 rename 额外往返
 *    （零额外请求语义——呼应 ADR-0001 SSE/零冗余；防 future 改为固定标题或改时序）
 */
const fs = require('fs');
const path = require('path');

const COMPOSE_PATH = path.join(__dirname, '..', 'composables', 'useAiChat.uts');
const src = fs.readFileSync(COMPOSE_PATH, 'utf8');

/** 提取函数体：从 `function 名(` 到该函数的顶层 `\n\t}` */
function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n\t}', start));
}

describe('会话标题自动命名契约（doStreamChat 建会话段）', () => {
  const body = fnBody('doStreamChat');

  it('首条消息 >20 字时标题取前 20 字（substring(0, 20)）', () => {
    expect(body).toContain('.content.length > 20');
    expect(body).toContain('.content.substring(0, 20)');
  });

  it('有内容时取全文（>20 才截断，否则原样）', () => {
    expect(body).toContain('history[0].content.length > 20');
  });

  it('空内容兜底「新会话」', () => {
    expect(body).toContain("sessionTitle = '新会话'");
    expect(body).toContain('history[0].content.length > 0');
  });

  it('标题在创建会话时传入（createAiSessionApi 首参），零额外请求', () => {
    // 钉住「创建即携带」：标题直接进 createAiSessionApi，而非建会话后再 rename 往返
    expect(body).toContain('createAiSessionApi(sessionTitle');
  });
});
