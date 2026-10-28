/**
 * 职位「需持证件」接线守护（#1945 / spec #1935 草稿 19）
 *
 * 分类：**接线（wiring）** —— 它防的是「被拆掉 / 被改成第二份实现」，不构成 ③ 门的行为证据
 * （行为证据在 `jobCredentialDisplayBehavior.test.js`，真跑 `utils/jobCredentialDisplay.uts`）。
 *
 * 守的四条接线：
 *   ① 卡面那一行文案只能来自单点 `jobCredentialNotice`，且**无要求时不渲染**（v-if 判长度）；
 *   ② 证件筛选组的选项只能来自 `buildCredentialFilterOptions`（不许页面手写证件清单——那就是第二份字典）；
 *   ③ 选中证件必须经 `credentialFilterParam` 折成查询参数（0/非法值不发出去），并落进 `credential_id`；
 *   ④ DTO 字段与参数名两端同名：`required_credentials` / `required_credential_ids` / `credential_id`。
 */
const path = require('path');

// readText 走读取层归一（CRLF → LF，ADR-0019）：裸 readFileSync 在 Windows 检出下会让
// 断言在 CI 与本地判出两种结果（仓内守卫 scripts/check-contract-read.mjs 判红）
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');

const read = (rel) => readText(path.join(ROOT, rel));

const PAGE = 'pages/jobs/job-list.uvue';
const API = 'api/job.uts';
const TYPES = 'types/job.uts';

describe('职位卡「需持 X 证」：文案单点接线', () => {
  it('页面从 utils/jobCredentialDisplay 引入单点（不是页面内联拼字符串）', () => {
    const src = read(PAGE);
    expect(src).toMatch(/from ['"]\.\.\/\.\.\/utils\/jobCredentialDisplay['"]/);
    expect(src).toMatch(/\bjobCredentialNotice\b/);
  });

  it('卡面渲染受「非空」守卫：无证件要求时不出现空标签', () => {
    const src = read(PAGE);
    const guarded = /v-if="[^"]*credentialNotice[^"]*\.length\s*>\s*0[^"]*"/.test(src)
      || /v-if="[^"]*credentialNoticeOf\([^"]*\)\.length\s*>\s*0[^"]*"/.test(src);
    expect(guarded).toBe(true);
  });
});

describe('证件筛选组：选项与参数接线', () => {
  it('筛选组选项来自 buildCredentialFilterOptions（禁止页面手写证件清单）', () => {
    const src = read(PAGE);
    expect(src).toMatch(/\bbuildCredentialFilterOptions\b/);
    expect(src).not.toMatch(/『?全部证件』?\s*,\s*[^\n]*'叉车司机/);
  });

  it('选中证件经 credentialFilterParam 折算后写进 credential_id', () => {
    const src = read(PAGE);
    expect(src).toMatch(/\bcredentialFilterParam\b/);
    expect(src).toMatch(/credential_id\s*:/);
  });

  it('页面不再用假数据填充证件：源码里没有 mock 证件清单', () => {
    const src = read(PAGE);
    expect(src).not.toMatch(/mockCredential|fakeCredential|演示数据/);
  });
});

describe('api / types：参数与 DTO 字段两端同名', () => {
  it('列表出口只在 > 0 时拼 credential_id（不把 0/负数发给后端换 400）', () => {
    const src = read(API);
    expect(src).toMatch(/query\['credential_id'\]/);
    expect(src).toMatch(/credential_id!\s*>\s*0/);
  });

  it('DTO 映射读出 required_credential_ids 与 required_credentials', () => {
    const src = read(API);
    expect(src).toMatch(/required_credential_ids/);
    expect(src).toMatch(/required_credentials/);
    expect(src).toMatch(/credential_id/);
  });

  it('类型层声明同名字段（改字段名即红，防两端漂移）', () => {
    const src = read(TYPES);
    expect(src).toMatch(/required_credential_ids\s*:\s*number\[\]/);
    expect(src).toMatch(/required_credentials\s*:\s*JobRequiredCredential\[\]/);
    expect(src).toMatch(/credential_id\s*\?:\s*number/);
  });
});
