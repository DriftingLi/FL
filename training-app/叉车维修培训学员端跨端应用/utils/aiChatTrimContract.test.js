/**
 * AI 助手「自定义模型输入 trim 防御」源码契约测试（T2 / 规格 #552）
 *
 * ai-assistant.uvue 无法在 jest import，沿用源码契约测试缝（读源文本、提取函数体、断言语义）。
 * 先例：utils/secureStorage.test.js。
 *
 * 钉住的行为契约（修复线上报错：Base URL 带前导空白 → 后端 url.Parse 报
 * "first path segment in URL cannot contain colon"）：
 * 1) 请求体构造处（buildBody custom 分支）对 custom_api_key / custom_base_url / custom_model
 *    三字段发送前 trim（兜底，旧本地已存脏值也能救）
 * 2) 确认表单处（confirmCustomModel）对三字段归一化回写（trim 后入 refs，防脏值流入水合/落盘）
 */
const fs = require('fs');
const path = require('path');

const PAGE_PATH = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const src = fs.readFileSync(PAGE_PATH, 'utf8');

/**
 * 提取函数体：兼容两种声明形态——
 * - `function 名(`（composable）
 * - `const 名 : 类型 = (`（uvue 页面箭头函数）
 * 从声明处到该函数顶层缩进闭合的 `\n\t}`（页面脚本段为 tab 缩进）。
 */
function fnBody(name) {
  let start = src.indexOf(`function ${name}`);
  if (start === -1) {
    // 兼容箭头函数声明：const buildBody ... = (
    start = src.indexOf(`const ${name}`);
    if (start !== -1) {
      const assign = src.indexOf('(', start);
      if (assign !== -1) start = assign;
    }
  }
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n\t}', start));
}

describe('自定义模型输入 trim 防御契约', () => {
  describe('buildBody custom 分支（发送前 trim 兜底）', () => {
    const body = fnBody('buildBody');

    it('custom_api_key 发送前 trim', () => {
      expect(body).toContain("params['custom_api_key'] = customApiKey.value.trim()");
    });

    it('custom_base_url 发送前 trim', () => {
      expect(body).toContain("params['custom_base_url'] = customBaseUrl.value.trim()");
    });

    it('custom_model 发送前 trim', () => {
      expect(body).toContain("params['custom_model'] = customModel.value.trim()");
    });
  });

  describe('confirmCustomModel（确认时归一化回写 refs）', () => {
    const body = fnBody('confirmCustomModel');

    it('model / base_url / api_key 确认时均 trim 后回写', () => {
      expect(body).toContain('customModel.value = customModel.value.trim()');
      expect(body).toContain('customBaseUrl.value = customBaseUrl.value.trim()');
      expect(body).toContain('customApiKey.value = customApiKey.value.trim()');
    });
  });
});
