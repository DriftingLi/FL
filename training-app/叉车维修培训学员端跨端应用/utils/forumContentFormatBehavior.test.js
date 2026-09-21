/**
 * 论坛正文格式声明 `content_format` · **行为级**测试（ADR-0044 / ADR-0025；票 #1240 P1）
 *
 * 本票在移动端要买的两件事，一件一条判据：
 *   ① **消费**：DTO 层把格式字段归一后带上 —— **只有显式 `markdown` 是 markdown**，
 *      缺省 / 空串 / 大小写不符 / 未来新值一律落 `text`（与后端 `normalizeContentFormat` 同向，
 *      也照根 ADR-0044 的缺省语义：不带该字段的客户端天然落纯文本分支，不被语法擅自解释）。
 *   ② **编辑不变式**：`updateForumTopicApi` 的 PUT 载荷**不含** `content_format` —— 编辑是全量替换
 *      语义，带上它就会把 Markdown 帖静默重置成纯文本（根 ADR-0044 的原话）。
 *
 * 为什么是行为而不是源码文本：这两条都只有**跑起来**才看得见 —— 「键集合里没有它」与
 * 「归一结果是什么」都不是字面量断言能证明的。缝：`utils/utsHarness.js` 的 `loadUts`
 * 把 `.uts` 去类型后当 JS 真执行（先例 `utils/recruitDisplayBehavior.test.js`）。
 * 依赖（`api/helpers.uts` / `api/forumDto.uts` / request 出口）**注入真件或替身**：
 * helpers 与 DTO 都载真件，只有网络出口是替身 —— 被测的是载荷构造与归一，不是 HTTP。
 *
 * **成对取证**（③ 门判据③）：末组把 `content_format` 注入变异副本的 PUT 载荷，
 * 证明第 ② 条的判据**有牙**（坏实现一定判红，而不是「恰好没写」）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const API_DIR = path.join(__dirname, '..', 'api');
const HELPERS_UTS = path.join(API_DIR, 'helpers.uts');
const DTO_UTS = path.join(API_DIR, 'forumDto.uts');
const FORUM_UTS = path.join(API_DIR, 'forum.uts');
const TYPES_FORUM = path.join(__dirname, '..', 'types', 'forum.uts');

/** 真件：helpers（零 import，空绑定即可）与 DTO 构造层（依赖 helpers，注入真件） */
const helpers = () => loadUts(HELPERS_UTS, {});
const dto = () => loadUts(DTO_UTS, { ...helpers() });

/** request 出口的替身：记下**实际被调用的形态**，不真发网络 */
function makeRequestMocks() {
  const captured = [];
  const ok = () => Promise.resolve({});
  return {
    captured,
    post: (url, data) => { captured.push({ via: 'post', url, data }); return ok(); },
    del: (url) => { captured.push({ via: 'del', url }); return ok(); },
    uploadFile: (url, filePath) => { captured.push({ via: 'uploadFile', url, filePath }); return Promise.resolve(''); },
    getMapped: (url, mapper) => { captured.push({ via: 'getMapped', url }); return ok(); },
    postMapped: (url, data, mapper) => { captured.push({ via: 'postMapped', url, data }); return ok(); },
    requestMapped: (options, mapper) => { captured.push({ via: 'requestMapped', options }); return ok(); },
  };
}

/** 载入真件 `api/forum.uts`（或变异副本），回读「载荷构造」这一层 */
function loadForumApi(file = FORUM_UTS) {
  const mocks = makeRequestMocks();
  const api = loadUts(file, { ...mocks, ...helpers(), ...dto() });
  return { api, captured: mocks.captured };
}

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树，不进仓） */
function loadMutatedForumApi(replacements) {
  let src = readText(FORUM_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-format-'));
  const file = path.join(dir, 'forum.uts');
  fs.writeFileSync(file, src);
  return loadForumApi(file);
}

const payloadOf = (captured) => {
  const call = captured.find((c) => c.via === 'requestMapped');
  expect(call).toBeDefined();
  return call.options;
};

// ===== ① 消费：DTO 归一 =====

describe('DTO 归一：只有显式 markdown 是 markdown，其余一律 text（真执行 api/forumDto.uts）', () => {
  it('归一判据（与后端 normalizeContentFormat 同向，含两侧空白 TrimSpace）', () => {
    const d = dto();
    expect(d.normalizeContentFormat('markdown')).toBe('markdown');
    expect(d.normalizeContentFormat('  markdown  ')).toBe('markdown');
    expect(d.normalizeContentFormat('text')).toBe('text');
    expect(d.normalizeContentFormat('')).toBe('text');
    expect(d.normalizeContentFormat(null)).toBe('text');
    expect(d.normalizeContentFormat('MD')).toBe('text');      // 大小写不宽容
    expect(d.normalizeContentFormat('html')).toBe('text');    // 契约外的取值不许漏进来
  });

  it('三个 builder 都带上归一后的字段（缺省 = text）', () => {
    const d = dto();
    expect(d.buildTopic({ id: 1, title: 'T', content: 'C', content_format: 'markdown' }).content_format).toBe('markdown');
    expect(d.buildTopic({ id: 1, title: 'T', content: 'C' }).content_format).toBe('text');
    expect(d.buildReply({ id: 2, content: 'C', content_format: 'markdown' }).content_format).toBe('markdown');
    expect(d.buildReply({ id: 2, content: 'C' }).content_format).toBe('text');
    expect(d.buildTopicDetail({ topic: { id: 3, title: 'T', content: 'C', content_format: 'markdown' }, replies: [] }).content_format).toBe('markdown');
    expect(d.buildTopicDetail({ topic: { id: 3, title: 'T', content: 'C' }, replies: [] }).content_format).toBe('text');
  });
});

// ===== ② 类型层（接线）：三个 DTO 都声明了字段 =====

describe('类型层（接线守护）：三个论坛 DTO 各声明一次 content_format', () => {
  it('ForumTopic / ForumReply / ForumTopicDetail 都声明；行为面由上一组承重', () => {
    const src = readText(TYPES_FORUM);
    expect((src.match(/content_format : string/g) || []).length).toBe(3);
    for (const name of ['ForumTopic', 'ForumReply', 'ForumTopicDetail']) {
      const start = src.indexOf(`export type ${name} = {`);
      expect([name, start > -1]).toEqual([name, true]);
      const body = src.slice(start, src.indexOf('\n}', start));
      expect([name, body.includes('content_format : string')]).toEqual([name, true]);
    }
  });
});

// ===== ③ 编辑不变式：PUT 载荷不含格式字段 =====

describe('编辑不变式：`updateForumTopicApi` 的 PUT 载荷不含 content_format（真执行 api/forum.uts）', () => {
  it('载荷键集合 = title/content/images/category（category 非空时）', async () => {
    const { api, captured } = loadForumApi();
    await api.updateForumTopicApi(7, '标题', '正文', [], 'discussion');
    const options = payloadOf(captured);
    expect(options.method).toBe('PUT');
    expect(options.url).toBe('/forum/topics/7');
    expect(Object.keys(options.data).sort()).toEqual(['category', 'content', 'images', 'title']);
    expect('content_format' in options.data).toBe(false);
  });

  it('category 为空时条件携带语义不变（键集合相应少一个）', async () => {
    const { api, captured } = loadForumApi();
    await api.updateForumTopicApi(7, '标题', '正文');
    expect(Object.keys(payloadOf(captured).data).sort()).toEqual(['content', 'images', 'title']);
  });

  it('内容本身照旧带上（不是把整个载荷阉掉换来的干净）', async () => {
    const { api, captured } = loadForumApi();
    await api.updateForumTopicApi(7, '标题', '正文', ['https://e.com/a.png'], 'question');
    const data = payloadOf(captured).data;
    expect([data.title, data.content, data.images, data.category])
      .toEqual(['标题', '正文', ['https://e.com/a.png'], 'question']);
  });
});

// ===== ④ 成对取证：注入变异后，第 ③ 组必须判红 =====

describe('成对取证（必红）：把 content_format 塞进编辑载荷后，编辑不变式的判据必须判红', () => {
  it('必不红（对照）：真源载荷里没有该键', async () => {
    const { api, captured } = loadForumApi();
    await api.updateForumTopicApi(7, '标题', '正文', [], 'discussion');
    expect('content_format' in payloadOf(captured).data).toBe(false);
  });

  it('必红 · 变异：载荷里塞进 content_format 后，键集合断言与「不含该键」断言都会红', async () => {
    const anchor = "        images: images,\n    } as UTSJSONObject\n    if (category.length > 0) {";
    const broken = loadMutatedForumApi([
      [anchor, "        images: images,\n        content_format: 'markdown',\n    } as UTSJSONObject\n    if (category.length > 0) {"],
    ]);
    await broken.api.updateForumTopicApi(7, '标题', '正文', [], 'discussion');
    const data = payloadOf(broken.captured).data;
    expect(data.content_format).toBe('markdown');                 // 坏实现真的会带上
    expect(Object.keys(data)).toContain('content_format');        // ⇒ 第 ③ 组的键集合断言判红
  });
});
