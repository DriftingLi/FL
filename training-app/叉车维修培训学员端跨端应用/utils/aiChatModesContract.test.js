/**
 * AI 助手「专业版消费双模式 modes」源码契约测试（T5 / 规格 #552）
 *
 * aiAssistant.uts / ai-assistant.uvue 无法在 jest import，沿用源码契约测试缝
 * （读源文本、提取函数体、断言语义）。先例：utils/secureStorage.test.js。
 *
 * 钉住的行为契约：
 * 1) API 层提供 getAiModesApi（GET /ai-assistant/modes，normal/expert 双绑定）
 * 2) 专业版页面消费 modes：将 normal/expert 并入模型展示（与 /models 并存）
 * 3) 选中某模式后，请求体以 model_source=admin 驱动并携带对应 config_id
 */
const fs = require('fs');
const path = require('path');

const API_PATH = path.join(__dirname, '..', 'api', 'aiAssistant.uts');
const PAGE_PATH = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-assistant.uvue');
const apiSrc = fs.readFileSync(API_PATH, 'utf8');
const pageSrc = fs.readFileSync(PAGE_PATH, 'utf8');

/** 兼容 `export function 名(` 的提取辅助 */
function fnBody(src, name) {
  let start = src.indexOf(`function ${name}`);
  if (start === -1) start = src.indexOf(`export function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('双模式 models 消费契约', () => {
  describe('API 层 getAiModesApi（GET /ai-assistant/modes）', () => {
    const body = fnBody(apiSrc, 'getAiModesApi');

    it('走 /ai-assistant/modes 端点', () => {
      expect(body).toContain('/ai-assistant/modes');
    });

    it('解析并回填 normal / expert 双模型', () => {
      expect(body).toContain("['normal']");
      expect(body).toContain("['expert']");
    });
  });

  // #657 契约对齐实现：本块原断言 `getAiModesApi` / `selectMode`，但页面实际消费的是
  // `getAiModelsApi` / `selectModel` —— 契约自 #552 诞生起即基于错误假设（非后来改坏）。
  // 维护者决策 = 路径 A：契约对齐实现，不改页面代码。
  // 另：原第三条 `toContain('selectMode')` 曾被 `selectModel` 的**子串**意外命中而假绿
  //（同 ADR-0007「零命中锁被子串命中」一类），此处一并换成语义断言。
  describe('专业版页面消费模型源（#657 对齐实现）', () => {
    it('页面引入 getAiModelsApi（模型列表来源；getAiModesApi 仅 API 层导出）', () => {
      expect(pageSrc).toContain('getAiModelsApi');
    });

    it('selectModel 将选中模型落到 user/admin 源并落盘', () => {
      const start = pageSrc.indexOf('function selectModel');
      expect(start).toBeGreaterThan(-1);
      const body = pageSrc.slice(start, pageSrc.indexOf('\n\t}', start));
      expect(body).toContain('currentModelId.value = m.id');
      expect(body).toContain("currentModelSource.value = isUser ? 'user' : 'admin'");
      expect(body).toContain('persistSettings()');
    });

    it('admin 源请求体由 currentModelSource 驱动（语义断言，非子串巧合）', () => {
      expect(pageSrc).toContain('model_source: currentModelSource.value');
    });
  });
  });
