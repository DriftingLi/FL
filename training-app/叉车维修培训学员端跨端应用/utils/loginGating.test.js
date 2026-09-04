/**
 * 登录页门控回填流程契约测试（Issue #545）
 *
 * login.uvue 无法在 jest 中渲染，以源码文本断言门控顺序（外部行为契约）：
 * 1) 入口显隐：仅在"存在已保存凭据 && 设备支持生物识别"时显示脱敏入口
 * 2) 门控顺序：authenticate 通过才 loadSecureCredentials 回填；失败不回填不清除
 * 3) 取消勾选：立即 clearSecureCredentials 并收起入口
 * 4) 保存时机：仅账号密码登录成功后保存；微信一键登录不触及凭据
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
    const saveIdx = body.indexOf('saveSecureCredentials(uname, password.value)');
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
