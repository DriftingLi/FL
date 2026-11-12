/**
 * #2310 · `done.content` 是终端权威全文 —— **行为级**守护（端到端真执行）
 *
 * ## 它在守什么
 *
 * 后端 `done` 载荷里的 `content` 是「终端权威全文」：只在它与已推增量不等时出现
 * （判据 `fullContent != streamed.String()`，见 `backend/internal/aiassistant/handler.go`）。
 * 成因**不止**配图归一：现网三轮实测里「中力 E01 轮少了尾部免责声明」「纯图片轮追加了兜底追问」
 * 都没有配图改写。故客户端口径是**无条件的「有 content 就整体替换累积正文」**。
 *
 * 移动端 `feat/2296` 落地时 `api/aiAssistant.uts` 的 done 分支只调 `onDone()`、不读
 * `data.content`（#2296 已随 PR #2311 合入 master，缺陷随它进主干）⇒ 纯图轮丢掉追问、
 * 配图轮显示改写前的正文。票面验收：「两条分支各有真执行的行为用例」。
 *
 * ## 为什么必须真跑
 *
 * 读源码文本只能证明「有一行赋值」，证明不了「替换真的发生在同一条助手消息上」、
 * 也证明不了「不带 content 时累积增量没被动过」。本文件把被测面**串起来真执行**：
 *
 *   `api/aiAssistant.uts` 的 `streamChatApi` 真解析 SSE（替身只给 `uni.request` 回包）
 *     → `composables/useAiChat.uts` 的 `sendInput` 真组装消息 / 累积增量 / 收尾
 *     → 断言 `messages.value` 里那条助手消息的最终 `content`
 *
 * 断言的是**产出物**（气泡里那句话），不是字面量；变异取证（末组）证明判据有牙。
 * `utsHarness.loadUts` 的 fail-closed：任一模块新增/减少 import 都当场红，不会静默少注入。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const ENVELOPE_UTS = path.join(ROOT, 'api', 'envelope.uts');
const API_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');
const CHAT_UTS = path.join(ROOT, 'composables', 'useAiChat.uts');
/** #2299 起 useAiChat 的早退判据 `canSend` 住这里；按**真件**注入（不是手抄一份等价逻辑） */
const QUOTA_UTS = path.join(ROOT, 'utils', 'aiImageQuota.uts');

/** `data: <json>` 的 SSE 帧；doneData 为 null 时写 `null`（非诊断轮的真实形态） */
function sseOf(chunks, doneData) {
  let s = '';
  for (const c of chunks) s += 'event: message\ndata: ' + JSON.stringify({ content: c }) + '\n\n';
  s += 'event: done\ndata: ' + (doneData === null ? 'null' : JSON.stringify(doneData)) + '\n\n';
  return s;
}

/**
 * 装载两个真源模块并把它们接起来。
 * - API 侧：`uni.request` 替身立即回包（本仓 `streamChatApi` 是整包解析，不是增量回调）；
 * - Chat 侧：`streamChatApi` 绑定成真 API 模块的导出，其余会话 API 给最小替身。
 *
 * `import type { Ref } from 'vue'` 在 `utsHarness` 里会被识别成绑定名（该 harness 只支持
 * `import { … }` / `import type { … }`）；这里在**临时副本**里把两行拆开归一化，
 * 不改工作树真源。归一化后该 import 变成纯类型、被 stripUtsPlugin 去掉，绑定清单回到 6 个运行期符号。
 */
function loadPair(options) {
  const opt = options || {};
  const h = loadUts(HELPERS_UTS, {});
  const quota = loadUts(QUOTA_UTS, {});
  const envelope = loadUts(ENVELOPE_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    mergeQueryParams: () => '',
  });
  const responseText = opt.sse == null ? sseOf(['增量A', '增量B'], { content: '权威全文' }) : opt.sse;
  const uni = {
    request: (o) => { o.success({ statusCode: 200, data: responseText }); return {}; },
    showToast: () => {},
    showModal: () => {},
  };
  const api = loadUts(opt.apiFile || API_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    patchMapped: () => Promise.resolve({}),
    del: () => Promise.resolve({}),
    uploadFile: () => Promise.resolve({}),
    renewOrExpire: () => Promise.resolve(false),
    AUTH_EXPIRED_MESSAGE: '登录已过期',
    decodeList: envelope.decodeList,
    decodePage: envelope.decodePage,
    toNumber: h.toNumber,
    toStr: h.toStr,
    toBool: h.toBool,
    API_BASE_URL: 'https://example.test/api',
    AI_REQUEST_TIMEOUT: 60000,
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'access-token',
    createSseParser: loadUts(path.join(ROOT, 'utils', 'sseIncrementalParser.uts'), {}).createSseParser,
    decodeUtf8: loadUts(path.join(ROOT, 'utils', 'sseIncrementalParser.uts'), {}).decodeUtf8,
    uni: uni,
    console: console,
  });

  let chatSrc = readText(CHAT_UTS);
  const originalImport = "import { ref, type Ref } from 'vue'";
  expect(chatSrc.includes(originalImport)).toBe(true); // 归一化锚点必须真的在源里
  chatSrc = chatSrc.replace(originalImport, "import { ref } from 'vue'\nimport type { Ref } from 'vue'");
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-done-content-'));
  const chatFile = path.join(dir, 'useAiChat.uts');
  fs.writeFileSync(chatFile, chatSrc);

  const chat = loadUts(chatFile, {
    ref: (v) => ({ value: v }),
    getAiSessionsApi: () => Promise.resolve([]),
    createAiSessionApi: () => Promise.resolve({ id: 1, title: '' }),
    deleteAiSessionApi: () => Promise.resolve(),
    getAiSessionMessagesApi: () => Promise.resolve([]),
    streamChatApi: api.streamChatApi,
    // #2361：useAiChat 的 onDone 新增运行期依赖 parseDiagnosisMeta（与回看同一个解析器）
    parseDiagnosisMeta: api.parseDiagnosisMeta,
    toStr: h.toStr,
    // #2299 起 useAiChat 的早退判据是 `canSend`（有字 ∨ 有图）—— 依赖注入按真件补
    canSend: quota.canSend,
    uni: uni,
    setTimeout: () => {},
    console: console,
  });
  return { api, chat, uni };
}

/**
 * 发一轮（真调 sendInput）。
 * doStreamChat 是 async：首轮会 await createAiSessionApi / loadSessions 才把助手占位
 * push 进 messages，故必须 flush 微任务后再读气泡 —— 这不是替身造假，是真实时序。
 */
async function sendOne(chat) {
  const state = chat.useAiChat({ value: '' });
  state.inputText.value = '这是一条用户问题';
  state.sendInput('标签', (_history, _sid, _key) => ({}), [], '');
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  return state;
}

/** 气泡里那条助手消息的正文 */
function assistantText(state) {
  const list = state.messages.value;
  const assistant = list.filter((m) => m.role === 'assistant');
  expect(assistant.length).toBe(1);
  return assistant[0].content;
}

describe('#2310 done.content 是终端权威全文（行为级，端到端真执行）', () => {
  it('done 带 content ⇒ 助手气泡整体替换为 content（不是追加、不是保留累积）', async () => {
    const { chat } = loadPair({ sse: sseOf(['增量A', '增量B'], { content: '权威全文（配图已归一）' }) });
    const state = await sendOne(chat);
    expect(assistantText(state)).toBe('权威全文（配图已归一）');
    expect(state.thinking.value).toBe(false);
  });

  it('纯图片轮那种「done 追加兜底追问」同样无条件替换（不是只在配图轮才替换）', async () => {
    const accumulated = '这台车的情况资料暂时没覆盖到。';
    const authoritative = accumulated + '🤔 麻烦您补充一下品牌、型号和故障码…';
    const { chat } = loadPair({ sse: sseOf([accumulated], { content: authoritative }) });
    expect(assistantText(await sendOne(chat))).toBe(authoritative);
  });

  it('done 不带 content（只有 meta）⇒ 保留累积增量，一字不动', async () => {
    const { chat } = loadPair({ sse: sseOf(['增量A', '增量B'], { meta: { badges: [], elapsed: 10 } }) });
    expect(assistantText(await sendOne(chat))).toBe('增量A增量B');
  });

  it('done 的 data 为 null（非诊断功能）⇒ 容空、保留累积增量、不抛', async () => {
    const { chat } = loadPair({ sse: sseOf(['增量A'], null) });
    expect(assistantText(await sendOne(chat))).toBe('增量A');
  });

  it('done 带空串 content ⇒ 不替换（空串不是权威全文）', async () => {
    const { chat } = loadPair({ sse: sseOf(['增量A'], { content: '' }) });
    expect(assistantText(await sendOne(chat))).toBe('增量A');
  });

  describe('检测器有牙：注入变异的副本，证明上面判据真能判红', () => {
    it('把 done 分支的载荷解析删掉（退回 onDone()）⇒ 替换用例当场红', async () => {
      const src = readText(API_UTS);
      const anchor = "\t} else if (event == 'done') {";
      expect(src.includes(anchor)).toBe(true);
      // 纯字符串切片：从 done 分支起点切到函数末尾，换成「只调 onDone()」的坏实现。
      // 不用正则 —— 模板字符串里 \s 会被吞成 s，那正是本仓 ADR-0019 那类静默失效。
      const branchStart = src.indexOf(anchor);
      const tail = src.indexOf('\n}\n\n/**', branchStart);
      expect(tail).toBeGreaterThan(branchStart);
      const broken = src.slice(0, branchStart)
        + "\t} else if (event == 'done') {\n\t\tif (callbacks.onDone != null) {\n\t\t\tcallbacks.onDone(null)\n\t\t}\n\t}\n}"
        + src.slice(tail + 2);
      expect(broken).not.toBe(src); // 变异必须真落上
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-done-content-mut-'));
      const file = path.join(dir, 'aiAssistant.uts');
      fs.writeFileSync(file, broken);
      const { chat } = loadPair({ apiFile: file, sse: sseOf(['增量A'], { content: '权威全文' }) });
      const text = assistantText(await sendOne(chat));
      // 坏实现：权威全文没被替换。**只钉这一件事** —— 自 #2298 起，「没解析出 done 载荷」
      // 同时意味着收尾帧没被识别，通道会再补一条「回答未完整返回」的收尾文案（这是正确行为），
      // 所以这里不能用整串相等，否则两条各自正确的守护会互相判红。
      expect(text.startsWith('增量A')).toBe(true);
      expect(text).not.toContain('权威全文');
    });
  });
});
