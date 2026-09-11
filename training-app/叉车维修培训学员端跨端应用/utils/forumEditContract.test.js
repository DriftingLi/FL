/**
 * 论坛编辑链路契约（#811，refs #662）
 *
 * 背景：#662 诊断确认「编辑帖子」是一条活的、必然失败的链路——
 * 后端 PUT /forum/topics/:id 未注册（404），且编辑态回填读的是详情 DTO **不存在**的
 * `scope`（buildTopicDetail 垫的 'general' 被 normalizeCategory 归一成 'discussion'），
 * 任何问答帖 / 备考经验帖一进编辑态就被显示成「讨论」，PUT 落地后照此保存即降级类别。
 *
 * .uvue/.uts 不可被 jest import，走源码契约缝（先例 forumContract / jobsMineEntryContract）。
 * 本文件钉四件事，防回归：
 *  1) 编辑态回填读 category（明细），**不得**再读 scope（旧分类词汇，后端已不回）；
 *  2) 保存链路 isEdit 分支调 updateForumTopicApi 且携带 selectedCategory；
 *  3) updateForumTopicApi 的请求形态：PUT /forum/topics/{id}，载荷 title/content/images(+category)；
 *  4) 类型层把 legacy scope 标 @deprecated，防止再次被误读；注释不再写「未注册」。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

describe('论坛编辑链路契约（#811）', () => {
  const create = read('pages/forum/forum-create.uvue');
  const api = read('api/forum.uts');
  const types = read('types/index.uts');

  it('编辑态回填读 category（详情 DTO 唯一权威字段），不再读 scope', () => {
    // 明细：必须读 category
    expect(create).toMatch(/selectedCategory\.value = normalizeCategory\(data\.category\)/);
    // 明细：不得再出现读 scope 的回填（旧 bug 形态）
    expect(create).not.toMatch(/normalizeCategory\(data\.scope\)/);
    expect(create).not.toMatch(/normalizeCategory\([^)]*\.scope\)/);
  });

  it('保存链路：isEdit 分支调 updateForumTopicApi 并携带 selectedCategory；非编辑走 create', () => {
    expect(create).toMatch(
      /await updateForumTopicApi\(editTopicId\.value, title\.value\.trim\(\), content\.value\.trim\(\), images\.value, selectedCategory\.value\)/
    );
    expect(create).toMatch(
      /await createForumTopicApi\(title\.value\.trim\(\), content\.value\.trim\(\), images\.value, selectedCategory\.value\)/
    );
  });

  it('updateForumTopicApi：PUT /forum/topics/{id}，载荷 title/content/images（category 非空才带）', () => {
    const fnStart = api.indexOf('export function updateForumTopicApi');
    expect(fnStart).toBeGreaterThan(-1);
    // 取 JSDoc 注释块 + 函数体（注释与实现同属该接口的事实面）
    const docStart = api.lastIndexOf('/**', fnStart);
    const fn = api.slice(docStart, api.indexOf('\n}', fnStart) + 2);
    expect(fn).toContain("const url = '/forum/topics/' + topicId.toString()");
    expect(fn).toMatch(/method:\s*'PUT'/);
    // 三个必带字段
    expect(fn).toContain('title: title');
    expect(fn).toContain('content: content');
    expect(fn).toContain('images: images');
    // category 条件携带（空串不发，后端归一 discussion）
    expect(fn).toMatch(/if \(category\.length > 0\) \{\s*payload\['category'\] = category/);
    // 注释已是「已落地」事实，不再写「未注册」（#662 的缺陷登记已随 #811 收口）
    expect(fn).not.toContain('未注册');
    expect(fn).toContain('#811');
    expect(fn).toContain('已落地');
  });

  it('类型层：legacy scope 标 @deprecated（判类别一律用 category）', () => {
    const scopeDecl = types.indexOf('scope : string');
    expect(scopeDecl).toBeGreaterThan(-1);
    // 声明上方 5 行内必须带 @deprecated 提示
    const ahead = types.slice(Math.max(0, scopeDecl - 400), scopeDecl);
    expect(ahead).toContain('@deprecated');
    expect(ahead).toContain('category');
  });

  it('类别归一语义（ADR-0040）：只保真 question，历史 experience 与资源域残留一律归 discussion 不抛错', () => {
    const start = create.indexOf('function normalizeCategory');
    expect(start).toBeGreaterThan(-1);
    const fn = create.slice(start, create.indexOf('\n    }', start));
    expect(fn).toContain("if (value == 'question') return value");
    expect(fn).toContain("return 'discussion'");
  });
});
