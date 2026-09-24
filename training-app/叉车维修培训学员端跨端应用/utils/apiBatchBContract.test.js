/**
 * T15 批 B 跨模块出口收紧契约测试（#653，parent epic #638 / ADR-0007）
 *
 * 批 B 点名的四张面（index / exam-info / resources / profile-setup）里真缺口有两处：resources 域的
 * `api/material.uts`（整文件裸 `get().then()`；收紧面与幻影路由锁写在 `utils/resourcesContract.test.js`，
 * 因为那两个文件按 ADR-0023 归属 resources 模块），与 profile-setup 寄在共享文件里的
 * `api/auth.uts` 的 `updateProfileApi`（本文件锁它）。index / exam-info / profile-setup 三张面**没有域
 * api 文件**（`utils/modules.js` 声明面：各只有一个页面文件）⇒ 对它们交付**不回潮锁**（ADR-0007：票面写
 * 「清零」而实测已零时，锁住现状而不是删东西凑数）。
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
const PROFILE_PAGE = 'pages/profile/profile.uvue';

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

  /**
   * 判据本体（真源用例与注入自检共用同一条）：这条 PUT 出口若退回裸通路、或偏离 `put()` 的请求形态，
   * 这里列出违例。判据被改坏到失去判别力时，注入用例立刻红。
   */
  function outletViolations(body) {
    const v = [];
    if (!body.includes('return requestMapped<ProfileChangeRequest>(opts,')) v.push('未走 requestMapped 出口');
    if (/(?:^|[^A-Za-z0-9_$])put\(/.test(body)) v.push('仍有裸 put( 调用');
    if (body.includes('.then(')) v.push('仍有 .then() 拆包');
    for (const shape of ["opts.method = 'PUT'", 'opts.data = payload', "url: '/auth/profile'"]) {
      if (!body.includes(shape)) v.push('请求形态偏离 put()，缺 ' + shape);
    }
    return v;
  }

  it('updateProfileApi 经 requestMapped 出口，请求形态与 put() 逐字一致', () => {
    const body = fnBodyOf(code, 'updateProfileApi');
    expect(body).not.toBe('');
    expect(outletViolations(body)).toEqual([]);
    expect(body).toContain('(data : UTSJSONObject) : ProfileChangeRequest => buildReviewRequest(data)');
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

  it('出口判据具备判别力（写回 put().then() 必须被同一条判据逐项抓到）', () => {
    const body = fnBodyOf(code, 'updateProfileApi');
    const regressed = body
      .replace("return requestMapped<ProfileChangeRequest>(opts,",
        "return put('/auth/profile', payload).then((data : UTSJSONObject) : ProfileChangeRequest => {")
      .replace("opts.method = 'PUT'\n", '')
      .replace('opts.data = payload\n', '')
      .replace("const opts : RequestOptions = { url: '/auth/profile' } as RequestOptions\n", '');
    expect(regressed).not.toBe(body);
    expect(outletViolations(regressed)).toEqual([
      '未走 requestMapped 出口',
      '仍有裸 put( 调用',
      '仍有 .then() 拆包',
      "请求形态偏离 put()，缺 opts.method = 'PUT'",
      '请求形态偏离 put()，缺 opts.data = payload',
      "请求形态偏离 put()，缺 url: '/auth/profile'",
    ]);
    // 对照组：真源跑同一条判据必须零违例（防判据退化成「逢调用必报」）
    expect(outletViolations(body)).toEqual([]);
  });
});

describe('消费面零改动（票面「调用方仅机械适配」的实测：七处调用点全部 void 用法）', () => {
  /** 三个消费文件（实测七个调用点，`api/auth.uts` 文件头逐条列了行号） */
  const CONSUMERS = [PROFILE_SETUP, FLOWS, PROFILE_PAGE];

  /** 调用点行：出现 `updateProfileApi(` / `uploadAvatarApi(` 的行（import 行不带左括号，天然排除） */
  function callLines(src) {
    return src.split(/\r?\n/).map((l) => l.trim())
      .filter((l) => /(updateProfileApi|uploadAvatarApi)\s*\(/.test(l));
  }
  /** 判据本体：只有「整句就是一次 await、返回值不落地」才算 void 用法；其余形态都是消费了返回值 */
  function nonVoidCallLines(src) {
    return callLines(src).filter((l) => !/^await (updateProfileApi|uploadAvatarApi)\([^)]*\)$/.test(l));
  }

  it.each(CONSUMERS)('%s 的每个调用点都是 await 后丢弃（收紧管道不需要页面适配）', (rel) => {
    const src = stripComments(read(rel));
    expect(callLines(src).length).toBeGreaterThan(0);
    expect(nonVoidCallLines(src)).toEqual([]);
  });

  it('全仓调用点总数 = 7（新增消费方要走 DTO，别在这里悄悄加计数）', () => {
    const total = CONSUMERS.reduce((n, rel) => n + callLines(stripComments(read(rel))).length, 0);
    expect(total).toBe(7);
  });

  it('profile-setup 的 import 面逐字保持（类型名零改动 ⇒ 页面零 diff）', () => {
    expect(read(PROFILE_SETUP)).toMatch(/import \{ uploadAvatarApi, updateProfileApi \} from '\.\.\/\.\.\/api\/auth'/);
  });

  it('判据具备判别力（把一处 await 改成接返回值，同一条判据必须抓到它）', () => {
    const src = stripComments(read(PROFILE_PAGE));
    expect(nonVoidCallLines(src)).toEqual([]);
    const injected = src.replace('await updateProfileApi(patch)', 'const req = await updateProfileApi(patch)');
    expect(injected).not.toBe(src);
    expect(nonVoidCallLines(injected)).toEqual(['const req = await updateProfileApi(patch)']);
  });
});

/** 批 B 四域名下的路径（域 api 只有 resources 那两个；index / exam-info / profile-setup 无域 api 文件） */
const isBatchBPath = (p) =>
  /^pages[/\\](index|exam-info|profile-setup|resources)[/\\]/.test(p) ||
  /^api[/\\](material|contribution)\.uts$/.test(p);

describe('api/ 层豁免面棘轮（票面 ③：整层零豁免，唯一残挂 #654）', () => {
  /** 术前实测（`utils/guardAllowlist.js`）：api/ 层只剩这一条，摘除动作在 #654 的射程里 */
  const API_LAYER_RESIDUAL = ['api/checkin.uts'];
  const apiExemptions = () => allowlistPaths().filter((p) => /^api[/\\]/.test(p));

  it('批 B 四域名下零豁免（术前即为零 ⇒ 不回潮锁；resources 侧另有同向锁）', () => {
    expect(allowlistPaths().filter(isBatchBPath)).toEqual([]);
  });

  it('api/ 层豁免集合恰好等于登记的残表面（只减不增，增须同批改这一行）', () => {
    expect(apiExemptions().sort()).toEqual(API_LAYER_RESIDUAL);
  });

  it('判据具备判别力（注入一条批 B 域豁免，两条锁必须各自抓到它）', () => {
    const injected = allowlistPaths().concat(['api/material.uts']);
    expect(injected.filter(isBatchBPath)).toEqual(['api/material.uts']);
    expect(injected.filter((p) => /^api[/\\]/.test(p)).sort())
      .toEqual([...API_LAYER_RESIDUAL, 'api/material.uts'].sort());
  });
});

describe('批 B 四模块零直发请求不回潮（页面层不碰请求层）', () => {
  const MODULES = ['pages/index', 'pages/exam-info', 'pages/profile-setup', 'pages/resources'];

  /** 判据本体：一个源文件的直发请求面（真源扫描与注入自检共用同一条） */
  function directRequestHits(src) {
    const code = stripComments(src);
    const hits = [];
    if (/uni\.request\s*\(/.test(code)) hits.push('uni.request 裸调');
    if (/from '[^']*api\/request'/.test(code)) hits.push('import api/request');
    return hits;
  }

  const moduleSources = () => {
    const out = [];
    for (const dir of MODULES) for (const rel of h.sourceFilesIn(dir)) out.push([rel, read(rel)]);
    return out;
  };

  it('四个模块目录下每个源文件都无 uni.request 裸调、无 api/request import', () => {
    const files = moduleSources();
    expect(files.length).toBeGreaterThan(0);
    expect(files.filter(([, src]) => directRequestHits(src).length > 0)).toEqual([]);
  });

  it('判据具备判别力（同一判据跑注入源必须两条违例都抓到，跑真源必须零）', () => {
    const clean = read(PROFILE_SETUP);
    expect(directRequestHits(clean)).toEqual([]);
    const injected = clean + "\n    uni.request({ url: 'x' })\n    import { get } from '../../api/request'\n";
    expect(directRequestHits(injected)).toEqual(['uni.request 裸调', 'import api/request']);
  });

  it('每个模块目录都真的有源文件（扫描面为空会让上一条假绿）', () => {
    for (const dir of MODULES) {
      expect(h.sourceFilesIn(dir).length).toBeGreaterThan(0);
    }
  });
});
