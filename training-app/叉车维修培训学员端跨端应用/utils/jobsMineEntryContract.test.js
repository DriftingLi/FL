/**
 * 就业在线「我的简历」入口契约（#705 简历入口重挂）
 *
 * 背景：招聘 tab 是纯 mock 已随 #705 退场（#720），/pages/resume/* 三页唯一入口
 * 重挂到 job-list（设计 docs/prototype/recruit-mine-design.png）。
 * 本文件钉住重挂的三条语义：
 * 1) 三页均可达且路由均已注册（无死链）；
 * 2) 资料卡用真实登录态（authStore），不引入收藏数等 mock（设计图统计行因
 *    后端无职位收藏 API 而裁剪，禁止回填假数据）；
 * 3) 入口区在职位列表之上，滚动不遮挡列表操作。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

describe('就业在线简历入口契约（#705 退场后重挂）', () => {
  const src = read('pages/jobs/job-list.uvue');

  it('简历三页导航齐备：资料卡→总览、在线简历→编辑、附件简历→附件', () => {
    expect(src).toContain("uni.navigateTo({ url: '/pages/resume/resume' })");
    expect(src).toContain("uni.navigateTo({ url: '/pages/resume/resume-edit' })");
    expect(src).toContain("uni.navigateTo({ url: '/pages/resume/resume-attach' })");
    for (const fn of ['onResume', 'onResumeEdit', 'onResumeAttach']) {
      expect(src).toMatch(new RegExp(`@click="${fn}"`));
    }
  });

  it('目标路由均已在 pages.json 注册（无死链）', () => {
    const pagesJson = read('pages.json');
    for (const route of ['pages/resume/resume', 'pages/resume/resume-edit', 'pages/resume/resume-attach']) {
      expect(pagesJson).toContain(`"${route}"`);
    }
  });

  it('资料卡取真实登录态（authStore computed），无 mock 统计回填', () => {
    expect(src).toContain("from '../../stores/auth'");
    expect(src).toContain('const avatarUrl = computed<string>');
    expect(src).toContain('const displayName = computed<string>');
    expect(src).not.toMatch(/公告收藏|职位收藏|合集收藏/);
    expect(src).not.toContain('mock');
  });

  it('入口区置于职位列表之上（滚动区首块）', () => {
    expect(src).toMatch(/<scroll-view[^>]*>[\s\S]*?class="mine-section"[\s\S]*?class="job-list"/);
  });
});
