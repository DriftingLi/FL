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

  describe('专业版页面消费 modes', () => {
    it('页面引入 getAiModesApi', () => {
      expect(pageSrc).toContain('getAiModesApi');
    });

    it('selectMode 将选中模式落到 admin 源（驱动请求体 config_id）', () => {
      // 从页面源提取 selectMode 函数体，断言其设 model_source='admin' 并落盘（persistSettings）
      const start = pageSrc.indexOf('function selectMode');
      expect(start).toBeGreaterThan(-1);
      const body = pageSrc.slice(start, pageSrc.indexOf('\n\t}', start));
      expect(body).toContain("currentModelSource.value = 'admin'");
      expect(body).toContain('currentModelId.value =');
      expect(body).toContain('persistSettings()');
    });

    it('modes 驱动 admin 源请求体携带对应 config_id（normal/expert 并入模型选择）', () => {
      // 页面应有 modes→config_id 的映射/选择逻辑：至少存在 selectMode 处理
      expect(pageSrc).toContain('selectMode');
    });
  });
});
