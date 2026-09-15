/**
 * 全局搜索升级契约测试（#979：M1–M6 + 章节分区）
 *
 * 钉住的是「行为在源码里的落点」——页面与 api 的接线、以及几条**曾经真实出错**的口径：
 * - M1 落点：五分区都必须有可打开的落点，且不得回退成 toast 死链
 * - M2 入口：dashboard 顶部栏必须有搜索入口
 * - M3 封装与 token：搜索页只用既有 token；被引用的组件必须声明 lang="scss"
 *   且**变量名在 uni.scss 里真实存在**（app-chip/app-empty-state 曾用 $text-placeholder
 *   这类不存在的名字 ⇒ 声明被静默丢弃）
 * - M4 呈现：snippet 投影与高亮、命中位置标注、历史单条删除
 * - M5 竞态与丢页：请求序号 + 只在成功时推进页码
 * - M6 口径：搜索是公开路由且**没有** OptionalAuth（router.go:89-91）⇒ 必须显式传 credential_id
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const SEARCH_PAGE = read('pages/search/search.uvue');
const SEARCH_API = read('api/search.uts');
const SEARCH_TYPES = read('types/search.uts');
const SEARCH_DISPLAY = read('utils/searchDisplay.uts');
const DASHBOARD = read('pages/dashboard/dashboard.uvue');
const PRACTICE_DO = read('pages/practice/practice-do.uvue');
const PRACTICE_SESSION = read('pages/practice/composables/usePracticeSession.uts');
const PRACTICE_API = read('api/practice.uts');

/** 取 <style> 块（含 lang 属性行） */
function styleBlock(src) {
  const start = src.indexOf('<style');
  return start < 0 ? '' : src.slice(start);
}

/** 取源码里引用的 SCSS 变量名（去重） */
function scssVars(src) {
  return [...new Set((styleBlock(src).match(/\$[a-z][a-z0-9-]*/g) || []))];
}

describe('M1 落点：每条结果都能打开（ADR-0049 决策 2）', () => {
  it('页面不再有「开发中」toast 死链', () => {
    expect(SEARCH_PAGE).not.toContain('资料详情开发中');
    expect(SEARCH_PAGE).not.toContain('题目详情开发中');
    expect(SEARCH_PAGE).not.toContain('页面开发中');
  });

  it('落点收敛到 utils/searchDisplay.searchItemPath 单点', () => {
    expect(SEARCH_PAGE).toContain("import { splitHighlight, searchItemPath");
    expect(SEARCH_PAGE).toContain('const url = searchItemPath(item)');
    expect(SEARCH_PAGE).toContain('uni.navigateTo({ url: url })');
  });

  it('章节分区被真正消费（此前 SearchResult 类型里没有 chapters ⇒ 全部 tab 静默丢掉整个分区）', () => {
    expect(SEARCH_TYPES).toContain('chapters : SearchCategoryResult');
    expect(SEARCH_PAGE).toContain("{ label: '章节', value: 'chapter' }");
    expect(SEARCH_PAGE).toContain("sectionLabel('chapter')");
    expect(SEARCH_API).toContain("data['chapters']");
  });
});

describe('M2 入口：dashboard 顶部栏有搜索', () => {
  it('dashboard 有搜索按钮且指向搜索页', () => {
    expect(DASHBOARD).toContain('@click="onSearchClick"');
    expect(DASHBOARD).toContain("uni.navigateTo({ url: '/pages/search/search' })");
  });

  it('入口不止一处：题库页与商城页的既有入口未被破坏', () => {
    expect(read('pages/practice/practice.uvue')).toContain("'/pages/search/search'");
    expect(read('pages/mall/mall.uvue')).toContain("'/pages/search/search'");
  });
});

describe('M3 封装层与 token 化', () => {
  it('搜索页使用既有组件（app-chip / app-empty-state）', () => {
    expect(SEARCH_PAGE).toContain("from '../../components/app-chip/app-chip.uvue'");
    expect(SEARCH_PAGE).toContain("from '../../components/app-empty-state/app-empty-state.uvue'");
    expect(SEARCH_PAGE).toContain('<AppChip ');
    expect(SEARCH_PAGE).toContain('<AppEmptyState ');
  });

  it('搜索页样式块声明 lang="scss"（否则 SCSS 变量不会被预处理）', () => {
    expect(styleBlock(SEARCH_PAGE)).toMatch(/<style lang="scss">/);
  });

  it('已 token 化的色值不再以裸 hex 出现', () => {
    const style = styleBlock(SEARCH_PAGE);
    for (const hex of ['#2979ff', '#333333', '#666666', '#999999', '#ffffff', '#f0f0f0', '#f5f5f5']) {
      expect(style.toLowerCase()).not.toContain(hex);
    }
  });

  const TOKEN_FILES = {
    'pages/search/search.uvue': SEARCH_PAGE,
    'components/app-chip/app-chip.uvue': read('components/app-chip/app-chip.uvue'),
    'components/app-empty-state/app-empty-state.uvue': read('components/app-empty-state/app-empty-state.uvue'),
  };

  it.each(Object.entries(TOKEN_FILES))('%s 引用的 $变量在 uni.scss 里都存在', (rel, src) => {
    const uni = read('uni.scss');
    const missing = scssVars(src).filter((v) => !uni.includes(v + ':'));
    expect(missing).toEqual([]);
  });

  it.each(Object.entries(TOKEN_FILES))('%s 的样式块声明 lang="scss"', (rel, src) => {
    expect(styleBlock(src)).toMatch(/<style lang="scss">/);
  });
});

describe('M4 呈现：投影 + 高亮 + 命中标注 + 历史单条删除', () => {
  it('标题与片段都走高亮切段，命中片段用 item-hl 加粗描色', () => {
    expect(SEARCH_PAGE).toContain('itemTitleSegments(item)');
    expect(SEARCH_PAGE).toContain('itemSnippetSegments(item)');
    expect(SEARCH_PAGE).toContain(":class=\"seg.hit ? 'item-hl' : 'item-plain'\"");
    expect(styleBlock(SEARCH_PAGE)).toContain('.item-hl');
  });

  it('片段投影走 utils/searchDisplay.displaySnippet（snippet 优先、summary 兜底）', () => {
    expect(SEARCH_PAGE).toContain('return displaySnippet(item)');
    expect(SEARCH_DISPLAY).toContain('export function displaySnippet');
  });

  it('命中在回复时标注（否则点进去找不到关键词）', () => {
    expect(SEARCH_PAGE).toContain('itemHitLabel(item)');
    expect(SEARCH_DISPLAY).toContain("if (hitField == 'reply') return HIT_REPLY_LABEL");
  });

  it('结果携带封面时渲染缩略图（cover 此前完全未消费）', () => {
    expect(SEARCH_PAGE).toContain('v-if="item.cover.length > 0"');
    expect(styleBlock(SEARCH_PAGE)).toContain('.item-cover');
  });

  it('搜索历史与 Web 同口径：上限 10 / 去重前置 / 单条删除 / 清空', () => {
    expect(SEARCH_PAGE).toContain('const HISTORY_MAX = 10');
    expect(SEARCH_PAGE).toContain("searchHistory.value.filter((s : string) : boolean => s != kw)");
    expect(SEARCH_PAGE).toContain('function onRemoveHistory(kw : string) : void');
    expect(SEARCH_PAGE).toContain('@click="onRemoveHistory(kw)"');
    expect(SEARCH_PAGE).toContain('function onClearHistory() : void');
  });
});

describe('M5 竞态与丢页', () => {
  it('请求序号守卫：旧响应回来直接丢弃', () => {
    expect(SEARCH_PAGE).toContain('let searchSeq = 0');
    expect(SEARCH_PAGE).toContain('const seq = searchSeq + 1');
    expect(SEARCH_PAGE).toContain('if (seq != searchSeq) return');
  });

  it('页码只在请求成功后才推进（不再 page++ 后请求，失败即静默丢一页）', () => {
    expect(SEARCH_PAGE).not.toMatch(/page\.value\s*\+\+/);
    expect(SEARCH_PAGE).toContain('page.value = requestPage');
    const from = SEARCH_PAGE.indexOf('function onLoadMore()');
    const body = SEARCH_PAGE.slice(from, SEARCH_PAGE.indexOf('\n\t}', from));
    expect(body).toContain('runSearch(page.value + 1, true)');
    expect(body).not.toContain('page.value =');
  });

  it('loading 只由最新一次请求收口（旧响应回来不改 loading）', () => {
    expect(SEARCH_PAGE).toContain('if (seq == searchSeq) loading.value = false');
  });
});

describe('M6 证件分区：公开路由必须显式传 credential_id', () => {
  it('api/search.uts 下发 credential_id，且只在 >0 时下发', () => {
    expect(SEARCH_API).toContain("params['credential_id'] = credentialId.toString()");
    expect(SEARCH_API).toContain('if (credentialId > 0)');
  });

  it('两个出口都接受并透传证件', () => {
    expect(SEARCH_API).toMatch(/export function searchAllApi\(keyword : string, credentialId : number = 0\)/);
    expect(SEARCH_API).toMatch(
      /export function searchByTypeApi\(keyword : string, type : string, page : number = 1, pageSize : number = 20, credentialId : number = 0\)/,
    );
  });

  it('页面解析当前证件并随请求下发（含首次搜索前的就绪等待）', () => {
    expect(SEARCH_PAGE).toContain('getCurrentCredentialApi');
    expect(SEARCH_PAGE).toContain('function ensureCredential() : Promise<void>');
    expect(SEARCH_PAGE).toContain('await ensureCredential()');
    expect(SEARCH_PAGE).toContain('searchAllApi(kw, credentialId.value)');
    expect(SEARCH_PAGE).toContain('searchByTypeApi(kw, tab.value, requestPage, pageSize.value, credentialId.value)');
  });

  it('onShow 重新解析证件（跟随证件切换，不吃上一个证件的分区结果）', () => {
    expect(SEARCH_PAGE).toContain('credentialRefresh = loadCredential()');
    expect(SEARCH_PAGE).toMatch(/onShow\(\(\) : void => \{/);
  });
});

describe('契约字段：移动端不再漏消费后端新增字段', () => {
  it('types/search.uts 登记 snippet / hit_field / parent_id', () => {
    expect(SEARCH_TYPES).toContain('snippet : string');
    expect(SEARCH_TYPES).toContain('hit_field : string');
    expect(SEARCH_TYPES).toContain('parent_id : number');
  });

  it('api/search.uts 映射这三个字段（漏映射 = 静默丢字段）', () => {
    expect(SEARCH_API).toContain("snippet: toStr(obj['snippet'])");
    expect(SEARCH_API).toContain("hit_field: toStr(obj['hit_field'])");
    expect(SEARCH_API).toContain("parent_id: toNumber(obj['parent_id'])");
  });
});

describe('题目落点：复用练习单题栈（ADR-0049 决策 4 / Web #983 同构）', () => {
  it('api/practice.uts 有按 id 取题，走 mapper-callback 出口', () => {
    expect(PRACTICE_API).toContain('export function getQuestionByIdApi(questionId : number) : Promise<Question>');
    expect(PRACTICE_API).toContain("getMapped<Question>('/question-bank/questions/' + questionId.toString()");
  });

  it('composable 有 single 模式：取单题、提交按 free 记账、不落进度', () => {
    expect(PRACTICE_SESSION).toContain("if (mode == 'single')");
    expect(PRACTICE_SESSION).toContain('const one = await getQuestionByIdApi(questionId)');
    expect(PRACTICE_SESSION).toContain("practice_type: mode == 'single' ? 'free' : mode");
    expect(PRACTICE_SESSION).toContain("if (mode != 'free' && mode != 'single')");
  });

  it('页面读 question_id 并切到单题形态：无答题卡 / 无交卷 / 有回题库出口', () => {
    expect(PRACTICE_DO).toContain("const qidParam = options['question_id']");
    expect(PRACTICE_DO).toContain("mode.value = 'single'");
    expect(PRACTICE_DO).toContain('v-if="session.showAnswerCard.value && !isSingle"');
    expect(PRACTICE_DO).toContain('v-if="session.showSummary.value && !isSingle"');
    expect(PRACTICE_DO).toContain('@click="onGoPractice"');
  });
});

describe('搜索页与 api 的零直发请求纪律', () => {
  it('页面不直接 uni.request', () => {
    expect(SEARCH_PAGE).not.toMatch(/uni\.request\s*\(/);
  });

  it('页面不自己拼 query 串（query 序列化单点在 request 层）', () => {
    expect(SEARCH_PAGE).not.toContain('encodeURIComponent');
  });
});
