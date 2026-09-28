/**
 * credential.uts switchCredentialApi —— **行为层**测试（评审 K4 · #1346）
 *
 * 守护的契约：后端 `PATCH /api/me/credential` 已实现（training_catalog.go SetCurrentCredential），
 * 切换失败（网络/500/证件校验拒绝）时 `switchCredentialApi` **必须 reject**，交页面 catch 显示「切换失败」——
 * **不得**再用 mock 兜成 `{ success: true }`（那是 #643「后端未实现」时代的离线兜底，前提已失效）。
 *
 * 真执行 `api/credential.uts`（utsHarness），把 `requestMapped` 注入成 reject，验证上抛。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const CRED = path.join(ROOT, 'api', 'credential.uts');

function loadCred(requestMappedImpl) {
  const uni = { getStorageSync: () => '', setStorageSync: () => {}, showToast: () => {} };
  const bindings = {
    requestMapped: requestMappedImpl,
    getMapped: requestMappedImpl,
    toNumber: (v, d = 0) => (v == null ? d : (Number.isNaN(parseFloat(`${v}`)) ? d : parseFloat(`${v}`))),
    toNumberOrNull: (v) => (v == null ? null : (Number.isNaN(parseFloat(`${v}`)) ? null : parseFloat(`${v}`))),
    toStr: (v, d = '') => (v == null ? d : `${v}`),
    uni,
  };
  return loadUts(CRED, bindings);
}

describe('credential.uts switchCredentialApi：失败上抛、不 mock 假成功（K4 #1346）', () => {
  test('请求失败时必须 reject，不得 resolve success:true', async () => {
    const mod = loadCred(() => Promise.reject(new Error('500 backend reject')));
    // 现状（带 mock 兜底）会 resolve { success:true } ⇒ 本断言判红；去掉兜底后 reject ⇒ 绿
    await expect(mod.switchCredentialApi(1)).rejects.toBeInstanceOf(Error);
  });

  test('成功时透传后端返回的 credential（不吞成功、不读 mock）', async () => {
    const passthrough = (opts, mapper) =>
      Promise.resolve(mapper({
        credential: { id: 3, code: 'welder', name: '焊工证', category: 'special_operation', description: '', level: null, sort_order: 3, status: 1, created_at: '', updated_at: '' },
      }));
    const mod = loadCred(passthrough);
    const r = await mod.switchCredentialApi(3);
    expect(r.success).toBe(true);
    expect(r.credential.id).toBe(3);
  });
});
