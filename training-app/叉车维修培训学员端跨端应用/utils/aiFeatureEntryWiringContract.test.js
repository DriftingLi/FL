/**
 * 「故障咨询」宫格 → 后端功能键 fault_diagnosis 契约测试（#921）—— 源码契约测试缝
 * （读源文本、断言语义；与 aiProGateContract / aiChatModesContract 同款）。
 *
 * 为什么需要它：`pages/ai-assistant/ai-assistant.uvue` 无法在 jest import（uvue），而本票的
 * 行为链横跨三处：常量表（哪一格带后端功能键）→ 请求体（键要进 `POST /ai-assistant/chat`）
 * → 轮次边界（键只属于宫格触发的那一轮，且**不得**从发送入口的早退分支漏出去）。
 *
 * 背景（诊断结论，2026-09-13）：根仓库 `ADR-0032` 决策 3 已把 `fault_consult` / `fault_code_query`
 * 下线，替代物原文写明「同一用户入口（智能维修诊断）」⇒ 这一格接 `fault_diagnosis` 是兑现
 * 既有决定，**零后端改动**（`StreamChatReq.FeatureKey` 后端早已支持）。
 *
 * 断言纪律（#921 评审修正，重要）：本文件早期版本的「一轮即消」断言是**存在性**断言，
 * 有两条已知假绿路径，均已用**变异实验**证明并修掉——
 *   ① 顺序盲：把 `onInputSend` 改成「先清后读」会让功能**完全失效**（每轮都送空键），
 *      旧断言仍全绿 ⇒ 现在断言**有序序列**（indexOf 单调递增），而非逐条存在；
 *      （#999 实锤：早期版本的断言本身就把「先清后读」写成期望顺序，等于钉住了这个失效形态；
 *       现改为「取走 → 清空」，见 ③ 的说明。）
 *   ② 越界盲：`PAGE.slice(indexOf(fn))` 一直切到文件尾，后面别的函数里的赋值也算命中
 *      ⇒ 现在用 `fnBody()` 把切片**限在函数体内**（大括号配平，仿 aiChatTrimContract）。
 * 同理，**不得**靠「某行存在」代替「顺序正确」——这类断言看不出早退分支漏键。
 *
 * 已知边界（如实声明）：本票不含后端改动，也不含真机取证 —— 契约测试只能钉客户端这一半；
 * 真机门与本地编译门（移动端 `docs/adr/0008` 的 ① / ④）另行执行并在 PR 证据段如实记录。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');

const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');
const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
const CHAT = read('composables/useAiChat.uts');
const API = read('api/aiAssistant.uts');
const TYPES = read('types/ai.uts');

/** 取某个函数/箭头函数体（大括号配平，切到闭合处为止）——防止切片越界到文件尾造成假绿。 */
function fnBody(src, sig) {
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

/** 取常量表里某一格的声明行（按 key 定位，避免顺序变化导致误判）。 */
function tileLine(key) {
  const line = CONSTANTS.split('\n').find((l) => l.indexOf(`key: '${key}'`) >= 0);
  if (!line) throw new Error(`常量表缺宫格 ${key}`);
  return line;
}

/** 断言若干片段在源码里**按给定顺序**出现（有序，而非各自存在）。 */
function expectOrdered(src, parts) {
  const idx = parts.map((p) => {
    const i = src.indexOf(p);
    expect(i).toBeGreaterThanOrEqual(0); // 先给「缺片段」一个明确失败，再判顺序
    return i;
  });
  for (let i = 1; i < idx.length; i++) {
    if (!(idx[i] > idx[i - 1])) {
      throw new Error(`顺序不符：'${parts[i]}' 应出现在 '${parts[i - 1]}' 之后\n实际下标 ${JSON.stringify(idx)}`);
    }
  }
}

describe('「故障咨询」入口 → fault_diagnosis 接线（#921）契约', () => {
  describe('① 常量表：只有「故障咨询」这一格带后端功能键', () => {
    it('宫格项类型有可选 featureKey（后端功能键，与本地 key 是两回事）', () => {
      expect(TYPES).toMatch(/export type AIFunctionItem = \{/);
      expect(TYPES).toMatch(/featureKey \?: string/);
    });

    it('「故障咨询」带 featureKey = fault_diagnosis，且本地 key 不变', () => {
      const line = tileLine('fault-consult');
      expect(line).toMatch(/featureKey: 'fault_diagnosis'/);
      // 本地 key 是 UI 标识（图标/埋点），不因接线而改 —— 改它等于断掉既有引用
      expect(line).toMatch(/key: 'fault-consult'/);
    });

    it('其余 4 格**不带** featureKey（范围外，见 #971，不在本票顺手加戏）', () => {
      for (const key of ['fault-code', 'drawing', 'exercise', 'knowledge']) {
        expect(tileLine(key)).not.toMatch(/featureKey/);
      }
    });

    it('常量里的键在**后端注册表**里真实存在，且是诊断适配器（防后端改名/下线后静默退化）', () => {
      // 为什么值得跨仓校验：未注册的键**不报错** —— 适配器回退通用大模型通道、featureChatKeys
      // 不命中，于是「键写错」表现为「悄悄变成通用对话 + 照常计费」。ADR-0032 决策 3 干掉的
      // fault_consult / fault_code_query 就是这么消失的。
      const registry = read('../../backend/internal/service/ai_feature_registry.go');
      expect(registry).toMatch(/FeatureFaultDiagnosis\s*=\s*"fault_diagnosis"/);
      expect(registry).toMatch(/name: FeatureFaultDiagnosis, label: "智能维修诊断".*adapter: aiAdapterDiagnosis/);
    });
  });

  describe('② 请求体：功能键进 /ai-assistant/chat', () => {
    it('buildBody 契约带第三参 featureKey（由调用方按轮次提供，不从闭包偷读）', () => {
      expect(CHAT).toMatch(
        /export type ChatBodyBuilder = \(history : AiChatMessage\[\], sessionID : number, featureKey : string\) => UTSJSONObject/
      );
    });

    it('页面 buildBody 把 featureKey 落进请求体', () => {
      expect(PAGE).toMatch(/const buildBody : ChatBodyBuilder = \(history : AiChatMessage\[\], sessionID : number, featureKey : string\)/);
      expect(fnBody(PAGE, 'const buildBody : ChatBodyBuilder')).toMatch(/params\['feature_key'\] = featureKey/);
    });

    it('功能键由宫格点击置入（未带键的格子置空，行为与从前一致）', () => {
      const fn = fnBody(PAGE, 'function onFunctionClick');
      expect(fn).toMatch(/pendingFeatureKey\.value = item\.featureKey \?\? ''/);
    });
  });

  describe('③ 轮次边界：键只属于那一轮，且不从早退分支漏出（#921 评审修正）', () => {
    it('onInputSend 里「取走 → 清空」**先于**两条模型校验的早退 return', () => {
      // 变异实验证明过的假绿路径：把这里改成「先清后取」或把清空挪到早退分支之后，
      // 功能会静默失效／键会泄漏，而**存在性**断言全都看不出来。故断言**有序序列**：
      // 取走 → 清空 → 早退守卫 → 发送。
      //
      // #999 真机实测（2026-09-15）：本文件早期版本把前两步写反（要求「清空 → 取走」），
      // 于是把**功能恒失效**（每轮送空键 ⇒ feature_key 恒为 ''，专项诊断通道从未生效）
      // 当成正确顺序钉住；探针显示 `doStreamChat 入参 featureKey=`（空）。故顺序修正为
      // 「取走 → 清空」——两行都在早退守卫之前，「早退不漏键」的意图不变。
      const fn = fnBody(PAGE, 'function onInputSend');
      expectOrdered(fn, [
        'const featureKey = pendingFeatureKey.value',
        "pendingFeatureKey.value = ''",
        '暂无可用的 AI 模型',
        '请先配置自定义模型',
        'sendInput(currentModelName.value, buildBody, [], featureKey)',
      ]);
    });

    it('早退分支在清空之后：键不会残留给下一条无关提问（限免通道被误用的路径）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      const cleared = fn.indexOf("pendingFeatureKey.value = ''");
      // 两个 return 都必须晚于清空 —— 早退时不带键、也不留键
      const returns = fn.split('return').length - 1;
      expect(returns).toBeGreaterThanOrEqual(2);
      expect(cleared).toBeGreaterThan(-1);
      expect(fn.lastIndexOf('return')).toBeGreaterThan(cleared);
    });

    it('页面持有 pendingFeatureKey 状态（空串 = 通用对话）', () => {
      expect(PAGE).toMatch(/const pendingFeatureKey = ref<string>\(''\)/);
    });
  });

  describe('④ 会话归属：**不得**给会话带功能键（带了就永久不可见）', () => {
    it('建会话 API 不接受也不发送 feature_key', () => {
      expect(API).toMatch(/export function createAiSessionApi\(title : string = '新对话', modelName : string = ''\)/);
      expect(API).not.toMatch(/payload\['feature_key'\]/);
    });

    it('列表 API 不带功能键（后端按功能键分区，缺参落遗留 ai_assistant）', () => {
      // 后端 ListSessions：featureKey=="" ⇒ FeatureAIAssistant，WHERE feature_key = ?
      // ⇒ 若会话带了专项键而列表不查该区，该会话在「最近对话」里**永久不可见**（#921 首版实测回归）。
      expect(API).toMatch(/export function getAiSessionsApi\(\)/);
      const fn = fnBody(API, 'export function getAiSessionsApi()');
      expect(fn).not.toMatch(/feature_key/);
    });

    it('建会话调用传且只传两个实参（键只走本轮对话请求）', () => {
      const fn = fnBody(CHAT, 'async function doStreamChat');
      expect(fn).toMatch(/createAiSessionApi\(sessionTitle, sessionTag\)/);
      expect(fn).not.toMatch(/createAiSessionApi\(sessionTitle, sessionTag, featureKey\)/);
    });

    it('buildBody 仍按轮次拿键（会话不带键 ≠ 对话不带键）', () => {
      const fn = fnBody(CHAT, 'async function doStreamChat');
      expect(fn).toMatch(/buildBody\(history, sessionID, featureKey\)/);
    });
  });

  describe('⑤ sendInput 显式透传 featureKey（UTS 无可选参数默认值机制）', () => {
    it('签名与实现、返回对象三处一致（否则 guard 规则 J「实参少于必选形参」会红）', () => {
      expect(CHAT).toMatch(/sendInput : \(sessionTag : string, buildBody : ChatBodyBuilder, images : string\[\], featureKey : string\) => void/);
      expect(CHAT).toMatch(/function sendInput\(sessionTag : string, buildBody : ChatBodyBuilder, images : string\[\], featureKey : string\) : void/);
      expect(CHAT).toMatch(/doStreamChat\(sessionTag, buildBody, featureKey\)/);
    });
  });

  describe('⑥ 不改的东西（防后续会话加戏）', () => {
    it('不重新注册 fault_consult（根仓库 ADR-0032 决策 3 已下线，接回需先推翻该条）', () => {
      expect(CONSTANTS).not.toMatch(/'fault_consult'/);
      expect(PAGE).not.toMatch(/'fault_consult'/);
    });

    it('不引入「普通/专家」双绑定：移动端仍直接挑 admin 模型（移动端 ADR 0009 决定 3）', () => {
      expect(PAGE).not.toMatch(/mode\s*:\s*'(normal|expert)'/);
    });
  });
});
