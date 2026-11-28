/**
 * 回复浮层「原讨论内容」引文 · **行为级**测试
 *
 * 为什么跑函数而不是断言 `.uvue` 里出现过什么：引文的判据是「压成一行、不留记号、超长截断」，
 * 这三条只有把正文真喂进去看产出才算数（接线守护见 forumContract，不构成 ③ 证据）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 真执行 `utils/forumReplyQuote.uts`（先例 forumBodyBehavior）。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const QUOTE_UTS = path.join(__dirname, 'forumReplyQuote.uts');

const quote = () => loadUts(QUOTE_UTS, {});

describe('引文：Markdown 正文压成一行', () => {
  it('去掉标题 / 引用 / 强调 / 行内代码记号，只留文字', () => {
    const { replyQuoteExcerpt } = quote();
    expect(replyQuoteExcerpt('## 标题\n> 引用一句\n这是**粗体**与 *斜体* 还有 `code`')).toBe(
      '标题 引用一句 这是粗体与 斜体 还有 code'
    );
  });

  it('链接只留文字，图片整段丢弃', () => {
    const { replyQuoteExcerpt } = quote();
    expect(replyQuoteExcerpt('参见[官方文档](https://example.com/a) 与 ![截图](https://example.com/b.png) 完')).toBe(
      '参见官方文档 与 完'
    );
  });

  it('换行与连续空白压成一个空格（引文只占一行）', () => {
    const { replyQuoteExcerpt } = quote();
    expect(replyQuoteExcerpt('第一段\n\n第二段   第三段')).toBe('第一段 第二段 第三段');
  });

  it('代码块整段丢弃、分隔线与有序列表序号不残留（引文只留文字）', () => {
    const { replyQuoteExcerpt } = quote();
    expect(replyQuoteExcerpt('正文\n\n---\n\n1. 第一\n2. 第二\n\n```python\nprint("x")\n```')).toBe(
      '正文 第一 第二'
    );
  });

  it('超长正文截断到上限并补省略号', () => {
    const { replyQuoteExcerpt, REPLY_QUOTE_MAX_CHARS } = quote();
    const raw = '甲'.repeat(REPLY_QUOTE_MAX_CHARS + 20);
    const out = replyQuoteExcerpt(raw);
    expect(out.length).toBe(REPLY_QUOTE_MAX_CHARS + 1);
    expect(out.endsWith('…')).toBe(true);
  });

  it('恰好等于上限时不截断（不补省略号）', () => {
    const { replyQuoteExcerpt, REPLY_QUOTE_MAX_CHARS } = quote();
    const raw = '乙'.repeat(REPLY_QUOTE_MAX_CHARS);
    expect(replyQuoteExcerpt(raw)).toBe(raw);
  });

  it('空正文 / 上限 <= 0 都返回空串（面板据此整块不渲染）', () => {
    const { replyQuoteExcerpt } = quote();
    expect(replyQuoteExcerpt('')).toBe('');
    expect(replyQuoteExcerpt('   \n  ')).toBe('');
    expect(replyQuoteExcerpt('有内容', 0)).toBe('');
  });

  it('文案单点在模型里（面板不自己写中文）', () => {
    const { REPLY_QUOTE_LABEL, REPLY_VIEW_ORIGINAL } = quote();
    expect(REPLY_QUOTE_LABEL).toBe('正在回复 · 原讨论内容');
    expect(REPLY_VIEW_ORIGINAL).toBe('查看原帖');
  });
});
