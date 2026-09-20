/**
 * 招聘者简历库 / 简历详情 —— **接线层**契约测试（ADR-0022 P3 = #1196）
 *
 * 与 `utils/recruiterResumeBehavior.test.js` 的分工：那个文件真跑 `api/recruit.uts`，
 * 断言请求形状与「下载后交系统打开」的行为；本文件断言**源码结构与字段边界** ——
 * 行为测试自己搭依赖，接线断了它照样可能绿。
 *
 * 四组守护，对应票面的三条判据：
 *   A. **脱敏字段清单与真源逐项对齐**：真源 = `backend/internal/service/resume_projection.go`
 *      的 `desensitize()`（ADR-0053 §4 / spec #1051）。**不是硬编码清单** ——
 *      本测试现读那个文件，把它的返回字段与移动端模型/映射/页面逐项对账：
 *      两端各自增一个字段都会被判红（票面要求「不得增删」）。
 *   B. **未授权态不渲染敏感面**：详情页源码里不得出现电话 / 微信 / 现居地 / 上传 PDF /
 *      工作照 / 证件原图的渲染锚点 —— 那六项只在 approved 后经 `/contact` 的 6 键取得。
 *   C. **已授权态**：真实姓名 / 电话 / 微信原生渲染；工作照可点开；上传 PDF 与打码 PDF
 *      走同一个「下载后交系统打开」的出路。
 *   D. **筛选抽屉 = 后端已支持的 8 维**（不多不少），列表「加载更多」按 page_size=20。
 *
 * 真源读取假设（fail-closed）：`backend/internal/service/resume_projection.go` 必须
 * 在**本分支的工作树里**且与 `origin/master` 逐字节一致（本票分支从 P1 切出时已带上）。
 * 读不到或读不出 `desensitize` 的返回块 ⇒ 直接判红，不静默跳过（静默跳过等于假的绿）。
 */
const fs = require('fs');
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const REPO_ROOT = path.join(ROOT, '..', '..');

const RECRUIT_UTS = path.join(ROOT, 'api', 'recruit.uts');
const REQUEST_UTS = path.join(ROOT, 'api', 'request.uts');
/**
 * ⚠️ 收口（#1195 + #1196 叠加）后，简历库**一级面**的唯一页面是 P2 建的
 * `pages/recruiter/resumes.uvue`（P3 原建的 `resume-library.uvue` 已并入它并删除）——
 * 本常量指向那个合并后的面。**判据一字未改**，改的只是「正文住在哪个文件里」这一层。
 */
const LIBRARY_UVUE = path.join(ROOT, 'pages', 'recruiter', 'resumes.uvue');
const DETAIL_UVUE = path.join(ROOT, 'pages', 'recruiter', 'resume-detail.uvue');
const DRAWER_UVUE = path.join(ROOT, 'pages', 'recruiter', 'components', 'recruiter-filter-drawer.uvue');
const PROJECTION_GO = path.join(REPO_ROOT, 'backend', 'internal', 'service', 'resume_projection.go');

const recruitSrc = readText(RECRUIT_UTS);
const requestSrc = readText(REQUEST_UTS);
const librarySrc = readText(LIBRARY_UVUE);
const detailSrc = readText(DETAIL_UVUE);
const drawerSrc = readText(DRAWER_UVUE);

/**
 * 真源：从 `desensitize()` 的 `return RecruitResumeCard{…}` 块里取 Go 字段名。
 * @returns {string[]} 形如 ['UserID','RealName',…]
 */
function desensitizeFieldsFromTruth() {
  if (!fs.existsSync(PROJECTION_GO)) {
    throw new Error(
      `真源不在此工作树：${PROJECTION_GO}\n`
      + '本票的字段清单必须与 resume_projection.go 逐项对账，读不到就不判（fail-closed，不静默跳过）。'
    );
  }
  const src = readText(PROJECTION_GO);
  const start = src.indexOf('func desensitize(');
  if (start === -1) throw new Error('resume_projection.go 里找不到 desensitize()');
  const retIdx = src.indexOf('return RecruitResumeCard{', start);
  if (retIdx === -1) throw new Error('desensitize() 里找不到 return RecruitResumeCard{');
  const end = src.indexOf('\n\t}', retIdx);
  if (end === -1) throw new Error('desensitize() 的返回块没找到结尾');
  const block = src.slice(retIdx, end);
  const fields = [];
  block.split('\n').forEach((line) => {
    const m = line.match(/^\s*([A-Z][A-Za-z0-9]*)\s*:/);
    if (m) fields.push(m[1]);
  });
  if (fields.length === 0) throw new Error('desensitize() 的返回块解析不出字段');
  return fields;
}

/** Go 字段名 → 后端 JSON 键（脱敏卡响应的键名，也是移动端读取的键名） */
const GO_FIELD_TO_JSON = {
  UserID: 'user_id',
  RealName: 'real_name',
  RealNameMasked: 'real_name_masked',
  ExpectedPositionID: 'expected_position_id',
  ExpectedPositionExtra: 'expected_position_extra',
  ExpectedRegions: 'expected_regions',
  SalaryMin: 'salary_min',
  SalaryMax: 'salary_max',
  SalaryNegotiable: 'salary_negotiable',
  AvailableIn: 'available_in',
  JobNature: 'job_nature',
  ExperienceYears: 'experience_years',
  SelfIntro: 'self_intro',
  ResumeExperiences: 'resume_experiences',
  ResumeCertifications: 'resume_certifications',
  UpdatedAt: 'updated_at',
};

/** 真源刻意**不含**的敏感面（Go 字段名）。出现即判失败 —— 票面判据一。 */
const FORBIDDEN_IN_DESENSITIZE = [
  'ContactPhone', 'Wechat', 'Region', 'ResumeFileURL', 'Photos', 'Visibility', 'CreatedAt',
];

describe('A. 脱敏字段清单与真源 resume_projection.go 逐项对账', () => {
  test('A1：真源可读，且 desensitize() 的字段集与票面 11 项口径一致', () => {
    const fields = desensitizeFieldsFromTruth();
    // 真源当前的实际字段（16 个 Go 字段 / 14 个业务面 + 2 个姓名别名 = 票面 11 项清单）
    expect(fields).toEqual([
      'UserID', 'RealName', 'RealNameMasked',
      'ExpectedPositionID', 'ExpectedPositionExtra', 'ExpectedRegions',
      'SalaryMin', 'SalaryMax', 'SalaryNegotiable',
      'AvailableIn', 'JobNature', 'ExperienceYears', 'SelfIntro',
      'ResumeExperiences', 'ResumeCertifications', 'UpdatedAt',
    ]);
  });

  test('A2：真源里**没有**任何敏感面字段（出现电话/微信/现居地/上传PDF/工作照/可见性即判失败）', () => {
    const fields = desensitizeFieldsFromTruth();
    const leaked = fields.filter((f) => FORBIDDEN_IN_DESENSITIZE.includes(f));
    expect(leaked).toEqual([]);
  });

  test('A3：真源字段 → 移动端 TypeScript 类型逐项齐备', () => {
    const block = recruitSrc.slice(
      recruitSrc.indexOf('export type RecruitResumeCard = {'),
      recruitSrc.indexOf('}', recruitSrc.indexOf('export type RecruitResumeCard = {')),
    );
    const missing = [];
    desensitizeFieldsFromTruth().forEach((goField) => {
      const jsonKey = GO_FIELD_TO_JSON[goField];
      if (!jsonKey) { missing.push(`${goField}(无 JSON 键映射)`); return; }
      if (!new RegExp(`\\b${jsonKey}\\s*:`).test(block)) missing.push(`${goField}→${jsonKey}`);
    });
    expect(missing).toEqual([]);
  });

  test('A4：真源字段 → 移动端映射函数 `buildRecruitResumeCard` 逐项读取（字段边界在此收敛）', () => {
    const start = recruitSrc.indexOf('export function buildRecruitResumeCard(');
    expect(start).toBeGreaterThan(-1);
    const body = recruitSrc.slice(start, recruitSrc.indexOf('\n}', start));
    const missing = [];
    desensitizeFieldsFromTruth().forEach((goField) => {
      const jsonKey = GO_FIELD_TO_JSON[goField];
      if (!jsonKey) { missing.push(`${goField}(无 JSON 键映射)`); return; }
      if (!body.includes(`obj['${jsonKey}']`)) missing.push(`${goField}→${jsonKey}`);
    });
    expect(missing).toEqual([]);
  });

  test('A5：移动端**不得读取**真源清单外的敏感键（即便后端误混进响应也读不出）', () => {
    // 只查脱敏卡映射与列表/详情页面：明文面（contact）本来就要读这些键，另行守护
    const desensitizeReads = ['contact_phone', 'wechat', 'resume_file_url', 'photos', 'image_urls', 'imageUrls'];
    const cardMapper = recruitSrc.slice(
      recruitSrc.indexOf('export function buildRecruitResumeCard('),
      recruitSrc.indexOf('\n}', recruitSrc.indexOf('export function buildRecruitResumeCard(')),
    );
    desensitizeReads.forEach((key) => {
      expect(cardMapper).not.toContain(`obj['${key}']`);
    });
    // `region`（现居地）与脱敏卡无关：卡片模型里不得有承接它的字段
    const cardType = recruitSrc.slice(
      recruitSrc.indexOf('export type RecruitResumeCard = {'),
      recruitSrc.indexOf('}', recruitSrc.indexOf('export type RecruitResumeCard = {')),
    );
    expect(cardType).not.toMatch(/\bregion\s*:/);
    expect(cardType).not.toMatch(/\bcontact_phone\s*:/);
    expect(cardType).not.toMatch(/\bwechat\s*:/);
  });
});

describe('B. 未授权态：只渲染真源清单内字段 + 给打码 PDF 次要出口', () => {
  test('B1：详情页的「联系方式」段在 contact 为空时只有「发起交换」，没有任何明文锚点', () => {
    const start = detailSrc.indexOf('<text class="section-title">联系方式</text>');
    expect(start).toBeGreaterThan(-1);
    const end = detailSrc.indexOf('<view class="card">', start);
    const section = detailSrc.slice(start, end === -1 ? detailSrc.length : end);
    // 未授权分支存在
    expect(section).toContain('v-if="!contactLoading && contact == null"');
    expect(section).toContain('发起交换');
    // 未授权分支里没有明文渲染（明文整段挂在 `contact != null` 之下）
    const unauthorizedBlock = section.slice(0, section.indexOf('contact != null'));
    expect(unauthorizedBlock).not.toContain('plainPhone');
    expect(unauthorizedBlock).not.toContain('plainWechat');
    expect(unauthorizedBlock).not.toContain('plainName');
    expect(unauthorizedBlock).not.toContain('resumeFileUrl');
    expect(unauthorizedBlock).not.toContain('photos');
  });

  test('B2：明文只在 `contact != null` 分支内取值（computed 在 contact 为空时恒空串）', () => {
    ['plainName', 'plainPhone', 'plainWechat', 'photos', 'plainCerts', 'resumeFileUrl'].forEach((name) => {
      const start = detailSrc.indexOf(`const ${name} = computed<`);
      expect(start).toBeGreaterThan(-1);
      const body = detailSrc.slice(start, detailSrc.indexOf('})', start));
      // 每个明文 computed 必须先判 contact == null 并提前返回空值
      expect(body).toContain('if (c == null) return');
    });
  });

  test('B3：脱敏卡的持证**已去图** —— 只有编号/有效期，没有证件原图', () => {
    // 卡片模型上无图片字段
    const certType = recruitSrc.slice(
      recruitSrc.indexOf('export type RecruitCertification = {'),
      recruitSrc.indexOf('}', recruitSrc.indexOf('export type RecruitCertification = {')),
    );
    expect(certType).not.toMatch(/image/i);
    // 未授权面的模板只用编号/有效期
    const maskedCertUse = detailSrc.slice(
      detailSrc.indexOf('v-for="(cert, idx) in certifications"'),
      detailSrc.indexOf('</view>', detailSrc.indexOf('v-for="(cert, idx) in certifications"')),
    );
    expect(maskedCertUse).toContain('cert.cert_no');
    expect(maskedCertUse).toContain('cert.expire_date');
    expect(maskedCertUse).not.toContain('imagesOf');
  });

  test('B4：打码 PDF 是**次要出口**且不要求授权（按钮在联系方式段之外，未授权也可点）', () => {
    expect(detailSrc).toContain('downloadAndOpenMaskedResumePdfApi(userId.value)');
    // 上传 PDF 的按钮以 resumeFileUrl 非空为前提（未授权时该值为空串 ⇒ 不渲染）
    expect(detailSrc).toMatch(/v-if="resumeFileUrl\.length > 0"/);
    expect(recruitSrc).toContain("'/recruit/resumes/' + userId.toString() + '/pdf'");
  });
});

describe('C. 已授权态：6 键明文 + 工作照可点开 + 同一个 PDF 出路', () => {
  test('C1：明文 6 键在 api 层逐键映射，且**恰好**这 6 个键', () => {
    const start = recruitSrc.indexOf('export function getRecruitResumeContactApi(');
    const body = recruitSrc.slice(start, recruitSrc.indexOf('\n}', start));
    ['real_name', 'contact_phone', 'wechat', 'resume_file_url', 'photos', 'resume_certifications'].forEach((k) => {
      expect(body).toContain(`data['${k}']`);
    });
    const readKeys = [...body.matchAll(/data\['([a-z_]+)'\]/g)].map((m) => m[1]);
    expect([...new Set(readKeys)].sort()).toEqual([
      'contact_phone', 'photos', 'real_name', 'resume_certifications', 'resume_file_url', 'wechat',
    ]);
  });

  test('C2：详情页把 6 键逐一渲染（真实姓名 / 电话 / 微信 / 工作照 / 证件 / 上传 PDF）', () => {
    expect(detailSrc).toContain('{{ emptyAsDash(plainName) }}');
    expect(detailSrc).toContain('{{ emptyAsDash(plainPhone) }}');
    expect(detailSrc).toContain('{{ emptyAsDash(plainWechat) }}');
    expect(detailSrc).toContain('v-if="photos.length > 0"');
    expect(detailSrc).toContain('v-if="plainCerts.length > 0"');
    expect(detailSrc).toContain('onPreviewPhoto(');
    expect(detailSrc).toContain('onDownloadUploadedPdf');
  });

  test('C3：上传 PDF 与打码 PDF 走同一个出口（同一个 api 函数家族）', () => {
    // 打码端点
    expect(detailSrc).toContain('downloadAndOpenMaskedResumePdfApi');
    // 上传 PDF 也走带鉴权的下载出口，而不是另起一套
    expect(detailSrc).toContain('downloadAndOpenAuthedFile(path,');
    // 页面层不得裸调 uni.downloadFile（先例：资料下载收在 api 层）
    expect(detailSrc).not.toContain('uni.downloadFile');
    expect(detailSrc).not.toContain('uni.openDocument');
  });

  test('C4：403 被当成「未授权」而不是失败 —— 不弹错误、不清登录态、不跳登录页', () => {
    expect(detailSrc).toContain('isContactForbidden(e)');
    // 详情页只有一处 reLaunch，且是招聘者身份守卫（不是 403 分支）
    const relaunches = [...detailSrc.matchAll(/uni\.reLaunch\(/g)];
    expect(relaunches.length).toBe(1);
    // 取 onLoad 钩子本体（它已按 UTS 无提升的要求挪到文件末）
    const guard = detailSrc.slice(detailSrc.indexOf('onLoad((options : OnLoadOptions)'));
    expect(guard).toContain("uni.reLaunch({ url: '/pages/recruiter/login' })");
    expect(guard).toContain('isRecruiterActive()');
    // 403 分支里不出现导航
    const contactFn = detailSrc.slice(
      detailSrc.indexOf('async function loadContact'),
      detailSrc.indexOf('async function loadDetail'),
    );
    expect(contactFn).not.toContain('reLaunch');
    expect(contactFn).not.toContain('navigateTo');
  });
});

describe('D. 筛选抽屉 = 后端 8 维（不多不少）+ 列表加载更多 page_size=20', () => {
  test('D1：抽屉渲染 8 个筛选维度，恰好对应后端 api/recruit.go:60-88 的参数名', () => {
    // 每个维度一个输入/一组 chip
    expect(drawerSrc).toContain('onRegionInput');       // region
    expect(drawerSrc).toContain('onPositionIdInput');   // position_id
    expect(drawerSrc).toContain('onCredentialIdInput'); // credential_id
    expect(drawerSrc).toContain('onSalaryMinInput');    // salary_min
    expect(drawerSrc).toContain('onSalaryMaxInput');    // salary_max
    expect(drawerSrc).toContain('onExperienceMinInput');// experience_min
    expect(drawerSrc).toContain('onJobNatureTap');      // job_nature
    expect(drawerSrc).toContain('onAvailableInTap');    // available_in
    // 8 个键在 emit 的对象里逐个落位
    const apply = drawerSrc.slice(drawerSrc.indexOf('function onApply'), drawerSrc.indexOf('function onClose'));
    ['region', 'position_id', 'credential_id', 'salary_min', 'salary_max', 'experience_min', 'job_nature', 'available_in']
      .forEach((k) => expect(apply).toContain(`${k}:`));
  });

  test('D2：本轮不用的两维（experience_years / experience_max）不得出现在抽屉与 api 的参数面', () => {
    // 注意匹配边界用 `(?![\w])` 而不是 `\b`：`_` 属于 `\w`，`\bexperience_years\b`
    // 在 `experience_years_x` 上判不出边界；同时用「非标识符字符」收尾，
    // 这样注释里写 `experience_years`（说明「本轮不用」）不会把合法的注释误伤 ——
    // 真正的判据是它们**不作为参数键**出现（下面 filterBuilder 那两条）。
    [drawerSrc].forEach((src) => {
      expect(src).not.toMatch(/experience_years(?![\w])/);
      expect(src).not.toMatch(/experience_max(?![\w])/);
    });
    const filterBuilder = recruitSrc.slice(
      recruitSrc.indexOf('function buildFilterParams('),
      recruitSrc.indexOf('/** 空筛选'),
    );
    expect(filterBuilder).not.toMatch(/experience_years(?![\w])/);
    expect(filterBuilder).not.toMatch(/experience_max(?![\w])/);
  });

  test('D3：region 未做本地归一（不拼省、不补「市」）—— 归一交给后端 RegionCityName', () => {
    expect(drawerSrc).toContain('region: region.value.trim()');
    expect(drawerSrc).not.toContain("+ '/'");
    expect(recruitSrc).toContain('RegionCityName');
    // 移动端不得自带一份「市名 → 苏州市」的映射表
    expect(recruitSrc).not.toMatch(/苏州市|浙江省|江苏省/);
  });

  test('D4：列表「加载更多」按 page_size=20（RECRUIT_PAGE_SIZE 单点），且页码由请求侧自记', () => {
    expect(recruitSrc).toMatch(/export const RECRUIT_PAGE_SIZE : number = 20/);
    expect(recruitSrc).toMatch(/params\['page_size'\] = RECRUIT_PAGE_SIZE\.toString\(\)/);
    // 列表页累积并自增 page（响应不回显 page）
    expect(librarySrc).toContain('page.value = page.value + 1');
    expect(librarySrc).toContain('getRecruitResumesApi(filters.value, page.value)');
    expect(librarySrc).toContain('@scrolltolower="onLoadMore"');
    // 到底判据只看「本批不足一页」或「已够 total」
    expect(librarySrc).toContain('result.items.length < RECRUIT_PAGE_SIZE || items.value.length >= result.total');
  });

  test('D5：列表与详情都有招聘者身份守卫（学员态 reLaunch 到招聘者登录页，先于取数）', () => {
    [librarySrc, detailSrc].forEach((src) => {
      expect(src).toContain('isRecruiterActive()');
      expect(src).toContain("uni.reLaunch({ url: '/pages/recruiter/login' })");
    });
    // 守卫必须早于取数调用
    expect(librarySrc.indexOf('guardRecruiter()')).toBeLessThan(librarySrc.indexOf('loadResumes(true)'));
  });
});

describe('E. 403 分支在 request 出口里与 401 分开（不清态、不跳转）', () => {
  test('E1：handleStatus 有独立 403 分支，且不碰登录态与导航', () => {
    const start = requestSrc.indexOf('if (statusCode == 403) {');
    expect(start).toBeGreaterThan(-1);
    const body = requestSrc.slice(start, requestSrc.indexOf('\n    }', start));
    expect(body).toContain('statusCode = 403');
    expect(body).not.toContain('handleUnauthorized');
    expect(body).not.toContain('removeStorage');
    expect(body).not.toContain('reLaunch');
    // 401 仍是「返回 false 交给刷新/登出链」的那条
    expect(requestSrc).toContain('if (statusCode == 401) {\n        return false\n    }');
  });
});
