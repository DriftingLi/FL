/**
 * WYSIWYG 输入区**形态契约**（#2640，Parent #2631；ADR-0073）
 *
 * ## 这是**接线守护**，不是 ③ 门证据（`docs/agents/guards.md`）
 *
 * 它守的是「形态接线没有被悄悄拆掉」：常驻四键还在、触摸目标没被缩小、编辑器侧没有图片入口、
 * 记号还走 insertText 逃生舱、点亮还取 delta。这些都是**源码结构**，node 里跑不出来
 * （`.uvue` 的模板/样式不参与 UTS 行为缝）。
 *
 * **行为面由谁承重**（缺了它本文件会假绿，故逐条点名）：
 *   · 「按钮集是什么、点亮怎么判、降级怎么提示」⇒ `utils/forumWysiwygInputBehavior.test.js`
 *     （真执行 `utils/forumWysiwygInput.uts`，含三种坏实现的必红取证）；
 *   · 「delta → 论坛档 Markdown 的产物」⇒ `utils/forumDeltaMarkdownBehavior.test.js`（T1，#2638）；
 *   · 「文字类样式只落白名单承载标签」⇒ `utils/uvueFontCarrierContract.test.js`（全仓口径，本文件不重复）。
 *
 * ## 判据来源（逐条对应票面验收标准，不自己加戏）
 *
 * 常驻 4 枚 = 粗体/斜体/删除线/列表（下划线永久退出、行内代码不作常驻），加「更多」开关 = 5 个触摸目标；
 * 二级面板两组 + 操作组（粘贴）；触摸目标 ≥88rpx（R2）；面板容量余量 = 0（R1）；
 * 点亮以 getContents 的 delta 为准；记号走 insertText；编辑器侧无图片入口、正文图片丢弃并**持续可见**提示（R3）；
 * 图片入口两态、提示带合并成一行、字数在正文右下（#2689）；
 * 失焦后点按钮走 readOnly 绕行；键盘高度上报给宿主、组件不自加偏移。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const INPUT_REL = 'components/forum-wysiwyg-input.uvue';
const TOOLBAR_REL = 'components/forum-wysiwyg-toolbar.uvue';
/** #2689：图片区两态从输入区拆成独立展示件（输入区撞 600 预算，ADR-0023 ③ 拆而不覆盖）。 */
const IMAGE_AREA_REL = 'components/forum-wysiwyg-image-area.uvue';
const MODEL_REL = 'utils/forumWysiwygInput.uts';
/** #2659：收敛判定与落地（reconcileMarkerInsert / settleMarkerInsert）在 delta 管线里，与序列化同文件。 */
const DELTA_REL = 'utils/forumDeltaMarkdown.uts';
/** #2962：工具态状态机（面板开合 / 只读保活 / 选区闸门）从输入区拆成 composable（行数预算，ADR-0023 ③）。 */
const TOOL_STATE_REL = 'composables/useWysiwygToolState.uts';

const input = read(INPUT_REL);
/** #2658：格式工具条（常驻四键 + 更多面板）拆成独立件，按钮集与触摸目标的判据落在它身上。 */
const toolbar = read(TOOLBAR_REL);
const imageArea = read(IMAGE_AREA_REL);
const model = read(MODEL_REL);
const delta = read(DELTA_REL);
const toolState = read(TOOL_STATE_REL);

/** 取某个函数/方法的函数体（从签名到下一个同缩进的 `}`），用于「这个函数里不许出现 X」类判据。 */
function bodyOf(src, signature) {
    const start = src.indexOf(signature);
    expect([signature, start >= 0]).toEqual([signature, true]);
    const rest = src.slice(start);
    // 顶层函数闭合在行首、类内方法闭合缩进 4 空格 —— 取两者中更早的那个，别吞掉下一个函数
    const candidates = [rest.indexOf('\n}'), rest.indexOf('\n    }')].filter((i) => i > 0);
    expect([signature, candidates.length > 0]).toEqual([signature, true]);
    return rest.slice(0, Math.min(...candidates));
}

/** 去掉 HTML 注释、块注释与行注释：注释里点名按钮中文名不算「模板硬编码文案」。
 *  行注释从「行首」或行内「非冒号字符 + //」起剥，避免误伤 `https://` 这类 URL。 */
function stripComments(src) {
    return src
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:])\/\/.*$/gm, '$1');
}

// ===== ① 常驻四键：数据驱动，恰好四枚 + 「更多」开关 = 5 个触摸目标 =====

describe('常驻四键由数据驱动（按钮集是单点，不是模板硬编码）', () => {
    it('工具条遍历 richPrimaryActions 的结果，不自己列按钮', () => {
        expect(toolbar).toContain('const primaryActions : RichAction[] = richPrimaryActions(props.toolbarVariant)');
        expect(toolbar).toContain('const panelGroups : RichPanelGroup[] = richPanelGroups(props.toolbarVariant)');
        expect(toolbar).toContain('v-for="(act, idx) in primaryActions"');
        expect(toolbar).toContain('@click="onAction(act.id)"');
        // 拆分的接缝：工具条只 emit，输入区持有 editorCtx 才决定怎么落地
        expect(toolbar).toContain("(e: 'action', id: string): void");
        expect(toolbar).toContain("emit('action', id)");
        expect(input).toContain('<ForumWysiwygToolbar :active-ids="activeIds" :open="toolOpen" :toolbar-variant="toolbarVariant"');
        // #2967：粘贴与格式动作同一条 action 通道，单独的 paste 事件已退役
        expect(input).not.toContain('@paste="onPasteTap"');
        expect(toolbar).not.toContain("emit('paste')");
        // #2709：工具条按下要单独上报（touchstart 早于 blur），失焦检测才分得清「点键」与「粘贴被丢弃」
        expect(input).toContain('@press="onToolbarPress"');
    });

    it('#2967 两处调用点显式给形态：回复浮层 entry、发帖页 full（缺省不隐含形态）', () => {
        expect(read('pages/forum/components/forum-reply-panel.uvue')).toContain('variant="reply" toolbar-variant="entry"');
        expect(read('pages/forum/forum-create.uvue')).toContain('toolbar-variant="full"');
        expect(input).toContain('toolbarVariant?: string');
        expect(input).toContain(':toolbar-variant="toolbarVariant"');
        // 组件侧只有中性缺省，形态由调用方定
        expect(input).toContain("toolbarVariant: 'full'");
    });

    it('full 形态：常驻行恰好 5 个触摸目标 —— 4 枚数据动作 + 「更多」开关（#2689 决策 2）', () => {
        // 4 枚来自 richPrimaryActions 的 v-for，第 5 枚是「更多」开关（full 分支里）
        expect(toolbar).toContain('v-for="(act, idx) in primaryActions"');
        expect(toolbar).toContain('class="tool-btn tool-btn-more ml-16"');
        // 常驻行里不再有独立的「粘贴」键（粘贴已收进面板「操作」组）
        expect(toolbar).not.toContain('tool-btn-paste');
    });

    it('entry 形态：常驻行恰好 1 个触摸目标 —— 只有「格式」入口，⋯ 不存在（#2967 决策⑤）', () => {
        // 入口即面板头：整行一个 view，点它开合；不是「⋯」方键
        expect(toolbar).toContain('v-if="isEntry"');
        expect(toolbar).toContain('class="entry-row"');
        expect(toolbar).toContain('{{ primaryActions[0].label }}');
        expect(toolbar).toContain('@click="onTogglePanel"');
        // entry 分支（从入口行到 full 分支开始）里没有「⋯」、也没有第二枚按键
        const entryBranch = stripComments(toolbar.slice(toolbar.indexOf('class="entry-row"'), toolbar.indexOf('class="primary-row"')));
        expect(entryBranch.length).toBeGreaterThan(0);
        expect(entryBranch).not.toContain('⋯');
        expect(entryBranch).not.toContain('tool-btn');
        // 展开/收起用同一个素材旋转：收起朝下、展开朝上
        expect(entryBranch).toContain('src="/static/icons/ai-chat/chevron-model.png"');
        expect(entryBranch).toContain("'entry-arrow-open': open");
        expect(toolbar).toContain('.entry-arrow {');
        expect(toolbar).toContain('transform: rotate(-90deg);');
        expect(toolbar).toContain('.entry-arrow-open {');
        expect(toolbar).toContain('transform: rotate(90deg);');
    });

    it('B / I / S 是**真字形**不是图片：加粗 / 斜体 / 删除线（细线模拟）', () => {
        expect(toolbar).toContain('{{ act.glyph }}');
        expect(toolbar).toContain("'tool-glyph-bold': act.glyphStyle == 'bold'");
        expect(toolbar).toContain("'tool-glyph-italic': act.glyphStyle == 'italic'");
        expect(toolbar).toContain("'tool-glyph-strike': act.glyphStyle == 'strike'");
        expect(toolbar).toContain('.tool-glyph-bold {');
        expect(toolbar).toContain('font-weight: bold;');
        expect(toolbar).toContain('.tool-glyph-italic {');
        expect(toolbar).toContain('font-style: italic;');
        // uvue 不支持 text-decoration ⇒ 删除线用绝对定位细线模拟
        expect(toolbar).toContain('.strike-line {');
        expect(toolbar).toContain('position: absolute;');
        // 字形不是图片：**full 的常驻行**里没有 <image>（方向箭头是切片图标，见 DESIGN.md；
        // #2967 起箭头落在 entry 入口行里，那是「开合控件」不是格式键）
        const primaryRow = toolbar.slice(toolbar.indexOf('class="primary-row"'), toolbar.indexOf('class="panel"'))
        expect(primaryRow.length).toBeGreaterThan(0);
        expect(primaryRow).not.toContain('<image');
    });

    it('模板里不出现中文按钮名（文案单点在 utils/forumWysiwygInput）', () => {
        for (const src of [stripComments(input), stripComments(toolbar)]) {
            // #2967：入口文案「格式」与操作组「粘贴」也进数据层；组名同理
            for (const label of ['粗体', '斜体', '删除线', '引用', '代码块', '行内代码', '链接', '格式', '粘贴', '字重', '结构', '操作']) {
                expect([label, src.includes(label)]).toEqual([label, false]);
            }
        }
        expect(stripComments(toolbar)).toContain('{{ act.label }}');
    });

    it('下划线不在按钮集里（永久退出），行内代码不作常驻', () => {
        expect(model).not.toContain("RICH_ACTION_UNDERLINE");
        expect(model).not.toContain("formatAction('underline'");
        // 行内代码只出现在面板「记号」组，不在 richPrimaryActions 里
        const primary = bodyOf(model, 'export function richPrimaryActions(');
        expect(primary).not.toContain('CODE_INLINE');
    });
});

// ===== ② 触摸目标 ≥88rpx（R2）与面板容量余量 = 0（R1）=====

describe('触摸目标 ≥88rpx（R2）：常驻键、面板键、更多键都不缩小', () => {
    it('常驻键 88×88rpx', () => {
        expect(toolbar).toMatch(/\.tool-btn \{[\s\S]*?width: 88rpx;[\s\S]*?height: 88rpx;/);
    });

    it('面板键高度 88rpx、最小宽度 88rpx（不为塞按钮缩小）', () => {
        expect(toolbar).toMatch(/\.panel-btn \{[\s\S]*?min-width: 88rpx;[\s\S]*?height: 88rpx;/);
    });

    it('「更多」键只加宽不加高（120rpx × 88rpx；只在 full 形态出现）', () => {
        expect(toolbar).toMatch(/\.tool-btn-more \{[\s\S]*?width: 120rpx;/);
    });

    it('entry 入口行：高 88rpx、撑满工具行（不给固定 width，避免 88/120rpx 方键的老账）', () => {
        const block = toolbar.match(/\.entry-row \{[\s\S]*?\}/);
        expect(block).not.toBeNull();
        expect(block[0]).toContain('height: 88rpx;');
        expect(block[0]).not.toMatch(/[^-]width:/);
    });

    it('88 这个数只有一处来源：模型常量与样式表交叉对齐（防两处各写一份漂移）', () => {
        const declared = model.match(/RICH_TOUCH_TARGET_RPX = (\d+)/);
        expect(declared).not.toBeNull();
        const rpx = declared[1] + 'rpx';
        expect(toolbar).toContain('width: ' + rpx + ';');
        expect(toolbar).toContain('height: ' + rpx + ';');
        expect(toolbar).toContain('min-width: ' + rpx + ';');
    });
});

// ===== ③ 编辑器侧没有图片入口（图片只从下方单一 ＋ 进）=====

describe('编辑器侧无图片插入入口：正文不产出 ![alt](url)（R3）', () => {
    it('不调用 insertImage，也不开编辑器的图片工具条/尺寸/缩放', () => {
        expect(input).not.toContain('insertImage');
        expect(input).toContain(':show-img-size="false"');
        expect(input).toContain(':show-img-toolbar="false"');
        expect(input).toContain(':show-img-resize="false"');
    });

    it('图片只有一个入口：下方 ＋ 方块 → emit add-image', () => {
        expect(imageArea).toContain('@click="onAdd"');
        expect(imageArea).toContain("emit('add-image')");
        expect(imageArea).toContain("'＋'");
        // 输入区只把动作原样转出去，自己不画入口
        expect(input).toContain('@add-image="onAddImage"');
        expect(input).toContain("emit('add-image')");
        // 双入口（拍照 / 相册各一枚）已退场
        expect(imageArea).not.toContain("onAddImage('camera')");
        expect(imageArea).not.toContain("onAddImage('album')");
    });

    it('图片入口两态（#2689 决策 3）：未选图整宽卡片 / 已选图缩略图行 + 继续添加 / 选满隐藏', () => {
        expect(imageArea).toContain('v-if="imageCount == 0 && !isReply"');
        expect(imageArea).toContain('class="image-empty"');
        expect(imageArea).toContain('支持添加图片，最多 {{ maxImages }} 张');
        // #2691：回复面（variant=reply）的添加入口交给浮层底部按钮，本组件不再画末尾「＋」方块
        expect(imageArea).toContain('v-if="!isReply && imageCount < maxImages"');
        expect(imageArea).toContain("{{ uploading ? '上传中' : '继续添加' }}");
        // 缩略图右上 × 可删
        expect(imageArea).toContain('class="image-remove"');
        expect(imageArea).toContain('@click.stop="onRemove(idx)"');
    });

    it('#2691 回复面图片带：独立浅底带 + 顶部细分隔线，无图时整块不渲染', () => {
        // 有图才出现（回复面不渲染空态卡）
        expect(imageArea).toContain('v-if="visible"');
        expect(imageArea).toContain("'image-band': isReply");
        expect(imageArea).toContain("'image-row-band': isReply");
        expect(imageArea).toMatch(/\.image-band\s*\{[^}]*border-top:/);
    });

    it('字数在正文右下（#2689 决策 5）：紧贴编辑区（工具条之前）、图片之前', () => {
        const toolbarAt = input.indexOf('<ForumWysiwygToolbar');
        const counterAt = input.indexOf('class="counter-text"');
        const imageAt = input.indexOf('<ForumWysiwygImageArea');
        expect(toolbarAt).toBeGreaterThan(-1);
        expect(counterAt).toBeGreaterThan(-1);
        expect(counterAt).toBeLessThan(toolbarAt);
        expect(imageAt).toBeGreaterThan(toolbarAt);
        expect(input).toContain("markdownLength.value.toString() + '/' + props.maxlength.toString()");
    });

    it('丢弃提示并进提示带：**持续可见**的一行，不是 toast（#2689 决策 4）', () => {
        expect(input).toContain('<view v-if="noticeBand.length > 0" class="notice-band"');
        expect(input).toContain('richFidelityNotice(issues.value)');
        expect(input).not.toContain('showToast');
        expect(input).not.toContain('showModal');
    });

    it('提示带：保真 / 粘贴 / 属地 / 边界合并成一行，多条用「；」连接，无提示不占位', () => {
        const band = bodyOf(input, 'const noticeBand = computed<string>');
        expect(band).toContain('fidelityNotice.value');
        expect(band).toContain('pasteNotice.value');
        expect(band).toContain('pasteHint.value');
        expect(band).toContain('regionNotice');
        expect(band).toContain('boundaryNotice');
        expect(band).toContain("parts.join('；')");
        // 同一时刻最多一行：四个旧提示行各自独立的模板节点已退场
        expect(input).not.toContain('class="fidelity-notice"');
        expect(input).not.toContain('class="paste-notice"');
        expect(input).not.toContain('class="paste-hint-row"');
        expect(input).not.toContain('class="region-notice"');
        expect(input).not.toContain('class="boundary-notice"');
    });
});

// ===== ④ 记号逃生舱：一律 insertText，不经 editor 的 link/code 属性 =====

describe('记号四件走 insertText 逃生舱（editor 的 link 丢 URL、code 是 no-op）', () => {
    it('有选区包裹 / 无选区插模板，都经 buildMarker + insertText', () => {
        expect(toolState).toContain('ctx.getSelectionText({');
        expect(toolState).toContain('buildMarker(action.id, selected)');
        expect(toolState).toContain('ctx.insertText({');
    });

    it('插入后必须收敛：insertText 在真机上不删选区（#2659），写成后按 delta 复核并 setContents', () => {
        const apply = bodyOf(toolState, 'function applyAction(action : RichAction) : void {');
        expect(apply).toContain('settleMarkerInsert(ctx, preDelta, selected, text, deps.refresh)');
        // 顶层函数（0 缩进）的收尾是 "\n}"，bodyOf 会在内层 "\n    }" 处截断 ⇒ 从签名切到文件末尾
        const settle = delta.slice(delta.indexOf('export function settleMarkerInsert('));
        expect(settle).toContain('reconcileMarkerInsert(');
        expect(settle).toContain('ctx.setContents(');
    });

    it('不调用 insertLink（链接输出 [文字](URL) 由作者自填 URL）', () => {
        expect(input).not.toContain('insertLink');
        expect(toolState).not.toContain('insertLink');
    });

    it('分割线走 insertDivider，格式走 format', () => {
        expect(toolState).toContain('ctx.insertDivider({');
        expect(toolState).toContain('ctx.format(action.formatName, action.formatValue)');
    });
});

// ===== ⑤ 点亮判据：一律取 delta，不取 statuschange 的 detail =====

describe('回显以 getContents 的 delta 为准（statuschange 只是信号）', () => {
    it('唯一刷新点 = getContents → serializeDelta → richActiveFromDelta', () => {
        const refresh = bodyOf(input, 'function refreshFromEditor()');
        expect(refresh).toContain('ctx.getContents({');
        expect(refresh).toContain('serializeDelta(deltaJson)');
        expect(refresh).toContain('richActiveFromDelta(deltaJson)');
        expect(refresh).toContain("emit('update:markdown', result.markdown)");
        expect(refresh).toContain("emit('update:issues', result.issues)");
    });

    it('statuschange 处理器不读 detail（累积快照不是事实源）', () => {
        const handler = bodyOf(input, 'function onStatusChange(');
        expect(handler).not.toContain('.detail');
        expect(handler).toContain('refreshFromEditor()');
    });

    it('input 事件同样只走刷新点', () => {
        expect(bodyOf(input, 'function onEditorInput(')).toContain('refreshFromEditor()');
    });
});

// ===== ⑥ 失焦后点按钮的绕行（readOnly=true → 下一个 tick 执行）=====

describe('失焦后点格式按钮仍可用：readOnly 保住选区，format 推到下一个 tick', () => {
    it('失焦置 readOnly=true、聚焦置 false', () => {
        expect(bodyOf(input, 'function onEditorBlur()')).toContain('readOnly.value = true');
        expect(bodyOf(input, 'function onEditorFocus()')).toContain('readOnly.value = false');
        expect(input).toContain(':read-only="readOnly"');
    });

    it('点按钮时若处于 readOnly，则 setTimeout 后执行并立刻恢复可编辑', () => {
        // 动作分发实体在 composable（#2962 拆出），输入区只留转发
        expect(input).toContain('tool.onActionTap(id)');
        const tap = bodyOf(toolState, 'function onActionTap(id : string) : void {');
        expect(tap).toContain('if (deps.isReadOnly()) {');
        expect(tap).toContain('setTimeout(() => {');
        expect(tap).toContain('applyAction(action)');
        expect(tap).toContain('deps.setReadOnly(false)');
    });

    // #2658：只读态会挡住「点编辑器重新聚焦」。一次被原生层丢弃的粘贴会让编辑器失焦，
    // 只读若一直挂着，作者就再也点不进去，只能退出重进。
    it('只读态必须自己放开：失焦后 READONLY_HOLD_MS 到点恢复可编辑', () => {
        const blur = bodyOf(input, 'function onEditorBlur()');
        expect(blur).toContain('readOnly.value = true');
        // #2962 起这一放开加了工具态闸门（工具态可以开很久，放开就丢选区），但非工具态必须照旧放开：
        expect(blur).toContain('READONLY_HOLD_MS');
        expect(blur).toMatch(/setTimeout\(\(\) => \{[\s\S]*?readOnly\.value = false[\s\S]*?\}, READONLY_HOLD_MS\)/);
        expect(input).toContain('const READONLY_HOLD_MS : number =');
    });
});

// ===== ⑥b 粘贴：原生粘贴丢弃 text/html，改走「粘贴」按钮 → insertText（#2658）=====

describe('粘贴走 insertText 同一条 content-change 路径（原生粘贴会整段丢弃 text/html）', () => {
    it('「粘贴」收在数据层「操作」组（#2967）：组与项不再是模板硬编码', () => {
        // 操作组由 richPanelGroups 提供，模板只遍历 group.actions
        expect(toolbar).toContain('v-for="(group, gi) in panelGroups"');
        expect(toolbar).toContain('v-for="act in group.actions"');
        expect(toolbar).not.toContain('>操作</text>');
        expect(toolbar).not.toContain('{{ pasteLabel }}');
        // 文案与 id 都在数据层：粘贴项取 utils/forumPaste 的 PASTE_LABEL
        expect(model).toContain("import { PASTE_LABEL } from './forumPaste'");
        expect(model).toContain("export const PASTE_ACTION_ID = 'paste'");
        expect(model).toContain('function pasteAction() : RichAction {');
        // richPanelGroups 内部有 if 块收尾，不能用 bodyOf（按缩进取体会截断）⇒ 直接取到下一个函数之前
        const groupsBody = model.slice(model.indexOf('export function richPanelGroups('), model.indexOf('export function richActionById('));
        expect(groupsBody).toContain('pasteAction()');
        // 接缝：工具条只发 action，输入区按 id 分叉去读剪贴板
        expect(input).toContain('if (id == PASTE_ACTION_ID) {');
        expect(input).toContain('onPasteTap()');
        expect(input).toContain('function onPasteTap()');
    });

    it('读剪贴板 → 归一 → insertText，且失败分支都有可见提示', () => {
        const tap = bodyOf(input, 'function onPasteTap()');
        expect(tap).toContain('uni.getClipboardData({');
        expect(tap).toContain('normalizePastedText(res.data)');
        expect(tap).toContain('insertPastedText(text)');
        expect(tap).toContain('PASTE_NOTICE_EMPTY');
        expect(tap).toContain('PASTE_NOTICE_UNREADABLE');

        const insert = bodyOf(input, 'function insertPastedText(');
        expect(insert).toContain('ctx.insertText({');
        expect(insert).toContain('PASTE_NOTICE_DEGRADED');
        expect(insert).toContain('PASTE_NOTICE_INSERT_FAILED');
        expect(insert).toContain('refreshFromEditor()');
    });

    it('降级提示并进提示带，不是 toast（#2689 决策 4）', () => {
        expect(input).toContain('<view v-if="noticeBand.length > 0" class="notice-band"');
        // 粘贴提示在时整条可点，走「粘贴」按钮同一条路径
        expect(input).toContain(":class=\"{ 'notice-band-tap': pasteHint.length > 0 }\"");
        expect(input).toContain('@click="onNoticeBandTap"');
        expect(bodyOf(input, 'function onNoticeBandTap()')).toContain('onPasteTap()');
        expect(toolbar).not.toContain('showToast');
        expect(toolbar).not.toContain('showModal');
        expect(input).not.toContain('showToast');
        expect(input).not.toContain('showModal');
    });

    it('原生粘贴被丢弃后给可点提示，判据来自纯逻辑模块', () => {
        const blur = bodyOf(input, 'function onEditorBlur()');
        expect(blur).toContain('detectDroppedPaste()');
        const detect = bodyOf(input, 'function detectDroppedPaste()');
        expect(detect).toContain('shouldOfferPasteHint(');
        expect(detect).toContain('PASTE_HINT_AFTER_DROP');
    });

    it('粘贴自身那次 @input 不清提示，之后任何正文变化都清', () => {
        const handler = bodyOf(input, 'function onEditorInput(');
        expect(handler).toContain('shouldClearPasteNotice(pasteNoticeAt.value, Date.now())');
        expect(handler).toContain("pasteNotice.value = ''");
    });
});

// ===== ⑥c 小程序端取 ctx 的作用域与「点键不误弹提示」（#2709）=====

describe('mp-weixin 取 EditorContext 必须限定组件作用域（#2709 根因）', () => {
    it('createEditorContextAsync 带上本组件实例：选择器初始只覆盖页面范围，取不到组件内 editor', () => {
        // 实例必须在 setup 顶层取（事件回调里 getCurrentInstance() 拿不到当前实例）
        expect(input).toContain('const selfInstance = getCurrentInstance()');
        expect(input).toContain('options.component = proxy');
        // 传实例与调用 ctx 在同一处：先补作用域，再 createEditorContextAsync
        const ready = bodyOf(input, 'function onEditorReady()');
        expect(ready).toContain('options.component = proxy');
        expect(ready).toContain('uni.createEditorContextAsync(options)');
    });

    it('fail 不再空吞：取不到 ctx 时留一条可定位日志（否则所有键静默失效）', () => {
        const ready = bodyOf(input, 'function onEditorReady()');
        expect(ready).not.toContain('fail: (_error : UniError) => {}');
        expect(ready).toContain('createEditorContextAsync 失败');
    });

    it('点工具条引起的失焦不算「粘贴被丢弃」：touchstart 先上报，失焦检测按窗口早退', () => {
        expect(toolbar).toContain('@touchstart="onPress"');
        expect(toolbar).toContain("emit('press')");
        const detect = bodyOf(input, 'function detectDroppedPaste()');
        expect(detect).toContain('toolbarPressAt');
        expect(detect).toContain('TOOLBAR_PRESS_WINDOW_MS');
        // 失焦后等一拍再判：touchstart 可能晚于 blur
        expect(bodyOf(input, 'function onEditorBlur()')).toContain('BLUR_SETTLE_MS');
    });
});

// ===== ⑦ 键盘：只上报高度，组件不自加偏移 =====

describe('键盘弹起：上报高度给宿主，组件不自加偏移（避免与面板 bottom 双重位移）', () => {
    it('监听 onKeyboardHeightChange 并 emit keyboard-height', () => {
        expect(input).toContain('uni.onKeyboardHeightChange((res) => {');
        expect(input).toContain("emit('keyboard-height', res.height)");
    });

    it('卸载时让回调失效，且不无参 off（无参会摘掉宿主页自己的监听）', () => {
        expect(input).toContain('onUnmounted(() => {');
        expect(input).toContain('keyboardAlive = false');
        expect(stripComments(input)).not.toContain('uni.offKeyboardHeightChange()');
    });

    it('组件内不出现面板位移计算（那是宿主的 replyPanelLayout）', () => {
        expect(input).not.toContain('panelBottomPx(');
        expect(input).not.toContain('keyboardHeightPx');
    });
});

// ===== ⑧ 对外只吐 markdown 与保真结果（编辑态不出组件）=====

describe('对外契约：只 emit 序列化后的 markdown 与保真检查结果', () => {
    it('emit 声明齐全', () => {
        for (const e of ['update:markdown', 'update:issues', 'add-image', 'remove-image', 'preview-image', 'keyboard-height']) {
            expect([e, input.includes("(e: '" + e + "'")]).toEqual([e, true]);
        }
    });

    it('旧的档位契约（update:content / update:format）已退场', () => {
        expect(input).not.toContain("'update:content'");
        expect(input).not.toContain("'update:format'");
        expect(input).not.toContain('FORMAT_MARKDOWN');
    });

    it('不直连解析器（界面层只经序列化器这一条单点）', () => {
        expect(input).not.toContain('parseMarkdown');
        expect(input).not.toContain('SUBSET_');
        expect(input).not.toContain("from '../../../utils/markdown'");
    });
});

// ===== ⑨ 样式红线（本票新增的判据，全仓口径另由 uvueFontCarrierContract 执法）=====

describe('宿主写入通道（#2642 @某人）：组件暴露 insertMention，一次点击一次写入', () => {
    it('defineExpose 暴露 insertMention；写入走 insertText 逃生舱并回流保真检查', () => {
        expect(input).toContain('defineExpose({ insertMention, clear })');
        expect(input).toContain('function insertMention(');
        // 写入路径与记号同源：insertText + refreshFromEditor（不是另起一套 setContent）
        expect(input).toContain('ctx.insertText({');
        // ready 之前点击的暂存：避免 @某人 因编辑器未挂载而丢失
        expect(input).toContain('pendingMention');
        expect(input).toContain('ctx_insertMentionText(context, text)');
    });
});

describe('样式红线：scss + 只用 class 选择器 + 编辑器本体不承载文字样式', () => {
    it('<style> 必须 lang="scss"', () => {
        expect(input).toContain('<style lang="scss">');
    });

    it('编辑器本体只放盒模型与底色（font-size / color 落 <editor> 会被忽略并判错）', () => {
        const block = input.match(/\.rich-editor \{[\s\S]*?\}/);
        expect(block).not.toBeNull();
        expect(block[0]).not.toContain('font-size');
        // 只禁独立的 color 声明（background-color 是底色，允许）
        expect(block[0]).not.toMatch(/(^|[\s;{])color:/);
    });

    it('不使用 uvue 原生端不支持的写法（gap / calc / var(--x) / transition / animation）', () => {
        for (const banned of ['gap:', 'calc(', 'var(--', 'transition:', 'animation:']) {
            expect([banned, input.includes(banned)]).toEqual([banned, false]);
        }
    });
});

// ===== 工具态（#2962，父规格 #2961）：面板开合是受控的，工具态内只读保活 =====

describe('工具态接线（#2962）：面板开合受控 + 工具态内只读保活 + 点编辑区回输入态', () => {
    it('面板开合是受控的：open 由输入区持有，工具条只 emit toggle', () => {
        expect(toolbar).toContain('open?: boolean');
        expect(toolbar).toContain("(e: 'toggle', open: boolean): void");
        expect(toolbar).toContain("emit('toggle', !props.open)");
        // 面板渲染读 prop，不读内部 ref —— 否则面板状态会有第二个事实源
        expect(toolbar).toContain('<view v-if="open" class="panel">');
        expect(toolbar).not.toContain('panelOpen');
    });

    it('输入区把 open/toggle 接上，且工具态开关是唯一权威（toolOpen 来自 composable）', () => {
        // 状态机在 composable 里（行数预算），输入区只持实例与别名 —— 别名必须是同一个 ref，不许再造一份
        expect(input).toContain('const tool = useWysiwygToolState({');
        expect(input).toContain('const toolOpen = tool.toolOpen');
        expect(input).not.toContain('const toolOpen = ref<boolean>(false)');
        expect(input).toContain('@toggle="onToolbarToggle"');
        expect(input).toContain('function onToolbarToggle(open : boolean) : void {');
        expect(input).toContain('tool.onToggle(open)');
        expect(toolState).toContain('const toolOpen = ref<boolean>(false)');
        expect(toolState).toContain('function enterToolState() : void {');
        expect(toolState).toContain('function leaveToolState() : void {');
    });

    it('进工具态：先记选区，再收键盘（父规格的拍板顺序不可反）', () => {
        const block = bodyOf(toolState, 'function enterToolState() : void {');
        expect(block).toContain('toolOpen.value = true');
        expect(block).toContain('deps.setReadOnly(true)');
        expect(block).toContain('getSelectionText(');
        expect(block).toContain('uni.hideKeyboard()');
        expect(block.indexOf('getSelectionText(')).toBeLessThan(block.indexOf('uni.hideKeyboard()'));
    });

    it('失焦后的只读保活**只在非工具态**定时放开（工具态可以开很久，放开就丢选区）', () => {
        const block = bodyOf(input, 'function onEditorBlur() : void {');
        expect(block).toContain('if (!toolOpen.value) {');
        expect(block).toMatch(/if \(!toolOpen\.value\) \{\s*setTimeout/);
    });

    it('点编辑区退出工具态：外层 touchstart 早于这次点击的聚焦判定', () => {
        expect(input).toContain('@touchstart="onEditorTouchStart"');
        expect(input).toContain('tool.onEditorTouchStart()');
        const block = bodyOf(toolState, 'function onEditorTouchStart() : void {');
        expect(block).toContain('if (!toolOpen.value) return');
        expect(block).toContain('leaveToolState()');
    });

    it('工具态内动作走选区闸门：快照 → 现场核对 → 取消时提示，不静默落到别处', () => {
        expect(toolState).toContain('function runToolAction(action : RichAction) : void {');
        expect(toolState).toContain('shouldCancelToolAction(snapshot, res.text)');
        expect(toolState).toContain('RICH_SELECTION_LOST_NOTICE');
        // onActionTap 在工具态先分叉，不许直接 applyAction
        const block = bodyOf(toolState, 'function onActionTap(id : string) : void {');
        expect(block).toContain('if (toolOpen.value) {');
        expect(block).toContain('runToolAction(action)');
    });

    it('工具态内执行完动作要把只读收回去（否则「保持选区」在第一次动作后就失效）', () => {
        const block = bodyOf(toolState, 'function runToolAction(action : RichAction) : void {');
        expect(block).toContain('deps.setReadOnly(false)');
        expect(block).toContain('if (toolOpen.value) {');
        expect(block).toContain('deps.setReadOnly(true)');
    });
});
