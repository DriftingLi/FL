/**
 * mall 试点手术契约测试（TDD: RED → GREEN，refs #640 / refactor epic #638 T02）
 *
 * 钉住试点交付的两类契约（1) 2)；编号是类别不是 describe 数，3) 4) 是「执法点在别处」的口径登记），
 * 采用项目既有源码契约缝（.uvue/.uts 不可被 jest import）：
 * 1) api 收紧：getCourseListApi 经 requestMapped mapper-callback 出口（#639 出口首个真实消费者）
 * 2) 组件接线：mall.uvue 以显式 import 使用三个模块私有组件（Q17 安置规则）
 * 3) 600 软预算 / 目录 ≤2 层：**已由声明面执法**（`utils/modules.js` +
 *    `utils/modulesDeclarationContract.test.js` 的 A3/A5/A10），本文件不再各写一遍（ADR-0023 票 C #1219）
 * 4) mall 域文件机械坑位（catch : any / .detail 直取，即试点验收的「清零」项）：本文件不再写这类锁
 *    ——#654 起豁免面已删，执法点只有全工程守护 `utils/utsAndroidCompile.test.js` 规则 H/I 一处
 */
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

describe('api 收紧契约（getCourseListApi 经 requestMapped 出口家族）', () => {
  const src = read('api/course.uts');
  const req = read('api/request.uts');

  it('request.uts 提供 GET 便捷面 getMapped：委派 requestMapped 且沿用 get() 的手动 query 序列化（不交平台自动序列化，防跨端漂移）', () => {
    const start = req.indexOf('export function getMapped');
    expect(start).toBeGreaterThan(-1);
    const body = req.slice(start, req.indexOf('\n}', start));
    expect(body).toContain('requestMapped<T>(');
    expect(body).toContain('encodeURIComponent');
  });

  it('course.uts 引入 getMapped 出口', () => {
    expect(src).toMatch(/import\s*\{[^}]*getMapped[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('getCourseListApi 函数体经 getMapped 且映射函数为 buildCourseListResult', () => {
    const start = src.indexOf('export function getCourseListApi');
    expect(start).toBeGreaterThan(-1);
    const body = src.slice(start, src.indexOf('\n}', start));
    expect(body).toContain('getMapped<CourseListResult>');
    expect(body).toContain('buildCourseListResult');
    // mock 降级行为保持（UI 冻结：无网络时仍出 mock 数据）
    expect(body).toContain('getMockCourseList()');
  });
});

describe('模块私有组件接线契约（Q17 安置：pages/<module>/components/ 显式 import）', () => {
  const page = read('pages/mall/mall.uvue');
  const COMPONENTS = ['mall-sort-bar', 'mall-category-sidebar', 'mall-float-actions'];

  it.each(COMPONENTS)('组件文件存在于 pages/mall/components/%s.uvue', (name) => {
    expect(h.exists(`pages/mall/components/${name}.uvue`)).toBe(true);
  });

  it.each(COMPONENTS)('mall.uvue 显式 import 组件 %s', (name) => {
    expect(page).toContain(`./components/${name}.uvue`);
  });

  it('主页面模板实际使用三个组件标签（非只 import 不用；PascalCase 先例同 AiChatNav）', () => {
    expect(page).toMatch(/<MallSortBar[\s/>]/);
    expect(page).toMatch(/<MallCategorySidebar[\s/>]/);
    expect(page).toMatch(/<MallFloatActions[\s/>]/);
  });
});
