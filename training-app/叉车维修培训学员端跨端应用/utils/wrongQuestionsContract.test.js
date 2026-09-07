/**
 * 错题本手术契约测试（refs #675 / T03c，parent #641）
 *
 * 沿用源码契约缝（.uvue 不可 jest import）。先例：mallPilotContract、profileContract。
 * 钉住：双组件存在与接线、600 预算机检、allowlist 无错题本条目。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

function nonTestSources(dirRel) {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, dirRel));
  return out;
}

describe('组件存在契约（pages/profile/components/）', () => {
  it.each(['wrong-stats-card.uvue', 'wrong-question-card.uvue'])('%s 存在', (f) => {
    expect(fs.existsSync(path.join(ROOT, 'pages/profile/components', f))).toBe(true);
  });
});

describe('WrongStatsCard 接线契约', () => {
  const page = read('pages/profile/wrong-questions.uvue');
  const card = () => read('pages/profile/components/wrong-stats-card.uvue');

  it('页面使用 <WrongStatsCard :today-count> 标签', () => {
    expect(page).toMatch(/<WrongStatsCard[\s\S]*?:today-count="todayCount"/);
  });

  it('页面 import 卡片（显式 import，Q17 安置）', () => {
    expect(page).toContain("./components/wrong-stats-card.uvue");
  });

  it('卡片自拉 stats（getWrongQuestionStatsApi 在卡片内，页面不再拉 stats）', () => {
    expect(card()).toContain('getWrongQuestionStatsApi');
    expect(page).not.toContain('getWrongQuestionStatsApi');
  });

  it('卡片声明 todayCount prop 且模板消费 props 值', () => {
    const c = card();
    expect(c).toContain('todayCount?: number');
    expect(c).toMatch(/\{\{\s*(props\.todayCount|todayCount)\s*\}\}/);
  });
});

describe('WrongQuestionCard 接线契约', () => {
  const page = read('pages/profile/wrong-questions.uvue');
  const card = () => read('pages/profile/components/wrong-question-card.uvue');

  it('页面 v-for 改用 <WrongQuestionCard> 且透传 item/expanded/redo-result/last-redo-answer', () => {
    expect(page).toMatch(/<WrongQuestionCard[\s\S]*?:item="item"/);
    expect(page).toContain(':expanded="expandedId == item.question_id"');
    expect(page).toContain(':redo-result="expandedId == item.question_id ? redoResult : null"');
    expect(page).toContain(':last-redo-answer="lastRedoAnswer"');
  });

  it('页面 import 卡片（显式 import，Q17 安置）', () => {
    expect(page).toContain("./components/wrong-question-card.uvue");
  });

  it('提交经 submitRedo 事件带答案载荷（页面不再持 selectedKeys）', () => {
    expect(page).toContain('@submit-redo="onSubmitRedo(item, $event)"');
    expect(page).not.toMatch(/const selectedKeys\s*=/);
  });

  it('卡片声明 item/expanded/redoResult/lastRedoAnswer 四 props', () => {
    const c = card();
    for (const p of ['item?: WrongQuestionItem', 'expanded?: boolean', 'redoResult?:', 'lastRedoAnswer?: string']) {
      expect(c).toContain(p);
    }
  });

  it('卡片 emits 仅 toggleExpand/submitRedo/remove（无 selectOption 残留）', () => {
    const c = card();
    expect(c).toContain("defineEmits(['toggleExpand', 'submitRedo', 'remove'])");
    expect(c).not.toContain('selectOption');
  });

  it('卡片 submit 携带选中载荷（selectedKeys.value.slice()）', () => {
    expect(card()).toContain("emit('submitRedo', selectedKeys.value.slice())");
  });

  it('页面 16 个展示 helper 已搬走（formatDateStr/getTypeName/isExpanded/hasRedoResult/redoResult*/parseAnswerKeys/isSelected/isAnswerContains/getOptionClass/getOptionKeyClass/isMultiChoice）', () => {
    for (const fn of ['function formatDateStr', 'function getTypeName', 'function isMultiChoice', 'function isExpanded', 'function hasRedoResult', 'function redoResultCorrect', 'function redoResultClass', 'function redoResultLabel', 'function redoResultAnswer', 'function redoResultUserAnswer', 'function parseAnswerKeys', 'function redoResultLocalMatch', 'function isSelected', 'function isAnswerContains', 'function getOptionClass', 'function getOptionKeyClass']) {
      expect(page).not.toContain(fn);
    }
  });

  it('页面对已搬走符号零悬空使用（编译盲区锁：定义删了调用必须同步）', () => {
    // getTypeName/isMultiChoice 允许经 import 消费（utils/wrongQuestionDisplay 共享纯函数）
    const withoutImport = page.replace(/import \{[^}]*\} from '[^']*'/g, '');
    expect(withoutImport).not.toMatch(/(?<![\w$.])selectedKeys\b/);
    expect(withoutImport).not.toMatch(/(?<![\w$.])hasRedoResult\s*\(/);
    expect(withoutImport).not.toMatch(/(?<![\w$.])getOptionClass\s*\(/);
    expect(withoutImport).not.toMatch(/(?<![\w$.])isExpanded\s*\(/);
    expect(withoutImport).not.toMatch(/(?<![\w$.])parseAnswerKeys\s*\(/);
    expect(withoutImport).not.toMatch(/(?<![\w$.])redoResultLocalMatch\s*\(/);
  });

  it('展示纯函数收敛于 utils/wrongQuestionDisplay（页面与卡片 import 同一份，无第二实现）', () => {
    const card = read('pages/profile/components/wrong-question-card.uvue');
    expect(page).toContain("import { getTypeName, isMultiChoice } from '../../utils/wrongQuestionDisplay'");
    expect(card).toContain("import { getTypeName, isMultiChoice } from '../../../utils/wrongQuestionDisplay'");
    // 卡片内不得再有本地定义
    expect(card).not.toMatch(/\n\s*function getTypeName\(/);
    expect(card).not.toMatch(/\n\s*function isMultiChoice\(/);
  });
});

describe('600 行软预算机检（wrong-questions 模块文件）', () => {
  it('页面 + 双组件全部 ≤600 行', () => {
    const over = [
      'pages/profile/wrong-questions.uvue',
      'pages/profile/components/wrong-stats-card.uvue',
      'pages/profile/components/wrong-question-card.uvue',
    ].map((rel) => ({ file: rel, lines: read(rel).split('\n').length }))
      .filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });
});

describe('allowlist 不回潮（错题本域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 wrong-questions 相关文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/wrong-questions|wrong-stats|wrong-question-card/);
  });
});
