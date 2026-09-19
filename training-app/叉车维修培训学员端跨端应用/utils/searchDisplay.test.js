/**
 * 搜索展示纯函数单元测试（#979）
 *
 * utils/searchDisplay.uts 是 UTS，Node 无法直接 import，按仓库惯例（format.test.js /
 * pointsDisplay.test.js）以镜像实现验证算法行为；文末的「镜像同步」用例把 .uts 源码
 * 与镜像逐条对齐，防止两份实现悄悄分叉。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const SRC_PATH = path.join(__dirname, 'searchDisplay.uts');
const src = readText(SRC_PATH);

// ===== 镜像实现（与 searchDisplay.uts 保持一致）=====

const HIT_REPLY_LABEL = '命中在回复';

function sectionLabel(type) {
  if (type === 'course') return '课程';
  if (type === 'chapter') return '章节';
  if (type === 'question') return '题目';
  if (type === 'content') return '精选内容';
  if (type === 'topic') return '帖子';
  return '';
}

function hitLabel(hitField) {
  if (hitField === 'reply') return HIT_REPLY_LABEL;
  return '';
}

function searchItemPath(item) {
  const id = item.id.toString();
  if (item.type === 'course') return '/pages/courses/course-detail?id=' + id;
  if (item.type === 'chapter') {
    if (item.parent_id <= 0) return '';
    return '/pages/courses/chapter-view?course_id=' + item.parent_id.toString() + '&chapter_id=' + id;
  }
  if (item.type === 'question') return '/pages/practice/practice-do?mode=single&question_id=' + id;
  if (item.type === 'content') return '/pages/featured/featured-detail?id=' + id;
  if (item.type === 'topic') return '/pages/forum/forum-detail?id=' + id;
  return '';
}

function splitHighlight(text, keyword) {
  const segments = [];
  const plain = text;
  if (plain.length === 0) return segments;
  const kw = keyword.trim();
  if (kw.length === 0) {
    segments.push({ text: plain, hit: false });
    return segments;
  }
  const lower = plain.toLowerCase();
  const needle = kw.toLowerCase();
  let cursor = 0;
  for (;;) {
    const idx = lower.indexOf(needle, cursor);
    if (idx < 0) break;
    if (idx > cursor) segments.push({ text: plain.substring(cursor, idx), hit: false });
    segments.push({ text: plain.substring(idx, idx + needle.length), hit: true });
    cursor = idx + needle.length;
  }
  if (cursor < plain.length) segments.push({ text: plain.substring(cursor), hit: false });
  return segments;
}

function stripEdges(text) {
  let t = text;
  if (t.startsWith('…')) t = t.substring(1);
  while (t.length > 0 && t.endsWith('…')) t = t.substring(0, t.length - 1);
  return t;
}

function isSameText(a, b) {
  const x = stripEdges(a);
  const y = stripEdges(b);
  if (x.length === 0 || y.length === 0) return false;
  const short = x.length <= y.length ? x : y;
  const long = x.length <= y.length ? y : x;
  return long.substring(0, short.length) === short;
}

function itemPrimaryText(item) {
  if (item.type === 'question' && item.snippet.length > 0) return item.snippet;
  return item.title;
}

function shouldShowSnippet(item) {
  if (item.type === 'question') return false;
  const raw = item.snippet.length > 0 ? item.snippet : item.summary;
  if (raw.length === 0) return false;
  if (item.title.length === 0) return true;
  return !isSameText(item.title, raw);
}

// ===== 用例 =====

describe('splitHighlight：关键词切段（高亮由端上做，ADR-0049 决策 6）', () => {
  it('命中段与非命中段按原文顺序拼回，且不丢字符', () => {
    const segs = splitHighlight('叉车维修与叉车保养', '叉车');
    expect(segs).toEqual([
      { text: '叉车', hit: true },
      { text: '维修与', hit: false },
      { text: '叉车', hit: true },
      { text: '保养', hit: false },
    ]);
    expect(segs.map((s) => s.text).join('')).toBe('叉车维修与叉车保养');
  });

  it('大小写无关（后端 LIKE 也是 LOWER 匹配）', () => {
    const segs = splitHighlight('Forklift FORKLIFT', 'forklift');
    expect(segs.filter((s) => s.hit).map((s) => s.text)).toEqual(['Forklift', 'FORKLIFT']);
  });

  it('关键词为空 → 单段不命中（不切成空数组，避免正文整段消失）', () => {
    expect(splitHighlight('叉车', '')).toEqual([{ text: '叉车', hit: false }]);
    expect(splitHighlight('叉车', '   ')).toEqual([{ text: '叉车', hit: false }]);
  });

  it('文本为空 → 空数组', () => {
    expect(splitHighlight('', '叉车')).toEqual([]);
  });

  it('无命中 → 单段不命中；整段命中 → 单段命中', () => {
    expect(splitHighlight('叉车维修', '电焊')).toEqual([{ text: '叉车维修', hit: false }]);
    expect(splitHighlight('叉车', '叉车')).toEqual([{ text: '叉车', hit: true }]);
  });

  it('关键词首尾空白先 trim（用户多打空格不该导致零命中）', () => {
    expect(splitHighlight('叉车维修', '  叉车  ')).toEqual([
      { text: '叉车', hit: true },
      { text: '维修', hit: false },
    ]);
  });
});

describe('hitLabel：命中位置标注（回复命中必须说清）', () => {
  it('reply → 标注；title / body / 空 → 不标注（高亮本身已表达）', () => {
    expect(hitLabel('reply')).toBe(HIT_REPLY_LABEL);
    expect(hitLabel('title')).toBe('');
    expect(hitLabel('body')).toBe('');
    expect(hitLabel('')).toBe('');
  });
});

describe('itemPrimaryText / shouldShowSnippet：结果行不重复渲染同一句话（#979 观感修复）', () => {
  const Q = {
    type: 'question',
    title: '气压制动的机动车辆，当气压升至600kPa且不适用制动的情况下，停止空气压缩机（）后，其气压的…',
    summary: '气压制动的机动车辆，当气压升至600kPa且不适用制动的情况下，停止空气压缩机（）后，其气压的…',
    snippet: '…适用制动的情况下，停止空气压缩机（）后，其气压的降低不应超过10kPa。',
  };

  it('题目：主行改用命中窗口，且不再渲染第二行（后端 title == summary == 题干截断）', () => {
    expect(itemPrimaryText(Q)).toBe(Q.snippet);
    expect(shouldShowSnippet(Q)).toBe(false);
  });

  it('题目无 snippet 时退回 title（老客户端口径，不空行）', () => {
    const old = { type: 'question', title: '题干', summary: '题干', snippet: '' };
    expect(itemPrimaryText(old)).toBe('题干');
    expect(shouldShowSnippet(old)).toBe(false);
  });

  it('课程/帖子：有独立标题面 ⇒ 主行是标题、第二行是片段', () => {
    const c = { type: 'course', title: '场（厂）内机动车辆基础', summary: '场车的工作原理…', snippet: '…对标 N1 考试大纲第一章。' };
    expect(itemPrimaryText(c)).toBe('场（厂）内机动车辆基础');
    expect(shouldShowSnippet(c)).toBe(true);
  });

  it('片段与标题属同一段文本（截断关系，含首尾省略号）⇒ 不显示第二行', () => {
    const ch = { type: 'chapter', title: '第一章 起步与行驶安全', summary: '', snippet: '第一章 起步与行驶安全 起步操作规程…' };
    expect(shouldShowSnippet(ch)).toBe(false);
    const rev = { type: 'chapter', title: '…第一章 起步与行驶安全', summary: '', snippet: '第一章 起步与行驶安全' };
    expect(shouldShowSnippet(rev)).toBe(false);
  });

  it('无片段（snippet 与 summary 都空）⇒ 不显示第二行；标题为空但片段在 ⇒ 显示', () => {
    expect(shouldShowSnippet({ type: 'topic', title: '帖子标题', summary: '', snippet: '' })).toBe(false);
    expect(shouldShowSnippet({ type: 'topic', title: '', summary: '', snippet: '正文窗口' })).toBe(true);
  });

  it('snippet 为空时回退 summary 做同一段文本比较（后端「只增不破」兼容字段）', () => {
    const legacy = { type: 'course', title: '课程名', summary: '课程名', snippet: '' };
    expect(shouldShowSnippet(legacy)).toBe(false);
  });
});

describe('sectionLabel：分区中文名（五分区，含 #982 新增的章节）', () => {
  it.each([
    ['course', '课程'],
    ['chapter', '章节'],
    ['question', '题目'],
    ['content', '精选内容'],
    ['topic', '帖子'],
    ['unknown', ''],
  ])('%s → %s', (type, label) => {
    expect(sectionLabel(type)).toBe(label);
  });
});

describe('searchItemPath：落点（ADR-0049 决策 2「搜到但打不开不合格」）', () => {
  it('五分区各有落点，且都指向已注册页面', () => {
    expect(searchItemPath({ type: 'course', id: 7, parent_id: 0 })).toBe('/pages/courses/course-detail?id=7');
    expect(searchItemPath({ type: 'chapter', id: 31, parent_id: 7 })).toBe(
      '/pages/courses/chapter-view?course_id=7&chapter_id=31',
    );
    expect(searchItemPath({ type: 'question', id: 99, parent_id: 0 })).toBe(
      '/pages/practice/practice-do?mode=single&question_id=99',
    );
    expect(searchItemPath({ type: 'content', id: 12, parent_id: 0 })).toBe('/pages/featured/featured-detail?id=12');
    expect(searchItemPath({ type: 'topic', id: 5, parent_id: 0 })).toBe('/pages/forum/forum-detail?id=5');
  });

  it('章节缺 parent_id（契约异常）→ 空串，调用方不得绑定点击', () => {
    expect(searchItemPath({ type: 'chapter', id: 31, parent_id: 0 })).toBe('');
  });

  it('未知类型 → 空串（不猜落点）', () => {
    expect(searchItemPath({ type: 'material', id: 1, parent_id: 0 })).toBe('');
  });

  it('落点页面全部在 pages.json 注册（防幻影路由）', () => {
    const pagesJson = readText(path.join(__dirname, '..', 'pages.json'));
    const paths = ['course', 'chapter', 'question', 'content', 'topic'].map((t) =>
      searchItemPath({ type: t, id: 1, parent_id: 1 }),
    );
    expect(paths.filter((p) => p.length === 0)).toEqual([]);
    for (const p of paths) {
      const route = p.replace(/^\//, '').split('?')[0];
      expect(pagesJson).toContain('"' + route + '"');
    }
  });
});

describe('镜像同步：searchDisplay.uts 与本文件逐条一致', () => {
  it('splitHighlight / hitLabel / sectionLabel / searchItemPath 都在 .uts 内成文', () => {
    for (const fn of ['export function splitHighlight', 'export function hitLabel', 'export function sectionLabel', 'export function searchItemPath', 'export function projectSnippet', 'export function displaySnippet']) {
      expect(src).toContain(fn);
    }
  });

  it('.uts 内的落点表与本文件镜像同式（改一处必须改两处）', () => {
    expect(src).toContain("'/pages/courses/course-detail?id=' + id");
    expect(src).toContain("'/pages/courses/chapter-view?course_id=' + item.parent_id.toString() + '&chapter_id=' + id");
    expect(src).toContain("'/pages/practice/practice-do?mode=single&question_id=' + id");
    expect(src).toContain("'/pages/featured/featured-detail?id=' + id");
    expect(src).toContain("'/pages/forum/forum-detail?id=' + id");
  });

  it('去重口径在本文件与 .uts 内同式（题目主行=命中窗口；截断关系比较去首尾省略号）', () => {
    expect(src).toContain('export function itemPrimaryText');
    expect(src).toContain('export function shouldShowSnippet');
    expect(src).toContain("if (item.type == 'question' && item.snippet.length > 0) return item.snippet");
    expect(src).toContain("if (item.type == 'question') return false");
    expect(src).toContain('function stripEdges');
    expect(src).toContain('function isSameText');
  });

  it('高亮镜像同式：小写比较 + indexOf 循环推进游标', () => {
    expect(src).toContain('const lower = plain.toLowerCase()');
    expect(src).toContain('const needle = kw.toLowerCase()');
    expect(src).toContain('lower.indexOf(needle, cursor)');
  });

  it('投影同源：projectSnippet 走 parseMarkdown，不自己写行内正则剥标记（ADR-0044 同源原则）', () => {
    expect(src).toContain("import { parseMarkdown } from './markdown'");
    const from = src.indexOf('export function projectSnippet');
    const body = src.slice(from, src.indexOf('\n}', from));
    expect(body).toContain('parseMarkdown(raw)');
    // 投影内部不得出现任何行内 markdown 正则（那是 stripInline 的实现面）
    expect(body).not.toMatch(/\.replace\(/);
  });

  it('snippet 为空时回退兼容字段 summary（后端「只增不破」）', () => {
    expect(src).toContain('item.snippet.length > 0 ? item.snippet : item.summary');
  });
});
