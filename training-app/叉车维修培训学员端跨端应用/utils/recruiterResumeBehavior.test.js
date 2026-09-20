/**
 * 招聘者简历库 / 简历详情 / 打码 PDF —— **行为层**测试（ADR-0022 P3 = #1196）
 *
 * 与 `utils/recruiterResumeContract.test.js` 的分工（沿用 #1124 的两层口径）：
 *   - 本文件**真跑** `api/recruit.uts`：请求形状（URL / query / page）、403 分流、
 *     「下载后交系统打开」的出路（`uni.downloadFile` → `uni.openDocument`）都在运行期断言；
 *   - 契约文件断言源码文本（字段清单、页面不触敏感键、filter 抽屉 8 维）—— 证明接线没断。
 *
 * 为什么 403 要专门测：后端明文端点**未授权返回 403（不是 404）**。若把它并进「HTTP 错误」，
 * 详情页会把每个未授权简历都渲染成「加载失败」，且这条分支必须**不清登录态、不跳登录页**
 * （唯一 reLaunch 出口是 `handleUnauthorized`，见 `concurrent401RefreshContract.test.js` 的 C2）。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const RECRUIT = path.join(ROOT, 'api', 'recruit.uts');
const REQUEST = path.join(ROOT, 'api', 'request.uts');

/** 8 维筛选的「全部留空」形态 */
function emptyFilters() {
  return {
    region: '',
    position_id: null,
    credential_id: null,
    salary_min: null,
    salary_max: null,
    experience_min: null,
    job_nature: '',
    available_in: '',
  };
}

/**
 * 把 `api/recruit.uts` 跑起来，并记录它发出的每一次请求。
 * @param {{ statusCode?: number, data?: object, reject?: Error }} reply 下一次 get/post 的应答
 */
function loadRecruit(reply = {}) {
  const calls = [];
  const uni = {
    downloadFileCalls: [],
    openDocumentCalls: [],
    toasts: [],
    downloadFile(opts) {
      uni.downloadFileCalls.push(opts);
      const r = reply.download || { statusCode: 200, tempFilePath: '/tmp/masked.pdf' };
      if (r.fail) opts.fail(r.fail);
      else opts.success(r);
    },
    openDocument(opts) {
      uni.openDocumentCalls.push(opts);
      const r = reply.open || {};
      if (r.fail) opts.fail(r.fail);
      else if (opts.success) opts.success({});
    },
  };

  const respond = (url, params) => {
    calls.push({ url, params });
    if (reply.reject) return Promise.reject(reply.reject);
    return Promise.resolve(reply.data || {});
  };

  const bindings = {
    get: (url, params) => respond(url, params),
    post: (url, data) => respond(url, data),
    API_BASE_URL: 'https://example.test/api',
    toNumber: (v, d = 0) => (v == null ? d : (Number.isNaN(parseFloat(`${v}`)) ? d : parseFloat(`${v}`))),
    toNumberOrNull: (v) => (v == null ? null : (Number.isNaN(parseFloat(`${v}`)) ? null : parseFloat(`${v}`))),
    toStr: (v, d = '') => (v == null ? d : `${v}`),
    errMsg: (e, fallback) => (e instanceof Error && e.message ? e.message : fallback),
    STORAGE_KEY_TOKEN: 'auth_token',
    getStorage: () => 'recruiter-access-token',
    uni,
  };

  const mod = loadUts(RECRUIT, bindings);
  return { mod, calls, uni };
}

describe('A. 简历库列表：page_size=20 + 8 维筛选的形状', () => {
  test('A1：首次加载发 page=1&page_size=20，且只有这两个键（未筛选时不发空键）', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi(emptyFilters(), 1);
    expect(calls.length).toBe(1);
    expect(calls[0].url).toBe('/recruit/resumes');
    // 只校验存在的键：空值维度**不得**出现在 query 里（写成空串会被后端当条件）
    expect(calls[0].params.page).toBe('1');
    expect(calls[0].params.page_size).toBe('20');
    expect(Object.keys(calls[0].params).sort()).toEqual(['page', 'page_size']);
  });

  test('A2：「加载更多」= 同一组筛选换 page=2（响应不回显 page，页码由请求侧自记）', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 40 } });
    const filters = emptyFilters();
    filters.region = '杭州市';
    await mod.getRecruitResumesApi(filters, 1);
    await mod.getRecruitResumesApi(filters, 2);
    expect(calls.length).toBe(2);
    expect(calls[0].params.page).toBe('1');
    expect(calls[1].params.page).toBe('2');
    expect(calls[0].params.page_size).toBe('20');
    expect(calls[1].params.page_size).toBe('20');
    // 筛选在第二批里保持不丢
    expect(calls[1].params.region).toBe('杭州市');
  });

  test('A3：8 维筛选逐维落到 query（键名与后端 api/recruit.go 的参数名逐字一致）', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi({
      region: '杭州市',
      position_id: 3,
      credential_id: 1,
      salary_min: 5000,
      salary_max: 12000,
      experience_min: 2,
      job_nature: 'fulltime',
      available_in: '随时到岗',
    }, 1);
    const p = calls[0].params;
    expect(p.region).toBe('杭州市');
    expect(p.position_id).toBe('3');
    expect(p.credential_id).toBe('1');
    expect(p.salary_min).toBe('5000');
    expect(p.salary_max).toBe('12000');
    expect(p.experience_min).toBe('2');
    expect(p.job_nature).toBe('fulltime');
    expect(p.available_in).toBe('随时到岗');
    expect(Object.keys(p).sort()).toEqual([
      'available_in', 'credential_id', 'experience_min', 'job_nature',
      'page', 'page_size', 'position_id', 'region', 'salary_max', 'salary_min',
    ]);
  });

  test('A4：region 原样透传（归一是后端 RegionCityName 的职责，移动端不自造格式）', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi({ ...emptyFilters(), region: '  苏州  ' }, 1);
    expect(calls[0].params.region).toBe('苏州'); // 只 trim，不拼省/不补「市」
  });

  test('A5：本轮不用的两维（experience_years 精确匹配 / experience_max）永远不发', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi({ ...emptyFilters(), experience_min: 3 }, 1);
    expect(calls[0].params.experience_years).toBe(undefined);
    expect(calls[0].params.experience_max).toBe(undefined);
  });

  test('A6：<=0 的位置/证书/薪资维度折成「不发该键」（后端对 0 与缺省的处理不同）', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi({ ...emptyFilters(), position_id: 0, credential_id: -1, salary_min: 0, salary_max: 0 }, 1);
    expect(calls[0].params.position_id).toBe(undefined);
    expect(calls[0].params.credential_id).toBe(undefined);
    expect(calls[0].params.salary_min).toBe(undefined);
    expect(calls[0].params.salary_max).toBe(undefined);
  });

  test('A6b：experience_min=0 是**有意义**的一维（后端 >= 0 保留全部）⇒ 必须发出去', async () => {
    const { mod, calls } = loadRecruit({ data: { items: [], total: 0 } });
    await mod.getRecruitResumesApi({ ...emptyFilters(), experience_min: 0 }, 1);
    expect(calls[0].params.experience_min).toBe('0');
  });

  test('A7：列表映射只读脱敏卡字段，items/total 从响应取', async () => {
    const { mod } = loadRecruit({
      data: {
        items: [{
          user_id: 7, real_name: '张*丰', real_name_masked: '张*丰',
          expected_regions: ['江苏省/苏州市'], salary_min: 6000, salary_max: 9000,
          experience_years: 3, self_intro: '五年叉车维修', updated_at: '2026-09-20T10:00:00Z',
          contact_state: 'approved', contact_source: 'recruiter',
          resume_experiences: [{ company: '某物流', position: '维修工', start_date: '2020-01', end_date: '', description: '保养' }],
          resume_certifications: [{ credential_id: 4, cert_no: 'N1-001', expire_date: '2028-01-01', image_urls: ['should-not-be-read'] }],
          // 后端若混进敏感键，移动端**读不出来**（无对应读取行）——见契约测试 C 组
          contact_phone: '13800000000', wechat: 'wx_should_not_leak', region: '江苏省/苏州市/工业园区',
        }],
        total: 1,
      },
    });
    const r = await mod.getRecruitResumesApi(emptyFilters(), 1);
    expect(r.total).toBe(1);
    expect(r.items.length).toBe(1);
    expect(r.items[0].real_name_masked).toBe('张*丰');
    expect(r.items[0].expected_regions).toEqual(['江苏省/苏州市']);
    expect(r.items[0].resume_certifications[0].cert_no).toBe('N1-001');
    // 脱敏卡模型上**没有**任何承接这些键的字段
    expect(r.items[0].contact_phone).toBe(undefined);
    expect(r.items[0].wechat).toBe(undefined);
    expect(r.items[0].region).toBe(undefined);
    expect(r.items[0].resume_file_url).toBe(undefined);
    expect(r.items[0].photos).toBe(undefined);
    expect(r.items[0].resume_certifications[0].image_urls).toBe(undefined);
  });
});

describe('B. 明文联系方式：恰好 6 键 + 403 分流', () => {
  test('B1：响应 6 键全部映射出来（real_name/contact_phone/wechat/resume_file_url/photos/resume_certifications）', async () => {
    const { mod, calls } = loadRecruit({
      data: {
        real_name: '张三丰',
        contact_phone: '13800000000',
        wechat: 'wx_zhangsan',
        resume_file_url: 'https://example.test/api/uploads/resume.pdf',
        photos: ['https://example.test/api/uploads/p1.jpg'],
        resume_certifications: [{ credential_id: 4, cert_no: 'N1-001', image_urls: ['https://example.test/api/uploads/c1.jpg'] }],
      },
    });
    const dto = await mod.getRecruitResumeContactApi(7);
    expect(calls[0].url).toBe('/recruit/resumes/7/contact');
    expect(dto.real_name).toBe('张三丰');
    expect(dto.contact_phone).toBe('13800000000');
    expect(dto.wechat).toBe('wx_zhangsan');
    expect(dto.resume_file_url).toBe('https://example.test/api/uploads/resume.pdf');
    expect(dto.photos).toEqual(['https://example.test/api/uploads/p1.jpg']);
    expect(dto.resume_certifications.length).toBe(1);
  });

  test('B2：明文请求带 silent（403 是预期分支，不该弹错误 toast）', () => {
    const { mod, calls } = loadRecruit({ data: {} });
    mod.getRecruitResumeContactApi(7);
    // get(url, null, opts) 的单向断言：调用点必须把 silent 交给 request 层
    const src = readText(RECRUIT);
    expect(src).toMatch(/const opts : RequestOptions = \{ url: '\/recruit\/resumes\/' \+ userId\.toString\(\) \+ '\/contact', silent: true \}/);
    expect(calls.length).toBe(1);
  });

  test('B3：403 被识别为「无有效授权」而不是普通失败', () => {
    const { mod } = loadRecruit();
    const forbidden = new Error('无有效授权');
    forbidden.statusCode = 403;
    const other = new Error('请求失败 (500)');
    other.statusCode = 500;
    expect(mod.isContactForbidden(forbidden)).toBe(true);
    expect(mod.isContactForbidden(other)).toBe(false);
    expect(mod.isContactForbidden(null)).toBe(false);
    expect(mod.isContactForbidden(new Error('网络连接失败'))).toBe(false);
  });

  test('B4：403 不触发登出 / 不 reLaunch（唯一出口是 401 的 handleUnauthorized）', () => {
    const src = readText(REQUEST);
    const start = src.indexOf("if (statusCode == 403) {");
    expect(start).toBeGreaterThan(-1);
    const body = src.slice(start, src.indexOf('\n    }', start));
    expect(body).not.toContain('handleUnauthorized');
    expect(body).not.toContain('logoutAndReject');
    expect(body).not.toContain('removeStorage');
    expect(body).not.toContain('reLaunch');
    // 403 真值挂在 Error 上，供页面分流
    expect(body).toContain('statusCode = 403');
  });
});

describe('C. 打码 PDF：下载后交系统打开（不当 JSON 信封解析）', () => {
  test('C1：命中 /recruit/resumes/{id}/pdf，带 Bearer 且**不解析响应体**', async () => {
    const { mod, uni } = loadRecruit({ download: { statusCode: 200, tempFilePath: '/tmp/masked.pdf' } });
    const message = await mod.downloadAndOpenMaskedResumePdfApi(7);
    expect(message).toBe('');
    expect(uni.downloadFileCalls.length).toBe(1);
    expect(uni.downloadFileCalls[0].url).toBe('https://example.test/api/recruit/resumes/7/pdf');
    expect(uni.downloadFileCalls[0].header.Authorization).toBe('Bearer recruiter-access-token');
    expect(uni.openDocumentCalls.length).toBe(1);
    expect(uni.openDocumentCalls[0].filePath).toBe('/tmp/masked.pdf');
    expect(uni.openDocumentCalls[0].fileType).toBe('pdf');
  });

  test('C2：HTTP 非 200 ⇒ 给失败原因，**不**去 openDocument', async () => {
    const { mod, uni } = loadRecruit({ download: { statusCode: 404 } });
    const message = await mod.downloadAndOpenMaskedResumePdfApi(7);
    expect(message).toContain('404');
    expect(uni.openDocumentCalls.length).toBe(0);
  });

  test('C3：401 ⇒ 明确提示重新登录，不当成「文件失败」', async () => {
    const { mod, uni } = loadRecruit({ download: { statusCode: 401 } });
    const message = await mod.downloadAndOpenMaskedResumePdfApi(7);
    expect(message).toBe('登录已过期，请重新登录');
    expect(uni.openDocumentCalls.length).toBe(0);
  });

  test('C4：系统没有能打开 PDF 的应用 ⇒ 明确提示，不静默', async () => {
    const { mod } = loadRecruit({ download: { statusCode: 200, tempFilePath: '/tmp/x.pdf' }, open: { fail: true } });
    const message = await mod.downloadAndOpenMaskedResumePdfApi(7);
    expect(message).toBe('无法打开此文件');
  });

  test('C5：网络失败/超时也都返回可展示文案（页面只 toast，不需要 try/catch）', async () => {
    const { mod } = loadRecruit({ download: { fail: { errMsg: 'downloadFile:fail timeout' } } });
    expect(await mod.downloadAndOpenMaskedResumePdfApi(7)).toBe('文件下载超时，请稍后重试');
    const { mod: mod2 } = loadRecruit({ download: { fail: { errMsg: 'downloadFile:fail unknown' } } });
    expect(await mod2.downloadAndOpenMaskedResumePdfApi(7)).toBe('文件下载失败，请检查网络后重试');
  });

  test('C6：上传 PDF 与打码 PDF 走**同一个**出路（同一函数、只有路径不同）', () => {
    const src = readText(RECRUIT);
    // 打码 PDF 端点走公共出口，而不是自带一份 downloadFile 实现
    const masked = src.slice(src.indexOf('export function downloadAndOpenMaskedResumePdfApi'));
    expect(masked).toContain('downloadAndOpenAuthedFile(');
    expect(masked).not.toContain('uni.downloadFile');
    // 全文只有一处 downloadFile / 一处 openDocument 调用点
    expect((src.match(/uni\.downloadFile\(/g) || []).length).toBe(1);
    expect((src.match(/uni\.openDocument\(/g) || []).length).toBe(1);
  });
});

describe('D. 发起交换与我的交换申请', () => {
  test('D1：发起交换 POST /recruit/contact-requests，body 用小写 snake_case 二键', async () => {
    const { mod, calls } = loadRecruit({ data: { id: 11, student_user_id: 7, status: 'pending', message: '您好' } });
    const dto = await mod.createRecruitContactRequestApi(7, '您好');
    expect(calls[0].url).toBe('/recruit/contact-requests');
    expect(calls[0].params.student_user_id).toBe(7);
    expect(calls[0].params.message).toBe('您好');
    expect(dto.status).toBe('pending');
    expect(dto.id).toBe(11);
  });

  test('D2：我的交换申请列表带 page/page_size=20，并**读回显**（与简历库相反的那套行为）', async () => {
    const { mod, calls } = loadRecruit({
      data: { items: [], page: 2, page_size: 20, total: 30 },
    });
    const r = await mod.getRecruitContactRequestsApi(2, 20);
    expect(calls[0].url).toBe('/recruit/contact-requests');
    expect(calls[0].params.page).toBe('2');
    expect(calls[0].params.page_size).toBe('20');
    expect(r.page).toBe(2);
    expect(r.page_size).toBe(20);
  });
});
