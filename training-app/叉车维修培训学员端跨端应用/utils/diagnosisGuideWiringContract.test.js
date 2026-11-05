/**
 * 诊断引导面的**接线契约**（#2296）—— 源码契约测试缝（读源文本、断言语义）
 *
 * 本文件守的是**票面验收里那几条「接线」判据**，行为那半在 `utils/diagnosisGuideBehavior.test.js`：
 *   · 页面上的诊断专有判定**只经单一判据函数**，不出现按功能键字面比较；
 *   · 引导区整块留白只有一个判据（`guideVisible`），且 `health.ok=false` ⇒ 不渲染；
 *   · 组好的句子**进输入框、不自动发送**（票面「可编辑后再发送」）；
 *   · 取数只有四个 GET（intents / health / systems / faults），没有第五条自造端点。
 *
 * 断言纪律（沿用 #921 评审修正）：函数体一律用 `fnBody()` 限在函数体内（大括号配平），
 * 且「检测器有没有牙」用手工变形样本自证 —— 存在性断言是本仓踩过的假绿。
 */
const path = require('path');

const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const PAGE = read('pages/ai-assistant/ai-feature.uvue');
const GUIDE_UI = read('components/ai-chat/ai-chat-diagnosis-guide.uvue');
const COMPOSABLE = read('composables/useDiagnosisGuide.uts');
const API = read('api/aiAssistant.uts');
const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');
const COMPOSE = read('utils/diagnosisCompose.uts');
const STATE = read('utils/diagnosisGuide.uts');
const INPUT = read('components/ai-chat/ai-chat-input.uvue');

/** 取函数体（大括号配平）——防止切片越界到文件尾造成假绿 */
function fnBody(src, sig) {
  const start = src.indexOf(sig);
  if (start < 0) throw new Error('找不到源码片段: ' + sig);
  const open = src.indexOf('{', start);
  if (open < 0) throw new Error('片段后找不到 { : ' + sig);
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(open, i + 1);
    }
  }
  throw new Error('大括号未配平: ' + sig);
}

/** 去模板/JS 注释：「注释里提到某个字段」不等于「代码里用了它」 */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 页面上有没有「按功能键字面量比较」的诊断判定（禁形态） */
function hasFeatureKeyLiteralCompare(src) {
  const code = stripCodeComments(src);
  return /featureKey\s*={2,3}\s*'fault_diagnosis'/.test(code)
    || /(currentFeatureKey|featureKey)\.value\s*={2,3}\s*'fault_diagnosis'/.test(code);
}

describe('诊断引导面接线契约（#2296）', () => {
  describe('① 单一判据函数（票面：诊断专有判定只经一个函数，不按功能键字面比较）', () => {
    it('页面用 isDiagnosisFeature 定位「本页是不是诊断页」，不做字面比较', () => {
      // 判据是「从常量表拿到这个函数」，不是 import 的具体排列（合并 import 不算违背单一判据）
      expect(PAGE).toMatch(/import \{[^}]*\bisDiagnosisFeature\b[^}]*\} from '.\/ai-assistant-constants'/);
      expect(PAGE).toMatch(/isDiagnosis\.value = isDiagnosisFeature\(item\.featureKey\)/);
      expect(hasFeatureKeyLiteralCompare(PAGE)).toBe(false);
    });

    it('判据函数读注册表 adapter 列（不是键集合）—— 换一个诊断类功能键不需要改页面', () => {
      const fn = fnBody(CONSTANTS, 'function isDiagnosisFeature');
      expect(fn).toMatch(/adapter == 'diagnosis'/);
      expect(fn).not.toMatch(/'fault_diagnosis'/);
    });

    it('页面里每一处诊断专有渲染都挂在 isDiagnosis 上（欢迎区与引导入口各一处）', () => {
      expect(PAGE).toMatch(/v-if="isDiagnosis && guideVisible && messages\.length == 0"/);
      expect(PAGE).toMatch(/v-if="isDiagnosis && guideVisible"/);
    });

    it('检测器有牙：塞一处字面量比较必须判红', () => {
      const mutated = PAGE.replace(
        'isDiagnosis.value = isDiagnosisFeature(item.featureKey)',
        "isDiagnosis.value = item.featureKey == 'fault_diagnosis'"
      );
      expect(mutated).not.toBe(PAGE);
      expect(hasFeatureKeyLiteralCompare(mutated)).toBe(true);
    });
  });

  describe('② 留白判定只有一个判据（票面：取不到 / health 为否 ⇒ 整块不渲染）', () => {
    it('health.ok !== true 或 intents 失败 ⇒ guideVisible=false，且把字典一起丢掉', () => {
      const fn = fnBody(COMPOSABLE, 'async function loadIntents');
      // 两条失败路径都必须落到「整块留白 + 清空字典」这一处
      expect(fn).toMatch(/if \(!healthOk \|\| !intentsOk\) \{/);
      expect(fn).toMatch(/brands\.value = \[\]/);
      expect(fn).toMatch(/faq\.value = \[\]/);
      expect(fn).toMatch(/guideVisible\.value = false/);
    });

    it('进页拉一次、不轮询（没有 setInterval / 定时重拉）', () => {
      expect(COMPOSABLE).not.toMatch(/setInterval/);
      expect(PAGE).not.toMatch(/setInterval/);
    });

    it('非诊断页不发引导请求（不白打四个 GET）', () => {
      const fn = fnBody(PAGE, 'onLoad((options : OnLoadOptions)');
      expect(fn).toMatch(/if \(isDiagnosis\.value\) \{/);
      expect(fn).toMatch(/loadDiagnosisIntents\(\)/);
    });

    it('检测器有牙：把留白判定改成恒真必须判红', () => {
      const mutated = COMPOSABLE.replace('if (!healthOk || !intentsOk) {', 'if (false) {');
      expect(mutated).not.toBe(COMPOSABLE);
      const fn = fnBody(mutated, 'async function loadIntents');
      expect(fn).not.toMatch(/if \(!healthOk \|\| !intentsOk\) \{/);
    });
  });

  describe('③ 组好句进输入框、不自动发送（票面：可编辑后再发送）', () => {
    it('「填入输入框」只写草稿并关面板，不调 sendInput', () => {
      const fn = fnBody(PAGE, 'function onConfirmSentence');
      expect(fn).toMatch(/draftText\.value = text/);
      expect(fn).toMatch(/closeGuide\(\)/);
      expect(fn).not.toMatch(/sendInput\(/);
    });

    it('草稿经 AiChatInput 的 draft 属性送到输入框（不是直接改 useAiChat 的 inputText）', () => {
      expect(PAGE).toMatch(/<AiChatInput :draft="draftText"/);
      expect(INPUT).toMatch(/draft\?: string/);
      // 判据是「草稿变化会写进输入框」这条语义，不钉具体的取源写法（typed computed 是 ④c 逼出来的）
      expect(INPUT).toMatch(/const draftText = computed<string>/);
      expect(INPUT).toMatch(/watch\(draftText, \(v : string\) => \{/);
    });

    it('发出去之后草稿作废（否则同一句会被反复塞回输入框）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/draftText\.value = ''/);
    });

    it('检测器有牙：把「填入输入框」改成直接发送必须判红', () => {
      const mutated = PAGE.replace('draftText.value = text', 'sendInput(featureTitle.value, buildBody, [], currentFeatureKey.value)');
      expect(mutated).not.toBe(PAGE);
      const fn = fnBody(mutated, 'function onConfirmSentence');
      expect(fn).toMatch(/sendInput\(/);
    });
  });

  describe('④ 取数只有四个 GET（没有自造端点）', () => {
    it('四个封装都指向 /ai-assistant/diagnosis/*，且 health 的失败被当作业务态', () => {
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/health'/);
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/intents'/);
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/systems'/);
      expect(API).toMatch(/'\/ai-assistant\/diagnosis\/faults'/);
      // 全仓不再出现旧的四个只读端点（已随 #2237 硬删，接回来必 404）
      expect(API).not.toMatch(/diagnosis\/(brands|models|fault-codes|manual)/);
    });

    it('集合恒非空数组：解析层空集合就是空数组，不抛给调用方', () => {
      const fn = fnBody(API, 'function asObjectArray');
      expect(fn).toMatch(/if \(raw == null\) return \[\] as Array<UTSJSONObject>/);
    });

    it('型号档第二层走 faults 根层，系统档走 systems（判据来自纯函数）', () => {
      const fn = fnBody(STATE, 'function secondSourceOf');
      expect(fn).toMatch(/'faults-root'/);
      expect(fn).toMatch(/'systems'/);
      const load = fnBody(COMPOSABLE, 'async function loadSecond');
      expect(load).toMatch(/if \(src == 'faults-root'\) \{/);
      expect(load).toMatch(/getDiagnosisFaultsApi\(brandId, ''\)/);
      expect(load).toMatch(/getDiagnosisSystemsApi\(brandId\)/);
    });

    it('检测器有牙：把健康探测删掉必须判红（否则会画出点不动的品牌卡）', () => {
      const mutated = COMPOSABLE.replace('const h = await getDiagnosisHealthApi()', 'const h = { ok: true, app_configured: true } as DiagnosisHealth');
      expect(mutated).not.toBe(COMPOSABLE);
      expect(fnBody(mutated, 'async function loadIntents')).not.toMatch(/getDiagnosisHealthApi\(\)/);
    });
  });

  describe('⑤ 面板是浮层不是新页面（不碰打包面门）', () => {
    it('pages.json 没有新增诊断引导页', () => {
      const pages = JSON.parse(read('pages.json'));
      const flat = JSON.stringify(pages);
      expect(flat).not.toMatch(/ai-chat-diagnosis-guide/);
    });

    it('面板组件绝对定位盖住页面（与既有 pro-sheet 同形态），且没有 transition / gap 之类原生端不支持的能力', () => {
      const style = GUIDE_UI.slice(GUIDE_UI.indexOf('<style'));
      expect(style).toMatch(/lang="scss"/);
      expect(style).toMatch(/position: absolute/);
      expect(style).not.toMatch(/\bgap\s*:/);
      expect(style).not.toMatch(/\btransition\s*:/);
      expect(style).not.toMatch(/calc\(/);
    });
  });

  describe('⑥ 拼句与判档是纯函数模块（页面/组件不各写一份）', () => {
    it('组句只在 utils/diagnosisCompose.uts 里实现，页面与面板都只 import', () => {
      expect(PAGE).not.toMatch(/我的\$\{|出现故障现象：/);
      expect(GUIDE_UI).not.toMatch(/我的\$\{|出现故障现象：/);
      expect(COMPOSE).toMatch(/export function composeDiagnosisSentence/);
      expect(COMPOSE).toMatch(/export function pickerKindOf/);
    });

    it('面板的标题随档位变（型号档问维修部位，系统档问故障现象）', () => {
      expect(GUIDE_UI).toMatch(/pickerKindOf\(brand\.value\) == 'model'/);
      expect(GUIDE_UI).toMatch(/选要查的维修部位 \/ 主题/);
      expect(GUIDE_UI).toMatch(/选故障现象/);
    });
  });
});