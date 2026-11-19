/**
 * 剪贴板粘贴归一 · **行为级**测试（#2658，Parent #2631）
 *
 * 测的是 utils/forumPaste.uts 经 utils/utsHarness.js 去类型后**真执行**的行为：HTML 源串怎么变成
 * 一段能直接进论坛档的纯文本、失焦提示与提示存活窗口的边界在哪。不是手抄镜像，也不测 UVUE 渲染。
 *
 * 为什么这些断言值得存在（第一性原理）：uni-app x 的 <editor> 没有 paste 钩子，Android 原生层把
 * text/html 整段丢弃 —— 富文本粘贴既不进 delta 也不发 @input，序列化器的降级路径永远不触发。
 * 于是这条修复的**唯一**承重点就是本模块：它必须保证「取回来的东西」一定满足论坛档的产物红线
 * （无裸标签、无 &nbsp;、无图片语法），否则修完等于把 HTML 源码塞进了正文。
 *
 * 末组**成对取证**：把实现注入变异真执行，证明判据有牙（坏实现必红）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');

const UTS = path.join(__dirname, 'forumPaste.uts');

/** 每次新实例（模块级状态不串） */
const mod = () => loadUts(UTS, {});

/** 读真源 → 注入变异 → 落临时目录 → 真执行（不改工作树，不进仓） */
function loadMutated(replacements) {
    let src = readText(UTS);
    for (const [from, to] of replacements) {
        expect([from, src.includes(from)]).toEqual([from, true]); // 锚点失效即红：变异必须真进去
        src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'forum-paste-'));
    const file = path.join(dir, 'forumPaste.uts');
    fs.writeFileSync(file, src);
    return loadUts(file, {});
}

const RICH_HTML =
    '<p>标题一</p>' +
    '<p><b>粗体</b> 普通&nbsp;文字</p>' +
    '<ul><li>列表项一</li><li>列表项二</li></ul>' +
    '<p><a href="https://example.com">链接文字</a></p>' +
    '<p><img src="a.png" alt="图"></p>';

// ===== ① HTML 富文本 → 纯文本（AC：不静默、降级落地）=====

describe('HTML 富文本粘成纯文本：段落与列表保住行结构', () => {
    it('标题 / 段落 / 列表项各占一行，行内标签不换行', () => {
        const text = mod().normalizePastedText(RICH_HTML);
        expect(text.split('\n')).toEqual(['标题一', '粗体 普通 文字', '列表项一', '列表项二', '链接文字']);
    });

    it('链接退化成它的文字（URL 不进正文）', () => {
        const text = mod().normalizePastedText(RICH_HTML);
        expect(text).toContain('链接文字');
        expect(text).not.toContain('https://example.com');
    });

    it('<br> / <hr> 也换行（作者按回车就是想要换行）', () => {
        expect(mod().normalizePastedText('第一行<br>第二行')).toBe('第一行\n第二行');
    });

    it('图片丢弃：<img> 一个字节都不进正文（R3：图片只走下方＋）', () => {
        const text = mod().normalizePastedText(RICH_HTML);
        expect(text).not.toContain('a.png');
        expect(text).not.toContain('alt');
        expect(text).not.toContain('![');
    });

    it('产物红线：无裸标签、无 &nbsp;、无残留实体', () => {
        const text = mod().normalizePastedText(RICH_HTML);
        expect(text).not.toContain('<');
        expect(text).not.toContain('>');
        expect(text).not.toContain('&nbsp;');
        expect(text).not.toContain('&amp;');
        expect(text).not.toContain('\u00a0');
    });

    it('居中 / 字号这类剥不掉的属性只留文字（不保留样式声明）', () => {
        const text = mod().normalizePastedText('<p style="text-align:center;font-size:32px">居中标题</p>');
        expect(text).toBe('居中标题');
    });
});

// ===== ② 实体解码：&nbsp; 是论坛档明令禁止的产物 =====

describe('实体解码', () => {
    it('&nbsp; / &#160; / &#xa0; 都归一成普通空格（不留在正文里）', () => {
        const m = mod();
        expect(m.normalizePastedText('a&nbsp;b')).toBe('a b');
        expect(m.normalizePastedText('a&#160;b')).toBe('a b');
        expect(m.normalizePastedText('a&#xa0;b')).toBe('a b');
    });

    it('&amp; / &lt; / &gt; / &quot; / &#39; 解成对应字符', () => {
        const m = mod();
        expect(m.normalizePastedText('a &amp; b')).toBe('a & b');
        expect(m.normalizePastedText('1 &lt; 2 &gt; 0')).toBe('1 < 2 > 0');
    });

    it('不认识的实体原样保留（不猜）', () => {
        expect(mod().normalizePastedText('a &foo; b')).toBe('a &foo; b');
    });
});

// ===== ③ 纯文本分支：不误伤、不误判 =====

describe('纯文本剪贴板：只做归一，不剥内容', () => {
    it('不含标签形态时按原样保留（含 < > 的数学式不误判成 HTML）', () => {
        const m = mod();
        expect(m.normalizePastedText('a < b > c')).toBe('a < b > c');
        expect(m.normalizePastedText('if (a<b) then c>d')).toBe('if (a<b) then c>d');
    });

    it('普通多行文本保留换行', () => {
        expect(mod().normalizePastedText('第一行\n第二行\n\n第四行')).toBe('第一行\n第二行\n\n第四行');
    });

    it('不换行空格（U+00A0）换成普通空格', () => {
        expect(mod().normalizePastedText('普通文本\u00a0带空格')).toBe('普通文本 带空格');
    });

    it('首尾空白去掉；空剪贴板得空串', () => {
        const m = mod();
        expect(m.normalizePastedText('  hello  ')).toBe('hello');
        expect(m.normalizePastedText('')).toBe('');
        expect(m.normalizePastedText('   ')).toBe('');
    });
});

// ===== ④ 图片语法：图片单一入口的兜底 =====

describe('Markdown 图片语法被剥掉（R3：正文不产出 ![alt](url)）', () => {
    it('纯文本里的 ![alt](url) 被移除', () => {
        const text = mod().normalizePastedText('看图 ![alt](https://example.com/a.png) 结束');
        expect(text).not.toContain('![');
        expect(text).not.toContain('a.png');
        expect(text).toContain('看图');
        expect(text).toContain('结束');
    });

    it('不成形的 ![x] 保留（不猜作者意图）', () => {
        expect(mod().normalizePastedText('这不是图片 ![x] 语法')).toBe('这不是图片 ![x] 语法');
    });
});

// ===== ⑤ 两个判据的边界 =====

describe('失焦提示判据 shouldOfferPasteHint：三条件缺一不可', () => {
    it('正文为空 + 剪贴板有字 + 聚焦够久 ⇒ 提示', () => {
        expect(mod().shouldOfferPasteHint(1000, 0, 5)).toBe(true);
    });

    it('正文已有内容 ⇒ 不提示（说明写入成功过）', () => {
        expect(mod().shouldOfferPasteHint(1000, 3, 5)).toBe(false);
    });

    it('剪贴板为空 ⇒ 不提示（没什么可提示的）', () => {
        expect(mod().shouldOfferPasteHint(1000, 0, 0)).toBe(false);
    });

    it('聚焦太短 ⇒ 不提示（点进点出不是粘贴尝试）', () => {
        expect(mod().shouldOfferPasteHint(100, 0, 5)).toBe(false);
    });

    it('门槛正好等于 PASTE_HINT_MIN_FOCUS_MS 算「够久」（边界含等号）', () => {
        const m = mod();
        expect(m.shouldOfferPasteHint(m.PASTE_HINT_MIN_FOCUS_MS, 0, 5)).toBe(true);
        expect(m.shouldOfferPasteHint(m.PASTE_HINT_MIN_FOCUS_MS - 1, 0, 5)).toBe(false);
    });
});

describe('提示存活窗口 shouldClearPasteNotice：粘贴自身那次 @input 不算内容变化', () => {
    it('从未设过提示（setAt=0）⇒ 不该清', () => {
        expect(mod().shouldClearPasteNotice(0, 999999)).toBe(false);
    });

    it('窗口内 ⇒ 不清（粘贴刚落地那一下 @input 不能把自己刚设的提示抹掉）', () => {
        const m = mod();
        expect(m.shouldClearPasteNotice(1000, 1100)).toBe(false);
        expect(m.shouldClearPasteNotice(1000, 1000 + m.PASTE_NOTICE_TTL_MS)).toBe(false);
    });

    it('窗口外 ⇒ 清（之后任何正文变化都让提示消失）', () => {
        const m = mod();
        expect(m.shouldClearPasteNotice(1000, 1000 + m.PASTE_NOTICE_TTL_MS + 1)).toBe(true);
    });
});

// ===== ⑥ 文案常量：三种出口都在，不能只留成功那一条 =====

describe('提示文案：降级 / 空 / 读不到 / 写不进，四条出口都不静默', () => {
    it('四条提示都非空且互不相同', () => {
        const m = mod();
        const notices = [m.PASTE_NOTICE_DEGRADED, m.PASTE_NOTICE_EMPTY, m.PASTE_NOTICE_UNREADABLE, m.PASTE_NOTICE_INSERT_FAILED];
        for (const n of notices) expect(n.length).toBeGreaterThan(0);
        expect(new Set(notices).size).toBe(notices.length);
    });

    it('按钮文案是「粘贴」，失焦提示指向这条按钮', () => {
        const m = mod();
        expect(m.PASTE_LABEL).toBe('粘贴');
        expect(m.PASTE_HINT_AFTER_DROP).toContain(m.PASTE_LABEL);
    });

    it('降级提示点了「纯文本」这件事（AC：作者要知道格式没保住）', () => {
        expect(mod().PASTE_NOTICE_DEGRADED).toContain('纯文本');
    });
});

// ===== ⑦ 成对取证：把坏实现注入真执行，判据必须有牙 =====

describe('成对取证：变异实现必红', () => {
    it('把「有内容就不提示」反过来 ⇒ 正文非空也会提示（判据有牙）', () => {
        const m = loadMutated([['if (contentLength > 0) return false', 'if (contentLength > 0) return true']]);
        expect(m.shouldOfferPasteHint(1000, 3, 5)).toBe(true);
    });

    it('把块级闭合标签的换行改成空格 ⇒ 段落塌成一行（判据有牙）', () => {
        const m = loadMutated([
            ["else if (closing && isBlockTag(name)) out = newline(out)", "else if (closing && isBlockTag(name)) out += ' '"],
        ]);
        expect(m.normalizePastedText('<p>甲</p><p>乙</p>').split('\n').length).toBe(1);
    });

    it('把 &nbsp; 的解码结果换掉 ⇒ 产物里出现别的字符（判据有牙）', () => {
        const m = loadMutated([["if (name == 'nbsp' || name == '#160' || name == '#xa0') return ' '", "if (name == 'nbsp' || name == '#160' || name == '#xa0') return '-'"]]);
        expect(m.normalizePastedText('前&nbsp;后')).toBe('前-后');
    });

    it('把提示存活窗口收到 0 ⇒ 粘贴自身那次 @input 立刻抹掉提示（判据有牙）', () => {
        const m = loadMutated([['return nowMs - setAtMs > PASTE_NOTICE_TTL_MS', 'return nowMs - setAtMs > 0']]);
        expect(m.shouldClearPasteNotice(1000, 1001)).toBe(true);
    });
});
