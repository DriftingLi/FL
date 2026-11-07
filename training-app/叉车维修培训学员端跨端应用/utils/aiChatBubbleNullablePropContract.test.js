/**
 * 气泡「可空 prop 的 null」源码契约（#2328）
 *
 * 事故：智能维修诊断助手发消息后不回复。链子是
 *   1) UTS 侧 `AiSessionMessage` 的 `sources` / `images` 是 optional，构造时被显式填 null
 *      （`api/aiAssistant.uts` 的 `buildSessionMessage`、`composables/useAiChat.uts` 的助手占位消息）；
 *   2) Vue 的 prop default 只对 `undefined` 生效（`_resolvePropValue` 的 `value === void 0`），
 *      null 原样穿过 `withDefaults`；
 *   3) 模板里 `sources.length` 于是在 null 上抛 TypeError，整帧渲染中断 ⇒ 页面数据不再更新
 *      ⇒ 表现为「输入了不回复」。
 *
 * `content` 在服务端映射里走过 `toStr`（恒为字符串），但它同样是可空 prop —— 同一条缝上
 * 再落一个 null 就是同一个崩溃，故一并收口。
 *
 * `ai-chat-bubble.uvue` 不能在 jest 里 import，沿用源码契约缝（读源文本断言，先例
 * `utils/aiChatImageContract.test.js`）：只钉「模板认不认归一后的名字」这层可被静态判定的契约。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const BUBBLE_PATH = path.join(__dirname, '..', 'components', 'ai-chat', 'ai-chat-bubble.uvue');
const src = readText(BUBBLE_PATH);

const template = src.slice(src.indexOf('<template>'), src.indexOf('</template>'));
const script = src.slice(src.indexOf('<script'));

/** 取某个 `const <name> = computed…` 起到本语句结束的源码片段（够断言守卫形状即可）。 */
function computedBody(name) {
  const at = script.indexOf(`const ${name} = computed`);
  if (at === -1) throw new Error(`未找到 computed ${name}`);
  return script.slice(at, script.indexOf('})', at));
}

describe('气泡对可空 prop 的归一契约（#2328）', () => {
  describe('模板不得对可空 prop 直接取 .length（崩溃本位）', () => {
    it('不读裸 content.length', () => {
      expect(template).not.toMatch(/\bcontent\.length\b/);
    });

    it('不读裸 sources.length', () => {
      expect(template).not.toMatch(/\bsources\.length\b/);
    });
  });

  describe('归一 computed 存在，且形状与 userImages 一致（null → 空值）', () => {
    it('messageContent：null 归一成空字符串', () => {
      const body = computedBody('messageContent');
      expect(body).toContain('props.content');
      expect(body).toContain('if (v == null) return \'\'');
    });

    it('messageSources：null 归一成空数组', () => {
      const body = computedBody('messageSources');
      expect(body).toContain('props.sources');
      expect(body).toContain('if (v == null) return [] as AiDiagnosisSource[]');
    });
  });

  describe('模板只消费归一后的值', () => {
    it('用户文字行读 messageContent', () => {
      expect(template).toContain('messageContent.length > 0');
      expect(template).toContain('displayContent(messageContent)');
    });

    it('来源组件读 messageSources，不再直传 props.sources', () => {
      expect(template).toContain(':sources="messageSources"');
      expect(template).not.toContain(':sources="sources"');
    });

    it('助手 markdown 块吃归一后的正文', () => {
      expect(script).toContain('aiAnswerBlocks(messageContent.value)');
      expect(script).not.toContain('aiAnswerBlocks(props.content)');
    });
  });

  /**
   * #2299 在用户文字行与助手块之间插了附图 `v-if`，`v-else` 于是从「非用户消息」
   * 错配成「没有附图」——有文字无附图的用户消息把同一段话又渲染了一遍 markdown。
   * 钉住：助手块从此走显式条件，别再靠 v-else 承接。
   */
  describe('助手块的分流条件是显式的，不靠 v-else 承接', () => {
    /**
     * #2354（决议 D5 第 1 条）：气泡自带的 8 条分支模板搬进了四面共用的
     * `components/markdown/markdown-blocks.uvue`，于是原来的 `<view … class="md-body">`
     * 这一行**物理消失**了。这条断言守的语义是「助手块用显式条件、不靠 v-else 承接」，
     * 与承载它的标签名无关 ⇒ 按 ADR-0007「可更新文件路径指向，禁删 / 弱化断言」**等价改写**：
     * 断言的仍然是**同一个 v-if 条件**，只是跟着承载要素搬到了 `<MarkdownBlocks>` 上。
     */
    it('助手块用 v-if="role != \'user\'"', () => {
      expect(template).toContain('<MarkdownBlocks v-if="role != \'user\'"');
    });

    it('不再有 `<view v-else class="md-body">`，也不再有 v-else 承接助手块', () => {
      expect(template).not.toContain('<view v-else class="md-body">');
      expect(template).not.toMatch(/<MarkdownBlocks\s+v-else/);
    });
  });
});
