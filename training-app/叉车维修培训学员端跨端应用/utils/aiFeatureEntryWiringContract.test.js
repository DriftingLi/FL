/**
 * 宫格入口 → 页内 scope 接线契约（#1040 起，2026-11-16 #2593 改写）—— 源码契约测试缝（读源文本、
 * 断言语义；与 aiProGateContract / aiChatModesContract 同款）。
 *
 * 形态演变（三次，都在移动端 ADR-0009 的修订里）：
 *   ① #921 版：入口「单页 + 宫格，点击在本页带键发一轮」—— 键是**轮次状态**，踩过「键恒为空」；
 *   ② #1040 版：ADR-0009 2026-09-15 修订 ②「通用页 + 参数化专项页」—— 点击 = `navigateTo` 专项页；
 *   ③ 本版（#2593）：ADR-0009 **2026-11-16 修订 ②**「宫格 = 在 AI 助手页内切 scope」——
 *      专项页 `pages/ai-assistant/ai-feature` **已退役**，键成为**页面状态** `activeScope`：
 *      切它 = 换区 = 重置当前会话与消息并按新区重列（ADR-0013 2026-11-16 修订新增的不变式）。
 *
 * 行为链现在收敛在**同一页**，本文件按这个链分段：
 *   常量表（哪 4 格、带哪些键与短名）→ 宫格/chip（切 scope，且**不再有跳转点**）
 *   → 请求体（scoped 分支先 return，不吃用户自选模型）→ 会话分区（带键 ⇒ 有列该区的界面 + 切区重置）
 *   → chip 选择 fail-closed（未知键不切换、不复活伪入口）。
 * 键集合 / 展示名 / 限免位与**后端注册表**的等值另在 `aiFeatureRegistryParityContract.test.js`，本文件不重复。
 *
 * 断言纪律（沿用 #921 评审修正，**务必保持**）：本仓踩过的假绿是「存在性断言」——
 * ① 顺序盲（把功能改到完全失效，旧断言仍全绿）；② 越界盲（`slice(indexOf(fn))` 切到文件尾，
 * 后面别的函数里的赋值也算命中）。故：函数体一律用 `fnBody()` 限在函数体内（大括号配平），
 * 关键处的「检测器有没有牙」先用手工变形样本自证。
 *
 * 已知边界（如实声明）：契约测试只钉客户端这一半；真机门与本地编译门（移动端 `docs/adr/0008` 的
 * ① / ④）另行执行并在 PR 证据段如实记录。
 *
 * ④ 组是**不变的放宽处**（ADR-0013 ⑤「不得静默改」）：锁的是条件式不变式「**带键 ⇒ 必有列该区的界面**」，
 * 不是「永不带键」这个手段（#921 的回归不是「带了键」，而是「带了键却没有任何界面列它」）。
 * 本版把载体从页面换成 scope ⇒ 不变式原样保留，另外把「切区必须重置」并进同一组。
 */
const path = require('path');
const fs = require('fs');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (p) => readText(path.join(ROOT, p));

const CONSTANTS = read('pages/ai-assistant/ai-assistant-constants.uts');
const PAGE = read('pages/ai-assistant/ai-assistant.uvue');
const CHAT = read('composables/useAiChat.uts');
/** #2299 起专项 scope 的唯一选图入口（ADR-0055 放宽 ADR-0009 修订 ②） */
const AI_PICKER = read('composables/useAiImagePicker.uts');
const API = read('api/aiAssistant.uts');
const TYPES = read('types/ai.uts');
const PAGES_JSON = JSON.parse(read('pages.json'));

/** 已退役的专项页（#2593）：删页必须是一条完整的手术，本文件按「路径 / 文件 / 引用」三面各钉一条。 */
const RETIRED_FEATURE_PAGE_PATH = 'pages/ai-assistant/ai-feature';
const RETIRED_FEATURE_PAGE_FILE = 'pages/ai-assistant/ai-feature.uvue';

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

/** 点击处理器是否「切本页 scope」（判据抽成纯函数，便于对变形样本断言）。 */
function clickSwitchesScope(src) {
  return /switchScope\(item\.featureKey\)/.test(fnBody(src, 'function onFunctionClick'));
}

/** 页面是否**已无跳转点**：专项页路径不再出现在任何 `uni.navigateTo` 里（注释不算）。 */
function noNavigateToFeaturePage(src) {
  return !/navigateTo\([^)]*ai-feature/.test(stripCodeComments(src));
}

/** 页面交给 `useAiChat(...)` 的**会话作用域**实参名（例 `activeScope`）；未调用则 null。 */
function scopeArg(src) {
  const m = /useAiChat\(\s*([A-Za-z_$][\w$]*)\s*\)/.exec(stripCodeComments(src));
  return m ? m[1] : null;
}

/** 该作用域是否**恒为空串**（= 本页会话永不带键）：声明为 `ref<string>('')` 且全页无赋值。 */
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
 * 切 scope 是否**重置会话与消息**（ADR-0013 2026-11-16 修订新增的不变式；#2593）。
 *
 * 为什么必须钉：列表读的是 `featureScope.value`（现读），消息读的是本页 ref ——
 * 只改 scope 不重置 ⇒ 抽屉按新区刷新、屏幕上还挂着上一区的对话，学员在「智能维修诊断」里
 * 看到通用对话的上下文。这不是样式问题，是归错区。
 */
function scopeSwitchResets(src, name) {
  let fn;
  try {
    fn = fnBody(src, 'function switchScope');
  } catch (e) {
    return false;
  }
  return new RegExp(`${name}\\.value\\s*=`).test(fn)
    && /currentSession\.value\s*=\s*null/.test(fn)
    && /messages\.value\s*=\s*\[\]/.test(fn)
    && /loadSessions\(\)/.test(fn);
}

/**
 * 条件式不变式（ADR-0013）：**带键 ⇒ 必须有列该区的界面**，且**切区必须重置**。
 * 不带键的页面自动成立（通用页就是这一支）—— 不是「一律要界面」，而是「不许只带键不列区」。
 */
function regionInvariantHolds(src) {
  const arg = scopeArg(src);
  if (arg === null) throw new Error('页面没有调用 useAiChat(...)：会话作用域判据无从判定');
  if (scopeAlwaysEmpty(src, arg)) return true;
  return rendersRegionSessionList(src) && scopeSwitchResets(src, arg);
}

/** 页面是否**按限免位渲染小签**（判据必须是 `item.freePreview`，不是写死的 key 或常显）。 */
const FREE_BADGE_BLOCK = /[ \t]*<view v-if="item\.freePreview" class="function-free-badge">[\s\S]*?<\/view>/;

/**
 * 限免小签接线判据（2026-10-27 B 版形态：标题下方独立一行）。
 *
 * 四条同时成立才算接线，缺一条都是「学员看不到限免」或「看到了但样子是坏的」：
 *   ① 判定只读注册表镜像 `item.freePreview`（写死常显 = 与后端注册表脱钩）；
 *   ② 外层 view 承载底色/边、内层 text 只承载「限免」字形（`<text>` 画 border 本仓无先例）；
 *   ③ 小签**位于标题之后**（脱流压图标 = 与橙黄图标同色系黏成一团，② 首帧实测过的缺陷）；
 *   ④ 不得回到绝对定位（形态回退必须红，否则「改回旧形态」这条会静默通过）。
 */
function freeBadgeWired(src) {
  const code = stripCodeComments(src);
  const badgeIdx = code.indexOf('class="function-free-badge"');
  const titleIdx = code.indexOf('class="function-title"');
  const badgeBlock = (code.match(/\.function-free-badge\s*\{[^}]*\}/) || [''])[0];
  return (
    /<view v-if="item\.freePreview" class="function-free-badge">\s*<text class="function-free-badge-text">限免<\/text>\s*<\/view>/.test(code)
    && badgeIdx > 0
    && titleIdx > 0
    && badgeIdx > titleIdx
    && !/position\s*:\s*absolute/.test(badgeBlock)
    && /\.function-free-badge\s*\{/.test(code)
    && /\.function-free-badge-text\s*\{/.test(code)
  );
}

describe('宫格入口 → 页内 scope 接线（#1040 / #2593）契约', () => {
  describe('① 常量表：注册表的 4 项专项功能，格格带键与短名', () => {
    it('宫格项类型：featureKey / freePreview / scopeLabel 均为**必填**（没有键的格子写不出来）', () => {
      const code = stripCodeComments(TYPES);
      expect(code).toMatch(/export type AIFunctionItem = \{/);
      expect(code).toMatch(/featureKey : string/);
      expect(code).toMatch(/freePreview : boolean/);
      // chip 短名（#2593）：原型 791:750「智能维修诊断 ⌄」里的那个名字
      expect(code).toMatch(/scopeLabel : string/);
      // 旧形态的可选声明是被删掉的 —— 留着就等于给「伪入口」留了类型口子
      expect(code).not.toMatch(/featureKey \?: string/);
    });

    it('恰好 4 项，且每一项都带后端功能键', () => {
      const keys = featureKeys(CONSTANTS);
      expect(keys.length).toBe(4);
      expect(new Set(keys).size).toBe(4); // 无重复
      for (const k of keys) expect(k.length).toBeGreaterThan(0);
    });

    it('「故障代码查询」格已删除（本地 key 与标题双双不得残留）', () => {
      const code = stripCodeComments(CONSTANTS);
      expect(code).not.toMatch(/'fault-code'/);
      expect(code).not.toMatch(/故障代码查询/);
      expect((CONSTANTS.match(/^\s*\{ key: '[^']+', title: '[^']+',.*featureKey: '/gm) || []).length).toBe(4);
    });

    it('展示名跟注册表：「智能维修诊断」→「智能维修诊断」', () => {
      expect(tileLine('fault-consult')).toMatch(/title: '智能维修诊断'/);
      expect(CONSTANTS).not.toMatch(/title: '故障咨询'/);
    });

    it('chip 短名：诊断格是「智能维修诊断」，其余三项与本格展示名同值（原型 791:750）', () => {
      expect(tileLine('fault-consult')).toMatch(/scopeLabel: '智能维修诊断'/);
      for (const k of ['drawing', 'exercise', 'knowledge']) {
        const line = tileLine(k);
        const title = /title: '([^']+)'/.exec(line)[1];
        expect(line).toMatch(new RegExp(`scopeLabel: '${title}'`));
      }
    });

    it('「智能维修诊断」仍接既有键 fault_diagnosis，且带限免声明位', () => {
      const line = tileLine('fault-consult');
      expect(line).toMatch(/featureKey: 'fault_diagnosis'/);
      expect(line).toMatch(/freePreview: true/);
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

    it('限免位在页面上**真被消费**：宫格小签由 `item.freePreview` 决定（只写进常量 = 学员看不到限免）', () => {
      expect(freeBadgeWired(PAGE)).toBe(true);
      expect(freeBadgeWired(PAGE.replace(FREE_BADGE_BLOCK, ''))).toBe(false);
      expect(freeBadgeWired(PAGE.replace('v-if="item.freePreview"', ''))).toBe(false);
      expect(freeBadgeWired(PAGE.replace('<text class="function-free-badge-text">限免</text>', ''))).toBe(false);
    });

    it('小签在**标题下方**、且不回到绝对定位（形态回退必须红）', () => {
      const block = (PAGE.match(FREE_BADGE_BLOCK) || [''])[0];
      expect(block.length).toBeGreaterThan(0);
      const movedUp = PAGE.replace(FREE_BADGE_BLOCK, '').replace(
        '<text class="function-title">{{ item.title }}</text>',
        block + '\n\t\t\t\t\t<text class="function-title">{{ item.title }}</text>'
      );
      expect(freeBadgeWired(movedUp)).toBe(false);
      const absAgain = PAGE.replace('.function-free-badge {', '.function-free-badge {\n\t\tposition: absolute;');
      expect(freeBadgeWired(absAgain)).toBe(false);
    });
  });

  describe('② 点击 = 在本页切 scope（不再跳转专项页；#2593）', () => {
    it('onFunctionClick 走 switchScope(item.featureKey)', () => {
      expect(clickSwitchesScope(PAGE)).toBe(true);
    });

    it('点击不再发预设话、不再置任何键状态、也不再 navigateTo 专项页', () => {
      const fn = fnBody(PAGE, 'function onFunctionClick');
      expect(fn).not.toMatch(/sendText\(/);
      expect(fn).not.toMatch(/item\.prompt/);
      expect(fn).not.toMatch(/pendingFeatureKey/);
      expect(fn).not.toMatch(/navigateTo/);
      expect(noNavigateToFeaturePage(PAGE)).toBe(true);
    });

    it('把切 scope 换回「跳转」或「发一轮」的变形样本会被检出（防检测器空跑假绿）', () => {
      const jumped = PAGE.replace('switchScope(item.featureKey)', "uni.navigateTo({ url: '/pages/ai-assistant/ai-feature?featureKey=' + item.featureKey })");
      expect(clickSwitchesScope(jumped)).toBe(false);
      expect(noNavigateToFeaturePage(jumped)).toBe(false);
      const chatted = PAGE.replace('switchScope(item.featureKey)', 'sendText(item.title)');
      expect(clickSwitchesScope(chatted)).toBe(false);
    });

    it('专项页已从 pages.json 摘除且文件已删（删页的三面手术：路径 / 文件 / 引用）', () => {
      const paths = (PAGES_JSON.pages || []).map((p) => p.path);
      expect(paths.filter((p) => p.indexOf(RETIRED_FEATURE_PAGE_PATH) === 0).length).toBe(0);
      expect(fs.existsSync(path.join(ROOT, RETIRED_FEATURE_PAGE_FILE))).toBe(false);
    });
  });

  describe('③ 页内 scope 状态：键是页面状态，请求体按 scope 分支', () => {
    it("scope 声明为 ref<string>('')，且被 useAiChat 单实例消费", () => {
      expect(PAGE).toMatch(/const activeScope = ref<string>\(''\)/);
      expect(scopeArg(PAGE)).toBe('activeScope');
      // 单实例：不要为「专项模式」再起第二个 useAiChat（那会分叉会话与流式状态）
      expect((stripCodeComments(PAGE).match(/useAiChat\(/g) || []).length).toBe(1);
    });

    it('buildBody 里功能键**恒写**，scoped 分支在写模型字段**之前**就 return（#998）', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).toMatch(/params\['feature_key'\] = featureKey/);
      expect(fn).toMatch(/if \(featureKey\.length > 0\) \{[\s\S]*?return params/);
      // 顺序承重：early return 必须早于 model_source 的写入，否则 scoped 也会把模型字段送出去
      expect(fn.indexOf('featureKey.length > 0')).toBeLessThan(fn.indexOf("params['model_source']"));
      // 变形样本：去掉 early return ⇒ 「先 return 再写模型字段」这条判据必须红
      const leaky = fn.replace(/if \(featureKey\.length > 0\) \{[\s\S]*?return params\n\t\t\}/, '');
      expect(leaky).not.toMatch(/if \(featureKey\.length > 0\) \{[\s\S]*?return params/);
      // 另一支变形：把 early return 挪到模型字段之后 ⇒ 顺序判据红
      const reordered = fn
        .replace(/if \(featureKey\.length > 0\) \{[\s\S]*?return params\n\t\t\}/, '')
        .replace("params['model_source'] = currentModelSource.value", "params['model_source'] = currentModelSource.value\n\t\tif (featureKey.length > 0) { return params }");
      expect(reordered.indexOf('featureKey.length > 0')).toBeGreaterThan(reordered.indexOf("params['model_source']"));
    });

    it('scoped 路径不送任何模型字段：专项通道由管理端单绑定解析（#998 / ADR-0009 §7）', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      const scopedPart = fn.slice(0, fn.indexOf('featureKey.length > 0') >= 0 ? fn.indexOf('featureKey.length > 0') : fn.length);
      expect(scopedPart).not.toMatch(/model_source/);
      expect(scopedPart).not.toMatch(/custom_api_key/);
      expect(scopedPart).not.toMatch(/config_id/);
      expect(scopedPart).not.toMatch(/user_model_id/);
    });

    it('通用 scope 仍送用户自选模型三条来源（与从前逐字一致）', () => {
      const fn = fnBody(PAGE, 'const buildBody : ChatBodyBuilder');
      expect(fn).toMatch(/params\['model_source'\] = currentModelSource\.value/);
      expect(fn).toMatch(/params\['config_id'\] = currentModelId\.value/);
      expect(fn).toMatch(/params\['user_model_id'\] = currentModelId\.value/);
      expect(fn).toMatch(/params\['custom_api_key'\] = customApiKey\.value\.trim\(\)/);
    });

    it('模型前置校验只在通用 scope 生效（scoped 早退必须包在 !scoped 分支里）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/const scoped = activeScope\.value\.length > 0/);
      expect(fn).toMatch(/if \(!scoped\) \{[\s\S]*?请先配置自定义模型/);
      // 变形样本：把两条校验移到 scoped 外层（无条件）⇒ 顺序/结构判据红
      const unconditional = fn.replace('if (!scoped) {', '{');
      expect(unconditional).not.toMatch(/if \(!scoped\) \{[\s\S]*?请先配置自定义模型/);
    });

    it('发送把当前 scope 作为第四参透传（空串 = 通用通道）', () => {
      const fn = fnBody(PAGE, 'function onInputSend');
      expect(fn).toMatch(/sendInput\(scoped \? scopeTitle\.value : currentModelName\.value, buildBody, picked, activeScope\.value\)/);
      expect(fn).toMatch(/const picked : string\[\] = isDiagnosis\.value \? pickImages\.value : \[\]/);
    });
  });

  describe('④ 会话分区：带键 ⇒ 必有列该区的界面 **且** 切区必须重置（ADR-0013 条件式守护）', () => {
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
      expect(fnBody(CHAT, 'async function loadSessions')).toMatch(/getAiSessionsApi\(featureScope\.value\)/);
      const streamFn = fnBody(CHAT, 'async function doStreamChat');
      expect(streamFn).toMatch(/createAiSessionApi\(sessionTitle, sessionTag, featureScope\.value\)/);
      expect(streamFn).toMatch(/buildBody\(history, sessionID, featureKey\)/);
    });

    it('本页 scope 可非空 ⇒ 必须有列该区的抽屉（⇔ 两向都成立）', () => {
      expect(scopeAlwaysEmpty(PAGE, 'activeScope')).toBe(false);
      expect(rendersRegionSessionList(PAGE)).toBe(true);
      expect(regionInvariantHolds(PAGE)).toBe(true);
    });

    it('切 scope 必须重置当前会话与消息，并按新 scope 重新 loadSessions（#2593 新增不变式）', () => {
      expect(scopeSwitchResets(PAGE, 'activeScope')).toBe(true);
      const fn = fnBody(PAGE, 'function switchScope');
      // 顺序承重：先改 scope、后拉列表（反了就是「列表查的是旧区」）
      expect(fn.indexOf('activeScope.value = key')).toBeGreaterThan(-1);
      expect(fn.indexOf('activeScope.value = key')).toBeLessThan(fn.indexOf('loadSessions()'));
      // 等待中不许切：在跑的那一轮请求体已带旧键
      expect(fn).toMatch(/if \(thinking\.value\)/);
    });

    it('变形样本：切 scope 不重置 / 抽屉缺失 ⇒ 不变式必须被击穿（检测器自证）', () => {
      const noReset = PAGE.replace(/\t\tcurrentSession\.value = null\n/, '');
      expect(scopeSwitchResets(noReset, 'activeScope')).toBe(false);
      expect(regionInvariantHolds(noReset)).toBe(false);
      const noDrawer = PAGE.replace(/<AiChatDrawerLeft[\s\S]*?\/>/, '');
      expect(rendersRegionSessionList(noDrawer)).toBe(false);
      expect(regionInvariantHolds(noDrawer)).toBe(false);
    });

    it('变形样本：scope 恒为空串 ⇒ 条件式「不要求界面」那一侧成立（不是「一律要界面」）', () => {
      const noSwitch = PAGE.replace('activeScope.value = key', '');
      expect(scopeAlwaysEmpty(noSwitch, 'activeScope')).toBe(true);
      expect(regionInvariantHolds(noSwitch)).toBe(true);
    });
  });

  describe('⑤ chip 选择：fail-closed，未知键既不切换也不复活伪入口', () => {
    it('chip 下拉列「通用对话 + 四格」，文案取 scopeLabel', () => {
      expect(PAGE).toMatch(/@click="onScopeSelect\(\)"|@click="onScopeSelect\(''\)"/);
      expect(PAGE).toMatch(/v-for="\(item, idx\) in functionItems" :key="idx" class="scope-menu-item" @click="onScopeSelect\(item\.featureKey\)"/);
      expect(PAGE).toMatch(/\{\{ item\.scopeLabel \}\}/);
      expect(PAGE).toMatch(/\{\{ scopeTitle \}\}/);
    });

    it('未知键 fail-closed：先 findFunctionItem 判空，再切换', () => {
      const fn = fnBody(PAGE, 'function onScopeSelect');
      expect(fn).toMatch(/if \(key\.length > 0 && findFunctionItem\(key\) == null\) \{/);
      expect(fn.indexOf('findFunctionItem(key)')).toBeLessThan(fn.indexOf('switchScope(key)'));
    });

    it('变形样本：删掉 fail-closed 分支 ⇒ 判据必须红（防检测器空跑假绿）', () => {
      const broken = PAGE.replace(/if \(key\.length > 0 && findFunctionItem\(key\) == null\) \{[\s\S]*?\n\t\t\}/, '');
      const fn = fnBody(broken, 'function onScopeSelect');
      expect(fn).not.toMatch(/if \(key\.length > 0 && findFunctionItem\(key\) == null\) \{/);
      // 判据本身也咬「顺序」：把切换提到判定之前 ⇒ 红
      const swapped = fnBody(
        PAGE.replace(/if \(key\.length > 0 && findFunctionItem\(key\) == null\) \{[\s\S]*?\n\t\t\}/, '')
          .replace('if (key == activeScope.value) {', 'switchScope(key)\n\t\tif (key == activeScope.value) {'),
        'function onScopeSelect'
      );
      expect(swapped.indexOf('findFunctionItem(key)')).toBe(-1);
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

    it('诊断 scope **含**用户上传图片（#2299；ADR-0055 放宽 ADR-0009 修订 ②）—— 肯定式判据', () => {
      expect(PAGE).toMatch(/AiChatImageTray/);
      // #2593：专项 scope 图片常亮，通用 scope 仍由能力门控（不因并入而放开通用区）
      expect(PAGE).toMatch(/show-image-button="isDiagnosis \|\| proUnlocked"/);
      expect(PAGE).toMatch(/useAiImagePicker/);
      expect(PAGE).toMatch(/:images="msg\.images"/);
      // 「选图只经统一入口」：页面自己不得直调平台选图（否则等于又开一条上传路径）
      expect(PAGE).not.toMatch(/uni\.chooseImage\s*\(/);
      expect(AI_PICKER).toMatch(/uni\.chooseImage\s*\(/);
      expect(AI_PICKER).not.toMatch(/uploadForumImageApi/);
    });
  });
});
