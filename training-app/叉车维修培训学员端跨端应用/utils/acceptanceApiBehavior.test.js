/**
 * 验收域 API 行为测试（#2743）
 *
 * 守的是用户点得出来的闭环：列表 → 详情 → 提交 → 历史里能看到 → 不能重复提交。
 * 数据源目前是端内 mock（`ACCEPTANCE_USE_MOCK = true`），所以这里同时钉住
 * 「mock 模式下真实传输层一次都没被调用」——否则「跑通了」可能只是悄悄走了网络。
 *
 * 每次用例都重新装载 mock + api（loadUts 每次是全新实例），用例之间互不串数据。
 */
const h = require('./utsHarness');

function loadApi() {
  const form = h.loadUts('utils/acceptanceForm.uts', {});
  const mock = h.loadUts('api/acceptanceMock.uts', {
    acceptanceRollupResult: form.acceptanceRollupResult,
  });
  const calls = { getMapped: 0, getPaged: 0, postMapped: 0, uploadFile: 0 };
  const transport = (name) => (...args) => {
    calls[name] += 1;
    throw new Error('mock 模式不应触发真实请求：' + name + '(' + args.length + ')');
  };
  const api = h.loadUts('api/acceptance.uts', {
    getMapped: transport('getMapped'),
    getPaged: transport('getPaged'),
    postMapped: transport('postMapped'),
    uploadFile: transport('uploadFile'),
    decodeList: () => {
      throw new Error('mock 模式不应触发 decodeList');
    },
    toNumber: (v, d) => (v == null || isNaN(parseFloat(v)) ? (d == null ? 0 : d) : parseFloat(v)),
    toStr: (v, d) => (v == null ? (d == null ? '' : d) : String(v)),
    mockAcceptanceTasks: mock.mockAcceptanceTasks,
    mockAcceptanceDetail: mock.mockAcceptanceDetail,
    mockAcceptanceHistory: mock.mockAcceptanceHistory,
    mockAcceptanceSubmit: mock.mockAcceptanceSubmit,
    mockAcceptanceUpload: mock.mockAcceptanceUpload,
  });
  return { api, calls };
}

/** 把详情里的项按给定结论拼成交付载荷 */
function payloadOf(detail, results, remarkOf, proofsOf) {
  return detail.items.map((it) => ({
    item_id: it.id,
    result: results[it.id] || 'pass',
    remark: remarkOf && remarkOf[it.id] ? remarkOf[it.id] : '',
    proofs: proofsOf && proofsOf[it.id] ? proofsOf[it.id] : [],
  }));
}

describe('mock 开关与数据源', () => {
  it('开关为 true 时全走端内数据，真实传输层零调用', async () => {
    const { api, calls } = loadApi();
    expect(api.ACCEPTANCE_USE_MOCK).toBe(true);
    await api.listPendingAcceptanceApi();
    await api.listAcceptanceHistoryApi(1, 10);
    const detail = await api.getAcceptanceDetailApi(101);
    await api.getAcceptanceDetailApi(detail.task.id);
    expect(calls).toEqual({ getMapped: 0, getPaged: 0, postMapped: 0, uploadFile: 0 });
  });

  it('待验收列表只回 pending 单据', async () => {
    const { api } = loadApi();
    const tasks = await api.listPendingAcceptanceApi();
    expect(tasks.length).toBe(2);
    expect(tasks.every((t) => t.status === 'pending')).toBe(true);
    expect(tasks.map((t) => t.id)).toEqual([101, 102]);
  });

  it('按状态筛选：submitted / done 各自只回对应单据', async () => {
    const { api } = loadApi();
    const submitted = await api.listAcceptanceTasksApi('submitted');
    expect(submitted.length).toBe(0);
    const done = await api.listAcceptanceTasksApi('done');
    expect(done.map((t) => t.id).sort()).toEqual([201, 202]);
  });

  it('详情带回完整验收项，初始全部未确认', async () => {
    const { api } = loadApi();
    const detail = await api.getAcceptanceDetailApi(101);
    expect(detail.task.code).toBe('YS-2026-1101');
    expect(detail.items.length).toBe(4);
    expect(detail.items.every((it) => it.result === '')).toBe(true);
    expect(detail.task.submitted_at).toBeNull();
  });

  it('不存在的验收单被拒绝（页面据此显示「验收单不存在」）', async () => {
    const { api } = loadApi();
    await expect(api.getAcceptanceDetailApi(9999)).rejects.toThrow('验收单不存在');
  });
});

describe('凭证上传', () => {
  it('mock 原样回本地路径与类型', async () => {
    const { api } = loadApi();
    const p = await api.uploadAcceptanceProofApi('wxfile://tmp/a.jpg', '凭证图片', 'image');
    expect(p).toEqual({ url: 'wxfile://tmp/a.jpg', name: '凭证图片', kind: 'image' });
  });
});

describe('提交闭环', () => {
  it('提交 → 列表不再出现 → 详情保留逐项结论与凭证 → 历史可见', async () => {
    const { api } = loadApi();
    const detail = await api.getAcceptanceDetailApi(101);
    const proof = { url: 'wxfile://tmp/a.jpg', name: '凭证图片', kind: 'image' };
    const payload = payloadOf(detail, {}, null, { 1001: [proof] });

    const res = await api.submitAcceptanceApi(101, payload);
    expect(res.status).toBe('submitted');
    expect(res.result).toBe('pass');
    expect(res.submitted_at.length).toBeGreaterThan(0);

    const pending = await api.listPendingAcceptanceApi();
    expect(pending.map((t) => t.id)).toEqual([102]);

    const after = await api.getAcceptanceDetailApi(101);
    expect(after.task.status).toBe('submitted');
    expect(after.items[0].proofs.length).toBe(1);
    expect(after.items[0].proofs[0].url).toBe('wxfile://tmp/a.jpg');
    expect(after.items.every((it) => it.result === 'pass')).toBe(true);

    const history = await api.listAcceptanceHistoryApi(1, 50);
    const row = history.items.find((it) => it.id === 101);
    expect(row).toBeTruthy();
    expect(row.status).toBe('submitted');
    expect(row.failed_count).toBe(0);
  });

  it('带「待整改」项时整体结论是 rectify（与页面同一口径）', async () => {
    const { api } = loadApi();
    const detail = await api.getAcceptanceDetailApi(102);
    const payload = payloadOf(detail, { 2003: 'rectify' }, { 2003: '压力值偏离，需复检' }, {
      2001: [{ url: 'wxfile://tmp/b.jpg', name: '凭证图片', kind: 'image' }],
    });
    const res = await api.submitAcceptanceApi(102, payload);
    expect(res.result).toBe('rectify');
    const after = await api.getAcceptanceDetailApi(102);
    expect(after.items[2].remark).toBe('压力值偏离，需复检');
  });

  it('提交时带整单备注，详情里能读回', async () => {
    const { api } = loadApi();
    const detail = await api.getAcceptanceDetailApi(101);
    const payload = payloadOf(detail, {}, null, {
      1001: [{ url: 'wxfile://tmp/a.jpg', name: '凭证图片', kind: 'image' }],
    });
    await api.submitAcceptanceApi(101, payload, '复检安排在周五');
    const after = await api.getAcceptanceDetailApi(101);
    expect(after.task.remark).toBe('复检安排在周五');
  });

  it('同一张单不能提交两次（防重复提交）', async () => {
    const { api } = loadApi();
    const detail = await api.getAcceptanceDetailApi(101);
    const payload = payloadOf(detail, {}, null, {
      1001: [{ url: 'wxfile://tmp/a.jpg', name: '凭证图片', kind: 'image' }],
    });
    await api.submitAcceptanceApi(101, payload);
    await expect(api.submitAcceptanceApi(101, payload)).rejects.toThrow('已提交');
  });

  it('不存在的单提交被拒绝', async () => {
    const { api } = loadApi();
    await expect(api.submitAcceptanceApi(9999, [])).rejects.toThrow('验收单不存在');
  });
});

describe('历史列表', () => {
  it('种子里已有两条历史，且按提交时间倒序', async () => {
    const { api } = loadApi();
    const page = await api.listAcceptanceHistoryApi(1, 50);
    expect(page.items.length).toBe(2);
    expect(page.items[0].id).toBe(201); // 2026-11-08 > 2026-10-30
    expect(page.items[1].id).toBe(202);
    expect(page.hasMore).toBe(false);
  });
});
