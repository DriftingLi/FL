/**
 * 回复 composer 提交语义 · 行为级测试（#2642，Parent #2631；ADR-0073）
 *
 * 真执行 composables/useReplyComposer.uts（去类型后当 JS 跑），钉两件事：
 *   ① AC①：回复提交必带 content_format=markdown（作者格式偏好退出输入路径）；
 *   ② AC②：空 / 纯空白禁用提交（trim 闸门 + canSubmitReply），失败只提示不禁用（保真 issues 不进闸门）。
 *
 * 缝 = utils/utsHarness.js 的 loadUts（ADR-0073 唯一测试 seam）。注入的是真件 FORMAT_MARKDOWN
 * （从 forumChainHarness.forumBodyModule() 取），只有 vue / api / picker / uni 是替身 —— 被测的是编排。
 *
 * 成对取证：把提交格式换回 FORMAT_TEXT，证明「markdown」判据有牙。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

const { loadUts, readText } = require('./utsHarness');
const { forumBodyModule } = require('./forumChainHarness');

const COMPOSER = path.join(__dirname, '..', 'composables', 'useReplyComposer.uts');

const vueStub = () => ({
    ref: (v) => ({ value: v }),
    computed: (fn) => ({ get value() { return fn(); } }),
});

function harness(opts = {}) {
    const rec = { calls: [], toast: [] };
    const body = forumBodyModule();
    const uni = { showToast: (o) => { rec.toast.push(o.title); } };
    const picker = () => ({
        images: { value: [] },
        previewImages: { get value() { return []; } },
        uploading: { value: false },
        onAddImage: () => {}, onRemoveImage: () => {}, onPreviewImage: () => {},
    });
    const composer = loadUts(opts.file || COMPOSER, {
        ...vueStub(),
        replyForumTopicApi: (...args) => { rec.calls.push(args); return Promise.resolve({ id: 77 }); },
        useForumImagePicker: picker,
        FORUM_MAX_REPLY_IMAGES: 3,
        FORMAT_MARKDOWN: body.FORMAT_MARKDOWN,
        FORMAT_TEXT: body.FORMAT_TEXT,
        uni,
    });
    return { api: composer.useReplyComposer(), rec };
}

describe('AC①：回复提交固定 content_format=markdown', () => {
    it('replyForumTopicApi 第 4 参 = markdown，content = trim 后草稿', async () => {
        const { api, rec } = harness();
        api.replyContent.value = '  序列化后的正文  ';
        await api.onSubmitReply(42, () => {});
        expect(rec.calls.length).toBe(1);
        expect(rec.calls[0][0]).toBe(42);
        expect(rec.calls[0][1]).toBe('序列化后的正文');
        expect(rec.calls[0][3]).toBe('markdown');
    });

    it('结果类型不再暴露 replyFormat / onFormatChange', () => {
        const { api } = harness();
        expect(api.replyFormat).toBeUndefined();
        expect(api.onFormatChange).toBeUndefined();
    });
});

describe('AC②：空 / 纯空白禁用提交，失败只提示', () => {
    it('初始态 canSubmitReply=false；纯空白仍 false', () => {
        const { api } = harness();
        expect(api.canSubmitReply.value).toBe(false);
        api.replyContent.value = '   \n   ';
        expect(api.canSubmitReply.value).toBe(false);
    });

    it('纯空白点提交不发请求，只 toast', async () => {
        const { api, rec } = harness();
        api.replyContent.value = '   ';
        await api.onSubmitReply(1, () => {});
        expect(rec.calls.length).toBe(0);
        expect(rec.toast).toContain('请输入回复内容');
    });

    it('提交成功后 onPosted 上抛、清空草稿与图片', async () => {
        const { api, rec } = harness();
        let posted = null;
        api.replyContent.value = 'ok';
        await api.onSubmitReply(7, (r) => { posted = r; });
        expect(posted.id).toBe(77);
        expect(api.replyContent.value).toBe('');
        expect(api.replyImages.value).toEqual([]);
    });
});

describe('成对取证：判据有牙', () => {
    it('把提交格式换回 FORMAT_TEXT → markdown 判据必红', async () => {
        const rec = { calls: [], toast: [] };
        const body = forumBodyModule();
        // 变异副本：把 composer 的 FORMAT_MARKDOWN import 与引用都换成 FORMAT_TEXT，真执行
        let src = readText(COMPOSER).split('FORMAT_MARKDOWN').join('FORMAT_TEXT');
        const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reply-bad-'));
        const file = path.join(dir, 'useReplyComposer.uts');
        fs.writeFileSync(file, src);
        const bad = loadUts(file, {
            ...vueStub(),
            replyForumTopicApi: (...args) => { rec.calls.push(args); return Promise.resolve({ id: 1 }); },
            useForumImagePicker: () => ({
                images: { value: [] }, previewImages: { get value() { return []; } }, uploading: { value: false },
                onAddImage: () => {}, onRemoveImage: () => {}, onPreviewImage: () => {},
            }),
            FORUM_MAX_REPLY_IMAGES: 3,
            FORMAT_TEXT: body.FORMAT_TEXT,
            uni: { showToast: () => {} },
        });
        const api = bad.useReplyComposer();
        api.replyContent.value = 'hello';
        await api.onSubmitReply(1, () => {});
        expect(rec.calls[0][3]).toBe('text');
        expect(rec.calls[0][3]).not.toBe('markdown');
    });
});
