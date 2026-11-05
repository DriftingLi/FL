/**
 * 诊断引导第二层取数的**跨栈锚**（#2310 retro）
 *
 * ## 它在守什么
 *
 * #2296 的缺陷链：规格复述层写错端点口径（`picker==='model'` 走 `faults` 根层）→
 * 移动端照抄进实现与注释 → 三个守护把错的钉死（其一还把「改对」判成坏实现）。
 * 全链条没有一处锚到真正对的 **Web 真源**（`DiagnosisGuide.vue` 从始至终不分档统一调
 * `systems`）。本契约把「移动端第二层与 Web 同源」变成机检：
 *
 *   Web `watch(brandId)` 取数体 —— 恒 `listDiagnosisSystems`，无 `listDiagnosisFaults`、
 *   无 `pickerKindOf` 分支（**真源锚**：Web 被改坏或改了口径 ⇒ 红，提醒两端一起改）
 *   Web api 层 `listDiagnosisSystems` —— 端点字面量 `/diagnosis/systems`（语义链闭合）
 *   移动端 `loadSecond` —— 恒 `getDiagnosisSystemsApi(brandId)`，无 `getDiagnosisFaultsApi(brandId,'')`
 *
 * 与 `diagnosisGuideWiringContract` 的分工：那边锁「移动端不再分支」，这边锁「**两端同源**」——
 * 移动端单方面改、或 Web 真源改了移动端没跟上，都会在这里红。
 *
 * 断言纪律（#921 / guards.md ③）：每条负向断言配一个**注入变异的样本**证明它真能判红
 * （必不红 / 必红成对）。Web 文件缺失 = 当场抛（fail-closed），不静默跳过。
 * EOL 归一走共享读者 `readText`（ADR-0019）。
 */
const path = require('path');

const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
/** Web 真源在本仓根 frontend/（相对本文件上三级）；工作树全仓检出，必有此文件 */
const WEB_GUIDE = path.join(__dirname, '..', '..', '..', 'frontend', 'src', 'components', 'ai-assistant', 'DiagnosisGuide.vue');
const WEB_API = path.join(__dirname, '..', '..', '..', 'frontend', 'src', 'api', 'aiAssistant.ts');
const MOBILE_COMPOSABLE = path.join(ROOT, 'composables', 'useDiagnosisGuide.uts');

/** 去注释（块 + 行）：注释里提到某端点不算「代码里调了它」 */
function stripComments(src) {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

/** 取函数体（大括号配平）—— 防止切片越界到文件尾造成假绿 */
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

const WEB_GUIDE_CODE = stripComments(readText(WEB_GUIDE));
const WEB_API_CODE = stripComments(readText(WEB_API));
const MOBILE_CODE = stripComments(readText(MOBILE_COMPOSABLE));

const WEB_WATCHER = fnBody(WEB_GUIDE_CODE, 'watch(brandId, async (id) => {');
const WEB_SYSTEMS_FN = fnBody(WEB_API_CODE, 'listDiagnosisSystems(brand: string) {');
const MOBILE_LOAD_SECOND = fnBody(MOBILE_CODE, 'async function loadSecond() : Promise<void> {');

describe('诊断引导第二层取数跨栈锚（#2310 retro：Web 真源 ⇔ 移动端同源）', () => {
  describe('① Web 真源（锚）：两档第二层恒走 systems，不按 picker 分支', () => {
    it('web watch(brandId) 取数体调 listDiagnosisSystems，不碰 listDiagnosisFaults / pickerKindOf', () => {
      expect(WEB_WATCHER).toContain('listDiagnosisSystems(id)');
      expect(WEB_WATCHER).not.toContain('listDiagnosisFaults');
      expect(WEB_WATCHER).not.toContain('pickerKindOf');
    });

    it('web api 层的 listDiagnosisSystems 端点字面量是 /diagnosis/systems（语义链闭合）', () => {
      expect(WEB_SYSTEMS_FN).toContain("'/diagnosis/systems'");
    });

    it('检测器有牙：把 Web 真源改回 faults 根层 ⇒ ① 组负向断言判红', () => {
      const brokenWeb = WEB_GUIDE_CODE.split('listDiagnosisSystems(id)').join('listDiagnosisFaults(id)');
      expect(brokenWeb).not.toBe(WEB_GUIDE_CODE); // 变异必须真落上
      const watcher = fnBody(brokenWeb, 'watch(brandId, async (id) => {');
      expect(watcher).toContain('listDiagnosisFaults'); // 坏实现：Web 侧分支回根层
    });
  });

  describe('② 移动端消费侧：loadSecond 与 Web 同源', () => {
    it('loadSecond 恒调 getDiagnosisSystemsApi(brandId)，无 faults 根层分支', () => {
      expect(MOBILE_LOAD_SECOND).toContain('getDiagnosisSystemsApi(brandId)');
      expect(MOBILE_LOAD_SECOND).not.toContain("getDiagnosisFaultsApi(brandId, '')");
      expect(MOBILE_LOAD_SECOND).not.toContain("'faults-root'");
    });

    it('检测器有牙：把 loadSecond 改回 faults 根层 ⇒ ② 组负向断言判红', () => {
      const brokenMobile = MOBILE_CODE
        .split('getDiagnosisSystemsApi(brandId)')
        .join("getDiagnosisFaultsApi(brandId, '')");
      expect(brokenMobile).not.toBe(MOBILE_CODE); // 变异必须真落上
      const load = fnBody(brokenMobile, 'async function loadSecond() : Promise<void> {');
      expect(load).toContain("getDiagnosisFaultsApi(brandId, '')"); // 坏实现
    });
  });
});
