/**
 * AI 助手「专业版图片随对话发送」源码契约测试（T4 / 规格 #552）
 *
 * aiAssistant.uts / types/index.uts / ai-assistant.uvue 无法在 jest import，
 * 沿用源码契约测试缝（读源文本、提取函数体、断言语义）。先例：utils/secureStorage.test.js。
 *
 * 钉住的行为契约：
 * 1) API 层提供 uploadAiImageApi，调用 AI 专用 `/ai-assistant/upload-image`（复用通用
 *    uploadFile，multipart）；并严禁复用 `/forum/upload-image`
 * 2) 上传成功取回图片 URL
 * 3) 消息体类型支持 images 承载（AiChatMessage 携带可选 images）——后端
 *    StreamChatReq.Messages[].Images 仅最后一条用户消息生效
 */
const fs = require('fs');
const path = require('path');

const API_PATH = path.join(__dirname, '..', 'api', 'aiAssistant.uts');
const TYPES_PATH = path.join(__dirname, '..', 'types', 'index.uts');
const apiSrc = fs.readFileSync(API_PATH, 'utf8');
const typesSrc = fs.readFileSync(TYPES_PATH, 'utf8');

/** 兼容 `export function 名(` 的提取辅助 */
function fnBody(src, name) {
  let start = src.indexOf(`function ${name}`);
  if (start === -1) start = src.indexOf(`export function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('图片随对话发送契约', () => {
  describe('API 层 uploadAiImageApi（走 AI 专用端点，禁复用论坛端点）', () => {
    const body = fnBody(apiSrc, 'uploadAiImageApi');

    it('调用 AI 专用 /ai-assistant/upload-image 上传', () => {
      expect(body).toContain('/ai-assistant/upload-image');
    });

    it('复用通用 uploadFile 上传（multipart）', () => {
      expect(body).toContain('uploadFile');
    });

    it('严禁复用 /forum/upload-image', () => {
      expect(body).not.toContain('/forum/upload-image');
    });

    it('上传成功取回图片 URL', () => {
      expect(body).toContain("['url']");
    });
  });

  describe('消息体类型承载 images（AiChatMessage 可选 images）', () => {
    it('AiChatMessage 声明可选 images 数组', () => {
      const mt = typesSrc.slice(typesSrc.indexOf('export type AiChatMessage'), typesSrc.indexOf('\n}', typesSrc.indexOf('export type AiChatMessage')));
      expect(mt).toContain('images ?: string[]');
    });
  });
});
