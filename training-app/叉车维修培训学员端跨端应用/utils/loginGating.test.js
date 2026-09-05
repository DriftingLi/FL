/**
 * 登录页门控回填流程契约测试（Issue #545 + ADR-0004 快捷登录增补）
 *
 * login.uvue 无法在 jest 中渲染，以源码文本断言门控顺序（外部行为契约）：
 * 1) 入口显隐：仅在"存在已保存凭据 && 设备支持生物识别"时显示快捷登录入口
 * 2) 门控顺序：authenticate 通过才 loadSecureCredentials 回填；失败不回填不清除
 * 3) 取消勾选：立即 clearSecureCredentials 并收起入口
 * 4) 保存时机：仅账号密码登录成功后保存（含 refresh_token）；微信一键登录不触及凭据
 * 5) 快捷登录：authenticate → loadSecureToken → auth.quickLogin → reLaunch；失败降级回填
 * 6) 孤儿凭据自愈：确定性不支持 + 完整凭据 → 降级仅账号；API 失败（definitive=false）不清理
 */
const fs = require('fs');
const path = require('path');

const src = fs.readFileSync(path.join(__dirname, '..', 'pages', 'login', 'login.uvue'), 'utf8');

const script = src.slice(src.indexOf('<script'), src.lastIndexOf('</script>'));

function fnBody(name) {
  const start = script.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return script.slice(start, script.indexOf('\n    }', start));
}

describe('登录页门控顺序契约（onBiometricUnlock）', () => {
  const body = fnBody('onBiometricUnlock');

  it('先认证后回填：authenticate 结果为 true 才 loadSecureCredentials', () => {
    const authIdx = body.indexOf('biometric.authenticate');
    const loadIdx = body.indexOf('loadSecureCredentials()');
    expect(authIdx).toBeGreaterThan(-1);
    expect(loadIdx).toBeGreaterThan(authIdx);
  });

  it('认证通过才回填账号与密码两个字段', () => {
    expect(body).toContain('username.value = cred.u');
    expect(body).toContain('password.value = cred.p');
  });

  it('认证通过后回填视为重新勾选记住密码（保持生命周期语义）', () => {
    expect(body).toContain('rememberMe.value = true');
  });

  it('回填成功收起脱敏入口', () => {
    expect(body).toContain('showedStored.value = false');
  });

  it('凭据缺失时给出提示而非静默失败', () => {
    expect(body).toContain('未找到保存的凭据');
  });
});

describe('取消勾选清除契约（toggleRemember）', () => {
  const body = fnBody('toggleRemember');

  it('取消勾选立即清除凭据', () => {
    const checkIdx = body.indexOf('if (!rememberMe.value)');
    const clearIdx = body.indexOf('clearSecureCredentials()');
    expect(checkIdx).toBeGreaterThan(-1);
    expect(clearIdx).toBeGreaterThan(checkIdx);
  });

  it('取消勾选同时收起脱敏入口', () => {
    expect(body).toContain('showedStored.value = false');
  });
});

describe('保存时机契约（登录成功路径）', () => {
  it('仅勾选记住密码且账号非空才保存（密码在登录成功后落盘）', () => {
    const body = fnBody('onSubmit');
    expect(body).toContain('rememberMe.value && uname.length > 0');
    const saveIdx = body.indexOf('saveSecureCredentials(uname, password.value, auth.getRefreshToken())');
    const clearIdx = body.indexOf('clearSecureCredentials()');
    expect(saveIdx).toBeGreaterThan(-1);
    expect(clearIdx).toBeGreaterThan(saveIdx);
  });

  it('页面进入时按"存在凭据"决定脱敏入口显隐（onLoad/onShow 生命周期）', () => {
    expect(script).toContain('hasStoredCredentials()');
    expect(script).toMatch(/hasStoredCredentials\(\)[\s\S]{0,80}showedStored\.value = true/);
  });

  it('脱敏入口同时受设备生物识别能力门控', () => {
    expect(src).toContain('showedStored && biometric.isSupported.value');
  });

  it('凭据存储只经 secureStorage 抽象层（无散落的 storage 直调）', () => {
    expect(script).toContain("from '../../utils/secureStorage'");
    expect(script).not.toContain('uni.setStorageSync');
    expect(script).not.toContain('uni.getStorageSync');
  });
});

describe('快捷登录契约（onQuickLogin，ADR-0004 增补）', () => {
  const body = fnBody('onQuickLogin');

  it('门控顺序：先 authenticate，通过后才 loadSecureToken', () => {
    const authIdx = body.indexOf('biometric.authenticate');
    const tokenIdx = body.indexOf('loadSecureToken()');
    expect(authIdx).toBeGreaterThan(-1);
    expect(tokenIdx).toBeGreaterThan(authIdx);
  });

  it('静默续登后先回写轮换令牌再进 dashboard（旧 rt 已失效，包络是唯一跨登出副本）', () => {
    const quickIdx = body.indexOf('auth.quickLogin(rt)');
    const updateIdx = body.indexOf('updateSecureToken(newRt)');
    const reIdx = body.indexOf("uni.reLaunch({ url: '/pages/dashboard/dashboard' })");
    expect(quickIdx).toBeGreaterThan(-1);
    expect(updateIdx).toBeGreaterThan(quickIdx);
    expect(reIdx).toBeGreaterThan(updateIdx);
  });

  it('续登失败提示过期并降级回填（不静默失败）', () => {
    expect(body).toContain('快捷登录已过期，请验证后登录');
    expect(body).toContain('onBiometricUnlock()');
  });

  it('无令牌时走回填兜底（再次验证身份后回填表单）', () => {
    const tokenIdx = body.indexOf('loadSecureToken()');
    const fallbackIdx = body.indexOf('onBiometricUnlock()');
    expect(tokenIdx).toBeGreaterThan(-1);
    expect(fallbackIdx).toBeGreaterThan(tokenIdx);
  });

  it('防重入：quickLogging 进行中直接返回', () => {
    expect(body).toContain('if (quickLogging.value)');
  });

  it('入口绑定切换为 onQuickLogin，文案走动态标签', () => {
    expect(src).toContain('@click="onQuickLogin"');
    expect(src).toContain('{{ quickLoginLabel }}');
    expect(src).not.toContain('解锁填充');
  });
});

describe('孤儿凭据自愈契约（initBiometricGate，ADR-0004）', () => {
  const body = fnBody('initBiometricGate');

  it('先 await checkSupport 拿确定性结果，再做入口/回填判定（防 isSupported 初值误判）', () => {
    const supportIdx = body.indexOf('await biometric.checkSupport()');
    const storedIdx = body.indexOf('hasStoredCredentials()');
    expect(supportIdx).toBeGreaterThan(-1);
    expect(storedIdx).toBeGreaterThan(supportIdx);
  });

  it('自愈条件三合：确定性不支持（definitive=true）+ 存在完整凭据才降级', () => {
    const condIdx = body.indexOf('!support.supported && support.definitive && hasStoredCredentials()');
    const degradeIdx = body.indexOf('saveAccountOnly(cred.u)');
    expect(condIdx).toBeGreaterThan(-1);
    expect(degradeIdx).toBeGreaterThan(condIdx);
  });

  it('自愈只清密码密文，保留账号（saveAccountOnly 收敛到与登录时降级同款终态）', () => {
    expect(body).toContain('loadSecureCredentials()');
    expect(body).toContain('saveAccountOnly(cred.u)');
  });

  it('降级后走账号回填分支（hasStoredCredentials 已为 false，loadSavedAccount 生效）', () => {
    const degradeIdx = body.indexOf('saveAccountOnly(cred.u)');
    const refillIdx = body.indexOf('loadSavedAccount()');
    expect(refillIdx).toBeGreaterThan(degradeIdx);
  });
});
