/**
 * 专项页「诊断筛选」契约（#1042）—— 源码契约测试缝（读源文本、断言语义；与
 * aiFeatureEntryWiringContract / aiFeatureRegistryParityContract 同款）。
 *
 * 锁的是一件事：**筛选只在诊断功能出现**，判据是注册表的**传输适配器**（`adapter`，
 * 根仓库 ADR-0047 §7）而不是比较功能键字符串（web 先例 `frontend/src/config/aiFeatures.ts` 的
 * `isDiagnosisFeature`）。
 *
 * 为什么必须锁（三条都是**写错不报错、只是静默坏**）：
 *   ① 判据写成键字面量（`featureKey == 'fault_diagnosis'`）⇒ 注册表日后新增诊断类功能时，
 *      筛选无声地不再出现；而「按 adapter 判」只需注册表多一行。
 *   ② brand/model 挪出 `isDiagnosisFeature` 守卫 ⇒ **所有**专项功能都会带上诊断过滤字段，
 *      请求体污染与契约漂移全程静默（后端对非诊断功能忽略它们，不会报错）。
 *   ③ 页面自己再判一次（而不是走 `isDiagnosis` 单点）⇒ 判据分叉，两处迟早不一致。
 *
 * 断言纪律（沿用本仓 #921 起的修正）：每个判据都先对**注入违规的变形样本**证明检测器有效，
 * 再对真实文件断言合规 —— 否则「检测器空跑」会被读成绿。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');

const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');
const FEATURE_PAGE = read('pages/ai-assistant/ai-feature.uvue');
const API = read('api/aiAssistant.uts');

/** 取 `sig` 之后第一个 `{ … }` 的**块体**（大括号配平）——防止切片越界到文件尾造成假绿。 */
function block(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error(`找不到源码片段: ${sig}`);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error(`片段后找不到 '{': ${sig}`);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error(`大括号未配平: ${sig}`);
}

/** 去掉模板/JS/CSS 注释 —— 「注释里提到某标识」不等于「代码里用了它」。 */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 子串出现次数 */
function count(haystack, needle) {
  let n = 0;
  let i = 0;
  while ((i = haystack.indexOf(needle, i)) >= 0) {
    n++;
    i += needle.length;
  }
  return n;
}

/**
 * 判据①（抽成纯函数，便于对变形样本断言）：`isDiagnosisFeature` 是否**读 adapter**、
 * 且**不认功能键字面量**。返回 false = 不合格（守卫缺失也不抛，直接判不合格）。
 */
function judgmentReadsAdapter(constantsSrc) {
  let body;
  try {
    body = block(constantsSrc, 'export function isDiagnosisFeature');
  } catch (e) {
    return false;
  }
  if (!/adapter/.test(body)) return false;
  // 不许出现诊断功能的**键字面量**：键标识「走后端哪条通道」，adapter 才是 UI 判据
  if (/'fault_diagnosis'|"fault_diagnosis"/.test(body)) return false;
  // 也不许对键名做任何 fault 字样嗅探（含 fault_consult / fault_code_query 这类近邻命名）
  if (/fault/i.test(body)) return false;
  return true;
}

/**
 * 判据②：`buildBody` 里 brand/model 的写入**全部落在 `isDiagnosisFeature` 守卫内**。
 * 判据是「函数体内出现次数 == 守卫块内出现次数」—— 任何一个写在守卫外都会让两者不等。
 */
function brandModelGuardedByAdapter(pageSrc) {
  let body;
  let guard;
  try {
    body = block(pageSrc, 'const buildBody : ChatBodyBuilder');
    guard = block(body, 'if (isDiagnosisFeature(featureKey))');
  } catch (e) {
    return false;
  }
  for (const needle of ["params['brand'] =", "params['model'] ="]) {
    const inBody = count(body, needle);
    if (inBody === 0) return false; // 字段根本没送 ⇒ 筛选不可能生效
    if (inBody !== count(guard, needle)) return false;
  }
  return true;
}

/** 判据③：页面是否走 `isDiagnosis` 单点判定（经 isDiagnosisFeature），且不出现键字面量。 */
function pageGatesFilterByAdapter(pageSrc) {
  const code = stripCodeComments(pageSrc);
  if (!/const isDiagnosis = computed\(\(\) : boolean => isDiagnosisFeature\(currentFeatureKey\.value\)\)/.test(code)) return false;
  // 筛选面板的渲染条件必须是 isDiagnosis（不是键比较、不是别的布尔）
  if (!/v-if="isDiagnosis"/.test(code)) return false;
  // 页面不得出现诊断功能的键字面量（判据分叉的另一种形态）
  if (/'fault_diagnosis'/.test(code)) return false;
  return true;
}

describe('专项页诊断筛选契约（#1042）', () => {
  describe('① 判定读 adapter，不认功能键字面量（ADR-0047 §7）', () => {
    it('isDiagnosisFeature 存在、读 adapter、且不含任何 fault 字样', () => {
      expect(judgmentReadsAdapter(CONSTANTS)).toBe(true);
    });

    it('变形样本自证：把判据改成键字面量 ⇒ 变红', () => {
      const broken = CONSTANTS.replace(
        "return FUNCTION_ITEMS[i].adapter == 'diagnosis'",
        "return FUNCTION_ITEMS[i].featureKey == 'fault_diagnosis'"
      );
      expect(broken).not.toBe(CONSTANTS); // 锚点确实命中（否则下面是在测原文件）
      expect(judgmentReadsAdapter(broken)).toBe(false);
    });

    it('变形样本自证：把判据改成 fault 字样嗅探 ⇒ 变红', () => {
      const broken = CONSTANTS.replace(
        "return FUNCTION_ITEMS[i].adapter == 'diagnosis'",
        "return FUNCTION_ITEMS[i].featureKey.indexOf('fault') >= 0"
      );
      expect(judgmentReadsAdapter(broken)).toBe(false);
    });

    it('变形样本自证：判据函数整个删掉 ⇒ 变红（而不是被当成合规）', () => {
      const broken = CONSTANTS.replace('export function isDiagnosisFeature', 'function isDiagnosisFeatureRemoved');
      expect(judgmentReadsAdapter(broken)).toBe(false);
    });
  });

  describe('② buildBody：品牌/车型只在 isDiagnosisFeature 守卫内（非诊断功能一个字段都不多送）', () => {
    it('brand/model 的写入全部落在守卫内', () => {
      expect(brandModelGuardedByAdapter(FEATURE_PAGE)).toBe(true);
    });

    it('变形样本自证：守卫条件换成「永远为真」⇒ 变红', () => {
      const broken = FEATURE_PAGE.replace('if (isDiagnosisFeature(featureKey))', 'if (true)');
      expect(brandModelGuardedByAdapter(broken)).toBe(false);
    });

    it('变形样本自证：把 brand 写到守卫之外（泄漏给所有专项功能）⇒ 变红', () => {
      const leaked = FEATURE_PAGE.replace(
        "params['feature_key'] = featureKey",
        "params['feature_key'] = featureKey\n\t\t\tparams['brand'] = selectedBrand.value"
      );
      expect(leaked).not.toBe(FEATURE_PAGE);
      expect(brandModelGuardedByAdapter(leaked)).toBe(false);
    });

    it('变形样本自证：把 brand/model 整块删掉（筛选不生效）⇒ 变红', () => {
      const dropped = FEATURE_PAGE.replace(/if \(isDiagnosisFeature\(featureKey\)\) \{[\s\S]*?\n\t\t\t\}/, '');
      expect(brandModelGuardedByAdapter(dropped)).toBe(false);
    });
  });

  describe('③ 页面走 isDiagnosis 单点判定，筛选面板按它渲染', () => {
    it('页面经 isDiagnosisFeature 判定，且筛选面板的条件是 isDiagnosis', () => {
      expect(pageGatesFilterByAdapter(FEATURE_PAGE)).toBe(true);
    });

    it('变形样本自证：面板条件改成键比较（判据分叉）⇒ 变红', () => {
      const broken = FEATURE_PAGE.replace('v-if="isDiagnosis"', "v-if=\"currentFeatureKey == 'fault_diagnosis'\"");
      expect(broken).not.toBe(FEATURE_PAGE);
      expect(pageGatesFilterByAdapter(broken)).toBe(false);
    });

    it('品牌清单只在诊断功能进页时拉取（非诊断功能不请求诊断代理）', () => {
      const onLoad = block(FEATURE_PAGE, 'onLoad((options : OnLoadOptions)');
      expect(onLoad).toMatch(/if \(isDiagnosis\.value\) \{\s*loadBrands\(\)/);
      // 故障码**不预取**：折叠面板不该白跑一次代理查询（按需才查）
      expect(onLoad).not.toMatch(/loadFaultCodes/);
    });
  });

  describe('④ 三个数据源：API 层同形调用 + 页面按需消费', () => {
    it('API 层导出三个诊断只读调用，且都走 getMapped 出口（本仓契约收紧后的形态）', () => {
      const fns = ['getDiagnosisBrandsApi', 'getDiagnosisModelsApi', 'getDiagnosisFaultCodesApi'];
      for (const fn of fns) {
        expect(API).toMatch(new RegExp(`export function ${fn}\\(`));
        expect(block(API, `export function ${fn}`)).toMatch(/getMapped</);
      }
    });

    it('打的是**既有**后端端点（本票零后端改动：/ai-assistant/diagnosis/*）', () => {
      expect(API).toMatch(/getMapped<AiDiagnosisBrandOption\[\]>\('\/ai-assistant\/diagnosis\/brands'/);
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/models'/);
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/fault-codes'/);
    });

    it('页面只import 这三个源，并按需调用（品牌进页拉、车型随品牌、故障码按查询）', () => {
      expect(FEATURE_PAGE).toMatch(
        /import \{ getDiagnosisBrandsApi, getDiagnosisModelsApi, getDiagnosisFaultCodesApi \} from '\.\.\/\.\.\/api\/aiAssistant'/
      );
      expect(block(FEATURE_PAGE, 'async function loadBrands')).toMatch(/getDiagnosisBrandsApi\(\)/);
      expect(block(FEATURE_PAGE, 'async function onBrandSelect')).toMatch(/getDiagnosisModelsApi\(/);
      expect(block(FEATURE_PAGE, 'async function loadFaultCodes')).toMatch(/getDiagnosisFaultCodesApi\(/);
    });

    it('品牌/车型是**联动**：换品牌清空车型、`all` 不请求车型（同 web 侧行为）', () => {
      const sel = block(FEATURE_PAGE, 'async function onBrandSelect');
      expect(sel).toMatch(/selectedModel\.value = ''/);
      expect(sel).toMatch(/models\.value = \[\]/);
      expect(sel).toMatch(/brandValue == 'all'/);
    });

    it('故障码分页：首查覆盖、翻页追加（「加载更多」不是刷新）', () => {
      const load = block(FEATURE_PAGE, 'async function loadFaultCodes');
      expect(load).toMatch(/if \(page <= 1\)/);
      expect(load).toMatch(/faultCodes\.value\.push\(/);
    });

    it('品牌/车型用结构化字段传参，**不拼进消息文本**（#1042 的裁定，见 api 层长注释）', () => {
      const body = block(FEATURE_PAGE, 'const buildBody : ChatBodyBuilder');
      // 结构化：写进请求体
      expect(body).toMatch(/params\['brand'\] = selectedBrand\.value/);
      expect(body).toMatch(/params\['model'\] = selectedModel\.value/);
      // 不拼文本：消息历史按原样进 body，不做前缀/后缀拼接
      expect(body).toMatch(/messages: history/);
      expect(body).not.toMatch(/history\[.*\]\.content \+/);
      expect(body).not.toMatch(/brandLabelOf\(/);
    });
  });

  describe('⑤ 明确不做（防加戏）', () => {
    it('不含用户上传图片（ADR-0009 修订 ② 已排除，本票不复活）', () => {
      expect(FEATURE_PAGE).not.toMatch(/chooseImage|uploadAiImageApi|showImageButton/);
    });

    it('不改通用页：通用页不出现诊断筛选的状态或数据源', () => {
      const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
      expect(stripCodeComments(PAGE)).not.toMatch(/getDiagnosisBrandsApi|getDiagnosisModelsApi|getDiagnosisFaultCodesApi/);
      expect(stripCodeComments(PAGE)).not.toMatch(/isDiagnosisFeature/);
    });
  });
});
