/**
 * #2361 · 诊断能力徽章与耗时 —— **行为级**守护（端到端真执行）
 *
 * ## 它在守什么
 *
 * 票面验收五条里，有四条是「看不见的东西**真的**不出现」：
 * ①  badges 空集不渲染空行、elapsed 为 0 / 缺席不渲染耗时脚注；
 * ②  当轮与回看**逐项一致**（同字段、同渲染件、同归一函数）；
 * ③  meta 为 null（非诊断轮 / 存量消息）不出空壳；
 * ⑤  「meta → 渲染视图模型」的归一必须**真执行**后断言产物。
 *
 * ## 为什么必须真跑
 *
 * 读源码文本只能证明「有个 if 判了 null」，证明不了「三种『没有』态产出的视图模型
 * 完全一致」，也证明不了「SSE 当轮与 history 回看喂进同一个字段后渲染出同一份结果」。
 * 故本文件把三层都装成真件串起来跑：
 *
 *   `api/aiAssistant.uts` 的 `parseDiagnosisMeta`（两路共用的**唯一**解析器）
 *     → `utils/aiDiagnosisMeta.uts` 的 `metaViewOf`（**唯一**归一入口）
 *     → 断言视图模型（徽章文案数组 + 耗时文案 + visible）
 *
 * 端到端两段：SSE `done.meta` 走 `useAiChat.sendInput` 真跑到助手消息的 `meta`；
 * 回看走 `getAiSessionMessagesApi` 真跑到 `AiSessionMessage.meta`。两者再用**同一个**
 * `metaViewOf` 归一，断言逐项相等 —— 这就是票面验收②「同源」的机械判据。
 *
 * ## 变异取证
 *
 * 末组把 `metaViewOf` 的 `visible` 判据改成恒 `true`（空集也出块），证明上面
 * 「不出空壳」判据有牙。仓内纪律：不展示变异，判据等于没验过。
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
const META_UTS = path.join(ROOT, 'utils', 'aiDiagnosisMeta.uts');
const QUOTA_UTS = path.join(ROOT, 'utils', 'aiImageQuota.uts');

/** 一条徽章（三字段与后端 `DiagnosisBadge` DTO 逐字同形，无 omitempty） */
function badge(name, emoji, cls) {
  return { name: name, emoji: emoji, cls: cls };
}

/** `data: <json>` 的 SSE 帧；doneData 为 null 时写 `null`（非诊断轮的真实形态） */
function sseOf(chunks, doneData) {
  let s = '';
  for (const c of chunks) s += 'event: message\ndata: ' + JSON.stringify({ content: c }) + '\n\n';
  s += 'event: done\ndata: ' + (doneData === null ? 'null' : JSON.stringify(doneData)) + '\n\n';
  return s;
}

/**
 * 装载纯函数模块（零运行期依赖 ⇒ 无需任何绑定）。
 */
function loadMeta() {
  return loadUts(META_UTS, {});
}

/**
 * 装载 api + chat 两个真源并接线。
 * - `getMapped` 替身对**会话消息**返回 `opt.messages`（回看路径），其余返回空对象；
 * - `uni.request` 替身立即回包 `opt.sse`（当轮路径）；
 * - chat 侧的 `import { ref, type Ref } from 'vue'` 在临时副本里拆成两行归一化（同 #2310 先例）。
 */
function loadPair(options) {
  const opt = options || {};
  const meta = loadMeta();
  const h = loadUts(HELPERS_UTS, {});
  const quota = loadUts(QUOTA_UTS, {});
  const envelope = loadUts(ENVELOPE_UTS, {
    getMapped: () => Promise.resolve({}),
    postMapped: () => Promise.resolve({}),
    toNumber: h.toNumber,
    mergeQueryParams: () => '',
  });
  const responseText = opt.sse == null ? sseOf(['增量A'], null) : opt.sse;
  const uni = {
    request: (o) => { o.success({ statusCode: 200, data: responseText }); return {}; },
    showToast: () => {},
    showModal: () => {},
  };
  const api = loadUts(API_UTS, {
    // 回看路径：`getAiSessionMessagesApi` → `getMapped(url, null, map)` ⇒ 这里同步调 map
    getMapped: (url, params, map) => Promise.resolve(map({ messages: opt.messages == null ? [] : opt.messages })),
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
    uni: uni,
    console: console,
  });

  let chatSrc = readText(CHAT_UTS);
  const originalImport = "import { ref, type Ref } from 'vue'";
  expect(chatSrc.includes(originalImport)).toBe(true); // 归一化锚点必须真在源里
  chatSrc = chatSrc.replace(originalImport, "import { ref } from 'vue'\nimport type { Ref } from 'vue'");
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-meta-'));
  const chatFile = path.join(dir, 'useAiChat.uts');
  fs.writeFileSync(chatFile, chatSrc);

  const chat = loadUts(chatFile, {
    ref: (v) => ({ value: v }),
    getAiSessionsApi: () => Promise.resolve([]),
    createAiSessionApi: () => Promise.resolve({ id: 1, title: '' }),
    deleteAiSessionApi: () => Promise.resolve(),
    getAiSessionMessagesApi: () => api.getAiSessionMessagesApi(1),
    streamChatApi: api.streamChatApi,
    parseDiagnosisMeta: api.parseDiagnosisMeta,
    toStr: h.toStr,
    canSend: quota.canSend,
    uni: uni,
    setTimeout: () => {},
    console: console,
  });
  return { api: api, chat: chat, meta: meta, h: h };
}

/** 发一条消息并 flush 微任务，返回 useAiChat 的状态对象 */
async function sendOne(chat) {
  const state = chat.useAiChat({ value: '' });
  state.inputText.value = '这是一条用户问题';
  state.sendInput('标签', () => ({}), [], '');
  for (let i = 0; i < 4; i++) await Promise.resolve();
  return state;
}

function assistantOf(state) {
  const list = state.messages.value.filter((m) => m.role === 'assistant');
  expect(list.length).toBe(1);
  return list[0];
}

describe('#2361 诊断能力徽章与耗时（行为级，端到端真执行）', () => {
  describe('解析器 parseDiagnosisMeta：两路共用的唯一入口', () => {
    test('badges + elapsed 齐全 ⇒ 三字段原样透传、不另造词表', () => {
      const { api } = loadPair({});
      const m = api.parseDiagnosisMeta({
        badges: [badge('知识检索', '📚', 'cap-rag'), badge('风险警示', '⚠️', 'cap-risk')],
        elapsed: 11.8,
      });
      expect(m).not.toBeNull();
      expect(m.badges.length).toBe(2);
      expect(m.badges[0]).toEqual({ name: '知识检索', emoji: '📚', cls: 'cap-rag' });
      expect(m.badges[1]).toEqual({ name: '风险警示', emoji: '⚠️', cls: 'cap-risk' });
      expect(m.elapsed).toBe(11.8);
    });

    test('当轮 omitempty 缺 badges 键 ⇒ 归一成空数组（不是 undefined、不是 null）', () => {
      const { api } = loadPair({});
      const m = api.parseDiagnosisMeta({ elapsed: 7.2 });
      expect(Array.isArray(m.badges)).toBe(true);
      expect(m.badges.length).toBe(0);
      expect(m.elapsed).toBe(7.2);
    });

    test('缺 elapsed 键 ⇒ 归一成 0', () => {
      const { api } = loadPair({});
      const m = api.parseDiagnosisMeta({ badges: [badge('知识检索', '📚', 'cap-rag')] });
      expect(m.elapsed).toBe(0);
      expect(m.badges.length).toBe(1);
    });

    test('meta 为 null（非诊断轮 / 存量消息）⇒ 返回 null，不造空壳对象', () => {
      const { api } = loadPair({});
      expect(api.parseDiagnosisMeta(null)).toBeNull();
    });

    test('未知 cls 与缺字段的徽章照样透传（不按 cls 过滤、不改写）', () => {
      const { api } = loadPair({});
      const m = api.parseDiagnosisMeta({ badges: [badge('未知能力', '🔧', 'cap-not-in-any-table')] });
      expect(m.badges[0].cls).toBe('cap-not-in-any-table');
      expect(m.badges[0].name).toBe('未知能力');
    });
  });

  describe('归一入口 metaViewOf：三种「没有」态产出的视图模型必须完全一致', () => {
    test('null / 空 badges / elapsed 为 0 ⇒ 三个视图逐字段相同且 visible=false', () => {
      const { meta } = loadPair({});
      const a = meta.metaViewOf(null);
      const b = meta.metaViewOf({ badges: [], elapsed: 0 });
      const c = meta.metaViewOf({});
      expect(a).toEqual(b);
      expect(a).toEqual(c);
      expect(a.visible).toBe(false);
      expect(a.badges.length).toBe(0);
      expect(a.elapsedText).toBe('');
    });

    test('只有耗时没有徽章 ⇒ visible=true、徽章区为空、耗时文案就位（票面：不出空行）', () => {
      const { meta } = loadPair({});
      const v = meta.metaViewOf({ badges: [], elapsed: 11.8 });
      expect(v.visible).toBe(true);
      expect(v.badges.length).toBe(0);
      expect(v.elapsedText).toBe('耗时 11.8 秒');
    });

    test('只有徽章没有耗时 ⇒ visible=true、耗时文案为空串（票面：不出耗时脚注）', () => {
      const { meta } = loadPair({});
      const v = meta.metaViewOf({ badges: [badge('知识检索', '📚', 'cap-rag')], elapsed: 0 });
      expect(v.visible).toBe(true);
      expect(v.elapsedText).toBe('');
      expect(v.badges.length).toBe(1);
    });

    test('徽章文案装配：emoji + 空格 + 名称；缺一侧不留前导/尾随空格', () => {
      const { meta } = loadPair({});
      expect(meta.badgeLabelOf(badge('知识检索', '📚', 'cap-rag'))).toBe('📚 知识检索');
      expect(meta.badgeLabelOf(badge('知识检索', '', 'cap-rag'))).toBe('知识检索');
      expect(meta.badgeLabelOf(badge('', '📚', 'cap-rag'))).toBe('📚');
    });

    test('名称与图标都空的残条被丢弃（后端回放能出现空串）', () => {
      const { meta } = loadPair({});
      const v = meta.metaViewOf({
        badges: [badge('', '', 'cap-empty'), badge('知识检索', '📚', 'cap-rag')],
        elapsed: 0,
      });
      expect(v.badges.length).toBe(1);
      expect(v.badges[0].name).toBe('知识检索');
    });

    test('耗时一位小数（与 web 同口径，避免同一条回答两端读成 11.8 / 12）', () => {
      const { meta } = loadPair({});
      expect(meta.elapsedLabelOf(11.84)).toBe('耗时 11.8 秒');
      expect(meta.elapsedLabelOf(0)).toBe('');
      expect(meta.elapsedLabelOf(-1)).toBe('');
    });
  });

  describe('当轮：done.meta 落到同一条助手消息的 meta 字段', () => {
    test('诊断轮 done 带 meta ⇒ 助手消息拿到徽章与耗时', async () => {
      const sse = sseOf(['增量A'], {
        content: '权威全文',
        meta: { badges: [badge('知识检索', '📚', 'cap-rag'), badge('风险警示', '⚠️', 'cap-risk')], elapsed: 11.8 },
      });
      const { chat } = loadPair({ sse: sse });
      const state = await sendOne(chat);
      const msg = assistantOf(state);
      expect(msg.content).toBe('权威全文');
      expect(msg.meta).toBeDefined();
      expect(msg.meta.badges.length).toBe(2);
      expect(msg.meta.elapsed).toBe(11.8);
    });

    test('非诊断轮 done 为 null ⇒ 助手消息没有 meta 字段（不写 null，票面③不出空壳）', async () => {
      const { chat } = loadPair({ sse: sseOf(['增量A'], null) });
      const state = await sendOne(chat);
      const msg = assistantOf(state);
      expect(msg.meta).toBeUndefined();
    });
  });

  describe('回看：messages[].meta 落到同一条字段，并与当轮逐项一致（票面验收②）', () => {
    test('回看消息的 meta 被解析到 AiSessionMessage.meta，且归一结果与当轮完全相同', async () => {
      const badges = [badge('知识检索', '📚', 'cap-rag'), badge('风险警示', '⚠️', 'cap-risk')];
      const sse = sseOf(['增量A'], { content: '权威全文', meta: { badges: badges, elapsed: 11.8 } });
      const { api, chat, meta } = loadPair({
        sse: sse,
        messages: [
          { id: 1, role: 'user', content: '问题', created_at: '2026-11-09 10:00:00' },
          {
            id: 2,
            role: 'assistant',
            content: '权威全文',
            created_at: '2026-11-09 10:00:10',
            meta: { badges: badges, elapsed: 11.8 },
          },
        ],
      });

      // 当轮
      const state = await sendOne(chat);
      const live = meta.metaViewOf(assistantOf(state).meta);

      // 回看
      const rows = await api.getAiSessionMessagesApi(1);
      expect(rows.length).toBe(2);
      const back = rows[1];
      expect(back.role).toBe('assistant');
      const replay = meta.metaViewOf(back.meta);

      expect(replay).toEqual(live);
      expect(replay.visible).toBe(true);
      expect(replay.elapsedText).toBe('耗时 11.8 秒');
      expect(replay.badges.length).toBe(2);
    });

    test('回看的 meta 为 null（非诊断轮 / 存量消息）⇒ 字段不出现，归一为不显示', async () => {
      const { api, meta } = loadPair({
        messages: [{ id: 1, role: 'assistant', content: '旧回答', created_at: '2026-11-09 09:00:00', meta: null }],
      });
      const rows = await api.getAiSessionMessagesApi(1);
      expect(rows.length).toBe(1);
      expect(rows[0].meta).toBeUndefined();
      expect(meta.metaViewOf(rows[0].meta).visible).toBe(false);
    });
  });

  describe('检测器有牙：注入变异的副本，证明上面判据真能判红', () => {
    test('把 metaViewOf 的 visible 改成恒 true ⇒「空态不出空壳」用例当场红', () => {
      const src = readText(META_UTS);
      const anchor = 'const visible = badges.length > 0 || elapsedText.length > 0';
      expect(src.includes(anchor)).toBe(true); // 锚点必须真在源里
      const mutated = src.replace(anchor, 'const visible = true');
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-meta-mut-'));
      const file = path.join(dir, 'aiDiagnosisMeta.uts');
      fs.writeFileSync(file, mutated);
      const bad = loadUts(file, {});
      // 变异只改变 visible 这一行 ⇒ 用**非 null 但两个字段都为空**的输入才走到它
      // （null 走 emptyMetaView 的早退，不经过该行，拿它验不出牙）。
      const emptyish = { badges: [], elapsed: 0 };
      expect(loadMeta().metaViewOf(emptyish).visible).toBe(false);
      expect(bad.metaViewOf(emptyish).visible).toBe(true);
      // 顺带锁住变异件仍然遵守「null 早退」：证明变异范围精确、判据不是靠副作用蒙对
      expect(bad.metaViewOf(null).visible).toBe(false);
    });
  });
});
