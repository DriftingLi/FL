/**
 * 帮助中心展示纯函数单元测试（#1081）
 *
 * `utils/faqDisplay.uts` 是 UTS，Node 无法直接 import，按仓库惯例（`searchDisplay.test.js` /
 * `pointsDisplay.test.js` / `format.test.js`）以**镜像实现**验证算法行为；文末的「镜像同步」用例
 * 把 `.uts` 源码与镜像逐条对齐，防止两份实现悄悄分叉。
 *
 * 本文件断言的是**产物行为**（给定分类+关键词 → 出什么 chip、出什么分组），不是页面源码文本：
 * 页面对这层的接线由 `utils/helpCenterContract.test.js` 单独钉住。
 */
const fs = require('fs');
const path = require('path');

const SRC_PATH = path.join(__dirname, 'faqDisplay.uts');
const src = fs.readFileSync(SRC_PATH, 'utf8');

// ===== 镜像实现（与 faqDisplay.uts 保持一致）=====

function faqSearching(keyword) {
  return keyword.trim().length > 0;
}

function faqMatches(entry, keyword) {
  const k = keyword.trim().toLowerCase();
  if (k.length === 0) return true;
  if (entry.question.toLowerCase().indexOf(k) >= 0) return true;
  return entry.answer.toLowerCase().indexOf(k) >= 0;
}

function faqHitCount(entries, keyword) {
  let n = 0;
  for (let i = 0; i < entries.length; i++) {
    if (faqMatches(entries[i], keyword)) n = n + 1;
  }
  return n;
}

function buildFaqNav(categories, keyword, activeCode) {
  const nav = [];
  const searching = faqSearching(keyword);
  let total = 0;
  for (let i = 0; i < categories.length; i++) {
    total = total + faqHitCount(categories[i].entries, keyword);
  }
  nav.push({ code: '', label: '全部 (' + total.toString() + ')', active: activeCode.length === 0 && !searching });
  for (let i = 0; i < categories.length; i++) {
    const c = categories[i];
    nav.push({
      code: c.code,
      label: c.title + ' (' + faqHitCount(c.entries, keyword).toString() + ')',
      active: activeCode === c.code && !searching,
    });
  }
  return nav;
}

function buildFaqGroups(categories, keyword, activeCode) {
  const searching = faqSearching(keyword);
  const out = [];
  for (let i = 0; i < categories.length; i++) {
    const c = categories[i];
    if (!searching && activeCode.length > 0 && c.code !== activeCode) continue;
    const entries = [];
    for (let j = 0; j < c.entries.length; j++) {
      if (faqMatches(c.entries[j], keyword)) entries.push(c.entries[j]);
    }
    if (entries.length === 0) continue;
    out.push({ code: c.code, title: c.title, entries });
  }
  if (out.length === 1) {
    return [{ code: out[0].code, title: '', entries: out[0].entries }];
  }
  return out;
}

function faqEmptyText(keyword) {
  if (faqSearching(keyword)) return '没有匹配的问题';
  return '帮助内容正在准备中';
}

function faqEmptyIcon(keyword) {
  if (faqSearching(keyword)) return '🔍';
  return '🎧';
}

// ===== 夹具：形状取自后端种子（7 类 / 每类若干条，answer 纯文本）=====

const entry = (id, question, answer) => ({ id, question, answer, sort_order: 0 });

const CATEGORIES = [
  {
    code: 'account',
    id: 1,
    sort_order: 1,
    title: '账号与登录',
    entries: [
      entry(11, '怎么注册学员账号？', '在登录页点「注册」。'),
      entry(12, '忘记密码怎么办？', '在登录页点「忘记密码」，用手机号接收验证码后重设。'),
    ],
  },
  {
    code: 'points',
    id: 2,
    sort_order: 2,
    title: '积分与任务',
    entries: [
      entry(21, '积分怎么获得？', '任务中心领取、每日打卡直记、论坛发帖与回复。'),
      entry(22, '每日任务什么时候重置？', '每天 0 点重置。'),
    ],
  },
];

describe('faqSearching：搜索态判定', () => {
  it.each([
    ['', false],
    ['   ', false],
    ['积分', true],
    ['  积分  ', true],
  ])('keyword=%j → %s', (keyword, expected) => {
    expect(faqSearching(keyword)).toBe(expected);
  });
});

describe('faqMatches：问题或答案命中即算（口径同 Web 帮助中心）', () => {
  it('空关键词全命中（含首尾空白）', () => {
    expect(faqMatches(CATEGORIES[0].entries[0], '')).toBe(true);
    expect(faqMatches(CATEGORIES[0].entries[0], '   ')).toBe(true);
  });

  it('命中问题**正文**', () => {
    expect(faqMatches(CATEGORIES[0].entries[0], '注册')).toBe(true);
  });

  it('命中**答案**（只判问题会漏掉这一条 —— 判别力用例）', () => {
    // 「验证码」只出现在第 2 条的答案里，问题文本里没有
    const target = CATEGORIES[0].entries[1];
    expect(target.question.includes('验证码')).toBe(false);
    expect(target.answer.includes('验证码')).toBe(true);
    expect(faqMatches(target, '验证码')).toBe(true);
  });

  it('大小写无关（英文关键词）', () => {
    const e = entry(31, 'What is APPID?', 'appid 在 manifest 里配置。');
    expect(faqMatches(e, 'appid')).toBe(true);
    expect(faqMatches(e, 'APPID')).toBe(true);
  });

  it('未命中 → false（不是恒真）', () => {
    expect(faqMatches(CATEGORIES[0].entries[0], '不存在的词')).toBe(false);
  });
});

describe('faqHitCount：分类 chip 上的数字', () => {
  it('空关键词 = 该分类全部条目数', () => {
    expect(faqHitCount(CATEGORIES[0].entries, '')).toBe(2);
  });

  it('按当前关键词计数（跨问题与答案）', () => {
    expect(faqHitCount(CATEGORIES[0].entries, '注册')).toBe(1);
    expect(faqHitCount(CATEGORIES[0].entries, '登录页')).toBe(2);
    expect(faqHitCount(CATEGORIES[0].entries, '验证码')).toBe(1);
  });

  it('无命中 → 0', () => {
    expect(faqHitCount(CATEGORIES[0].entries, 'zzz')).toBe(0);
  });
});

describe('buildFaqNav：分类 chip 行', () => {
  it('首项是「全部」且数字为各类命中数之和', () => {
    const nav = buildFaqNav(CATEGORIES, '', '');
    expect(nav[0].code).toBe('');
    expect(nav[0].label).toBe('全部 (4)');
    expect(nav[0].active).toBe(true);
  });

  it('其余项按传入顺序（= 后端 sort_order），label 含分类名与命中数', () => {
    const nav = buildFaqNav(CATEGORIES, '', '');
    expect(nav.map((n) => n.code)).toEqual(['', 'account', 'points']);
    expect(nav[1].label).toBe('账号与登录 (2)');
  });

  it('chip 数字随关键词变化（搜索态下仍显示各分类命中数）', () => {
    const nav = buildFaqNav(CATEGORIES, '积分', '');
    expect(nav[0].label).toBe('全部 (1)');
    expect(nav[1].label).toBe('账号与登录 (0)');
    expect(nav[2].label).toBe('积分与任务 (1)');
  });

  it('搜索态下**不标记任何选中**（筛选跨分类，芯片只承担信息面）', () => {
    const nav = buildFaqNav(CATEGORIES, '积分', 'points');
    expect(nav.filter((n) => n.active)).toEqual([]);
  });

  it('非搜索态：选中 code 的那一项 active，同时候「全部」不再是 active', () => {
    const nav = buildFaqNav(CATEGORIES, '', 'points');
    expect(nav[0].active).toBe(false);
    expect(nav[1].active).toBe(false);
    expect(nav[2].active).toBe(true);
  });

  it('空分类列表 → 只剩「全部 (0)」', () => {
    const nav = buildFaqNav([], '', '');
    expect(nav.length).toBe(1);
    expect(nav[0].label).toBe('全部 (0)');
  });
});

describe('buildFaqGroups：右侧/下方问答分组', () => {
  it('未搜索未选分类 → 全部分类，多分组保留分类标题', () => {
    const groups = buildFaqGroups(CATEGORIES, '', '');
    expect(groups.map((g) => g.code)).toEqual(['account', 'points']);
    expect(groups[0].title).toBe('账号与登录');
    expect(groups[0].entries.length).toBe(2);
  });

  it('选中某分类 → 只留该分类', () => {
    const groups = buildFaqGroups(CATEGORIES, '', 'points');
    expect(groups.map((g) => g.code)).toEqual(['points']);
  });

  it('搜索跨分类：已选中 points 时搜「登录页」仍出 account（搜索态忽略分类选择）', () => {
    const groups = buildFaqGroups(CATEGORIES, '登录页', 'points');
    expect(groups.map((g) => g.code)).toEqual(['account']);
  });

  it('只有**一个**分组有内容时隐藏分类标题（同 Web 帮助中心，不啰嗦）', () => {
    const groups = buildFaqGroups(CATEGORIES, '积分', '');
    expect(groups.length).toBe(1);
    expect(groups[0].title).toBe('');
    expect(groups[0].entries.length).toBe(1);
  });

  it('分组内条目被关键词过滤（不是整组保留）', () => {
    const groups = buildFaqGroups(CATEGORIES, '注册', '');
    expect(groups.length).toBe(1);
    expect(groups[0].entries.map((e) => e.id)).toEqual([11]);
  });

  it('无命中 → 空数组（页面据此走空态）', () => {
    expect(buildFaqGroups(CATEGORIES, 'zzz', '')).toEqual([]);
  });

  it('空分类（entries: []）不产出空分组', () => {
    const withEmpty = [...CATEGORIES, { code: 'job', id: 3, sort_order: 3, title: '就业与简历', entries: [] }];
    const groups = buildFaqGroups(withEmpty, '', '');
    expect(groups.map((g) => g.code)).toEqual(['account', 'points']);
  });

  it('选中一个空分类 → 空数组（走空态，而不是空标题卡片）', () => {
    const withEmpty = [...CATEGORIES, { code: 'job', id: 3, sort_order: 3, title: '就业与简历', entries: [] }];
    expect(buildFaqGroups(withEmpty, '', 'job')).toEqual([]);
  });
});

describe('空态文案与图标：搜索无命中 ≠ 内容未就绪', () => {
  it('搜索态 → 「没有匹配的问题」+ 🔍', () => {
    expect(faqEmptyText('zzz')).toBe('没有匹配的问题');
    expect(faqEmptyIcon('zzz')).toBe('🔍');
  });

  it('非搜索态 → 「帮助内容正在准备中」+ 🎧', () => {
    expect(faqEmptyText('')).toBe('帮助内容正在准备中');
    expect(faqEmptyIcon('')).toBe('🎧');
    expect(faqEmptyText('   ')).toBe('帮助内容正在准备中');
  });

  it('两种空态文案**不相同**（否则用户分不清「没搜到」和「平台没内容」）', () => {
    expect(faqEmptyText('zzz')).not.toBe(faqEmptyText(''));
  });
});

describe('镜像同步：faqDisplay.uts 与本文件逐条一致', () => {
  it('七个纯函数都在 .uts 内成文并导出', () => {
    for (const fn of [
      'export function faqSearching',
      'export function faqMatches',
      'export function faqHitCount',
      'export function buildFaqNav',
      'export function buildFaqGroups',
      'export function faqEmptyText',
      'export function faqEmptyIcon',
    ]) {
      expect(src).toContain(fn);
    }
  });

  it('命中口径同式：trim + toLowerCase + 问题/答案各判一次', () => {
    expect(src).toContain("const k = keyword.trim().toLowerCase()");
    expect(src).toContain('entry.question.toLowerCase().indexOf(k) >= 0');
    expect(src).toContain('entry.answer.toLowerCase().indexOf(k) >= 0');
  });

  it('chip 文案同式：`全部 (N)` 与 `分类名 (N)`', () => {
    expect(src).toContain("'全部 (' + total.toString() + ')'");
    expect(src).toContain("c.title + ' (' + faqHitCount(c.entries, keyword).toString() + ')'");
  });

  it('搜索态不标记选中（`!searching` 同时守 activeCode 两项）', () => {
    expect(src).toContain('activeCode.length == 0 && !searching');
    expect(src).toContain('activeCode == c.code && !searching');
  });

  it('跨分类筛选同式：搜索时跳过 activeCode 过滤', () => {
    expect(src).toContain('if (!searching && activeCode.length > 0 && c.code != activeCode) continue');
  });

  it('单分组隐藏标题同式', () => {
    expect(src).toContain('if (out.length == 1)');
    expect(src).toContain("return [{ code: out[0].code, title: '', entries: out[0].entries } as FaqGroup]");
  });

  it('空态文案同式（两种空态分开）', () => {
    expect(src).toContain("if (faqSearching(keyword)) return '没有匹配的问题'");
    expect(src).toContain("return '帮助内容正在准备中'");
    expect(src).toContain("if (faqSearching(keyword)) return '🔍'");
    expect(src).toContain("return '🎧'");
  });

  it('不吞字符：本模块不做 markdown 渲染 / 不改写 answer（口径「原样换行」）', () => {
    // answer 只参与 indexOf 命中判定与透传，绝不 replace/split
    expect(src).not.toMatch(/entry\.answer\.(replace|split)/);
    expect(src).not.toContain('parseMarkdown');
  });
});
