/**
 * 生物认证 composable 平台契约测试（Issue #545）
 *
 * useBiometric.uts 含条件编译（#ifdef），jest 环境无法执行，沿用源码契约缝：
 * 断言各平台分支的外部行为语义与门控顺序。
 *
 * 契约（对应规格 #542）：
 * - H5：showModal 模拟，确定=成功、取消/失败=不通过
 * - APP / MP-WEIXIN：真实 SOTER；认证模式必须是 fingerPrint 或 facial 才算成功
 * - 其余平台（MP-ALIPAY 等）：恒不支持（凭据降级的判定源头）
 */
const fs = require('fs');
const path = require('path');

const src = fs.readFileSync(path.join(__dirname, '..', 'composables', 'useBiometric.uts'), 'utf8');

function between(startMark, endMark) {
  const s = src.indexOf(startMark);
  if (s === -1) throw new Error(`未找到 ${startMark}`);
  return src.slice(s, src.indexOf(endMark, s));
}

describe('useBiometric 平台分支契约', () => {
  it('对外能力面固定：isSupported + checkSupport + authenticate', () => {
    expect(src).toContain('checkSupport');
    expect(src).toContain('authenticate');
    expect(src).toContain('isSupported');
  });

  /** useBiometric.uts 中 `// #ifdef H5` 出现两处（checkSupport 与 authenticate），取 authenticate 内的分段 */
  function branch(marker) {
    const fnStart = src.indexOf('async function authenticate');
    if (fnStart === -1) throw new Error('未找到 authenticate');
    const s = src.indexOf(marker, fnStart);
    if (s === -1) throw new Error(`authenticate 内未找到 ${marker}`);
    return src.slice(s, src.indexOf('// #endif', s));
  }

  describe('H5 模拟分支（authenticate 内）', () => {
    const h5 = () => branch('// #ifdef H5');

    it('showModal 模拟且确认=通过、取消=不通过', () => {
      expect(h5()).toContain('uni.showModal');
      expect(h5()).toContain('resolve(true)');
      expect(h5()).toContain('resolve(false)');
      expect(h5()).toContain('res.confirm');
    });

    it('弹窗失败（fail 回调）也不通过', () => {
      expect(h5()).toContain('fail:');
    });
  });

  describe('APP / 微信真实 SOTER 分支（authenticate 内）', () => {
    const app = () => branch('// #ifdef APP || MP-WEIXIN');

    it('认证走 startSoterAuthentication，请求指纹+面容双模式', () => {
      expect(app()).toContain('uni.startSoterAuthentication');
      expect(app()).toContain("'fingerPrint'");
      expect(app()).toContain("'facial'");
    });

    it('认证模式白名单校验：仅 fingerPrint/facial 算成功（防 authMode 透传）', () => {
      expect(app()).toContain("authMode == 'fingerPrint' || authMode == 'facial'");
    });

    it('SOTER 调用失败 resolve(false) 而非 reject（调用方可静默重试）', () => {
      expect(app()).toContain('fail: (err)');
      expect(app()).toContain('resolve(false)');
    });
  });

  describe('checkSupport 分支契约', () => {
    const check = between('async function checkSupport', 'async function authenticate');

    it('APP/微信检测支持性走 checkIsSupportSoterAuthentication，异常置为不支持', () => {
      expect(check).toContain('uni.checkIsSupportSoterAuthentication');
      expect(check).toContain('isSupported.value = false');
    });

    it('原生端必须用 success/fail 回调取结果，禁止 await 返回值强转（kotlin.Unit 坑）', () => {
      expect(check).toContain('success: (res) =>');
      expect(check).toContain('fail: (err) =>');
      // 回归防护：await 该 API 会得到 kotlin.Unit，as UTSJSONObject 运行时 ClassCastException
      expect(check).not.toContain('await uni.checkIsSupportSoterAuthentication');
      expect(check).not.toContain('as UTSJSONObject');
    });

    it('返回 SupportCheckResult 三字段（supported/definitive/modes）', () => {
      expect(src).toContain('export type SupportCheckResult');
      expect(src).toContain('supported : boolean');
      expect(src).toContain('definitive : boolean');
      expect(src).toContain('modes : string[]');
    });

    it('success 路径 supported 取决于 supportMode 是否非空（调用成功≠设备支持）', () => {
      expect(check).toContain('supported: modes.length > 0');
      expect(check).toContain('definitive: true');
    });

    it('fail 路径 definitive=false：支持性未知，调用方不得清理本地数据（孤儿凭据自愈前提）', () => {
      expect(check).toContain('definitive: false');
    });

    it('对外能力面新增 supportModes（入口动态文案数据源）', () => {
      expect(src).toContain('supportModes');
    });
  });

  describe('兜底分支（MP-ALIPAY 等未覆盖平台）', () => {
    it('不支持平台恒定不通过——凭据降级的判定源头', () => {
      const fallback = branch('// #ifndef H5 || APP || MP-WEIXIN');
      expect(fallback).toContain('return false');
    });

    it('checkSupport 兜底为确定性不支持（definitive=true，可触发孤儿凭据自愈）', () => {
      // checkSupport 在 authenticate 之前，首个 #ifndef 属于 checkSupport
      const fallback = between('// #ifndef H5 || APP || MP-WEIXIN', 'async function authenticate');
      expect(fallback).toContain('supported: false');
      expect(fallback).toContain('definitive: true');
    });
  });
});
