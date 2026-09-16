/**
 * 宫格入口 → 参数化专项页 接线契约（#1040）—— 源码契约测试缝（读源文本、断言语义；
 * 与 aiProGateContract / aiChatModesContract 同款）。
 *
 * 本文件是 `#921` 版接线契约（`「故障咨询」宫格 → fault_diagnosis`）在**形态变更后**的重写。
 * 形态变更 = 移动端 ADR-0009 的 2026-09-15 修订 ②：入口由「单页 + 宫格，点击在本页带键发一轮」
 * 改为「**通用页 + 参数化专项页**，点击 = `navigateTo` 专项页并带 `featureKey`」。
 * 因此原文件里这些断言**随形态过期**，本文件替换它们：
 *   - 「其余 4 格**不带** featureKey」（#971 范围外那句）—— 现在 4 格**全带**键；
 *   - 「`onFunctionClick` 置 `pendingFeatureKey`」—— 通用页**不再持有**该状态；
 *   - 「`onInputSend` 里取走 → 清空 → 早退守卫」的有序断言 —— 该轮次边界随状态一起消失。
 *
 * 行为链现在横跨四处，本文件按这个链分段：
 *   常量表（哪 4 格、带哪些键）→ 通用页（点击 = 跳转，且**结构上**没有键状态）
 *   → 专项页（按键定位、键只进本轮请求体、不吃「用户自选模型」面）→ pages.json（页面已注册）。
 * 键集合 / 展示名 / 限免位与**后端注册表**的等值另在 `aiFeatureRegistryParityContract.test.js`
 * （交付物 5），本文件不重复。
 *
 * 断言纪律（沿用 #921 评审修正，**务必保持**）：本仓踩过的假绿是「存在性断言」——
 * ① 顺序盲（把功能改到完全失效，旧断言仍全绿）；② 越界盲（`slice(indexOf(fn))` 切到文件尾，
 * 后面别的函数里的赋值也算命中）。故：函数体一律用 `fnBody()` 限在函数体内（大括号配平），
 * 关键处的「检测器有没有牙」先用手工变形样本自证。
 *
 * 已知边界（如实声明）：契约测试只钉客户端这一半；真机门与本地编译门（移动端 `docs/adr/0008` 的
 * ① / ④）另行执行并在 PR 证据段如实记录。会话按键**分区**不在本票（见 #1041），故 ④ 组维持原样。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');

const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');
const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
const FEATURE_PAGE = read('pages/ai-assistant/ai-feature.uvue');
const CHAT = read('composables/useAiChat.uts');
const API = read('api/aiAssistant.uts');
const TYPES = read('types/ai.uts');
const PAGES_JSON = JSON.parse(read('pages.json'));

const FEATURE_PAGE_PATH = 'pages/ai-assistant/ai-feature';

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

/** 去掉模板/JS/CSS 注释 —— 「注释里提到某标识」不等于「代码里用了它」（本页新注释提到过旧状态名） */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 取常量表里某一格的声明行（按本地 key 定位，避免顺序变化导致误判）。 */
function tileLine(key) {
  const line = CONSTANTS.split('\n').find((l) => l.indexOf(`key: '${key}'`) >= 0);
  if (!line) throw new Error(`常量表缺宫格 ${key}`);
  return line;
}

/** 常量表里所有 featureKey（按出现序）。 */
function featureKeys(src) {
  const out = [];
  const re = /featureKey:\s*'([^']+)'/g;
  let m;
  while ((m = re.exec(src)) !== null) out.push(m[1]);
  return out;
}

/** 点击处理器是否「跳转专项页并带键」（判据抽成纯函数，便于对变形样本断言）。 */
function clickNavigates(src) {
  return /uni\.navigateTo\(\{ url: '\/pages\/ai-assistant\/ai-feature\?featureKey=' \+ item\.featureKey \}\)/.test(
    fnBody(src, 'function onFunctionClick')
  );
}

/** 通用页是否**结构上**没有键状态：类型属性/函数体里都不再出现该状态名。 */
function noRoundKeyState(src) {
  return !/pendingFeatureKey/.test(stripCodeComments(src));
}

describe('宫格入口 → 参数化专项页 接线（#1040）契约', () => {
  describe('① 常量表：注册表的 4 项专项功能，格格带键', () => {
    it('宫格项类型：featureKey / freePreview 均为**必填**（没有键的格子写不出来）', () => {
      const code = stripCodeComments(TYPES);
      expect(code).toMatch(/export type AIFunctionItem = \{/);
      expect(code).toMatch(/featureKey : string/);
      expect(code).toMatch(/freePreview : boolean/);
      // 旧形态的可选声明是被删掉的 —— 留着就等于给「伪入口」留了类型口子
      // （注释里会提到旧写法，故按去注释后的代码判）
      expect(code).not.toMatch(/featureKey \?: string/);
    });

    it('恰好 4 项，且每一项都带后端功能键', () => {
      const keys = featureKeys(CONSTANTS);
      expect(keys.length).toBe(4);
      expect(new Set(keys).size).toBe(4); // 无重复
      for (const k of keys) expect(k.length).toBeGreaterThan(0);
    });

    it('「故障代码查询」格已删除（本地 key 与标题双双不得残留）', () => {
      // 常量文件头注释里必然提到这个名字（解释为什么删）⇒ 按去注释后的代码判
      const code = stripCodeComments(CONSTANTS);
      expect(code).not.toMatch(/'fault-code'/);
      expect(code).not.toMatch(/故障代码查询/);
      // 宫格因此只剩 4 项（旧的 5 项 = 4 真功能 + 1 伪入口）。
      // 计数要认「宫格项」的形态（含 title + featureKey）——`RIGHT_MENU_ITEMS` 也是 `{ key: '…' }` 开头，
      // 用裸 `{ key: '` 计数会把它一起数进来（本文件自测踩过：数出 8）。
      expect((CONSTANTS.match(/^\s*\{ key: '[^']+', title: '[^']+',.*featureKey: '/gm) || []).length).toBe(4);
    });

    it('展示名跟注册表：「故障咨询」→「智能维修诊断」', () => {
      expect(tileLine('fault-consult')).toMatch(/title: '智能维修诊断'/);
      expect(CONSTANTS).not.toMatch(/title: '故障咨询'/);
    });

    it('「智能维修诊断」仍接既有键 fault_diagnosis，且带限免声明位', () => {
      const line = tileLine('fault-consult');
      expect(line).toMatch(/featureKey: 'fault_diagnosis'/);
      expect(line).toMatch(/freePreview: true/);
      // 本地 key 是 UI 标识（图标/引用），不因展示名变更而改
      expect(line).toMatch(/key: 'fault-consult'/);
    });

    it('其余 3 格各接自己的键、且不限免（注册表 freePreview 的镜像）', () => {
      expect(tileLine('drawing')).toMatch(/featureKey: 'drawing_recognition'/);
      expect(tileLine('exercise')).toMatch(/featureKey: 'exercise_solving'/);
      expect(tileLine('knowledge')).toMatch(/featureKey: 'maintenance_knowledge'/);
      for (const k of ['drawing', 'exercise', 'knowledge']) {
        expect(tileLine(k)).toMatch(/freePreview: false/);
      }
    });
  });

  describe('② 点击 = 跳转专项页（不再「在本页带键发一轮」）', () => {
    it('onFunctionClick 用 navigateTo 跳专项页并带 featureKey', () => {
      expect(clickNavigates(PAGE)).toBe(true);
    });

    it('点击不再发预设话、不再置任何键状态', () => {
      const fn = fnBody(PAGE, 'function onFunctionClick');
      expect(fn).not.toMatch(/sendText\(/);
      expect(fn).not.toMatch(/item\.prompt/);
      expect(fn).not.toMatch(/pendingFeatureKey/);
    });

    it('把跳转换成「发一轮」的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = PAGE.replace(
        "uni.navigateTo({ url: '/pages/ai-assistant/ai-feature?featureKey=' + item.featureKey })",
        "sendText(item.title)"
      );
      expect(clickNavigates(broken)).toBe(false);
      expect(fnBody(broken, 'function onFunctionClick')).toMatch(/sendText\(/);
    });

    it('专项页已在 pages.json 注册（没注册 = 点击报「页面不存在」）', () => {
      const paths = (PAGES_JSON.pages || []).map((p) => p.path);
      expect(paths).toContain(FEATURE_PAGE_PATH);
      // 注册的是**一个**参数化页面，不是 4 个页面文件（ADR-0009 修订 ② 的页面数裁定）
      expect(paths.filter((p) => p.indexOf('pages/ai-assistant/ai-feature') === 0).length).toBe(1);
    });
  });

  describe('③ 通用页**结构上**没有键状态（#999 那类缺陷不可能再发生）', () => {
    it('通用页不再出现 pendingFeatureKey（注释里提过不算，按去注释后的代码判）', () => {
      expect(noRoundKeyState(PAGE)).toBe(true);
      // 变形样本自证：把状态加回去 ⇒ 判据变红
      expect(noRoundKeyState(PAGE.replace('<script setup lang="uts">', "<script setup lang=\"uts\">\nconst pendingFeatureKey = ref<string>('')"))).toBe(false);
    });

    it('通用页发送时功能键显式传空串（第四参恒为空 ⇒ 落通用通道）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/sendInput\(currentModelName\.value, buildBody, \[\], ''\)/);
      expect(fn).not.toMatch(/sendInput\(currentModelName\.value, buildBody, \[\], featureKey\)/);
    });

    it('通用页的 buildBody 不再有「带键就早退」的分支（那是专项页的形态）', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).not.toMatch(/featureKey\.length > 0/);
      // 空串仍显式发送：键字段留着，语义与「不发送」等价
      expect(fn).toMatch(/params\['feature_key'\] = featureKey/);
    });

    it('「请先配置自定义模型」的前置校验回到无条件形态（通用页每一轮都适用）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/if \(currentModelSource\.value == 'custom' && customModel\.value\.trim\(\)\.length == 0\)/);
    });
  });

  describe('④ 会话归属：**不得**给会话带功能键（带了就永久不可见；分区见 #1041，本票不改）', () => {
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

  describe('⑤ 专项页：按键定位，键只进本轮请求体', () => {
    it('参数来自 onLoad 的 featureKey 查询参数，并按键命中手写清单', () => {
      const fn = fnBody(FEATURE_PAGE, 'onLoad((options : OnLoadOptions)');
      expect(fn).toMatch(/options\['featureKey'\]/);
      expect(fn).toMatch(/findFunctionItem\(/);
    });

    it('未知/缺失的键 fail-closed（不进页）—— 不靠「悄悄退化成通用对话」复活伪入口', () => {
      const fn = fnBody(FEATURE_PAGE, 'onLoad((options : OnLoadOptions)');
      expect(fn).toMatch(/if \(item == null\)/);
      expect(fn).toMatch(/uni\.navigateBack\(\)/);
    });

    it('buildBody 把功能键落进请求体', () => {
      const fn = fnBody(FEATURE_PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).toMatch(/params\['feature_key'\] = featureKey/);
    });

    it('专项通道不吃「用户自选模型」面：请求体不送 model_source / custom_* / 模型 id（#998 口径）', () => {
      const fn = fnBody(FEATURE_PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).not.toMatch(/model_source/);
      expect(fn).not.toMatch(/custom_api_key/);
      expect(fn).not.toMatch(/config_id/);
      expect(fn).not.toMatch(/user_model_id/);
      // 密钥绝不能出现在请求体里（页面同理；注释里解释过该字段名，按去注释后的代码判）
      expect(stripCodeComments(FEATURE_PAGE)).not.toMatch(/custom_api_key/);
    });

    it('发送用页面级常量键（没有轮次状态可漏）', () => {
      const fn = fnBody(FEATURE_PAGE, 'function onInputSend');
      expect(fn).toMatch(/sendInput\(featureTitle\.value, buildBody, \[\], currentFeatureKey\.value\)/);
      expect(FEATURE_PAGE).toMatch(/const currentFeatureKey = ref<string>\(''\)/);
    });

    it('专项页不加载模型列表、无模型前置守卫（模型由管理端单绑定解析）', () => {
      const fn = fnBody(FEATURE_PAGE, 'function onInputSend');
      expect(fn).not.toMatch(/暂无可用的 AI 模型/);
      expect(FEATURE_PAGE).not.toMatch(/getAiModelsApi/);
    });

    it('删掉 fail-closed 分支的变形样本会被检出（防检测器空跑假绿）', () => {
      const broken = FEATURE_PAGE.replace(/if \(item == null\) \{[\s\S]*?\n\t\t\}/, 'if (false) { }');
      const fn = fnBody(broken, 'onLoad((options : OnLoadOptions)');
      expect(fn).not.toMatch(/if \(item == null\)/);
    });
  });

  describe('⑥ sendInput 显式透传 featureKey（UTS 无可选参数默认值机制）。本组不动', () => {
    it('签名与实现、返回对象三处一致（否则 guard 规则 J「实参少于必选形参」会红）', () => {
      expect(CHAT).toMatch(/sendInput : \(sessionTag : string, buildBody : ChatBodyBuilder, images : string\[\], featureKey : string\) => void/);
      expect(CHAT).toMatch(/function sendInput\(sessionTag : string, buildBody : ChatBodyBuilder, images : string\[\], featureKey : string\) : void/);
      expect(CHAT).toMatch(/doStreamChat\(sessionTag, buildBody, featureKey\)/);
    });
  });

  describe('⑦ 不改的东西（防后续会话加戏）', () => {
    it('不重新注册 fault_consult（根仓库 ADR-0032 决策 3 已下线，接回需先推翻该条）', () => {
      expect(CONSTANTS).not.toMatch(/'fault_consult'/);
      expect(PAGE).not.toMatch(/'fault_consult'/);
    });

    it('不引入「普通/专家」双绑定：移动端仍直接挑 admin 模型（移动端 ADR 0009 决定 3）', () => {
      expect(PAGE).not.toMatch(/mode\s*:\s*'(normal|expert)'/);
    });

    it('专项页不含用户上传图片（ADR-0009 修订 ② 已排除；#1042 的检索栏也不在本票）', () => {
      expect(FEATURE_PAGE).not.toMatch(/chooseImage|uploadAiImageApi|showImageButton/);
    });
  });
});
