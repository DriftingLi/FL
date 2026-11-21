/**
 * 微信资料一键填充 · 接线契约测试（#2712）
 *
 * 与 `utils/wechatProfileFillBehavior.test.js` 成对：那边跑行为（判据对不对），
 * 这边守接线（判据有没有被页面真的接上、有没有第二真源）。
 *
 * 缝 = 源码契约（.uvue 不可 jest import；先例 personalInfoContract / apiBatchBContract）。
 * 钉住：
 *   ① 纯逻辑 composable 零 store / 零 api 依赖（接口调用与页面状态留在壳层）；
 *   ② 页面 import 面与「判据不在页面重写」（页面不出现占位昵称正则、不出现 choose-cert 字面量）；
 *   ③ 入口可见条件与按字段徽标（个人信息页）；
 *   ④ 文案唯一定义点（提交成功提示 / 截断提示只在 composable）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const PAGE = 'pages/profile-setup/profile-setup.uvue';
const FLOWS = 'pages/profile-setup/composables/profile-setup-flows.uts';
const SHARED = 'utils/wechatProfileFill.uts';
const PROFILE_PAGE = 'pages/profile/personal-info.uvue';
const PROFILE_FLOWS = 'pages/profile/composables/personal-info-flows.uts';

describe('拆出物存在契约（模块私有 composables + 跨切面共享判据）', () => {
  it.each([[FLOWS, 'profile-setup-flows.uts'], [SHARED, 'wechatProfileFill.uts']])('%s 存在', (rel) => {
    expect(fs.existsSync(path.join(ROOT, rel))).toBe(true);
  });
});

describe('① 纯逻辑 composable 零 store / 零 api 依赖', () => {
  const flows = () => read(FLOWS);

  it('不 import stores / api（接口调用与页面状态留在壳层）', () => {
    const f = flows();
    expect(f).not.toMatch(/from '\.[^']*stores\//);
    expect(f).not.toMatch(/from '\.[^']*api\//);
    expect(f).not.toContain('useAuthStore');
  });

  it('零模块级可变状态（无 let / 无 ref 实例）', () => {
    const f = flows();
    expect(f).not.toMatch(/^let /m);
    expect(f).not.toMatch(/^export const \w+ = ref\(/m);
  });

  it('判据本体在共享件，composable 只 import 不重写', () => {
    const f = flows();
    expect(f).toContain("from '../../../utils/wechatProfileFill.uts'");
    // 占位昵称正则只允许出现在共享件里（第二真源会让两页判据漂移）
    expect(f).not.toContain('微信学员');
    expect(read(SHARED)).toContain('微信学员');
  });
});

describe('② 页面接线：import 面与「判据不在页面重写」', () => {
  const page = () => read(PAGE);

  it('页面显式 import 纯逻辑 composable（非 easycom）', () => {
    expect(page()).toContain("from './composables/profile-setup-flows.uts'");
  });

  it('页面不重写判据：无占位昵称正则、无 choose-cert 字面量、无 pending_fields 解析', () => {
    const p = page();
    expect(p).not.toContain('微信学员');
    expect(p).not.toContain('/pages/guide/choose-cert');
    expect(p).not.toContain('pending_fields');
  });

  it('页面读 from 参数并交给 composable 的出口判据', () => {
    const p = page();
    expect(p).toMatch(/onLoad\(\(options : UTSJSONObject\)/);
    expect(p).toContain("options['from']");
    expect(p).toContain('exitTargetOf(from.value)');
    expect(p).toContain('homeFallbackUrl()');
  });

  it('页面按页面栈回退（回不去才兜底首页）', () => {
    const p = page();
    expect(p).toContain('getCurrentPages()');
    expect(p).toContain('uni.navigateBack()');
  });

  it('提交后不改本地昵称（等 /auth/me 回填，避免审核前就显示未过审的值）', () => {
    expect(page()).not.toContain('auth.updateUser(');
  });
});

describe('③ 个人信息页：入口可见条件与按字段徽标', () => {
  const page = () => read(PROFILE_PAGE);
  const flows = () => read(PROFILE_FLOWS);

  it('入口 cell 存在且按 showWechatFill 条件渲染', () => {
    const p = page();
    expect(p).toContain('用微信资料完善');
    expect(p).toContain('v-if="showWechatFill"');
    expect(p).toContain("uni.navigateTo({ url: '/pages/profile-setup/profile-setup' })");
  });

  it('入口可见性判据唯一定义在共享件（填写页与个人信息页同源，无第二实现）', () => {
    expect(read(SHARED)).toContain('export function wechatFillEntryVisibleOf(');
    expect(flows()).not.toContain('export function wechatFillEntryVisibleOf(');
    expect(page()).toContain('wechatFillEntryVisibleOf(authStore.user.value, authStore.loginProvider.value)');
  });

  it('徽标按字段挂：头像与昵称各一处，不再共用一个 hasPending', () => {
    const p = page();
    expect(p).toContain('<text v-if="hasPendingNickname" class="cell-badge">审核中</text>');
    expect(p).toContain('<text v-if="hasPendingAvatar" class="cell-badge">审核中</text>');
    expect(p).not.toContain('v-if="hasPending"');
  });
});

describe('④ 文案唯一定义点', () => {
  it('提交成功提示只在 composable（页面不写字面量）', () => {
    expect(read(FLOWS)).toContain("'资料已提交，审核通过后生效'");
    expect(read(PAGE)).not.toContain('资料已提交，审核通过后生效');
  });

  it('截断提示只在 composable（页面调 truncateToastOf）', () => {
    expect(read(FLOWS)).toContain("'昵称最多 30 个字，已截断'");
    expect(read(PAGE)).not.toContain('昵称最多 30 个字');
    expect(read(PAGE)).toContain('truncateToastOf()');
  });
});
