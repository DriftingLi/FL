/**
 * #1732 · AI 对话设置的**持久面 + 三 source 归一化** —— 行为级守护
 *
 * ## 它在守什么
 *
 * 本票把两件东西从页面/ api 层收进 `composables/useAiModelSelection.uts`：
 *   ① 本地存储读写（`loadAiChatSettings` / `saveAiChatSettings` / `SETTINGS_KEY`）——
 *      api 层只该做 HTTP 与 DTO 映射，不该有存储 IO（#1732 验收①）。
 *   ② **三 source 排他规则**（admin / user / custom 的字段回填与清空）——
 *      从前 `ai-assistant.uvue` 的 `persistSettings` 与 `ai-settings.uvue` 的
 *      `onSave` / `reclaimLockedModelSource` 各写一份，失一处即**两页不对称且无守护**。
 *
 * 源码文本契约证明不了「落盘的那条规则真的清空了 api_key」——本文件把模块**真跑起来**
 * （`utsHarness.loadUts` + 内存存储替身真的落在 key `ai_chat_settings` 上），断言只有跑起来才看得见的语义：
 *
 *   1. **持久面往返**：`saveAiChatSettings` → `loadAiChatSettings` 字段逐字保留；
 *      缺键 / 坏 JSON 一律回落默认值（fail-closed 到可用默认，不抛给页面）。
 *   2. **归一化排他**：user 与 admin **恒清 api_key**（明文 key 只属于 custom）；
 *      admin 与 user 都清 base_url；custom 保留表单三项值；展示名按 name → model → previous 兜底。
 *   3. **回收形态**：`reclaimLockedAiChatSettings` = 平台模型 + 第一个平台模型（有则选），
 *      model / base_url / api_key 全清 —— 两页只决定「何时调」。
 *   4. **M1 必红变异**：把 user 分支的 `s.api_key = ''` 删掉 → 第 2 条当场红
 *      （证明这条断言真能判红，不是恒绿）。
 *
 * fail-closed 由 utsHarness 兜底：模块若新增/减少 import，缺绑定当场红。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const SELECTION_UTS = path.join(ROOT, 'composables', 'useAiModelSelection.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');
const STORAGE_UTS = path.join(ROOT, 'utils', 'storage.uts');
const API_UTS = path.join(ROOT, 'api', 'aiAssistant.uts');

/** 真源：helpers 是零依赖纯函数（手抄 toStr/toNumber 正是漂移起点） */
const helpers = () => loadUts(HELPERS_UTS, {});

/**
 * 内存存储替身：与 `utils/storage.uts` 的真实现同形（uni.getStorageSync 缺键回 ''，
 * setStorageSync 落 JSON 字符串），断言才能落在「真的写了一行 key=value」上。
 */
function memoryStorage() {
  const box = {};
  return {
    box,
    getStorage: (k) => (k in box ? box[k] : ''),
    setStorage: (k, v) => { box[k] = v; },
  };
}

/** 装载 useAiModelSelection.uts（imports：三个 type-only + storage 两函数 + helpers 两函数） */
function loadSelection(file) {
  const h = helpers();
  const mem = memoryStorage();
  const mod = loadUts(file || SELECTION_UTS, {
    getStorage: mem.getStorage,
    setStorage: mem.setStorage,
    toNumber: h.toNumber,
    toStr: h.toStr,
  });
  return { mod, mem };
}

const PLATFORM = [
  { id: 1, name: '基座模型', model: 'j/base-1', source: 'admin', description: '' },
  { id: 2, name: '进阶模型', model: 'j/base-2', source: 'admin', description: '' },
];
const SAVED = [
  { id: 7, name: '我的模型', base_url: 'https://u.test/v1', model: 'u/one', api_key: 'sk-user', created_at: '' },
];
const customOf = (o = {}) => Object.assign({ name: '', model: '', base_url: '', api_key: '' }, o);

/**
 * 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树，不进仓）；
 * 锚点断言先行：替换串不在源里就当场红 —— 变异必须真的进去了，测试才有意义。
 */
function mutateUts(file, replacements, outName) {
  let src = readText(file);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]);
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-model-selection-'));
  const out = path.join(dir, outName);
  fs.writeFileSync(out, src);
  return out;
}

describe('持久面：键 ai_chat_settings 的往返与 fail-closed（#1732）', () => {
  it('save → load 字段逐字保留，且只写这一个键', () => {
    const { mod, mem } = loadSelection();
    mod.saveAiChatSettings({
      model_source: 'custom',
      model_id: 0,
      model_name: '我的',
      base_url: 'https://c.test/v1',
      model: 'c/one',
      api_key: 'sk-custom',
    });
    expect(Object.keys(mem.box)).toEqual(['ai_chat_settings']);
    expect(JSON.parse(mem.box['ai_chat_settings'])).toEqual({
      model_source: 'custom',
      model_id: 0,
      model_name: '我的',
      base_url: 'https://c.test/v1',
      model: 'c/one',
      api_key: 'sk-custom',
    });
    expect(mod.loadAiChatSettings()).toEqual({
      model_source: 'custom',
      model_id: 0,
      model_name: '我的',
      base_url: 'https://c.test/v1',
      model: 'c/one',
      api_key: 'sk-custom',
    });
  });

  it('缺键 / 坏 JSON / 空字符串 一律回落默认值，不抛给页面', () => {
    const { mod, mem } = loadSelection();
    expect(mod.loadAiChatSettings()).toEqual(mod.defaultAiChatSettings());
    mem.box['ai_chat_settings'] = '{ 坏掉的 json';
    expect(mod.loadAiChatSettings()).toEqual(mod.defaultAiChatSettings());
    mem.box['ai_chat_settings'] = '';
    expect(mod.loadAiChatSettings()).toEqual(mod.defaultAiChatSettings());
  });

  it('读到的字段缺失时逐字段兜底（旧版本落盘的数据仍可用）', () => {
    const { mod, mem } = loadSelection();
    mem.box['ai_chat_settings'] = JSON.stringify({ model_id: '9' });
    expect(mod.loadAiChatSettings()).toEqual({
      model_source: 'admin',
      model_id: 9,
      model_name: '默认模型',
      base_url: '',
      model: '',
      api_key: '',
    });
  });
});

describe('归一化：三 source 排他规则（单源，两页共用）', () => {
  it('custom：表单三项值原样落盘，展示名按 name → model → previous 依次兜底', () => {
    const { mod } = loadSelection();
    const prev = mod.defaultAiChatSettings();
    const withName = mod.normalizeAiChatSettings('custom', 0, PLATFORM, SAVED, customOf({ name: '我的', model: 'c/one', base_url: 'https://c.test/v1', api_key: 'sk-c' }), prev);
    expect(withName).toEqual({ model_source: 'custom', model_id: 0, model_name: '我的', model: 'c/one', base_url: 'https://c.test/v1', api_key: 'sk-c' });
    const noName = mod.normalizeAiChatSettings('custom', 0, PLATFORM, SAVED, customOf({ model: 'c/two', base_url: 'https://c.test/v1', api_key: 'sk-c' }), prev);
    expect(noName.model_name).toBe('c/two');
    const none = mod.normalizeAiChatSettings('custom', 0, PLATFORM, SAVED, customOf({ base_url: 'https://c.test/v1', api_key: 'sk-c' }), prev);
    expect(none.model_name).toBe('默认模型');
  });

  it('user：回填已保存模型的 name/model/base_url，但 api_key 恒清空（本地不落明文 key）', () => {
    const { mod } = loadSelection();
    const prev = mod.defaultAiChatSettings();
    prev.api_key = 'sk-left-over';
    const s = mod.normalizeAiChatSettings('user', 7, PLATFORM, SAVED, customOf(), prev);
    expect(s).toEqual({ model_source: 'user', model_id: 7, model_name: '我的模型', model: 'u/one', base_url: 'https://u.test/v1', api_key: '' });
  });

  it('admin：回填平台模型名，base_url 与 api_key 双双清空（平台模型不带自带地址与 key）', () => {
    const { mod } = loadSelection();
    const prev = mod.defaultAiChatSettings();
    prev.base_url = 'https://stale.test/v1';
    prev.api_key = 'sk-stale';
    const s = mod.normalizeAiChatSettings('admin', 2, PLATFORM, SAVED, customOf(), prev);
    expect(s).toEqual({ model_source: 'admin', model_id: 2, model_name: '进阶模型', model: 'j/base-2', base_url: '', api_key: '' });
  });

  it('未命中的 id 沿用 previous 的展示字段，来源仍按入参收敛（不新造第四种来源）', () => {
    const { mod } = loadSelection();
    const prev = mod.defaultAiChatSettings();
    prev.model_name = '上一次的';
    prev.model = 'j/prev';
    const s = mod.normalizeAiChatSettings('weird', 99, PLATFORM, SAVED, customOf(), prev);
    expect(s.model_source).toBe('admin');
    expect(s.model_name).toBe('上一次的');
    expect(s.model).toBe('j/prev');
    expect(s.base_url).toBe('');
    expect(s.api_key).toBe('');
  });

  it('reclaim：回收为「平台模型 + 第一个平台模型」，三项自带地址与 key 全清', () => {
    const { mod } = loadSelection();
    expect(mod.reclaimLockedAiChatSettings(PLATFORM)).toEqual({
      model_source: 'admin',
      model_id: 1,
      model_name: '基座模型',
      model: 'j/base-1',
      base_url: '',
      api_key: '',
    });
    // 平台模型列表为空时仍可回收（id 落 0、展示名回落默认模型）
    expect(mod.reclaimLockedAiChatSettings([])).toEqual({
      model_source: 'admin',
      model_id: 0,
      model_name: '默认模型',
      model: '',
      base_url: '',
      api_key: '',
    });
  });
});

describe('验收①：存储 IO 已迁出 api 层', () => {
  it('api/aiAssistant.uts 里 SETTINGS_KEY / ai_chat_settings 零命中', () => {
    const src = readText(API_UTS);
    expect(src.includes('SETTINGS_KEY')).toBe(false);
    expect(src.includes('ai_chat_settings')).toBe(false);
  });
});

describe('M1 必红变异：user 分支不再清 api_key → 第 2 条当场红', () => {
  it('变异体把存量 key 带进 user 来源（证明「恒清空」这条断言真能判红）', () => {
    const from = '\t\t// 明文 key 只属于 custom 来源：user 走服务端 user_model_id 鉴权，本地不落 key\n\t\ts.api_key = \'\'';
    const file = mutateUts(SELECTION_UTS, [[from, '']], 'useAiModelSelection.uts');
    const { mod } = loadSelection(file);
    const prev = mod.defaultAiChatSettings();
    prev.api_key = 'sk-left-over';
    const s = mod.normalizeAiChatSettings('user', 7, PLATFORM, SAVED, customOf(), prev);
    expect(s.api_key).toBe('sk-left-over');
  });
});
