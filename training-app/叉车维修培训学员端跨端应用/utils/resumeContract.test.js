/**
 * resume 模块手术契约测试（T09，parent #647 / ADR-0007）
 *
 * 钉住 resume（在线简历编辑）手术交付的契约：
 * 1) 600 行软预算：pages/resume/** 全部源文件 + api/resume.uts ≤600，**新建 composable 与 section 组件同样计入**
 *    （T07 维护者裁定 B 硬化口径：防「把 922 行页面挪成 900 行 composable」的假达标）
 * 2) 模块目录 ≤2 层
 * 3) 模块必需源文件清单完整（删任一文件即红，扫描面不靠数量下限兜底）
 * 4) composable 接线：resume-edit.uvue 以显式 import 使用模块私有 composable，且 composable 有显式结果类型
 * 5) 组件接线零孤儿：页面 import 的组件文件必须存在，组件文件必须被页面引用（#779 回归教训）
 * 6) 页面 ↔ 组件接口对账：prop / 事件双向无孤儿，`update:` 前缀按 kebab 归一（本票唯一新增接口面）
 * 7) allowlist 不回潮：resume 域文件不得出现在 GUARD_ALLOWLIST
 * 8) 零直发请求：pages/resume/** 不直接 uni.request；请求只经 api/resume.uts
 * 9) 域 api 收紧：5 个 DTO 出口经 mapper-callback 家族，3 个裸透传在白名单内（multipart 上传 ×2 + void 删除）
 * 10) 幻影路由锁（#662 口径）：api 层每条路由都落在后端已注册清单内（job_card.go / resume_view.go / training_catalog.go）
 * 11) 删除禁区「resume 不用删」的行为保持点：草稿回填 / 服务端回显 / 完善度 12 项 / 三处 actionSheet /
 *     教育·工作经历增删 / 保存四条校验 / 本地草稿双 key / 保存成功回列表 / 保存栏双入口逐项仍在
 * 12) 拆分判据锁（T09 新增）：页面壳层不得自持编辑态 ref；教育·工作列表（v-model 风险区）留在壳层
 * 13) 零消费出口白名单：setVisibility / getViewStats / uploadWorkPhoto 是既有零消费出口，保留决策入锁
 * 14) #1204 notfound 单一出口：写端（request.uts 的 404 消息前缀）→ 判定出口（resume.uts 的
 *     isResumeNotFound）→ 两个消费方（总览页 / 编辑页 composable）不再各抄一份 `obj['kind']` 死判据
 *
 * 本套件是**接线守护**（源码文本 + 结构对账，不构成 ③ 门的行为证据）：它守的是「页面↔组件↔composable↔域 api」
 * 的接线，行为兜底 = ④ 编译门（Kotlin 形态）+ ①a 真机逐页冒烟；接口改名/漏绑由本套件在 CI 上先红。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const EDIT_PAGE = 'pages/resume/resume-edit.uvue';
const MODULE_PAGES = [
  'pages/resume/resume.uvue',
  'pages/resume/resume-edit.uvue',
  'pages/resume/resume-attach.uvue',
];

/** 模块必需源文件清单（删任一即红） */
const REQUIRED_SOURCE_FILES = [
  'pages/resume/resume.uvue',
  'pages/resume/resume-edit.uvue',
  'pages/resume/resume-attach.uvue',
  'pages/resume/composables/useResumeEdit.uts',
  'pages/resume/components/resume-progress-card.uvue',
  'api/resume.uts',
];

/** 页面 ↔ 组件接口对账表（本票唯一新增接口面，改名必须红）
 *  ⚠️ 只有「纯展示且卡宽不随宿主变化」的 section 才允许抽组件：①a 真机实测
 *  基本信息 / 求职期望卡抽成组件后卡宽 +70px（组件根撑满父容器 vs 页面级 view 按内容收缩）
 *  ⇒ 已回退为页面级卡，锁见「拆分判据锁」节。 */
const WIRING = [
  { page: EDIT_PAGE, tag: 'ResumeProgressCard', comp: 'pages/resume/components/resume-progress-card.uvue' },
];

/** T07 口径：新建 composable / section 组件同样计入预算 */
const BUDGET_FILES = REQUIRED_SOURCE_FILES;
const LINE_BUDGET = 600;

const camelToKebab = (s) => s.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase());
/** `update:realName` → `update:real-name`（Vue 模板事件名 kebab 归一，逐段转换） */
const normalizeName = (s) => s.split(':').map(camelToKebab).join(':');

/** 组件 defineProps<{...}> 的字段名（覆盖 `x? : T` / `x : T` 两种写法） */
function definePropsNames(src) {
  const m = /defineProps<\{([\s\S]*?)\}>/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/([A-Za-z_$][\w$]*)\s*\??\s*:/g)].map((x) => x[1]);
}

/** 组件 defineEmits<{ (e: 'x', …): void }>() 的事件名（调用签名形态） */
function defineEmitsNames(src) {
  const m = /defineEmits<\{([\s\S]*?)\}>\(\)/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/\(\s*e\s*:\s*'([^']+)'/g)].map((x) => x[1]);
}

/** 页面里某个组件的属性区（自闭合标签；本票三个组件用法均为自闭合多行标签） */
function usageAttrs(pageSrc, tag) {
  const m = new RegExp('<' + tag + '\\b([\\s\\S]*?)/>').exec(pageSrc);
  return m === null ? null : m[1];
}

/** 页面在某组件上绑定的 prop 名（kebab）
 *  ⚠️ 负向后顾必须排除 `@update:real-name` 里的那个 `:` —— 否则事件绑定会被当成 prop 绑定，
 *  双向对账同时被自己的写法命中（T05 已录的「锁被断言字符串自身命中」坑位）。 */
function boundProps(attrs) {
  return [...attrs.matchAll(/(?<![\w-]):([a-zA-Z][\w-]*)\s*=/g)].map((m) => m[1]);
}

/** 页面在某组件上监听的事件名（kebab，含 `update:` 前缀） */
function listenedEvents(attrs) {
  return [...attrs.matchAll(/@([a-zA-Z][\w-]*(?::[a-zA-Z][\w-]*)?)\s*=/g)].map((m) => m[1]);
}

/** prop 对账：页面绑定了但组件没声明 */
function undeclaredProps(pageSrc, compSrc, tag) {
  const attrs = usageAttrs(pageSrc, tag);
  if (attrs === null) return null;
  const declared = definePropsNames(compSrc).map(camelToKebab);
  return boundProps(attrs).filter((b) => !declared.includes(b));
}

/** prop 对账：组件声明了但页面没绑定（孤儿 prop） */
function unboundProps(pageSrc, compSrc, tag) {
  const attrs = usageAttrs(pageSrc, tag);
  if (attrs === null) return null;
  const bound = boundProps(attrs);
  return definePropsNames(compSrc).map(camelToKebab).filter((d) => !bound.includes(d));
}

/** 事件对账：页面监听了但组件没声明 */
function undeclaredEvents(pageSrc, compSrc, tag) {
  const attrs = usageAttrs(pageSrc, tag);
  if (attrs === null) return null;
  const declared = defineEmitsNames(compSrc).map(normalizeName);
  return listenedEvents(attrs).filter((l) => !declared.includes(l));
}

/** 事件对账：组件声明了但页面没监听（孤儿 emit） */
function unlistenedEvents(pageSrc, compSrc, tag) {
  const attrs = usageAttrs(pageSrc, tag);
  if (attrs === null) return null;
  const listened = listenedEvents(attrs);
  return defineEmitsNames(compSrc).map(normalizeName).filter((d) => !listened.includes(d));
}

/** 页面引用的 composable 成员名（`edit.<name>`，模板与 script 都算） */
function composableMemberRefs(pageSrc, alias) {
  const re = new RegExp('\\b' + alias + '\\.([A-Za-z_$][\\w$]*)', 'g');
  return [...new Set([...pageSrc.matchAll(re)].map((m) => m[1]))];
}

/** composable 显式结果类型里声明的字段名（返回面的权威清单） */
function declaredResultFields(src) {
  const m = /export type UseResumeEditResult = \{([\s\S]*?)\n\}/.exec(src);
  if (m === null) return [];
  return [...m[1].matchAll(/^\s*([A-Za-z_$][\w$]*)\s*:/gm)].map((x) => x[1]);
}

function countLines(abs) {
  return readText(abs).split('\n').length;
}

function resumeSourceFiles() {
  const out = [];
  const walk = (d) => {
    if (!fs.existsSync(d)) return;
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, 'pages/resume'));
  return out;
}

/** 抹掉注释（保留字符串）：`://` 例外，防误杀 URL 字面量 */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

describe('600 行软预算机检（pages/resume/** + api/resume.uts）', () => {
  it('resume 模块全部源文件 ≤600 行（含新建 composable 与 section 组件）', () => {
    const over = BUDGET_FILES.map((rel) => ({
      file: rel,
      lines: countLines(path.join(ROOT, rel)),
    })).filter((x) => x.lines > LINE_BUDGET);
    expect(over).toEqual([]);
  });

  it('页面从 922 行落到软预算内（手术目标本身也断言，防「只挪注释」的假达标）', () => {
    expect(countLines(path.join(ROOT, EDIT_PAGE))).toBeLessThanOrEqual(LINE_BUDGET);
    expect(countLines(path.join(ROOT, EDIT_PAGE))).toBeLessThan(700);
  });

  it('预算判据具备红能力（601 行合成输入必须被抓到，600 行放行）', () => {
    const probe = [{ file: 'synthetic-601', lines: LINE_BUDGET + 1 }, { file: 'synthetic-600', lines: LINE_BUDGET }];
    expect(probe.filter((x) => x.lines > LINE_BUDGET)).toEqual([{ file: 'synthetic-601', lines: 601 }]);
  });

  it('模块目录 ≤2 层', () => {
    const deep = resumeSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/resume'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });

  it('模块必需源文件清单完整', () => {
    const missing = REQUIRED_SOURCE_FILES.filter((f) => !exists(f));
    expect(missing).toEqual([]);
    expect(resumeSourceFiles().length).toBeGreaterThanOrEqual(4);
  });
});

describe('composable 接线契约（T09 拆分：显式 import 模块私有 composable）', () => {
  const composable = 'pages/resume/composables/useResumeEdit.uts';

  it('resume-edit.uvue import useResumeEdit composable', () => {
    expect(read(EDIT_PAGE)).toContain("from './composables/useResumeEdit'");
  });

  it('composable 声明显式结果类型（Kotlin error18 规避，守护规则 U 同款口径）', () => {
    expect(read(composable)).toMatch(/export function useResumeEdit\(\)\s*:\s*UseResumeEditResult/);
  });

  it('composable 返回对象显式标注结果类型（as UseResumeEditResult，与 stores/auth 同款写法）', () => {
    expect(read(composable)).toContain('} as UseResumeEditResult');
  });

  it('页面不 import 域 api / 不碰请求层（只经 composable）', () => {
    const page = read(EDIT_PAGE);
    expect(page).not.toContain('api/resume');
    expect(page).not.toContain('api/request');
  });

  it('页面引用的每个 composable 成员都在显式结果类型内（④c 编译门实测抓到的缺口，本锁防复发）', () => {
    const refs = composableMemberRefs(read(EDIT_PAGE), 'edit');
    const declared = declaredResultFields(read(composable));
    expect(declared.length).toBeGreaterThan(25);
    expect(refs.length).toBeGreaterThan(20);
    expect(refs.filter((r) => !declared.includes(r))).toEqual([]);
  });

  it('成员存在性判据具备红能力（注入一个未声明的成员必须被抓到）', () => {
    const page = read(EDIT_PAGE).replace('edit.completion.value', 'edit.completionX.value');
    const declared = declaredResultFields(read(composable));
    expect(composableMemberRefs(page, 'edit').filter((r) => !declared.includes(r))).toEqual(['completionX']);
  });
});

describe('组件接线零孤儿（#779 回归锁：import 的组件文件必须存在）', () => {
  it('页面 import 的每个模块私有组件文件都真实存在', () => {
    const missing = [];
    for (const page of MODULE_PAGES) {
      const src = read(page);
      const re = /from\s+'\.\/components\/([^']+\.uvue)'/g;
      let m;
      while ((m = re.exec(src)) !== null) {
        if (!fs.existsSync(path.join(ROOT, 'pages/resume/components', m[1]))) {
          missing.push(page + ' -> components/' + m[1]);
        }
      }
    }
    expect(missing).toEqual([]);
  });

  it('组件目录内不存在孤儿文件（每个 .uvue 都被某页面显式 import）', () => {
    const dir = path.join(ROOT, 'pages/resume/components');
    expect(fs.existsSync(dir)).toBe(true);
    const pagesSrc = MODULE_PAGES.map((p) => read(p)).join('\n');
    const orphans = fs.readdirSync(dir)
      .filter((f) => f.endsWith('.uvue'))
      .filter((f) => !pagesSrc.includes('./components/' + f));
    expect(orphans).toEqual([]);
  });

  it('孤儿判据具备红能力（注入一个不存在的 import 必须被列出）', () => {
    const injected = read(EDIT_PAGE).replace(
      "from './components/resume-progress-card.uvue'",
      "from './components/resume-progress-card-missing.uvue'",
    );
    const re = /from\s+'\.\/components\/([^']+\.uvue)'/g;
    const missing = [];
    let m;
    while ((m = re.exec(injected)) !== null) {
      if (!fs.existsSync(path.join(ROOT, 'pages/resume/components', m[1]))) missing.push(m[1]);
    }
    expect(missing).toEqual(['resume-progress-card-missing.uvue']);
  });
});

describe('页面 ↔ 组件接口对账（prop / 事件双向无孤儿）', () => {
  it.each(WIRING.map((w) => [w.tag, w]))('%s：页面绑定的每个 prop 都在组件 defineProps 内', (tag, w) => {
    const attrs = usageAttrs(read(w.page), tag);
    expect(attrs).not.toBeNull();
    expect(boundProps(attrs).length).toBeGreaterThan(0);
    expect(undeclaredProps(read(w.page), read(w.comp), tag)).toEqual([]);
  });

  it.each(WIRING.map((w) => [w.tag, w]))('%s：组件声明的每个 prop 都被页面绑定（无孤儿 prop）', (tag, w) => {
    expect(unboundProps(read(w.page), read(w.comp), tag)).toEqual([]);
  });

  it.each(WIRING.map((w) => [w.tag, w]))('%s：页面监听的每个事件都在组件 defineEmits 内（含 update: 前缀归一）', (tag, w) => {
    expect(undeclaredEvents(read(w.page), read(w.comp), tag)).toEqual([]);
  });

  it.each(WIRING.map((w) => [w.tag, w]))('%s：组件声明的每个事件都被页面监听（无孤儿 emit）', (tag, w) => {
    expect(unlistenedEvents(read(w.page), read(w.comp), tag)).toEqual([]);
  });

  it('对账表本身有效（1 个组件、1 prop、0 事件，防解析器静默失效）', () => {
    const props = WIRING.reduce((n, w) => n + definePropsNames(read(w.comp)).length, 0);
    const emits = WIRING.reduce((n, w) => n + defineEmitsNames(read(w.comp)).length, 0);
    expect(props).toBe(1);
    expect(emits).toBe(0);
  });

  it('对账锁具备红能力（注入 prop 改名 / 注入孤儿事件必须被抓到）', () => {
    const page = read(EDIT_PAGE);
    const progress = read('pages/resume/components/resume-progress-card.uvue');

    // ① 页面侧把 :completion 写错 → prop 对账必须红
    const badPropPage = page.replace(':completion="edit.completion.value"', ':completions="edit.completion.value"');
    expect(undeclaredProps(badPropPage, progress, 'ResumeProgressCard')).toEqual(['completions']);
    // 反向：组件侧声明改名而页面未改 → 孤儿 prop 必须红
    const badPropComp = progress.replace('completion? : number', 'completions? : number');
    expect(unboundProps(page, badPropComp, 'ResumeProgressCard')).toEqual(['completions']);

    // ② 组件侧新增一个页面没监听的 emit → 孤儿 emit 必须红
    const badEventComp = progress.replace(
      'const props = withDefaults(',
      "const emit = defineEmits<{ (e: 'progressTap'): void }>()\n    const props = withDefaults(",
    );
    expect(unlistenedEvents(page, badEventComp, 'ResumeProgressCard')).toEqual(['progress-tap']);
    // 反向：页面监听了组件没声明的事件 → 必须红
    const badEventPage = page.replace(' />', ' @progress-tap="onSave" />');
    expect(undeclaredEvents(badEventPage, progress, 'ResumeProgressCard')).toEqual(['progress-tap']);
  });
});

describe('allowlist 不回潮（resume 域违例清零的锁）', () => {
  // ADR-0023 决策 ⑧：`GUARD_ALLOWLIST` 的唯一声明点是 `utils/guardAllowlist.js`，
  // 消费方一律 require 取用 —— **不得再解析源码文本**（旧形态「indexOf + slice」的终止符缺失会静默扩扫全文）。
  const { allowlistPaths } = require('./guardAllowlist');
  const isResumePath = (p) => /^pages\/resume\//.test(p) || p === 'api/resume.uts';

  it('GUARD_ALLOWLIST 不含 resume 域文件', () => {
    expect(allowlistPaths().filter(isResumePath)).toEqual([]);
  });

  it('判据具备红能力（注入一条 resume 豁免必须被抓到）', () => {
    const injected = allowlistPaths().concat(['pages/resume/resume-edit.uvue']);
    expect(injected.filter(isResumePath)).toEqual(['pages/resume/resume-edit.uvue']);
  });
});

describe('零直发请求（页面层不直接 uni.request）', () => {
  it.each(MODULE_PAGES)('%s 不直接调用 uni.request', (page) => {
    expect(read(page)).not.toMatch(/uni\.request\s*\(/);
  });

  it('模块内唯一请求出口是 api/resume.uts（页面与 composable 都不 import request 层）', () => {
    expect(read('pages/resume/composables/useResumeEdit.uts')).toContain("from '../../../api/resume'");
    for (const page of MODULE_PAGES) {
      expect(read(page)).not.toContain("from '../../api/request'");
    }
  });
});

describe('域 api 收紧（T09 / ADR-0007）：DTO 出口走 mapper-callback 家族', () => {
  const api = read('api/resume.uts');

  it('引入 getMapped / requestMapped 出口家族，且不再 import 裸 get/put', () => {
    expect(api).toMatch(/import\s*\{[^}]*getMapped[^}]*requestMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(api).not.toMatch(/import\s*\{[^}]*\bget\b[^}]*\}\s*from\s*'\.\/request'/);
    expect(api).not.toMatch(/import\s*\{[^}]*\bput\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  const MAPPED_FNS = [
    'getResumeApi',
    'saveResumeApi',
    'setVisibilityApi',
    'getViewStatsApi',
    'getPositionsApi',
  ];

  it.each(MAPPED_FNS)('%s 经 mapper 出口（getMapped/requestMapped + 箭头包裹 build*）', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    expect(body).toMatch(/(get|request)Mapped<[A-Za-z_$][\w$]*(?:\[\])?>\(/);
    expect(body).toMatch(/\(data : UTSJSONObject\) : [A-Za-z_$][\w$]*(?:\[\])? => (build[A-Za-z_$][\w$]*|to[A-Za-z_$][\w$]*)\(data/);
  });

  it('PUT 面走 requestMapped + opts.method 赋值（与既有 put() 逐字同通路，T05 先例）', () => {
    for (const name of ['saveResumeApi', 'setVisibilityApi']) {
      const start = api.indexOf('export function ' + name);
      const body = api.slice(start, api.indexOf('\n}', start));
      expect(body).toContain("opts.method = 'PUT'");
      expect(body).toContain('requestMapped<');
    }
  });

  /** 裸透传白名单：multipart 上传 ×2（mapped 家族无 multipart 出口）+ void 删除 ×1（#1198 端点被跨栈守护钉住） */
  const RAW_PASSTHROUGH = {
    uploadWorkPhotoApi: /uploadFile\('\/resume\/image'/,
    uploadResumePdfApi: /uploadFile\('\/resume\/pdf'/,
    deleteAttachmentApi: /del\('\/resume\/pdf'/,
  };

  it.each(Object.keys(RAW_PASSTHROUGH))('%s 保持裸透传（未误加 DTO）', (name) => {
    const start = api.indexOf('export function ' + name);
    expect(start).toBeGreaterThan(-1);
    const body = api.slice(start, api.indexOf('\n}', start));
    expect(body).not.toMatch(/Mapped</);
    expect(body).toMatch(RAW_PASSTHROUGH[name]);
  });

  it('api 层不再出现裸 get( / put( 调用（收紧面全量，防漏网）', () => {
    const code = stripComments(api);
    expect(code).not.toMatch(/[^a-zA-Z]get\(/);
    expect(code).not.toMatch(/[^a-zA-Z]put\(/);
  });

  it('零消费出口白名单（保留决策，不是惯性）：3 个函数在移动端零调用点', () => {
    const pagesSrc = MODULE_PAGES.map((p) => read(p)).join('\n');
    const composablesSrc = read('pages/resume/composables/useResumeEdit.uts');
    for (const name of ['setVisibilityApi', 'getViewStatsApi', 'uploadWorkPhotoApi']) {
      expect(api).toContain('export function ' + name);
      expect(pagesSrc.includes(name)).toBe(false);
      expect(composablesSrc.includes(name)).toBe(false);
    }
  });
});

describe('幻影路由锁（#662 口径）：api 层路由必须落在后端已注册清单内', () => {
  const API_DIR = path.join(ROOT, '..', '..', 'backend', 'internal', 'api');

  /** `/resume` 组的已注册路由：job_card.go（同组 6 条）+ resume_view.go（查看聚合，另一处注册） */
  function registeredResumeRoutes() {
    const out = [];
    for (const file of ['job_card.go', 'resume_view.go']) {
      const src = stripComments(readText(path.join(API_DIR, file)));
      expect(src).toContain('Group("/resume"');
      const re = /g\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"/g;
      let m;
      while ((m = re.exec(src)) !== null) out.push('/resume' + m[2]);
    }
    return out;
  }

  it('后端 /resume 组已注册 7 条（6 + view-stats），且 /positions 公开读已注册', () => {
    const registered = registeredResumeRoutes();
    expect(registered).toContain('/resume');
    expect(registered).toContain('/resume/visibility');
    expect(registered).toContain('/resume/view-stats');
    expect(registered).toContain('/resume/pdf');
    expect(registered).toContain('/resume/image');
    expect(registered.length).toBe(7);

    const catalog = stripComments(readText(path.join(API_DIR, 'training_catalog.go')));
    expect(catalog).toMatch(/rg\.GET\("\/positions"/);
  });

  it('api/resume.uts 的每条路由都在已注册清单内（含 /positions）', () => {
    const api = stripComments(read('api/resume.uts'));
    const used = [...new Set([...api.matchAll(/'(\/(?:resume|positions)[^']*)'/g)].map((m) => m[1]))];
    expect(used.length).toBe(6);

    const registered = registeredResumeRoutes().concat(['/positions']);
    expect(used.filter((u) => !registered.includes(u))).toEqual([]);
  });

  it('幻影路由判据具备红能力（注入一条未注册路由必须被列出）', () => {
    const api = stripComments(read('api/resume.uts'));
    const injected = api.replace("'/resume/view-stats'", "'/resume/view-stat'");
    const used = [...new Set([...injected.matchAll(/'(\/(?:resume|positions)[^']*)'/g)].map((m) => m[1]))];
    const registered = registeredResumeRoutes().concat(['/positions']);
    expect(used.filter((u) => !registered.includes(u))).toEqual(['/resume/view-stat']);
  });
});

describe('删除禁区「resume 不用删」：行为保持点逐项仍在', () => {
  const composable = read('pages/resume/composables/useResumeEdit.uts');
  const page = read(EDIT_PAGE);

  it('草稿回填：主草稿 key 与教育经历独立 key 双读写仍在，且按字段逐项回填', () => {
    expect(composable).toContain("const STORAGE_KEY_RESUME_EDU = 'user_resume_edu_draft'");
    expect(composable).toContain('getStorageJSON(STORAGE_KEY_RESUME)');
    expect(composable).toContain('getStorageJSON(STORAGE_KEY_RESUME_EDU)');
    expect(composable).toContain("realName.value = toStr(saved['real_name'])");
    expect(composable).toContain("expectedRegionsText.value = toStr(saved['expected_regions_text'])");
  });

  it('服务端回显：GET /resume 回填全部展示字段 + 工作经历重建 + 岗位名回填 + 空列表补一行', () => {
    expect(composable).toContain('getResumeApi().then((r : ResumeData) : void => {');
    expect(composable).toContain('experienceYears.value = r.experience_years > 0 ? r.experience_years.toString() : \'\'');
    expect(composable).toContain('currentVisibility.value = r.visibility.length > 0 ? r.visibility : \'hidden\'');
    expect(composable).toContain('expectedRegionsText.value = r.expected_regions.join(\',\')');
    expect(composable).toContain('if (expItems.value.length == 0) addExp()');
    expect(composable).toContain('selectedPositionName.value = positions.value[i].name');
  });

  it('加载失败可见：notfound 判定走 `api/resume.uts` 的**唯一出口**，页面/composable 不再各抄一份死判据', () => {
    // #1204 之前，这里钉的是 `e['kind'] == 'notfound'` 的**复制判据** —— 而 `Error` 上没有 `kind`
    // ⇒ 分支恒不命中（新用户编辑页误报「简历加载失败」）。现在两端都消费同一个 `isResumeNotFound()`。
    expect(composable).toContain("import { getResumeApi, isResumeNotFound, saveResumeApi, getPositionsApi } from '../../../api/resume'");
    expect(composable).toContain('if (!isResumeNotFound(e)) {');
    expect(composable).toContain("uni.showToast({ title: '简历加载失败', icon: 'none' })");
    // 死判据不得复活：既不得再直读 `kind`，也不得再靠后端中文文案匹配
    expect(composable).not.toContain("obj['kind']");
    expect(composable).not.toContain("indexOf('不存在')");
  });

  it('完善度：12 项判据逐项仍在（含手机号 11 位、薪资三选一、教育/工作内容判空）', () => {
    expect(composable).toContain('if (contactPhone.value.length == 11) filled += 1');
    expect(composable).toContain('if (toNumber(salaryMin.value) > 0 || toNumber(salaryMax.value) > 0 || salaryNegotiable.value) filled += 1');
    expect(composable).toContain('return Math.round(filled * 100 / 12)');
    expect(composable).toContain('function hasEduContent() : boolean');
    expect(composable).toContain('function hasExpContent() : boolean');
  });

  it('三处 actionSheet：意向岗位（含未加载提示）/ 工作类型 / 到岗时间 / 学历 逐项仍在', () => {
    expect(composable).toContain("uni.showToast({ title: '岗位列表加载中', icon: 'none' })");
    expect(composable).toContain('function onPositionTap() : void');
    expect(composable).toContain('function onJobNatureTap() : void');
    expect(composable).toContain('function onAvailableInTap() : void');
    expect(composable).toContain('function onDegreeTap(idx : number) : void');
    expect(composable).toContain('uni.showActionSheet({');
    expect(composable).toContain("const availableInValues : string[] = ['immediate', '1w', '2w', '1m']");
    expect(composable).toContain("const jobNatureValues : string[] = ['fulltime', 'parttime', 'contract']");
    expect(composable).toContain("const degreeOptions : string[] = ['初中及以下', '高中/中专/技校', '大专', '本科', '硕士及以上']");
  });

  it('多段经历增删：教育/工作各自 add / remove 逐项仍在，且默认补一行', () => {
    expect(composable).toContain('function addEdu() : void');
    expect(composable).toContain('function removeEdu(idx : number) : void');
    expect(composable).toContain('function addExp() : void');
    expect(composable).toContain('function removeExp(idx : number) : void');
    expect(composable).toContain('if (eduItems.value.length == 0) addEdu()');
    expect(composable).toContain('eduItems.value.splice(idx, 1)');
    expect(composable).toContain('expItems.value.splice(idx, 1)');
  });

  it('保存四条校验：姓名必填 / 11 位手机号 / 自我介绍 ≤1000 / 薪资非负且下限不高于上限', () => {
    expect(composable).toContain("uni.showToast({ title: '请填写姓名', icon: 'none' })");
    expect(composable).toContain("uni.showToast({ title: '请填写11位手机号', icon: 'none' })");
    expect(composable).toContain("uni.showToast({ title: '自我介绍不超过1000字', icon: 'none' })");
    expect(composable).toContain("uni.showToast({ title: '薪资不能为负数', icon: 'none' })");
    expect(composable).toContain("uni.showToast({ title: '最低薪资不能高于最高薪资', icon: 'none' })");
  });

  it('保存载荷：整页 upsert 字段面（含空数组显式提交、visibility 用回显值）保持不变', () => {
    expect(composable).toContain('resume_certifications: [],');
    expect(composable).toContain('resume_file_url: \'\',');
    expect(composable).toContain('photos: [],');
    expect(composable).toContain('visibility: currentVisibility.value,');
    expect(composable).toContain('if (e.company.trim().length == 0 && e.role.trim().length == 0) continue');
  });

  it('保存成功：双 key 落盘 + 「保存成功」+ 返回简历列表页（失败提示保留）', () => {
    expect(composable).toContain('setStorage(STORAGE_KEY_RESUME, JSON.stringify(draft))');
    expect(composable).toContain('setStorage(STORAGE_KEY_RESUME_EDU, JSON.stringify(eduData))');
    expect(composable).toContain("uni.showToast({ title: '保存成功', icon: 'success', duration: 800 })");
    expect(composable).toContain("goBack('/pages/resume/resume')");
    expect(composable).toContain("uni.showToast({ title: errMsg(e, '保存失败'), icon: 'none' })");
  });

  it('保存入口双份与顶栏形态保持：顶栏「保存」+ 底部「保存简历」，标题「在线简历」', () => {
    expect(page).toContain('<text class="nav-save">保存</text>');
    expect(page).toContain('<text class="nav-title">在线简历</text>');
    expect(page).toContain('<text class="save-btn-text">保存简历</text>');
    expect(page).toContain('@click="onSave"');
  });

  it('模块文件集合不减：三条路由与四个 api 消费点仍在', () => {
    const routes = [...read('pages.json').matchAll(/"path"\s*:\s*"([^"]+)"/g)].map((m) => m[1]);
    for (const r of ['pages/resume/resume', 'pages/resume/resume-edit', 'pages/resume/resume-attach']) {
      expect(routes).toContain(r);
    }
    expect(read('pages/resume/resume-attach.uvue')).toContain("deleteAttachmentApi('pdf')");
  });
});

describe('拆分判据锁（T09 新增）：壳层不自持编辑态，「卡宽随宿主变化」的卡一律留壳层', () => {
  const page = read(EDIT_PAGE);

  it('页面壳层零自持编辑态（不出现 ref< 声明，编辑态全归 useResumeEdit）', () => {
    const script = page.slice(page.indexOf('<script'), page.indexOf('</script>'));
    expect(script).not.toMatch(/\bref</);
    expect(script).not.toMatch(/\breactive</);
  });

  it('自持态的判据具备红能力（注入一行 ref 声明必须被抓到）', () => {
    const script = page.slice(page.indexOf('<script'), page.indexOf('</script>'));
    const injected = script + "\n    const leaked = ref<string>('')\n";
    expect(injected).toMatch(/\bref</);
  });

  it('教育 / 工作经历列表（v-model 风险区）留在页面壳层：两处 v-for + 逐字段 v-model 仍在', () => {
    expect(page).toContain('v-for="(item, idx) in edit.eduItems.value"');
    expect(page).toContain('v-for="(item, idx) in edit.expItems.value"');
    expect(page).toContain('v-model="item.school"');
    expect(page).toContain('v-model="item.start_month"');
    expect(page).toContain('v-model="item.desc"');
  });

  it('基本信息 / 求职期望卡留在页面壳层（①a 实测：抽成组件后卡宽 +70px、下方内容上移 35px）', () => {
    // 两张卡必须是**页面级** `<view class="card">`，且字段仍走 composable 的 v-model 透传
    expect(page).toContain('<text class="card-title">基本信息</text>');
    expect(page).toContain('<text class="card-title">求职期望</text>');
    expect(page).toContain('v-model="edit.realName.value"');
    expect(page).toContain('v-model="edit.expectedRegionsText.value"');
    expect(page).toContain('@click="edit.onPositionTap()"');
    // 不得存在把这两张卡抽走的组件文件（回退后仍在 = 有人重做但没跑 ①a）
    const comps = fs.readdirSync(path.join(ROOT, 'pages/resume/components'));
    expect(comps).toEqual(['resume-progress-card.uvue']);
  });

  it('几何判据具备红能力（把卡标记成组件形态必须被抓到）', () => {
    const injected = page.replace('<text class="card-title">基本信息</text>', '<ResumeBasicCard />');
    expect(injected).not.toContain('<text class="card-title">基本信息</text>');
    expect(fs.readdirSync(path.join(ROOT, 'pages/resume/components'))).not.toContain('resume-basic-card.uvue');
  });

  it('组件只做「原始值 prop + 单参事件」：组件模板内不出现 v-model 绑定', () => {
    for (const w of WIRING) {
      const src = read(w.comp);
      const tpl = src.slice(src.indexOf('<template>'), src.indexOf('</template>'));
      // 判**绑定形态**而非裸词：组件头注释里正当地写着「禁跨组件 v-model」（T05 已录的注释自命中坑位）
      expect(tpl).not.toMatch(/v-model[:=]/);
    }
  });
});

/**
 * #1204 notfound 单一出口：`api/request.uts`（写端）→ `api/resume.uts`（判定出口）→ 两个消费方。
 *
 * ⚠️ 本节的断言是**接线守护**（源码文本），**不构成 ③ 门的行为证据** —— 行为证据在本模块的
 * `utils/resumeNotFoundBehavior.test.js`（真跑 request→resume→编辑页 composable，含变异必红侧）。
 * 两节分工：行为测试会自己搭依赖（接线断了它照样绿），本节负责「接线没断」。
 */
describe('#1204 notfound 单一出口（写端前缀 → 判定出口 → 两个消费方）', () => {
  const req = read('api/request.uts');
  const api = read('api/resume.uts');
  const overview = read('pages/resume/resume.uvue');
  const composable = read('pages/resume/composables/useResumeEdit.uts');

  it('写端：request.uts 有 404 分支，状态码经**消息前缀**承载（与 403 同构，不给 any 动态加属性）', () => {
    expect(req).toContain("export const NOT_FOUND_MESSAGE_PREFIX = '404:'");
    const start = req.indexOf('if (statusCode == 404) {');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n    }', start));
    expect(body).toContain('NOT_FOUND_MESSAGE_PREFIX + notFoundMsg');
    // 404 是**预期分支**：与 401 分开（不清登录态、不跳登录页）
    const code = stripComments(body);
    expect(code).not.toContain('handleUnauthorized');
    expect(code).not.toContain('logoutAndReject');
    expect(code).not.toContain('removeStorage');
    expect(code).not.toContain('reLaunch');
    // 不得回到「给 any 动态加 statusCode」（UTS→Kotlin error18 的旧写法）
    expect(code).not.toMatch(/\.statusCode\s*=\s*404/);
    expect(code).not.toMatch(/as any\)\.statusCode/);
  });

  it('读端唯一出口：resume.uts 的 isResumeNotFound 只认 request 的常量，不再按后端中文文案匹配', () => {
    expect(api).toMatch(/import\s*\{[^}]*NOT_FOUND_MESSAGE_PREFIX[^}]*\}\s*from\s*'\.\/request'/);
    expect(api).toContain('export function isResumeNotFound(e : any | null) : boolean {');
    expect(api).toContain("return errMsg(e, '').startsWith(NOT_FOUND_MESSAGE_PREFIX)");
    // 旧判据与它包装出的假 `kind` 语义必须消失（后端改文案即静默失效的那条路）
    const code = stripComments(api);
    expect(code).not.toContain("indexOf('不存在')");
    expect(code).not.toContain("indexOf('404')");
    expect(code).not.toContain('RESUME_NOT_FOUND');
  });

  it('两个消费方都走同一实现，且都不再直读错误对象的 kind', () => {
    for (const src of [overview, composable]) {
      expect(src).not.toContain("obj['kind']");
      expect(src).toContain('isResumeNotFound(e)');
      expect(src).toMatch(/import \{[^}]*isResumeNotFound[^}]*\} from '[^']*api\/resume'/);
    }
    // 总览页：未建 ⇒ 0 项 + 标记「已加载」，引导卡判据仍是「已加载且未填满」
    expect(overview).toContain('resumeFilledCount.value = 0');
    expect(overview).toContain('resumeStatusLoaded.value = true');
    expect(overview).toContain('return resumeStatusLoaded.value && resumeFilledCount.value < RESUME_FIELD_TOTAL');
    expect(overview).toContain('<text class="resume-guide-text">去填写简历</text>');
  });

  it('判据具备红能力：把历史形态（`obj[\'kind\']` 死判据）注入回去必须被本节的断言抓到', () => {
    const legacy = "            let isNotFound = false\n"
      + "            try {\n"
      + "                const obj = e as UTSJSONObject\n"
      + "                if (obj['kind'] != null && `${obj['kind']}` == 'notfound') {\n"
      + "                    isNotFound = true\n"
      + "                }\n"
      + "            } catch (_ex) {}\n"
      + "            if (isNotFound) {";
    const injected = overview.replace('if (isResumeNotFound(e)) {', legacy);
    expect(injected).not.toBe(overview);
    // 与上面「不再直读 kind」是**同一条判据**：注入后它必须命中（否则那条断言是恒真的空跑）
    expect(injected).toContain("obj['kind']");
  });
});
