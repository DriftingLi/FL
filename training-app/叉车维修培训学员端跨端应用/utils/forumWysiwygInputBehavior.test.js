/**
 * WYSIWYG 输入区按钮集 / 回显判据 / 保真提示 · **行为级**测试（#2640，Parent #2631）
 *
 * 测的是 utils/forumWysiwygInput.uts 经 utils/utsHarness.js 去类型后**真执行**的行为：
 * 按钮集是什么、点亮怎么判、降级怎么告诉作者。不是手抄镜像，也不测 UVUE 渲染
 * （AC 明定：唯一测试 seam 是纯 .uts 模块，不测微信开发者工具、不新建测试腿）。
 *
 * 为什么这些断言值得存在（第一性原理）：格式边界由**三处共守**（ADR-0073），按钮集是其中唯一
 * 面向作者的一处 —— 它多一枚论坛档渲染不出的按钮，作者就会「点了白点」。故这里钉三件事：
 *   ① 常驻恰三枚、面板两组容量余量 = 0；
 *   ② 按钮集 ⊆ 论坛档可表达格式（与 T1 序列化器的 MARKER_KINDS 同值，跨文件对齐）；
 *   ③ 点亮判据取 delta 的**累积快照**，不取 statuschange 的 detail。
 * 末组**成对取证**：把实现注入变异真执行，证明判据有牙（坏实现必红）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const UTS = path.join(__dirname, 'forumWysiwygInput.uts');
const DELTA_UTS = path.join(__dirname, 'forumDeltaMarkdown.uts');

/** 每次新实例（模块级状态不串） */
const mod = () => loadUts(UTS, {});
const delta = (ops) => JSON.stringify({ ops });

/** 读真源 → 注入变异 → 落临时目录 → 真执行（不改工作树，不进仓） */
function loadMutated(replacements) {
    let src = readText(UTS);
    for (const [from, to] of replacements) {
        expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真进去
        src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wysiwyg-input-'));
    const file = path.join(dir, 'forumWysiwygInput.uts');
    fs.writeFileSync(file, src);
    return loadUts(file, {});
}

// ===== ① 常驻四键：粗体 / 斜体 / 删除线 / 列表（加「更多」开关 = 5 个触摸目标）=====

describe('常驻四键：粗体 / 斜体 / 删除线 / 列表（恰好四枚）', () => {
    it('恰好四枚，顺序固定', () => {
        const m = mod();
        expect(m.richPrimaryActions().map((a) => a.id)).toEqual(['bold', 'italic', 'strike', 'list']);
    });

    it('下划线永久退出、行内代码不作常驻', () => {
        const m = mod();
        const ids = m.richPrimaryActions().map((a) => a.id);
        expect(ids).not.toContain('underline');
        expect(ids).not.toContain('code-inline');
        expect(ids).not.toContain('code');
    });

    it('四枚都是 format 动作、formatValue 为 null（切换语义）', () => {
        const m = mod();
        for (const act of m.richPrimaryActions()) {
            expect(act.kind).toBe(m.RICH_KIND_FORMAT);
            expect(act.formatName).toBe(act.id);
            expect(act.formatValue).toBeNull();
            expect(act.label.length).toBeGreaterThan(0);
        }
    });

    it('四枚都带字形与真样式（B 加粗 / I 斜体 / S 删除线 / ≡ 纯字形）', () => {
        const m = mod();
        const acts = m.richPrimaryActions();
        expect(acts.map((a) => a.glyph)).toEqual(['B', 'I', 'S', '≡']);
        expect(acts.map((a) => a.glyphStyle)).toEqual([
            m.RICH_GLYPH_STYLE_BOLD,
            m.RICH_GLYPH_STYLE_ITALIC,
            m.RICH_GLYPH_STYLE_STRIKE,
            m.RICH_GLYPH_STYLE_PLAIN,
        ]);
        for (const act of acts) {
            expect(act.icon).toBe('');
        }
    });

    it('触摸目标下限是 88rpx（R2）', () => {
        expect(mod().RICH_TOUCH_TARGET_RPX).toBe(88);
    });
});

// ===== ② 二级面板：两组、容量余量 = 0、记号组与 T1 同值 =====

describe('二级面板：格式组 + 记号组', () => {
    it('两组，id 与顺序固定', () => {
        const m = mod();
        expect(m.richPanelGroups().map((g) => g.id)).toEqual(['format', 'marker']);
        expect(m.richPanelGroups().map((g) => g.label)).toEqual(['格式', '记号']);
    });

    it('格式组 = 标题 / 列表 / 分割线（容量余量 0）', () => {
        const m = mod();
        const group = m.richPanelGroups()[0];
        expect(group.actions.map((a) => a.id)).toEqual(['header', 'list', 'divider']);
        expect(group.actions[0].kind).toBe(m.RICH_KIND_FORMAT);
        expect(group.actions[1].kind).toBe(m.RICH_KIND_FORMAT);
        expect(group.actions[2].kind).toBe(m.RICH_KIND_DIVIDER);
    });

    it('记号组 = 引用 / 代码块 / 行内代码 / 链接，且 id 与 T1 的 MARKER_KINDS 同值', () => {
        const m = mod();
        const t1 = loadUts(DELTA_UTS, {});
        const group = m.richPanelGroups()[1];
        expect(group.actions.map((a) => a.id)).toEqual(['quote', 'code-block', 'code-inline', 'link']);
        expect(group.actions.map((a) => a.id)).toEqual(t1.MARKER_KINDS);
        for (const act of group.actions) {
            expect(act.kind).toBe(m.RICH_KIND_MARKER);
            expect(act.formatName).toBe('');
        }
    });

    it('面板每项都带中文名与图标字形（#2689 决策 2：不能只有符号）', () => {
        const m = mod();
        for (const group of m.richPanelGroups()) {
            for (const act of group.actions) {
                expect(act.label.length).toBeGreaterThan(0);
                expect(act.icon.length).toBeGreaterThan(0);
                expect(act.glyph).toBe('');
            }
        }
    });

    it('按钮集 ⊆ 论坛档可表达格式：不出现下划线 / 颜色 / 对齐 / 字号 / 上下标 / 缩进', () => {
        const m = mod();
        const all = m.richPrimaryActions().concat(
            m.richPanelGroups().reduce((acc, g) => acc.concat(g.actions), [])
        );
        // 列表同时是常驻键与面板项（常驻是快捷入口、面板是全菜单），去重后再比
        const formatNames = Array.from(new Set(
            all.filter((a) => a.kind === m.RICH_KIND_FORMAT).map((a) => a.formatName)
        ));
        expect(formatNames.sort()).toEqual(['bold', 'header', 'italic', 'list', 'strike']);
        for (const banned of ['underline', 'color', 'backgroundColor', 'align', 'script', 'indent', 'size']) {
            expect(formatNames).not.toContain(banned);
        }
    });
});

// ===== ③ 按 id 取动作 =====

describe('richActionById：常驻与面板全表可查', () => {
    it('常驻与面板的每个 id 都能取到，且 kind 一致', () => {
        const m = mod();
        expect(m.richActionById('bold').kind).toBe(m.RICH_KIND_FORMAT);
        expect(m.richActionById('divider').kind).toBe(m.RICH_KIND_DIVIDER);
        expect(m.richActionById('link').kind).toBe(m.RICH_KIND_MARKER);
        expect(m.richActionById('code-inline').label).toBe('行内代码');
    });

    it('未知 id 返回 null（模板不自己拼动作对象）', () => {
        const m = mod();
        expect(m.richActionById('underline')).toBeNull();
        expect(m.richActionById('')).toBeNull();
    });
});

// ===== ④ 回显判据：一律取 delta，累积语义 =====

describe('richActiveFromDelta：点亮集合由 delta 推导', () => {
    it('空 delta / 非法 JSON / 无 ops 都返回空集', () => {
        const m = mod();
        expect(m.richActiveFromDelta(delta([]))).toEqual([]);
        expect(m.richActiveFromDelta('not json')).toEqual([]);
        expect(m.richActiveFromDelta(JSON.stringify({}))).toEqual([]);
    });

    it('累积语义：bold + italic 同时生效时两枚都点亮', () => {
        const m = mod();
        const d = delta([
            { insert: '甲', attributes: { bold: true, italic: true } },
            { insert: '\n' },
        ]);
        expect(m.richActiveFromDelta(d).sort()).toEqual(['bold', 'italic']);
    });

    it('行内格式挂文字 op：删除线 / 行内代码', () => {
        const m = mod();
        expect(m.richActiveFromDelta(delta([{ insert: '甲', attributes: { strike: true } }, { insert: '\n' }])))
            .toEqual(['strike']);
        expect(m.richActiveFromDelta(delta([{ insert: '甲', attributes: { code: true } }, { insert: '\n' }])))
            .toEqual(['code-inline']);
    });

    it('块级格式挂换行 op：标题 / 列表 / 引用', () => {
        const m = mod();
        expect(m.richActiveFromDelta(delta([{ insert: '甲' }, { insert: '\n', attributes: { header: 2 } }])))
            .toEqual(['header']);
        expect(m.richActiveFromDelta(delta([{ insert: '甲' }, { insert: '\n', attributes: { list: 'bullet' } }])))
            .toEqual(['list']);
        expect(m.richActiveFromDelta(delta([{ insert: '甲' }, { insert: '\n', attributes: { blockquote: true } }])))
            .toEqual(['quote']);
    });

    it('行内 + 块级同时生效：两处属性都读到', () => {
        const m = mod();
        const d = delta([
            { insert: '甲', attributes: { bold: true } },
            { insert: '\n', attributes: { header: 1 } },
        ]);
        expect(m.richActiveFromDelta(d).sort()).toEqual(['bold', 'header']);
    });

    it('取最后一个文本 op：光标处的格式才是点亮项', () => {
        const m = mod();
        const d = delta([
            { insert: '甲', attributes: { bold: true } },
            { insert: '乙', attributes: { italic: true } },
            { insert: '\n' },
        ]);
        expect(m.richActiveFromDelta(d)).toEqual(['italic']);
    });

    it('attributes 为 null 时不点亮任何键', () => {
        const m = mod();
        expect(m.richActiveFromDelta(delta([{ insert: '甲', attributes: null }, { insert: '\n' }]))).toEqual([]);
    });
});

// ===== ⑤ 保真提示：持续可见的一行文案 =====

describe('richFidelityNotice：降级项翻成作者看得懂的话', () => {
    it('空 issues → 空串（组件据此不渲染该行）', () => {
        expect(mod().richFidelityNotice([])).toBe('');
    });

    it('正文图片被丢弃 → R3 的持续可见提示', () => {
        const m = mod();
        expect(m.richFidelityNotice(['image-dropped'])).toBe(m.RICH_IMAGE_NOTICE);
        expect(m.RICH_IMAGE_NOTICE).toBe('正文图片已忽略，请用下方＋添加');
    });

    it('样式降级 → 中文名 + 「已降级为纯文本」', () => {
        const m = mod();
        expect(m.richFidelityNotice(['style-dropped:underline'])).toBe('下划线已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:align'])).toBe('对齐已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:size'])).toBe('字号已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:color'])).toBe('颜色已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:background'])).toBe('背景色已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:script'])).toBe('上下标已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:indent'])).toBe('缩进已降级为纯文本');
    });

    it('同类降级去重、多项用顿号连接', () => {
        const m = mod();
        expect(m.richFidelityNotice(['style-dropped:underline', 'style-dropped:underline']))
            .toBe('下划线已降级为纯文本');
        expect(m.richFidelityNotice(['style-dropped:underline', 'style-dropped:color']))
            .toBe('下划线、颜色已降级为纯文本');
    });

    it('图片与样式同时出现：用分号连接，图片提示在后', () => {
        const m = mod();
        expect(m.richFidelityNotice(['image-dropped', 'style-dropped:underline']))
            .toBe('下划线已降级为纯文本；' + m.RICH_IMAGE_NOTICE);
    });

    it('未知属性原样透出（不吞掉新出现的降级项）', () => {
        expect(mod().richFidelityNotice(['style-dropped:fontFamily'])).toBe('fontFamily已降级为纯文本');
    });
});

// ===== ⑥ 成对取证：坏实现必红 =====

describe('成对取证：判据有牙', () => {
    it('常驻四键被换成下划线 → 「不含 underline」判据必红', () => {
        const bad = loadMutated([
            [
                "formatAction(RICH_ACTION_STRIKE, '删除线', 'S', RICH_GLYPH_STYLE_STRIKE, '')",
                "formatAction('underline', '下划线', 'U', RICH_GLYPH_STYLE_PLAIN, '')",
            ],
        ]);
        expect(bad.richPrimaryActions().map((a) => a.id)).toContain('underline');
    });

    it('点亮判据改成只读最后一个 op → 行内格式丢失，累积判据必红', () => {
        const bad = loadMutated([
            ['        inlineAttrs = attrs\n        blockAttrs = attrs', '        blockAttrs = attrs'],
        ]);
        const d = delta([
            { insert: '甲', attributes: { bold: true } },
            { insert: '\n', attributes: { header: 1 } },
        ]);
        expect(bad.richActiveFromDelta(d)).toEqual(['header']);
    });

    it('保真提示丢掉图片项 → R3 判据必红', () => {
        const bad = loadMutated([['            hasImage = true', '            hasImage = false']]);
        expect(bad.richFidelityNotice(['image-dropped'])).toBe('');
    });
});
