/**
 * 诊断来源资料契约测试（#988）—— 源码契约测试缝（读源文本、断言语义；与
 * `aiFeatureEntryWiringContract` / `aiChatImageContract` 同款）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` 与新增的
 * `components/ai-chat/ai-chat-sources.uvue` 都无法在 jest import（uvue），而本票的
 * 行为链横跨四层，任何一层断了都「不报错、不崩溃、只是看不见出处」：
 *
 *   后端 SSE `sources` 事件 → `handleStreamSSEEvent` 分支 → `onSources` 回调
 *   → `useAiChat` 挂到助手消息 → 气泡渲染来源组件
 *   回看路径：`GET /sessions/:id/messages` 的 sources 列 → `buildSessionMessage` → 同一字段
 *
 * 断言纪律（沿用 #999 的教训，重要）：**不得**靠「某行存在」代替「在正确的位置被调用」。
 * 本文件把每条断言压在**函数体切片**里（`fnBody()` 大括号配平，防 #921 那个「切片越界到文件尾」
 * 的假绿路径），并对**注入变异的样本**断言检测器会判红（防「空跑假绿」）。
 *
 * 已知边界（如实声明）：本票**无后端改动**（`sources` 事件与 `ai_chat_messages.sources` 列
 * 早在 ADR-0032 / ADR-0033 落地）。契约测试只能钉客户端这一半；真机门与本地编译门
 * （移动端 `docs/adr/0008` 的 ①②④）另行执行并在 PR 证据段如实记录。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');

const API = read('api/aiAssistant.uts');
const CHAT = read('composables/useAiChat.uts');
const TYPES = read('types/ai.uts');
const TYPES_INDEX = read('types/index.uts');
const SOURCES = read('components/ai-chat/ai-chat-sources.uvue');
const BUBBLE = read('components/ai-chat/ai-chat-bubble.uvue');
const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
// 专项页（#1040 / PR #1059 之后的**诊断入口**）：诊断对话现在只发生在这一页 —— 只给通用页
// 接线等于把本票交付到用户到不了的页面。
const FEATURE_PAGE = read('pages/ai-assistant/ai-feature.uvue');

/** 取某个函数体（大括号配平，切到闭合处为止）—— 防止切片越界到文件尾造成假绿。 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error(`找不到源码片段: ${sig}`);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error(`片段后找不到 '{': ${sig}`);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open + 1, i);
    }
  }
  throw new Error(`大括号不配平: ${sig}`);
}

/** SSE 处理器里 `sources` 分支之后的那段文本（分支被删/被搬走 ⇒ 返回空串） */
function sseSourcesBranch(src) {
  const body = fnBody(src, 'function handleStreamSSEEvent');
  const i = body.indexOf("event == 'sources'");
  return i < 0 ? '' : body.slice(i);
}

/** 来源组件 `<style>` 块内容 */
function styleBlock(src) {
  const m = src.match(/<style[^>]*>([\s\S]*?)<\/style>/);
  if (!m) throw new Error('找不到 <style> 块');
  return m[1];
}

describe('#988 诊断来源资料消费契约', () => {
  // ────────────────────────────────────────────────────────────────
  describe('① SSE：`sources` 事件必须被解析并回调（此前被静默丢弃）', () => {
    it('`sources` 分支在 handleStreamSSEEvent 内、且真的调用了解析与回调', () => {
      const branch = sseSourcesBranch(API);
      expect(branch.length).toBeGreaterThan(0);                       // 锚点：分支得在
      expect(branch).toContain('parseDiagnosisSources(');              // 解析走单点
      expect(branch).toContain('callbacks.onSources(');                // 回调真的被调用
    });

    it('①b 检测器自检：删掉整个分支后，① 的判据必须判红（防空跑假绿）', () => {
      const branch = sseSourcesBranch(API);
      const mutated = API.replace(branch, '\n\t\t// 分支被删（变异样本）\n');
      expect(sseSourcesBranch(mutated)).toBe('');                      // 切片找不到入口
      expect(sseSourcesBranch(mutated)).not.toContain('callbacks.onSources(');
    });

    it('①c 检测器自检：分支还在、但回调被换成空实现时，① 的判据必须判红', () => {
      const branch = sseSourcesBranch(API);
      const mutated = API.replace(branch, branch.replace(
        'callbacks.onSources(parseDiagnosisSources(raw as Array<UTSJSONObject>))',
        'void 0'));
      expect(sseSourcesBranch(mutated)).not.toContain('callbacks.onSources(');
    });

    it('`StreamChatCallbacks` 暴露 onSources（否则回调无从传进来）', () => {
      const t = fnBody(API, 'export type StreamChatCallbacks');
      expect(t).toMatch(/onSources\s*\?:\s*\(/);
    });

    it('兜底：分支带 try/catch，坏 payload 不得把整条流打断', () => {
      const branch = sseSourcesBranch(API);
      expect(branch).toContain('try {');
      expect(branch).toContain('catch');
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('② 单点解析：实时与回看走同一份实现（形状必须一致）', () => {
    it('`parseDiagnosisSources` 只定义一次', () => {
      const defs = API.match(/function parseDiagnosisSources\s*\(/g) || [];
      expect(defs.length).toBe(1);
    });

    it('定义体把 metadata 展平并按字段兜底（page_start / page_end / source_url）', () => {
      const body = fnBody(API, 'export function parseDiagnosisSources');
      expect(body).toContain("item['metadata']");
      expect(body).toContain("m['page_start']");
      expect(body).toContain("m['page_end']");
      expect(body).toContain("m['source_url']");
      expect(body).toContain('toNumber(');            // 数字字段兜底
    });

    it('空 text 的坏条目被丢弃（不给用户渲染空气泡）', () => {
      const body = fnBody(API, 'export function parseDiagnosisSources');
      expect(body).toMatch(/text\.length == 0\)\s*continue/);
    });

    it('两处调用点都在：SSE 分支 与 buildSessionMessage', () => {
      expect(sseSourcesBranch(API)).toContain('parseDiagnosisSources(');
      expect(fnBody(API, 'function buildSessionMessage')).toContain('parseDiagnosisSources(');
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('③ 回看：`buildSessionMessage` 必须读 sources 列并落到消息字段上', () => {
    it('读 `obj[\'sources\']` 且赋给 `msg.sources`', () => {
      const body = fnBody(API, 'function buildSessionMessage');
      expect(body).toContain("obj['sources']");
      expect(body).toContain('msg.sources = ');
    });

    it('③b 检测器自检：删掉映射后必须判红', () => {
      const body = fnBody(API, 'function buildSessionMessage');
      const mutated = API.replace(body, body.replace('msg.sources = ', '/* dropped */ '));
      expect(fnBody(mutated, 'function buildSessionMessage')).not.toContain('msg.sources = ');
    });

    it('空数组不写入字段（避免 `sources: []` 让气泡的多余判断为真）', () => {
      const body = fnBody(API, 'function buildSessionMessage');
      expect(body).toMatch(/list\.length > 0[\s\S]{0,40}msg\.sources = list/);
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('④ 类型层：AiSessionMessage 有 sources，AiDiagnosisSource 已导出', () => {
    it('`AiDiagnosisSource` 三字段形状（id/text 必填，页码与 URL 可选）', () => {
      const body = fnBody(TYPES, 'export type AiDiagnosisSource');
      expect(body).toMatch(/id\s*:\s*string/);
      expect(body).toMatch(/text\s*:\s*string/);
      expect(body).toMatch(/page_start\s*\?\s*:\s*number/);
      expect(body).toMatch(/page_end\s*\?\s*:\s*number/);
      expect(body).toMatch(/source_url\s*\?\s*:\s*string/);
    });

    it('`AiSessionMessage.sources` 是可选字段', () => {
      const body = fnBody(TYPES, 'export type AiSessionMessage');
      expect(body).toMatch(/sources\s*\?\s*:\s*AiDiagnosisSource\[\]/);
    });

    it('barrel 导出带上新类型（消费方从 types/index 取）', () => {
      expect(TYPES_INDEX).toContain('type AiDiagnosisSource');
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('⑤ composable：当轮来源必须挂到那条助手消息上', () => {
    it('`doStreamChat` 给 streamChatApi 传了 onSources 且写入 assistantMsg.sources', () => {
      const body = fnBody(CHAT, 'function doStreamChat');
      expect(body).toContain('onSources:');                       // 回调注册
      expect(body).toContain('assistantMsg.sources = sources');   // 落到同一字段
    });

    it('⑤b 检测器自检：把赋值删掉后必须判红', () => {
      const body = fnBody(CHAT, 'function doStreamChat');
      const mutated = CHAT.replace(
        body, body.replace('assistantMsg.sources = sources', '/* dropped */'));
      expect(fnBody(mutated, 'function doStreamChat'))
        .not.toContain('assistantMsg.sources = sources');
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('⑥ 渲染：来源组件 + 气泡接线 + 页面传参', () => {
    it('来源组件**不再自己解析**：纯函数走 utils/aiSourcesDisplay（ADR-0007 单点，禁第二实现）', () => {
      // 为什么这条是承重的：这些纯函数原先住在组件里 ⇒ jest 无法 import `.uvue` ⇒ 守护只能
      // 断言**源码文本**（改坏行为、文本还在，测试照样绿）。抽到 utils 后行为由**真单测**执行
      // （`utils/aiSourcesDisplay.test.js` 的镜像实现），本组只守「组件没有长出第二实现」。
      expect(SOURCES).toContain("import { extractImagePaths, stripImageMarkers, pageLabel } from '../../utils/aiSourcesDisplay'");
      expect(SOURCES).not.toContain('function stripAssistantPrefix');
      expect(SOURCES).not.toContain('function extractImagePaths');
      expect(SOURCES).not.toContain('function stripImageMarkers');
      expect(SOURCES).not.toContain('function pageLabel');
      expect(SOURCES).not.toContain("const IMAGE_OPEN = '<<IMAGE:'");
    });

    it('真单测在位（防「把镜像测试删掉、只留文本断言」的回潮）', () => {
      // 锚点：`<<IMAGE:…>>` 的解析行为必须有一份**可执行**的测试在守。
      const displayTest = read('utils/aiSourcesDisplay.test.js');
      expect(displayTest).toContain('function extractImagePaths');
      expect(displayTest).toContain('function stripImageMarkers');
      expect(displayTest).toContain("describe('aiSourcesDisplay：镜像同步");
    });

    it('子路径 → URL 走 aiManualUrl（组件不手拼 base）', () => {
      expect(SOURCES).toContain("import { aiManualUrl } from '../../api/aiAssistant'");
      expect(SOURCES).toContain('aiManualUrl(paths[i])');
      expect(SOURCES).not.toMatch(/API_BASE_URL\s*\+/);     // 页面/组件里不许自己拼 base
    });

    it('图片可点开大图（uni.previewImage）', () => {
      expect(SOURCES).toContain('uni.previewImage(');
      expect(SOURCES).toContain('current: url');
    });

    it('气泡在助手消息且有来源时才渲染来源组件', () => {
      expect(BUBBLE).toContain("import AiChatSources from './ai-chat-sources.uvue'");
      expect(BUBBLE).toMatch(/v-if="role == 'assistant' && sources\.length > 0"/);
      expect(BUBBLE).toContain('<AiChatSources');
    });

    it('气泡的 sources 默认值是空数组 ⇒ 未传值的调用方零 diff（R2）', () => {
      expect(BUBBLE).toMatch(/sources\s*:\s*\(\)\s*=>\s*\[\]\s*as\s*AiDiagnosisSource\[\]/);
    });

    it('两个页面都把消息上的 sources 传进气泡（通用页 + 专项页）', () => {
      expect(PAGE).toContain(':sources="msg.sources"');
      expect(FEATURE_PAGE).toContain(':sources="msg.sources"');
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('⑦ 手册代理 URL：断言**产物**，不是片段（复盘：片段断言放过过一个 404）', () => {
    /**
     * 检测器：`aiManualUrl` 是否拼了**完整**路径段。
     * 提成纯函数是为了让「注入变异必须判红」能真的验证这个判据本身 —— 只断言片段存在
     * （旧版写 `expect(body).toContain("'/diagnosis/manual/'")`）对**丢段**完全无感：
     * 少了 `/ai-assistant` 的坏实现照样满足它，而线上每张手册图都 404。
     */
    function manualSegmentIsComplete(src) {
      const body = fnBody(src, 'export function aiManualUrl');
      return body.includes("API_BASE_URL + '/ai-assistant/diagnosis/manual/'");
    }

    it('锚点：`API_BASE_URL` 不含 `/ai-assistant` ⇒ 构造器必须自己带上这一段', () => {
      const env = read('config/env.uts');
      expect(env).toContain("const DEV_API_BASE_URL = 'https://www.gccsmile.com/api'");
      expect(env).toContain("const PROD_API_BASE_URL = 'https://www.gccsmile.com/api'");
      // 若哪天 base 自带 /ai-assistant，这条会红 —— 那时才允许改构造器（并同步本组）。
      expect(env).not.toContain('/api/ai-assistant');
    });

    it('`aiManualUrl` 拼出 `/ai-assistant/diagnosis/manual/` **全段**', () => {
      expect(manualSegmentIsComplete(API)).toBe(true);
    });

    it('⑦a 检测器自检：改回「丢 /ai-assistant」的历史写法必须判红', () => {
      const body = fnBody(API, 'export function aiManualUrl');
      const mutated = API.replace(body, body.replace(
        "'/ai-assistant/diagnosis/manual/'", "'/diagnosis/manual/'"));
      expect(manualSegmentIsComplete(mutated)).toBe(false);   // 判据真的能判红，不是空跑
    });

    it('产物形状：三片拼起来 == 直连实测可用的那条 URL', () => {
      // 三片 = base（config/env.uts 的常量） + 段（构造器自己拼） + 已编码子路径。
      const BASE = 'https://www.gccsmile.com/api';
      const SEGMENT = '/ai-assistant/diagnosis/manual/';
      const SUBPATH = 'ep_byd_pmw20_service_manual_en/page_85_643.png';
      // 这条 URL 是 2026-09-16 直连生产验过的：带段 → 200 / image/png；丢段 → 404。
      expect(BASE + SEGMENT + SUBPATH).toBe(
        'https://www.gccsmile.com/api/ai-assistant/diagnosis/manual/ep_byd_pmw20_service_manual_en/page_85_643.png');
      const body = fnBody(API, 'export function aiManualUrl');
      expect(body).toContain("API_BASE_URL + '" + SEGMENT + "'");   // 与上面同一个 SEGMENT 常量
      expect(body).toContain("segs.join('/')");                     // 子路径那一片
      expect(body).toContain('encodeURIComponent(');
      expect(body).toContain("split('/')");
    });

    it('⑦b 检测器自检：去掉 encodeURIComponent 后必须判红（中文目录名会 400）', () => {
      const body = fnBody(API, 'export function aiManualUrl');
      const mutated = API.replace(body, body.replace('encodeURIComponent(p)', 'p'));
      expect(fnBody(mutated, 'export function aiManualUrl')).not.toContain('encodeURIComponent(');
    });

    it('⑦c 回归守护：`encodeURIComponent` 在 UTS 返回 `String?`，直接 push 进 string[] 会编译失败', () => {
      // 这不是假想：本票第一次 ④c（`build:kotlin-all`）就死在这里 ——
      // `index.kt:16553: error: argument type mismatch: actual type is 'String?', but 'String' was expected.`
      const lines = API.split('\n').filter((l) => l.includes('.push(encodeURIComponent('));
      expect(lines.length).toBeGreaterThan(0);              // 锚点：该写法仍在，别退化成空跑
      for (const l of lines) {
        expect(l).toMatch(/\?\?/);                          // 必须有非空兜底
      }
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('⑧ uvue 约束：新组件的样式块不得踩原生端不支持的写法', () => {
    it('`<style>` 声明 lang="scss"', () => {
      expect(SOURCES).toMatch(/<style lang="scss">/);
    });

    it('样式里没有 `gap`（uvue 静默忽略 ⇒ 布局会坏）', () => {
      expect(styleBlock(SOURCES)).not.toMatch(/\bgap\s*:/);
    });

    it('样式里没有 tag 选择器（uvue 只支持 class / group）', () => {
      expect(styleBlock(SOURCES))
        .not.toMatch(/^\s*(view|text|image|scroll-view|input)\s*\{/m);
    });

    it('样式里没有 CSS 变量（原生端不支持 var(--x)）', () => {
      expect(styleBlock(SOURCES)).not.toContain('var(--');
    });

    it('间距用子元素 margin class（对齐 `gap` 替换模式）', () => {
      expect(styleBlock(SOURCES)).toContain('margin-top');
      expect(SOURCES).toContain("'source-card-gap': ci > 0");
      expect(SOURCES).toContain("'source-image-gap': ii > 0");
    });
  });

  // ────────────────────────────────────────────────────────────────
  describe('⑨ 专项页不得引入模型前置校验（#1059 结构性口径；替代原「带键豁免」断言）', () => {
    // 背景：#988 早期的第二个提交曾给**通用页**的守卫加 `featureKey.length == 0` 前缀，让带键通道
    // 绕开「平台模型列表为空」这道门。**该实现已作废**：PR #1059（#1040）把专项对话搬到了
    // `ai-feature.uvue`，该页不加载模型列表、也不设模型前置校验 ⇒ 那个问题被**结构性**消解。
    // 本组守护的是**新结构**：若有人把通用页那道守卫（或同类早退）复制进专项页，就等于把 #1045
    // 报过的缺陷重新引入一次 —— 平台模型列表为空时，专项通道会再次静默失效（只弹 toast、不发请求）。
    it('专项页的 onInputSend 不做任何模型前置校验', () => {
      const fn = fnBody(FEATURE_PAGE, 'function onInputSend');
      expect(fn.length).toBeGreaterThan(0);              // 锚点：函数在
      expect(fn).toContain('sendInput(');                // 锚点：它确实在发送
      expect(fn).not.toContain('暂无可用的 AI 模型');
      expect(fn).not.toContain('models.value.length');
      expect(fn).not.toContain('userModels.value.length');
    });

    it('⑨b 检测器自检：把通用页那道守卫注入专项页后必须判红', () => {
      const fn = fnBody(FEATURE_PAGE, 'function onInputSend');
      const mutated = FEATURE_PAGE.replace(fn, fn.replace(
        'inputText.value = text',
        "if (models.value.length == 0) { uni.showToast({ title: '暂无可用的 AI 模型', icon: 'none' }); return }\n\t\tinputText.value = text"));
      const m = fnBody(mutated, 'function onInputSend');
      expect(m).toContain('暂无可用的 AI 模型');
    });

    it('专项页也不加载模型列表（与「不做前置校验」互为佐证）', () => {
      expect(FEATURE_PAGE).not.toContain('getAiModelsApi');
      expect(FEATURE_PAGE).not.toContain('loadModels');
    });
  });
});
