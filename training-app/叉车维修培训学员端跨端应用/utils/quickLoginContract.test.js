/**
 * 快捷登录令牌同步契约测试（ADR-0004 增补）
 *
 * stores/auth.uts 无法在 jest 中执行，沿用源码契约缝。
 * 钉住的契约：refresh_token 每次轮换都必须同步回写加密凭据包络（updateSecureToken），
 * 否则包内 rt 立即过期，快捷登录在会话期内任意一次 401 自动刷新后即永久失效。
 */
const fs = require('fs');
const path = require('path');

const src = fs.readFileSync(path.join(__dirname, '..', 'stores', 'auth.uts'), 'utf8');

function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('轮换令牌同步契约（tryRefreshToken / quickLogin → updateSecureToken）', () => {
  it('store 引入 updateSecureToken（包络回写的唯一入口）', () => {
    expect(src).toContain("import { updateSecureToken } from '../utils/secureStorage'");
  });

  it('401 自动刷新路径轮换后回写包络', () => {
    const body = fnBody('tryRefreshToken');
    const setIdx = body.indexOf('setAuthData(result.token');
    const syncIdx = body.indexOf('updateSecureToken(result.refresh_token)');
    expect(setIdx).toBeGreaterThan(-1);
    expect(syncIdx).toBeGreaterThan(setIdx);
  });

  it('快捷登录路径轮换后回写包络（跨登出滑动续期）', () => {
    const body = fnBody('quickLogin');
    const setIdx = body.indexOf('setAuthData(result.token');
    const syncIdx = body.indexOf('updateSecureToken(result.refresh_token)');
    expect(setIdx).toBeGreaterThan(-1);
    expect(syncIdx).toBeGreaterThan(setIdx);
  });

  it('quickLogin 不依赖本地已有 user：refresh 成功后经 /auth/me 补齐', () => {
    const body = fnBody('quickLogin');
    expect(body).toContain('getUserInfoApi()');
    expect(body).toContain("removeStorage(STORAGE_KEY_TOKEN)");
  });

  it('quickLogin 返回轮换出的新令牌（调用方据此判定成功）', () => {
    const body = fnBody('quickLogin');
    expect(body).toContain('return result.refresh_token');
    expect(src).toContain('quickLogin: (rt : string) : Promise<string> => quickLogin(rt)');
  });
});
