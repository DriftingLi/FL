/**
 * 内容精选「正文可达」行为锁（2026-11 · 行为腿）
 *
 * 缺陷形态（本套件守的那一条）：后端自 000062 起有 `content_type = 'docx'` 的文章 ——
 * 写侧把 `content` 置空（`service.featuredContentBody`），正文是 `docx_url` 指向的原始 Word。
 * 移动端当时只读 `content`：
 *   1. api 映射丢掉 `content_type` / `docx_url`；
 *   2. 页面把空 `content` 交给 `parseMarkdown` ⇒ **0 个块** ⇒ 正文区整片空白（用户症状）。
 * 标题 / 封面 / 上下篇都正常，所以表现为「部分文章内容无法正常显示」。
 *
 * 为什么是行为守护：判据必须落在**跑起来的映射与判定**上 ——
 *   · 映射面：真执行 `api/featured.uts`，注入桩 request，喂后端真实响应形状；
 *   · 判定面：真执行 `utils/featuredBody.uts` 的分支矩阵；
 *   · 正文面：真执行 `utils/markdown.uts` + `utils/markdownInline.uts`，按渲染层等价投影
 *     （块 text → 行内 runs 取叶子）断言「可见字符不为 0」，覆盖多格式 / 多内容类型。
 *
 * 形状纪律：所有被测模块都是标量入参 / 标量出参，无自定义类型数组（#905 血账）。
 */
const path = require('path');
const { loadUts, importedNames, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const HELPERS = path.join(ROOT, 'api', 'helpers.uts');
const FEATURED_API = path.join(ROOT, 'api', 'featured.uts');
const FEATURED_BODY = path.join(__dirname, 'featuredBody.uts');
const MARKDOWN = path.join(__dirname, 'markdown.uts');
const MARKDOWN_INLINE = path.join(__dirname, 'markdownInline.uts');

const body = () => loadUts(FEATURED_BODY, {});
const md = () => loadUts(MARKDOWN, {});
const inline = () => loadUts(MARKDOWN_INLINE, {});

/** 注入真实 helpers（toStr / toNumber 不重写第二份），其余依赖用桩 */
function featuredApiWith(uniStub) {
  const helpers = loadUts(HELPERS, {});
  let captured = null;
  const mod = loadUts(FEATURED_API, {
    getMapped: (_url, _params, mapper) => {
      captured = mapper;
      return Promise.resolve(null);
    },
    post: () => Promise.resolve(),
    decodePage: () => ({}),
    toNumber: helpers.toNumber,
    toStr: helpers.toStr,
    uni: uniStub,
  });
  /** 先真跑一次详情请求（让映射函数被 api 层取用），再把同一响应交给它 */
  const load = async (json) => {
    await mod.getFeaturedContentDetailApi(json.content_id);
    return captured(json);
  };
  return { mod, load };
}

/** 后端公开详情响应（Go DTO 的 JSON 形状；字段按 key 字母序，见 ADR-0009） */
const docxResponse = (over = {}) => Object.assign({
  category: 'company',
  category_label: '公司动态',
  content: '',
  content_id: 901,
  content_type: 'docx',
  cover_image: '',
  created_at: '2026-11-18T10:00:00+08:00',
  docx_url: 'https://cdn.example.test/featured/docx/a.docx',
  next: null,
  prev: null,
  published_at: '2026-11-18T10:00:00+08:00',
  related: [],
  sort_order: 0,
  source: '',
  status: 1,
  summary: '',
  title: '叉车安全操作规程（Word 版）',
  view_count: 12,
}, over);

const markdownResponse = (over = {}) => docxResponse(Object.assign({
  content: '## 一、日常保养\n\n- 检查电解液\n- 清洁接线柱\n\n> 严禁带电作业',
  content_type: 'markdown',
  docx_url: '',
}, over));

/** 渲染层等价投影：声明 inline 的 featured 档，块文本/列表项再过一次分词取叶子 */
function visibleText(blocks, fin = inline()) {
  const parts = [];
  for (const b of blocks) {
    if (b.type === 'list' || b.type === 'table') {
      for (const it of b.items || []) parts.push(fin.inlineRunsPlainText(fin.splitInlineRuns(it)));
    } else if (b.type === 'image') {
      for (const it of b.items || []) parts.push(it);
    } else if (b.type === 'divider') {
      // 无文字
    } else {
      parts.push(fin.inlineRunsPlainText(fin.splitInlineRuns(b.text || '')));
    }
  }
  return parts.join('\n');
}

describe('① 映射面：详情响应必须带出正文形态（此前是丢字段的真缺陷）', () => {
  it('docx 文章：content_type / docx_url 都要映射，content 仍是空串（不伪造正文）', async () => {
    const { load } = featuredApiWith({});
    const d = await load(docxResponse());
    expect(d.content_type).toBe('docx');
    expect(d.docx_url).toBe('https://cdn.example.test/featured/docx/a.docx');
    expect(d.content).toBe('');
  });

  it('markdown 文章：两字段照原样映射', async () => {
    const { load } = featuredApiWith({});
    const d = await load(markdownResponse());
    expect(d.content_type).toBe('markdown');
    expect(d.docx_url).toBe('');
    expect(d.content.length).toBeGreaterThan(0);
  });

  it('存量行（无这两个键）：content_type 归一为 markdown，docx_url 为空串 —— 行为与改造前一致', async () => {
    const { load } = featuredApiWith({});
    const legacy = docxResponse({ content: '正文', content_type: undefined, docx_url: undefined });
    const d = await load(legacy);
    expect(d.content_type).toBe('markdown');
    expect(d.docx_url).toBe('');
    expect(d.content).toBe('正文');
  });

  it('注入面与真 import 一致（不漂第二份依赖清单）', () => {
    const helpers = loadUts(HELPERS, {});
    const need = importedNames(readText(FEATURED_API));
    // uni 是全局量（非 import），不在 importedNames 里；其余逐条必须由测试注入
    expect(need.sort()).toEqual(['decodePage', 'getMapped', 'post', 'toNumber', 'toStr'].sort());
    expect(typeof helpers.toStr).toBe('function');
  });
});

describe('② 判定面：正文形态矩阵（页面只消费这一处结论）', () => {
  it('docx + 有地址 ⇒ docx（走下载打开，不是空白）', () => {
    expect(body().featuredBodyKind('docx', '', 'https://e.test/a.docx')).toBe(body().BODY_KIND_DOCX);
  });

  it('docx + 缺地址 ⇒ docx-missing（可见提示，不是空白）', () => {
    expect(body().featuredBodyKind('docx', '', '')).toBe(body().BODY_KIND_DOCX_MISSING);
  });

  it('markdown + 有正文 ⇒ markdown', () => {
    expect(body().featuredBodyKind('markdown', '正文', '')).toBe(body().BODY_KIND_MARKDOWN);
  });

  it('markdown + 空正文 ⇒ empty（可见提示，不是空白）', () => {
    expect(body().featuredBodyKind('markdown', '', '')).toBe(body().BODY_KIND_EMPTY);
  });

  it('未知类型按 markdown 处理（与后端 normalizeContentType 的方向一致，不静默吞正文）', () => {
    expect(body().featuredBodyKind('', '正文', '')).toBe(body().BODY_KIND_MARKDOWN);
    expect(body().featuredBodyKind('rtf', '正文', '')).toBe(body().BODY_KIND_MARKDOWN);
  });

  it('docx 带残留 Markdown 正文时仍按 docx 渲染（类型优先），但完整性检查要报出来', () => {
    const b = body();
    expect(b.featuredBodyKind('docx', '残留', 'https://e.test/a.docx')).toBe(b.BODY_KIND_DOCX);
    expect(b.featuredBodyIssue('docx', '残留', 'https://e.test/a.docx')).toBe(b.ISSUE_DOCX_HAS_CONTENT);
  });
});

describe('③ 完整性预警面（要求 5：提前发现并预警）', () => {
  it('健康数据不报异常', () => {
    const b = body();
    expect(b.featuredBodyIssue('markdown', '正文', '')).toBe('');
    expect(b.featuredBodyIssue('docx', '', 'https://e.test/a.docx')).toBe('');
  });

  it('三种异常各自具名，文案唯一', () => {
    const b = body();
    expect(b.featuredBodyIssue('docx', '', '')).toBe(b.ISSUE_DOCX_URL_MISSING);
    expect(b.featuredBodyIssue('docx', '残留', 'https://e.test/a.docx')).toBe(b.ISSUE_DOCX_HAS_CONTENT);
    expect(b.featuredBodyIssue('markdown', '', '')).toBe(b.ISSUE_CONTENT_EMPTY);
  });

  it('文件名投影：去 query / hash、取最后一段；空地址退到「Word 文档」', () => {
    const b = body();
    expect(b.featuredDocxFileName('https://e.test/featured/docx/a.docx?v=1#p=2')).toBe('a.docx');
    expect(b.featuredDocxFileName('https://e.test/x/y/%E6%96%87%E6%A1%A3.docx')).toBe('%E6%96%87%E6%A1%A3.docx');
    expect(b.featuredDocxFileName('')).toBe('Word 文档');
  });

  it('异常形态必须给可见提示（不是空白），正常形态不给', () => {
    const b = body();
    expect(b.featuredBodyHint(b.BODY_KIND_DOCX_MISSING)).toBe(b.HINT_DOCX_MISSING);
    expect(b.featuredBodyHint(b.BODY_KIND_EMPTY)).toBe(b.HINT_EMPTY);
    expect(b.featuredBodyHint(b.BODY_KIND_MARKDOWN)).toBe('');
    expect(b.featuredBodyHint(b.BODY_KIND_DOCX)).toBe('');
  });
});

describe('④ 正文可达面（回归：不同格式 / 不同内容类型都不许丢字）', () => {
  const cases = [
    ['纯段落', '叉车电池的日常保养包括补水与清洁。', ['补水与清洁']],
    ['多级标题', '# 一级\n\n## 二级\n\n### 三级', ['一级', '二级', '三级']],
    ['粗体斜体', '**重点** 与 *斜体* 都要显示', ['重点', '斜体']],
    ['无序列表', '- 检查轮胎\n- 检查货叉', ['检查轮胎', '检查货叉']],
    ['有序列表', '1. 断电\n2. 泄压', ['断电', '泄压']],
    ['引用', '> 严禁带电作业', ['严禁带电作业']],
    ['代码块', '```\nconst a = 1\n```', ['const a = 1']],
    ['表格退回原文（精选档不声明 table）', '| 码 | 含义 |\n| --- | --- |\n| E01 | 低压 |', ['E01', '低压']],
    ['行内链接', '详见 [说明书](https://e.com/m) 第三章', ['说明书', '第三章']],
    ['图片 alt', '![故障图](https://e.com/a.png)', ['故障图']],
    ['HTML 标签原样可见', '<p>段落一</p><p>段落二</p>', ['段落一', '段落二']],
    ['HTML 实体不吞字', '温度 &amp; 压力 &lt; 阈值', ['温度', '阈值']],
    ['特殊符号', '① ② ③ × → ≤ ± ㎡ ℃', ['①', '℃']],
    ['未闭合围栏仍可见', '```\n游离代码行', ['游离代码行']],
    ['块级公式降级可见', '$$\na^2+b^2=c^2\n$$', ['a^2+b^2=c^2']],
    ['行内公式不被斜体撕碎', '公式 $a_1 + b_2$ 保留', ['a_1 + b_2']],
    ['乘号不吞字', '2 * 3 = 6，* 是乘号', ['2 * 3 = 6', '是乘号']],
    ['下划线变量不变', '变量 snake_case_name 不变', ['snake_case_name']],
    ['CRLF 正文', '第一行\r\n第二行', ['第一行', '第二行']],
    ['中文序号段', '一、故障码定位', ['故障码定位']],
  ];

  for (const [name, src, tokens] of cases) {
    it(`${name}：关键文字全部出现在可见投影里`, () => {
      const text = visibleText(md().parseMarkdown(src, md().SUBSET_FEATURED));
      for (const t of tokens) expect([name, t, text.includes(t)]).toEqual([name, t, true]);
    });
  }

  it('docx 文章不靠 Markdown 正文承重：可见正文为 0 是**数据事实**，页面因此必须分叉到文档入口', async () => {
    const { load } = featuredApiWith({});
    const d = await load(docxResponse());
    // 复刻改造前页面的投影：空 content ⇒ 0 块（这正是空白的来源）
    expect(md().parseMarkdown(d.content, md().SUBSET_FEATURED).length).toBe(0);
    // 改造后的结论：形态判定给出文档入口，而不是 empty
    expect(body().featuredBodyKind(d.content_type, d.content, d.docx_url)).toBe(body().BODY_KIND_DOCX);
  });
});

describe('⑤ docx 打开出路（下载动作在 api 层，页面禁裸调）', () => {
  /** 记录调用的 uni 桩：可编排下载 / 打开的成功与失败 */
  function uniStub(plan) {
    const calls = { download: [], open: [], toast: [] };
    return {
      calls,
      uni: {
        downloadFile(opts) {
          calls.download.push(opts);
          const reply = plan.download || {};
          setTimeout(() => {
            if (reply.fail) opts.fail({ errMsg: reply.fail });
            else opts.success({ statusCode: reply.statusCode ?? 200, tempFilePath: reply.tempFilePath ?? '/tmp/a.docx' });
          }, 0);
        },
        openDocument(opts) {
          calls.open.push(opts);
          const reply = plan.open || {};
          setTimeout(() => {
            if (reply.fail) opts.fail({ errMsg: reply.fail });
            else opts.success();
          }, 0);
        },
        showToast() {},
      },
    };
  }

  it('成功：先下载再打开，返回空串', async () => {
    const s = uniStub({});
    const { mod } = featuredApiWith(s.uni);
    const err = await mod.openFeaturedDocxApi('https://e.test/featured/docx/a.docx');
    expect(err).toBe('');
    expect(s.calls.download.length).toBe(1);
    expect(s.calls.open.length).toBe(1);
    expect(s.calls.open[0].filePath).toBe('/tmp/a.docx');
  });

  it('地址为空：不发起下载，直接给出可读原因', async () => {
    const s = uniStub({});
    const { mod } = featuredApiWith(s.uni);
    expect(await mod.openFeaturedDocxApi('')).toBe('文档地址无效');
    expect(s.calls.download.length).toBe(0);
  });

  it('HTTP 非 200：报下载失败，不进入打开', async () => {
    const s = uniStub({ download: { statusCode: 404 } });
    const { mod } = featuredApiWith(s.uni);
    expect(await mod.openFeaturedDocxApi('https://e.test/a.docx')).toContain('404');
    expect(s.calls.open.length).toBe(0);
  });

  it('下载超时：给超时文案；打开失败：给「无法打开此文档」', async () => {
    const t = uniStub({ download: { fail: 'downloadFile:fail timeout' } });
    const { mod } = featuredApiWith(t.uni);
    expect(await mod.openFeaturedDocxApi('https://e.test/a.docx')).toContain('超时');

    const o = uniStub({ open: { fail: 'openDocument:fail unsupported' } });
    const { mod: mod2 } = featuredApiWith(o.uni);
    expect(await mod2.openFeaturedDocxApi('https://e.test/a.docx')).toBe('无法打开此文档');
  });
});
