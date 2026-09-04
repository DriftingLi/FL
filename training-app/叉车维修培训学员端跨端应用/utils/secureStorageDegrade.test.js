/**
 * 凭据降级矩阵契约测试（Issue #548 / TDD RED 先行）
 *
 * 降级语义（ADR-0004 决策 A）：生物识别不可用的平台只记住账号，密码不落盘。
 * 本文件先写期望契约（RED），再在 secureStorage / login 中实现（GREEN）。
 */
const fs = require('fs');
const path = require('path');

const SEC_PATH = path.join(__dirname, 'secureStorage.uts');
const src = fs.readFileSync(SEC_PATH, 'utf8');

describe('凭据降级契约（仅记住账号）', () => {
  it('存在账号降级保存函数 saveAccountOnly（新增 API，密码不入库）', () => {
    expect(src).toContain('function saveAccountOnly');
  });

  it('账号降级保存后 has 标志为 false（不构成完整凭据包络）', () => {
    const start = src.indexOf('function saveAccountOnly');
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('has: false');
    expect(body).toContain('u: username');
  });

  it('账号降级写入密码字段为空串（不落盘任何密码内容）', () => {
    const start = src.indexOf('function saveAccountOnly');
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toMatch(/p:\s*''/);
  });

  it('包络判定不受降级数据干扰：has=false 时 hasStoredCredentials 恒 false', () => {
    const start = src.indexOf('function hasStoredCredentials');
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('has !== true');
  });
});

describe('降级回填契约（loadSavedAccount）', () => {
  it('存在降级账号读取函数 loadSavedAccount', () => {
    expect(src).toContain('function loadSavedAccount');
  });

  it('完整凭据模式（has=true）不触发降级回填（返回空串）', () => {
    const start = src.indexOf('function loadSavedAccount');
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('env.has === true');
  });

  it('降级模式（has=false）且账号非空时返回账号', () => {
    const start = src.indexOf('function loadSavedAccount');
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('env.u.length > 0 ? env.u');
  });

  it('降级回填不返回密码字段（只返回账号字符串）', () => {
    const start = src.indexOf('function loadSavedAccount');
    const body = src.slice(start, src.indexOf('\n}', start));
    // loadSavedAccount 返回类型是 string，不是 StoredCredentials
    expect(body).not.toContain('env.p');
  });
});
