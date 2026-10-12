/**
 * api/envelope.uts 行为守护（ADR-0037 ④ / #1661 PR-A 交付物）。
 *
 * ③ 门判据（#1156）：「故意弄坏被测物，它会不会红」——本文件**成对取证**：
 *   - 必不红对照：真件（loadUts 真执行 api/envelope.uts）过全部行为断言；
 *   - 必红变异：对真源做定向变异后逐条断言**坏行为真的显现**（非 ReferenceError——
 *     夹具缺绑定只会测到夹具环，变异必须能跑起来并产出坏结果）。
 *
 * 变异清单（每条对应一个 ADR-0037 钉死的行为面）：
 *   M1 hasMore 边界公式坏掉（page*pageSize<total 的 < 变 <=）；
 *   M2 **forum 误读复现**：`data['total']` → `data['pages']`——正是 api/forum.uts:343 的历史误读
 *      （读邻键当 total），真实 total 被丢、hasMore 退化为 items.length==pageSize，必须红；
 *   M3 no-total warn 通道静默（warn 文本被改）；
 *   M4 no-hit warn 通道静默（warn 文本被改）；
 *   M5 decodeList 元素映射死掉（items.push 被掏空）。
 *
 * 依赖真源：toNumber 取自 loadUts(api/helpers.uts)（#1273：不手抄镜像）；
 * getMapped 以 stub 注入（网络出口），bindings 键 = 标识符名（utsHarness 契约，#1404：自有导出不注入）。
 */
const path = require('path');
const fs = require('fs');
const os = require('os');
const uts = require('./utsHarness');

const ROOT = path.resolve(__dirname, '..');
const ENV_UTS = path.join(ROOT, 'api', 'envelope.uts');
const HELPERS_UTS = path.join(ROOT, 'api', 'helpers.uts');

const SHELL_SRC = uts.readText(ENV_UTS);

// 导入面自检：与文件头 import 逐一对齐——错位即夹具坏（bindings 掏空会静默假绿，先 fail 在这里）
expect(uts.importedNames(SHELL_SRC).slice().sort()).toEqual(['getMapped', 'toNumber']);
// 导出面自检：三个函数出口 + DEFAULT_KEYS（export type 被 utsHarness 有意忽略，形态由 ④ 门证）
expect(uts.exportedNames(SHELL_SRC).slice().sort()).toEqual(['DEFAULT_KEYS', 'decodeList', 'decodePage', 'getPaged']);

const toNumber = uts.loadUts(HELPERS_UTS, {}).toNumber;

// getMapped stub：记下 (url, params)，按 FIXTURE 站点回信封；map 仍走真 decodePage（真链路到解码为止）
const FIXTURE = {};
const calls = [];
const stubGetMapped = (url, params, map) => {
    calls.push({ url: url, params: Object.assign({}, params) });
    return Promise.resolve(map(FIXTURE[url] != null ? FIXTURE[url] : {}));
};

const BINDINGS = { getMapped: stubGetMapped, toNumber: toNumber };
const mod = uts.loadUts(ENV_UTS, BINDINGS);

const toItem = (obj) => ({ id: obj['id'] });

/** 变异副本：锚点失效（from 不在源里）即红；落临时目录 loadUts 真执行 */
function mutatedCopy(replacements) {
    let src = SHELL_SRC;
    for (const pair of replacements) {
        expect([pair[0], src.includes(pair[0])]).toEqual([pair[0], true]);
        src = src.split(pair[0]).join(pair[1]);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'envelope-behavior-'));
    const file = path.join(dir, 'envelope.uts');
    fs.writeFileSync(file, src);
    return uts.loadUts(file, BINDINGS);
}

let warnSpy;
beforeEach(() => {
    calls.length = 0;
    for (const k of Object.keys(FIXTURE)) delete FIXTURE[k];
    warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
});
afterEach(() => {
    warnSpy.mockRestore();
});
const warned = (sub) => warnSpy.mock.calls.some((a) => typeof a[0] === 'string' && a[0].includes(sub));

describe('api/envelope.uts 必不红对照（真执行）', () => {
    test('decodePage：total 在 ⇒ hasMore = page*pageSize < total（含边界 30 不再取、31 取）', () => {
        const r30 = mod.decodePage({ replies: [{ id: 1 }], total: 30 }, '/x', 3, 10, toItem, ['replies']);
        expect(r30.hasMore).toBe(false);
        expect(r30.total).toBe(30);
        const r31 = mod.decodePage({ replies: [{ id: 1 }], total: 31 }, '/x', 3, 10, toItem, ['replies']);
        expect(r31.hasMore).toBe(true);
    });

    test('decodePage：total 缺 ⇒ 回落 items.length + no-total warn；10/10 续读、3/10 停、空批次恒假', () => {
        const full = mod.decodePage({ replies: [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }, { id: 5 }, { id: 6 }, { id: 7 }, { id: 8 }, { id: 9 }, { id: 10 }] }, '/x', 1, 10, toItem, ['replies']);
        expect(full.total).toBe(10);
        expect(full.hasMore).toBe(true);
        expect(warned('[envelope] no-total /x')).toBe(true);

        const short = mod.decodePage({ replies: [{ id: 1 }, { id: 2 }, { id: 3 }] }, '/x', 1, 10, toItem, ['replies']);
        expect(short.total).toBe(3);
        expect(short.hasMore).toBe(false);

        const empty = mod.decodePage({ replies: [] }, '/x', 1, 10, toItem, ['replies']);
        expect(empty.items).toEqual([]);
        expect(empty.hasMore).toBe(false);
    });

    test('decodePage：total=0 是合法值（不是缺失）——不打 no-total、hasMore 假', () => {
        const r = mod.decodePage({ replies: [], total: 0 }, '/x', 1, 10, toItem, ['replies']);
        expect(r.total).toBe(0);
        expect(r.hasMore).toBe(false);
        expect(warned('[envelope] no-total')).toBe(false);
    });

    test('decodePage：keys 全落空 ⇒ no-hit warn + 空页（console.warn 不 reject）', () => {
        const r = mod.decodePage({ other: 1 }, '/miss', 1, 10, toItem, ['replies']);
        expect(r.items).toEqual([]);
        expect(r.total).toBe(0);
        expect(r.hasMore).toBe(false);
        expect(warned('[envelope] no-hit /miss')).toBe(true);
    });

    test('decodePage：默认 keys ["items","list"] 兜底 + items 优先于 list', () => {
        const viaList = mod.decodePage({ list: [{ id: 7 }] }, '/x', 1, 10, toItem);
        expect(viaList.items.map((i) => i.id)).toEqual([7]);
        const viaItems = mod.decodePage({ items: [{ id: 8 }], list: [{ id: 9 }] }, '/x', 1, 10, toItem);
        expect(viaItems.items.map((i) => i.id)).toEqual([8]);
    });

    test('decodeList：list 键 → 逐元素 toItem；keys 落空 ⇒ [] + no-hit；空数组在键上 ⇒ [] 不打 no-hit', () => {
        const ok = mod.decodeList({ list: [{ id: 1 }, { id: 2 }] }, '/x', toItem);
        expect(ok.map((i) => i.id)).toEqual([1, 2]);
        const miss = mod.decodeList({}, '/miss', toItem);
        expect(miss).toEqual([]);
        expect(warned('[envelope] no-hit /miss')).toBe(true);
        const empty = mod.decodeList({ list: [] }, '/x', toItem);
        expect(empty).toEqual([]);
        expect(warned('[envelope] no-hit /x')).toBe(false);
    });

    test('getPaged：拼 page/page_size + query（null 值跳过）传给 getMapped，解码走 decodePage（keys 透传）', async () => {
        FIXTURE['/forum/my-replies'] = { replies: [{ id: 1 }, { id: 2 }], total: 11 };
        const out = await mod.getPaged('/forum/my-replies', 2, 10, toItem, { keyword: 'abc', tag: 7, ghost: null }, ['replies']);
        expect(out.items.map((i) => i.id)).toEqual([1, 2]);
        expect(out.total).toBe(11);
        expect(out.hasMore).toBe(2 * 10 < 11);
        const last = calls[calls.length - 1];
        expect(last.url).toBe('/forum/my-replies');
        expect(last.params).toEqual({ page: '2', page_size: '10', keyword: 'abc', tag: '7' });
    });

    test('getPaged：query 缺省不注入多余键', async () => {
        FIXTURE['/plain'] = { items: [], total: 0 };
        await mod.getPaged('/plain', 1, 20, toItem);
        const last = calls[calls.length - 1];
        expect(last.params).toEqual({ page: '1', page_size: '20' });
    });
});

describe('api/envelope.uts 必红变异（坏行为必须显现）', () => {
    test('M1：hasMore 边界公式坏掉（< → <=）⇒ 边界页误判续读', () => {
        const bad = mutatedCopy([['hasMore = page * pageSize < total', 'hasMore = page * pageSize <= total']]);
        const r = bad.decodePage({ replies: [{ id: 1 }], total: 30 }, '/x', 3, 10, toItem, ['replies']);
        expect(r.hasMore).toBe(true);
    });

    test('M2：forum 误读复现（data["total"] → data["pages"]）⇒ 真实 total 丢失、hasMore 退化', () => {
        const bad = mutatedCopy([["const totalRaw = data['total']", "const totalRaw = data['pages']"]]);
        const r = bad.decodePage({ replies: [{ id: 1 }], total: 25 }, '/x', 1, 10, toItem, ['replies']);
        expect(r.total).toBe(1);
        expect(r.hasMore).toBe(false);
    });

    test('M3：no-total warn 通道静默 ⇒ warn 断言红（空页结构不变，牙齿在 warn 上）', () => {
        const bad = mutatedCopy([["'[envelope] no-total '", "'[envelope] MUTATED-NO-TOTAL '"]]);
        bad.decodePage({ replies: [{ id: 1 }] }, '/x', 1, 10, toItem, ['replies']);
        expect(warned('[envelope] no-total /x')).toBe(false);
        expect(warned('[envelope] MUTATED-NO-TOTAL /x')).toBe(true);
    });

    test('M4：no-hit warn 通道静默 ⇒ warn 断言红', () => {
        const bad = mutatedCopy([["'[envelope] no-hit '", "'[envelope] MUTATED-NO-HIT '"]]);
        bad.decodePage({}, '/miss', 1, 10, toItem, ['replies']);
        expect(warned('[envelope] no-hit /miss')).toBe(false);
        expect(warned('[envelope] MUTATED-NO-HIT /miss')).toBe(true);
    });

    test('M5：decodeList 元素映射死掉 ⇒ 返回空数组', () => {
        const bad = mutatedCopy([[
            'for (let i = 0; i < arr.length; i++) {\n        items.push(toItem(arr[i]))\n    }\n    return items',
            'for (let i = 0; i < arr.length; i++) {\n    }\n    return items',
        ]]);
        const out = bad.decodeList({ list: [{ id: 3 }] }, '/x', toItem);
        expect(out).toEqual([]);
    });
});
