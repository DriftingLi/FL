/**
 * 搜索展示纯函数单元测试（#979）
 *
 * utils/searchDisplay.uts 是 UTS，Node 无法直接 import，按仓库惯例（format.test.js /
 * pointsDisplay.test.js）以镜像实现验证算法行为；文末的「镜像同步」用例把 .uts 源码
 * 与镜像逐条对齐，防止两份实现悄悄分叉。
 */
const fs = require('fs');
const path = require('path');

const SRC_PATH = path.join(__dirname, 'searchDisplay.uts');
const src = fs.readFileSync(SRC_PATH, 'utf8');

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
    const pagesJson = fs.readFileSync(path.join(__dirname, '..', 'pages.json'), 'utf8');
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
