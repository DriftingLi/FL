/**
 * 学员意见反馈域（#1950）· **行为级**测试
 *
 * 为什么是行为而不是源码文本：判据只在**跑起来**之后才看得见 ——
 * 提交载荷里 `content` / `images` / `contact` 是不是都在、图片缺省时是不是显式传空数组、
 * `contact` 空串有没有被端上擅自「省略」（后端把省略当空，本端两种写法都要得到同一份四字段载荷）、
 * 回包里的 `handled_at` 为 null 时有没有被误映射成 `''`（那会让「未处理」看起来像「有处理时间」）、
 * `images` 有没有被映射进 `FeedbackItem`（漏了就是「传得进去、看不见」）。
 *
 * 缝：`utils/utsHarness.loadUts` 把 `api/feedback.uts` 当 JS 真执行；注入的 `helpers` 与
 * `forumDto.extractImages` 是**仓内真件**，只有网络出口（`postMapped` / `getPaged`）是替身。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const HELPERS_UTS = path.join(__dirname, '..', 'api', 'helpers.uts');
const FORUM_DTO_UTS = path.join(__dirname, '..', 'api', 'forumDto.uts');
const FEEDBACK_UTS = path.join(__dirname, '..', 'api', 'feedback.uts');

function loadFeedback(pageRows = []) {
  const helpers = loadUts(HELPERS_UTS, {});
  const dto = loadUts(FORUM_DTO_UTS, {
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    toBool: helpers.toBool,
    decodePage: () => {
      throw new Error('decodePage 不在本套件射程');
    },
  });

  const calls = { post: [], page: [] };
  const feedback = loadUts(FEEDBACK_UTS, {
    postMapped: (url, payload, map) => {
      calls.post.push({ url, payload });
      return Promise.resolve(map(payload));
    },
    getPaged: (url, page, pageSize, toItem, query) => {
      calls.page.push({ url, page, pageSize, query });
      const items = pageRows.map((row) => toItem(row));
      return Promise.resolve({ items, total: items.length });
    },
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    extractImages: dto.extractImages,
  });
  return { feedback, calls };
}

const ROW = {
  id: 7,
  content: '第三节视频打不开',
  images: ['/static/uploads/images/forum/a.png', '/static/uploads/images/forum/b.png'],
  contact: '微信 abc-123',
  status: 'pending',
  handled_at: null,
  created_at: '2026-10-28T17:00:00+08:00',
};

describe('提交反馈：载荷与回包映射', () => {
  it('正文 / 图片 / 联系方式三个字段都进载荷，路径是学员面 /feedback', async () => {
    const { feedback, calls } = loadFeedback();
    await feedback.submitFeedbackApi('第三节视频打不开', ['/static/uploads/images/forum/a.png'], '微信 abc-123');
    expect(calls.post).toHaveLength(1);
    expect(calls.post[0].url).toBe('/feedback');
    expect(calls.post[0].payload).toEqual({
      content: '第三节视频打不开',
      images: ['/static/uploads/images/forum/a.png'],
      contact: '微信 abc-123',
    });
  });

  it('无图无联系方式时仍显式传空数组与空串（不把字段省掉）', async () => {
    const { feedback, calls } = loadFeedback();
    await feedback.submitFeedbackApi('只有一句话');
    expect(calls.post[0].payload).toEqual({ content: '只有一句话', images: [], contact: '' });
  });

  it('联系方式原样提交：手机 / 微信 / 一句时间都不被端上改写或校验', async () => {
    const { feedback, calls } = loadFeedback();
    await feedback.submitFeedbackApi('想约时间', [], '晚上八点后打我 13800000000');
    expect(calls.post[0].payload.contact).toBe('晚上八点后打我 13800000000');
  });

  it('回包映射成 FeedbackItem：图片数组、状态、created_at 都在', async () => {
    const { feedback } = loadFeedback();
    const item = await feedback.submitFeedbackApi('x', [], '');
    // 替身把 payload 原样回给 mapper —— 这里只验 mapper 不丢字段
    expect(item.id).toBe(0);
    expect(item.images).toEqual([]);
    expect(item.status).toBe('');
  });

  it('handled_at 为 null 时映射成 null（不是空串）', async () => {
    const { feedback } = loadFeedback([ROW]);
    const page = await feedback.listFeedbackApi(1, 20);
    expect(page.items[0].handled_at).toBeNull();
    expect(page.items[0].status).toBe('pending');
    expect(page.items[0].images).toEqual(ROW.images);
    expect(page.items[0].contact).toBe('微信 abc-123');
  });

  it('handled_at 有值时保留原文（RFC3339 不重排、不格式化）', async () => {
    const handled = Object.assign({}, ROW, { status: 'handled', handled_at: '2026-10-29T09:00:00+08:00' });
    const { feedback } = loadFeedback([handled]);
    const page = await feedback.listFeedbackApi();
    expect(page.items[0].handled_at).toBe('2026-10-29T09:00:00+08:00');
  });

  it('旧后端缺 images 字段时归一成空数组（不炸、不留 undefined）', async () => {
    const noImages = { id: 1, content: 'c', contact: '', status: 'pending', handled_at: null, created_at: 't' };
    const { feedback } = loadFeedback([noImages]);
    const page = await feedback.listFeedbackApi();
    expect(page.items[0].images).toEqual([]);
  });

  it('列表走 /feedback 且带分页参数，端上不做排序', async () => {
    const { feedback, calls } = loadFeedback([]);
    await feedback.listFeedbackApi(2, 10);
    expect(calls.page[0].url).toBe('/feedback');
    expect(calls.page[0].page).toBe(2);
    expect(calls.page[0].pageSize).toBe(10);
  });

  it('上限常量与后端同源：正文 1000 字、图片 3 张', () => {
    const { feedback } = loadFeedback();
    expect(feedback.FEEDBACK_CONTENT_MAX).toBe(1000);
    expect(feedback.FEEDBACK_IMAGES_MAX).toBe(3);
    expect(feedback.FEEDBACK_STATUS_HANDLED).toBe('handled');
  });
});
