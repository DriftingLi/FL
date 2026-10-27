/**
 * 原生直开薄壳（`utils/systemBrowser.uts`）与其消费面的 **接线守护**（票 #2035）
 *
 * ## 这是接线守护，不是 ③ 门证据（`docs/agents/guards.md`）
 * 它钉的是三条**行为缝射程外的结构/编译期事实**：
 *   ① error15 具名导入接线（安卓原生类逐名单引 + 消费面全具名、零命名空间导入形态）；
 *   ② 端分叉的**区域归属**（`openExternalBrowser` 唯一调用点落在 `#ifdef APP-ANDROID` 区内、
 *      复制+toast 落在 `#ifdef MP-WEIXIN` 区内）——排他性是编译期事实，`loadUts` 里 `#ifdef`
 *      是惰性注释（双支同执），行为断言管不到「哪端只剩哪支」，只能由源码结构钉；
 *   ③ fail-safe 顺序（`getUniActivity()` 为 null 时静默 return **先于** `startActivity`）。
 *
 * **行为面由谁兜底**（缺了它本文件会假绿，逐条点名，guards.md 末节问 3）：
 *   · 编排（空链接闸 / 弹窗字样 / 确认闸 / 副作用落点，成对取证）⇒ `externalLinkDispatchBehavior.test.js`；
 *   · 协议闸（http(s) 白名单）⇒ `markdownInlineBehavior.test.js` ④ 组（含变异必红）；
 *   · 域名投影 ⇒ `markdownInlineBehavior.test.js` 域名组 + `forumBodyBehavior.test.js`。
 * 原生 Intent 支本身无行为守护的**原因**（默认导入超出 utsHarness 射程 + 非 App 端空实现）
 * 登记在 `docs/agents/guards.md`「例外登记」段。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const SB_REL = 'utils/systemBrowser.uts';
const DP_REL = 'utils/externalLinkDispatch.uts';
const MD_REL = 'pages/forum/components/forum-markdown-blocks.uvue';
const IMPORT_STAR = 'import ' + String.fromCharCode(42) + ' from';

// ===== ① error15 具名导入接线 =====

describe('① 原生/消费面全具名（error15：命名空间导入会 Kotlin 编译失败）', () => {
  it('systemBrowser.uts 的安卓类逐名单引（Intent / Uri 各一条，这是 UTS 原生形态，非违规）', () => {
    const sb = read(SB_REL);
    expect(sb).toContain("import Intent from 'android.content.Intent'");
    expect(sb).toContain("import Uri from 'android.net.Uri'");
    expect(sb).not.toContain(IMPORT_STAR);
  });
  it('分派模块具名导入 openExternalBrowser（不命名空间化）', () => {
    const dp = read(DP_REL);
    expect(dp).toContain("import { openExternalBrowser } from './systemBrowser'");
    expect(dp).not.toContain(IMPORT_STAR);
  });
  it('消费面收敛：论坛块组件不再直引 systemBrowser，外链只经分派模块', () => {
    const md = read(MD_REL);
    expect(md).not.toContain('systemBrowser');
    expect(md).toContain("import { dispatchExternalLinkTap } from '../../../utils/externalLinkDispatch'");
    expect(md.split('dispatchExternalLinkTap(').length - 1).toBe(1);   // 委托点唯一
  });
});

// ===== ② 端分叉区域归属（编译期事实） =====

describe('② 分派两支各在其 #ifdef 区内，且 APP-ANDROID 支调用点唯一', () => {
  const dp = read(DP_REL);
  it('openExternalBrowser(link) 恰好一次，且夹在 #ifdef APP-ANDROID 与其 #endif 之间', () => {
    expect(dp.split('openExternalBrowser(link)').length - 1).toBe(1);
    const ifdefAndroid = dp.indexOf('// #ifdef APP-ANDROID');
    const endifAndroid = dp.indexOf('// #endif', ifdefAndroid);
    const call = dp.indexOf('openExternalBrowser(link)');
    expect(ifdefAndroid >= 0 && call > ifdefAndroid && call < endifAndroid).toBe(true);
  });
  it('复制 + toast 整段夹在 #ifdef MP-WEIXIN 与其 #endif 之间，且 APP-ANDROID 区在前（两支不重叠）', () => {
    const ifdefAndroid = dp.indexOf('// #ifdef APP-ANDROID');
    const endifAndroid = dp.indexOf('// #endif', ifdefAndroid);
    const ifdefWx = dp.indexOf('// #ifdef MP-WEIXIN');
    const endifWx = dp.indexOf('// #endif', ifdefWx);
    const clip = dp.indexOf('uni.setClipboardData');
    const toast = dp.indexOf("uni.showToast({ title: '链接已复制'");
    expect(endifAndroid < ifdefWx).toBe(true);
    expect(clip > ifdefWx && clip < endifWx).toBe(true);
    expect(toast > ifdefWx && toast < endifWx).toBe(true);
  });
});

// ===== ③ fail-safe 顺序 =====

describe('③ getUniActivity 拿不到时静默 return —— 且闸在 startActivity 之前（UGC 面绝不崩页）', () => {
  it('null 检查行存在并先于 activity.startActivity(intent)', () => {
    const sb = read(SB_REL);
    const guard = sb.indexOf('if (activity == null) return');
    const start = sb.indexOf('activity.startActivity(intent)');
    expect(guard >= 0 && start > guard).toBe(true);
  });
  it('Intent 构造是 ACTION_VIEW + Uri.parse 的显式直开形态（不是别的动作换掉了语义）', () => {
    expect(read(SB_REL)).toContain('new Intent(Intent.ACTION_VIEW, Uri.parse(url))');
  });
});
