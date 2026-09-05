/**
 * 安全凭据存储契约测试（TDD: RED → GREEN 钉住行为契约）
 *
 * secureStorage.uts 无法在 jest 中 import（.uts 非 JS 模块），故沿用项目
 * 既有的"源码契约测试"缝：读取源文本，断言函数体的外部行为语义。
 * 先例：utils/pointsBalance.test.js、utils/dashboardPage.test.js。
 *
 * 钉住的行为契约（对应 Issue #545 / 规格 #542）：
 * 1) 凭据包络：has=true 且账号、密码均非空才算"存在已保存凭据"
 * 2) 保存/读取：结构完整（三字段齐全），clear 后 has 恒 false（幂等）
 * 3) 存储 key 唯一性：凭据写入只经 STORAGE_KEY_CREDENTIALS 常量
 */
const fs = require('fs');
const path = require('path');

const SEC_PATH = path.join(__dirname, 'secureStorage.uts');
const APP_CONST_PATH = path.join(__dirname, '..', 'constants', 'app.uts');
const STORAGE_PATH = path.join(__dirname, 'storage.uts');

const src = fs.readFileSync(SEC_PATH, 'utf8');
const appConst = fs.readFileSync(APP_CONST_PATH, 'utf8');

/** 提取函数体：从函数声明到顶层级 "\n}" */
function fnBody(name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('凭据包络判定契约（hasStoredCredentials）', () => {
  const body = fnBody('hasStoredCredentials');

  it('has 标志不为 true 时判定不存在', () => {
    expect(body).toContain('env.has !== true');
  });

  it('账号或密码任一为空串判定不存在（length > 0 检查）', () => {
    expect(body).toContain('env.u.length > 0');
    expect(body).toContain('env.p.length > 0');
  });

  it('载体缺失时短路返回不存在', () => {
    expect(body).toContain('if (env == null)');
  });
});

describe('凭据读写与清除契约', () => {
  it('saveSecureCredentials 落盘结构四字段齐全（u/p/has=true/rt）', () => {
    const body = fnBody('saveSecureCredentials');
    expect(body).toContain('u: username');
    expect(body).toContain('p: password');
    expect(body).toContain('has: true');
    expect(body).toContain('rt: rt');
  });

  it('loadSecureToken 仅完整凭据包络且 rt 非空时返回令牌（快捷登录数据源，与 loadSecureCredentials 同口径）', () => {
    const body = fnBody('loadSecureToken');
    expect(body).toContain('env.has !== true');
    expect(body).toContain('env.u.length == 0 || env.p.length == 0');
    expect(body).toContain('return env.rt');
  });

  it('updateSecureToken 回写轮换令牌：仅完整包络生效，空 rt 与降级包络为 no-op（滑动续期回写口）', () => {
    const body = fnBody('updateSecureToken');
    expect(body).toContain('if (rt.length == 0)');
    expect(body).toContain('env.has !== true');
    expect(body).toContain('env.u.length == 0 || env.p.length == 0');
    expect(body).toContain('env.rt = rt');
    expect(body).toContain('writeEnvelope(env)');
  });

  it('fromJSON 对后补 rt 字段容忍：旧包络缺失 rt 键时按空串处理（向前兼容）', () => {
    const body = fnBody('fromJSON');
    expect(body).toContain("obj['rt']");
    expect(body).toContain("let rt = ''");
  });

  it('saveAccountOnly 降级包络不含 rt（无门控则无快捷登录）', () => {
    const body = fnBody('saveAccountOnly');
    expect(body).toContain('rt: \'\'');
  });

  it('loadSecureCredentials 与包络判定同口径（has!==true 或字段为空即 null）', () => {
    const body = fnBody('loadSecureCredentials');
    expect(body).toContain('env.has !== true');
    expect(body).toContain('env.u.length == 0 || env.p.length == 0');
  });

  it('loadSecureCredentials 返回完整载体（回填依赖 u/p/has 齐全）', () => {
    const body = fnBody('loadSecureCredentials');
    expect(body).toContain('return env');
  });

  it('clearSecureCredentials 双后端清除（加密条目 + 明文残留，幂等）', () => {
    const body = fnBody('clearSecureCredentials');
    expect(body).toContain('        removeSecureItem(STORAGE_KEY_CREDENTIALS)');
    expect(body).toContain('removeStorage(STORAGE_KEY_CREDENTIALS)');
  });
});

describe('存储 key 唯一性契约', () => {
  it('凭据读写清除全部经由 STORAGE_KEY_CREDENTIALS 常量', () => {
    expect(src).toContain('STORAGE_KEY_CREDENTIALS');
    // 明文路径的存储调用全部使用常量而非字面量（加密路径经插件 API 传同一常量）
    const storageCalls = src.match(/setStorageJSON\(([^)]*)\)|getStorageJSON\(([^)]*)\)|removeStorage\(([^)]*)\)|(?<![\w$.])(setSecureItem|getSecureItem|removeSecureItem|hasSecureItem|clearSecureItems)\(([^)]*)\)/g) || [];
    expect(storageCalls.length).toBeGreaterThanOrEqual(4);
    for (const call of storageCalls) {
      expect(call).toContain('STORAGE_KEY_CREDENTIALS');
      expect(call).not.toMatch(/['"]\w+['"]/); // 不允许内联字面量 key
    }
  });

  it('App 端加密后端已接入（uni-secure-storage 插件，条件编译 #ifdef APP，具名导入）', () => {
    expect(src).toContain("from '@/uni_modules/uni-secure-storage'");
    expect(src).toContain('// #ifdef APP');
    expect(src).toContain('    return getSecureStorageCapabilities().supported');
    // error15：插件具名导出 + import * 即 Kotlin 编译失败，禁止命名空间导入
    expect(src).not.toContain('import * as secStore');
  });

  it('constants 中 STORAGE_KEY_CREDENTIALS 存在且旧 LAST key 已删除', () => {
    expect(appConst).toContain('STORAGE_KEY_CREDENTIALS');
    expect(appConst).not.toContain('STORAGE_KEY_LAST_USERNAME');
    expect(appConst).not.toContain('STORAGE_KEY_LAST_PASSWORD');
  });

  it('底层存储封装存在（setStorageJSON/getStorageJSON/removeStorage）', () => {
    const storageSrc = fs.readFileSync(STORAGE_PATH, 'utf8');
    expect(storageSrc).toContain('export function setStorageJSON');
    expect(storageSrc).toContain('export function getStorageJSON');
    expect(storageSrc).toContain('export function removeStorage');
  });
});
