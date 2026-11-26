/**
 * 选证整页「两段式」—— **行为层**测试（#2907，自 #2535 首页浮层判据迁来）
 *
 * 为什么是行为腿：票面要求「首页证件面板那四条判据（点选只改待确认项 / 完成才落库 /
 * 恰一次 PATCH / 同证件不发 PATCH）迁到整页后必须仍成立」。这些只有真跑起来才看得见
 * ——源码锁只能证明「有个 onConfirm」，证明不了「点选真的没发请求」「同证件真的零 PATCH」。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`（去掉类型当 JS 真跑），依赖注入替身。
 * .uvue 跑不了（渲染层）⇒ 状态与动作全在 `pages/guide/composables/useChooseCertSelection.uts`，
 * 判据落在该 composable 上；页面接线由 `chooseCertGuideBehavior` / `dashboardContract` 两侧源码锁兜。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const COMPOSABLE = path.join(__dirname, '..', 'pages', 'guide', 'composables', 'useChooseCertSelection.uts');

/** 最小 uni：toast 收集（本件只用到 showToast） */
function makeUni() {
  const toasts = [];
  return { uni: { showToast: (o) => { toasts.push(o.title); } }, toasts };
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

const LEVEL_2 = GROUPS[1].items[0];

/** 起一份选证面；`getCurrent` 默认「当前证件 = 叉车司机N1证（id 1）」 */
function loadSelection({ getCurrent, getAll, switchApi, pickProvince } = {}) {
  const { uni, toasts } = makeUni();
  const patches = [];
  const cacheWrites = [];
  const picked = [];
  const mod = loadUts(COMPOSABLE, {
    ref: (v) => ({ value: v }),
    getCurrentCredentialApi: getCurrent == null
      ? () => Promise.resolve(dict({ id: 1, code: 'forklift_n1', name: '叉车司机N1证' }))
      : getCurrent,
    getAllCredentialsApi: getAll == null ? () => Promise.resolve(GROUPS) : getAll,
    switchCredentialApi: (id) => {
      patches.push(id);
      return switchApi == null ? Promise.resolve({ success: true, credential: null }) : switchApi(id);
    },
    writeCurrentCredentialCache: (c) => { cacheWrites.push(c == null ? null : c.id); },
    DEFAULT_PROVINCE: '全国',
    pickProvince: (cb) => { picked.push(cb); },
    uni,
  });
  return { mod, face: mod.useChooseCertSelection(), toasts, patches, cacheWrites, picked };
}

async function loaded(extra = {}) {
  const ctx = loadSelection(extra);
  await ctx.face.loadList();
  return ctx;
}

describe('选证整页预选：进来就选好当前证件（#2907 票面 ②）', () => {
  test('预选当前证件：selectedId / currentId 都对齐后端回显，且零 PATCH', async () => {
    const { face, patches } = await loaded();
    expect(face.currentId.value).toBe(1);
    expect(face.selectedId.value).toBe(1);
    expect(patches).toEqual([]);
  });

  test('无当前证件（注册流落地）：不预选，selectedId 保持 0', async () => {
    const { face, patches } = await loaded({ getCurrent: () => Promise.resolve(null) });
    expect(face.currentId.value).toBe(0);
    expect(face.selectedId.value).toBe(0);
    expect(patches).toEqual([]);
  });

  test('重拉（失败重试 / 回前台）不覆盖已点选的值', async () => {
    const { face } = await loaded();
    face.onSelect(LEVEL_2);
    expect(face.selectedId.value).toBe(7);
    await face.loadList();
    expect(face.selectedId.value).toBe(7);
  });
});

describe('字典装载失败只砸列表，不砸整页（#1349 判据迁到整页）', () => {
  test('列表失败 ⇒ listFailed + 分组空，但预选值仍在、不弹 toast（页面走空态+重试）', async () => {
    const { face, toasts } = await loaded({ getAll: () => Promise.reject(new Error('list 500')) });
    expect(face.listFailed.value).toBe(true);
    expect(face.groups.value).toEqual([]);
    // 两段各自失败各自呈现：字典失败不能让用户看不见「当前是哪一证」
    expect(face.selectedId.value).toBe(1);
    expect(face.currentId.value).toBe(1);
    expect(toasts).toEqual([]);
  });

  test('重试成功 ⇒ listFailed 复位、分组落位', async () => {
    let n = 0;
    const ctx = loadSelection({
      getAll: () => {
        n += 1;
        return n === 1 ? Promise.reject(new Error('list 500')) : Promise.resolve(GROUPS);
      },
    });
    await ctx.face.loadList();
    expect(ctx.face.listFailed.value).toBe(true);
    await ctx.face.loadList();
    expect(ctx.face.listFailed.value).toBe(false);
    expect(ctx.face.groups.value.length).toBe(2);
  });
});

describe('两段式：点选只改待提交项，完成才落库（#2535 Q3 判据迁到整页）', () => {
  test('点选只改 selectedId：不 PATCH、当前证件不动、不写缓存', async () => {
    const { face, patches, cacheWrites } = await loaded();
    face.onSelect(LEVEL_2);
    expect(face.selectedId.value).toBe(7);
    expect(face.currentId.value).toBe(1);
    expect(patches).toEqual([]);
    expect(cacheWrites).toEqual([]);
  });

  test('完成才落库：恰一次 PATCH，落的是选中项，后端确认值写缓存并成为当前证件', async () => {
    const { mod, face, patches, cacheWrites } = await loaded({
      // 后端确认切换（回带凭证）——未确认的 200 按失败处理，那是 #1346 的既有口径
      switchApi: (id) => Promise.resolve({ success: true, credential: dict({ id: id, code: 'maintenance_L2', name: '二级', level: 2, category: 'skill_level' }) }),
    });
    face.onSelect(LEVEL_2);
    const result = await face.onConfirm();

    expect(result).toBe(mod.SUBMIT_SWITCHED);
    expect(patches).toEqual([7]);
    expect(cacheWrites).toEqual([7]);
    expect(face.currentId.value).toBe(7);
    expect(face.selectedId.value).toBe(7);
  });

  test('选中值 == 当前证件：未变更，零 PATCH 零缓存写（不选直接返回同理）', async () => {
    const { mod, face, patches, cacheWrites } = await loaded();
    const result = await face.onConfirm();

    expect(result).toBe(mod.SUBMIT_UNCHANGED);
    expect(patches).toEqual([]);
    expect(cacheWrites).toEqual([]);
    expect(face.currentId.value).toBe(1);
  });

  test('一个都没选：提示「请先选择一个证书」，零 PATCH、留在本页', async () => {
    const { mod, face, toasts, patches } = await loaded({ getCurrent: () => Promise.resolve(null) });
    const result = await face.onConfirm();

    expect(result).toBe(mod.SUBMIT_NONE);
    expect(toasts).toEqual(['请先选择一个证书']);
    expect(patches).toEqual([]);
  });

  test('后端未确认（success=false）：按失败处理——不写缓存、当前证件不动、停在 failed', async () => {
    const { mod, face, toasts, patches, cacheWrites } = await loaded({
      switchApi: () => Promise.resolve({ success: false, credential: null }),
    });
    face.onSelect(LEVEL_2);
    const result = await face.onConfirm();

    expect(result).toBe(mod.SUBMIT_FAILED);
    expect(patches).toEqual([7]);
    expect(cacheWrites).toEqual([]);
    expect(face.currentId.value).toBe(1);
    expect(toasts).toEqual(['切换失败，请重试']);
  });

  test('请求抛错：同样停在 failed、当前证件不动（可重试）', async () => {
    const { mod, face, toasts } = await loaded({
      switchApi: () => Promise.reject(new Error('network')),
    });
    face.onSelect(LEVEL_2);
    const result = await face.onConfirm();

    expect(result).toBe(mod.SUBMIT_FAILED);
    expect(face.currentId.value).toBe(1);
    expect(toasts).toEqual(['切换失败，请重试']);
  });

  test('提交中再次点完成不叠加请求（isSubmitting 守卫）', async () => {
    let release = null;
    const { mod, face, patches } = await loaded({
      switchApi: (id) => new Promise((resolve) => { release = () => resolve({ success: true, credential: dict({ id, name: '二级', level: 2, category: 'skill_level' }) }); }),
    });
    face.onSelect(LEVEL_2);
    const first = face.onConfirm();
    const second = await face.onConfirm();
    release();
    await first;

    expect(patches).toEqual([7]);
    expect(second).toBe(mod.SUBMIT_FAILED);
  });
});

describe('常量与归属（#2907）', () => {
  test('首页入口的 from 取值就是 dashboard；四种提交结果是互不相同的字符串', () => {
    const { mod } = loadSelection({});
    expect(mod.CHOOSE_CERT_FROM_DASHBOARD).toBe('dashboard');
    const results = [mod.SUBMIT_SWITCHED, mod.SUBMIT_UNCHANGED, mod.SUBMIT_NONE, mod.SUBMIT_FAILED];
    expect(new Set(results).size).toBe(4);
  });

  test('省份是展示性字段：选择只改本地回显，与选证流无关', async () => {
    const { face, patches, picked } = await loaded();
    face.onProvinceTap();
    // 真机是系统 action sheet 回调；这里拿被捕的回调模拟「用户选了广东」
    expect(picked.length).toBe(1);
    picked[0]('广东');
    expect(face.province.value).toBe('广东');
    expect(patches).toEqual([]);
  });
});
