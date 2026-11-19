/**
 * Delta→Markdown 序列化管线 · **行为级**测试（#2638，Parent #2631）
 *
 * 测的是「编辑器 delta → 论坛档 Markdown → parseMarkdown + splitInlineRuns」这条**真执行链**：
 * utils/forumDeltaMarkdown.uts 经 utils/utsHarness.js 去类型后当 JS 跑；解析链经
 * utils/forumChainHarness.js 载真件 markdown.uts / markdownInline.uts。不是手抄镜像。
 *
 * 断言外部可观察行为：产出 markdown + 论坛档解析出的块/run；不断言内部函数名、私有分支、拼接顺序。
 * 末组**成对取证**（③ 门判据③）：把实现注入变异真执行，证明判据有牙（坏实现必红）。
 *
 * 不建第三腿、不测 UVUE / 微信开发者工具（AC 明定）。契约腿（content_format POST 带 / PUT 不带）
 * 由 utils/forumContentFormatBehavior.test.js 在同一行为腿里守，本文件只钉「序列化器与契约正交」。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText, importedNames } = require('./utsHarness');
const { markdownModule, markdownInlineModule } = require('./forumChainHarness');

const UTS = path.join(__dirname, 'forumDeltaMarkdown.uts');

/** 每次新实例（模块级状态不串） */
const mod = () => loadUts(UTS, {});
const delta = (ops) => JSON.stringify({ ops });

/** 论坛档解析链：真解析器 + 真分词器 */
function chain() {
    const md = markdownModule();
    const inline = markdownInlineModule();
    return {
        md,
        inline,
        blocks: (s) => md.parseMarkdown(s, md.SUBSET_FORUM),
        runs: (b) => (b.text ? inline.splitInlineRuns(b.text) : []),
    };
}

/** 读真源 → 注入变异 → 落临时目录 → 真执行（不改工作树，不进仓） */
function loadMutated(replacements) {
    let src = readText(UTS);
    for (const [from, to] of replacements) {
        expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真进去
        src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'delta-md-'));
    const file = path.join(dir, 'forumDeltaMarkdown.uts');
    fs.writeFileSync(file, src);
    return loadUts(file, {});
}

// ===== ① 记号构造器：4 种 × 有/无选区 = 8/8 =====

describe('记号构造器（字面量逃生舱）：4 种 × 有选区/无选区 8/8', () => {
    it('引用：有选区逐行加 > ，无选区插模板', () => {
        const m = mod();
        expect(m.buildMarker(m.MARKER_QUOTE, '甲\n乙')).toBe('> 甲\n> 乙');
        expect(m.buildMarker(m.MARKER_QUOTE, '')).toBe('> 引用内容');
    });
    it('围栏代码块：有选区包三反引号，无选区插模板', () => {
        const m = mod();
        expect(m.buildMarker(m.MARKER_CODE_BLOCK, 'fn()')).toBe('```\nfn()\n```');
        expect(m.buildMarker(m.MARKER_CODE_BLOCK, '')).toBe('```\n代码块\n```');
    });
    it('行内代码：有选区包反引号，无选区插模板', () => {
        const m = mod();
        expect(m.buildMarker(m.MARKER_CODE_INLINE, 'run')).toBe('`run`');
        expect(m.buildMarker(m.MARKER_CODE_INLINE, '')).toBe('`行内代码`');
    });
    it('链接：有选区包 [文字](URL)，无选区插模板；URL 原样', () => {
        const m = mod();
        expect(m.buildMarker(m.MARKER_LINK, '文字')).toBe('[文字](https://example.com)');
        expect(m.buildMarker(m.MARKER_LINK, '')).toBe('[链接文字](https://example.com)');
    });
    it('记号分组与显示名（二级面板「记号」组，与富文本按钮分开）', () => {
        const m = mod();
        expect(m.MARKER_KINDS).toEqual([m.MARKER_QUOTE, m.MARKER_CODE_BLOCK, m.MARKER_CODE_INLINE, m.MARKER_LINK]);
        expect(m.MARKER_KINDS.map((k) => m.markerLabel(k))).toEqual(['> 引用', '代码块', '行内代码', '链接']);
    });
});

// ===== ② Round-trip：delta → markdown → 论坛档块/run（6 条）=====

describe('round-trip：序列化产物经论坛档解析，块/run 结构对账', () => {
    const c = chain();
    const mdOf = (m, ops) => m.serializeDelta(delta(ops)).markdown;
    const shape = (s) => c.blocks(s).map((b) => b.type);

    it('引用（blockquote 属性挂换行 op）→ quote 块', () => {
        const m = mod();
        const md = mdOf(m, [{ insert: 'quoted' }, { insert: '\n', attributes: { blockquote: true } }]);
        expect([md, shape(md)]).toEqual(['> quoted', ['quote']]);
        expect(c.runs(c.blocks(md)[0])[0].text).toBe('quoted');
    });

    it('围栏（字面记号）→ code 块', () => {
        const m = mod();
        const md = mdOf(m, [{ insert: '```\nfn()\n```' }, { insert: '\n' }]);
        expect(shape(md)).toEqual(['code']);
        expect(c.blocks(md)[0].text).toBe('fn()');
    });

    it('行内代码属性 → paragraph + code run', () => {
        const m = mod();
        const md = mdOf(m, [{ insert: 'use ' }, { insert: 'run', attributes: { code: true } }, { insert: ' here' }, { insert: '\n' }]);
        expect(shape(md)).toEqual(['paragraph']);
        const runs = c.runs(c.blocks(md)[0]);
        expect(runs.map((r) => [r.text, r.code])).toEqual([['use ', false], ['run', true], [' here', false]]);
    });

    it('链接字面记号 → paragraph + link run，URL 原样', () => {
        const m = mod();
        const md = mdOf(m, [{ insert: 'see [文字](https://a.com/x?k=1)' }, { insert: '\n' }]);
        expect(md).toBe('see [文字](https://a.com/x?k=1)');
        expect(shape(md)).toEqual(['paragraph']);
        const linkRun = c.runs(c.blocks(md)[0]).find((r) => r.link);
        expect([linkRun.text, linkRun.link]).toEqual(['文字', 'https://a.com/x?k=1']);
    });

    it('Quill 自动链接 link:true → 纯文本，不成链', () => {
        const m = mod();
        const md = mdOf(m, [{ insert: '自动链接', attributes: { link: true } }, { insert: '\n' }]);
        expect([md, shape(md)]).toEqual(['自动链接', ['paragraph']]);
        expect(c.runs(c.blocks(md)[0]).some((r) => !!r.link)).toBe(false);
    });

    it('富文本 + 记号混排 → paragraph/heading/quote/code/divider/paragraph', () => {
        const m = mod();
        const md = mdOf(m, [
            { insert: 'intro', attributes: { bold: true } }, { insert: '\n' },
            { insert: 'H', attributes: { bold: true } }, { insert: '\n', attributes: { header: 1 } },
            { insert: 'q' }, { insert: '\n', attributes: { blockquote: true } },
            { insert: '```\nfn()\n```' }, { insert: '\n' },
            { insert: { divider: true } }, { insert: '\n' },
            { insert: 'tail' },
        ]);
        expect(shape(md)).toEqual(['paragraph', 'heading', 'quote', 'code', 'divider', 'paragraph']);
        expect(c.runs(c.blocks(md)[0])[0].bold).toBe(true);
    });
});

// ===== ③ 子集安全：降级不误升级 / 拉平引用 / 普通空格 / 禁用构造 =====

describe('子集安全：产物只用论坛档能渲染的记号', () => {
    const c = chain();

    it('下划线属性 → 纯文本且不误升级成粗/斜（并记 style-dropped）', () => {
        const m = mod();
        const r = m.serializeDelta(delta([
            { insert: '下划线', attributes: { underline: true } }, { insert: '\n' },
        ]));
        expect(r.markdown).toBe('下划线');
        expect(r.issues).toContain('style-dropped:underline');
        const runs = c.runs(c.blocks(r.markdown)[0]);
        expect(runs.map((x) => [x.text, x.bold, x.italic, x.strike, x.code]))
            .toEqual([['下划线', false, false, false, false]]);
    });

    it('被编辑器拉平的引用（setContents 拉平成 <p>，无 blockquote 属性）→ paragraph，不成 quote', () => {
        const m = mod();
        // 编辑器不支持引用块：setContents 把它拉平成普通段落，delta 里既无 blockquote 属性也无 '> ' 记号
        const md = m.serializeDelta(delta([{ insert: '引用内容' }, { insert: '\n' }])).markdown;
        expect(md).toBe('引用内容');
        expect(c.blocks(md).map((b) => b.type)).toEqual(['paragraph']);
        expect(c.blocks(md).map((b) => b.type)).not.toContain('quote');
        // 引用只能由字面记号构造器显式产出（不会由富文本属性隐式产生）
        expect(m.buildMarker(m.MARKER_QUOTE, 'x')).toBe('> x');
    });

    it('普通空格原样保留（不写实体）', () => {
        const m = mod();
        const md = m.serializeDelta(delta([{ insert: 'a b  c' }, { insert: '\n' }])).markdown;
        expect(md).toBe('a b  c');
        expect(md.includes('&')).toBe(false);
    });

    it('序列化不生成 HTML / 空格实体 / 正文内嵌图片语法', () => {
        const m = mod();
        // 富文本属性（字号/颜色/对齐等）必须降级为纯文本，不能翻译成 inline style / &nbsp; / HTML 标签
        const md = m.serializeDelta(delta([
            { insert: '彩色文字', attributes: { color: '#f00', size: '24', align: 'center', background: '#ff0' } }, { insert: '\n' },
            { insert: 'a b  c' }, { insert: '\n' },
            { insert: { image: 'http://x/a.png' } }, { insert: '\n' },
        ])).markdown;
        expect(md.includes('<')).toBe(false);   // 不生成 HTML 标签
        expect(md.includes('&nbsp;')).toBe(false); // 普通空格不写实体
        expect(md.includes('style=')).toBe(false); // 不写 inline style
        expect(md.includes('![')).toBe(false);     // 正文不产图片语法（R3）
        expect(md).toBe('彩色文字\na b  c');
    });
});

// ===== ④ 图片 R3 + 空/纯空白 =====

describe('保真检查与提交禁用', () => {
    it('图片 op 被丢弃且经 issues 上报（R3 禁静默丢失）', () => {
        const m = mod();
        const r = m.serializeDelta(delta([
            { insert: '前文' }, { insert: { image: 'http://x/a.png' } }, { insert: '后文' }, { insert: '\n' },
        ]));
        expect(r.issues).toContain('image-dropped');
        expect(r.markdown.includes('![')).toBe(false);
        expect(r.markdown.includes('http://x/a.png')).toBe(false);
    });

    it('空 delta / 纯空白 delta → isEmpty=true（支撑提交禁用）', () => {
        const m = mod();
        expect(m.serializeDelta(delta([])).isEmpty).toBe(true);
        expect(m.serializeDelta(delta([{ insert: '   ' }, { insert: '\n' }])).isEmpty).toBe(true);
        expect(m.serializeDelta(delta([{ insert: '有字' }, { insert: '\n' }])).isEmpty).toBe(false);
        expect(m.serializeDelta('not json').isEmpty).toBe(true);
    });

    it('deltaToMarkdown 兼容封装只取 markdown 串', () => {
        const m = mod();
        const d = delta([{ insert: 'x', attributes: { bold: true } }, { insert: '\n' }]);
        expect(m.deltaToMarkdown(d)).toBe('**x**');
    });
});

// ===== ⑤ 模块契约：自包含纯函数 + 与 content_format 契约正交 =====

describe('模块契约：纯函数、零依赖、不碰 content_format', () => {
    it('importedNames 为空（序列化器零运行期依赖，注入不了坏上游）', () => {
        expect(importedNames(readText(UTS))).toEqual([]);
    });
    it('API 与产物都不含 content_format（契约腿由 forumContentFormatBehavior 守）', () => {
        const m = mod();
        const d = delta([{ insert: 'x', attributes: { bold: true } }, { insert: '\n' }]);
        expect(Object.keys(m).some((k) => k.toLowerCase().includes('format'))).toBe(false);
        expect(m.serializeDelta(d).markdown.includes('content_format')).toBe(false);
    });
});

// ===== ⑥ 成对取证：把实现改坏，判据必须红（证明有牙）=====

describe('成对取证：坏实现必红', () => {
    it('去掉粗体包裹 → round-trip 的 bold run 断言失败', () => {
        const bad = loadMutated([
            ["if (isTrue(attr(attrs, 'bold'))) s = '**' + s + '**'", "if (false) s = '**' + s + '**'"],
        ]);
        const c = chain();
        const mk = (m) => c.runs(c.blocks(m.serializeDelta(delta([
            { insert: 'x', attributes: { bold: true } }, { insert: '\n' },
        ])).markdown)[0]).some((r) => r.bold);
        expect(mk(bad)).toBe(false);  // 坏实现：确实丢粗体
        expect(mk(mod())).toBe(true); // 正实现（对照）：同一 delta 必须是粗体
    });

    it('去掉图片记账 → issues 不再上报 image-dropped', () => {
        const bad = loadMutated([["issues.push(ISSUE_IMAGE)", "void 0"]]);
        const d = delta([{ insert: { image: 'http://x/a.png' } }, { insert: '\n' }]);
        expect(bad.serializeDelta(d).issues).toEqual([]);       // 坏实现：静默丢失
        expect(mod().serializeDelta(d).issues).toContain('image-dropped'); // 正实现：上报
    });

    it('把链接 URL 改坏 → 构造器输出变化（证明 8/8 判据有牙）', () => {
        const bad = loadMutated([
            ["return '[' + selected + '](https://example.com)'", "return '[' + selected + ']()'"],
        ]);
        expect(bad.buildMarker(bad.MARKER_LINK, '文字')).toBe('[文字]()');
        expect(mod().buildMarker(mod().MARKER_LINK, '文字')).toBe('[文字](https://example.com)');
    });
});

// ===== ⑦ 记号替换选区：写完之后把文档收敛到唯一正确形态（#2659）=====

describe('记号写入选区：有选区一律替换、不重复（#2659 真机缺陷形收敛）', () => {
    it('实测缺陷形（写在选区前、原文没删）→ 收敛成 "> abc"', () => {
        const m = mod();
        const repaired = m.reconcileMarkerInsert(delta([{ insert: 'abc' }]), 'abc', '> abc', delta([{ insert: '> abcabc' }]));
        expect(repaired).not.toBeNull();
        expect(m.deltaPlainText(repaired)).toBe('> abc');
    });

    it('平台语义正常（就地替换）→ 返回 null，绝不再改一遍', () => {
        const m = mod();
        expect(m.reconcileMarkerInsert(delta([{ insert: 'abc' }]), 'abc', '> abc', delta([{ insert: '> abc' }]))).toBeNull();
    });

    it('无选区（插模板）→ 返回 null，替换是选区专属语义', () => {
        const m = mod();
        expect(m.reconcileMarkerInsert(delta([{ insert: 'abc' }]), '', '> 引用内容', delta([{ insert: 'abc> 引用内容' }]))).toBeNull();
    });

    it('选区在原文中段且唯一 → 收敛为就地替换，前后文原样', () => {
        const m = mod();
        const repaired = m.reconcileMarkerInsert(
            delta([{ insert: '前言 abc 后语' }]), 'abc', '`abc`',
            delta([{ insert: '前言 `abc`abc 后语' }]),
        );
        expect(m.deltaPlainText(repaired)).toBe('前言 `abc` 后语');
    });

    it('收敛保留选区外的行内属性（粗体不被冲掉）', () => {
        const m = mod();
        const repaired = m.reconcileMarkerInsert(
            delta([{ insert: '粗体', attributes: { bold: true } }, { insert: '普通abc' }]), 'abc', '`abc`',
            delta([
                { insert: '粗体', attributes: { bold: true } },
                { insert: '普通' },
                { insert: '`abc`' },
                { insert: 'abc' },
            ]),
        );
        expect(m.deltaPlainText(repaired)).toBe('粗体普通`abc`');
        expect(m.deltaToMarkdown(repaired)).toBe('**粗体**普通`abc`');
    });

    it('选区文本重复时靠「写在哪儿」反推落点（去掉 marker 必须还原成原文）', () => {
        const m = mod();
        const repaired = m.reconcileMarkerInsert(delta([{ insert: 'ab ab' }]), 'ab', '> ab', delta([{ insert: '> abab ab' }]));
        expect(m.deltaPlainText(repaired)).toBe('> ab ab');
    });

    it('形状不认识（既不是已正确也不是缺陷形）→ 返回 null，宁可不修不可误删', () => {
        const m = mod();
        expect(m.reconcileMarkerInsert(delta([{ insert: 'abc' }]), 'abc', '> abc', delta([{ insert: 'xyz' }]))).toBeNull();
        // 选区文本与原文对不上（脏输入）同样不动手
        expect(m.reconcileMarkerInsert(delta([{ insert: 'abc' }]), 'zz', '> zz', delta([{ insert: '> zzabc' }]))).toBeNull();
    });

    it('deltaPlainText 只累加文本 op（图片/分割线零宽度）', () => {
        const m = mod();
        expect(m.deltaPlainText(delta([{ insert: 'a' }, { insert: { image: 'http://x/a.png' } }, { insert: 'b' }]))).toBe('ab');
        expect(m.deltaPlainText('not json')).toBe('');
    });
});

describe('收敛判据的成对取证：拆掉安全闸必须红', () => {
    it('去掉「形状不认识就不动手」→ 陌生形状被误改写（证明闸有牙）', () => {
        const bad = loadMutated([['if (postText != inserted) return null', 'if (false) return null']]);
        const args = [delta([{ insert: 'abc' }]), 'abc', '> abc', delta([{ insert: 'xyz' }])];
        expect(bad.reconcileMarkerInsert(...args)).not.toBeNull();  // 坏实现：动手了
        expect(mod().reconcileMarkerInsert(...args)).toBeNull();      // 正实现：不动手
    });
});

