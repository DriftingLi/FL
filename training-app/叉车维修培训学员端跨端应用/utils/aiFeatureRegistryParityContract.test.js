/**
 * 移动端功能入口清单 ⟷ 后端功能注册表 **一致性契约**（ADR-0009 2026-09-15 修订 ② 的交付物 5）
 * —— 源码契约测试缝（读源文本、断言语义；与 aiProGateContract / aiFeatureEntryWiringContract 同款）。
 *
 * 锁四件事（**键集合**（原文用词）/ **展示名** / **限免位** `freePreview` / **传输适配器** `adapter`）：
 *   ① 键集合：移动端宫格项的功能键集合 == 注册表里**专项对话功能**（`aiFeatureIsChat` =
 *      管理端单绑定且声明计费）的键集合。多一个（如「故障代码查询」那种注册表里没有键的伪入口）
 *      或少一个都会红。
 *   ② 展示名：每一项的 title == 注册表该键的 `label`（「故障咨询」→「智能维修诊断」即此条）。
 *   ③ 限免位：每一项的 `freePreview` == 注册表该行的 `freePreview` 声明（缺省 false）。
 *   ④ 传输适配器：每一项的 `adapter` == 注册表该行的 `adapter`（ADR-0047 §7）。**这是「专用 UI」的
 *      唯一判据** —— 诊断筛选就按它判定（见 `aiFeatureDiagnosisFilterContract.test.js`）。手写清单若把
 *      adapter 写错，#1042 那种「诊断专有 UI」会**静默**出现在错误的页面上（或该出现的不出现），
 *      两端都不报错，只有这条测试拦得住。
 *
 * 为什么必须跨仓校验：未注册的键**不报错** —— 适配器回退通用大模型通道、`featureChatKeys`
 * 不命中，于是「键写错 / 键下线」表现为「悄悄变成通用对话 + 照常计费」。根仓库 ADR-0032 决策 3
 * 干掉的 `fault_consult` / `fault_code_query` 就是这么消失的；移动端清单仍是**手写**（ADR-0009
 * 修订 ② 的配套口径：不照抄 web 的 codegen），手写与注册表之间只有这条测试拦着。
 *
 * 格序**不在**契约内（原文锁的是「键集合」）⇒ 断言用集合 / 映射比对，不用位置比对：
 * 日后调整宫格顺序不该让本文件变红。
 *
 * 断言纪律（沿用 #921 评审修正）：每个判据都先用**注入违规的变形样本**证明检测器有效，
 * 再对真实文件断言合规 —— 否则「检测器空跑」会被读成绿。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8');

const CONSTANTS_PATH = 'pages/ai-assistant/ai-assistant-constants.uts';
const REGISTRY_PATH = '../../backend/internal/service/ai_feature_registry.go';

/**
 * 解析注册表的**功能键常量**（`FeatureXxx = "xxx"`）。
 * 只认 const 块里那种「名字 + 字符串字面量」的形态。
 */
function parseFeatureKeyConsts(src) {
  const map = {};
  const re = /^\s*(Feature[A-Za-z0-9_]+)\s*=\s*"([^"]+)"/gm;
  let m;
  while ((m = re.exec(src)) !== null) map[m[1]] = m[2];
  return map;
}

/**
 * 解析注册表的**传输适配器常量**（`aiAdapterXxx aiAdapter = "xxx"`）。
 * 适配器是「这套功能归哪套客户端 UI」的判据（ADR-0047 §7），注册表行的 `adapter:` 引用的是
 * 常量名（`aiAdapterDiagnosis`）而不是字面值，故必须先解常量再比对。
 */
function parseAdapterConsts(src) {
  const map = {};
  const re = /^\s*(aiAdapter[A-Za-z0-9_]*)\s+aiAdapter\s*=\s*"([^"]+)"/gm;
  let m;
  while ((m = re.exec(src)) !== null) map[m[1]] = m[2];
  return map;
}

/**
 * 解析注册表的**专项对话功能行**：`aiFeatureIsChat(bindingKind, billed)` 的判定原地复刻
 * （管理端单绑定 + 声明计费），故移动端清单与它的口径是同一个谓词，不是又一次手抄。
 * 返回 `{ key, label, freePreview, adapter }`，键序 = 注册表声明序（本文件不用它，仅供排查时打印）。
 */
function parseChatFeatures(registrySrc) {
  const consts = parseFeatureKeyConsts(registrySrc);
  const adapters = parseAdapterConsts(registrySrc);
  const out = [];
  for (const line of registrySrc.split('\n')) {
    const head = line.match(/^\s*\{name:\s*(Feature[A-Za-z0-9_]+),\s*label:\s*"([^"]*)"/);
    if (!head) continue;
    if (!/bindingKind:\s*bindingAdminSingle/.test(line)) continue;
    if (!/billed:\s*true/.test(line)) continue;
    const key = consts[head[1]];
    if (!key) throw new Error(`注册表行引用了未声明的功能键常量：${head[1]}`);
    // adapter 是**必填**列：漏声明就直接抛（沉默地缺一列比报错危险 —— 判据会静默失效）
    const ref = line.match(/adapter:\s*(aiAdapter[A-Za-z0-9_]*)/);
    if (!ref) throw new Error(`注册表行未声明 adapter：${head[1]}`);
    const adapter = adapters[ref[1]];
    if (!adapter) throw new Error(`注册表行引用了未声明的适配器常量：${ref[1]}`);
    out.push({ key, label: head[2], freePreview: /freePreview:\s*true/.test(line), adapter });
  }
  return out;
}

/** 去掉注释 —— 「注释里解释过为什么删掉它」不等于「代码里把它加回来了」（本仓反复踩到的假红形态） */
function stripCodeComments(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 解析移动端手写清单：`{ key: '…', title: '…', …, featureKey: '…', freePreview: <bool>, adapter: '…' },` */
function parseMobileItems(constantsSrc) {
  const out = [];
  for (const line of constantsSrc.split('\n')) {
    const m = line.match(/^\s*\{ key: '([^']+)', title: '([^']+)',.*featureKey: '([^']+)', freePreview: (true|false), adapter: '([^']+)' \},\s*$/);
    if (!m) continue;
    out.push({ localKey: m[1], title: m[2], featureKey: m[3], freePreview: m[4] === 'true', adapter: m[5] });
  }
  return out;
}

/** 三件契约的比对，返回**问题清单**（空数组 = 全等）。抽成纯函数，便于对变形样本断言。 */
function parityIssues(mobileItems, registryFeatures) {
  const issues = [];
  const mobileKeys = mobileItems.map((i) => i.featureKey);
  const registryKeys = registryFeatures.map((f) => f.key);

  // ① 键集合（顺序无关）
  const mobileSet = new Set(mobileKeys);
  const registrySet = new Set(registryKeys);
  if (mobileKeys.length !== mobileSet.size) issues.push('移动端清单有重复的 featureKey');
  for (const k of registrySet) {
    if (!mobileSet.has(k)) issues.push(`移动端清单缺少注册表功能键：${k}`);
  }
  for (const k of mobileSet) {
    if (!registrySet.has(k)) issues.push(`移动端清单多出注册表没有的功能键（伪入口）：${k}`);
  }

  // ② 展示名 + ③ 限免位 + ④ 传输适配器
  const byKey = new Map(registryFeatures.map((f) => [f.key, f]));
  for (const item of mobileItems) {
    const reg = byKey.get(item.featureKey);
    if (!reg) continue; // 多出的键已在 ① 报过，不重复报
    if (item.title !== reg.label) {
      issues.push(`展示名不一致：${item.featureKey} 移动端「${item.title}」≠ 注册表「${reg.label}」`);
    }
    if (item.freePreview !== reg.freePreview) {
      issues.push(`限免位不一致：${item.featureKey} 移动端 ${item.freePreview} ≠ 注册表 ${reg.freePreview}`);
    }
    if (item.adapter !== reg.adapter) {
      issues.push(`传输适配器不一致：${item.featureKey} 移动端「${item.adapter}」≠ 注册表「${reg.adapter}」`);
    }
  }
  return issues;
}

const CONSTANTS = read(CONSTANTS_PATH);
const REGISTRY = read(REGISTRY_PATH);
const MOBILE_ITEMS = parseMobileItems(CONSTANTS);
const CHAT_FEATURES = parseChatFeatures(REGISTRY);

describe('功能入口清单 ⟷ 后端注册表 一致性契约（ADR-0009 修订 ② · 交付物 5）', () => {
  describe('① 解析器自身有牙（先证明检测器不空跑）', () => {
    it('注册表解析到的是**专项对话功能**（4 项），且键是真实常量值而非常量名', () => {
      expect(CHAT_FEATURES.length).toBe(4);
      expect(CHAT_FEATURES.map((f) => f.key).sort()).toEqual(
        ['drawing_recognition', 'exercise_solving', 'fault_diagnosis', 'maintenance_knowledge'].sort()
      );
      for (const f of CHAT_FEATURES) {
        expect(f.key).toMatch(/^[a-z_]+$/); // 不是 `FeatureXxx`
        expect(f.label.length).toBeGreaterThan(0);
        // adapter 必须解析到**字面值**（llm / diagnosis），不是常量名 `aiAdapterXxx`
        expect(['llm', 'diagnosis']).toContain(f.adapter);
      }
      // 非对话功能（billed:false 或双模式）不得混进来
      expect(CHAT_FEATURES.map((f) => f.key)).not.toContain('grade_short_answer');
      expect(CHAT_FEATURES.map((f) => f.key)).not.toContain('ai_assistant_normal');
    });

    it('清单解析到 4 项，且键/展示名/限免位/适配器都取到了（不是空跑）', () => {
      expect(MOBILE_ITEMS.length).toBe(4);
      for (const it of MOBILE_ITEMS) {
        expect(it.featureKey.length).toBeGreaterThan(0);
        expect(it.title.length).toBeGreaterThan(0);
        expect(typeof it.freePreview).toBe('boolean');
        expect(['llm', 'diagnosis']).toContain(it.adapter);
      }
    });

    it('注入违规的变形样本会让比对变红（改展示名 / 抽掉一项 / 塞进伪入口）', () => {
      expect(parityIssues(MOBILE_ITEMS, CHAT_FEATURES)).toEqual([]);

      const renamed = MOBILE_ITEMS.map((i) => (i.featureKey === 'fault_diagnosis' ? { ...i, title: '故障咨询' } : i));
      expect(parityIssues(renamed, CHAT_FEATURES).join()).toMatch(/展示名不一致/);

      const dropped = MOBILE_ITEMS.filter((i) => i.featureKey !== 'drawing_recognition');
      expect(parityIssues(dropped, CHAT_FEATURES).join()).toMatch(/缺少注册表功能键/);

      const fake = MOBILE_ITEMS.concat([{ localKey: 'fault-code', title: '故障代码查询', featureKey: 'fault_code_query', freePreview: false }]);
      expect(parityIssues(fake, CHAT_FEATURES).join()).toMatch(/伪入口/);

      const flipped = MOBILE_ITEMS.map((i) => (i.featureKey === 'maintenance_knowledge' ? { ...i, freePreview: true } : i));
      expect(parityIssues(flipped, CHAT_FEATURES).join()).toMatch(/限免位不一致/);

      // 把「维保知识」的适配器写成 diagnosis ⇒ 诊断筛选会跑到那一页去（判据变红）
      const swapped = MOBILE_ITEMS.map((i) => (i.featureKey === 'maintenance_knowledge' ? { ...i, adapter: 'diagnosis' } : i));
      expect(parityIssues(swapped, CHAT_FEATURES).join()).toMatch(/传输适配器不一致/);
    });
  });

  describe('② 真实文件：键集合 / 展示名 / 限免位 / 传输适配器 四件全等', () => {
    it('四项契约全等（唯一的综合判据）', () => {
      expect(parityIssues(MOBILE_ITEMS, CHAT_FEATURES)).toEqual([]);
    });

    it('适配器：4 项专项功能里**只有**智能维修诊断走 diagnosis —— #1042 诊断筛选的注册表依据', () => {
      const regDiag = CHAT_FEATURES.filter((f) => f.adapter === 'diagnosis').map((f) => f.key);
      expect(regDiag).toEqual(['fault_diagnosis']);
      const mobileDiag = MOBILE_ITEMS.filter((i) => i.adapter === 'diagnosis').map((i) => i.featureKey);
      expect(mobileDiag).toEqual(['fault_diagnosis']);
      // 检测器自证：把别的功能也标成 diagnosis ⇒ 集合断言变红
      const extra = CHAT_FEATURES.map((f) => (f.key === 'drawing_recognition' ? { ...f, adapter: 'diagnosis' } : f));
      expect(extra.filter((f) => f.adapter === 'diagnosis').map((f) => f.key).sort()).not.toEqual(['fault_diagnosis']);
    });

    it('限免位：只有智能维修诊断（fault_diagnosis）限免 —— 注册表 freePreview 的真实读数', () => {
      const free = CHAT_FEATURES.filter((f) => f.freePreview).map((f) => f.key);
      expect(free).toEqual(['fault_diagnosis']);
      const mobileFree = MOBILE_ITEMS.filter((i) => i.freePreview).map((i) => i.featureKey);
      expect(mobileFree).toEqual(['fault_diagnosis']);
    });

    it('「故障代码查询」不得复活（注册表里没有它的键；留着就是点了走通用对话的伪入口）', () => {
      // 注释被剥掉再判：常量文件的头注释**必须**能解释「为什么删了它」（那份解释里就有这个名字），
      // 判据是「代码里没有它」。若真把它加回清单，注释剥离不影响检出（下面自证）。
      const code = stripCodeComments(CONSTANTS);
      expect(code).not.toMatch(/fault[_-]code/i);
      expect(code).not.toMatch(/fault_code_query/);
      expect(code).not.toMatch(/故障代码查询/);
      // 检测器自证：把该格加回清单 ⇒ 判据变红。
      // 样本必须写成**完整的格形态**（含 adapter）—— 少了 adapter 的话它压根不被
      // `parseMobileItems` 认作一格，自证会静默空跑（写这条时踩过：样本缺列 ⇒ 断言拿到空串）。
      const revived = CONSTANTS.replace(
        /\n\]/,
        "\n    { key: 'fault-code', title: '故障代码查询', icon: '\\uD83D\\uDD0D', featureKey: 'fault_code_query', freePreview: false, adapter: 'llm' },\n]"
      );
      expect(stripCodeComments(revived)).toMatch(/故障代码查询/);
      expect(parityIssues(parseMobileItems(revived), CHAT_FEATURES).join()).toMatch(/伪入口/);
    });
  });
});
