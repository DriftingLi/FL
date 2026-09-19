/**
 * 「我的笔记」页契约测试（#1082，parent #1078 笔记域泛化）
 *
 * .uvue 无法在 jest 中渲染，故按仓库惯例（materialsPageContract / profilePage）以源码契约钉住
 * **产物级**事实 —— 不只断言「字面量出现过」，而是断言**跨文件对得上**：
 * ① 入口 path 必须命中 pages.json 已注册路由（死链 = 判红）+ 路由页文件真的存在；
 * ② 页面 import 的 api/note 符号必须真的被该文件导出（缺导出 = 编译期才炸）；
 * ③ 三态必须是**互斥分支**且失败态排在空态之前（#816：失败不许退化成「暂无数据」）；
 * ④ 跳题 URL 的目标页必须真的读 mode + question_id（否则徽标点不动 = 静默失效，先例 #1071）；
 * ⑤ 编辑形态与 practice-do 的笔记**逐值同构**（maxlength 取自对方源码，不写死第二份）；
 * ⑥ 老写入路径（/api/questions/:id/note + usePracticeNotes）逐字未动。
 *
 * 自命中防护：断言里必然出现被锁 token，故按文件读源码做包含判断，不做全域零引用扫描。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const PAGE_REL = 'pages/profile/notebook.uvue';
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const page = read(PAGE_REL);
const pageScript = page.slice(page.indexOf('<script'), page.lastIndexOf('</script>'));
const pageStyle = page.slice(page.indexOf('<style'), page.lastIndexOf('</style>'));
const pageTemplate = page.slice(page.indexOf('<template>'), page.lastIndexOf('</template>'));
const api = read('api/note.uts');

/** 模板里被绑定的事件处理函数名（@click / @click.stop / 弹窗壳的 @close / @confirm；带实参的取函数名） */
function eventHandlers(tpl) {
  const names = new Set();
  for (const m of tpl.matchAll(/@(?:click(?:\.stop)?|close|confirm)="([A-Za-z_$][\w$]*)/g)) names.add(m[1]);
  return [...names];
}

describe('#1082 入口接线：notebook 占位已接上已注册路由（不是死链）', () => {
  const pagesConf = JSON.parse(read('pages.json'));
  const registered = new Set(pagesConf.pages.map((p) => p.path));

  it('新页已注册且在盘上', () => {
    expect(registered.has('pages/profile/notebook')).toBe(true);
    expect(exists(PAGE_REL)).toBe(true);
  });

  it('profile 工具宫格 notebook 条目 available=true 且 path 指向已注册路由', () => {
    const profile = read('pages/profile/profile.uvue');
    const m = /key: 'notebook'[^}]*title: '([^']+)'[^}]*path: '([^']*)'[^}]*available: (true|false)/.exec(profile);
    expect(m).not.toBeNull();
    expect(m[3]).toBe('true');
    expect(m[2]).toBe('/pages/profile/notebook');
    expect(registered.has(m[2].slice(1))).toBe(true);
  });

  it('「笔记本」仍是原型里的文案与图标（接线不改原型条目）', () => {
    expect(read('pages/profile/profile.uvue')).toContain("key: 'notebook', title: '笔记本', icon: '📔'");
  });
});

describe('#1082 数据面：走 api/note，符号必须真的被导出', () => {
  it('页面只从 api/note 取数，不直发 request', () => {
    expect(pageScript).toMatch(/from '\.\.\/\.\.\/api\/note'/);
    expect(pageScript).not.toMatch(/from '[^']*api\/request/);
  });

  it('页面用到的 api/note 导出符号都已在该文件 export（缺导出 = 编译期才炸）', () => {
    const imported = /import\s*\{([^}]*)\}\s*from\s*'\.\.\/\.\.\/api\/note'/.exec(pageScript)[1];
    const names = imported.split(',').map((s) => s.trim()).filter(Boolean);
    expect(names.length).toBeGreaterThanOrEqual(7);
    for (const n of names) {
      expect({ name: n, exported: api.includes('export function ' + n) || api.includes('export const ' + n) })
        .toEqual({ name: n, exported: true });
    }
  });

  it('四个端点与 scope 三态都在 api 层成文（页面不自己拼 URL）', () => {
    expect(api).toContain("getMapped<NoteListResult>('/notes'");
    expect(api).toContain("postMapped<NoteItem>('/notes'");
    expect(api).toContain("put('/notes/' + id.toString()");
    expect(api).toContain("del('/notes/' + id.toString()");
    expect(api).toContain("export const NOTE_SCOPE_ALL = 'all'");
    expect(api).toContain("export const NOTE_SCOPE_QUESTION = 'question'");
    expect(api).toContain("export const NOTE_SCOPE_STANDALONE = 'standalone'");
    expect(pageTemplate).not.toMatch(/\/notes(\?|')/);
  });

  it('question_id 走 toNumberOrNull（NULL 是独立笔记的语义，不许压成 0）', () => {
    expect(api).toContain("question_id: toNumberOrNull(obj['question_id'])");
  });
});

describe('#1082 三态：失败 / 加载 / 空互斥，且失败优先于空', () => {
  it('三态是互斥分支（v-if + v-else-if 链）', () => {
    expect(pageTemplate).toContain('v-if="loadError.length > 0"');
    expect(pageTemplate).toContain('v-else-if="loading"');
    expect(pageTemplate).toContain('v-else-if="items.length == 0"');
  });

  it('失败态排在空态之前（请求失败不许退化成「暂无笔记」）', () => {
    const err = pageTemplate.indexOf('v-if="loadError.length > 0"');
    const empty = pageTemplate.indexOf('v-else-if="items.length == 0"');
    expect(err).toBeGreaterThan(-1);
    expect(empty).toBeGreaterThan(err);
  });

  it('失败态给可点重试位，且重试真的重新取数（不是空壳按钮）', () => {
    expect(pageTemplate).toContain('@click="onRetry"');
    expect(pageScript).toMatch(/function onRetry\(\)\s*:\s*void\s*\{\s*loadNotes\(\)/);
  });

  it('翻页失败只在页脚补重试位、不清屏（已加载部分不丢）', () => {
    expect(pageTemplate).toContain('v-else-if="moreError"');
    expect(pageTemplate).toContain('@click="onRetryMore"');
    expect(pageScript).toMatch(/function onRetryMore\(\)\s*:\s*void\s*\{\s*onLoadMore\(\)/);
    // 翻页失败分支里不得把 items 清空
    const catchBlock = pageScript.slice(pageScript.indexOf('load more failed'));
    expect(catchBlock.slice(0, 200)).not.toContain('items.value = []');
  });

  it('空态文案按筛选项分档（三档都有，且取自 notebookDisplay 唯一实现）', () => {
    expect(pageScript).toMatch(/noteEmptyText\(scope\.value\)/);
    const util = read('utils/notebookDisplay.uts');
    for (const t of ['还没有笔记', '还没有题目笔记', '还没有独立笔记']) {
      expect(util).toContain(t);
    }
  });
});

describe('#1082 列表：倒序分页 + 防重入 + 切段重取', () => {
  it('触底翻页走 scrolltolower，且分页参数进 api', () => {
    expect(pageTemplate).toContain('@scrolltolower="onLoadMore"');
    expect(pageScript).toContain('listNotesApi(scope.value, next, pageSize)');
    expect(pageScript).toContain('listNotesApi(scope.value, 1, pageSize)');
  });

  it('防重入：加载中 / 翻页中 / 已到底都不再发请求', () => {
    expect(pageScript).toMatch(/if \(loading\.value \|\| loadingMore\.value \|\| noMore\.value\) return/);
    expect(pageScript).toMatch(/if \(loading\.value\) return/);
  });

  it('切段清空旧筛选的残余行再重取（不混两段的数据）', () => {
    const fn = pageScript.slice(pageScript.indexOf('function onScopeClick'));
    const body = fn.slice(0, fn.indexOf('\n    }'));
    expect(body).toContain('items.value = []');
    expect(body).toContain('loadNotes()');
    expect(body).toContain('if (scope.value == val) return');
  });

  it('分段控件三档与后端 scope 同名，且不使用下拉/筛选栏替代', () => {
    expect(pageScript).toContain("label: '全部', value: NOTE_SCOPE_ALL");
    expect(pageScript).toContain("label: '题目笔记', value: NOTE_SCOPE_QUESTION");
    expect(pageScript).toContain("label: '独立笔记', value: NOTE_SCOPE_STANDALONE");
    expect(pageTemplate).toContain('v-for="(opt, idx) in scopeOptions"');
  });

  it('编辑器复用模块内 InfoDialog 弹窗壳（表单真值源留在页面，先例 personal-info）', () => {
    expect(pageScript).toContain("import InfoDialog from './components/info-dialog.uvue'");
    expect(pageTemplate).toMatch(/<InfoDialog[\s\S]*?<\/InfoDialog>/);
    // 真值源仍在页面：draft / saving 由页面持有
    expect(pageScript).toContain("const draft = ref<string>('')");
    expect(pageScript).toContain('const saving = ref<boolean>(false)');
  });
});

describe('#1082 卡片：首行标题 + 摘要 + 时间 + 可跳题的题目徽标', () => {
  it('卡片用 notebookDisplay 的三件套（首行标题 / 摘要 / 徽标），不自己重写截断', () => {
    expect(pageScript).toMatch(/noteTitle\(content\)/);
    expect(pageScript).toMatch(/noteExcerpt\(content\)/);
    expect(pageScript).toMatch(/noteBadgeText\(questionContent\)/);
    expect(pageTemplate).toContain('titleText(item.content)');
    expect(pageTemplate).toContain('excerptText(item.content)');
    expect(pageTemplate).toContain('badgeText(item.question_content)');
  });

  it('独立笔记走纯标签、题目笔记走可点徽标（二选一，不是同一块）', () => {
    expect(pageTemplate).toContain('v-if="isStandalone(item)"');
    expect(pageTemplate).toMatch(/v-else class="note-tag note-tag-question"/);
  });

  it('徽标点击真的发起到 practice-do 的跳转，且带 question_id', () => {
    expect(pageTemplate).toContain('@click="onJumpQuestion(item)"');
    expect(pageScript).toContain("'/pages/practice/practice-do?mode=single&question_id=' + qid!.toString()");
  });

  it('跳题目标页真的读 mode 与 question_id（传了不读 = 静默失效，先例 #1071）', () => {
    const target = read('pages/practice/practice-do.uvue');
    const onLoadAt = target.indexOf('onLoad((options');
    expect(onLoadAt).toBeGreaterThan(-1);
    const body = target.slice(onLoadAt, onLoadAt + 600);
    expect(body).toContain("options['mode']");
    expect(body).toContain("options['question_id']");
  });

  it('没有 question_id 时不跳（独立笔记/脏数据不产生死跳转）', () => {
    const fn = pageScript.slice(pageScript.indexOf('function onJumpQuestion'));
    const body = fn.slice(0, fn.indexOf('\n    }'));
    expect(body).toContain('if (qid == null || qid <= 0) return');
  });

  it('时间取 updated_at（列表按更新时间倒序的口径）', () => {
    expect(pageTemplate).toContain('displayTime(item.updated_at)');
  });
});

describe('#1082 编辑：新建独立笔记 + 编辑同构 practice-do（纯文本 2000）', () => {
  it('新建走 createNoteApi、编辑走 updateNoteApi（按 editingId 分流）', () => {
    const fn = pageScript.slice(pageScript.indexOf('async function onSave'));
    const body = fn.slice(0, fn.indexOf('\n    }'));
    expect(body).toContain('updateNoteApi(editingId.value, content)');
    expect(body).toContain('createNoteApi(content)');
  });

  it('textarea 上限与 practice-do 的笔记输入框**逐值同构**（从对方源码取，不写死第二份）', () => {
    const practiceDo = read('pages/practice/practice-do.uvue');
    const m = /class="note-textarea"[^>]*maxlength="(\d+)"/.exec(practiceDo);
    expect(m).not.toBeNull();
    expect(pageTemplate).toContain('maxlength="' + m[1] + '"');
    expect(m[1]).toBe('2000');
  });

  it('空内容不许提交（后端 400 前置拦在端上）', () => {
    expect(pageScript).toContain("content.trim().length == 0");
    expect(pageScript).toContain("'笔记内容不能为空'");
  });

  it('保存失败保持浮层打开（输入不丢），成功才关', () => {
    const fn = pageScript.slice(pageScript.indexOf('async function onSave'));
    const body = fn.slice(0, fn.indexOf('\n    }'));
    const catchAt = body.indexOf('save failed');
    expect(catchAt).toBeGreaterThan(-1);
    // catch 段内不得关闭浮层
    expect(body.slice(catchAt)).not.toContain('editorVisible.value = false');
  });

  it('删除走确认框 + removeNoteApi', () => {
    expect(pageScript).toContain('uni.showModal(');
    expect(pageScript).toContain('removeNoteApi(item.id)');
    expect(pageScript).toContain("确定删除这条笔记吗？");
  });

  it('模板里每个事件处理函数都在 script 内定义（无死绑定）', () => {
    const handlers = eventHandlers(pageTemplate);
    expect(handlers.length).toBeGreaterThanOrEqual(9);
    const missing = handlers.filter((h) => !new RegExp('function ' + h + '\\(').test(pageScript));
    expect(missing).toEqual([]);
  });
});

describe('#1082 老路径不受影响：/api/questions/:id/note 与 practice-do 笔记写入', () => {
  it('题目维度三个端点逐字仍在 api/questionInteraction.uts', () => {
    const qi = read('api/questionInteraction.uts');
    expect(qi).toContain('get(`/questions/${questionId}/note`)');
    expect(qi).toContain('put(`/questions/${questionId}/note`');
    expect(qi).toContain('del(`/questions/${questionId}/note`)');
  });

  it('usePracticeNotes 仍从老 api 取三件套（写入路径未被本票改写）', () => {
    const uts = read('pages/practice/composables/usePracticeNotes.uts');
    expect(uts).toContain("import { getNoteApi, saveNoteApi, deleteNoteApi } from '../../../api/questionInteraction'");
    expect(uts).toContain('saveNoteApi(questionId, noteContent.value)');
  });

  it('新页不碰 practice-do 与 usePracticeNotes（同一资源两条路径各自成立）', () => {
    expect(api).toContain('api/questionInteraction.uts');
    expect(api).toContain('逐字不变');
    expect(page).not.toContain('usePracticeNotes');
  });
});

describe('#1082 判据自检：关键断言能判红（不是恒绿的假守护）', () => {
  const pagesConf = JSON.parse(read('pages.json'));
  const registered = new Set(pagesConf.pages.map((p) => p.path));

  it('路由登记判据能判红：没注册的 path 不会被认作已注册', () => {
    expect(registered.has('pages/profile/notebook')).toBe(true);
    expect(registered.has('pages/profile/notebook-typo')).toBe(false);
  });

  it('跳题键判据能判红：目标页读 question_id，但不读 camelCase questionId', () => {
    const target = read('pages/practice/practice-do.uvue');
    const body = target.slice(target.indexOf('onLoad((options'), target.indexOf('onLoad((options') + 600);
    expect(body).toContain("options['question_id']");
    expect(body).not.toContain("options['questionId']");
  });

  it('三态顺序判据有区分度：失败态与空态锚点互不相同（否则顺序断言恒空跑）', () => {
    const err = pageTemplate.indexOf('v-if="loadError.length > 0"');
    const empty = pageTemplate.indexOf('v-else-if="items.length == 0"');
    expect(err).toBeGreaterThan(-1);
    expect(empty).toBeGreaterThan(-1);
    expect(err).not.toBe(empty);
  });
});

describe('#1082 uvue 兼容性（pages/profile/notebook.uvue 单编译单元）', () => {
  it('不使用 uvue 不支持的 CSS 属性与单位', () => {
    expect(/(^|[;{\s])gap\s*:/.test(pageStyle)).toBe(false);
    expect(pageStyle.includes('row-gap')).toBe(false);
    expect(pageStyle.includes('column-gap')).toBe(false);
    expect(pageStyle.includes('var(--')).toBe(false);
    expect(pageStyle.includes('currentColor')).toBe(false);
    expect(pageStyle.includes('calc(')).toBe(false);
    expect(/\d+(vh|vw)\b/.test(pageStyle)).toBe(false);
    expect(pageStyle.includes('display: grid')).toBe(false);
    expect(/transition|animation/.test(pageStyle)).toBe(false);
    expect(pageStyle.includes('max-height')).toBe(false);
  });

  it('只使用 class 选择器（无 tag / id / 伪类 / 属性选择器）', () => {
    expect(/:(hover|active|focus|first-child|last-child|nth-child|before|after)/.test(pageStyle)).toBe(false);
    expect(/^\s*(view|text|image|scroll-view|button|textarea)\s*\{/m.test(pageStyle)).toBe(false);
    expect(/^\s*#[\w-]+\s*\{/m.test(pageStyle)).toBe(false);
    expect(/^[^.@}\s][\w-]*\s*\{/m.test(pageStyle)).toBe(false);
  });

  it('模板与样式中的 class 一一对应（无死样式、无裸 class）', () => {
    const script = pageScript;
    const used = new Set();
    for (const m of pageTemplate.matchAll(/(?<!:)class="([^"]+)"/g)) {
      m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
    }
    for (const m of pageTemplate.matchAll(/:class="([^"]+)"/g)) {
      const expr = m[1];
      for (const q of expr.matchAll(/'?([A-Za-z][A-Za-z0-9_-]*)'?\s*:/g)) used.add(q[1]);
      for (const q of expr.matchAll(/'([^']+)'/g)) {
        const after = expr.slice(q.index + q[0].length);
        const before = expr.slice(0, q.index).trimEnd();
        if (q[1].length > 0 && !after.trimStart().startsWith(':') && !/(==|!=|===|!==)$/.test(before)) used.add(q[1]);
      }
    }
    expect(script.length).toBeGreaterThan(0);
    const defined = new Set();
    for (const m of pageStyle.matchAll(/\.([A-Za-z][A-Za-z0-9_-]*)/g)) defined.add(m[1]);

    expect([...used].filter((c) => !defined.has(c))).toEqual([]);
    expect([...defined].filter((c) => !used.has(c))).toEqual([]);
  });

  it('样式块声明 lang="scss"（本页无 SCSS 变量，但遵守 .uvue 约定）', () => {
    expect(page).toMatch(/<style lang="scss">/);
  });
});
