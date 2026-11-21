/**
 * 微信资料一键填充 · 行为级测试（#2712）
 *
 * 真执行两个纯逻辑件（去类型后当 JS 跑），钉 spec 里「只有跑起来才看得见」的判据：
 *   ① 空值判据：占位昵称的**全部生成形态**都算空，真叫「微信学员…」的用户不算空；
 *   ② 待填集合：只补空值，且**待审字段视为不可填**（后端对同字段已有待审会硬拒）；
 *   ③ 重试记忆：已成功的字段不再重做（头像上传成功清临时路径、昵称成功不重提）；
 *   ④ 返回栈：from=login 回 choose-cert，其余交页面按栈回退、回不去兜底首页；
 *   ⑤ 昵称截断：30 字上限，超了截断后提交；
 *   ⑥ pending_fields 解析：缺省即空数组，逐字段判定。
 *
 * 缝 = utils/utsHarness.js 的 loadUts（ADR-0073 唯一测试 seam）。注入的是真件
 * utils/wechatProfileFill.uts 的导出，没有手抄镜像。
 *
 * 成对取证：把占位判据退回前缀形态（startsWith），证明「完整模式」这条判据有牙。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const SHARED = path.join(__dirname, '..', 'utils', 'wechatProfileFill.uts');
const FLOWS = path.join(__dirname, '..', 'pages', 'profile-setup', 'composables', 'profile-setup-flows.uts');

/** 真件：共享判据 */
function shared(file) {
  return loadUts(file || SHARED, {});
}

/** 真件：填写页编排（依赖全部来自共享判据真件） */
function flows(file) {
  const s = shared();
  return loadUts(file || FLOWS, {
    fillTargetsOf: s.fillTargetsOf,
  });
}

/** 用户替身（只带判据用到的字段） */
function user(name, avatarUrl, pendingFields) {
  const u = { name: name, avatar_url: avatarUrl };
  if (pendingFields !== undefined) u.pending_fields = pendingFields;
  return u;
}

describe('① 空值判据：占位昵称的全部生成形态', () => {
  const s = () => shared();

  it('后端三种生成形态都算占位（6 位 / _prefix4 / _i）', () => {
    const m = s();
    expect(m.isPlaceholderNickname('微信学员abc123')).toBe(true);
    expect(m.isPlaceholderNickname('微信学员abc123_ab12')).toBe(true);
    expect(m.isPlaceholderNickname('微信学员abc123_1')).toBe(true);
    expect(m.isPlaceholderNickname('微信学员abc123_3')).toBe(true);
  });

  it('真叫「微信学员…」的用户不算占位（前缀判据会误伤，故用完整模式）', () => {
    const m = s();
    expect(m.isPlaceholderNickname('微信学员张三')).toBe(false);
    expect(m.isPlaceholderNickname('微信学员')).toBe(false);
    expect(m.isPlaceholderNickname('微信学员abc12')).toBe(false); // 5 位
    expect(m.isPlaceholderNickname('微信学员abc1234')).toBe(false); // 7 位
    expect(m.isPlaceholderNickname('微信学员abc123_12')).toBe(false); // 两位数字后缀
    expect(m.isPlaceholderNickname('张三')).toBe(false);
  });

  it('昵称为空 = 空串 / 纯空白 / 占位形态', () => {
    const m = s();
    expect(m.isNicknameEmpty('')).toBe(true);
    expect(m.isNicknameEmpty('   ')).toBe(true);
    expect(m.isNicknameEmpty(' 微信学员abc123 ')).toBe(true);
    expect(m.isNicknameEmpty('张三')).toBe(false);
  });

  it('头像为空 = 空串 / 纯空白（后端占位头像是空串，不是默认图 URL）', () => {
    const m = s();
    expect(m.isAvatarEmpty('')).toBe(true);
    expect(m.isAvatarEmpty('  ')).toBe(true);
    expect(m.isAvatarEmpty('https://cdn.example.com/a.webp')).toBe(false);
  });
});

describe('② 待填集合：只补空值，待审字段不可填', () => {
  const f = () => shared();

  it('新用户两半都空 ⇒ 两半都可填', () => {
    const t = f().fillTargetsOf(user('微信学员abc123', ''));
    expect(t).toEqual({ nickname: true, avatar: true });
  });

  it('有值的一半不可编辑、不提交', () => {
    const t = f().fillTargetsOf(user('张三', ''));
    expect(t.nickname).toBe(false);
    expect(t.avatar).toBe(true);
  });

  it('待审字段视为不可填（后端对同字段已有待审会硬拒）', () => {
    const t = f().fillTargetsOf(user('微信学员abc123', '', ['nickname']));
    expect(t.nickname).toBe(false);
    expect(t.avatar).toBe(true);
    const t2 = f().fillTargetsOf(user('微信学员abc123', '', ['nickname', 'avatar']));
    expect(t2).toEqual({ nickname: false, avatar: false });
  });

  it('两半都满 ⇒ 无可填（入口隐藏）', () => {
    const m = f();
    expect(m.hasFillTargetOf(user('张三', 'https://cdn.example.com/a.webp'))).toBe(false);
    expect(m.hasFillTargetOf(user('微信学员abc123', ''))).toBe(true);
  });

  it('入口可见性：仅微信登录 + 仍有可填的一半', () => {
    const m = f();
    expect(m.wechatFillEntryVisibleOf(user('微信学员abc123', ''), 'wechat')).toBe(true);
    expect(m.wechatFillEntryVisibleOf(user('微信学员abc123', ''), 'app')).toBe(false);
    expect(m.wechatFillEntryVisibleOf(user('张三', 'https://cdn.example.com/a.webp'), 'wechat')).toBe(false);
    expect(m.wechatFillEntryVisibleOf(null, 'wechat')).toBe(true);
  });
});

describe('③ 重试记忆：已成功的字段不再重做', () => {
  const f = () => flows();

  it('首次提交：两半都跑', () => {
    const steps = f().submitStepsOf(user('微信学员abc123', ''), {
      avatarPicked: '/tmp/a.jpg', nicknameInput: '张三', avatarDone: false, nicknameDone: false,
    });
    expect(steps).toEqual({ uploadAvatar: true, submitNickname: true });
  });

  it('头像已上传成功（临时路径已清）⇒ 重试只重做昵称', () => {
    const steps = f().submitStepsOf(user('微信学员abc123', ''), {
      avatarPicked: '', nicknameInput: '张三', avatarDone: true, nicknameDone: false,
    });
    expect(steps).toEqual({ uploadAvatar: false, submitNickname: true });
  });

  it('昵称已提交成功 ⇒ 重试只重做头像', () => {
    const steps = f().submitStepsOf(user('微信学员abc123', ''), {
      avatarPicked: '/tmp/a.jpg', nicknameInput: '张三', avatarDone: false, nicknameDone: true,
    });
    expect(steps).toEqual({ uploadAvatar: true, submitNickname: false });
  });

  it('两半都成功 ⇒ 无步骤（不重复提交）', () => {
    const steps = f().submitStepsOf(user('微信学员abc123', ''), {
      avatarPicked: '', nicknameInput: '张三', avatarDone: true, nicknameDone: true,
    });
    expect(steps).toEqual({ uploadAvatar: false, submitNickname: false });
  });

  it('待审字段即便有输入也不提交（后端会硬拒）', () => {
    const steps = f().submitStepsOf(user('微信学员abc123', '', ['nickname']), {
      avatarPicked: '/tmp/a.jpg', nicknameInput: '张三', avatarDone: false, nicknameDone: false,
    });
    expect(steps).toEqual({ uploadAvatar: true, submitNickname: false });
  });
});

describe('④ 返回栈：from 分叉', () => {
  const f = () => flows();

  it('from=login ⇒ 回 choose-cert（新用户仍走认证引导）', () => {
    expect(f().exitTargetOf('login')).toBe('/pages/guide/choose-cert');
  });

  it('其余来源 ⇒ 空串，交页面按栈回退', () => {
    expect(f().exitTargetOf('')).toBe('');
    expect(f().exitTargetOf('profile')).toBe('');
  });

  it('回不去时兜底首页', () => {
    expect(f().homeFallbackUrl()).toBe('/pages/dashboard/dashboard');
  });
});

describe('⑤ 昵称截断：30 字上限', () => {
  const f = () => flows();

  it('未超上限原样返回（trim 后）', () => {
    const m = f();
    expect(m.isNicknameTooLong('  张三  ')).toBe(false);
    expect(m.truncateNickname('  张三  ')).toBe('张三');
  });

  it('超上限截断到 30 字并判超', () => {
    const m = f();
    const long = '一'.repeat(31);
    expect(m.isNicknameTooLong(long)).toBe(true);
    expect(m.truncateNickname(long).length).toBe(30);
    expect(m.isNicknameTooLong('一'.repeat(30))).toBe(false);
  });
});

describe('⑥ pending_fields 解析：缺省即空数组', () => {
  const s = () => shared();

  it('老后端不返回 / 显式 null ⇒ 空数组', () => {
    const m = s();
    expect(m.pendingFieldsOf(null)).toEqual([]);
    expect(m.pendingFieldsOf(user('张三', ''))).toEqual([]);
    expect(m.pendingFieldsOf(user('张三', '', null))).toEqual([]);
  });

  it('逐字段判定', () => {
    const m = s();
    const u = user('张三', '', ['nickname', 'avatar']);
    expect(m.pendingFieldsOf(u)).toEqual(['nickname', 'avatar']);
    expect(m.hasPendingFieldOf(u, 'nickname')).toBe(true);
    expect(m.hasPendingFieldOf(u, 'avatar')).toBe(true);
    expect(m.hasPendingFieldOf(user('张三', '', ['avatar']), 'nickname')).toBe(false);
  });
});

describe('成对取证：占位判据退回前缀形态必须被同一条判据抓到', () => {
  it('startsWith 形态会误伤真叫「微信学员…」的用户', () => {
    const src = readText(SHARED);
    const mutated = src.replace(
      'return PLACEHOLDER_NICKNAME.test(name.trim())',
      "return name.trim().indexOf('微信学员') === 0",
    );
    expect(mutated).not.toBe(src);
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wechat-fill-'));
    const file = path.join(dir, 'wechatProfileFill.uts');
    fs.writeFileSync(file, mutated, 'utf8');
    const m = loadUts(file, {});
    // 真件判 false 的，变异件判 true ⇒ ① 的「真叫微信学员…的用户不算占位」当场翻红
    expect(m.isPlaceholderNickname('微信学员张三')).toBe(true);
    expect(shared().isPlaceholderNickname('微信学员张三')).toBe(false);
  });
});
