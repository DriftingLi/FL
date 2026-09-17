/**
 * 笔记展示纯函数单元测试（#1082「我的笔记」列表页）
 *
 * utils/notebookDisplay.uts 是 UTS，Node 无法直接 import，按仓库惯例
 * （format.test.js / checkinCalendar.test.js / pointsDisplay.test.js）以**镜像实现**验证算法行为；
 * 文末的「镜像同步」用例把 .uts 源码与镜像逐条对齐，防止两份实现悄悄分叉。
 *
 * 三项截断长度（40 / 80 / 18）与 Web 侧 `frontend/src/pages/student/Notebook.vue` 的
 * noteTitle / noteExcerpt / questionLabel 逐字同源 —— 两端同一列表观感，改一处必须同改，
 * 故末节跨端比对**真的去读 Web 文件**（缺文件时显式 skip，不静默假绿）。
 */
const fs = require('fs');
const path = require('path');

const UTS = path.join(__dirname, 'notebookDisplay.uts');
const WEB_NOTEBOOK = path.join(__dirname, '..', '..', '..', 'frontend', 'src', 'pages', 'student', 'Notebook.vue');

// ===== 镜像实现（与 notebookDisplay.uts 保持一致）=====

function noteTitle(content) {
  const first = content.split('\n')[0].trim();
  if (first.length > 40) return first.substring(0, 40) + '…';
  return first;
}

function noteExcerpt(content) {
  const lines = content.split('\n');
  if (lines.length <= 1) return '';
  const rest = lines.slice(1).join(' ').trim();
  if (rest.length > 80) return rest.substring(0, 80) + '…';
  return rest;
}

function noteBadgeText(questionContent) {
  const t = questionContent.trim();
  if (t.length == 0) return '题目笔记';
  if (t.length > 18) return '题目：' + t.substring(0, 18) + '…';
  return '题目：' + t;
}

function isStandaloneNote(questionId) {
  if (questionId == null) return true;
  return questionId <= 0;
}

function noteEmptyText(scope) {
  if (scope == 'question') return '还没有题目笔记';
  if (scope == 'standalone') return '还没有独立笔记';
  return '还没有笔记';
}

/** 40 字标题锁在源码里成文（同步判据的可复用形态，末节用它做红能力自检） */
function hasTitleLock(src) {
  return src.includes('first.length > 40') && src.includes('first.substring(0, 40) + ');
}

const SRC = fs.readFileSync(UTS, 'utf8');

// ===== 用例 =====

describe('noteTitle：正文首行即标题（ADR-0055 不给笔记加 title 列）', () => {
  it('多行正文只取首行，且去掉首尾空白', () => {
    expect(noteTitle('  第一行  \n第二行\n第三行')).toBe('第一行');
  });

  it('单行正文即整条标题；空串给空串（不是 undefined）', () => {
    expect(noteTitle('只有一行')).toBe('只有一行');
    expect(noteTitle('')).toBe('');
    expect(noteTitle('\n\n')).toBe('');
  });

  it('超过 40 字截断并带省略号；正好 40 字不截（边界不多吃一个字）', () => {
    const exact = 'a'.repeat(40);
    expect(noteTitle(exact)).toBe(exact);
    expect(noteTitle(exact + 'b')).toBe(exact + '…');
    expect(noteTitle(exact + 'b').length).toBe(41);
  });

  it('中文按字符计（不是字节）：41 个汉字被截到 40 字 + 省略号', () => {
    const cn = '汉'.repeat(41);
    expect(noteTitle(cn)).toBe('汉'.repeat(40) + '…');
  });
});

describe('noteExcerpt：首行之后的摘要（折叠单行 + 截断 80）', () => {
  it('只有一行 ⇒ 空串（列表不渲染第二行，不留空占位）', () => {
    expect(noteExcerpt('只有一行')).toBe('');
    expect(noteExcerpt('')).toBe('');
  });

  it('多行折叠成一行（空行不产出多余空格）', () => {
    expect(noteExcerpt('标题\n\n\n正文甲\n正文乙')).toBe('正文甲 正文乙');
  });

  it('超过 80 字截断；正好 80 字不截', () => {
    const exact = 'b'.repeat(80);
    expect(noteExcerpt('t\n' + exact)).toBe(exact);
    expect(noteExcerpt('t\n' + exact + 'c')).toBe(exact + '…');
  });
});

describe('noteBadgeText：题目徽标（题干摘要优先，缺摘要退回通用文案）', () => {
  it('缺摘要（空串/纯空白）退回「题目笔记」，徽标不空着', () => {
    expect(noteBadgeText('')).toBe('题目笔记');
    expect(noteBadgeText('   ')).toBe('题目笔记');
  });

  it('18 字以内整句显示；超过 18 字截断并带省略号', () => {
    expect(noteBadgeText('叉车制动系统')).toBe('题目：叉车制动系统');
    const long = '叉'.repeat(19);
    expect(noteBadgeText(long)).toBe('题目：' + '叉'.repeat(18) + '…');
  });
});

describe('isStandaloneNote：question_id 为空即独立笔记', () => {
  it('null / 0 / 负数都算独立笔记（契约漂移时零值不误判成题目笔记）', () => {
    expect(isStandaloneNote(null)).toBe(true);
    expect(isStandaloneNote(0)).toBe(true);
    expect(isStandaloneNote(-1)).toBe(true);
  });

  it('正数题目 ID 算题目笔记（题目 ID 从 1 起）', () => {
    expect(isStandaloneNote(1)).toBe(false);
    expect(isStandaloneNote(10086)).toBe(false);
  });
});

describe('noteEmptyText：空态按筛选项分档（scope 与后端参数同名）', () => {
  it('三态各有专属文案，不互相顶替', () => {
    expect(noteEmptyText('all')).toBe('还没有笔记');
    expect(noteEmptyText('question')).toBe('还没有题目笔记');
    expect(noteEmptyText('standalone')).toBe('还没有独立笔记');
  });

  it('未知 scope 兜底到「还没有笔记」（不显示裸 code）', () => {
    expect(noteEmptyText('')).toBe('还没有笔记');
    expect(noteEmptyText('future_scope')).toBe('还没有笔记');
  });
});

describe('镜像同步：notebookDisplay.uts 与本文件逐条一致', () => {
  it('标题 40 字截断在 .uts 内成文', () => {
    expect(hasTitleLock(SRC)).toBe(true);
  });

  it('摘要 80 字截断 + 折叠单行在 .uts 内成文', () => {
    expect(SRC).toContain("const lines = content.split('\\n')");
    expect(SRC).toContain("lines.slice(1).join(' ')");
    expect(SRC).toContain('rest.length > 80');
    expect(SRC).toContain('rest.substring(0, 80) + ');
  });

  it('徽标 18 字截断与通用兜底文案在 .uts 内成文', () => {
    expect(SRC).toContain("if (t.length == 0) return '题目笔记'");
    expect(SRC).toContain('t.length > 18');
    expect(SRC).toContain("return '题目：' + t");
  });

  it('独立笔记判定与空态三档在 .uts 内成文', () => {
    expect(SRC).toContain('if (questionId == null) return true');
    expect(SRC).toContain('return questionId <= 0');
    expect(SRC).toContain("if (scope == 'question') return '还没有题目笔记'");
    expect(SRC).toContain("if (scope == 'standalone') return '还没有独立笔记'");
    expect(SRC).toContain("return '还没有笔记'");
  });

  it('同步判据能判红（不是恒绿的假守护）', () => {
    const mutated = SRC.replace('first.substring(0, 40)', 'first.substring(0, 20)');
    expect(mutated).not.toBe(SRC); // 替换真的命中，否则本用例在空跑
    expect(hasTitleLock(mutated)).toBe(false);
  });
});

// 全仓 checkout 才跑得动跨端比对；单模块局部 checkout 时显式 skip（在报告里可见，不假绿）
const describeWeb = fs.existsSync(WEB_NOTEBOOK) ? describe : describe.skip;

describeWeb('跨端同源：移动端 notebookDisplay.uts vs Web Notebook.vue', () => {
  const web = fs.readFileSync(WEB_NOTEBOOK, 'utf8');

  it('三处截断长度两端逐字相同（40 / 80 / 18）', () => {
    expect(web).toContain('first.length > 40 ? first.slice(0, 40)');
    expect(web).toContain("rest.length > 80 ? rest.slice(0, 80)");
    expect(web).toContain("t.length > 18 ? '题目：' + t.slice(0, 18)");
    expect(SRC).toContain('first.length > 40');
    expect(SRC).toContain('rest.length > 80');
    expect(SRC).toContain("t.length > 18");
  });

  it('两端空态文案同源（三档一句不改）', () => {
    const block = web.slice(web.indexOf('const emptyText'), web.indexOf('})', web.indexOf('const emptyText')));
    for (const text of ['还没有题目笔记', '还没有独立笔记', '还没有笔记']) {
      expect(block).toContain(text);
      expect(SRC).toContain(text);
    }
  });
});
