/**
 * 随问图片（#2299）· **行为级**测试 —— 名额 / 上传 / 纯图轮次判定，全部**真执行**
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` —— 把 `composables/useAiImagePicker.uts` 与
 * `utils/aiImageQuota.uts` 当 JS 跑起来；注入的是**真件**的产量（quota 纯函数原样注入 picker），
 * 只有 `uni` 与上传出口是替身（被测的是编排，不是设备）。
 *
 * 为什么是行为而不是源码文本：票面验收里「第 4 张当场拦住并说明上限」「选错的图能删掉再重选」
 * 「上传失败给可读提示」「有图无字也能发」—— 全是**跑起来才看得见**的东西：
 * 交给 `uni.chooseImage` 的 count 是不是剩余额度、上传真发了几次、上传中占位张数对不对。
 *
 * **成对取证**（③ 判据③）：末组用注入变异的副本证明判据**有牙** —— 三种真实坏实现：
 *   ① 拆掉上传循环里的第二道名额闸门（并发多选会超限）；
 *   ② `count` 写死成上限、不按剩余额度（已选若干张后仍报满额）；
 *   ③ 上传中占位不计数（学员看不出「在传」还是「卡死」）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const PICKER_UTS = path.join(__dirname, '..', 'composables', 'useAiImagePicker.uts');
const QUOTA_UTS = path.join(__dirname, 'aiImageQuota.uts');

/** Vue 反应式的**最小替身**（node 侧解析不到 vue；被测的从来不是 Vue 的调度） */
const vueStub = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

const flush = () => new Promise((resolve) => setImmediate(resolve));

/**
 * 组装 picker 实例 + 替身记录。
 * @param opts.pick      下一次 chooseImage 回调给回的本地路径（不传则不回调成功分支）
 * @param opts.upload    上传出口：返回 URL 或 reject
 */
function harness(opts = {}) {
  const rec = { choose: [], upload: [], toast: [], loading: [], hideLoading: 0, preview: null };
  const uni = {
    chooseImage: (o) => {
      rec.choose.push(o);
      if (opts.pick) o.success({ tempFilePaths: opts.pick });
    },
    previewImage: (o) => { rec.preview = o; },
    showToast: (o) => { rec.toast.push(o.title); },
    showLoading: (o) => { rec.loading.push(o.title); },
    hideLoading: () => { rec.hideLoading += 1; },
  };
  const quota = loadUts(QUOTA_UTS, {});
  const uploadFn = opts.upload || (() => Promise.resolve('https://e.com/ok.png'));
  const picker = loadUts(PICKER_UTS, {
    ...vueStub(),
    uploadAiImageApi: (p) => { rec.upload.push(p); return uploadFn(p); },
    previewImages: (urls, index) => { rec.preview = { urls, current: index }; },
    resolveFileUrl: (u) => u,
    AI_MAX_IMAGES: 3,
    remainingSlots: quota.remainingSlots,
    capNotice: quota.capNotice,
    removeAt: quota.removeAt,
    uni,
  });
  return { api: picker.useAiImagePicker(), rec, quota };
}

// ===== ① 名额：三张上限、第 4 张当场拦住并说明上限、count 按剩余额度 =====

describe('名额上限（与服务端 diagnosisMaxImages 同值）：第 4 张当场拦住', () => {
  it('一次选 4 张 ⇒ 只进 3 张，第 4 张没被上传', async () => {
    const { api, rec } = harness({ pick: ['a.png', 'b.png', 'c.png', 'd.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage();
    await flush();
    expect(api.images.value).toEqual(['u-a.png', 'u-b.png', 'u-c.png']);
    expect(rec.upload).toEqual(['a.png', 'b.png', 'c.png']);
  });

  it('已满后再点附件 ⇒ 不再调起系统选择器，并按上限给可读文案', async () => {
    const { api, rec } = harness({ pick: ['a.png', 'b.png', 'c.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage();
    await flush();
    expect(api.images.value.length).toBe(3);
    rec.choose.length = 0;
    api.onAddImage();
    expect(rec.choose.length).toBe(0);
    expect(rec.toast[rec.toast.length - 1]).toBe('最多上传 3 张图片');
  });

  it('count = 剩余额度（已选 2 张 ⇒ 系统选择器最多收 1 张）', async () => {
    const { api, rec } = harness({ maxImages: 3, pick: ['a.png', 'b.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage();
    await flush();
    api.onAddImage();
    expect(rec.choose[1].count).toBe(1);
  });

  it('删掉一张后名额归还（不超限的前提下可再选 1 张）', async () => {
    const { api, rec } = harness({ pick: ['a.png', 'b.png', 'c.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage();
    await flush();
    api.onRemoveImage(1);
    expect(api.images.value).toEqual(['u-a.png', 'u-c.png']);
    api.onAddImage();
    expect(rec.choose[rec.choose.length - 1].count).toBe(1);
  });
});

// ===== ② 上传：可见占位、失败可读、逐张、不重入 =====

describe('上传：有占位、失败给可读提示、不静默丢图', () => {
  it('上传中 uploadingCount > 0（托盘可见占位的唯一来源），完成后归零', async () => {
    let release = null;
    const gate = new Promise((r) => { release = r; });
    const { api } = harness({ pick: ['a.png'], upload: () => gate });
    api.onAddImage();
    await flush();
    expect(api.uploadingCount.value).toBe(1);
    release('u-a.png');
    await flush();
    expect(api.uploadingCount.value).toBe(0);
    expect(api.images.value).toEqual(['u-a.png']);
  });

  it('部分失败 ⇒ 成功的仍进列，失败张数按实际计数给可读提示（不是静默丢）', async () => {
    const { api, rec } = harness({
      pick: ['a.png', 'b.png', 'c.png'],
      upload: (p) => (p === 'b.png' ? Promise.reject(new Error('boom')) : Promise.resolve('u-' + p)),
    });
    api.onAddImage();
    await flush();
    expect(api.images.value).toEqual(['u-a.png', 'u-c.png']);
    expect(rec.toast).toContain('1 张图片上传失败，请重试');
  });

  it('空选择（用户取消）不触发上传、不开占位', async () => {
    const { api, rec } = harness({ pick: [] });
    api.onAddImage();
    await flush();
    expect(rec.upload).toEqual([]);
    expect(api.uploadingCount.value).toBe(0);
  });

  it('上传中不重入（同一批图不会被上传两遍）', async () => {
    let release = null;
    const gate = new Promise((r) => { release = r; });
    const { api, rec } = harness({ pick: ['a.png'], upload: () => gate });
    api.onAddImage();
    await flush();
    api.onAddImage();
    expect(rec.choose.length).toBe(1);
    release('u-a.png');
    await flush();
    expect(api.images.value.length).toBe(1);
  });
});

// ===== ③ 纯图轮次判定（canSend 单一判据）=====

describe('纯图轮次：有图无字也能发，纯空白照旧不发', () => {
  it('文本为空 + 有图 ⇒ 能发（后端补默认话术）', () => {
    const { quota } = harness({});
    expect(quota.canSend('', 1)).toBe(true);
    expect(quota.canSend('   ', 2)).toBe(true);
  });

  it('文本为空 + 无图 ⇒ 不能发（空发送闸门仍在）', () => {
    const { quota } = harness({});
    expect(quota.canSend('', 0)).toBe(false);
    expect(quota.canSend('  \n ', 0)).toBe(false);
  });

  it('有字无图 ⇒ 照旧能发（老路径不受影响）', () => {
    const { quota } = harness({});
    expect(quota.canSend('面板第三排灯闪红', 0)).toBe(true);
  });

  it('清空（发送后）⇒ 图与占位都归零、名额归还', async () => {
    const { api } = harness({ pick: ['a.png', 'b.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage();
    await flush();
    api.clear();
    expect(api.images.value).toEqual([]);
    expect(api.uploadingCount.value).toBe(0);
  });
});

// ===== ④ 成对取证：注入变异后，上面的判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function loadMutated(replacements, opts = {}) {
  let src = readText(PICKER_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ai-picker-'));
  const file = path.join(dir, 'useAiImagePicker.uts');
  fs.writeFileSync(file, src, 'utf8');
  const rec = { choose: [], upload: [], toast: [], preview: null };
  const uni = {
    chooseImage: (o) => { rec.choose.push(o); if (opts.pick) o.success({ tempFilePaths: opts.pick }); },
    previewImage: () => {},
    showToast: (o) => { rec.toast.push(o.title); },
    showLoading: () => {},
    hideLoading: () => {},
  };
  const quota = loadUts(QUOTA_UTS, {});
  const m = loadUts(file, {
    ...vueStub(),
    uploadAiImageApi: (p) => { rec.upload.push(p); return Promise.resolve('u-' + p); },
    previewImages: () => {},
    resolveFileUrl: (u) => u,
    AI_MAX_IMAGES: 3,
    remainingSlots: quota.remainingSlots,
    capNotice: quota.capNotice,
    removeAt: quota.removeAt,
    uni,
  });
  return { api: m.useAiImagePicker(), rec };
}

describe('成对取证（必红）：判据在坏实现上确实会红', () => {
  it('必红 · 拆掉上传循环里的第二道闸门 ⇒ 并发多选会超限（4 张全进）', async () => {
    const broken = loadMutated(
      [['if (remainingSlots(AI_MAX_IMAGES, images.value.length) <= 0) break', 'if (false) break']],
      { pick: ['a.png', 'b.png', 'c.png', 'd.png'] },
    );
    broken.api.onAddImage();
    await flush();
    // 真源这里恒为 3（① 判据）；坏实现放进第 4 张 ⇒ ① 必红
    expect(broken.api.images.value.length).toBe(4);
  });

  it('必红 · count 写死成上限、不按剩余额度 ⇒ ① 的「count = 剩余额度」判据必红', async () => {
    const good = harness({ pick: ['a.png', 'b.png'], upload: (p) => Promise.resolve('u-' + p) });
    good.api.onAddImage();
    await flush();
    good.api.onAddImage();
    const broken = loadMutated(
      [['count: remainingSlots(AI_MAX_IMAGES, images.value.length),', 'count: AI_MAX_IMAGES,']],
      { pick: ['a.png', 'b.png'] },
    );
    broken.api.images.value = ['u-a.png', 'u-b.png'];
    broken.api.onAddImage();
    // 真源 1（3 - 2）；坏实现恒给 3 ⇒ ① 里 count 的断言判红
    expect([good.rec.choose[1].count, broken.rec.choose[0].count]).toEqual([1, 3]);
  });

  it('必红 · 上传中占位不计数 ⇒ ②「uploadingCount > 0」判据必红', async () => {
    const broken = loadMutated(
      [['uploadingCount.value = uploadingCount.value + 1', 'uploadingCount.value = 0']],
      { pick: ['a.png'] },
    );
    broken.api.onAddImage();
    // 循环体里的自增是同步执行的：真源此刻恒为 1（在传），坏实现恒 0 ⇒ ② 必红
    expect(broken.api.uploadingCount.value).toBe(0);
    await flush();
  });
});
