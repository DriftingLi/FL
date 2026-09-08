/**
 * 就业在线「我的简历」入口契约（#705 简历入口重挂）
 *
 * 背景：招聘 tab 是纯 mock 已随 #705 退场（#720），/pages/resume/* 三页唯一入口
 * 重挂到 job-list（设计 docs/prototype/recruit-mine-design.png）。
 * 形态裁定：三变体原型（prototype/jobs-resume 分支）拍板 B「并入真实域」——
 * 完善度直接由现有 ResumeData 八项字段计算（GET /resume 零后端改动），
 * 原型图的人口学 9 项/Lv/收藏统计不做（收藏 API 的 target_type 后端未收录、恒 0，
 * 接线先例见 resume.uvue:174-184，接口就绪前不展示）。
 * 本文件钉住四条语义：
 * 1) 三页均可达且路由均已注册（无死链）；
 * 2) 资料卡用真实登录态（authStore），不引入收藏数等假数据；
 * 3) 入口区在职位列表之上，滚动不遮挡列表操作；
 * 4) B 定稿：完善度由真实字段计算，人口学/Lv/收藏统计不得回潮。
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

  it('B 定稿：完善度由真实 ResumeData 八项计算，人口学/Lv/收藏统计不回潮', () => {
    expect(src).toContain("from '../../api/resume'");
    expect(src).toContain('function countResumeFilled');
    expect(src).toContain('const RESUME_FIELD_TOTAL = 8');
    expect(src).toMatch(/width: resumePct \+ '%'/);
    // 八项字段名必须全部出现在计算函数里（真实域，非新造字段）
    for (const f of ['real_name', 'contact_phone', 'wechat', 'region',
      'expected_position_extra', 'salary_negotiable', 'experience_years', 'self_intro']) {
      expect(src).toContain(f);
    }
    // 人口学 9 项 / Lv / 收藏统计：B 裁定不做
    expect(src).not.toMatch(/出生|民族|政治面貌|应届生|户籍|生源地/);
    expect(src).not.toMatch(/Lv\d|Lv\$|Lv\{|Lv3/);
    expect(src).not.toMatch(/公告收藏|职位收藏|合集收藏/);
  });
});
