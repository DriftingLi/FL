/**
 * 个人动态手术契约测试（refs #677 / T03e，parent #641）
 *
 * 沿用源码契约缝（.uvue 不可 jest import）。先例：personalInfoContract、wrongQuestionsContract。
 * 钉住：三组件存在与接线、tab 值逐字冻结、卡片扁平 props、
 * 取数逻辑所有权留页面、600 预算机检、allowlist 不回潮。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const PAGE = 'pages/profile/personal-activity.uvue';
const TABS = 'pages/profile/components/activity-tab-bar.uvue';
const USER = 'pages/profile/components/activity-user-card.uvue';
const CARD = 'pages/profile/components/activity-topic-card.uvue';

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

  it('ActivityTabBar 挂载 ×3（主 tab + likes 子 tab + interact 子 tab），ActivityUserCard ×1', () => {
    expect(page.match(/<ActivityTabBar/g)).toHaveLength(3);
    expect(page.match(/<ActivityUserCard/g)).toHaveLength(1);
    expect(page).toContain(`v-if="currentTab === 'likes'" :sub="true"`);
    expect(page).toContain(`v-if="currentTab === 'interact'" :sub="true"`);
  });

  it('ActivityTopicCard 挂载 ×7，v-for 别名 item 逐处保留（票面禁改名）且 :key="idx" 不变', () => {
    expect(page.match(/<ActivityTopicCard v-for="\(item, idx\) in /g)).toHaveLength(7);
    expect(page.match(/:key="idx"/g)).toHaveLength(7);
    for (const list of ['topics', 'likedTopics', 'favoriteItems', 'myQuestions', 'myReplies', 'observedTopics', 'viewHistory']) {
      expect(page).toContain(`in ${list}"`);
    }
  });

  it('点击映射逐变体钉死：id 类 tab 传 item.id，疑问/回复传 item.topic_id，收藏传整对象', () => {
    expect(page).toContain('@tap="onTopicClick(item.id)"');
    expect(page.match(/@tap="onTopicClick\(item\.id\)"/g)).toHaveLength(4);
    expect(page.match(/@tap="onTopicClick\(item\.topic_id\)"/g)).toHaveLength(2);
    expect(page).toContain('@tap="onFavoriteClick(item)"');
  });

  it('tab 初始值与分页参数不变（posts/liked/replies、pageSize=10）', () => {
    expect(page).toContain("const currentTab = ref<string>('posts')");
    expect(page).toContain("const likesSubTab = ref<string>('liked')");
    expect(page).toContain("const interactSubTab = ref<string>('replies')");
    expect(page).toContain('const pageSize = 10');
    expect(page).toContain('const page = ref<number>(1)');
  });
});

describe('tab 值逐字冻结（含「游览记录」错别字按 UI 冻结保留）', () => {
  const page = read(PAGE);

  it('三组 keys/labels 常量与手术前模板值一致', () => {
    expect(page).toContain("const mainTabKeys : string[] = ['posts', 'likes', 'interact', 'views']");
    expect(page).toContain("const mainTabLabels : string[] = ['我的帖子', '赞/收藏', '互动', '游览记录']");
    expect(page).toContain("const likesSubKeys : string[] = ['liked', 'favorited']");
    expect(page).toContain("const interactSubKeys : string[] = ['questions', 'replies', 'observed']");
    expect(page).toContain("const interactSubLabels : string[] = ['我的疑问', '我的回复', '我的围观']");
  });

  it('tab 切换处理器名不变（onSwitchTab/onSwitchLikesSubTab/onSwitchInteractSubTab）', () => {
    for (const fn of ['onSwitchTab', 'onSwitchLikesSubTab', 'onSwitchInteractSubTab']) {
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
      'repliesCount?: number', 'tagText?: string', 'showStats?: boolean', 'refHeader?: boolean', 'hasContent?: boolean']) {
      expect(card).toContain(p);
    }
    expect(card).not.toMatch(/item\??:|Topic\??:|Reply\??:/);
    expect(card).not.toMatch(/as unknown as|ref<\s*any/);
  });

  it('变体渲染与旧内联版一一对应（stats 仅 showStats、tag 仅 tagText 非空、回复头仅 refHeader、preview 仅 hasContent）', () => {
    expect(card).toContain('v-if="refHeader" class="reply-ref-title"');
    expect(card).toContain('v-else class="topic-title"');
    expect(card).toContain('v-if="hasContent" class="topic-preview"');
    expect(card).toContain('v-if="showStats" class="topic-stats"');
    expect(card).toContain('v-if="tagText.length > 0" class="tag-text"');
    // 模板经本地 displayTime 消费 utils/format（守护规则 S：模板直调 import 函数 = Kotlin error18 invoke）
    expect(card).toContain('<text class="topic-time">{{ displayTime(createdDate) }}</text>');
    expect(card).not.toMatch(/\{\{ formatDateStr\(/);
    expect(card).toContain('function displayTime(dateStr : string) : string');
    expect(card).toContain('return formatDateStr(dateStr)');
  });

  it('页面 favorites 点击仍带对象参数（onFavoriteClick(item) 消费面不变）', () => {
    expect(read(PAGE)).toContain('@tap="onFavoriteClick(item)"');
  });
});

describe('取数逻辑所有权留在页面（列表加载非组件职责）', () => {
  const page = () => read(PAGE);

  it('七大 loader + loadData + checkNoMore + hasData 全在页面', () => {
    for (const fn of ['function checkNoMore', 'async function loadMyTopics', 'async function loadLikedTopics',
      'async function loadFavorites', 'async function loadMyReplies', 'async function loadMyQuestions',
      'async function loadObservedTopics', 'async function loadViewHistory', 'async function loadData',
      'const hasData = computed<boolean>']) {
      expect(page()).toContain(fn);
    }
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

  it('api 消费面不变：六 API import 行逐字保留', () => {
    expect(page()).toContain("import { getMyTopicsApi, getMyRepliesApi, getMyLikedTopicsApi, getMyObservedTopicsApi, getMyViewHistoryApi } from '../../api/forum'");
    expect(page()).toContain("import { getFavoritesApi } from '../../api/favorite'");
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
      ['.topic-card', CARD], ['.reply-ref-title', CARD], ['.tag-text', CARD], ['.stat-text', CARD],
    ]) {
      expect(page).not.toContain(moved);
      expect(read(target)).toContain(moved);
    }
  });

  it('七条空态文案 + 页面标题逐字保留', () => {
    for (const t of ['发布一篇帖子，参与互动', '还没有点赞过帖子', '还没有收藏过内容', '还没有提出过疑问',
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

describe('allowlist 不回潮（个人动态域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 personal-activity 手术相关文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/personal-activity|activity-tab|activity-user|activity-topic/);
  });
});
