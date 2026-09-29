/**
 * 快捷登录契约测试（ADR-0004 增补 · #1391 换机制）
 *
 * stores/auth.uts 无法在 jest 中执行，沿用源码契约缝。
 *
 * 钉住的两件事：
 *  ① **快捷登录 = 凭据登录**：quickLogin 的体内走 `loginApi(`（POST /auth/login），
 *     不得再出现 `refreshTokenApi`（登出吊销后那条路永远 401，#1387）或 `getUserInfoApi`
 *     （loginApi 一次返回 token + user + refresh_token，无需回拉 /auth/me）。
 *  ② 令牌的**单一同步点**：任何拿到新 rt 的路径（tryRefreshToken / quickLogin）都必须
 *     在 setAuthData 之后经 updateSecureToken 回写加密凭据包络。
 *     #1391 之后包络里的 rt 只写不读（vestigial，登出即吊销），这条锁的是**落盘一致性**，
 *     不是「跨登出续命」——后者已随 #1387 的吊销口径作废。
 *  ③ #1398：quickLogin 的**成功信号 = 会话建立**（access token 落地），与 rt 是否下发**解绑**；
 *     rt 只作回写值。旧实现拿 `result.refresh_token` 当成功信号，后端不下发 rt 时会把
 *     「已登录」误报成失败（`loginApi` 的 `toStr(data['refresh_token'], '')` 显式容忍 rt 为空）。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const src = readText(path.join(__dirname, '..', 'stores', 'auth.uts'));

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

  it('快捷登录路径拿到新令牌后回写包络（单一同步点，与 tryRefreshToken 同款次序）', () => {
    const body = fnBody('quickLogin');
    const setIdx = body.indexOf('setAuthData(result.token');
    const syncIdx = body.indexOf('updateSecureToken(result.refresh_token)');
    expect(setIdx).toBeGreaterThan(-1);
    expect(syncIdx).toBeGreaterThan(setIdx);
  });
});

describe('快捷登录=凭据登录契约（#1391 换机制，钉住新机制而非旧机制的墓志铭）', () => {
  const body = fnBody('quickLogin');

  it('quickLogin 走 loginApi（POST /auth/login），不再用 rt 调 /auth/refresh', () => {
    expect(body).toContain('loginApi(');
    expect(body).not.toContain('refreshTokenApi');
  });

  it('不再需要 /auth/me 回拉，也不再有「拉 user 失败回滚 token」那一段', () => {
    expect(body).not.toContain('getUserInfoApi');
    expect(body).not.toContain('removeStorage(STORAGE_KEY_TOKEN)');
  });

  it('落盘走密码登录口径（provider=password）；成功信号与会话建立绑定、与 rt 解绑（#1398）', () => {
    expect(body).toContain("setAuthData(result.token, result.user, result.refresh_token, 'password')");
    // 成功信号 = token 落地；缺 rt 不影响「已登录」的事实
    expect(body).toContain('if (result == null || result.token.length == 0) return false');
    expect(body).toContain('return true');
    // 旧形状（把轮换出的 rt 当成功信号）不得回来 —— 后端不下发 rt 时它是假失败
    expect(body).not.toMatch(/return\s+result\.refresh_token/);
  });

  it('暴露面签名 = 凭据（username, password）返回成功与否（Promise<boolean>），不再是令牌串', () => {
    expect(src).toContain('quickLogin: (username : string, password : string) : Promise<boolean> => quickLogin(username, password)');
    expect(src).not.toContain('quickLogin(rt)');
  });
});
