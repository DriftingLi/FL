/**
 * credential.uts switchCredentialApi —— **行为层**测试（评审 K4 · #1346）
 *
 * 守护的契约：后端 `PATCH /api/me/credential` 已实现（training_catalog.go SetCurrentCredential），
 * 切换失败（网络/500/证件校验拒绝）时 `switchCredentialApi` **必须 reject**，交页面 catch 显示「切换失败」——
 * **不得**再用 mock 兜成 `{ success: true }`（那是 #643「后端未实现」时代的离线兜底，前提已失效）。
 *
 * 真执行 `api/credential.uts`（utsHarness），把 `patchMapped` 注入成 reject，验证上抛。
 *
 * #1734 问题 2：PATCH 出口从 `requestMapped({ method: 'PATCH' })` 手作长径收口成 `patchMapped` 短径
 * ⇒ 「发出了 PATCH 请求」这件事现在由**走哪个出口**表达。其余出口一律注入「一被调用就抛」的桩，
 * 这样若实现退回别的通路（putMapped / requestMapped / 裸 patch），测试立刻红，而不是静默换轨。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const CRED = path.join(ROOT, 'api', 'credential.uts');

/** 走错出口 = 立刻抛（防「换条通路把断言绕过去」的假绿） */
const wrongOutlet = (name) => () => Promise.reject(new Error('switchCredentialApi 不该走 ' + name + ' 出口'));

function loadCred(patchMappedImpl) {
  const uni = { getStorageSync: () => '', setStorageSync: () => {}, showToast: () => {} };
  const bindings = {
    patchMapped: patchMappedImpl,
    getMapped: () => Promise.resolve({}),
    putMapped: wrongOutlet('putMapped'),
    requestMapped: wrongOutlet('requestMapped'),
    patch: wrongOutlet('裸 patch'),
    toNumber: (v, d = 0) => (v == null ? d : (Number.isNaN(parseFloat(`${v}`)) ? d : parseFloat(`${v}`))),
    toNumberOrNull: (v) => (v == null ? null : (Number.isNaN(parseFloat(`${v}`)) ? null : parseFloat(`${v}`))),
    toStr: (v, d = '') => (v == null ? d : `${v}`),
    uni,
  };
  return loadUts(CRED, bindings);
}

describe('credential.uts switchCredentialApi：失败上抛、不 mock 假成功（K4 #1346）', () => {
  test('请求失败时必须 reject（且确实发出了 PATCH 请求，非无条件 reject）', async () => {
    const calls = [];
    const spy = (url, data, map) => { calls.push({ url, data }); return Promise.reject(new Error('500 backend reject')); };
    const mod = loadCred(spy);
    // 现状（带 mock 兜底）会 resolve { success:true } ⇒ 本断言判红；去掉兜底后 reject ⇒ 绿
    await expect(mod.switchCredentialApi(1)).rejects.toBeInstanceOf(Error);
    // 防「无条件 reject」假绿：断言真发了请求且形态正确（PATCH 语义 = 走的是 patchMapped 出口）
    expect(calls).toHaveLength(1);
    expect(calls[0].url).toContain('/me/credential');
    expect(calls[0].data.credential_id).toBe(1);
  });

  test('成功时透传后端返回的 credential（不吞成功、不读 mock）', async () => {
    const passthrough = (url, data, mapper) =>
      Promise.resolve(mapper({
        credential: { id: 3, code: 'welder', name: '焊工证', category: 'special_operation', description: '', level: null, sort_order: 3, status: 1, created_at: '', updated_at: '' },
      }));
    const mod = loadCred(passthrough);
    const r = await mod.switchCredentialApi(3);
    expect(r.success).toBe(true);
    expect(r.credential.id).toBe(3);
  });
});
