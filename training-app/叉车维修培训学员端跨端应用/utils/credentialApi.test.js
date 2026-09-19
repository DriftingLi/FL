/**
 * 证件 API 构造契约测试
 *
 * api/credential.uts 无法在 jest 中执行，沿用源码契约缝。
 * 钉住：buildCredentialItem 对可空字段 level 必须 null 容忍——
 * 特种作业证件后端返回 level: null，`as number` 强转在 Kotlin 下即
 * NullPointerException（真机日志 2026-09-06 02:00:04 事故）。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const src = readText(path.join(__dirname, '..', 'api', 'credential.uts'));

function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('buildCredentialItem 可空字段契约', () => {
  const body = fnBody('buildCredentialItem');

  it('level 走 null 容忍分支，禁止 as number 强转', () => {
    // 合并 master 后统一走 helpers 的 toNumberOrNull（等价实现，house style）；
    // 契约意图不变：可空字段必须容忍 null，且不得用 as number 强转。
    expect(body).toContain("level: toNumberOrNull(obj['level'])");
    expect(body).not.toContain("obj['level'] as number");
  });

  it('非可空数值字段仍走 toNumber 兜底', () => {
    expect(body).toContain('id: toNumber(obj[\'id\'])');
    expect(body).toContain('status: toNumber(obj[\'status\'])');
  });
});
