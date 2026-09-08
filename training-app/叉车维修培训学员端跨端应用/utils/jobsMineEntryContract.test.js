/**
 * 就业在线「招聘 → 我的（单页）」导航链契约（#705 简历入口重挂）
 *
 * 背景：招聘 tab 是纯 mock 已随 #705 退场（#720）。简历入口重挂到就业在线，
 * 用户 2026-09-08 定稿两层结构（对齐五屏原型，两页合一）：
 *   就业在线 profile → job-list（招聘：公告/职位双 tab，头像在 tab 行最右端，
 *   避开小程序胶囊遮挡区）→ 点头像直入 pages/resume/resume（「我的」= 简历总览页）
 *
 * 单页合并说明：曾建的 pages/jobs/jobs-mine 与 resume.uvue 功能重复（后者本就带
 * 资料卡/Lv/收藏统计/去填写简历/常用功能，均为既有真实功能），已删除，头像直跳 resume。
 * 完善度裁定（B「并入真实域」，原型分支 prototype/jobs-resume）：resume 页的
 * 「去填写简历」引导卡升级为完善度卡——由现有 ResumeData 八项字段计算（GET /resume
 * 零后端改动）；人口学 9 项不做（后端无字段）。Lv（学习时长推导）与收藏统计（已接
 * 真实 API、target_type 就绪前恒 0）是 resume 页既有功能，保留。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

describe('就业在线导航链契约（#705 招聘→我的，单页合并版）', () => {
  const list = read('pages/jobs/job-list.uvue');
  const mine = read('pages/resume/resume.uvue');

  it('job-list：公告/职位双 tab；头像在 tab 行最右端直跳 resume（单页）；导航栏右侧留给胶囊', () => {
    expect(list).toContain('<text class="header-title">招聘</text>');
    expect(list).toMatch(/@click="onTabChange\('notice'\)"/);
    expect(list).toMatch(/@click="onTabChange\('job'\)"/);
    expect(list).toMatch(/class="tab-avatar-btn" @click="onMine"/);
    expect(list).toContain("uni.navigateTo({ url: '/pages/resume/resume' })");
    // 导航栏右侧不得挂可点元素（小程序胶囊遮挡区）
    expect(list).not.toMatch(/class="header-right"[^>]*@click/);
    // 公告 tab 空态占位，不放假数据
    expect(list).toContain('暂无公告');
    expect(list).not.toContain('mockJobs');
    // 中间页已删：job-list 不得再引用 jobs-mine
    expect(list).not.toContain('jobs-mine');
  });

  it('中间页 jobs-mine 已删除（文件与路由都不存在，两页合一）', () => {
    expect(fs.existsSync(path.join(ROOT, 'pages/jobs/jobs-mine.uvue'))).toBe(false);
    expect(read('pages.json')).not.toContain('pages/jobs/jobs-mine');
  });

  it('resume（我的）：完善度卡由真实 ResumeData 八项计算，去完善/两功能卡导航齐备', () => {
    expect(mine).toContain('function countResumeFilled');
    expect(mine).toContain('const RESUME_FIELD_TOTAL = 8');
    expect(mine).toMatch(/width: resumePct \+ '%'/);
    expect(mine).toContain('去填写简历');
    expect(mine).toContain('完善度 {{ resumePct }}%');
    for (const f of ['real_name', 'contact_phone', 'wechat', 'region',
      'expected_position_extra', 'salary_negotiable', 'experience_years', 'self_intro']) {
      expect(mine).toContain(f);
    }
    expect(mine).toMatch(/class="resume-guide-btn" @click="onOnlineResume"/);
    expect(mine).toContain("uni.navigateTo({ url: '/pages/resume/resume-edit' })");
    expect(mine).toContain("uni.navigateTo({ url: '/pages/resume/resume-attach' })");
  });

  it('完善度卡加载门控：服务端返回前不展示（防闪烁），未建简历按 0 项呈现', () => {
    expect(mine).toContain('const resumeStatusLoaded = ref<boolean>(false)');
    expect(mine).toMatch(/resumeStatusLoaded\.value && resumeFilledCount\.value < RESUME_FIELD_TOTAL/);
  });

  it('B 定稿：人口学 9 项字段不回潮（Lv/收藏统计为 resume 页既有功能，不在此列）', () => {
    expect(mine).not.toMatch(/出生地区|出生年月|民族|政治面貌|应届生|户籍|生源地|特殊身份/);
  });

  it('简历三页路由均已在 pages.json 注册（无死链）', () => {
    const pagesJson = read('pages.json');
    for (const route of ['pages/resume/resume', 'pages/resume/resume-edit', 'pages/resume/resume-attach']) {
      expect(pagesJson).toContain(`"${route}"`);
    }
  });
});
