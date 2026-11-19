/**
 * WYSIWYG 输入区**形态契约**（#2640，Parent #2631；ADR-0073）
 *
 * ## 这是**接线守护**，不是 ③ 门证据（`docs/agents/guards.md`）
 *
 * 它守的是「形态接线没有被悄悄拆掉」：常驻三键还在、触摸目标没被缩小、编辑器侧没有图片入口、
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
 * 常驻 3 枚 = 粗体/斜体/删除线（下划线永久退出、行内代码不作常驻）；二级面板两组；
 * 触摸目标 ≥88rpx（R2）；面板容量余量 = 0（R1）；点亮以 getContents 的 delta 为准；
 * 记号走 insertText；编辑器侧无图片入口、正文图片丢弃并**持续可见**提示（R3）；
 * 失焦后点按钮走 readOnly 绕行；键盘高度上报给宿主、组件不自加偏移。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const INPUT_REL = 'components/forum-wysiwyg-input.uvue';
const TOOLBAR_REL = 'components/forum-wysiwyg-toolbar.uvue';
const MODEL_REL = 'utils/forumWysiwygInput.uts';

const input = read(INPUT_REL);
/** #2658：格式工具条（常驻三键 + 粘贴 + 面板）拆成独立件，按钮集与触摸目标的判据落在它身上。 */
const toolbar = read(TOOLBAR_REL);
const model = read(MODEL_REL);

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

/** 去掉 HTML 注释与块注释：注释里点名按钮中文名不算「模板硬编码文案」。 */
function stripComments(src) {
    return src.replace(/<!--[\s\S]*?-->/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
}

// ===== ① 常驻三键：数据驱动，恰好三枚 =====

describe('常驻三键由数据驱动（按钮集是单点，不是模板硬编码）', () => {
    it('工具条遍历 richPrimaryActions 的结果，不自己列按钮', () => {
        expect(toolbar).toContain('const primaryActions : RichAction[] = richPrimaryActions()');
        expect(toolbar).toContain('v-for="(act, idx) in primaryActions"');
        expect(toolbar).toContain('@click="onAction(act.id)"');
        // 拆分的接缝：工具条只 emit，输入区持有 editorCtx 才决定怎么落地
        expect(toolbar).toContain("(e: 'action', id: string): void");
        expect(toolbar).toContain("emit('action', id)");
        expect(input).toContain('<ForumWysiwygToolbar :active-ids="activeIds" @action="onActionTap" @paste="onPasteTap" />');
    });

    it('模板里不出现中文按钮名（文案单点在 utils/forumWysiwygInput）', () => {
        for (const src of [stripComments(input), stripComments(toolbar)]) {
            for (const label of ['粗体', '斜体', '删除线', '引用', '代码块', '行内代码', '链接']) {
                expect([label, src.includes(label)]).toEqual([label, false]);
            }
        }
        expect(stripComments(toolbar)).toContain('{{ act.label }}');
    });

    it('下划线不在按钮集里（永久退出），行内代码不作常驻', () => {
        expect(model).not.toContain("RICH_ACTION_UNDERLINE");
        expect(model).not.toContain("formatAction('underline'");
        // 行内代码只出现在面板「记号」组，不在 richPrimaryActions 里
        const primary = bodyOf(model, 'export function richPrimaryActions()');
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

    it('「更多」键只加宽不加高（120rpx × 88rpx）', () => {
        expect(toolbar).toMatch(/\.tool-btn-more \{[\s\S]*?width: 120rpx;/);
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
        expect(input).toContain('@click="onAddImage"');
        expect(input).toContain("emit('add-image')");
        expect(input).toContain("'＋'");
        // 双入口（拍照 / 相册各一枚）已退场
        expect(input).not.toContain("onAddImage('camera')");
        expect(input).not.toContain("onAddImage('album')");
    });

    it('丢弃提示是**持续可见**的一行，不是 toast', () => {
        // #2641：宿主可关掉本行自己渲染（发帖页要把它放到提交按钮附近，AC③）——两处只出现一次
        expect(input).toContain('<text v-if="showFidelityNotice && fidelityNotice.length > 0" class="fidelity-notice">');
        expect(input).toContain('richFidelityNotice(issues.value)');
        expect(input).not.toContain('showToast');
        expect(input).not.toContain('showModal');
    });
});

// ===== ④ 记号逃生舱：一律 insertText，不经 editor 的 link/code 属性 =====

describe('记号四件走 insertText 逃生舱（editor 的 link 丢 URL、code 是 no-op）', () => {
    it('有选区包裹 / 无选区插模板，都经 buildMarker + insertText', () => {
        expect(input).toContain('ctx.getSelectionText({');
        expect(input).toContain('buildMarker(action.id, res.text)');
        expect(input).toContain('ctx.insertText({');
    });

    it('不调用 insertLink（链接输出 [文字](URL) 由作者自填 URL）', () => {
        expect(input).not.toContain('insertLink');
    });

    it('分割线走 insertDivider，格式走 format', () => {
        expect(input).toContain('ctx.insertDivider({');
        expect(input).toContain('ctx.format(action.formatName, action.formatValue)');
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
        const tap = bodyOf(input, 'function onActionTap(');
        expect(tap).toContain('if (readOnly.value) {');
        expect(tap).toContain('setTimeout(() => {');
        expect(tap).toContain('applyAction(action)');
        expect(tap).toContain('readOnly.value = false');
    });

    // #2658：只读态会挡住「点编辑器重新聚焦」。一次被原生层丢弃的粘贴会让编辑器失焦，
    // 只读若一直挂着，作者就再也点不进去，只能退出重进。
    it('只读态必须自己放开：失焦后 READONLY_HOLD_MS 到点恢复可编辑', () => {
        const blur = bodyOf(input, 'function onEditorBlur()');
        expect(blur).toContain('readOnly.value = true');
        expect(blur).toContain('setTimeout(() => { readOnly.value = false }, READONLY_HOLD_MS)');
        expect(input).toContain('const READONLY_HOLD_MS : number =');
    });
});

// ===== ⑥b 粘贴：原生粘贴丢弃 text/html，改走「粘贴」按钮 → insertText（#2658）=====

describe('粘贴走 insertText 同一条 content-change 路径（原生粘贴会整段丢弃 text/html）', () => {
    it('常驻行有「粘贴」按钮，文案取自 utils/forumPaste 不硬编码', () => {
        expect(toolbar).toContain('@click="onPaste"');
        expect(toolbar).toContain('{{ pasteLabel }}');
        expect(toolbar).toContain('const pasteLabel : string = PASTE_LABEL');
        // 接缝两端都在：工具条 emit paste，输入区把粘贴落地
        expect(toolbar).toContain("emit('paste')");
        expect(input).toContain('@paste="onPasteTap"');
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

    it('降级提示是持续可见的一行，不是 toast', () => {
        expect(input).toContain('<text v-if="pasteNotice.length > 0" class="paste-notice">');
        expect(toolbar).not.toContain('showToast');
        expect(toolbar).not.toContain('showModal');
        expect(input).toContain('<view v-if="pasteHint.length > 0" class="paste-hint-row" @click="onPasteTap">');
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
        expect(input).toContain('defineExpose({ insertMention })');
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
