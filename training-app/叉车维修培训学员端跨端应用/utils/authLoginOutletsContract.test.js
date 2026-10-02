/**
 * 登录族八条有载荷出口收紧契约测试（#1324，parent epic #638 / ADR-0007）
 *
 * 本票把 `api/auth.uts` 文件头「乙」条登记的八条裸 `get|post|put().then()` 出口迁到
 * mapper-callback 家族（#653 批 B 覆盖了资料面，这八条是当时登记的独立残表）。三把锁：
 *
 * 1) 八条出口判据（含注入自检）：每条出口必须走 mapped 形态、无裸通路残留、无 `.then()` 拆包；
 *    形态特有的字段（`buildLoginResult` 接线 / PUT 三件套 / isNew 透传）逐条钉。
 *    `loginApi` 双层 catch 是**决策保留**（ADR-0007 静默回退双判：非静默回退、非本票禁区），
 *    这里锁它的位置与第二文案源模板 —— 退役属文案变更须另立票，不许在本票顺手删。
 * 2) 登录族不回潮：八条导出签名一字未动（十处调用点零适配的前提）、十处调用点原样在位、
 *    文件头「乙」条改写带票号（不留「现无 open 票覆盖」的烂指针）、import 无孤儿 get/put。
 * 3) 幻影路由锁（#662 口径，Q3-C 全组拼装）：八条出口的 method+path 必须命中后端注册面 ——
 *    后端注册链按「`r.Group("/api")` → 注册表把 `api` 组传给四个 Register* → 各文件在
 *    `rg.Group` / 直挂 / 空路径上注册」两段机械拼装（前缀参数化两支 + 空路径一支都读真源，
 *    不硬编码）。覆盖面边界写在 `registeredRoutes` 的注释里，不拿假绿冒充全覆盖。
 *
 * 本套件是**接线守护**（源码文本判据，不构成 ③ 门的行为证据）；判据与幻影路由锁各带注入自检。
 */
const h = require('./contractHarness');
const read = h.read;

const AUTH_API = 'api/auth.uts';
const STORES = 'stores/auth.uts';
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

// ── 锁 1：八条出口判据 ─────────────────────────────────────────────────────────

/** 八条出口的必备形态：`required` = 出口调用本体，`shapes` = 该出口特有的形态字段 */
const OUTLETS = [
  { fn: 'loginApi', label: 'postMapped', required: "return postMapped<LoginResult>('/auth/login', payload,", shapes: [] },
  { fn: 'recruiterLoginApi', label: 'postMapped', required: "return postMapped<LoginResult>('/auth/recruiter-login', payload,", shapes: [] },
  { fn: 'phoneLoginApi', label: 'postMapped', required: "return postMapped<LoginResult>('/auth/phone/login', payload,", shapes: ['=> buildLoginResult(data)'] },
  { fn: 'emailLoginApi', label: 'postMapped', required: "return postMapped<LoginResult>('/auth/email/login', payload,", shapes: ['=> buildLoginResult(data)'] },
  { fn: 'refreshTokenApi', label: 'postMapped', required: "return postMapped<RefreshTokenResult>('/auth/refresh', payload,", shapes: [] },
  { fn: 'getUserInfoApi', label: 'getMapped', required: "return getMapped<UserInfo>('/auth/me', null,", shapes: ['=> buildUserInfo(data)'] },
  { fn: 'mpWechatLoginApi', label: 'postMapped', required: "return postMapped<WechatLoginResult>('/auth/wx-login', payload,", shapes: ['isNewRaw'] },
  {
    fn: 'updateAccountApi',
    label: 'requestMapped',
    required: 'return requestMapped<LoginResult>(opts,',
    // PUT 无便捷面 ⇒ 与 put() 逐字同通路的三件套（#653 updateProfileApi 先例）
    shapes: ["opts.method = 'PUT'", 'opts.data = payload', "url: '/auth/account'", '=> buildLoginResult(data)'],
  },
];

/**
 * 判据本体（真源用例与注入自检共用同一条）：出口必须走 mapped 家族且无裸通路残留。
 * 违例被判据改坏到失去判别力时，注入用例立刻红。
 */
function outletViolations(body, spec) {
  const v = [];
  if (!body.includes(spec.required)) v.push(`未走 ${spec.label} 出口`);
  if (/(?:^|[^A-Za-z0-9_$])(post|get|put)\(/.test(body)) v.push('仍有裸通路调用');
  if (body.includes('.then(')) v.push('仍有 .then() 拆包');
  for (const shape of spec.shapes) if (!body.includes(shape)) v.push('形态缺失：' + shape);
  return v;
}

describe('锁 1：八条出口判据（#1324 验收 ①：全部走 mapper-callback 家族）', () => {
  const code = stripComments(read(AUTH_API));

  it('出口家族三条 mapped 出口进 import（孤儿 get/put 已随迁移移除）', () => {
    const m = /^import \{([^}]+)\} from '\.\/request'$/m.exec(code);
    expect(m).not.toBeNull();
    const names = m[1].split(',').map((s) => s.trim());
    for (const need of ['post', 'getMapped', 'postMapped', 'requestMapped', 'del', 'uploadFile']) {
      expect(names).toContain(need);
    }
    expect(names).not.toContain('get');
    expect(names).not.toContain('put');
  });

  it.each(OUTLETS)('$fn 走 $label 出口且形态字段在位', (spec) => {
    const body = fnBodyOf(code, spec.fn);
    expect(body).not.toBe('');
    expect(outletViolations(body, spec)).toEqual([]);
  });

  it('loginApi 双层 catch 保留是决策：外层文案源位置 + 第二文案源模板逐字在位（Q1-A）', () => {
    const body = fnBodyOf(code, 'loginApi');
    // 拓扑：catch 挂在 postMapped 的 promise 上（与术前 post().then() 完全同位）
    expect(body).toContain('}).catch((e) : LoginResult => {');
    expect(body.match(/\.catch\(/g)).toHaveLength(1);
    // 第二用户可见文案源（request 层 showError 之外）：模板 + 内层解析失败文案（双前缀怪癖来源）
    expect(body).toContain('登录失败，请检查网络后重试');
    expect(body).toContain("console.error('[loginApi] login failed:', e)");
    expect(body).toContain('登录响应解析失败');
    expect(body).toContain("console.error('[loginApi] parse exception:', e)");
  });

  it('出口判据具备判别力（八条写回裸 .then() 形态必须被同一条判据逐项抓到）', () => {
    for (const spec of OUTLETS) {
      const body = fnBodyOf(code, spec.fn);
      // 注入回归：把 mapped 出口改写回裸 post/get().then()
      const bare = spec.label === 'getMapped' ? 'get' : 'post';
      const regressed = body.replace(spec.required,
        `return ${bare}('/auth/regressed', payload).then((data : UTSJSONObject) : LoginResult => {`);
      expect(regressed).not.toBe(body);
      const hits = outletViolations(regressed, spec);
      expect(hits).toContain(`未走 ${spec.label} 出口`);
      expect(hits).toContain('仍有裸通路调用');
      expect(hits).toContain('仍有 .then() 拆包');
      // 对照组：真源跑同一条判据必须零违例（防判据退化成「逢调用必报」）
      expect(outletViolations(body, spec)).toEqual([]);
    }
  });
});

describe('锁 1b：buildLoginResult 三体合一（Q2-B：逐字节同体的三条才合，其余各自保形）', () => {
  const code = stripComments(read(AUTH_API));

  it('helper 存在、四行映射体与术前三条 .then() 体逐字一致、不对外导出', () => {
    const body = fnBodyOf(code, 'buildLoginResult');
    expect(body).not.toBe('');
    expect(body).toMatch(/function buildLoginResult\(data : UTSJSONObject\) : LoginResult \{/);
    expect(code).not.toContain('export function buildLoginResult');
    expect(body).toContain("const token = data['token'] as string");
    expect(body).toContain("const refreshToken = toStr(data['refresh_token'], '')");
    expect(body).toContain('const user = buildUserInfo(data)');
    expect(body).toContain('return { token: token, user: user, refresh_token: refreshToken } as LoginResult');
  });

  it('恰好三条出口接线（phone / email / updateAccount），带 try/catch 文案的两条与 mpWechat 不并入', () => {
    expect(code.match(/=> buildLoginResult\(data\)/g)).toHaveLength(3);
    expect(fnBodyOf(code, 'phoneLoginApi')).toContain('=> buildLoginResult(data)');
    expect(fnBodyOf(code, 'emailLoginApi')).toContain('=> buildLoginResult(data)');
    expect(fnBodyOf(code, 'updateAccountApi')).toContain('=> buildLoginResult(data)');
    // 各自保形：三条不并入的出口体内不得出现 helper（迁移没顺手扩散）
    expect(fnBodyOf(code, 'loginApi')).not.toContain('buildLoginResult');
    expect(fnBodyOf(code, 'recruiterLoginApi')).not.toContain('buildLoginResult');
    expect(fnBodyOf(code, 'mpWechatLoginApi')).not.toContain('buildLoginResult');
  });

  it('判别力：把一条接线写回内联体，计数判据必须从 3 掉到 2 抓到它', () => {
    const regressed = code.replace(
      "return postMapped<LoginResult>('/auth/email/login', payload, (data : UTSJSONObject) : LoginResult => buildLoginResult(data))",
      `return postMapped<LoginResult>('/auth/email/login', payload, (data : UTSJSONObject) : LoginResult => {
        const token = data['token'] as string
        const refreshToken = toStr(data['refresh_token'], '')
        const user = buildUserInfo(data)
        return { token: token, user: user, refresh_token: refreshToken } as LoginResult
      })`
    );
    expect(regressed).not.toBe(code);
    expect(regressed.match(/=> buildLoginResult\(data\)/g)).toHaveLength(2);
    expect(code.match(/=> buildLoginResult\(data\)/g)).toHaveLength(3);
  });
});

// ── 锁 2：登录族不回潮 ────────────────────────────────────────────────────────

/** 八条导出签名（票面验收 ②：一字未动 ⇒ 十处调用点零适配的结构前提） */
const SIGNATURES = [
  'export function loginApi(params : LoginParams) : Promise<LoginResult> {',
  'export function recruiterLoginApi(params : LoginParams) : Promise<LoginResult> {',
  'export function phoneLoginApi(phone : string, code : string) : Promise<LoginResult> {',
  'export function emailLoginApi(email : string, code : string) : Promise<LoginResult> {',
  'export function refreshTokenApi(refreshToken : string) : Promise<RefreshTokenResult> {',
  'export function getUserInfoApi() : Promise<UserInfo> {',
  'export function mpWechatLoginApi(code : string) : Promise<WechatLoginResult> {',
  'export function updateAccountApi(account : string, code : string) : Promise<LoginResult> {',
];

describe('锁 2：登录族不回潮（签名零改动 + 十处调用点原样在位 + 文件头指针不烂）', () => {
  const src = read(AUTH_API);

  it.each(SIGNATURES)('%s', (sig) => {
    expect(src).toContain(sig);
  });

  it('调用点普查在位（#1324 十处基线，#1391 后 stores 侧 8 处）：全部 await DTO', () => {
    // 普查基线 = #1324 收紧时的十处（stores 9 + flows 1）。#1391 把快捷登录从
    // 「refreshTokenApi + getUserInfoApi 回拉」换成「loginApi 一次拿全」，
    // stores 侧因此 -1（refreshTokenApi 只剩 tryRefreshToken）-1（getUserInfoApi 只剩 validateToken）+1（loginApi 多处一条）
    // ⇒ 现判据是**逐条点名的精确计数**，不是放宽：任何一处调用点消失或回潮都会红。
    const stores = read(STORES);
    const flows = read(FLOWS);
    const expectIn = (hay, needle, times) => {
      const n = hay.split(needle).length - 1;
      expect({ needle, n }).toEqual({ needle, n: times });
    };
    expectIn(stores, 'await recruiterLoginApi(params)', 1);
    expectIn(stores, 'await loginApi(params)', 2);
    expectIn(stores, 'await emailLoginApi(params.target, params.code)', 1);
    expectIn(stores, 'await phoneLoginApi(params.target, params.code)', 1);
    expectIn(stores, 'await mpWechatLoginApi(code)', 1);
    expectIn(stores, 'await refreshTokenApi(rt)', 1);
    expectIn(stores, 'await getUserInfoApi()', 1);
    expectIn(flows, 'await updateAccountApi(acc, code)', 1);
    const total = ['await recruiterLoginApi(params)', 'await loginApi(params)',
      'await emailLoginApi(params.target, params.code)', 'await phoneLoginApi(params.target, params.code)',
      'await mpWechatLoginApi(code)', 'await refreshTokenApi(rt)', 'await getUserInfoApi()']
      .reduce((n, s) => n + stores.split(s).length - 1, 0)
      + (flows.split('await updateAccountApi(acc, code)').length - 1);
    expect(total).toBe(9);
  });

  it('文件头「乙」条改写带 #1324 票号，旧的「现无 open 票覆盖」烂指针已除', () => {
    expect(src).toContain('已由 #1324');
    expect(src).not.toContain('现无 open 票覆盖');
    expect(src).not.toContain('登记为独立后续票');
    // 指针指到锁本身（读文件头的人能找到判据）
    expect(src).toContain('authLoginOutletsContract.test.js');
    // 甲条：七条 void 留裸的理由仍在（改写乙条不许把甲条一起删成空指针）
    expect(src).toContain('void 透传不硬套 identity map');
    expect(src).toContain('七个函数');
  });

  it('判别力：把一条签名改形，签名判据必须抓到', () => {
    for (const sig of SIGNATURES) {
      expect(src).toContain(sig);
      expect(src.replace(sig, sig.replace(' : Promise', ' : PromiseOrNull'))).not.toBe(src);
    }
    const regressed = src.replaceAll('已由 #1324', '现无 open 票覆盖');
    expect(regressed).not.toBe(src);
    expect(regressed).not.toContain('已由 #1324');
    expect(regressed).toContain('现无 open 票覆盖');
  });
});

// ── 锁 3：幻影路由锁（Q3-C 全组拼装） ────────────────────────────────────────

/**
 * 后端注册面拼装（method + 完整 path，含 `/api` 前缀）：
 *
 * 链条一（前缀参数化）：`router.go` 里 `api := r.Group("/api")`，`router.go` 用
 *   `reg.Register(api, rd, deps)` 驱动注册表，`routes_registry.go` 的注册表闭包以 `api` 为参调
 *   `auth.RegisterRoutes(api, …)` 与三个通道 `Register*Routes(api, …)` —— 锚点逐个断言在位，
 *   `api` 组即 `/api` 这一事实由文本链机械推导，不硬编码。
 *   域包 `/auth` 组前缀与 10 条内联路由读 `internal/auth/handler.go`（P2 波 3a 起
 *   `auth := api.Group("/auth")` 那 10 行随 handler 收进域包，入口仍是注册表递来的 `api` 组）。
 * 链条二（组上注册）：`internal/auth/handler_phone.go` / `handler_email.go` 的
 *   `rg.Group("/auth/<通道>")` 前缀 × `handler_code.go` 的 `g.POST(…)` 四条路由（参数化组，两支都读真源）；
 *   `handler_wechat.go` 直挂 `rg.POST("/auth/wx-login")`；`handler_profile_bind.go` 的
 *   `acct := rg.Group("/auth/account")` + **空路径** `acct.PUT("")`（票面提醒的第三形态）。
 *
 * 覆盖面边界（显式声明，不假装全量）：本锁只拼「路由存在性」，不拼中间件链 / 处理器绑定 /
 * handler 签名 —— 那些是后端 `go test` 的面；gin 组嵌套的运行时语义同样归后端测试，
 * 这里只钉「这段文本注册链下，八条移动端路由各自存在」。
 */
function registeredRoutes() {
  const out = [];
  const anchors = [];

  const router = stripComments(read('../../backend/internal/api/router.go'));
  const apiPrefix = (/api\s*:=\s*r\.Group\("([^"]+)"\)/.exec(router) || [])[1];
  anchors.push(['router api 组', apiPrefix]);
  anchors.push(['注册表驱动', /reg\.Register\(api, rd, deps\)/.test(router) ? 'yes' : undefined]);

  // 3a 起 /api/auth 的 10 条内联注册随 handler 收进 internal/auth：组前缀与路由体读域包 handler.go，
  // 装配那一步（注册表把 api 组交给域包）在 routes_registry.go 里钉住。
  const registry = stripComments(read('../../backend/internal/api/routes_registry.go'));
  const authHandler = stripComments(read('../../backend/internal/auth/handler.go'));
  const authPrefix = (/g\s*:=\s*rg\.Group\("([^"]+)"\)/.exec(authHandler) || [])[1];
  anchors.push(['auth 域包注册入口', /auth\.RegisterRoutes\(api,/.test(registry) ? 'yes' : undefined]);
  anchors.push(['handler.go auth 组', authPrefix]);
  for (const m of authHandler.matchAll(/\bg\.(GET|POST|PUT|DELETE)\("([^"]+)"/g)) {
    out.push(`${m[1]} ${apiPrefix}${authPrefix}${m[2]}`);
  }
  for (const fn of ['RegisterEmailAuthRoutes', 'RegisterPhoneAuthRoutes', 'RegisterWechatAuthRoutes', 'RegisterProfileBindRoutes']) {
    anchors.push([`${fn} 收到 api 组`, new RegExp(`${fn}\\(api,`).test(registry) ? 'yes' : undefined]);
  }

  // 通道组（前缀参数化两支）：组前缀来自 phone/email handler，路由体来自共享的 handler_code.go
  const channelRoutes = [...stripComments(read('../../backend/internal/auth/handler_code.go'))
    .matchAll(/\bg\.(GET|POST|PUT|DELETE)\("([^"]+)"/g)];
  anchors.push(['通道路由体', channelRoutes.length >= 4 ? 'yes' : undefined]);
  for (const file of ['handler_phone.go', 'handler_email.go']) {
    const g = (/rg\.Group\("([^"]+)"\)/.exec(stripComments(read(`../../backend/internal/auth/${file}`))) || [])[1];
    anchors.push([`${file} 组前缀`, g]);
    for (const m of channelRoutes) out.push(`${m[1]} ${apiPrefix}${g}${m[2]}`);
  }

  // 微信：直挂在 rg（= /api 组）上
  const wechat = stripComments(read('../../backend/internal/auth/handler_wechat.go'));
  for (const m of wechat.matchAll(/\brg\.(GET|POST|PUT|DELETE)\("([^"]+)"/g)) {
    out.push(`${m[1]} ${apiPrefix}${m[2]}`);
  }

  // 账号修改：组 + 空路径（`acct.PUT("")` ⇒ 路径即组本身）
  const bind = stripComments(read('../../backend/internal/auth/handler_profile_bind.go'));
  const acctPrefix = (/acct\s*:=\s*rg\.Group\("([^"]+)"/.exec(bind) || [])[1];
  anchors.push(['profile_bind acct 组', acctPrefix]);
  for (const m of bind.matchAll(/\bacct\.(GET|POST|PUT|DELETE)\("([^"]*)"/g)) {
    out.push(`${m[1]} ${apiPrefix}${acctPrefix}${m[2]}`);
  }

  // 锚点 fail-closed：链条上任何一环消失 ⇒ 拼装面静默缩水，先红在这里而不是靠路由锁兜
  for (const [name, value] of anchors) expect([name, value]).toEqual([name, expect.anything()]);
  return out;
}

describe('锁 3：幻影路由锁（#662 口径）：登录族八条路由必须落在后端已注册清单内', () => {
  const code = stripComments(read(AUTH_API));

  /** 前端八条：method 从出口形态读（getMapped→GET / PUT 形态→PUT / 其余 POST），path 从体里抽 */
  function frontendRoutes() {
    const out = [];
    for (const spec of OUTLETS) {
      const body = fnBodyOf(code, spec.fn);
      const path = /'((?:\/auth)\/[^']*)'/.exec(body);
      expect({ fn: spec.fn, found: Boolean(path) }).toEqual({ fn: spec.fn, found: true });
      const method = body.includes('getMapped<') ? 'GET'
        : body.includes("opts.method = 'PUT'") ? 'PUT' : 'POST';
      out.push(`${method} ${path[1]}`);
    }
    return out;
  }

  /** 注册面去掉 apiPrefix（API_BASE_URL 已含 /api，见 config/env.uts 与 router.go r.Group("/api")） */
  function registeredNoApiPrefix() {
    const prefix = (/api\s*:=\s*r\.Group\("([^"]+)"\)/.exec(stripComments(read('../../backend/internal/api/router.go'))) || [])[1];
    expect(prefix).toBe('/api');
    return registeredRoutes().map((r) => {
      const [method, path] = r.split(' ');
      return `${method} ${path.startsWith(prefix) ? path.slice(prefix.length) : path}`;
    });
  }

  const missingRoutes = (used) => {
    const registered = registeredNoApiPrefix();
    return used.filter((u) => !registered.includes(u));
  };

  it('八条 method+path 全部命中后端注册面（前缀参数化与空路径两支都按组拼装）', () => {
    const registered = registeredNoApiPrefix();
    // 拼装面非空防假绿（router auth 组 10 条 + 通道 8 条 + 微信 1 条 + acct 2 条）
    expect(registered.length).toBeGreaterThan(15);
    expect(frontendRoutes().sort()).toEqual([
      'GET /auth/me',
      'POST /auth/email/login',
      'POST /auth/login',
      'POST /auth/phone/login',
      'POST /auth/recruiter-login',
      'POST /auth/refresh',
      'POST /auth/wx-login',
      'PUT /auth/account',
    ]);
    expect(missingRoutes(frontendRoutes())).toEqual([]);
  });

  it('判据具备判别力（幻影路由必须被抓到；少拼一支组前缀同样抓到）', () => {
    expect(missingRoutes(['POST /auth/phantom-login'])).toEqual(['POST /auth/phantom-login']);
    // 方法维度：PUT /auth/account 若退化成 GET，命中不了 PUT 注册（防止只比路径不比方法）
    expect(missingRoutes(['GET /auth/account'])).toEqual(['GET /auth/account']);
  });
});
