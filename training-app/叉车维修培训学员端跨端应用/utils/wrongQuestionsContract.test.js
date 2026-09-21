/**
 * 错题本手术契约测试（refs #675 / T03c，parent #641）
 *
 * 沿用源码契约缝（.uvue 不可 jest import）。先例：mallPilotContract、profileContract。
 * 钉住：双组件存在与接线、600 预算机检、allowlist 无错题本条目。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
/** 豁免名单从单点读（ADR-0023 ⑧）：不再解析守护脚本源码文本取常量 */
const { allowlistPaths } = require('./contractHarness');

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

  it('提交经 submitRedo 事件带答案载荷（$event 显式断言 string[]，Kotlin 不做模板事件隐转）', () => {
    expect(page).toContain('@submit-redo="onSubmitRedo(item, $event as string[])"');
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
    // #1083 扩展：卡片另取「答案与解析」开合文案（仍是同一份 utils，零第二实现）
    expect(card).toContain("import { getTypeName, isMultiChoice, answerFoldLabel } from '../../../utils/wrongQuestionDisplay'");
    // 卡片内不得再有本地定义
    expect(card).not.toMatch(/\n\s*function getTypeName\(/);
    expect(card).not.toMatch(/\n\s*function isMultiChoice\(/);
  });
});

describe('四项补展示（#1083：图 / 答案与解析 / 上次答案 / 最近错误时间）', () => {
  const page = read('pages/profile/wrong-questions.uvue');
  const card = read('pages/profile/components/wrong-question-card.uvue');

  // ===== 取数面：api mapper 必须真把四个字段从后端 JSON 取出来 =====
  // 反向自检：光在展示层加渲染，字段恒空 ⇒ 卡片永远「无作答记录 / 不敢显图」，是坏实现的超集。
  it('api mapper 取 question.image_url / question.explanation / last_user_answer / last_wrong_at', () => {
    const api = read('api/wrongQuestion.uts');
    for (const key of [
      "questionObj['image_url']",
      "questionObj['explanation']",
      "obj['last_user_answer']",
      "obj['last_wrong_at']",
    ]) {
      expect(api).toContain(key);
    }
    // 后端顶层没有 user_answer 键（WrongQuestionDTO 无该字段）⇒ 不得再拿它充当「我上次选的答案」
    expect(api).not.toContain("last_user_answer: toStr(obj['user_answer'])");
    expect(api).toMatch(/last_user_answer: toStr\(obj\['last_user_answer'\]\)/);
  });

  it('WrongQuestionItem 契约含四个新字段（类型与 mapper 同步，缺一即编译期/运行期空值）', () => {
    const t = read('types/exam.uts');
    for (const f of ['image_url : string', 'explanation : string', 'last_user_answer : string', 'last_wrong_at : string']) {
      expect(t).toContain(f);
    }
  });

  // ===== 承载面：卡片与页面接线 =====
  it('页面把四个字段扁平透传（沿用扁平 prop 纪律，零对象 prop）', () => {
    for (const b of [':item-image-url="item.image_url"', ':item-answer="item.correct_answer"',
      ':item-explanation="item.explanation"', ':last-user-answer="item.last_user_answer"',
      ':last-wrong-at="item.last_wrong_at"']) {
      expect(page).toContain(b);
    }
    expect(page).not.toContain(':item="item"');
  });

  it('卡片四项均落到模板（不是只加了 prop 没用）', () => {
    // 1) 题干图片：无图零占位 —— v-if 与 <image> 同元素，空串时元素根本不建
    expect(card).toMatch(/<image v-if="displayImageUrl\.length > 0" class="record-question-img"/);
    expect(card).toContain(':src="displayImageUrl"');
    // 2) 答案与解析：默认收起（answerOpen 初值 false）+ 无解析空态
    expect(card).toMatch(/const answerOpen = ref<boolean>\(false\)/);
    expect(card).toMatch(/<view v-if="!expanded && answerOpen" class="answer-section">/);
    expect(card).toContain('暂无解析');
    // 3) 我上次选的答案 + 正确答案对照
    expect(card).toContain('我上次选的答案：{{ lastAnswerDisplay() }}');
    expect(card).toContain('正确答案：{{ itemAnswer.length > 0 ? itemAnswer : \'—\' }}');
    // 4) 最近错误时间
    expect(card).toMatch(/<text v-if="displayLastWrongDate\.length > 0" class="record-date record-date-last">最近 \{\{ displayLastWrongDate \}\}<\/text>/);
    expect(card).toMatch(/const displayLastWrongDate = computed<string>\(\(\) : string => \{\s*return formatDateTimeStr\(props\.lastWrongAt\)/);
  });

  it('答案与解析区在折叠态操作行之下（不抢「重做」的位置，且两态互斥不重影）', () => {
    const iActions = card.indexOf('v-if="!expanded" class="card-actions"');
    const iAnswer = card.indexOf('v-if="!expanded && answerOpen" class="answer-section"');
    expect(iActions).toBeGreaterThan(-1);
    expect(iAnswer).toBeGreaterThan(iActions);
  });

  it('开合按钮文案单一来源（utils 纯函数，收起/展开两态齐备）', () => {
    const utils = read('utils/wrongQuestionDisplay.uts');
    expect(utils).toMatch(/export function answerFoldLabel\(open : boolean\) : string \{\s*return open \? '收起答案' : '查看答案与解析'/);
    // 模板必须经本地函数包装消费（模板直调 import 函数 = Kotlin error18 invoke，守护规则 S）
    expect(card).toContain('{{ answerFoldText() }}');
    const tpl = card.slice(card.indexOf('<template>'), card.indexOf('</template>'));
    expect(tpl).not.toContain('answerFoldLabel(');
    expect(card).toMatch(/function answerFoldText\(\) : string \{\s*return answerFoldLabel\(answerOpen\.value\)/);
  });

  it('uvue 样式约束：新样式只用 class 选择器 / 无 gap / 无 CSS 变量', () => {
    const style = card.slice(card.indexOf('<style>'));
    for (const cls of ['.record-question-img', '.record-date-last', '.answer-section', '.answer-line', '.answer-explain', '.answer-empty']) {
      expect(style).toContain(cls);
    }
    expect(style).toContain('flex-direction: column;');   // uvue 无默认列向（img/块级元素会静默不撑开）
    expect(style).not.toMatch(/\bgap\s*:/);
    expect(style).not.toMatch(/var\(--/);
    expect(style).not.toMatch(/^\s*(view|text|image)\s*\{/m);  // tag 选择器不支持
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
  it('豁免面不含 wrong-questions 相关文件', () => {
    const hits = allowlistPaths().filter((p) => /wrong-questions|wrong-stats|wrong-question-card/.test(p));
    expect(hits).toEqual([]);
  });
});
