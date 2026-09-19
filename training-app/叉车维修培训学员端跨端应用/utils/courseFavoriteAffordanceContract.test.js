/**
 * 课程详情收藏位「可见性」契约（#1087 后续：收藏入口无视觉提示）
 *
 * ## 为什么需要这条守护
 *
 * 现象：课程详情页右上角的收藏入口在**未收藏时完全空白** —— 点击区域（`.header-right`）
 * 一直是好的、API 三连（check/add/remove）也一直是好的，但模板渲染的是**空串**，
 * 于是没有任何可点的视觉提示，用户报「课程详情缺了一个收藏功能」。
 *
 * 根因形态与 #1071 那批「图标槽位形同虚设」同族：**槽位在某个状态下合法地什么都不渲染**。
 * 本仓已有先例都用**成对字形**表达两态，本页是唯一一个未收藏态渲染空串的收藏位：
 *   - `pages/featured/featured-detail.uvue`   → `{{ isFavorited ? '⭐' : '☆' }}`
 *   - `pages/forum/components/forum-reply-list.uvue`   → `{{ item.liked ? '♥' : '♡' }}`
 *   - `pages/forum/components/forum-topic-header.uvue` → `{{ liked ? '♥' : '♡' }}`
 *   - `pages/courses/course-detail.uvue`       → `{{ isFavorited ? '❤️' : '' }}`  ← 空串
 *
 * ## 断言的是产物（模板两态都非空），不是源码片段
 *
 * 判据刻意**不**钉死具体码位（照 ADR-0007 先例：字形可因字体渲染再调整，守护不该拦这种改法），
 * 只钉住「两态都必须渲染出非空字形」这一条**行为**。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const PAGE = 'pages/courses/course-detail.uvue';

/**
 * 从模板里取出收藏位 `<text class="favorite-icon">{{ … ? 'A' : 'B' }}</text>` 的两个分支字面量。
 * 返回 `{ ok, trueBranch, falseBranch }`；解析失败时 ok=false（防「正则失效即假绿」）。
 */
function parseFavoriteIconBranches(src) {
  const m = /favorite-icon[^>]*>\{\{\s*[^?]+?\?\s*'([^']*)'\s*:\s*'([^']*)'\s*\}\}/.exec(src);
  if (m === null) return { ok: false, trueBranch: null, falseBranch: null };
  return { ok: true, trueBranch: m[1], falseBranch: m[2] };
}

describe('课程详情收藏位：两态都必须有可见字形（未收藏不得为空）', () => {
  const src = read(PAGE);

  it('收藏位仍用「已收藏/未收藏」二态三元表达式（结构未被换掉）', () => {
    expect(src).toContain('favorite-icon');
    expect(src).toContain('isFavorited');
    expect(parseFavoriteIconBranches(src).ok).toBe(true);
  });

  it('已收藏态渲染非空字形', () => {
    const { trueBranch } = parseFavoriteIconBranches(src);
    expect(trueBranch.length).toBeGreaterThan(0);
  });

  it('未收藏态渲染非空字形（本契约的核心：空串就是「没有收藏功能」的观感）', () => {
    const { falseBranch } = parseFavoriteIconBranches(src);
    expect(falseBranch.length).toBeGreaterThan(0);
  });

  it('两态字形不相同（否则看不出选中与否）', () => {
    const { trueBranch, falseBranch } = parseFavoriteIconBranches(src);
    expect(trueBranch).not.toBe(falseBranch);
  });

  it('字形不得只剩变体选择符（#1071 的坏形态：孤立 U+FE0F）', () => {
    const { trueBranch, falseBranch } = parseFavoriteIconBranches(src);
    for (const g of [trueBranch, falseBranch]) {
      expect(g.replace(/[\uFE0E\uFE0F]/g, '').length).toBeGreaterThan(0);
    }
  });

  it('点击接线仍在（可见性修好不等于把功能改掉）', () => {
    expect(src).toContain('@click="toggleFavorite"');
  });
});

describe('解析器具备红能力（防「正则失效即假绿」）', () => {
  const cases = [
    ["{{ isFavorited ? '❤️' : '' }}", '', '空串未收藏态必须被解析出来'],
    ["{{ isFavorited ? '❤️' : 'x' }}", 'x', '非空未收藏态'],
  ];

  it.each(cases)('能解析出未收藏分支：%s', (expr, expected, _why) => {
    const fake = `<text class="favorite-icon">${expr}</text>`;
    const r = parseFavoriteIconBranches(fake);
    expect(r.ok).toBe(true);
    expect(r.falseBranch).toBe(expected);
  });

  it('结构不匹配时 ok=false（不静默放过）', () => {
    expect(parseFavoriteIconBranches('<text class="favorite-icon">❤️</text>').ok).toBe(false);
  });
});
