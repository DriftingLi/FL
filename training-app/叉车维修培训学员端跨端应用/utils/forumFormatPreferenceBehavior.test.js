/**
 * 作者格式声明的**记忆位** · **行为级**测试（根 ADR-0044「首次默认纯文本、之后记住上次选择」；#1240 P3）
 *
 * 要机检的是 ADR-0044 那条决策的两个半边，各一条判据：
 *   ① **首次默认纯文本**：存储里没有该键 / 存了脏值 ⇒ 读出来是 `text`（Markdown 完全 opt-in）；
 *   ② **记住上次选择**：`writeAuthorFormat('markdown')` 之后 `readAuthorFormat()` 必须还是 `markdown`
 *      —— 且**发帖与回复共用同一个键**（写入后键集合大小恒为 1）。
 *
 * 为什么是行为而不是源码文本：「读出来的到底是什么」不是字面量断言能证明的。
 * 缝：`utils/utsHarness.js` 的 `loadUts`；**注入的是真执行的 `utils/storage.uts`**（只把它的 `uni`
 * 换成一份内存替身）—— 于是「读写是否真的经过存储层」也在判据里（③ 判据②「测的是该测的那一支」）。
 *
 * **成对取证**（③ 判据③）：末组用注入变异的副本证明判据**有牙** —— 三种真实坏实现：
 *   ① 不读存储（永远返回纯文本）；② 归一放宽（脏值被当成合法档位读出来）；③ 写入不归一（脏值落存储）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, importedNames, readText } = require('./utsHarness');

const PREF_UTS = path.join(__dirname, 'forumFormatPreference.uts');
const STORAGE_UTS = path.join(__dirname, 'storage.uts');
const BODY_UTS = path.join(__dirname, 'forumBody.uts');
const MD_UTS = path.join(__dirname, 'markdown.uts');

/** `utils/storage.uts` 引用的全局 `uni` —— 这里换成一份内存替身（键值都留在测试里，不碰真设备） */
function fakeUni(store) {
  return {
    setStorageSync: (k, v) => { store.set(k, v); },
    getStorageSync: (k) => (store.has(k) ? store.get(k) : ''),
    removeStorageSync: (k) => { store.delete(k); },
  };
}

/** 真源依赖注入表：存储层取**真执行的** `utils/storage.uts`，格式词汇取真执行的 `utils/forumBody.uts` */
function preferenceBindings(store) {
  const storage = loadUts(STORAGE_UTS, { uni: fakeUni(store) });
  const markdown = loadUts(MD_UTS, {});
  const body = loadUts(BODY_UTS, {
    parseMarkdown: markdown.parseMarkdown,
    SUBSET_FORUM: markdown.SUBSET_FORUM,
    SUBSET_CHAPTER: markdown.SUBSET_CHAPTER,
  });
  return {
    getStorage: storage.getStorage,
    setStorage: storage.setStorage,
    FORMAT_TEXT: body.FORMAT_TEXT,
    FORMAT_MARKDOWN: body.FORMAT_MARKDOWN,
  };
}

/** 每次取一个**全新模块实例**（连同全新存储） */
function pref(store = new Map()) {
  return { p: loadUts(PREF_UTS, preferenceBindings(store)), store };
}

const KEY = 'forum_author_content_format';

// ===== ① 首次默认纯文本 =====

describe('首次默认纯文本：读不到 / 读到脏值都落 `text`（Markdown 完全 opt-in）', () => {
  it('存储里没有该键 ⇒ `text`（「首次默认」不是页面里的初值，而是这条判据本身）', () => {
    const { p, store } = pref();
    expect(store.size).toBe(0);
    expect(p.readAuthorFormat()).toBe('text');
  });

  it('存了脏值（大小写不符 / 空串 / 契约外新值）⇒ 一律 `text`，与 DTO 层归一同向', () => {
    for (const dirty of ['MD', 'Markdown', 'md', 'html', '', ' markdown ']) {
      const store = new Map([[KEY, dirty]]);
      const { p } = pref(store);
      expect([dirty, p.readAuthorFormat()]).toEqual([dirty, 'text']);
    }
  });

  it('作者侧归一函数本身就是那个判据（`markdown` 之外一律 `text`）', () => {
    const { p } = pref();
    expect(p.normalizeAuthorFormat('markdown')).toBe('markdown');
    expect(p.normalizeAuthorFormat('text')).toBe('text');
    expect(p.normalizeAuthorFormat('')).toBe('text');
    expect(p.normalizeAuthorFormat('MD')).toBe('text');
  });
});

// ===== ② 记住上次选择（且发帖与回复共用一个记忆位）=====

describe('记住上次选择：写进去的档位，下一次读出来还是它', () => {
  it('`markdown` 往返成立，且真的经过存储层（键与值都对得上）', () => {
    const { p, store } = pref();
    p.writeAuthorFormat('markdown');
    expect(store.get(KEY)).toBe('markdown');
    expect(p.readAuthorFormat()).toBe('markdown');
  });

  it('`text` 也往返成立（选回纯文本同样是一次「上次选择」，不能被当成「没选过」）', () => {
    const store = new Map([[KEY, 'markdown']]);
    const { p } = pref(store);
    p.writeAuthorFormat('text');
    expect(store.get(KEY)).toBe('text');
    expect(p.readAuthorFormat()).toBe('text');
  });

  it('**发帖与回复共用一个记忆位**：一次写入只碰一个键（写两次键集合大小仍为 1）', () => {
    const { p, store } = pref();
    p.writeAuthorFormat('markdown');   // 例如发帖时选的那次
    expect(store.size).toBe(1);
    p.writeAuthorFormat('markdown');   // 例如回复时又确认了一次
    expect(store.size).toBe(1);
    expect([...store.keys()]).toEqual([KEY]);
  });

  it('写入前也归一：契约外的取值不许落进存储（否则脏值会永久留在设备上）', () => {
    const { p, store } = pref();
    p.writeAuthorFormat('html');
    expect(store.get(KEY)).toBe('text');
    p.writeAuthorFormat('MD');
    expect(store.get(KEY)).toBe('text');
  });
});

// ===== ③ 成对取证：注入变异后，上面的判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function loadMutated(replacements, store = new Map()) {
  let src = readText(PREF_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-format-pref-'));
  const file = path.join(dir, 'forumFormatPreference.uts');
  fs.writeFileSync(file, src);
  return loadUts(file, preferenceBindings(store));
}

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红（对照）：真源上 markdown 往返成立、脏值落 text', () => {
    const { p } = pref();
    p.writeAuthorFormat('markdown');
    expect(p.readAuthorFormat()).toBe('markdown');
    expect(p.normalizeAuthorFormat('MD')).toBe('text');
  });

  it('必红 · **不读存储**（永远返回纯文本）⇒ ②「往返成立」判据必然判红', () => {
    const broken = loadMutated([
      ['return normalizeAuthorFormat(getStorage(STORAGE_KEY_FORUM_AUTHOR_FORMAT))', 'return FORMAT_TEXT'],
    ]);
    broken.writeAuthorFormat('markdown');   // 写进去了
    expect(broken.readAuthorFormat()).toBe('text'); // 但读不回来 ⇒ ② 的往返断言判红
  });

  it('必红 · **归一放宽**（脏值被当成合法档位读出来）⇒ ①「脏值落 text」判据必然判红', () => {
    const store = new Map([[KEY, 'MD']]);
    const broken = loadMutated(
      [['return value == FORMAT_MARKDOWN ? FORMAT_MARKDOWN : FORMAT_TEXT', 'return value.length > 0 ? value : FORMAT_TEXT']],
      store
    );
    expect(broken.readAuthorFormat()).toBe('MD'); // 脏值漏出来了 ⇒ ① 的逐值断言判红
  });

  it('必红 · **写入不归一**（脏值落存储）⇒ ②「写入前也归一」判据必然判红', () => {
    const store = new Map();
    const broken = loadMutated(
      [['setStorage(STORAGE_KEY_FORUM_AUTHOR_FORMAT, normalizeAuthorFormat(format))', 'setStorage(STORAGE_KEY_FORUM_AUTHOR_FORMAT, format)']],
      store
    );
    broken.writeAuthorFormat('html');
    expect(store.get(KEY)).toBe('html'); // 契约外的值真的落了存储 ⇒ 该断言判红
  });
});

// ===== ④ 模块契约：键单点、归一不另起一份 =====

describe('模块契约：存储键与格式词汇都只有一处', () => {
  it('运行期 import 恰为存储读写 + 两个格式常量（不自持第二份归一 / 不直连 `uni`）', () => {
    expect(importedNames(readText(PREF_UTS)).sort()).toEqual(
      ['FORMAT_MARKDOWN', 'FORMAT_TEXT', 'getStorage', 'setStorage'].sort()
    );
  });

  it('存储键是常量单点：字面量只出现一次（散落两处则「共用一个记忆位」名存实亡）', () => {
    const src = readText(PREF_UTS);
    expect((src.match(/forum_author_content_format/g) || []).length).toBe(1);
  });
});
