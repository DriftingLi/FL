/**
 * 微信资料判据 · 行为级测试（#2712 引入，#2757 收窄）
 *
 * 真执行纯逻辑件（去类型后当 JS 跑），钉「只有跑起来才看得见」的判据：
 *   ① 空值判据：占位昵称的**全部生成形态**都算空，真叫「微信学员…」的用户不算空；
 *   ② pending_fields 解析：缺省即空数组，逐字段判定。
 *
 * #2757 把「用微信资料完善」入口与独立完善资料页一起下线，待填集合 / 入口可见性 /
 * 重试记忆 / 返回栈 / 昵称截断那几组判据随实现一起退役，本文件同步收窄。
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

/** 真件：共享判据 */
function shared(file) {
  return loadUts(file || SHARED, {});
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
});

describe('② pending_fields 解析：缺省即空数组', () => {
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
