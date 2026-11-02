/**
 * 笔记图片附件（#1949）· **行为级**测试
 *
 * 为什么是行为而不是源码文本：这些判据全在**跑起来**之后才看得见 —— 提交载荷里 `images` 到底是不是
 * 一份完整数组、编辑时是不是**全量替换**（删掉的图不会以「只传新增」的差量漏出去）、缺省调用是不是
 * 显式传了空数组而不是把字段省掉（后端「省略 = 清空」，两者语义不同）、回包里的 `images` 有没有被
 * 映射进 `NoteItem`（漏了就是「存得进去、回显不出来」）。
 *
 * 缝：`utils/utsHarness.loadUts` 把 `api/note.uts` 当 JS 真执行；注入的 `helpers` 与
 * `forumDto.extractImages` 是**仓内真件**，只有网络出口（`postMapped` / `putMapped` / `del` /
 * `getPaged`）是替身 —— 被测的是载荷编排与 DTO 映射，不是 HTTP。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const HELPERS_UTS = path.join(__dirname, '..', 'api', 'helpers.uts');
const FORUM_DTO_UTS = path.join(__dirname, '..', 'api', 'forumDto.uts');
const NOTE_UTS = path.join(__dirname, '..', 'api', 'note.uts');

/** 装配：真 helpers + 真 forumDto.extractImages，网络四口替身 */
function loadNote(pageRows = []) {
  const helpers = loadUts(HELPERS_UTS, {});
  const dto = loadUts(FORUM_DTO_UTS, {
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    toBool: helpers.toBool,
    // forumDto 的列表解码另有出路（`decodePage`），本套件只取 `extractImages`，故这里不提供实现
    decodePage: () => {
      throw new Error('decodePage 不在本套件射程');
    },
  });

  const calls = { post: [], put: [], del: [], page: [] };
  const note = loadUts(NOTE_UTS, {
    postMapped: (url, payload, map) => {
      calls.post.push({ url, payload });
      return Promise.resolve(map(payload));
    },
    putMapped: (url, payload, map) => {
      calls.put.push({ url, payload });
      return Promise.resolve(map(payload));
    },
    del: (url) => {
      calls.del.push(url);
      return Promise.resolve({});
    },
    getPaged: (url, page, pageSize, toItem, query) => {
      calls.page.push({ url, page, pageSize, query });
      const items = pageRows.map((row) => toItem(row));
      return Promise.resolve({ items, total: items.length });
    },
    toNumber: helpers.toNumber,
    toNumberOrNull: helpers.toNumberOrNull,
    toStr: helpers.toStr,
    extractImages: dto.extractImages,
  });
  return { note, calls };
}

describe('#1949 笔记图片：提交载荷', () => {
  it('新建时 images 原样进载荷（顺序即用户选择顺序）', async () => {
    const { note, calls } = loadNote();
    await note.createNoteApi('换季保养要点', ['note/a.png', 'note/b.png']);
    expect(calls.post).toHaveLength(1);
    expect(calls.post[0].url).toBe('/notes');
    expect(calls.post[0].payload.content).toBe('换季保养要点');
    expect(calls.post[0].payload.images).toEqual(['note/a.png', 'note/b.png']);
  });

  it('缺省不传图时载荷里仍是空数组（不是把字段省掉）', async () => {
    const { note, calls } = loadNote();
    await note.createNoteApi('只有文字');
    expect(calls.post[0].payload.images).toEqual([]);
    expect('images' in calls.post[0].payload).toBe(true);
  });

  it('编辑是全量替换：删掉的那张不会出现在载荷里', async () => {
    const { note, calls } = loadNote();
    await note.updateNoteApi(7, '改过的正文', ['note/b.png']);
    expect(calls.put).toHaveLength(1);
    expect(calls.put[0].url).toBe('/notes/7');
    expect(calls.put[0].payload.images).toEqual(['note/b.png']);
  });

  it('编辑清空图片时传空数组（后端据此全删，不留旧图）', async () => {
    const { note, calls } = loadNote();
    await note.updateNoteApi(7, '正文', []);
    expect(calls.put[0].payload.images).toEqual([]);
  });

  it('上限与后端同源：NOTE_IMAGES_MAX = 3', () => {
    const { note } = loadNote();
    expect(note.NOTE_IMAGES_MAX).toBe(3);
  });
});

describe('#1949 笔记图片：回包映射', () => {
  it('列表回包的 images 进 NoteItem（存得进去也回显得出来）', async () => {
    const { note } = loadNote([
      { id: 1, content: '带图的笔记', question_id: null, images: ['note/a.png', 'note/b.png'] },
    ]);
    const page = await note.listNotesApi('all', 1, 20);
    expect(page.items[0].images).toEqual(['note/a.png', 'note/b.png']);
  });

  it('旧后端不带 images 字段时归一成空数组（不是 undefined）', async () => {
    const { note } = loadNote([{ id: 2, content: '纯文字', question_id: 3 }]);
    const page = await note.listNotesApi('all', 1, 20);
    expect(page.items[0].images).toEqual([]);
  });

  it('images 里的非字符串项按 `toStr` 归一（DTO 层不把原始值带进模板）', async () => {
    const { note } = loadNote([{ id: 3, content: '脏数据', images: ['note/a.png', 42] }]);
    const page = await note.listNotesApi('all', 1, 20);
    expect(page.items[0].images).toEqual(['note/a.png', '42']);
  });
});
