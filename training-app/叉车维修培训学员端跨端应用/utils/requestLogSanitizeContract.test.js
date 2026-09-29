/**
 * #1398 请求体日志脱敏 —— **接线**契约（源码文本断言）
 *
 * 分工：`requestLogSanitizeBehavior.test.js` 证明 sanitizeForLog **算得对**（真跑）；
 * 本文件证明**生产打印点真的走它** —— 行为测试自己搭调用，接线被拆（改回原样打印）它照样绿。
 * 两个都要（先例：concurrent401Refresh 的 behavior + contract 成对）。
 *
 * 钉住三件事：
 *  1. debug 打印行的 `data:` 槽包了 `sanitizeForLog(options.data)`，不再原样吐 `options.data`；
 *  2. 敏感清单含 password / refresh_token / token / code，且打印行保留 `[request] >>> <method> <url>`
 *     前缀 —— ①a 真机 logcat 机检靠这一段（`credentialScopeContract` 记过口径）；
 *  3. 掩码替身存在。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const src = readText(path.join(__dirname, '..', 'api', 'request.uts'));

describe('请求体打印接线（#1398）', () => {
  it('debug 行的 data: 槽包了 sanitizeForLog，敏感值不再原样进 logcat', () => {
    expect(src).toContain(
      "console.log('[request] >>>', method, fullURL, 'data:', sanitizeForLog(options.data))"
    );
  });

  it('反向：不得再直接把裸 options.data 打进日志（退回旧形态即判红）', () => {
    expect(src).not.toContain("'data:', options.data)");
  });

  it('保留 [request] >>> <method> <fullURL> 前缀（①a 机检行的锚，不能被顺手改掉）', () => {
    expect(src).toMatch(/console\.log\(\s*'\[request\] >>>',\s*method,\s*fullURL,/);
  });

  it('敏感清单逐字在案：password / refresh_token / token / code / api_key / captcha_value', () => {
    expect(src).toContain("['password', 'refresh_token', 'token', 'code', 'api_key', 'captcha_value']");
  });

  it('掩码替身存在（不是把键删掉，是把值替换 —— 结构仍可读）', () => {
    expect(src).toContain("const LOG_SECRET_MASK = '***'");
  });
});
