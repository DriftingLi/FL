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

  it('卡片纯展示（零取数/零 emit/零 expose）——stats 归页面所有', () => {
    const c = card();
    expect(c).not.toContain('getWrongQuestionStatsApi');
    expect(c).not.toMatch(/defineEmits|defineExpose/);
    // 页面自拉 stats（卡片自拉 + ref<any> 桥接是 error18 根因，#687 已废）
    expect(page).toContain('getWrongQuestionStatsApi');
    expect(page).toContain('async function loadStats');
    expect(page).not.toMatch(/ref<\s*any/);
  });

  it('卡片声明 statsTotal/todayCount 两个展示 prop，模板消费', () => {
    const c = card();
    expect(c).toContain('statsTotal?: number');
    expect(c).toContain('todayCount?: number');
    expect(c).toMatch(/\{\{\s*statsTotal\s*\}\}/);
    expect(c).toMatch(/\{\{\s*todayCount\s*\}\}/);
  });

  it('页面 stats 声明先于消费它的 groups computed（Kotlin 局部无前向引用）', () => {
    const statsIdx = page.indexOf('const stats = ref<WrongQuestionStats>');
    const groupsIdx = page.indexOf('const groups = computed<GroupRow[]>');
    expect(statsIdx).toBeGreaterThan(-1);
    expect(groupsIdx).toBeGreaterThan(-1);
    expect(statsIdx).toBeLessThan(groupsIdx);
  });
});

describe('WrongQuestionCard 接线契约', () => {
  const page = read('pages/profile/wrong-questions.uvue');
  const card = () => read('pages/profile/components/wrong-question-card.uvue');

  it('页面 v-for 改用 <WrongQuestionCard> 且扁平透传（item 六字段/expanded/redo-result/last-redo-answer）', () => {
    expect(page).toMatch(/<WrongQuestionCard v-for="\(item, idx\) in records"/);
    for (const b of [':item-type="item.type"', ':item-title="item.title"', ':item-created-date="item.created_at"',
      ':wrong-count="item.wrong_count"', ':favorited="item.favorited"', ':options="item.options"',
      ':expanded="expandedId == item.question_id"', ':last-redo-answer="lastRedoAnswer"',
      ':redo-result="expandedId == item.question_id ? redoResult : null"']) {
      expect(page).toContain(b);
    }
    // 对象 prop 直传已禁（可选对象 prop 成员直读 = Kotlin error18，#687 复发根因）
    expect(page).not.toContain(':item="item"');
  });

  it('页面 import 卡片（显式 import，Q17 安置）', () => {
    expect(page).toContain("./components/wrong-question-card.uvue");
  });

  it('提交经 submitRedo 事件带答案载荷（页面不再持 selectedKeys）', () => {
    expect(page).toContain('@submit-redo="onSubmitRedo(item, $event)"');
    expect(page).not.toMatch(/const selectedKeys\s*=/);
  });

  it('卡片声明扁平 props（六原始字段 + expanded/redoResult/lastRedoAnswer），无 item 对象 prop', () => {
    const c = card();
    for (const p of ['itemType?: string', 'itemTitle?: string', 'itemCreatedDate?: string', 'wrongCount?: number',
      'favorited?: boolean', 'options?: QuestionOption[]', 'expanded?: boolean', 'redoResult?:', 'lastRedoAnswer?: string']) {
      expect(c).toContain(p);
    }
    // 可选对象 prop 成员直读 = Kotlin error18（#687 复发的真根因），扁平化后不得回潮
    expect(c).not.toMatch(/item\?:\s*WrongQuestionItem/);
  });

  it('卡片编译雷修复形态（#684 引入 error18，#687 复锁定真根因：可选对象 prop 直读）', () => {
    const c = card();
    // 无 as unknown as（上轮误判的病灶，仍是禁项）
    expect(c).not.toMatch(/as\s+unknown\s+as/);
    // display 值必须是 computed（模板 {{ x }} 裸引用；function 不自动调用 → 静默渲染源码）
    expect(c).toMatch(/const displayTypeName = computed/);
    expect(c).toMatch(/const displayDate = computed/);
    // props.redoResult 成员直读清零（Kotlin 对 props getter 无智能转换，须局部 val）
    expect(c).not.toMatch(/props\.redoResult\./);
    // 无任何 props.item. 成员直读
    expect(c).not.toMatch(/props\.item\./);
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
