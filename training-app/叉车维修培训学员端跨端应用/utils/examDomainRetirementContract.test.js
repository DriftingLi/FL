/**
 * exam 域等级考试下线契约测试（#810，refs #662 / ADR-0007 四门之「静态守护 + 契约 + 单测」）
 *
 * 背景：#662 诊断确认等级考试域已随后端 #284/#285 在 backend 与 Web 端退役（迁移 000008
 * 已 DROP 三表），移动端从未跟随，6 条路由无后端注册、页面却全部可导航到达；api 层
 * .catch() 把 404 静默降级成空态，掩盖了三个月。#810 把移动端「跟到底」。
 *
 * 钉住的契约：
 * 1) 域文件不存在：api/levelExam.uts、level-exam-do.uvue、level-exam-result.uvue
 * 2) 全域零命中：levelExam / LevelExam / level-exam（验收标准 §1）
 * 3) pages.json 无孤儿路由，且每条注册路由都指向真实存在的页面文件
 * 4) 共享类型面：域内 6 个类型清零，ExamQuestion / QuestionOption（模拟考在用）保留
 * 5) 入口改写（维护者决策 2026-09-10）：题库页「模考大赛」格子保留、沿用 level_exam key、
 *    改指 /pages/exam/mock-exam
 * 6) 行为保持：考试中心页保留模拟考试段与「模考历史」入口；首页九宫格保留「考试中心」
 *
 * 自命中防护（ADR-0007 T05 教训：「契约零命中锁会被断言字符串自身命中」）：
 * 断言里必然出现被锁 token（如 'level-exam-do'），故全域扫描**显式排除 *.test.js** ——
 * 本文件与被锁 token 共存是设计，不是漏网。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

/** 被锁 token：无 g 标志，避免 lastIndex 状态污染 */
const RETIRED_TOKEN = /levelExam|LevelExam|level-exam/;

const SCAN_DIRS = ['api', 'pages', 'types', 'stores', 'utils', 'components', 'composables', 'constants', 'config'];
const SKIP_DIRS = new Set(['node_modules', 'unpackage', '.git', '.hbuilderx', '.opencode', '.agents', '.vscode']);

function walk(dir, out = []) {
  if (!fs.existsSync(dir)) return out;
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (SKIP_DIRS.has(e.name) || e.name.startsWith('.')) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) { walk(p, out); continue; }
    if (/\.test\.[jt]sx?$/.test(e.name)) continue; // 自命中防护，见文件头
    if (/\.(uvue|uts|vue|ts|js|json)$/.test(e.name)) out.push(p);
  }
  return out;
}

const SCANNED_FILES = [
  ...SCAN_DIRS.flatMap((d) => walk(path.join(ROOT, d))),
  path.join(ROOT, 'pages.json'),
  path.join(ROOT, 'manifest.json'),
].filter((f) => fs.existsSync(f));

describe('#810 域文件不存在（等级考试域下线）', () => {
  it.each([
    'api/levelExam.uts',
    'pages/exam/level-exam-do.uvue',
    'pages/exam/level-exam-result.uvue',
  ])('%s 已删除', (rel) => {
    expect(exists(rel)).toBe(false);
  });
});

describe('全域零命中（验收标准 §1）', () => {
  it('扫描面足够大（防止 walk 静默失效导致假绿）', () => {
    expect(SCANNED_FILES.length).toBeGreaterThan(80);
  });

  it('全工程源码/配置无 levelExam / LevelExam / level-exam 命中', () => {
    const hits = SCANNED_FILES
      .filter((f) => RETIRED_TOKEN.test(fs.readFileSync(f, 'utf8')))
      .map((f) => path.relative(ROOT, f));
    expect(hits).toEqual([]);
  });

  it('守护自身可见：level_exam（下划线形态）刻意保留，不属被锁 token', () => {
    // 维护者决策 Q2=A：题库页格子沿用既有 feature key，故下划线形态不清零
    expect(RETIRED_TOKEN.test('level_exam')).toBe(false);
  });
});

describe('pages.json 无孤儿路由（域下线不残留路径）', () => {
  const src = read('pages.json');
  const routes = [...src.matchAll(/"path"\s*:\s*"([^"]+)"/g)].map((m) => m[1]);

  it('两条等级考试路由已摘除', () => {
    expect(routes).not.toContain('pages/exam/level-exam-do');
    expect(routes).not.toContain('pages/exam/level-exam-result');
  });

  it('模拟考试链路与考试中心页仍注册', () => {
    for (const p of ['pages/exam/exam', 'pages/exam/mock-exam', 'pages/exam/mock-exam-result']) {
      expect(routes).toContain(p);
    }
  });

  it('每条注册路由都指向真实存在的页面文件（死路由回归锁）', () => {
    const dangling = routes.filter((p) =>
      !fs.existsSync(path.join(ROOT, p + '.uvue')) && !fs.existsSync(path.join(ROOT, p + '.vue')));
    expect(dangling).toEqual([]);
  });
});

describe('共享类型面：域类型清零、共享类型保留', () => {
  const types = read('types/index.uts');

  it('types/index.uts 不含 LevelExam* 类型声明', () => {
    expect(types).not.toMatch(/export\s+type\s+LevelExam\w*/);
  });

  it('模拟考在用的 ExamQuestion / QuestionOption 保留（不连带打断 mock-exam）', () => {
    expect(types).toMatch(/export\s+type\s+ExamQuestion\s*=/);
    expect(types).toMatch(/export\s+type\s+QuestionOption\s*=/);
  });
});

describe('题库页入口改写（维护者决策 2026-09-10）', () => {
  const grid = read('pages/practice/components/practice-feature-grid.uvue');
  const page = read('pages/practice/practice.uvue');

  it('宫格仍 4 格（格子保留，不改名字与视觉）', () => {
    expect([...grid.matchAll(/class="feature-item"/g)].length).toBe(4);
  });

  it('「模考大赛」格子仍在且沿用 level_exam key', () => {
    expect(grid).toContain('模考大赛');
    expect(grid).toContain("onFeatureClick('level_exam')");
  });

  it('level_exam 分支改指 /pages/exam/mock-exam', () => {
    expect(page).toMatch(/feature == 'level_exam'\)\s*\{\s*uni\.navigateTo\(\{\s*url: '\/pages\/exam\/mock-exam'\s*\}\)/);
  });

  it('practice.uvue 不再指向已删做题页', () => {
    expect(page).not.toContain('level-exam-do');
  });
});

describe('行为保持：考试中心页与首页九宫格入口', () => {
  const exam = read('pages/exam/exam.uvue');
  const menuGrid = read('pages/dashboard/components/dashboard-menu-grid.uvue');

  it('考试中心页保留模拟考试段与「模考历史」入口', () => {
    expect(exam).toContain('模拟考试');
    expect(exam).toContain('模考历史');
    expect(exam).toContain("url: '/pages/exam/mock-exam?mode=new&duration=90'");
    expect(exam).toContain("url: '/pages/profile/mock-exam-records'");
  });

  it('考试中心页不再持有等级考试取数逻辑', () => {
    expect(exam).not.toMatch(/loadAvailable|loadHistory|availableExams|historyPage/);
  });

  it('首页九宫格保留「考试中心」入口（Q1=A：摘入口=丢功能，显式否决验收标准 §1 该条）', () => {
    expect(menuGrid).toContain("title: '考试中心'");
    expect(menuGrid).toContain("path: '/pages/exam/exam'");
    expect([...menuGrid.matchAll(/\{\s*key: '/g)].length).toBe(4);
  });
});

describe('模拟考试链路零改动（本票只改入口指向）', () => {
  it.each([
    'pages/exam/mock-exam.uvue',
    'pages/exam/mock-exam-result.uvue',
    'api/mockExam.uts',
    'pages/profile/mock-exam-records.uvue',
  ])('%s 仍在且未删', (rel) => {
    expect(exists(rel)).toBe(true);
  });
});
