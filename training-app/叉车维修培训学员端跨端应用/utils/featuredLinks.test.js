/**
 * utils/featuredLinks.uts 的行为锁（#2097 · 行为腿）
 *
 * 为什么必须真执行而不是读源码：一条链接能不能进列表，取决于**分词器的协议闸**
 * （markdownInline.inlineLinkSafe，http(s) 白名单）。「javascript: 不出现在产物里」这件事，
 * 源码文本断言证明不了 —— 这里把真 markdownInline 注入进去跑（先例 markdownInlineBehavior.test.js）。
 *
 * 断言取**字面产物**：顺序、去重、域名投影一次钉死。**范围限制也具名钉住**（裸 URL / 引用式 /
 * 图片链接都不算）—— 那与「运营在正文里用行内 [文字](url) 写外链」是同一份口径，不是解析漏了；
 * 要放宽得先改口径，别让下一个人把它当 bug 顺手改掉。
 */
const fs = require('fs');
const path = require('path');
const { loadUts, importedNames } = require('./utsHarness');

const MARKDOWN_INLINE = path.join(__dirname, 'markdownInline.uts');
const FEATURED_LINKS = path.join(__dirname, 'featuredLinks.uts');

/** 注入面真取 markdownInline 的实现：测试里不重写第二份分词（先例 aiAnswerBodyBehavior 逐次取新实例） */
const bindings = () => {
  const mi = loadUts(MARKDOWN_INLINE, {});
  return { splitInlineRuns: mi.splitInlineRuns, inlineLinkDomain: mi.inlineLinkDomain };
};
const mod = () => loadUts(FEATURED_LINKS, bindings());
const collect = (src) => mod().collectFeaturedLinks(src);

describe('featuredLinks（真执行）：内容精选正文外链的集中可达', () => {
  it('注入面与真 import 一致（清单不漂第二份）', () => {
    const src = fs.readFileSync(FEATURED_LINKS, 'utf8');
    expect([...importedNames(src)].sort()).toEqual(Object.keys(bindings()).sort());
  });

  it('行内链接成条：可见名 / 原 URL / 展示域名三样各归其位', () => {
    expect(collect('[点击 报名](https://a.com/reg)')).toEqual([
      { label: '点击 报名', url: 'https://a.com/reg', host: 'a.com' },
    ]);
  });

  it('按出现顺序出条，同一 URL 只留首次那条文字', () => {
    expect(collect('先 [B](https://b.cn) 后 [A](https://a.cn)')).toEqual([
      { label: 'B', url: 'https://b.cn', host: 'b.cn' },
      { label: 'A', url: 'https://a.cn', host: 'a.cn' },
    ]);
    expect(collect('[首次](https://x.cn/p) 然后 [改名](https://x.cn/p)')).toEqual([
      { label: '首次', url: 'https://x.cn/p', host: 'x.cn' },
    ]);
  });

  it('协议闸在分词器单点：坏协议与站内相对路径一条都不进列表', () => {
    expect(collect('[j](javascript:alert(1)) [d](data:texthtml,x) [f](ftp://z.cn/p) [站内](/news/9)')).toEqual([]);
  });

  it('只认行内 [文字](url)：裸 URL / 引用式 / 图片链接都不算（范围限制具名钉住）', () => {
    expect(collect('直接 https://a.com/x 没有中括号')).toEqual([]);
    expect(collect('[ref][1] 与 [1]: https://y.cn')).toEqual([]);
    expect(collect('看 ![故障图](https://e.com/a.png) 这张')).toEqual([]);
  });

  it('跨行不误配：左括号在上行、目标在下行时不产条', () => {
    expect(collect('[a]\n(bhttps://e.com)')).toEqual([]);
  });

  it('域名投影不吃端口与 userinfo（列表与确认弹窗同口径）', () => {
    expect(collect('[内](http://user:pw@host.cn:8080/p)')).toEqual([
      { label: '内', url: 'http://user:pw@host.cn:8080/p', host: 'host.cn' },
    ]);
  });

  it('空正文 ⇒ 空表（不抛、不返 null）', () => {
    expect(collect('')).toEqual([]);
  });
});
