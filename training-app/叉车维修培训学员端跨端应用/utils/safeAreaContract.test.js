const { readFileSync, readdirSync, statSync } = require('fs');
const { join } = require('path');

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
        const hits = utsFiles.filter((f) => readFileSync(f, 'utf-8').includes('getMenuButtonBoundingClientRect'));
        expect(hits.length).toBe(1);
        expect(hits[0].replace(/\\/g, '/')).toContain('utils/system.uts');
    });

    it('B+: useSafeArea 从 system.uts 导出且被页头件消费', () => {
        const sys = readFileSync(join(ROOT, 'utils', 'system.uts'), 'utf-8');
        expect(sys).toContain('export function useSafeArea');
        const nav = readFileSync(join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'), 'utf-8');
        expect(nav).toContain("from '../../utils/system.uts'");
    });

    it('C: 状态栏回退值 44 全仓唯一（页内不得出现别的数字字面量回退）', () => {
        const sys = readFileSync(join(ROOT, 'utils', 'system.uts'), 'utf-8');
        expect(sys).toContain('export const DEFAULT_STATUS_BAR_HEIGHT = 44');
        // 页头件 / ai-chat-nav 不允许自带回退数字（44/20 两派收敛到 system.uts）
        for (const f of [
            join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'),
            join(ROOT, 'components', 'ai-chat', 'ai-chat-nav.uvue'),
        ]) {
            const src = readFileSync(f, 'utf-8');
            expect(src).not.toMatch(/statusBarHeight[^;\n]*\b(20|44)\b/);
        }
    });

    it('D: 页头件右槽按胶囊让位（rightSlotWidth 驱动）且 <style lang="scss">', () => {
        const nav = readFileSync(join(ROOT, 'components', 'app-nav-bar', 'app-nav-bar.uvue'), 'utf-8');
        expect(nav).toContain('rightSlotWidth');
        expect(nav).toContain('<style lang="scss">');
        expect(nav).not.toContain('80rpx');
    });

    it('D+: ai-chat-nav 不再是独立页头形态（归一为 AppNavBar 包装）', () => {
        const src = readFileSync(join(ROOT, 'components', 'ai-chat', 'ai-chat-nav.uvue'), 'utf-8');
        expect(src).toContain('app-nav-bar');
        expect(src).not.toContain('<style>');
        expect(src).not.toContain('panel-icon');
    });

    it('渐进: manifest.json 有 app-plus 段（statusbar/safearea）且 appid 未动', () => {
        const raw = readFileSync(join(ROOT, 'manifest.json'), 'utf-8');
        const clean = raw.replace(/\/\*[\s\S]*?\*\//g, '');
        const m = JSON.parse(clean);
        expect(m.appid).toBe('__UNI__1C1D180');
        expect(m['mp-weixin'].appid).toBe('wx38c3e31b16a7ced0');
        expect(m['app-plus']).toBeDefined();
        expect(m['app-plus'].statusbar).toBeDefined();
        expect(m['app-plus'].safearea).toBeDefined();
    });
});
