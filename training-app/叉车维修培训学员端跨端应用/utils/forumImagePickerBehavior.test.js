/**
 * 论坛图片流水线（选图 / 逐张上传 / 删除 / 预览）· **行为级**测试
 * （ADR-0025 ⑦；`/code-review` 双轴评审后从两个宿主里抽出来的共享实现）
 *
 * 为什么是行为而不是源码文本：这段逻辑原先在发帖页与回复 composable 里各有一份（只差 9 / 3 这个限额），
 * 而它的判据全是**跑起来才看得见**的东西 —— 交给 `uni.chooseImage` 的 `sourceType` 到底含哪个来源、
 * 上传真的发了几次、限额闸门是不是在上传循环里也生效、失败计数对不对。
 * 缝：`utils/utsHarness.js` 的 `loadUts`；注入的是**真件的** `utils/forumDisplay.resolveFileUrl` 与
 * `utils/forumDetailDisplay.previewImages`，只有 `uni` 与网络上传出口是替身（被测的是编排，不是设备）。
 *
 * **成对取证**（③ 判据③）：末组用注入变异的副本证明判据**有牙** —— 两种真实坏实现：
 *   ① 双入口退化回共用的系统选择器（`sourceType: ['album','camera']`）；
 *   ② 限额闸门只剩选图那一处、上传循环里的那道被拆掉（并发多选时会超限）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const PICKER_UTS = path.join(__dirname, '..', 'composables', 'useForumImagePicker.uts');
const DISPLAY_UTS = path.join(__dirname, 'forumDisplay.uts');
const DETAIL_DISPLAY_UTS = path.join(__dirname, 'forumDetailDisplay.uts');

/**
 * Vue 反应式的**最小替身**：本仓 node 侧解析不到 `vue`（它是 uni-app-x 编译期依赖），
 * 而被测的从来不是 Vue 的调度 —— 是 picker 的编排。替的是**依赖**，被测物仍是仓内真件
 * （与假 `uni` 同一地位，不是手抄镜像）。
 */
const vueStub = () => ({
  ref: (v) => ({ value: v }),
  computed: (fn) => ({ get value() { return fn(); } }),
});

/** `flush`：`onAddImage` 内部不 await 上传，测试要等微任务跑完 */
const flush = () => new Promise((resolve) => setImmediate(resolve));

/**
 * 组装一个 picker 实例 + 全部替身记录。
 * @param maxImages 限额
 * @param pick      下一次 `chooseImage` 回调给回的本地路径（不传则不回调成功分支）
 * @param upload    上传出口：返回 URL 或 reject
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
  const display = loadUts(DISPLAY_UTS, {});
  const detailDisplay = loadUts(DETAIL_DISPLAY_UTS, { resolveFileUrl: display.resolveFileUrl, uni });
  const uploadFn = opts.upload || (() => Promise.resolve('https://e.com/ok.png'));
  const picker = loadUts(PICKER_UTS, {
    ...vueStub(),
    uploadForumImageApi: (p) => { rec.upload.push(p); return uploadFn(p); },
    previewImages: detailDisplay.previewImages,
    resolveFileUrl: display.resolveFileUrl,
    uni,
  });
  return { api: picker.useForumImagePicker(opts.maxImages == null ? 9 : opts.maxImages), rec };
}

// ===== ① 双入口直达（ADR-0025 ⑦）=====

describe('选图双入口：来源**直达**，不共用系统选择器', () => {
  it('`camera` ⇒ sourceType 只含 camera；`album` ⇒ 只含 album；count = 剩余额度', () => {
    const { api, rec } = harness({ maxImages: 9 });
    api.onAddImage('camera');
    api.onAddImage('album');
    expect(rec.choose.length).toBe(2);
    expect(rec.choose[0].sourceType).toEqual(['camera']);
    expect(rec.choose[1].sourceType).toEqual(['album']);
    expect([rec.choose[0].count, rec.choose[1].count]).toEqual([9, 9]);
    expect(rec.choose[0].sizeType).toEqual(['compressed']);
  });

  it('已选若干张后 count 只剩剩余额度', () => {
    const { api, rec } = harness({ maxImages: 3, pick: ['a.png'], upload: () => Promise.resolve('u1') });
    api.onAddImage('album');
    return flush().then(() => {
      api.onAddImage('album');
      expect(rec.choose[1].count).toBe(2);
    });
  });
});

// ===== ② 限额闸门（且闸门有两道）=====

describe('图片限额：选图前挡一次、上传循环里再挡一次（并发多选也不会超）', () => {
  it('额度用尽 ⇒ 不再调起系统选择器，并按**该面的限额**提示', async () => {
    const { api, rec } = harness({ maxImages: 3, pick: ['a.png', 'b.png', 'c.png', 'd.png'], upload: (p) => Promise.resolve('u-' + p) });
    api.onAddImage('album');
    await flush();
    expect(api.images.value.length).toBe(3);           // 上限就是 3（第 4 张没进来）
    rec.choose.length = 0;
    api.onAddImage('album');
    expect(rec.choose.length).toBe(0);                  // 不再调起选择器
    expect(rec.toast[rec.toast.length - 1]).toBe('最多上传 3 张图片');
  });

  it('限额是**参数**：同一个实现换个 maxImages 就换文案与闸门（发帖 9 / 回复 3 共用一个实现）', () => {
    const { api, rec } = harness({ maxImages: 9 });
    // 先灌满 9 张
    api.images.value = ['1', '2', '3', '4', '5', '6', '7', '8', '9'];
    api.onAddImage('camera');
    expect(rec.choose.length).toBe(0);
    expect(rec.toast[rec.toast.length - 1]).toBe('最多上传 9 张图片');
  });
});

// ===== ③ 逐张上传与失败计数 =====

describe('上传：逐张走上传出口，失败只计数不中断，并汇总提示', () => {
  it('全部成功 ⇒ images 追加、预览 URL 由 resolveFileUrl 预解析', async () => {
    const { api, rec } = harness({ maxImages: 9, pick: ['a.png', 'b.png'], upload: (p) => Promise.resolve('https://e.com/' + p) });
    api.onAddImage('camera');
    await flush();
    expect(rec.upload).toEqual(['a.png', 'b.png']);
    expect(api.images.value).toEqual(['https://e.com/a.png', 'https://e.com/b.png']);
    expect(api.previewImages.value).toEqual(['https://e.com/a.png', 'https://e.com/b.png']);
    expect([rec.loading.length, rec.hideLoading]).toEqual([1, 1]);
  });

  it('有失败 ⇒ 成功的仍进列表，失败数**按实际失败张数**汇总', async () => {
    const { api, rec } = harness({
      maxImages: 9,
      pick: ['a.png', 'b.png', 'c.png'],
      upload: (p) => (p === 'b.png' ? Promise.reject(new Error('boom')) : Promise.resolve('https://e.com/' + p)),
    });
    api.onAddImage('album');
    await flush();
    expect(api.images.value).toEqual(['https://e.com/a.png', 'https://e.com/c.png']);
    expect(rec.toast).toContain('1 张图片上传失败，请重试');
  });

  it('空选择（用户取消 / 没选）不触发上传', async () => {
    const { api, rec } = harness({ maxImages: 9, pick: [] });
    api.onAddImage('album');
    await flush();
    expect(rec.upload).toEqual([]);
    expect(rec.loading.length).toBe(0);
  });

  it('上传中不重入（避免同一批图被上传两遍）', async () => {
    let release = null;
    const gate = new Promise((r) => { release = r; });
    const { api, rec } = harness({ maxImages: 9, pick: ['a.png'], upload: () => gate });
    api.onAddImage('camera');
    await flush();
    api.onAddImage('camera');            // 上传还没结束时再点
    expect(rec.choose.length).toBe(1);
    release('https://e.com/a.png');
    await flush();
    expect(api.images.value.length).toBe(1);
  });
});

// ===== ④ 删除与预览 =====

describe('删除与预览', () => {
  it('删除按位置生效（返回新数组，不原地改）', () => {
    const { api } = harness({ maxImages: 9 });
    api.images.value = ['a', 'b', 'c'];
    api.onRemoveImage(1);
    expect(api.images.value).toEqual(['a', 'c']);
  });

  it('预览把**解析后的 URL** 交给 `uni.previewImage`，并带上当前下标', () => {
    const { api, rec } = harness({ maxImages: 9 });
    api.images.value = ['/uploads/a.png', 'https://e.com/b.png'];
    api.onPreviewImage(1);
    expect(rec.preview.current).toBe(1);
    expect(rec.preview.urls).toEqual(['/uploads/a.png', 'https://e.com/b.png']);
  });
});

// ===== ⑤ 成对取证：注入变异后，上面的判据必须判红 =====

/** 读真源 → 注入变异 → 落到临时目录 → 真执行（不改工作树、不进仓） */
function loadMutated(replacements, opts = {}) {
  let src = readText(PICKER_UTS);
  for (const [from, to] of replacements) {
    expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真的进去了
    src = src.split(from).join(to);
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-picker-'));
  const file = path.join(dir, 'useForumImagePicker.uts');
  fs.writeFileSync(file, src);

  const rec = { choose: [], upload: [], toast: [], loading: [], hideLoading: 0, preview: null };
  const uni = {
    chooseImage: (o) => { rec.choose.push(o); if (opts.pick) o.success({ tempFilePaths: opts.pick }); },
    previewImage: (o) => { rec.preview = o; },
    showToast: (o) => { rec.toast.push(o.title); },
    showLoading: () => {},
    hideLoading: () => {},
  };
  const display = loadUts(DISPLAY_UTS, {});
  const detailDisplay = loadUts(DETAIL_DISPLAY_UTS, { resolveFileUrl: display.resolveFileUrl, uni });
  const m = loadUts(file, {
    ...vueStub(),
    uploadForumImageApi: (p) => { rec.upload.push(p); return Promise.resolve('u-' + p); },
    previewImages: detailDisplay.previewImages,
    resolveFileUrl: display.resolveFileUrl,
    uni,
  });
  return { api: m.useForumImagePicker(opts.maxImages == null ? 9 : opts.maxImages), rec };
}

describe('成对取证（必红）：本套件的判据在坏实现上确实会红', () => {
  it('必不红（对照）：真源上双入口直达、限额闸门两道都在', async () => {
    const { api, rec } = harness({ maxImages: 3 });
    api.onAddImage('camera');
    expect(rec.choose[0].sourceType).toEqual(['camera']);
    api.images.value = ['1', '2', '3'];
    api.onAddImage('album');
    expect(rec.choose.length).toBe(1);
    expect(rec.toast[rec.toast.length - 1]).toBe('最多上传 3 张图片');
  });

  it('必红 · 双入口退化回共用的系统选择器 ⇒ ①「来源直达」判据必然判红', () => {
    const broken = loadMutated([['sourceType: [source]', "sourceType: ['album', 'camera']"]]);
    broken.api.onAddImage('camera');
    expect(broken.rec.choose[0].sourceType).toEqual(['album', 'camera']); // 坏实现真的两个来源都给
  });

  it('必红 · 拆掉上传循环里的限额闸门 ⇒ 并发多选会超限，③「上限就是 maxImages」判据判红', async () => {
    const broken = loadMutated([['if (images.value.length >= maxImages) break\n', '']], {
      maxImages: 3,
      pick: ['a', 'b', 'c', 'd'],
    });
    broken.api.onAddImage('album');
    await flush();
    expect(broken.api.images.value.length).toBe(4);   // 坏实现真让第 4 张进来了
    expect(broken.api.images.value.length).toBeGreaterThan(3); // ⇒ 「上限 = 3」的断言判红
  });
});
