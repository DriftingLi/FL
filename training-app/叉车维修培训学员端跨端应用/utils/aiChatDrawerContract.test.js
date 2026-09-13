/**
 * AI 助手抽屉契约 —— 两条**真机实测踩到的**坑，一条一断言。
 *
 * 为什么需要它（都有现场证据，都是"编译能过、真机才炸/才报"）：
 *
 *  ① **类型名义重复 = 运行时崩溃**：`components/ai-chat/ai-chat-drawer-right.uvue` 原来自带一份
 *     `type MenuItem = { key: string; label: string }`，而页面传进来的是
 *     `pages/ai-assistant/ai-assistant-constants.uts` 导出的同名类型。两个结构相同、**名义不同**的类型
 *     编译到 Kotlin 后是**两个类** ⇒ 真机抛
 *     `java.lang.ClassCastException: MenuItem cannot be cast to MenuItem`
 *     （栈顶就是该文件第 9 行的 `v-for` 行）⇒ 右侧抽屉**一渲染就炸、连遮罩都画不出来**，
 *     真机表现为「点右上角 ⋯ 没有反应」（issue #947）。
 *     **左侧抽屉**的类型来自 `types/index`（单一来源）⇒ 不炸 —— 这就是左右不对称的全部原因，
 *     也是当初把根因猜成"点击/定位"的原因（猜错了，实测日志直接给了答案）。
 *
 *  ② **`<view>` 上不许用文字类样式**：`ai-chat-drawer-left.uvue` 的 `.empty-tip` 曾在 `view` 上用
 *     `text-align / font-size / color`，真机报
 *     `style property text-align|font-size|color is only supported on <text>|<button>|<input>|<textarea>`
 *     （全项目 20 多处 `.empty-tip` 里只有它这么写；其余都把字体样式放在子元素 `.empty-text` 上）。
 *     这条会让 `hx-run` 的 error 扫描判 `exit=fail`，挡住开发内循环的干净收口。
 */
const fs = require('fs');
const path = require('path');

const RIGHT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-right.uvue');
const LEFT = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-drawer-left.uvue');
const read = (p) => fs.readFileSync(p, 'utf8');

describe('AI 助手抽屉契约（类型单一来源 / view 不承载文字样式）', () => {
  const right = read(RIGHT);
  const left = read(LEFT);

  it('① 右侧抽屉不自带 MenuItem 类型，必须与页面同源（否则 Kotlin 里是两个类 ⇒ ClassCastException）', () => {
    expect(right).not.toMatch(/^\s*type\s+MenuItem\s*=/m);
    expect(right).toContain(
      "import type { MenuItem } from '../../pages/ai-assistant/ai-assistant-constants'"
    );
    // prop 仍按该类型声明（改回 any/UTSJSONObject 就等于放弃这层约束）
    expect(right).toMatch(/menuItems\?:\s*MenuItem\[\]/);
  });

  it('② 左侧抽屉同样不自带类型，走共享 types（对照项：它一直是对的）', () => {
    expect(left).not.toMatch(/^\s*type\s+(AiSession|MenuItem)\s*=/m);
    expect(left).toContain("import type { AiSession } from '../../types/index'");
  });

  it('③ empty-tip 的字体样式落在 text 上，不能在 view 上（uvue 只允许 <text> 等元素）', () => {
    const at = left.indexOf('.empty-tip {');
    expect(at).toBeGreaterThan(-1);
    const block = left.slice(at, left.indexOf('}', at));
    expect(block).not.toMatch(/font-size|color|text-align/);
    expect(left).toContain('<text class="empty-text">');
    expect(left).toMatch(/\.empty-text\s*\{[^}]*font-size/);
  });
});
