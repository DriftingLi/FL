/**
 * 证件 API 构造契约测试
 *
 * api/credential.uts 无法在 jest 中执行，沿用源码契约缝。
 * 钉住：buildCredentialItem 对可空字段 level 必须 null 容忍——
 * 特种作业证件后端返回 level: null，`as number` 强转在 Kotlin 下即
 * NullPointerException（真机日志 2026-09-06 02:00:04 事故）。
 */
const fs = require('fs');
const path = require('path');

const src = fs.readFileSync(path.join(__dirname, '..', 'api', 'credential.uts'), 'utf8');

function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('buildCredentialItem 可空字段契约', () => {
  const body = fnBody('buildCredentialItem');

  it('level 走 null 容忍分支，禁止 as number 强转', () => {
    expect(body).toContain('let level : number | null = null');
    expect(body).toContain('levelRaw != null');
    expect(body).toContain('level = toNumber(levelRaw)');
    expect(body).not.toContain("obj['level'] as number");
  });

  it('非可空数值字段仍走 toNumber 兜底', () => {
    expect(body).toContain('id: toNumber(obj[\'id\'])');
    expect(body).toContain('status: toNumber(obj[\'status\'])');
  });
});
