/**
 * 激励批（#2523 / G1 B7）契约：task-center / points-detail / points-rules 三 Tab / 等级徽章贴片
 *
 * 源码契约缝：三页与等级徽章组件都无法被 jest import（同 pointsExchangeRecordsContract 先例），
 * 所以钉源码文本。本批的范围判据来自三处，缺一处就会白做或越界：
 *   ① 票正文「5 屏 / 3 路由」：task-center、points-detail、points-rules（页内三态，不新增路由）
 *      + 贴片 569:3907 等级 Lv1-9 徽章；
 *   ② G1_FINAL 整屏删除清单：任务中心「邀请好友注册 / 转发帖子」、points-detail
 *      「即将过期」chip 与「已过期」Tab **不得按原型恢复**；
 *   ③ N3 #2550 未上线：「游览商城 / 完成积分兑礼」按票面接**形状骨架**（图标 / 落点 / 流水事由），
 *      后端落地前 code 不会下发，页面不改结构即可承接。
 */
const path = require('path');
const fs = require('fs');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));
/** pages.json 路由拍平（#2579 起非 tabBar 页整目录切进分包，分包 path 是相对 root 的短名） */
const { registeredRoutes } = require('../scripts/lib/pages-routes.js');

const TASK = read('pages/points/task-center.uvue');
const DETAIL = read('pages/points/points-detail.uvue');
const RULES = read('pages/points/points-rules.uvue');
const BADGE = read('components/app-level-badge/app-level-badge.uvue');
const USER_ROW = read('pages/profile/components/profile-user-row.uvue');
const PROFILE = read('pages/profile/profile.uvue');
const DISPLAY = read('utils/pointsDisplay.uts');
const MODULES = read('utils/modules.js');
const PAGES_JSON = read('pages.json');

/** 只取渲染面：注释里的裁定解释不算「页面写了这个文案」 */
function tplOf(src) {
  const start = src.indexOf('<template>');
  const end = src.indexOf('</template>');
  if (start < 0 || end < 0) return '';
  return src.slice(start, end).replace(/<!--[\s\S]*?-->/g, '');
}

describe('#2523 激励批：三页路由与页内态', () => {
  it('三条路由都在 pages.json 已注册（本批不新增路由，票面 3 路由）', () => {
    const routes = registeredRoutes(PAGES_JSON);
    for (const p of ['pages/points/task-center', 'pages/points/points-detail', 'pages/points/points-rules']) {
      expect(routes).toContain(p);
    }
  });

  it('points-rules 三 Tab 是页内三态（原型 787:497 / 845:719 / 845:751，不新增页面）', () => {
    const tpl = tplOf(RULES);
    for (const label of ['基础规则', '消费积分详解', '任务中心']) {
      expect(tpl).toContain(label);
    }
    expect(RULES).toContain("const activeTab = ref<string>('basic')");
    expect(RULES).toContain("v-else-if=\"activeTab == 'consume'\"");
  });

  it('points-detail 收支两 Tab（「即将过期」随 G1 删除清单退役，不按原型恢复）', () => {
    const tpl = tplOf(DETAIL);
    expect(tpl).toContain('积分收入');
    expect(tpl).toContain('积分支出');
    expect(tpl).not.toContain('即将过期');
    expect(DETAIL).not.toMatch(/'all'/);
  });
});

describe('#2523 task-center：N3 #2550 两项新任务的形状骨架', () => {
  it('两个新任务码各有独立图标分支，不与既有码共用（后端落地即可显示）', () => {
    expect(TASK).toContain("if (code == 'browse_mall') return '🛒'");
    expect(TASK).toContain("if (code == 'exchange_gift') return '🎁'");
  });

  it('「去完成」落商城的积分兑换分区（两项奖励都发生在货架页），不落论坛/课程', () => {
    expect(TASK).toContain("if (code == 'browse_mall' || code == 'exchange_gift') return { url: '/pages/mall/mall?tab=points', tab: false } as TaskRoute");
    for (const line of TASK.split(/\r?\n/).filter((l) => l.includes("'browse_mall'") || l.includes("'exchange_gift'"))) {
      expect(line).not.toContain('forum');
      expect(line).not.toContain('courses');
    }
  });

  it('领取后流水事由有中文映射（缺映射会把裸 code 甩给用户）', () => {
    expect(DISPLAY).toContain("if (reason == 'browse_mall') return '游览商城'");
    expect(DISPLAY).toContain("if (reason == 'exchange_gift') return '完成积分兑礼'");
  });

  it('原型收口：进度恒显示、动作按钮蓝胶囊、余额卡有今日可得行', () => {
    const tpl = tplOf(TASK);
    expect(tpl).toContain('（{{ task.progress }}/{{ task.total }}）');
    expect(tpl).toContain('今日可得 +{{ todayEarnable }}，累计获得 {{ totalEarned }}');
    expect(TASK).toContain('.task-btn-action');
    expect(TASK).toMatch(/background: linear-gradient\(to bottom right, \$primary-grad-start, \$primary-grad-end\)/);
  });

  it('G1 删除清单：不恢复「邀请好友注册 / 转发帖子」两项任务', () => {
    expect(TASK).not.toContain('邀请好友注册');
    expect(TASK).not.toContain('转发帖子');
  });
});

describe('#2523 points-rules：三 Tab 的任务清单归位', () => {
  it('任务清单与分值留在「任务中心」Tab，且 0 点重置口径仍可见', () => {
    const tpl = tplOf(RULES);
    expect(tpl).toContain('每天 0 点北京时间重置');
    for (const [name, points] of [['每日登录', '5'], ['每日答题 1 次', '10'], ['完成首节课程', '20']]) {
      const i = tpl.indexOf('rule-name">' + name + '</text>');
      expect(i).toBeGreaterThan(-1);
      expect(tpl.indexOf('rule-points">+' + points + '<', i)).toBeGreaterThan(i);
    }
  });

  it('基础规则 Tab 明示暂不过期（根 ADR-0082），且模板无返点/过期口径', () => {
    const tpl = tplOf(RULES);
    expect(tpl).toContain('积分暂不过期');
    expect(tpl).not.toContain('即将过期');
    expect(tpl).not.toContain('每实付');
  });
});

describe('#2523 等级徽章贴片（569:3907：Lv1-9 → 三档 + 2/3/4 星）', () => {
  it('Lv 分档：≤3 初级 / ≤6 中级 / 其余高级，星数 2/3/4', () => {
    expect(BADGE).toContain('if (props.level <= 3) return 1');
    expect(BADGE).toContain('if (props.level <= 6) return 2');
    expect(BADGE).toContain('return [1, 2]');
    expect(BADGE).toContain('return [1, 2, 3]');
    expect(BADGE).toContain('return [1, 2, 3, 4]');
    for (const label of ['初级用户', '中级用户', '高级用户']) {
      expect(BADGE).toContain(`'${label}'`);
    }
  });

  it('包内路径写字面量（守护只看表达式文本），无 webp', () => {
    for (const p of ['badge-primary.png', 'badge-intermediate.png', 'badge-advanced.png', 'star.png']) {
      expect(BADGE).toContain(`src="/static/icons/level/${p}"`);
    }
    expect(BADGE.toLowerCase()).not.toContain('.webp');
  });

  it('切图文件在位且是真 PNG（包内禁 webp，ADR-0066）', () => {
    for (const p of ['badge-primary.png', 'badge-intermediate.png', 'badge-advanced.png', 'star.png']) {
      const file = path.join(ROOT, 'static/icons/level', p);
      expect(fs.existsSync(file)).toBe(true);
      const head = fs.readFileSync(file).subarray(0, 8);
      // PNG 魔术字节 89 50 4E 47；RIFF/WEBP 会以 52 49 46 46 开头
      expect(head[0]).toBe(0x89);
      expect(head[1]).toBe(0x50);
      expect(head[2]).toBe(0x4e);
      expect(head[3]).toBe(0x47);
    }
  });

  it('「我的」用户行接线到共用组件（任务/我的共用同一份规格）', () => {
    expect(USER_ROW).toContain("import appLevelBadge from '../../../components/app-level-badge/app-level-badge.uvue'");
    expect(USER_ROW).toContain('<app-level-badge :level="userLevel" />');
    // 段位文字 + 5 颗文本星已被徽章规格替代，不保留旧渲染
    expect(USER_ROW).not.toContain('🏅');
    expect(USER_ROW).not.toContain('star-active');
    expect(PROFILE).not.toContain(':completed-courses');
  });

  it('组件登记为跨切面基础设施（别让别的页 import 时触发跨模块私有件消费）', () => {
    expect(MODULES).toContain("'components/app-level-badge'");
  });
});
