const { readdirSync, statSync } = require('fs');
const { join } = require('path');
// 读取层归一（#1177 / ADR-0019 票 B）：仓内源码一律经 utsHarness.readText（唯一真源 normalizeEol）
const { readText } = require('./utsHarness');

// #1602 骨架层基建机检（契约 B/C/D + 渐进）：
// B. getMenuButtonBoundingClientRect 唯一真源 = utils/system.uts
// C. 状态栏回退值全仓唯一（DEFAULT_STATUS_BAR_HEIGHT = 44，消除 44/20 两派）
// D. 页头件满足胶囊让位（rightSlotWidth 来自 useSafeArea）且页头形态无第二份
// 渐进：manifest.json 有 app-plus 段且 appid 未改脏

const ROOT = join(__dirname, '..');

function walk(dir, ext, out) {
    if (out === undefined) out = [];
    for (const name of readdirSync(dir)) {
        if (name === 'node_modules' || name === '.git' || name === 'unpackage') continue;
        const p = join(dir, name);
        const st = statSync(p);
        if (st.isDirectory()) {
            walk(p, ext, out);
        } else if (name.endsWith(ext)) {
            out.push(p);
        }
    }
    return out;
}

describe('#1602 骨架层基建契约', () => {
    it('B: getMenuButtonBoundingClientRect 只在 utils/system.uts 出现（唯一真源）', () => {
        const utsFiles = walk(ROOT, '.uts');
        const hits = utsFiles.filter((f) => readText(f).includes('getMenuButtonBoundingClientRect'));
        expect(hits.length).toBe(1);
        expect(hits[0].replace(/\\/g, '/')).toContain('utils/system.uts');
    });

    it('B+: useSafeArea 从 system.uts 导出且被页头件消费', () => {
        const sys = readText(join(ROOT, 'utils', 'system.uts'));
        expect(sys).toContain('export function useSafeArea');
        const nav = readText(join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'));
        expect(nav).toContain("from '../../utils/system.uts'");
    });

    it('C: 状态栏回退值 44 全仓唯一（页内不得出现别的数字字面量回退）', () => {
        const sys = readText(join(ROOT, 'utils', 'system.uts'));
        expect(sys).toContain('export const DEFAULT_STATUS_BAR_HEIGHT = 44');
        // 页头件 / ai-chat-nav 不允许自带回退数字（44/20 两派收敛到 system.uts）
        for (const f of [
            join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'),
            join(ROOT, 'components', 'ai-chat', 'ai-chat-nav.uvue'),
        ]) {
            const src = readText(f);
            expect(src).not.toMatch(/statusBarHeight[^;\n]*\b(20|44)\b/);
        }
    });

    it('D: 页头件右槽按胶囊让位（rightSlotWidth 驱动）且 <style lang="scss">', () => {
        const nav = readText(join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'));
        expect(nav).toContain('rightSlotWidth');
        expect(nav).toContain('<style lang="scss">');
        expect(nav).not.toContain('80rpx');
    });

    it('D++: 胶囊让位含内容盒偏移补偿（右缘 ≤ 胶囊左边线−间隙，不落胶囊矩形）', () => {
        // 假绿教训（#1602 真链路复验）：只设 width 时右槽在 padding 后的内容盒里右移 24rpx，
        // 图标落进胶囊矩形。几何口径：margin-right = (750 − paddingX) − (capsuleLeft − clearance)，
        // 且 paddingX 必须与 scss .nav-bar-body 的实际 padding 一致。
        const nav = readText(join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'));
        expect(nav).toContain('margin-right');
        expect(nav).toMatch(/capsuleLeft\s*-\s*RIGHT_SLOT_CLEARANCE|RIGHT_SLOT_CLEARANCE/);
        // padding 常量与样式串用（$spacing-md = 24rpx，见 uni.scss）
        const scss = readText(join(ROOT, 'uni.scss'));
        const m = scss.match(/\$spacing-md:\s*(\d+)rpx/);
        expect(m).not.toBeNull();
        expect(Number(m[1])).toBe(24);
        expect(nav).toMatch(/NAV_BODY_PADDING_X\s*=\s*24/);
        expect(nav).toMatch(/padding:\s*0\s+\$spacing-md/);
    });

    it('D+: ai-chat-nav 不再是独立页头形态（归一为 AppNavBar 包装）', () => {
        const src = readText(join(ROOT, 'components', 'ai-chat', 'ai-chat-nav.uvue'));
        expect(src).toContain('app-nav-bar');
        expect(src).not.toContain('<style>');
        expect(src).not.toContain('panel-icon');
    });

    it('渐进: manifest.json 有 app-plus 段（statusbar/safearea）且 appid 未动', () => {
        const raw = readText(join(ROOT, 'manifest.json'));
        const clean = raw.replace(/\/\*[\s\S]*?\*\//g, '');
        const m = JSON.parse(clean);
        expect(m.appid).toBe('__UNI__1C1D180');
        expect(m['mp-weixin'].appid).toBe('wx38c3e31b16a7ced0');
        expect(m['app-plus']).toBeDefined();
        expect(m['app-plus'].statusbar).toBeDefined();
        expect(m['app-plus'].safearea).toBeDefined();
    });
});
