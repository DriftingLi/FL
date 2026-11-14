/**
 * dashboard 证件面板「待确认 → 完成才落库」—— **行为层**测试（#2535 Q3）
 *
 * 设计稿 324_9793 把 dashboard 的证件浮层升级成与 choose-cert 主态同源的完整面板
 * （省份 + 分组 + 热门考证 + 完成）。语义上多了一条**两段式**：
 * 面板内点选只改「待确认项」，点「完成」才真 PATCH 落库，关闭面板不改证件。
 *
 * 这条语义只有跑起来才看得见（源码锁只能钉「有没有那个函数」），所以放行为腿：
 * 1) 点选不落库：`switchCredentialApi` 零调用、面板不关、`currentCredentialId` 不动；
 * 2) 完成才落库：恰一次 PATCH，且走的是既有乐观更新链路（`onSelectCredential` 函数体未动）；
 * 3) 取消复位：关闭面板把待确认项复位成当前证件，下次打开不残留上次的临时选择；
 * 4) 省份是展示性字段：选择只改本地回显，不发请求。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const COMPOSABLE = path.join(ROOT, 'pages', 'dashboard', 'composables', 'use-dashboard-credential.uts');

/** 最小 uni：storage 走 map（缺键回 ''），toast 收集 */
function makeUni(init = {}) {
  const store = { ...init };
  const toasts = [];
  const uni = {
    getStorageSync: (k) => (store[k] === undefined ? '' : store[k]),
    setStorageSync: (k, v) => { store[k] = v; },
    removeStorageSync: (k) => { delete store[k]; },
    showToast: (o) => { toasts.push(o.title); },
    reLaunch: () => {},
    showActionSheet: () => {},
  };
  return { uni, store, toasts };
}

/** 后端 CredentialDict 的线上形态（字段序按 training_catalog_types.go:107 的字母序声明） */
const dict = (over) => ({
  category: 'special_operation', code: 'forklift_n1', created_at: '2026-08-27T22:03:10+08:00',
  description: '场内专用机动车辆作业-叉车司机N1', id: 1, level: null, name: '叉车司机N1证',
  sort_order: 1, status: 1, updated_at: '2026-08-27T22:03:10+08:00', ...over,
});

/** 两组：特种作业（单行胶囊）+ 技能等级（双行胶囊） */
const GROUPS = [
  {
    group_name: '特种作业配套上岗证', icon: 'x', category: 'special_operation',
    items: [dict({ id: 1, code: 'forklift_n1', name: '叉车司机N1证' }), dict({ id: 2, code: 'welder', name: '焊工证' })],
  },
  {
    group_name: '工程机械维修工', icon: 'x', category: 'skill_level',
    items: [dict({ id: 7, code: 'maintenance_L2', name: '二级', level: 2, category: 'skill_level' })],
  },
];

function loadComposable({ getCurrent, getAll, switchApi, pickProvince } = {}) {
  const { uni, toasts } = makeUni();
  const patches = [];
  const picked = [];
  const mod = loadUts(COMPOSABLE, {
    ref: (v) => ({ value: v }),
    getCurrentCredentialApi: getCurrent == null ? () => Promise.resolve(null) : getCurrent,
    getAllCredentialsApi: getAll == null ? () => Promise.resolve([]) : getAll,
    switchCredentialApi: (id) => {
      patches.push(id);
      return switchApi == null ? Promise.resolve({ success: true, credential: null }) : switchApi(id);
    },
    readCurrentCredentialCache: () => null,
    writeCurrentCredentialCache: () => {},
    DEFAULT_PROVINCE: '全国',
    pickProvince: (cb) => {
      picked.push(cb);
      if (pickProvince != null) pickProvince(cb);
    },
    uni,
  });
  return { face: mod.useDashboardCredential(), toasts, patches, picked };
}

/** 起一个「当前证件 = 叉车司机N1证（id 1）」的已加载面板 */
async function loadedFace(extra = {}) {
  const ctx = loadComposable({
    getCurrent: () => Promise.resolve(dict({ id: 1, code: 'forklift_n1', name: '叉车司机N1证' })),
    getAll: () => Promise.resolve(GROUPS),
    ...extra,
  });
  await ctx.face.loadCredentials();
  return ctx;
}

describe('证件面板两段式：点选只改待确认项，完成才落库（#2535 Q3）', () => {
  test('初值：待确认项 0、省份「全国」', () => {
    const { face } = loadComposable({});
    expect(face.pendingCredentialId.value).toBe(0);
    expect(face.province.value).toBe('全国');
  });

  test('列表到位后待确认项对齐当前证件（面板首次打开高亮不落空）', async () => {
    const { face } = await loadedFace();
    expect(face.currentCredentialId.value).toBe(1);
    expect(face.pendingCredentialId.value).toBe(1);
  });

  test('点选只改待确认项：不 PATCH、不关面板、当前证件不动', async () => {
    const { face, patches } = await loadedFace();
    face.onCertTap();
    expect(face.showCertDropdown.value).toBe(true);

    face.onPickCredential(GROUPS[1].items[0]);

    expect(face.pendingCredentialId.value).toBe(7);
    expect(face.currentCredentialId.value).toBe(1);
    expect(face.currentCert.value).toBe('叉车司机N1证');
    expect(face.showCertDropdown.value).toBe(true);
    expect(patches).toEqual([]);
  });

  test('完成才落库：恰一次 PATCH，且落的是待确认项', async () => {
    const { face, patches } = await loadedFace({
      // 后端确认切换（回带凭证）——未确认的 200 会按失败回滚，那是 #1346 的既有口径
      switchApi: (id) => Promise.resolve({
        success: true,
        credential: dict({ id: id, code: 'maintenance_L2', name: '二级', level: 2, category: 'skill_level' }),
      }),
    });
    face.onCertTap();
    face.onPickCredential(GROUPS[1].items[0]);
    face.onConfirmCredential();
    await Promise.resolve();
    await Promise.resolve();

    expect(patches).toEqual([7]);
    expect(face.currentCredentialId.value).toBe(7);
    expect(face.currentCert.value).toBe('二级');
    expect(face.showCertDropdown.value).toBe(false);
  });

  test('待确认项 == 当前证件时点完成：只关面板，不发 PATCH', async () => {
    const { face, patches } = await loadedFace();
    face.onCertTap();
    face.onConfirmCredential();
    await Promise.resolve();

    expect(patches).toEqual([]);
    expect(face.showCertDropdown.value).toBe(false);
  });

  test('取消（关闭面板）复位待确认项，且不改证件', async () => {
    const { face, patches } = await loadedFace();
    face.onCertTap();
    face.onPickCredential(GROUPS[1].items[0]);
    face.onCloseCertDropdown();

    expect(face.showCertDropdown.value).toBe(false);
    expect(face.pendingCredentialId.value).toBe(1);
    expect(face.currentCredentialId.value).toBe(1);
    expect(patches).toEqual([]);
  });

  test('再次打开面板不残留上次的临时选择', async () => {
    const { face } = await loadedFace();
    face.onCertTap();
    face.onPickCredential(GROUPS[1].items[0]);
    face.onCloseCertDropdown();
    face.onCertTap();

    expect(face.pendingCredentialId.value).toBe(1);
  });

  test('省份是展示性字段：选择只改本地回显，不发请求', async () => {
    const { face, patches, picked } = await loadedFace({
      pickProvince: (cb) => cb('广东'),
    });
    face.onProvinceTap();

    expect(picked.length).toBe(1);
    expect(face.province.value).toBe('广东');
    expect(patches).toEqual([]);
  });
});
