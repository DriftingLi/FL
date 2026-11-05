/**
 * 诊断引导 composable 的**取数与留白判定** · 行为级测试（#2296，行为腿）
 *
 * 为什么它必须真跑：票面判据是「引导数据取不到、或健康探测为否时，引导区整块不渲染」。
 * 读源码文本只能证明「有一句 `guideVisible.value = false`」，证明不了**它真的把已拿到的字典
 * 一起丢掉了**（那条正是 web 侧 #2244 决策① 的血账：留着品牌卡 = 点得动、点了就 502）。
 * 同样，预选现象「在不在词表里」决定它兑现还是落空，那也只有在跑一遍之后才看得见。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`。注入两样替身：
 *   ① `ref` —— 最小 reactive 盒（本仓没有 `vue` npm 包，composable 只用到 `ref(x).value`）；
 *   ② 四个取数函数 —— 由用例按场景给数据或抛错。
 * 被测的**状态机与组句不替身**：它们来自 `utils/diagnosisGuide.uts` 与 `utils/diagnosisCompose.uts`
 * 的真实导出（手抄镜像会让「真源改了而镜子没改」静默变绿，#1273 先例）。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const COMPOSE_UTS = path.join(__dirname, 'diagnosisCompose.uts');
const GUIDE_UTS = path.join(__dirname, 'diagnosisGuide.uts');
const COMPOSABLE_UTS = path.join(__dirname, '..', 'composables', 'useDiagnosisGuide.uts');

/** 最小 ref：composable 只用到 `.value` 读写 */
function miniRef(initial) {
  return { value: initial };
}

const ZHONGLI = { id: 'zhongli', name: '中力', tagline: '电动叉车', icon: '🏭', cls: '', picker: 'system' };
const LINDE = { id: 'linde', name: '林德', tagline: '内燃叉车', icon: '🏗', cls: '', picker: 'model' };
const FAQ = [{ q: '举升无力怎么查', a: '' }, { q: '电池报警怎么办', a: '' }];

const BRAND_ITEMS = [{ id: 'battery', name: '电池系统', icon: '' }, { id: 'brake', name: '制动系统', icon: '' }];
const MODEL_ITEMS = [{ id: 'e322', name: 'E-322', icon: '' }];
const FAULT_ITEMS = [{ id: 'f1', name: '举升无力', icon: '' }];

/**
 * 装载 composable。
 * @param {{healthOk?:boolean, healthThrows?:boolean, intentsThrows?:boolean, brands?:any[], systems?:any[], faults?:any[]}} scenario
 */
function loadCompose(scenario) {
  const compose = loadUts(COMPOSE_UTS, {});
  const guide = loadUts(GUIDE_UTS, {
    emptyDiagnosisSelection: compose.emptyDiagnosisSelection,
    pickerKindOf: compose.pickerKindOf,
  });
  const calls = { health: 0, intents: 0, systems: 0, faults: 0 };
  const mod = loadUts(COMPOSABLE_UTS, {
    ref: miniRef,
    // 状态机：真源导出，逐条注入（导出的名字取自 guide 模块本身）
    initialGuideState: guide.initialGuideState,
    openWithBrand: guide.openWithBrand,
    openWithFault: guide.openWithFault,
    selectBrand: guide.selectBrand,
    applySecond: guide.applySecond,
    selectFault: guide.selectFault,
    setExtra: guide.setExtra,
    secondSourceOf: guide.secondSourceOf,
    GUIDE_STEP_BRAND: guide.GUIDE_STEP_BRAND,
    GUIDE_STEP_SECOND: guide.GUIDE_STEP_SECOND,
    GUIDE_STEP_FAULT: guide.GUIDE_STEP_FAULT,
    GUIDE_STEP_CONFIRM: guide.GUIDE_STEP_CONFIRM,
    // 组句与判档：真源
    composeDiagnosisSentence: compose.composeDiagnosisSentence,
    // 取数替身（按场景）
    getDiagnosisHealthApi: () => {
      calls.health += 1;
      if (scenario.healthThrows) return Promise.reject(new Error('502'));
      return Promise.resolve({ ok: scenario.healthOk !== false, app_configured: true });
    },
    getDiagnosisIntentsApi: () => {
      calls.intents += 1;
      if (scenario.intentsThrows) return Promise.reject(new Error('502'));
      return Promise.resolve({ brands: scenario.brands || [], faq: FAQ });
    },
    getDiagnosisSystemsApi: () => {
      calls.systems += 1;
      return Promise.resolve(scenario.systems || BRAND_ITEMS);
    },
    getDiagnosisFaultsApi: (brand, second) => {
      calls.faults += 1;
      return Promise.resolve(scenario.faults || FAULT_ITEMS);
    },
  });
  // composable 的真身是工厂函数 `useDiagnosisGuide()`：真调用一次，拿回它的返回对象
  const api = mod.useDiagnosisGuide();
  return { mod: api, calls, compose, guide };
}

describe('诊断引导 composable（#2296）', () => {
  describe('① 留白判定：一个布尔量说了算', () => {
    it('health.ok=true 且 intents 成功 ⇒ 渲染，品牌与 FAQ 都拿到', async () => {
      const { mod } = loadCompose({ brands: [ZHONGLI, LINDE] });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(true);
      expect(mod.brands.value.length).toBe(2);
      expect(mod.faq.value.length).toBe(2);
    });

    it('health.ok=false ⇒ 整块留白，且**已拿到的字典一起丢掉**（否则是点得动、点了就 502 的死入口）', async () => {
      const { mod } = loadCompose({ healthOk: false, brands: [ZHONGLI, LINDE] });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(false);
      expect(mod.brands.value.length).toBe(0);
      expect(mod.faq.value.length).toBe(0);
    });

    it('health 请求本身失败（断网 / 5xx）⇒ 与 ok=false 同义', async () => {
      const { mod } = loadCompose({ healthThrows: true, brands: [ZHONGLI] });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(false);
      expect(mod.brands.value.length).toBe(0);
    });

    it('intents 失败 ⇒ 整块留白（此时 health 是通的也不画）', async () => {
      const { mod } = loadCompose({ intentsThrows: true });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(false);
    });

    it('品牌是空集 ⇒ 不渲染（一块空白面板比没有面板更糟）', async () => {
      const { mod } = loadCompose({ brands: [] });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(false);
    });

    it('每次调用都从「不渲染」开始（上一轮的成功不能把这一轮的失败盖住）', async () => {
      const { mod } = loadCompose({ brands: [ZHONGLI] });
      await mod.loadIntents();
      expect(mod.guideVisible.value).toBe(true);
      // 同一个实例再拉一次，这次上游挂了：字典必须被清掉，不能留着上一次的
      const broken = loadCompose({ healthOk: false });
      await broken.mod.loadIntents();
      expect(broken.mod.guideVisible.value).toBe(false);
    });
  });

  describe('② 取数：第二层按档位走对端点', () => {
    it('系统档（中力）⇒ systems 被调用一次', async () => {
      const { mod, calls } = loadCompose({ brands: [ZHONGLI] });
      await mod.loadIntents();
      mod.openGuideWithBrand('zhongli');
      await new Promise((r) => setImmediate(r));
      expect(calls.systems).toBe(1);
      expect(mod.secondItems.value.length).toBe(2);
    });

    it('型号档（林德）⇒ 第二层走 faults 根层（不带 second），不碰 systems', async () => {
      const { mod, calls } = loadCompose({ brands: [LINDE], faults: MODEL_ITEMS });
      await mod.loadIntents();
      mod.openGuideWithBrand('linde');
      await new Promise((r) => setImmediate(r));
      expect(calls.systems).toBe(0);
      expect(calls.faults).toBeGreaterThanOrEqual(1);
      expect(mod.secondItems.value[0].name).toBe('E-322');
    });

    it('组句拿的是真源纯函数：选完三层后 sentence 就是那句话', async () => {
      const { mod } = loadCompose({ brands: [ZHONGLI] });
      await mod.loadIntents();
      mod.openGuideWithBrand('zhongli');
      await new Promise((r) => setImmediate(r));
      await mod.pickSecond(BRAND_ITEMS[0]);
      mod.pickFault('举升无力');
      expect(mod.sentence.value).toBe('我的中力叉车（电池系统）出现故障现象：举升无力。请给我排查步骤、根因分析和维修方案。');
    });
  });

  describe('③ 预选兑现（取决于它在不在词表里 —— 这个事实只能由取数给）', () => {
    it('预选在词表里 ⇒ 选完第二层直接成句并进确认步', async () => {
      const { mod, guide } = loadCompose({ brands: [ZHONGLI] });
      await mod.loadIntents();
      mod.openGuideWithFault('', '举升无力');
      expect(mod.state.value.step).toBe(guide.GUIDE_STEP_BRAND);
      await mod.pickBrand('zhongli');
      await new Promise((r) => setImmediate(r));
      await mod.pickSecond(BRAND_ITEMS[0]);
      expect(mod.state.value.selection.faultName).toBe('举升无力');
      expect(mod.state.value.step).toBe(guide.GUIDE_STEP_CONFIRM);
      expect(mod.sentence.value.length).toBeGreaterThan(0);
    });

    it('预选不在词表里 ⇒ 停在第三步、不是假装已选', async () => {
      const { mod, guide } = loadCompose({ brands: [ZHONGLI], faults: [{ id: 'f9', name: '别的现象', icon: '' }] });
      await mod.loadIntents();
      mod.openGuideWithFault('', '词表里没有的现象');
      await mod.pickBrand('zhongli');
      await new Promise((r) => setImmediate(r));
      await mod.pickSecond(BRAND_ITEMS[0]);
      expect(mod.state.value.selection.faultName).toBe('');
      expect(mod.state.value.step).toBe(guide.GUIDE_STEP_FAULT);
      expect(mod.sentence.value).toBe('');
    });
  });

  describe('④ 关闭即干净（票面：点选到一半关闭再进入不残留上次预选）', () => {
    it('关面板后状态回到初始、句子清空、第二三层列表清空', async () => {
      const { mod, guide } = loadCompose({ brands: [ZHONGLI] });
      await mod.loadIntents();
      mod.openGuideWithBrand('zhongli');
      await new Promise((r) => setImmediate(r));
      await mod.pickSecond(BRAND_ITEMS[0]);
      mod.pickFault('举升无力');
      expect(mod.sentence.value.length).toBeGreaterThan(0);
      mod.closeGuide();
      expect(mod.guideOpen.value).toBe(false);
      expect(mod.state.value.step).toBe(guide.GUIDE_STEP_BRAND);
      expect(mod.state.value.brandId).toBe('');
      expect(mod.state.value.selection.faultName).toBe('');
      expect(mod.sentence.value).toBe('');
      expect(mod.secondItems.value.length).toBe(0);
      expect(mod.faultItems.value.length).toBe(0);
    });
  });
});