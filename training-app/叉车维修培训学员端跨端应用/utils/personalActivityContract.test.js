/**
 * 个人动态手术契约测试（refs #677 / T03e，parent #641）
 *
 * 沿用源码契约缝（.uvue 不可 jest import）。先例：personalInfoContract、wrongQuestionsContract。
 * 钉住：三组件存在与接线、tab 值逐字冻结、卡片扁平 props、
 * 取数逻辑所有权留页面、600 预算机检、allowlist 不回潮。
 *
 * ## #1159 改判（2026-09-18，维护者裁定 C）
 *
 * 原票面把「收藏条目点了没反应」判为 `onFavoriteClick` 缺 chapter/question 分支，
 * 并在本页补齐了落点表。改判理由与结论：
 *   - **同一份数据有两个列表承载面**（`favorites.uvue` 与 `personal-activity.uvue`），
 *     各自维护一份落点表 —— 本族缺陷的结构成因（#1089 / PR #1147 / #1159 三票同源）。
 *   - 裁定：**收藏的唯一列表承载面 = 「我的 → 收藏夹」**；个人动态**不**承载收藏，
 *     「赞/收藏」收成平铺的「赞过」（词表口径见 `CONTEXT.md` 的「赞过（liked topics）」）。
 *   - 故本页的落点表**整体删除**，不是补齐；#1159 的 harness 与 27 用例一并作废
 *     （`favorites.uvue` 侧的同类守护 `favoritesLandingContract.test.js` 保留且是唯一一份）。
 *   - 连带：卡片的 `tagText` prop / `.tag-text` 样式（摘除前唯一消费方就是那一格）一并删除。
 *
 * 下面是**反向守护**：钉住「收藏面不得回潮」，并把「摘除第二处」的前提
 * （第一处承载面的筛选入口仍在）钉在源码上 —— 前提被掏空时本守护判红。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const PAGE = 'pages/profile/personal-activity.uvue';
const FAVORITES = 'pages/profile/favorites.uvue';
const TABS = 'pages/profile/components/activity-tab-bar.uvue';
const USER = 'pages/profile/components/activity-user-card.uvue';
const CARD = 'pages/profile/components/activity-topic-card.uvue';

/** 取模板段并剥掉 HTML 注释：注释里解释「为什么没有收藏」不算入口，真入口才算 */
function templateOf(src) {
  const start = src.indexOf('<template>');
  const end = src.indexOf('</template>');
  if (start === -1 || end === -1) return '';
  return src.slice(start, end).replace(/<!--[\s\S]*?-->/g, ' ');
}

/** 剥注释（HTML / 块 / 行）：注释里解释「某个 prop 为什么删掉」不算残留，代码里引用才算 */
function stripComments(src) {
  return src
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/\/\/[^\n]*/g, ' ');
}

describe('拆出物存在契约（T03e 安置：头 + tab + 卡片组件化）', () => {
  it.each([TABS, USER, CARD])('%s 存在', (rel) => {
    expect(fs.existsSync(path.join(ROOT, rel))).toBe(true);
  });
});

describe('页面接线契约（显式 import + 挂载，Q17 安置）', () => {
  const page = read(PAGE);

  it('三组件均显式 import（非 easycom）', () => {
    for (const rel of ['./components/activity-tab-bar.uvue', './components/activity-user-card.uvue', './components/activity-topic-card.uvue']) {
      expect(page).toContain(rel);
    }
  });

  it('ActivityTabBar 挂载 ×2（主 tab + interact 子 tab），ActivityUserCard ×1', () => {
    expect(page.match(/<ActivityTabBar/g)).toHaveLength(2);
    expect(page.match(/<ActivityUserCard/g)).toHaveLength(1);
    expect(page).toContain(`v-if="currentTab === 'interact'" :sub="true"`);
  });

  it('ActivityTopicCard 挂载 ×6，v-for 别名 item 逐处保留（票面禁改名）且 :key="idx" 不变', () => {
    expect(page.match(/<ActivityTopicCard v-for="\(item, idx\) in /g)).toHaveLength(6);
    expect(page.match(/:key="idx"/g)).toHaveLength(6);
    for (const list of ['topics', 'likedTopics', 'myQuestions', 'myReplies', 'observedTopics', 'viewHistory']) {
      expect(page).toContain(`in ${list}"`);
    }
  });

  it('点击映射逐变体钉死：id 类 tab 传 item.id，疑问/回复传 item.topic_id（收藏整对象形态已随承载面摘除）', () => {
    expect(page.match(/@tap="onTopicClick\(item\.id\)"/g)).toHaveLength(4);
    expect(page.match(/@tap="onTopicClick\(item\.topic_id\)"/g)).toHaveLength(2);
    expect(page.match(/@tap="/g)).toHaveLength(6);
  });

  it('tab 初始值与分页参数不变（posts/replies、pageSize=10）', () => {
    expect(page).toContain("const currentTab = ref<string>('posts')");
    expect(page).toContain("const interactSubTab = ref<string>('replies')");
    expect(page).toContain('const pageSize = 10');
    expect(page).toContain('const page = ref<number>(1)');
  });
});

describe('tab 值逐字冻结（含「游览记录」错别字按 UI 冻结保留）', () => {
  const page = read(PAGE);

  it('三组 keys/labels 常量与手术前模板值一致（唯「赞/收藏」→「赞过」，ADR-0018）', () => {
    expect(page).toContain("const mainTabKeys : string[] = ['posts', 'likes', 'interact', 'views']");
    expect(page).toContain("const mainTabLabels : string[] = ['我的帖子', '赞过', '互动', '游览记录']");
    expect(page).toContain("const interactSubKeys : string[] = ['questions', 'replies', 'observed']");
    expect(page).toContain("const interactSubLabels : string[] = ['我的疑问', '我的回复', '我的围观']");
  });

  it('tab 切换处理器名不变（onSwitchTab/onSwitchInteractSubTab；likes 子 tab 处理器已摘）', () => {
    for (const fn of ['onSwitchTab', 'onSwitchInteractSubTab']) {
      expect(page).toContain(`@switch="${fn}"`);
      expect(page).toContain(`function ${fn}(`);
    }
  });
});

describe('卡片组件契约（纯展示 + 扁平 props，#687 教训）', () => {
  const card = read(CARD);
  const tabs = read(TABS);
  const user = read(USER);

  it('三组件零 api/store/定时器/uni 路由（纯展示；切换决策与取数留页面）', () => {
    for (const src of [card, tabs, user]) {
      expect(src).not.toMatch(/import.*(api|stores)\//);
      expect(src).not.toMatch(/uni\.(navigateTo|showToast|request)/);
      expect(src).not.toMatch(/setInterval|ref</);
    }
    expect(card).toContain("defineEmits(['tap'])");
    expect(tabs).toContain("defineEmits(['switch'])");
    expect(user).not.toMatch(/defineEmits/);
  });

  it('卡片 props 全扁平原始值（无对象 prop、无 as unknown as、无 ref<any>）', () => {
    for (const p of ['title?: string', 'content?: string', 'createdDate?: string', 'likesCount?: number',
      'repliesCount?: number', 'showStats?: boolean', 'refHeader?: boolean', 'hasContent?: boolean']) {
      expect(card).toContain(p);
    }
    expect(card).not.toMatch(/item\??:|Topic\??:|Reply\??:/);
    expect(card).not.toMatch(/as unknown as|ref<\s*any/);
  });

  it('变体渲染与旧内联版一一对应（stats 仅 showStats、回复头仅 refHeader、preview 仅 hasContent）', () => {
    expect(card).toContain('v-if="refHeader" class="reply-ref-title"');
    expect(card).toContain('v-else class="topic-title"');
    expect(card).toContain('v-if="hasContent" class="topic-preview"');
    expect(card).toContain('v-if="showStats" class="topic-stats"');
    // 模板经本地 displayTime 消费 utils/format（守护规则 S：模板直调 import 函数 = Kotlin error18 invoke）
    expect(card).toContain('<text class="topic-time">{{ displayTime(createdDate) }}</text>');
    expect(card).not.toMatch(/\{\{ formatDateStr\(/);
    expect(card).toContain('function displayTime(dateStr : string) : string');
    expect(card).toContain('return formatDateStr(dateStr)');
  });

  it('tagText 变体整体摘除（零消费方的死 prop / 死样式不留；注释里的说明不算残留）', () => {
    // 摘除前 tagText 的**唯一**消费方就是个人动态的「收藏」那一格；承载面摘除后它成了死 prop。
    // 留着的代价：收藏面回潮只需少写一处接线就「看起来还在」—— 故按 ADR-0018 补遗一并删除。
    const code = stripComments(card);
    expect(code).not.toMatch(/tagText/);
    expect(code).not.toContain('.tag-text');
    expect(read(PAGE)).not.toContain(':tag-text=');
    // 类型标签的唯一来源 = 收藏夹（把 target_type 映射成中文，不是裸枚举）
    expect(read(FAVORITES)).toContain('function getTypeLabel(');
  });
});

describe('取数逻辑所有权留在页面（列表加载非组件职责）', () => {
  const page = () => read(PAGE);

  it('六大 loader + loadData + checkNoMore + hasData 全在页面（loadFavorites 随承载面摘除）', () => {
    for (const fn of ['function checkNoMore', 'async function loadMyTopics', 'async function loadLikedTopics',
      'async function loadMyReplies', 'async function loadMyQuestions',
      'async function loadObservedTopics', 'async function loadViewHistory', 'async function loadData',
      'const hasData = computed<boolean>']) {
      expect(page()).toContain(fn);
    }
    expect(page()).not.toContain('function loadFavorites');
  });

  it('展示 helper 唯一实现点：formatDateStr 收敛 utils/format（卡片 import 消费，零第二实现）', () => {
    expect(page()).not.toContain('function formatDateStr');
    expect(page()).not.toContain('getContentPreview');
    expect(read(CARD)).not.toContain('function formatDateStr');
    expect(read(CARD)).toContain("import { formatDateStr } from '../../../utils/format'");
    expect(read('utils/format.uts')).toContain('export function formatDateStr');
  });

  it('contentPreview 唯一定义点在卡片（改名不改体，replace+80 截断逐字保留）', () => {
    expect(page()).not.toContain('contentPreview');
    const card = read(CARD);
    expect(card).toContain('function contentPreview(text : string) : string');
    expect(card).toContain("text.replace('\\n', ' ')");
    expect(card).toContain('single.substring(0, 80)');
  });

  it('api 消费面：forum 五 API import 行逐字保留，收藏 API 零引用', () => {
    expect(page()).toContain("import { getMyTopicsApi, getMyRepliesApi, getMyLikedTopicsApi, getMyObservedTopicsApi, getMyViewHistoryApi } from '../../api/forum'");
    expect(page()).not.toContain('api/favorite');
  });
});

describe('UI 像素结构锁定（样式归属互斥，无双写）', () => {
  const page = read(PAGE);

  it('页面保留骨架/空态/列表容器样式；tab、user-card、topic-card 块已迁出且页面无残留', () => {
    for (const cls of ['.container', '.page-header', '.content-scroll', '.empty-state', '.publish-btn', '.list-section', '.loading-more']) {
      expect(page).toContain(cls);
    }
    for (const [moved, target] of [
      ['.tab-item.active', TABS], ['.sub-tab-text.active', TABS],
      ['.user-avatar-placeholder', USER], ['.level-badge', USER],
      ['.topic-card', CARD], ['.reply-ref-title', CARD], ['.stat-text', CARD],
    ]) {
      expect(page).not.toContain(moved);
      expect(read(target)).toContain(moved);
    }
  });

  it('六条空态文案（收藏那条已随承载面摘除）+ 页面标题逐字保留', () => {
    for (const t of ['发布一篇帖子，参与互动', '还没有点赞过帖子', '还没有提出过疑问',
      '还没有回复过帖子', '还没有围观过帖子', '还没有浏览记录', '个人动态']) {
      expect(page).toContain(t);
    }
  });
});

describe('600 行软预算机检（personal-activity 手术文件）', () => {
  it('页面 + 三组件全部 ≤600 行', () => {
    const over = [PAGE, TABS, USER, CARD]
      .map((rel) => ({ file: rel, lines: read(rel).split('\n').length }))
      .filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });
});

describe('#1159 改判：个人动态不承载收藏（ADR-0018「收藏的唯一列表承载面」）', () => {
  const page = read(PAGE);

  it('页面源码零收藏面残留（api / 类型 / 状态 / 分支 / 落点函数 / 子 tab）', () => {
    expect(page).not.toMatch(/getFavoritesApi|FavoriteItem|favoriteItems|onFavoriteClick/);
    expect(page).not.toMatch(/likesSubKeys|likesSubLabels|likesSubTab|onSwitchLikesSubTab/);
    expect(page).not.toContain('favorited');
  });

  it('模板（剥注释）里不存在「收藏」任何入口', () => {
    expect(templateOf(page)).not.toContain('收藏');
  });

  it('子 tab 栏只剩「互动」一处（「赞过」是平铺 tab，第二处 :sub 不得回潮）', () => {
    expect(page.match(/:sub="true"/g)).toHaveLength(1);
    expect(page).toContain(`v-if="currentTab === 'interact'" :sub="true"`);
  });

  it('摘除的前提仍成立：唯一承载面已注册、筛选入口未掏空', () => {
    // 「摘掉第二处」的前提是「第一处完整」。落点表的完整性（五类 + 章节双键 + 缺课程提示）
    // 由 favoritesLandingContract.test.js 用行为断言守护 —— 此处**不复制第二套断言**
    // （本族缺陷的成因正是两份落点表），只钉承载面的**筛选入口**这一面（那边未覆盖）。
    expect(read('pages.json')).toContain('pages/profile/favorites');
    expect(fs.existsSync(path.join(ROOT, 'utils/favoritesLandingContract.test.js'))).toBe(true);
    const fav = read(FAVORITES);
    for (const t of ["value: 'course'", "value: 'chapter'", "value: 'topic'", "value: 'featured'", "value: 'question'"]) {
      expect(fav).toContain(t);
    }
  });

  it('落点表全仓只有一份（收藏侧），本页不得再长出 onItemClick/onFavoriteClick', () => {
    const pageSrc = read(PAGE);
    expect(pageSrc).not.toMatch(/function on(ItemClick|FavoriteClick)/);
    expect(read(FAVORITES)).toContain('function onItemClick(');
  });
});

describe('allowlist 不回潮（个人动态域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 personal-activity 手术相关文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/personal-activity|activity-tab|activity-user|activity-topic/);
  });
});
