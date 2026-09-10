/**
 * 学习资料页契约测试（#814，refs 首页第 3 格改「学习资料」）
 *
 * 背景：维护者 2026-09-11 裁定首页九宫格第 3 格「考试中心」→「学习资料」，并「一并删掉考试中心页」。
 * 后端 /api/materials 三条早已在产（material.go:25-37 + mobile_p1_contract_test.go），
 * 前端 api/material.uts 写好却零消费——本票补齐页面与两处入口接线。
 *
 * 钉住的契约：
 * 1) 新页在且已注册（pages/resources/materials.uvue + pages.json）
 * 2) 首页九宫格第 3 格 = 学习资料 → 新页；宫格仍 4 格；考试中心入口零残留
 * 3) profile 工具宫格「学习资料」接线（不再 available:false / 空 path）
 * 4) 考试中心页与路由已删（升级式推翻 #810 的「保留入口」裁定）
 * 5) 下载动作在 api 层（页面层零直发请求口径，先例 profileContract / dashboardContract）
 *
 * 自命中防护：断言里必然出现被锁 token（pages/exam/exam、考试中心），故读源码做包含判断，
 * 不做全域扫描；全域零引用由 examDomainRetirementContract 的扫描缝覆盖（排除 *.test.js）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

describe('学习资料入口（#814）', () => {
  const menu = read('pages/dashboard/components/dashboard-menu-grid.uvue');
  const profile = read('pages/profile/profile.uvue');

  it('首页九宫格第 3 格为「学习资料」并指向新页', () => {
    expect(menu).toContain("key: 'materials'");
    expect(menu).toContain("title: '学习资料'");
    expect(menu).toContain("path: '/pages/resources/materials'");
  });

  it('首页九宫格仍 4 格，顺序与原型一致（课程商城 / 题库练习 / 学习资料 / 考情资讯）', () => {
    const keys = [...menu.matchAll(/\{\s*key: '([^']+)'/g)].map((m) => m[1]);
    expect(keys).toEqual(['course-mall', 'question-bank', 'materials', 'exam-info']);
  });

  it('profile 工具宫格「学习资料」已接线（非 available:false 死格子）', () => {
    expect(profile).toContain("title: '学习资料'");
    expect(profile).toContain("path: '/pages/resources/materials'");
    expect(profile).not.toMatch(/key: 'materials'[^}]*available: false/);
  });
});

describe('考试中心页已删（#814 升级式推翻 #810 保留裁定）', () => {
  it('pages/exam/exam.uvue 不存在', () => {
    expect(exists('pages/exam/exam.uvue')).toBe(false);
  });

  it('pages.json 无 pages/exam/exam 路由', () => {
    const routes = [...read('pages.json').matchAll(/"path"\s*:\s*"([^"]+)"/g)].map((m) => m[1]);
    expect(routes).not.toContain('pages/exam/exam');
  });

  it('首页九宫格与 profile 工具宫格均无考试中心残留', () => {
    expect(read('pages/dashboard/components/dashboard-menu-grid.uvue')).not.toContain('考试中心');
    expect(read('pages/profile/profile.uvue')).not.toContain('考试中心');
  });
});

describe('学习资料页实现（pages/resources/materials.uvue）', () => {
  const page = read('pages/resources/materials.uvue');
  const api = read('api/material.uts');

  it('页面已注册且存在', () => {
    expect(exists('pages/resources/materials.uvue')).toBe(true);
    expect(read('pages.json')).toContain('pages/resources/materials');
  });

  it('数据源走 api/material（页面层不 import request）', () => {
    expect(page).toMatch(/from '\.\.\/\.\.\/api\/material'/);
    expect(page).not.toMatch(/from '[^']*api\/request/);
  });

  it('下载动作收在 api 层：页面无 uni.downloadFile / uni.openDocument 裸调', () => {
    expect(page).not.toMatch(/uni\.(download|upload)File\s*\(/);
    expect(page).not.toMatch(/uni\.openDocument\s*\(/);
    expect(api).toContain('export function downloadAndOpenMaterialApi');
    expect(api).toContain('uni.downloadFile(');
    expect(api).toContain('uni.openDocument(');
  });

  it('有失败态与空态两级（不静默吞错）', () => {
    expect(page).toContain('failed');
    expect(page).toContain('资料加载失败');
    expect(page).toContain('暂无学习资料');
  });

  it('分页保护：noMore + 加载中不重入', () => {
    expect(page).toContain('noMore');
    expect(page).toMatch(/if \(loading\.value\) return/);
    expect(page).toContain('onLoadMore');
  });
});
