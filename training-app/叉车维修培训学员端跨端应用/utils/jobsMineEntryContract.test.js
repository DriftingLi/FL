/**
 * 就业在线「招聘 → 我的 → 简历」导航链契约（#705 简历入口重挂）
 *
 * 背景：招聘 tab 是纯 mock 已随 #705 退场（#720），/pages/resume/* 三页入口
 * 重挂到就业在线。用户 2026-09-08 定稿三层结构（对齐五屏原型「招聘-我的」屏）：
 *   就业在线 profile → job-list（招聘，顶栏右上头像）→ jobs-mine（我的）→ 简历三页
 * 形态裁定：三变体原型（prototype/jobs-resume 分支）拍板 B「并入真实域」——
 * 完善度直接由现有 ResumeData 八项字段计算（GET /resume 零后端改动），
 * 原型图的人口学 9 项/Lv/收藏统计不做（收藏 API 的 target_type 后端未收录、恒 0，
 * 接线先例见 resume.uvue:174-184，接口就绪前不展示）。
 *
 * 本文件钉住：
 * 1) job-list 顶栏头像 → jobs-mine（简历入口不再嵌在职位列表顶部）；
 * 2) jobs-mine 三页可达 + 路由均已注册（无死链）；
 * 3) jobs-mine 资料卡真实登录态 + 完善度由真实 ResumeData 八项计算；
 * 4) 人口学 9 项 / Lv / 收藏统计不回潮。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

describe('就业在线导航链契约（#705 招聘→我的→简历）', () => {
  const list = read('pages/jobs/job-list.uvue');
  const mine = read('pages/jobs/jobs-mine.uvue');

  it('job-list：顶栏标题「招聘」+ 右上头像跳 jobs-mine，不再内嵌简历入口区', () => {
    expect(list).toContain('<text class="header-title">招聘</text>');
    expect(list).toMatch(/class="header-right" @click="onMine"/);
    expect(list).toContain("uni.navigateTo({ url: '/pages/jobs/jobs-mine' })");
    // 入口区已搬离 job-list：不应再出现完善度卡/去填写简历大卡
    expect(list).not.toContain('去填写简历');
    expect(list).not.toContain('countResumeFilled');
    expect(list).not.toContain('mine-resume-card');
  });

  it('jobs-mine：简历三页导航齐备（资料卡/去完善→总览、在线→编辑、附件→附件）', () => {
    expect(mine).toContain("uni.navigateTo({ url: '/pages/resume/resume' })");
    expect(mine).toContain("uni.navigateTo({ url: '/pages/resume/resume-edit' })");
    expect(mine).toContain("uni.navigateTo({ url: '/pages/resume/resume-attach' })");
    for (const fn of ['onResume', 'onResumeEdit', 'onResumeAttach']) {
      expect(mine).toMatch(new RegExp(`@click(\\.stop)?="${fn}"`));
    }
  });

  it('目标路由均已在 pages.json 注册（无死链）', () => {
    const pagesJson = read('pages.json');
    for (const route of [
      'pages/jobs/jobs-mine',
      'pages/resume/resume', 'pages/resume/resume-edit', 'pages/resume/resume-attach',
    ]) {
      expect(pagesJson).toContain(`"${route}"`);
    }
  });

  it('jobs-mine 版式对齐原型：资料卡 + 居中「去填写简历」大卡 + 常用功能两卡', () => {
    expect(mine).toContain('<text class="mine-resume-icon">📄</text>');
    expect(mine).toContain('去填写简历');
    expect(mine).toContain('完善度 {{ resumePct }}%');
    expect(mine).toContain('<text class="mine-sec-title">常用功能</text>');
    expect(mine).toMatch(/class="mine-resume-btn" @click\.stop="onResume"/);
    // 资料卡取真实登录态（authStore computed）
    expect(mine).toContain("from '../../stores/auth'");
    expect(mine).toContain('const avatarUrl = computed<string>');
    expect(mine).toContain('const displayName = computed<string>');
  });

  it('B 定稿：完善度由真实 ResumeData 八项计算，人口学/Lv/收藏统计不回潮', () => {
    expect(mine).toContain("from '../../api/resume'");
    expect(mine).toContain('function countResumeFilled');
    expect(mine).toContain('const RESUME_FIELD_TOTAL = 8');
    expect(mine).toMatch(/width: resumePct \+ '%'/);
    // 八项字段名必须全部出现在计算函数里（真实域，非新造字段）
    for (const f of ['real_name', 'contact_phone', 'wechat', 'region',
      'expected_position_extra', 'salary_negotiable', 'experience_years', 'self_intro']) {
      expect(mine).toContain(f);
    }
    // 人口学 9 项 / Lv / 收藏统计：B 裁定不做
    expect(mine).not.toMatch(/出生|民族|政治面貌|应届生|户籍|生源地/);
    expect(mine).not.toMatch(/Lv\d|Lv\$|Lv\{|Lv3/);
    expect(mine).not.toMatch(/公告收藏|职位收藏|合集收藏/);
  });
});
