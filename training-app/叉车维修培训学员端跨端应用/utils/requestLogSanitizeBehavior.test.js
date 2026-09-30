/**
 * #1398 请求体日志脱敏 —— **行为级**守护（真跑 `api/request.uts` 的 `sanitizeForLog`）
 *
 * 为什么不是源码契约锁：打码是安全承重逻辑，「改坏它会不会红」才是 ③ 门证据（见 docs/agents/guards.md）。
 * utsHarness 把 request.uts 当 JS 跑 ⇒ 这里调的是**真的** sanitizeForLog，不是手抄镜像。
 *
 * 钉住三件事：
 *  1. 敏感键（password / refresh_token / token / code / api_key / captcha_value）的值替换为掩码，非敏感键逐字保留；
 *  2. **返回副本、绝不改动入参**（入参 `options.data` 还要原样发出去）；
 *  3. null 原样返回 null，不炸。
 *
 * ⚠️ 边界：JS 的 `x as UTSJSONObject` 是类型擦除的 no-op（不抛），而 Kotlin 真机对非对象会抛
 *   ClassCastException 走 catch 分支。故本缝只覆盖**对象**与 **null** 两支 —— 标量 / 数组请求体
 *   本仓不存在（auth 请求体都是扁平 UTSJSONObject），不在射程内。
 */
const path = require('path');

/** 把 request.uts 当 JS 跑起来，取它导出的 sanitizeForLog */
const { loadUts } = require('./utsHarness');
const REQUEST_UTS = path.join(__dirname, '..', 'api', 'request.uts');

/**
 * request.uts 的 import 依赖（loadUts 按 import 清单 fail-closed 校验齐全）。
 * sanitizeForLog 只用模块内量（SENSITIVE_LOG_KEYS / LOG_SECRET_MASK / isSensitiveLogKey），
 * 其余给占位即可 —— 我们不调 request()，那些依赖不会被触发。
 */
function loadSanitizer() {
  const mod = loadUts(REQUEST_UTS, {
    gateRefresh: () => Promise.resolve(true),
    API_BASE_URL: 'http://127.0.0.1:8080',
    REQUEST_TIMEOUT: 15000,
    ENABLE_DEBUG_LOG: false,
    STORAGE_KEY_TOKEN: 'auth_token',
    STORAGE_KEY_USER: 'auth_user',
    getStorage: () => '',
    removeStorage: () => {},
    isContentUri: () => false,
    buildTempFilePath: () => '',
    UPLOAD_TMP_DIR: 'upload-tmp',
    isRecruiterActive: () => false,
  });
  return mod.sanitizeForLog;
}

describe('sanitizeForLog 行为（#1398：敏感值不进 logcat 由代码保证）', () => {
  test('敏感键全部打码，非敏感键逐字保留', () => {
    const sanitize = loadSanitizer();
    const out = sanitize({
      username: 'alice',
      password: 'P@ssw0rd-secret',
      refresh_token: 'eyJhbGciOiJSUzI1NiIs.long.jwt',
      token: 'abc',
      code: '123456',
      api_key: 'sk-proj-abc123',
      captcha_value: 'ab3d',
      remember: true,
    });
    // 非敏感：照常可见，保住日志的调试价值
    expect(out.username).toBe('alice');
    expect(out.remember).toBe(true);
    // 敏感：原值一个字都不留
    expect(out.password).not.toBe('P@ssw0rd-secret');
    expect(out.password).toBe('***');
    expect(out.refresh_token).toBe('***');
    expect(out.token).toBe('***');
    expect(out.code).toBe('***');
    // 同走本打印口的同类凭证（#1398 review 补）：第三方模型密钥 / 图形验证码
    expect(out.api_key).toBe('***');
    expect(out.captcha_value).toBe('***');
  });

  test('返回副本、绝不改动入参（入参还要原样发出去）', () => {
    const sanitize = loadSanitizer();
    const body = { username: 'bob', password: 'topsecret' };
    const out = sanitize(body);
    expect(out).not.toBe(body); // 不是同一对象
    expect(body.password).toBe('topsecret'); // 入参未被毁，请求体照常能发
    expect(out.password).toBe('***');
  });

  test('null（GET / DELETE 无请求体）原样返回 null，不炸', () => {
    const sanitize = loadSanitizer();
    expect(sanitize(null)).toBe(null);
  });

  test('判别力：非敏感键不被误伤（打码是按键、不是全量抹掉）', () => {
    const sanitize = loadSanitizer();
    const out = sanitize({ phone: '13800000000', code: '0000' });
    expect(out.phone).toBe('13800000000'); // phone 不在敏感清单
    expect(out.code).toBe('***'); // code 在
  });
});
