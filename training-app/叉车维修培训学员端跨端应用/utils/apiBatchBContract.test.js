/**
 * T15 批 B 跨模块出口收紧契约测试（#653，parent epic #638 / ADR-0007）
 *
 * 批 B 的四张面（index / exam-info / resources / profile-setup）里，只有 profile-setup 有一处
 * 真缺口：`api/auth.uts` 的 `updateProfileApi` 仍走裸 `put().then()`。其余三面术前实测即为
 * 「页面无域 api 文件、无豁免、无直发请求」⇒ 本文件对它们下的是**不回潮锁**（ADR-0007：票面写
 * 「清零」而实测已零时，交付不回潮锁而不是删东西凑数）。resources 域的出口收紧面在
 * `utils/resourcesContract.test.js`（该域 api 文件的归属契约同处）。
 *
 * 钉住的契约：
 * 1) `updateProfileApi` 走 `requestMapped` + `opts.method = 'PUT'`（与 `put()` 逐字同通路，
 *    先例 `api/credential.uts:162` 的 PATCH 形态）；签名与返回类型一字不动
 * 2) `uploadAvatarApi` 留裸 `uploadFile`，**理由写在文件头**（multipart 无 mapped 便捷面）
 * 3) 消费面零改动：五处调用点全部 `await` 后丢弃返回值 ⇒ 收紧管道不需要任何页面适配（票面 ②）
 * 4) `api/auth.uts` 文件头那句「其余资料类出口本轮不动」已被本票推翻，必须与实现同批更新
 * 5) api/ 层豁免面棘轮：整层只剩 `api/checkin.uts`（forum 域，退役面在 #654），只减不增
 * 6) 批 B 四模块零直发请求不回潮：`pages/<模块>/**` 无 `uni.request` 裸调、无 `api/request` import
 *
 * 本套件是**接线守护**（源码文本判据，不构成 ③ 门的行为证据）；每条判据带注入自检。
 */
const h = require('./contractHarness');
const read = h.read;
const { allowlistPaths } = require('./guardAllowlist');

const AUTH_API = 'api/auth.uts';
const PROFILE_SETUP = 'pages/profile-setup/profile-setup.uvue';
const FLOWS = 'pages/profile/composables/personal-info-flows.uts';

const stripComments = (src) =>
  src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');

/** 取 `function <name>` 的函数体（到行首 `}`；auth.uts 制表符缩进 ⇒ 行首 `}` 只在函数闭合处） */
function fnBodyOf(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const end = src.indexOf('\n}', start);
  return end === -1 ? src.slice(start) : src.slice(start, end);
}

describe('api/auth.uts 资料出口收紧（#653 兑现文件头遗留的 `updateProfileApi`）', () => {
  const code = stripComments(read(AUTH_API));

  it('requestMapped 进出口家族 import（PUT 无便捷面，只能走通路本体）', () => {
    expect(code).toMatch(/import\s*\{[^}]*\brequestMapped\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it('updateProfileApi 经 requestMapped 出口，请求形态与 put() 逐字一致', () => {
    const body = fnBodyOf(code, 'updateProfileApi');
    expect(body).not.toBe('');
    expect(body).toContain("opts.method = 'PUT'");
    expect(body).toContain('opts.data = payload');
    expect(body).toContain("url: '/auth/profile'");
    expect(body).toContain('return requestMapped<ProfileChangeRequest>(opts,');
    expect(body).toContain('(data : UTSJSONObject) : ProfileChangeRequest => buildReviewRequest(data)');
    // 旧形态不回潮
    expect(body).not.toMatch(/return put\(/);
    expect(body).not.toContain('.then(');
  });

  it('签名与显式 DTO 一字未动（票面 ②：调页零适配的前提）', () => {
    expect(code).toMatch(
      /export function updateProfileApi\(params : UpdateProfileParams\) : Promise<ProfileChangeRequest> \{/
    );
    // buildReviewRequest 仍是唯一映射点，且不对外暴露
    expect(code).toMatch(/function buildReviewRequest\(obj : UTSJSONObject\) : ProfileChangeRequest \{/);
    expect(code).not.toMatch(/export function buildReviewRequest/);
  });

  it('uploadAvatarApi 留裸 uploadFile 并登记理由（出口家族没有 mapped 的 multipart 面）', () => {
    const body = fnBodyOf(code, 'uploadAvatarApi');
    expect(body).toContain("return uploadFile('/auth/avatar', filePath, 'file')");
    expect(body).not.toContain('Mapped<');
    expect(read(AUTH_API)).toMatch(/裸出口白名单（T15 批 B #653 登记）/);
  });

  it('文件头「资料类出口本轮不动」的旧口径已推翻并与实现同批更新', () => {
    const src = read(AUTH_API);
    expect(src).not.toContain('其余登录 / 资料类出口本轮不动');
    expect(src).toContain('updateProfileApi` 走 `requestMapped');
    // 划清边界：profile 模块独有的账号面不属本票，免得被读成「整个 auth 资料面收完了」
    expect(src).toContain('updateAccountApi');
  });

  it('出口判据具备判别力（写回 put().then() 必须被判红）', () => {
    const src = read(AUTH_API);
    const regressed = src.replace(
      "return requestMapped<ProfileChangeRequest>(opts,",
      "return put('/auth/profile', payload).then((data : UTSJSONObject) : ProfileChangeRequest => {"
    );
    expect(regressed).not.toBe(src);
    expect(/return requestMapped<ProfileChangeRequest>\(opts,/.test(regressed)).toBe(false);
  });
});

describe('消费面零改动（票面 ②：五处调用点全部 void 用法）', () => {
  it('profile-setup 两处：await 后丢弃返回值，不索引裸 JSON', () => {
    const src = read(PROFILE_SETUP);
    expect(src).toContain('await uploadAvatarApi(avatarFilePath.value)');
    expect(src).toContain('await updateProfileApi({ nickname: trimmed })');
    expect(src).toMatch(/import \{ uploadAvatarApi, updateProfileApi \} from '\.\.\/\.\.\/api\/auth'/);
    expect(src).not.toMatch(/(updateProfileApi|uploadAvatarApi)\([^)]*\)\s*\.\s*then/);
  });

  it('profile 侧三处同形（共享出口的另一消费方也零适配）', () => {
    const src = stripComments(read(FLOWS));
    expect(src).toContain('await uploadAvatarApi(filePath)');
    expect(src).toContain('await updateProfileApi({ nickname: newName })');
    expect(src).toContain('await updateProfileApi({ company: c })');
    expect(src).not.toMatch(/await (updateProfileApi|uploadAvatarApi)\([^)]*\)\.\w/);
  });
});

describe('api/ 层豁免面棘轮（票面 ③：整层零豁免，唯一残挂 #654）', () => {
  /** 术前实测（`utils/guardAllowlist.js`）：api/ 层只剩这一条，且它是 forum 域的 #654 射程 */
  const API_LAYER_RESIDUAL = ['api/checkin.uts'];
  const apiExemptions = () => allowlistPaths().filter((p) => /^api[/\\]/.test(p));

  it('批 B 四模块名下零豁免（resources 域的逐条锁见 resourcesContract）', () => {
    const hits = allowlistPaths().filter((p) =>
      /^pages[/\\](index|exam-info|profile-setup|resources)[/\\]/.test(p) ||
      /^api[/\\](material|contribution|examInfo)\.uts$/.test(p)
    );
    expect(hits).toEqual([]);
  });

  it('api/ 层豁免集合恰好等于登记的残表面（只减不增，增须同批改这一行）', () => {
    expect(apiExemptions().sort()).toEqual(API_LAYER_RESIDUAL);
  });

  it('判据具备判别力（注入一条新的 api 层豁免必须同时被两条锁抓到）', () => {
    const injected = allowlistPaths().concat(['api/material.uts']);
    const isBatchB = (p) =>
      /^pages[/\\](index|exam-info|profile-setup|resources)[/\\]/.test(p) ||
      /^api[/\\](material|contribution|examInfo)\.uts$/.test(p);
    expect(injected.filter(isBatchB)).toEqual(['api/material.uts']);
    expect(injected.filter((p) => /^api[/\\]/.test(p)).sort())
      .toEqual(['api/checkin.uts', 'api/material.uts']);
  });
});

describe('批 B 四模块零直发请求不回潮（页面层不碰请求层）', () => {
  const MODULES = ['pages/index', 'pages/exam-info', 'pages/profile-setup', 'pages/resources'];

  it('四个模块目录下每个源文件都无 uni.request 裸调、无 api/request import', () => {
    const offenders = [];
    for (const dir of MODULES) {
      for (const rel of h.sourceFilesIn(dir)) {
        const src = stripComments(read(rel));
        if (/uni\.request\s*\(/.test(src)) offenders.push(`${rel}: uni.request 裸调`);
        if (/from '[^']*api\/request'/.test(src)) offenders.push(`${rel}: import api/request`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('判据具备判别力（注入一条裸调必须被抓到）', () => {
    const src = read(PROFILE_SETUP);
    const injected = src + "\n    uni.request({ url: 'x' })\n";
    expect(/uni\.request\s*\(/.test(stripComments(injected))).toBe(true);
    expect(/uni\.request\s*\(/.test(stripComments(src))).toBe(false);
  });

  it('每个模块目录都真的有源文件（扫描面为空会让上一条假绿）', () => {
    for (const dir of MODULES) {
      expect(h.sourceFilesIn(dir).length).toBeGreaterThan(0);
    }
  });
});
