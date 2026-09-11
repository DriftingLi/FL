/**
 * 资源投稿错误可见性契约测试（#816）
 *
 * 背景：真机实测「填好标题/简介 + 选好文件 + 证件已选 → 点『提交上传』无任何反馈」。
 * 三道前端前置校验都过（文件列表有条目、证件名已显示），失败在网络层——而
 * forum-contribution-form 的 catch 只写 console，界面零反馈；「我的上传」把加载失败
 * 也显示成「暂无上传资源」。本票只做「错误可见 + 取证日志」，不改后端与上传逻辑。
 *
 * 钉住的契约：
 * 1) 组件失败必有页内可见文案（持久 errorText，而非一闪而过的 toast）
 * 2) 失败日志带得出「哪个阶段失败」（上传文件 N/M / 提交投稿）
 * 3) my-uploads 区分「加载失败」与「暂无数据」，失败给重试位
 * 4) 上传/建档请求的超时与端点不被顺手改动（回归锁）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

describe('#816 投稿表单失败必须可见', () => {
  const src = read('pages/forum/components/forum-contribution-form.uvue');

  it('声明页内错误位 errorText 并在模板渲染', () => {
    expect(src).toMatch(/const errorText = ref<string>\(''\)/);
    expect(src).toMatch(/v-if="errorText\.length > 0"/);
    expect(src).toContain('submit-error-text');
  });

  it('提交前清空、失败时写入错误位（不是只 console）', () => {
    expect(src).toMatch(/errorText\.value = ''/);
    expect(src).toMatch(/errorText\.value = stage \+ '失败：' \+ errMsg\(e,/);
  });

  it('失败日志带阶段标识，供真机取证', () => {
    expect(src).toContain('submit failed at');
    expect(src).toMatch(/let stage = '上传文件'/);
    expect(src).toMatch(/stage = '提交投稿'/);
  });

  it('错误文案取自 api/helpers 的 errMsg（非裸字符串）', () => {
    expect(src).toMatch(/import \{ errMsg \} from '\.\.\/\.\.\/\.\.\/api\/helpers'/);
  });

  it('四条前端前置校验仍在（本票不动校验语义）', () => {
    for (const t of ['请输入资源标题', '请输入资源简介', '请先选定证件后再投稿', '请至少选择 1 个文件']) {
      expect(src).toContain(t);
    }
  });
});

describe('#816 我的上传区分失败与空态', () => {
  const src = read('pages/resources/my-uploads.uvue');

  it('声明 failed 标记并在 catch 置位', () => {
    expect(src).toMatch(/const failed = ref<boolean>\(false\)/);
    expect(src).toMatch(/failed\.value = true/);
  });

  it('失败态给重试位，空态才是「暂无上传资源」', () => {
    expect(src).toContain('加载失败，请检查网络后重试');
    expect(src).toMatch(/uploads\.length == 0 && !loading && failed/);
    expect(src).toMatch(/uploads\.length == 0 && !loading && !failed/);
    expect(src).toContain('暂无上传资源');
  });

  it('reload 复位失败标记后重取', () => {
    const body = src.slice(src.indexOf('function reload()'), src.indexOf('function reload()') + 220);
    expect(body).toContain('failed.value = false');
    expect(body).toContain('loadUploads()');
  });
});

describe('#816 上传链路回归锁（本票不改）', () => {
  const api = read('api/contribution.uts');

  it('上传端点与 120s 超时未变', () => {
    expect(api).toContain("url: '/contributions/upload-file'");
    expect(api).toMatch(/timeout: 120000/);
  });

  it('上传字段与建档载荷未变', () => {
    for (const k of ['file_url', 'file_name', 'file_size', 'content_type']) {
      expect(api).toContain(k);
    }
    expect(api).toContain("postMapped<ContributionItem>('/contributions'");
  });
});
