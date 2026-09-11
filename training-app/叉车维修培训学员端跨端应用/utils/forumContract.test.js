/**
 * forum 手术契约测试（T04，refs #642 / refactor epic #638）
 *
 * 钉住手术的三类契约（.uvue/.uts 不可被 jest import，走源码契约缝，先例 mallPilotContract）：
 * 1) 组件接线：forum.uvue / forum-detail.uvue 以显式 import 使用模块私有组件（Q17 安置规则）
 * 2) composable 下沉：列表/资源两域 + 详情/回复/举报三域状态离开壳层
 *    （招聘域已随 #705 退场部分整体删除，退场回归见「招聘 tab 退场契约」）
 * 3) 行为保持：手术不改像素与跳转语义——上传格子仍跳 forum-create（缺陷已登记 #662）、
 *    问答变体仍由 currentTab 驱动、
 *    回复栏 v-model 留壳层（uvue 跨组件 v-model 属编译风险区，composer 状态下沉即达预算）
 * 4) 600 软预算机检：pages/forum/** 全部源文件 ≤600 行 + 目录 ≤2 层（达标后锁住防回潮，Q8）
 * 5) allowlist 不回潮：forum 页面文件不得出现在 GUARD_ALLOWLIST
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 模块目录下全部源文件（.uvue/.uts，排除测试） */
function forumSourceFiles(dir = 'pages/forum') {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { walk(p); continue; }
      if (/\.(uvue|uts)$/.test(e.name) && !/\.test\./.test(e.name)) out.push(p);
    }
  };
  walk(path.join(ROOT, dir));
  return out;
}

const LIST_COMPONENTS = [
  'forum-tab-bar',
  'forum-square-sort-bar',
  'forum-qa-header',
  'forum-experience-sort-bar',
  'forum-featured-filter',
  'forum-checkin-card',
  'forum-topic-card',
  'forum-resource-panel',
];

const DETAIL_COMPONENTS = [
  'forum-topic-header',
  'forum-topic-body',
  'forum-reply-list',
  'forum-report-dialog',
];

describe('列表页组件接线契约（Q17 安置：pages/forum/components/ 显式 import）', () => {
  const page = read('pages/forum/forum.uvue');

  it.each(LIST_COMPONENTS)('组件文件存在于 pages/forum/components/%s.uvue', (name) => {
    expect(fs.existsSync(path.join(ROOT, 'pages/forum/components', `${name}.uvue`))).toBe(true);
  });

  it.each(LIST_COMPONENTS)('forum.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用全部组件标签（非只 import 不用）', () => {
    for (const c of ['ForumTabBar', 'ForumSquareSortBar', 'ForumQaHeader', 'ForumExperienceSortBar', 'ForumFeaturedFilter', 'ForumCheckinCard', 'ForumTopicCard', 'ForumResourcePanel']) {
      expect(page).toMatch(new RegExp(`<${c}[\\s/>]`));
    }
  });
});

describe('详情页组件接线契约', () => {
  const page = read('pages/forum/forum-detail.uvue');

  it.each(DETAIL_COMPONENTS)('组件文件存在于 pages/forum/components/%s.uvue', (name) => {
    expect(fs.existsSync(path.join(ROOT, 'pages/forum/components', `${name}.uvue`))).toBe(true);
  });

  it.each(DETAIL_COMPONENTS)('forum-detail.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('详情页模板实际使用全部组件标签', () => {
    for (const c of ['ForumTopicHeader', 'ForumTopicBody', 'ForumReplyList', 'ForumReportDialog']) {
      expect(page).toMatch(new RegExp(`<${c}[\\s/>]`));
    }
  });
});

describe('composable 下沉契约（列表两域 + 详情三域状态离开壳层）', () => {
  const page = read('pages/forum/forum.uvue');
  const feed = read('composables/useTopicFeed.uts');
  const resource = read('composables/useResourcePoints.uts');
  const detailPage = read('pages/forum/forum-detail.uvue');
  const detail = read('composables/useTopicDetail.uts');
  const composer = read('composables/useReplyComposer.uts');
  const report = read('composables/useReport.uts');

  it('列表页两个 composable 存在且以显式 UseXxxResult 返回（error18 先例）', () => {
    expect(feed).toContain('export function useTopicFeed');
    expect(feed).toContain('} as UseTopicFeedResult');
    expect(resource).toContain('export function useResourcePoints');
    expect(resource).toContain('} as UseResourcePointsResult');
  });

  it('列表页壳层仅接线 useTopicFeed；资源 composable 由 panel 组件自持消费（数据所有权单一）', () => {
    expect(page).toContain("import { useTopicFeed } from '../../composables/useTopicFeed'");
    expect(page).not.toContain('useRecruitFeed');
    expect(page).not.toContain('useResourcePoints');
    expect(read('pages/forum/components/forum-resource-panel.uvue')).toContain("from '../../../composables/useResourcePoints'");
  });

  it('列表页壳层 script 不持有招聘 mock 与金币公式（下沉验证）', () => {
    expect(page).not.toContain('mockJobs');
    expect(page).not.toContain('total_study_duration');
    expect(resource).toContain('total_study_duration');
  });

  it('详情页三个 composable 存在且以显式 UseXxxResult 返回', () => {
    expect(detail).toContain('export function useTopicDetail');
    expect(detail).toContain('} as UseTopicDetailResult');
    expect(composer).toContain('export function useReplyComposer');
    expect(composer).toContain('} as UseReplyComposerResult');
    expect(report).toContain('export function useReport');
    expect(report).toContain('} as UseReportResult');
  });

  it('详情页壳层接线三 composable；回复点赞本地态与存储持久化沉入 useTopicDetail', () => {
    expect(detailPage).toContain("import { useTopicDetail } from '../../composables/useTopicDetail'");
    expect(detailPage).toContain("import { useReplyComposer } from '../../composables/useReplyComposer'");
    expect(detailPage).toContain("import { useReport } from '../../composables/useReport'");
    expect(detail).toContain("'forum_reply_like_'");
    expect(detail).toContain('setStorageJSON');
    expect(detailPage).not.toContain('replyLikeState');
  });

  it('composer 提交经 onPosted 回调上抛列表侧变更（禁 defineExpose 反向桥接）', () => {
    expect(composer).toContain('onPosted(reply)');
    expect(detail).toContain('function afterReplyPosted');
    expect(detailPage).toContain('afterReplyPosted(reply)');
  });

  it('举报原因 reason 归弹窗组件自持（打开即重挂载清空），提交经 submit 事件参数上抛', () => {
    const dialog = read('pages/forum/components/forum-report-dialog.uvue');
    expect(dialog).toContain("const reason = ref<string>('')");
    expect(dialog).toContain('v-model="reason"');
    expect(detailPage).not.toContain('reportReason');
    expect(report).toContain('onSubmitReport(reason : string)');
  });

  it('列表查询语义逐字保留：广场 general/discussion，问答 all/question', () => {
    expect(feed).toContain("let scope = 'general'");
    expect(feed).toContain("let category = 'discussion'");
    expect(feed).toMatch(/currentTab\.value === 'hot'[\s\S]*?category = 'question'/);
  });

  it('展示纯函数集中在 utils/forumDisplay.uts 与 forumDetailDisplay.uts，组件经 computed/局部函数包装（守护规则 S：模板禁直调 import 函数）', () => {
    const display = read('utils/forumDisplay.uts');
    for (const fn of ['formatDateStr', 'resolveFileUrl', 'formatCountCompact', 'getContentPreview', 'getFirstImages', 'getAuthorInitial', 'getAvatarColor']) {
      expect(display).toContain(`export function ${fn}`);
    }
    const detailDisplay = read('utils/forumDetailDisplay.uts');
    expect(detailDisplay).toContain('export function formatDateTimeStr');
    expect(detailDisplay).toContain('export type ForumReplyDisplay');
    expect(detailDisplay).toContain('export function previewImages');
    const card = read('pages/forum/components/forum-topic-card.uvue');
    expect(card).toContain("from '../../../utils/forumDisplay'");
    expect(card).toContain('computed<string>(');
  });
});

describe('备考经验 tab 接线契约（#706：第四 tab 进场，复用 TopicCard + experience 类别）', () => {
  const page = read('pages/forum/forum.uvue');
  const feed = read('composables/useTopicFeed.uts');
  const tabBar = read('pages/forum/components/forum-tab-bar.uvue');
  const expBar = read('pages/forum/components/forum-experience-sort-bar.uvue');
  const createPage = read('pages/forum/forum-create.uvue');

  it('tab-bar 有备考经验 tab，点击 onSwitch(experience)', () => {
    expect(tabBar).toContain('备考经验');
    expect(tabBar).toMatch(/@click="onSwitch\('experience'\)"/);
  });

  it('列表查询语义（ADR-0040）：备考经验 scope=all + is_experience=true（判据是管理端认定，不是 category），最多赞→hot、最新→created（#727 排序档）、默认→latest', () => {
    expect(feed).toMatch(/currentTab\.value === 'experience'[\s\S]*?isExperience = 'true'/);
    // 反向锁：经验不是学员自述的意图，不得退化成 category='experience'（存量行已降级，发它必然空）
    expect(feed).not.toMatch(/category = 'experience'/);
    expect(feed).toMatch(/expSort\.value == 'hot' \? 'hot' : \(expSort\.value == 'latest' \? 'created' : 'latest'\)/);
  });

  it('expSort 状态与 onExpSortChange 沉在 useTopicFeed（数据所有权单一，壳层只接线）', () => {
    expect(feed).toContain("const expSort = ref<string>('default')");
    expect(feed).toContain('function onExpSortChange(val : string)');
    expect(feed).toContain('expSort: expSort,');
    expect(feed).toContain('onExpSortChange: (val : string) => onExpSortChange(val),');
  });

  it('壳层接线排序条：仅备考经验 tab 显示，帖子流复用广场 TopicCard（variant 表达式不变）', () => {
    expect(page).toContain("<ForumExperienceSortBar v-if=\"currentTab === 'experience'\" :exp-sort=\"expSort\" @sort-change=\"onExpSortChange\" />");
    expect(page).toMatch(/currentTab == 'square' \|\| currentTab == 'hot' \|\| currentTab == 'experience'/);
    expect(page).toMatch(/:variant="currentTab === 'hot' \? 'qa' : 'square'"/);
  });

  it('排序条组件为默认/最多赞/最新三 chip（原型 forum-6screens 备考经验屏）', () => {
    for (const label of ['默认', '最多赞', '最新']) {
      expect(expBar).toContain(label);
    }
    expect(expBar).toContain("emit('sortChange', val)");
  });

  it('发帖入口（ADR-0040）：经验 tab 退为只读策展流，发布口不再产出 experience，分类行无「备考经验」', () => {
    // 经验 tab 是只读策展流：列表查询走认定 is_experience=true（见上一条），
    // 发帖入口只能落 discussion/question —— 传 experience 后端直接 400
    expect(page).not.toMatch(/scope = 'experience'/);
    expect(createPage).not.toMatch(/value: 'experience'/);
    expect(createPage).toContain("{ label: '广场', value: 'discussion' }");
    expect(createPage).toContain("{ label: '资源', value: 'resource' }");
    expect(createPage).toContain("{ label: '知识问答', value: 'question' }");
  });
});

describe('精选筛选契约（#742 批次三：三 Tab 通用精选筛选 + 列表精选标识，接口契约勿改名）', () => {
  const src = read('api/forum.uts');
  const feed = read('composables/useTopicFeed.uts');
  const page = read('pages/forum/forum.uvue');
  const card = read('pages/forum/components/forum-topic-card.uvue');
  const bar = read('pages/forum/components/forum-featured-filter.uvue');
  const types = read('types/forum.uts');

  it('api：featured 参数按后端契约透传（featured=true|false，空串不传=不过滤；参数名勿改）', () => {
    expect(src).toMatch(/featured : string = ''/);
    expect(src).toMatch(/if \(featured\.length > 0\)/);
    expect(src).toContain("params['featured'] = featured");
  });

  it('api：buildTopic 回显后端 DTO 字段 is_featured（勿改名）', () => {
    expect(src).toContain("is_featured: toBool(obj['is_featured'])");
  });

  it('types：ForumTopic 携带 is_featured', () => {
    expect(types).toMatch(/is_featured : boolean/);
  });

  it('feed：featuredFilter 态与 onFeaturedChange 沉在 useTopicFeed（数据所有权单一），加载时透传', () => {
    expect(feed).toContain("const featuredFilter = ref<string>('')");
    expect(feed).toContain('function onFeaturedChange(val : string)');
    expect(feed).toContain('featuredFilter: featuredFilter,');
    expect(feed).toContain('onFeaturedChange: (val : string) => onFeaturedChange(val),');
    expect(feed).toContain('getForumTopicsApi(page.value, pageSize, scope, sort, order, category, statusParam, featuredFilter.value, isExperience)');
  });

  it('壳层接线：三 Tab 显示精选筛选条（资源 Tab 不显示）；切 Tab 精选筛选回全部帖（跨 Tab 不延续，对齐 Web）', () => {
    expect(page).toContain("<ForumFeaturedFilter v-if=\"currentTab === 'square' || currentTab === 'hot' || currentTab === 'experience'\" :featured=\"featuredFilter\" @featured-change=\"onFeaturedChange\" />");
    expect(page).toMatch(/featuredFilter\.value = ''/);
  });

  it('筛选条组件：全部帖 / ★ 精选 双 chip（选项与 Web 一致），点击 emit featuredChange', () => {
    expect(bar).toContain('全部帖');
    expect(bar).toContain('★ 精选');
    expect(bar).toContain("emit('featuredChange', val)");
  });

  it('列表标识：卡片 is_featured 由壳层扁平下发（R2 默认 false 零 diff），精选徽章文案与 Web 一致', () => {
    expect(page).toContain(':is-featured="item.is_featured"');
    expect(card).toContain('v-if="isFeatured"');
    expect(card).toContain('★ 精选');
  });
});

describe('备考经验认定契约（ADR-0040：is_experience 是管理端认定，不是 category）', () => {
  const src = read('api/forum.uts');
  const feed = read('composables/useTopicFeed.uts');
  const page = read('pages/forum/forum.uvue');

  it('api：is_experience 参数按后端契约透传（isExperience=true，空串不传=不过滤；参数名勿改）', () => {
    expect(src).toMatch(/isExperience : string = ''/);
    expect(src).toMatch(/if \(isExperience\.length > 0\)/);
    expect(src).toContain("params['is_experience'] = isExperience");
  });

  it('api：category 注释不再写「取值含 experience」的旧口径（意图只有两值）', () => {
    expect(src).not.toMatch(/discussion\|question\|experience/);
  });

  it('feed：经验分支清空 category、置 isExperience=true（不把 is_experience 当 category 别名）', () => {
    expect(feed).toMatch(/currentTab\.value === 'experience'[\s\S]*?category = ''[\s\S]*?isExperience = 'true'/);
    expect(feed).not.toMatch(/discussion\|question\|experience/);
  });

  it('经验 tab 空态不引导发布（只读策展流，对齐 Web）', () => {
    expect(page).toContain('还没有备考经验帖');
    expect(page).toContain("v-if=\"currentTab !== 'experience'\" class=\"empty-btn\"");
  });
});

describe('招聘 tab 退场契约（#705 退场部分先行；简历入口重挂留 #705 后续）', () => {
  const page = read('pages/forum/forum.uvue');
  const feed = read('composables/useTopicFeed.uts');
  const tabBar = read('pages/forum/components/forum-tab-bar.uvue');

  it('tab-bar / 壳层 / feed 均不再出现 recruit 分发点', () => {
    expect(tabBar).not.toMatch(/recruit|招聘/);
    expect(page).not.toMatch(/recruit|ForumRecruitPanel|\/pages\/resume\/resume/);
    expect(feed).not.toMatch(/recruit|招聘/);
  });

  it('招聘死件文件已删除（不留未接线的组件与 composable）', () => {
    expect(fs.existsSync(path.join(ROOT, 'pages/forum/components/forum-recruit-panel.uvue'))).toBe(false);
    expect(fs.existsSync(path.join(ROOT, 'composables/useRecruitFeed.uts'))).toBe(false);
  });
});

describe('行为保持契约（手术不改跳转、交互与乐观更新语义）', () => {
  const page = read('pages/forum/forum.uvue');
  const detailPage = read('pages/forum/forum-detail.uvue');
  const detail = read('composables/useTopicDetail.uts');
  const composer = read('composables/useReplyComposer.uts');

  it('头像入口统一跳个人动态（招聘退场后无简历直达分支）', () => {
    expect(page).toMatch(/function onPersonalActivity\(\) : void \{/);
    expect(page).toContain("uni.navigateTo({ url: '/pages/profile/personal-activity' })");
  });

  it('上传资源格子跳 forum-create 资源 tab（#760 复用发布页，upload-resource 退役）', () => {
    const panel = read('pages/forum/components/forum-resource-panel.uvue');
    expect(panel).toContain('/pages/forum/forum-create?scope=resource');
    expect(panel).not.toContain('/pages/resources/upload-resource');
  });

  it('TopicCard 广场/问答共用，变体由壳层 currentTab 下发', () => {
    expect(page).toMatch(/:variant="currentTab === 'hot' \? 'qa' : 'square'"/);
  });

  it('onShow 刷新语义保留：非资源 tab 重载列表，资源区不重复拉取', () => {
    expect(page).toMatch(/if \(currentTab\.value !== 'resource'\) \{\s*loadTopics\(true\)/);
  });

  it('详情页头部删除/编辑/举报三态 v-if 组合逐字保留', () => {
    expect(detailPage).toContain('v-if="canDelete"');
    expect(detailPage).toContain('v-if="canEdit && !canDelete"');
    expect(detailPage).toContain('v-if="!canDelete && !canEdit"');
  });

  it('回复栏留壳层且 v-model 绑定不变（uvue 跨组件 v-model 风险区，composer 状态下沉即达预算）', () => {
    expect(detailPage).toContain('v-model="replyContent"');
    expect(detailPage).toContain('v-if="replyImages.length < 3"');
    expect(detailPage).toContain("{{ submitting ? '发送中' : '发送' }}");
  });

  it('帖子/回复乐观更新与回滚逐字保留', () => {
    expect(detail).toContain('// 乐观更新');
    expect(detail).toContain('t.favorited = wasFavorited');
    expect(detail).toContain('t.liked = wasLiked');
    expect(detail).toContain('setReplyLikeState(reply.id, wasLiked, oldCount)');
    expect(composer).toContain('if (replyContent.value.trim().length == 0)');
    expect(composer).toContain('最多上传 3 张图片');
  });

  it('onLoad 参数解析与 loadDetail 时序保留', () => {
    expect(detailPage).toMatch(/topicId\.value = parseInt\(`\$\{id\}`\)/);
    expect(detailPage).toMatch(/\}\s*loadDetail\(\)\s*\}\)/);
  });
});

describe('api 收紧契约（forum 域经 mapper-callback 出口家族，refs #642/#639）', () => {
  const src = read('api/forum.uts');

  it('forum.uts 引入 getMapped/postMapped/requestMapped 出口，不再 import 裸 get/put', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*postMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).toMatch(/import\s*\{[^}]*requestMapped[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).not.toMatch(/import\s*\{[^}]*\bget\b[^}]*\}\s*from\s*'\.\/request'/);
    expect(src).not.toMatch(/import\s*\{[^}]*\bput\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('GET 面经 getMapped 且映射函数为既有 builder（buildTopicListResult/buildTopicDetail）', () => {
    expect(src).toContain('getMapped<ForumTopicListResult>(\'/forum/topics\'');
    expect(src).toContain('buildTopicListResult(data)');
    expect(src).toContain('getMapped<ForumTopicDetail>(url');
    expect(src).toContain('buildTopicDetail(data)');
  });

  it('POST 面经 postMapped；PUT/DELETE 经 requestMapped 显式 method（保请求形态不变）', () => {
    expect(src).toContain("postMapped<ForumReply>(url, payload");
    expect(src).toContain("postMapped<ForumTopic>('/forum/topics'");
    expect(src).toMatch(/method: 'PUT'/);
    expect(src).toMatch(/method: 'DELETE'/);
  });

  it('守护规则 H 清零：forum.uts 无 catch 参数 : any 注解', () => {
    expect(src).not.toMatch(/\.catch\(\(e : any/);
  });

  it('降级/报错语义保留：my-* 三接口 catch 空列表降级，topics/detail 失败仍抛错', () => {
    const degrades = (src.match(/后端未实现时降级为空列表/g) || []).length;
    expect(degrades).toBe(3);
    expect(src).toContain("errMsg(e, '获取帖子列表失败')");
    expect(src).toContain("errMsg(e, '获取帖子详情失败')");
  });
});

describe('600 行软预算机检（pages/forum/** 达标后锁定）', () => {
  it('forum 模块全部源文件 ≤600 行', () => {
    const over = forumSourceFiles().map((f) => ({
      file: path.relative(ROOT, f),
      lines: fs.readFileSync(f, 'utf8').split('\n').length,
    })).filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });

  it('模块目录 ≤2 层（pages/forum/<file 或 components/<file>>）', () => {
    const deep = forumSourceFiles().filter((f) => {
      const rel = path.relative(path.join(ROOT, 'pages/forum'), f);
      return rel.split(/[\\/]/).length > 2;
    }).map((f) => path.relative(ROOT, f));
    expect(deep).toEqual([]);
  });
});

describe('allowlist 不回潮（forum 域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 forum 域文件（页面与 api 双清零）', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/pages[/\\]forum/);
    expect(block).not.toMatch(/api[/\\]forum\.uts/);
  });
});
