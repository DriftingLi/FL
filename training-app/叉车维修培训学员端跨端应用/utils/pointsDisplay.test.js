/**
 * 积分域展示纯函数单元测试（refs #709）
 *
 * utils/pointsDisplay.uts 是 UTS，Node 无法直接 import，按仓库惯例（format.test.js /
 * checkinCalendar.test.js）以镜像实现验证算法行为；文末的「镜像同步」用例把 .uts 源码
 * 与镜像表逐键对齐，防止两份实现悄悄分叉。
 *
 * 收支方向口径与 Web 端 frontend/src/utils/pointsReason.ts 的 deltaKind 互为镜像。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
// ===== 镜像实现（与 pointsDisplay.uts 保持一致）=====

const REASON_LABELS = {
  ai_tokens: 'AI 学习助手对话',
  redeem_course: '兑换课程',
  redeem_real_paper: '兑换真题卷',
  accepted_bonus: '问答被采纳奖励',
  accept_action: '采纳回答奖励',
  featured_bonus: '帖子被加精奖励',
  contribution_approved: '投稿审核通过',
  contribution_tier: '投稿下载达阶',
  admin_penalty: '违规扣减',
  rollback: '系统退回',
  checkin: '每日打卡',
  daily_checkin: '每日打卡',
  daily_login: '每日登录',
  daily_quiz: '每日答题',
  daily_browse: '浏览帖子',
  growth_post: '发布帖子',
  growth_reply: '回复帖子',
  growth_mock: '完成模考',
  growth_first_experience: '发布首篇备考经验',
  newbie_profile_basic: '完善基础资料',
  newbie_profile_contact: '完善联系资料',
  newbie_credential: '选定目标证件',
  newbie_first_course: '完成首节课程',
};

const REF_TYPE_LABELS = {
  task: '任务',
  checkin: '打卡',
  course: '课程',
  real_paper: '真题卷',
  shop: '商城',
  ai: 'AI 助手',
  ai_chat: 'AI 助手',
  forum_topic: '社区',
  contribution: '投稿',
  admin: '系统',
};

function ledgerReasonLabel(reason, delta) {
  if (REASON_LABELS[reason]) return REASON_LABELS[reason];
  if (reason.indexOf('redeem_') === 0) return '商城兑换';
  if (delta >= 0) return '积分获得';
  return '积分消耗';
}

function ledgerRefTypeText(refType) {
  return REF_TYPE_LABELS[refType] || '其他';
}

function deltaText(delta) {
  if (delta >= 0) return '+' + delta.toString();
  return delta.toString();
}

function deltaKind(delta) {
  return delta >= 0 ? 'in' : 'out';
}

function earnablePoints(tasks) {
  let sum = 0;
  for (const t of tasks) {
    if (t.status !== 'claimed') sum += t.points;
  }
  return sum;
}

/** ref_type → 图标 class（与 .uts ledgerIconClass 同表） */
function ledgerIconClass(refType) {
  if (refType === 'task') return 'icon-task';
  if (refType === 'checkin') return 'icon-checkin';
  if (refType === 'course' || refType === 'real_paper' || refType === 'shop') return 'icon-redeem';
  if (refType === 'ai' || refType === 'ai_chat') return 'icon-redeem';
  if (refType === 'forum_topic' || refType === 'contribution') return 'icon-community';
  if (refType === 'admin') return 'icon-admin';
  return 'icon-default';
}

/** ref_type → 字形（与 .uts ledgerIconGlyph 同表） */
function ledgerIconGlyph(refType) {
  if (refType === 'task') return '◆';
  if (refType === 'checkin') return '✓';
  if (refType === 'course' || refType === 'real_paper' || refType === 'shop') return '★';
  if (refType === 'forum_topic' || refType === 'contribution') return '✎';
  if (refType === 'admin') return '!';
  return '·';
}

/** 字形表（供 it.each 展开；与 ledgerIconGlyph 镜像同源） */
function GlyphMap() {
  return {
    task: '◆',
    checkin: '✓',
    course: '★',
    real_paper: '★',
    shop: '★',
    forum_topic: '✎',
    contribution: '✎',
    admin: '!',
    unknown_x: '·',
  };
}

// ===== 用例 =====

describe('ledgerReasonLabel：流水事由中文（口径同 Web #512）', () => {
  it.each(Object.entries(REASON_LABELS))('已知 reason %s → %s', (reason, label) => {
    expect(ledgerReasonLabel(reason, 10)).toBe(label);
  });

  it('商城兑换动态键 redeem_<sku> 按前缀识别', () => {
    expect(ledgerReasonLabel('redeem_paper_pack_3', -20)).toBe('商城兑换');
  });

  it('未收录 reason 按 delta 方向兜底，不把原始 code 甩给用户', () => {
    expect(ledgerReasonLabel('some_future_reason', 5)).toBe('积分获得');
    expect(ledgerReasonLabel('some_future_reason', -5)).toBe('积分消耗');
    expect(ledgerReasonLabel('', 0)).toBe('积分获得');
  });

  it('退役任务 daily_checkin（#587 删配置）与打卡直记 checkin 都有中文，不显示裸 code', () => {
    expect(ledgerReasonLabel('daily_checkin', 5)).toBe('每日打卡');
    expect(ledgerReasonLabel('checkin', 10)).toBe('每日打卡');
  });
});

describe('ledgerRefTypeText / deltaText / deltaKind', () => {
  it('ref_type 全枚举有中文，未知归「其他」', () => {
    expect(ledgerRefTypeText('task')).toBe('任务');
    expect(ledgerRefTypeText('real_paper')).toBe('真题卷');
    expect(ledgerRefTypeText('ai_chat')).toBe('AI 助手');
    expect(ledgerRefTypeText('')).toBe('其他');
    expect(ledgerRefTypeText('brand_new_type')).toBe('其他');
  });

  it('delta 文本自带符号（后端 delta 已含方向）', () => {
    expect(deltaText(7)).toBe('+7');
    expect(deltaText(-30)).toBe('-30');
    expect(deltaText(0)).toBe('+0');
  });

  it('方向判定只看 delta 正负（rollback 等对冲不受 reason 影响，同 Web deltaKind）', () => {
    expect(deltaKind(1)).toBe('in');
    expect(deltaKind(-1)).toBe('out');
    expect(deltaKind(0)).toBe('in');
  });
});

describe('earnablePoints：今日可得上限（口径同 Web todayEarnable）', () => {
  it('todo / claimable 计入，claimed 排除', () => {
    const tasks = [
      { code: 'a', status: 'todo', points: 5 },
      { code: 'b', status: 'claimable', points: 10 },
      { code: 'c', status: 'claimed', points: 20 },
    ];
    expect(earnablePoints(tasks)).toBe(15);
  });

  it('空列表与全已领均为 0（不是硬编码 80）', () => {
    expect(earnablePoints([])).toBe(0);
    expect(earnablePoints([{ code: 'a', status: 'claimed', points: 20 }])).toBe(0);
  });
});

describe('ledgerIconClass / ledgerIconGlyph：ref_type → 图标（class 与字形同处收口）', () => {
  const CLASS_OF = {
    task: 'icon-task',
    checkin: 'icon-checkin',
    course: 'icon-redeem',
    real_paper: 'icon-redeem',
    shop: 'icon-redeem',
    ai: 'icon-redeem',
    ai_chat: 'icon-redeem',
    forum_topic: 'icon-community',
    contribution: 'icon-community',
    admin: 'icon-admin',
    unknown_x: 'icon-default',
    '': 'icon-default',
  };

  it.each(Object.entries(CLASS_OF))('ref_type %s → class %s', (refType, cls) => {
    expect(ledgerIconClass(refType)).toBe(cls);
  });

  it('兑换类（课程/真题/商城）与 AI 类共用一套底色，社区与投稿共用', () => {
    expect(new Set(['course', 'real_paper', 'shop', 'ai', 'ai_chat'].map(ledgerIconClass))).toEqual(
      new Set(['icon-redeem']),
    );
    expect(new Set(['forum_topic', 'contribution'].map(ledgerIconClass))).toEqual(new Set(['icon-community']));
  });

  it.each(Object.entries(GlyphMap()))('ref_type %s → 字形 %s', (refType, glyph) => {
    expect(ledgerIconGlyph(refType)).toBe(glyph);
  });

  it('未知 ref_type 落到占位字形，不留空文本', () => {
    expect(ledgerIconGlyph('unknown_x')).toBe('·');
    expect(ledgerIconGlyph('')).toBe('·');
  });

  it('.uts 内 class 与字形两张表按同一 ref_type 键成文（防只改一张）', () => {
    const src = readText(path.join(__dirname, 'pointsDisplay.uts'));
    for (const key of Object.keys(CLASS_OF)) {
      if (key === '' || key === 'unknown_x') continue;
      expect(src).toMatch(new RegExp(`refType == '${key}'`));
    }
  });
});

const WEB_TABLE = path.join(__dirname, '..', '..', '..', 'frontend', 'src', 'utils', 'pointsReason.ts');
// 全仓 checkout 才跑得动跨端比对；单模块局部 checkout 时显式 skip（在报告里可见，不假绿）
const describeWeb = fs.existsSync(WEB_TABLE) ? describe : describe.skip;

describeWeb('跨端文案表不分叉：移动端 .uts vs Web pointsReason.ts', () => {
  const utsSrc = readText(path.join(__dirname, 'pointsDisplay.uts'));

  it('Web 表内每个 reason 键，移动端文案必须逐字相同（改文案要两端同改）', () => {
    const webSrc = readText(WEB_TABLE);
    const from = webSrc.indexOf('const REASON_LABELS');
    const block = webSrc.slice(from, webSrc.indexOf('\n}', from));
    const pairs = [...block.matchAll(/(\w+):\s*\{\s*label:\s*'([^']+)'/g)];
    expect(pairs.length).toBeGreaterThan(5); // 解析失败立刻显形，不空跑假绿
    for (const [, reason, label] of pairs) {
      expect(utsSrc).toContain(`if (reason == '${reason}') return '${label}'`);
    }
  });

  it('移动端补齐的键（Web 表未收录）在此显式登记，防悄悄长第三份口径', () => {
    for (const extra of ['checkin', 'featured_bonus', 'daily_login', 'newbie_credential']) {
      expect(utsSrc).toContain(`if (reason == '${extra}')`);
    }
  });
});

describe('镜像同步：pointsDisplay.uts 与本文件表逐键一致', () => {
  const src = readText(path.join(__dirname, 'pointsDisplay.uts'));

  it.each(Object.entries(REASON_LABELS))('reason %s → %s 在 .uts 内成文', (reason, label) => {
    expect(src).toContain(`if (reason == '${reason}') return '${label}'`);
  });

  it.each(Object.entries(REF_TYPE_LABELS))('ref_type %s → %s 在 .uts 内成文', (refType, label) => {
    expect(src).toContain(`if (refType == '${refType}') return '${label}'`);
  });

  it('.uts 未收录 reason 也走 delta 兜底，与镜像同式', () => {
    expect(src).toContain("if (reason.indexOf('redeem_') == 0) return '商城兑换'");
    expect(src).toContain("if (delta >= 0) return '积分获得'");
  });

  it('.uts 无日历数学残留（#1943 过期筛选退役后不再 import 日期 util）', () => {
    expect(src).not.toContain('checkinCalendar');
    expect(src).not.toContain('formatDate');
    expect(src).not.toContain('y * 10000');
  });
});
