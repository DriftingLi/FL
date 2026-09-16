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
 * ① / ④）另行执行并在 PR 证据段如实记录。
 *
 * ④ 组是**本次唯一被放宽的硬约束**（ADR-0013 ⑤「不得静默改」，实现 PR 正文须显式声明这一处）：
 * 原锁「建会话**一律不得**带 `feature_key`」，现改为**条件式不变式**「**带键 ⇒ 必有列该区的界面**」。
 * 放宽的理由见 ADR-0013 ②：#921 的回归**不是「带了键」，而是「带了键却没有任何界面列它」**
 * ⇒ 该钉死的是这条不变式，不是「永不带键」这个手段（被否备选「带键建会话但不改守护」也在此列）。
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

/** 页面交给 `useAiChat(...)` 的**会话作用域**实参名（例 `currentFeatureKey` / `featureScope`）；未调用则 null。 */
function scopeArg(src) {
  const m = /useAiChat\(\s*([A-Za-z_$][\w$]*)\s*\)/.exec(stripCodeComments(src));
  return m ? m[1] : null;
}

/** 该作用域是否**恒为空串**（= 本页会话不带键）：声明为 `ref<string>('')` 且全页无赋值。 */
function scopeAlwaysEmpty(src, name) {
  const code = stripCodeComments(src);
  return new RegExp(`const\\s+${name}\\s*=\\s*ref<string>\\(''\\)`).test(code)
    && !new RegExp(`${name}\\.value\\s*=`).test(code);
}

/** 页面是否渲染「列该区会话」的界面：抽屉 + 绑上会话列表 + 真的拉过本页列表。 */
function rendersRegionSessionList(src) {
  const code = stripCodeComments(src);
  return /<AiChatDrawerLeft/.test(code)
    && /:sessions="sessions"/.test(code)
    && /loadSessions\(\)/.test(code);
}

/**
 * 条件式不变式（ADR-0013）：**带键 ⇒ 必须有列该区的界面**。
 * 不带键的页面自动成立（通用页就是这一支）—— 这不是「一律要界面」，而是「不许只带键不列区」。
 */
function regionInvariantHolds(src) {
  const arg = scopeArg(src);
  if (arg === null) throw new Error('页面没有调用 useAiChat(...)：会话作用域判据无从判定');
  return scopeAlwaysEmpty(src, arg) ? true : rendersRegionSessionList(src);
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

  describe('④ 会话分区：**带键 ⇒ 必有列该区的界面**（ADR-0013 条件式守护；本组是本次唯一放宽处）', () => {
    it('底层 api：两处都收 featureKey，且**空串不发键**（缺参 ⇒ 后端落遗留 ai_assistant 区）', () => {
      expect(API).toMatch(/export function getAiSessionsApi\(featureKey : string\)/);
      expect(API).toMatch(/export function createAiSessionApi\(title : string, modelName : string, featureKey : string\)/);

      const listFn = fnBody(API, 'export function getAiSessionsApi(featureKey : string)');
      expect(listFn).toMatch(/if \(featureKey\.length > 0\)/);
      expect(listFn).toMatch(/params\['feature_key'\] = featureKey/);

      const createFn = fnBody(API, 'export function createAiSessionApi(title : string, modelName : string, featureKey : string)');
      expect(createFn).toMatch(/if \(featureKey\.length > 0\)/);
      expect(createFn).toMatch(/payload\['feature_key'\] = featureKey/);
    });

    it('composable 把**页面作用域**透传给 list 与 create（单点，页面不另立一套）', () => {
      // 后端 ListSessions：featureKey=="" ⇒ FeatureAIAssistant（遗留区）
      // ⇒ 建会话与列会话**必须同一个作用域**，否则会话落进「没人列」的区（#921 首版实测回归）
      expect(fnBody(CHAT, 'async function loadSessions')).toMatch(/getAiSessionsApi\(featureScope\.value\)/);
      const streamFn = fnBody(CHAT, 'async function doStreamChat');
      expect(streamFn).toMatch(/createAiSessionApi\(sessionTitle, sessionTag, featureScope\.value\)/);
      // 本轮对话的键仍按轮次走 buildBody —— 「会话归属哪一区」与「本轮用哪个键」是两件事
      expect(streamFn).toMatch(/buildBody\(history, sessionID, featureKey\)/);
    });

    it('专项页：带本区键 **且** 有列该区的界面（⇔ 两向都成立）', () => {
      expect(scopeArg(FEATURE_PAGE)).toBe('currentFeatureKey');
      expect(FEATURE_PAGE).toMatch(/currentFeatureKey\.value = item\.featureKey/);
      expect(rendersRegionSessionList(FEATURE_PAGE)).toBe(true);
      expect(regionInvariantHolds(FEATURE_PAGE)).toBe(true);
      // 顺序承重：**先定位键、后拉本区列表**（反了就是「列表查了遗留区」这种假分区）
      const loadFn = fnBody(FEATURE_PAGE, 'onLoad((options : OnLoadOptions)');
      expect(loadFn.indexOf('currentFeatureKey.value = item.featureKey')).toBeGreaterThan(-1);
      expect(loadFn.indexOf('currentFeatureKey.value = item.featureKey')).toBeLessThan(loadFn.indexOf('loadSessions()'));
    });

    it('通用页：作用域**恒为空串**（不带键 ⇒ 行为与从前逐字一致），故不受「必有界面」约束', () => {
      expect(scopeArg(PAGE)).toBe('featureScope');
      expect(scopeAlwaysEmpty(PAGE, 'featureScope')).toBe(true);
    });

    it('变形样本：专项页少了抽屉 ⇒「带键却无界面」必须被检出（检测器自证）', () => {
      const broken = FEATURE_PAGE.replace(/<AiChatDrawerLeft[\s\S]*?\/>/, '');
      expect(rendersRegionSessionList(broken)).toBe(false);
      expect(regionInvariantHolds(broken)).toBe(false); // 键还在、界面没了 ⇒ 不变式被击穿
    });

    it('变形样本：专项页不再带键 ⇒ 条件式「不要求界面」那一侧成立（不是「一律要界面」）', () => {
      const noKey = FEATURE_PAGE.replace('currentFeatureKey.value = item.featureKey', '');
      expect(scopeAlwaysEmpty(noKey, 'currentFeatureKey')).toBe(true);
      expect(regionInvariantHolds(noKey)).toBe(true);
    });

    it('变形样本：通用页被塞进任何键 ⇒「通用页不带键」必须被检出（零回归的另一半）', () => {
      const broken = PAGE.replace("const featureScope = ref<string>('')", "const featureScope = ref<string>('fault_diagnosis')");
      expect(scopeAlwaysEmpty(broken, 'featureScope')).toBe(false);
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

    it('专项页不含用户上传图片（ADR-0009 修订 ② 已排除；#1042 只加了检索/筛选，图片仍不做）', () => {
      expect(FEATURE_PAGE).not.toMatch(/chooseImage|uploadAiImageApi|showImageButton/);
    });
  });
});
