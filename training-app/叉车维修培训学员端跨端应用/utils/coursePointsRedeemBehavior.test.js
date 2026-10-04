/**
 * #1421（移动端 ADR-0031 决策 1/4）：课程兑换动词与详情 DTO 迁出 —— **行为级**守护
 *
 * 真跑 `api/points.uts` 与 `api/course.uts`（`loadUts` 注入桩，先例 `credentialGroupedBehavior`），
 * 钉住两件事 —— 都是「只有跑起来才看得见」的：
 * 1) `redeemCoursePointsApi(courseId)` 的请求形状：POST `/points/shop/course/{id}/redeem`
 *    （后端 `points.go` 的 `g.POST("/shop/course/:courseId/redeem")`），响应四字段直读；
 *    业务失败（积分不足/已兑换/课程不存在）**原样 reject 后端文案** —— api 层不做字符串比对、
 *    不折成功、不静默（哨兵→状态码映射在服务端 `pointsErrStatus`，客户端只是搬运工）。
 * 2) 详情 mapper 逐字段迁出 `points_price` / `entitled`（「后端暂未返回」谎言注释的终结之地）：
 *    真 DTO 样本（形态抄 `backend/internal/course/service.go` CourseDTO 的线上投影）下，
 *    两槽必须解出真值；缺省/免费/未登录三形态各走各的可空语义，不得压成同一个值。
 *
 * C 组注入变异 = 成对取证的另一半（docs/agents/guards.md 判据 ③）。
 * 接线面（页面 import、幻影路由白名单、DTO 出口家族）由 `coursesContract` / `pointsRealApiContract` 守。
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const POINTS_UTS = path.join(ROOT, 'api', 'points.uts');
const COURSE_UTS = path.join(ROOT, 'api', 'course.uts');

/** helpers.uts 的真实现（同一判据不另抄一份镜像 —— 直接跑真源） */
const helpers = loadUts(path.join(ROOT, 'api', 'helpers.uts'), {});
const { toNumber, toNumberOrNull, toBool, toStr } = helpers;

/** 捕获 postMapped/getMapped 调用的最小桩：返回值由用例给（reject 用 err 槽） */
function makeRequestStub(reply) {
  const calls = [];
  const getMapped = (url, params, map, options) => {
    calls.push({ method: 'GET', url, params, silent: options == null ? null : options.silent });
    return reply == null ? Promise.resolve(map({})) : (reply.err != null ? Promise.reject(reply.err) : Promise.resolve(map(reply.data)));
  };
  const postMapped = (url, data, map) => {
    calls.push({ method: 'POST', url, data });
    return reply == null ? Promise.resolve(map({})) : (reply.err != null ? Promise.reject(reply.err) : Promise.resolve(map(reply.data)));
  };
  return { calls, getMapped, postMapped };
}

function loadPoints(reply) {
  const stub = makeRequestStub(reply);
  const mod = loadUts(POINTS_UTS, { getMapped: stub.getMapped, postMapped: stub.postMapped, toNumber, toStr });
  return { mod, calls: stub.calls };
}

function loadCourse(reply, source) {
  const stub = makeRequestStub(reply);
  const file = source == null ? COURSE_UTS : (() => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1421-r-'));
    const f = path.join(dir, 'course.uts');
    fs.writeFileSync(f, source);
    return f;
  })();
  const mod = loadUts(file, {
    getMapped: stub.getMapped,
    post: stub.postMapped,
    NOT_FOUND_MESSAGE_PREFIX: '404:',
    toNumber,
    toNumberOrNull,
    toBool,
    toStr,
    errMsg: helpers.errMsg,
  });
  return { mod, calls: stub.calls };
}

/** 变异真源；锚点不唯一即抛（防「变异静默落空 ⇒ 必红侧变假绿」，同 chapterNotFoundBehavior 纪律） */
function mutateOnce(src, from, to) {
  const parts = src.split(from);
  if (parts.length !== 2) {
    throw new Error(`变异锚点必须唯一：${JSON.stringify(from)} 命中 ${parts.length - 1} 次`);
  }
  return parts.join(to);
}

/** 后端 GET /course/:id 的有主体投影（course_info 里带 points_price/entitled；字段形态抄 CourseDTO） */
const detailPayload = (infoOver = {}, topOver = {}) => ({
  course_info: {
    course_id: 7, name: '叉车液压系统检修', description: 'd', cover_image: '',
    duration: 96, specialty_id: 2, level_id: 3, theory_hours: 4, practice_hours: 8,
    points_price: 300, entitled: false, ...infoOver,
  },
  chapters: [], progress: 0, is_enrolled: false, completed_chapters: 0, ...topOver,
});

describe('A. redeemCoursePointsApi：请求形状与失败语义（真 api/points.uts）', () => {
  test('A1 成功：POST 到 /points/shop/course/{id}/redeem，响应四字段直读（复用 PointsRedeemResult 出口）', async () => {
    const { mod, calls } = loadPoints({
      data: { balance: 200, total_earned: 500, sku: 'course:7', ref_id: '7' },
    });
    const res = await mod.redeemCoursePointsApi(7);
    expect(calls).toEqual([{ method: 'POST', url: '/points/shop/course/7/redeem', data: null }]);
    expect(res.balance).toBe(200);
    expect(res.total_earned).toBe(500);
    expect(res.sku).toBe('course:7');
    expect(res.ref_id).toBe('7');
  });

  test('A2 业务失败（后端哨兵 400「积分不足」）：原样 reject 该文案，api 层不吞错不改写', async () => {
    const { mod, calls } = loadPoints({ err: new Error('积分不足') });
    await expect(mod.redeemCoursePointsApi(7)).rejects.toThrow('积分不足');
    expect(calls).toHaveLength(1);
    // 判别力反向锁：源码里没有对失败文案的字符串比对（比对即语义劫持，口径同 #1268 不按中文文案判定）
    const src = stripComments(readText(POINTS_UTS));
    expect(src).not.toContain('积分不足');
    expect(src).not.toContain('已兑换');
  });

  test('A3 兑换动词零本地预扣：请求不带任何 payload 数字、响应之外不改余额事实（事实源在服务端管线）', async () => {
    const { mod, calls } = loadPoints({ data: { balance: 200, total_earned: 500, sku: 'course:7', ref_id: '7' } });
    await mod.redeemCoursePointsApi(7);
    expect(calls[0].data).toBeNull();
  });

  test('A4 course 域不得长出第二个兑换出口（兑换面单点归详情 + 动词单点归 points 域，ADR-0031 决策 1/5）', () => {
    const src = stripComments(readText(COURSE_UTS));
    expect(src).not.toContain('redeem');
    expect(src).not.toContain('/points/');
  });

  test('A5 装饰性余额预读走静默：getPointsBalanceApi(true) 把 silent 透传给 getMapped（默认不静默）', async () => {
    const bal = { data: { balance: 500, total_earned: 500, total_spent: 0 } };
    const silent = loadPoints(bal);
    await silent.mod.getPointsBalanceApi(true);
    expect(silent.calls[0].url).toBe('/points/balance');
    expect(silent.calls[0].silent).toBe(true);
    // 反向：默认调用不静默（其它消费方仍按 request 层弹错）
    const loud = loadPoints(bal);
    await loud.mod.getPointsBalanceApi();
    expect(loud.calls[0].silent).toBe(false);
  });
});

describe('B. 详情 mapper 逐字段迁出 points_price / entitled（真 DTO 样本，谎言注释终结）', () => {
  test('B1 付费未拥有：points_price=300、entitled=false 两槽都解出真值', async () => {
    const { mod } = loadCourse({ data: detailPayload() });
    const d = await mod.getCourseDetailApi(7);
    expect(d.points_price).toBe(300);
    expect(d.entitled).toBe(false);
  });

  test('B2 付费已拥有（兑换后重拉的服务端投影）：entitled=true —— 刷新/重进仍是已解锁的判据源', async () => {
    const { mod } = loadCourse({ data: detailPayload({ entitled: true }) });
    const d = await mod.getCourseDetailApi(7);
    expect(d.points_price).toBe(300);
    expect(d.entitled).toBe(true);
  });

  test('B3 免费课（后端 omitempty：两槽都缺省）：points_price=null 且 entitled=null，不得压成 0/false', async () => {
    const payload = detailPayload();
    delete payload.course_info.points_price;
    delete payload.course_info.entitled;
    const { mod } = loadCourse({ data: payload });
    const d = await mod.getCourseDetailApi(7);
    expect(d.points_price).toBeNull();
    expect(d.entitled).toBeNull();
  });

  test('B4 列表面：buildCourseItem 迁出 points_price（公开面无主体 ⇒ entitled 槽列表上不迁，见 types 注释）', async () => {
    const { mod } = loadCourse({
      data: { courses: [{ course_id: 1, name: 'A', points_price: 300 }, { course_id: 2, name: 'B' }], total: 2, page: 1, pages: 1 },
    });
    const res = await mod.getCourseListApi(1, 12, 0, 0, 0);
    expect(res.items[0].points_price).toBe(300);
    expect(res.items[1].points_price).toBeNull();
  });
});

describe('C. 判别力自检：变异真源必须让对应行为断言变红', () => {
  const courseSrc = readText(COURSE_UTS);

  test('C1 把 points_price 迁出摘掉（谎言注释的“合法续命”形态）⇒ B1 必须红', async () => {
    const mutated = mutateOnce(courseSrc, "\t\tpoints_price: pointsPrice,\n", '');
    const { mod } = loadCourse({ data: detailPayload() }, mutated);
    const d = await mod.getCourseDetailApi(7);
    expect(d.points_price).toBeUndefined();
  });

  test('C2 把 entitled 用 toBool 兜底（缺省被压成 false）⇒ B3 的「未登录≠未解锁」可空语义必须红', async () => {
    const mutated = mutateOnce(
      courseSrc,
      "\t\tif (entitledRaw != null) {\n\t\t\tentitled = toBool(entitledRaw, false)\n\t\t}",
      "\t\tentitled = toBool(entitledRaw, false)",
    );
    const payload = detailPayload();
    delete payload.course_info.entitled;
    const { mod } = loadCourse({ data: payload }, mutated);
    const d = await mod.getCourseDetailApi(7);
    expect(d.entitled).toBe(false); // 变异形态：读不到被压成「未解锁」—— B3 断言（toBeNull）在其上必红
  });

  test('C3 兑换端点前缀写错（course→courses）⇒ 请求形状断言必须红', async () => {
    const mutated = mutateOnce(readText(POINTS_UTS), "'/points/shop/course/'", "'/points/shop/courses/'");
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'fl-1421-p-'));
    const f = path.join(dir, 'points.uts');
    fs.writeFileSync(f, mutated);
    try {
      const stub = makeRequestStub(null);
      const mod = loadUts(f, { getMapped: stub.getMapped, postMapped: stub.postMapped, toNumber, toStr });
      await mod.redeemCoursePointsApi(7);
      expect(stub.calls[0].url).not.toBe('/points/shop/course/7/redeem');
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

/** 注释剥离（同 pointsRealApiContract 口径：说明性注释提到哨兵文案不算代码比对） */
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}
