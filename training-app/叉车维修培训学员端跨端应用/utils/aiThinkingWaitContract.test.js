/**
 * #2298 · 等待态与超时口径的源码契约（读源文本，接线腿）
 *
 * ## 它在守什么
 *
 * 票面有两条判据**只有形态能守**——它们写坏了都不报错，只是行为悄悄退回旧样子：
 *
 * 1. **单一超时口径**（验收第 2 条）：客户端超时必须与服务端 `aiStreamTimeout` 同量级、
 *    且留余量，仓内不能有第二个语义不同的超时值。口径的对齐靠注释 + 常量同名（维护者
 *    2026-11-06 裁决：不加跨栈生成/契约机器），所以这条守护要同时钉住「数字」和
 *    「指向哪份后端源码的注释」——只钉数字会让下一个人不知道去改哪边。
 * 2. **等待态可解释、可停、不串轮**（验收第 1 / 5 条）：`watch(thinking)` 进 / 出都收计时器；
 *    计时器与常亮都挂在 `onUnmounted` 上；诊断页常亮不能顺手开到通用页。更硬的行为判据在
 *    `utils/aiStreamFailureBehavior.test.js`（真跑被测模块）；这里守的是「接线被拆掉」。
 *
 * 另钉一条反向判据：**不得再出现固定 30 秒兜底复位**（规格决策 17 明令取消）——旧实现
 * `useAiChat.uts` 里那个 30 s 强制复位会让「真在跑的长回答」被提前宣告结束，正是本票要杀的形态。
 */
const path = require('path');

const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const ENV_UTS = readText(path.join(ROOT, 'config', 'env.uts'));
const API_UTS = readText(path.join(ROOT, 'api', 'aiAssistant.uts'));
const CHAT_UTS = readText(path.join(ROOT, 'composables', 'useAiChat.uts'));
const FEATURE_PAGE = readText(path.join(ROOT, 'pages', 'ai-assistant', 'ai-feature.uvue'));

describe('#2298 单一超时口径', () => {
  it('客户端超时 = 330000（服务端 300 s + 30 s 余量），且注释指回后端那一行', () => {
    expect(ENV_UTS).toContain('AI_REQUEST_TIMEOUT = 330000');
    // 注释必须能让下一个人找到服务端口径的出处：改后端就要同步改这里
    expect(ENV_UTS).toContain('aiStreamTimeout');
    expect(ENV_UTS).toContain('transport.go');
  });

  it('流式请求只认 AI_REQUEST_TIMEOUT，不再出现独立的硬编码超时', () => {
    expect(API_UTS).toContain('timeout: AI_REQUEST_TIMEOUT');
    // 第二口径的两种典型回潮形态
    expect(API_UTS).not.toContain('timeout: 120000');
    expect(API_UTS).not.toContain('timeout: 15000');
  });
});

describe('#2298 等待态（已等秒数 / 常亮 / 收尾）', () => {
  it('秒数来自等待态本身，文案带「已等」', () => {
    expect(FEATURE_PAGE).toContain('thinkingSeconds');
    expect(FEATURE_PAGE).toContain('已等');
    expect(FEATURE_PAGE).toContain('{{ thinkingLabel }}');
  });

  it('进 / 出等待态都收计时器（防串轮），且订阅的是 thinking', () => {
    expect(FEATURE_PAGE).toContain('watch(thinking');
    expect(FEATURE_PAGE).toContain('stopThinkingTimer()');
    // 停止必须真的 clearInterval，而不是只把变量置 null
    const stopFn = FEATURE_PAGE.slice(
      FEATURE_PAGE.indexOf('function stopThinkingTimer'),
      FEATURE_PAGE.indexOf('function restoreScreenOn')
    );
    expect(stopFn).toContain('clearInterval');
  });

  it('计时器与常亮都挂在页面卸载上（定时器不裸奔）', () => {
    // uni-app x 的页面卸载钩子是 `onUnload`（来自 @dcloudio/uni-app），**不是** Vue 的 onUnmounted ——
    // 写错不是静默失效，是编译期 error18「找不到名称」（2026-11-06 实测，第一次编译就撞上）
    expect(FEATURE_PAGE).toContain('onUnload(');
    expect(FEATURE_PAGE).not.toContain('onUnmounted(');
    const unmount = FEATURE_PAGE.slice(FEATURE_PAGE.indexOf('onUnload('));
    expect(unmount).toContain('stopThinkingTimer()');
    expect(unmount).toContain('restoreScreenOn()');
  });

  it('常亮只对诊断页开（通用页不点屏）', () => {
    expect(FEATURE_PAGE).toContain('if (isDiagnosis.value && !keepScreenOnByThinking)');
    expect(FEATURE_PAGE).toContain('uni.setKeepScreenOn({ keepScreenOn: true })');
    expect(FEATURE_PAGE).toContain('uni.setKeepScreenOn({ keepScreenOn: false })');
  });

  it('固定 30 秒兜底复位不得回潮（规格决策 17）', () => {
    for (const src of [CHAT_UTS, FEATURE_PAGE]) {
      expect(src).not.toContain('30000');
      expect(src).not.toContain('30 * 1000');
    }
  });
});

describe('#2298 检测器有牙', () => {
  it('把超时改回 120000 的样本会被同一判据判红', () => {
    const broken = ENV_UTS.replace('AI_REQUEST_TIMEOUT = 330000', 'AI_REQUEST_TIMEOUT = 120000');
    expect(broken).not.toBe(ENV_UTS);
    expect(broken).not.toContain('AI_REQUEST_TIMEOUT = 330000');
  });

  it('把 watch(thinking) 拆掉的样本会在「订阅」判据上判红', () => {
    const broken = FEATURE_PAGE.replace('watch(thinking', 'watch(somethingElse');
    expect(broken).not.toBe(FEATURE_PAGE);
    expect(broken).not.toContain('watch(thinking');
  });
});
