/**
 * 验收模块接线契约（#2743）
 *
 * 守「页面—状态层—API—mock」这条链的接线与 uvue 样式红线：
 * - 四页都在 pages.json 的子包里注册，且入口能从「我的」点进去
 * - 页面只消费 composable，不直连 api / 不裸发请求
 * - 提交前的校验口径只有 utils/acceptanceForm 一处
 * - mock 开关只有一个，切到真实接口时页面零改动
 * - `<style>` 必须是 scss、只写 class 选择器、不用原生端不支持的属性
 * - 模板 class 与样式定义双向对得上（漏 class = 样式静默不生效）
 */
const fs = require('fs');
const path = require('path');
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const PAGES = ['acceptance-list', 'acceptance-detail', 'acceptance-submit', 'acceptance-history'];
const pageRel = (name) => 'pages/acceptance/' + name + '.uvue';

function styleBlock(src) {
  const m = src.match(/<style([^>]*)>([\s\S]*?)<\/style>/);
  return { attrs: m ? m[1] : '', body: m ? m[2] : '' };
}

function templateClasses(src) {
  const names = new Set();
  const tpl = src.split('<script')[0];
  // 先摘掉 :class（否则 `:class="` 里的 `class="` 会被当成静态 class 捕进来）
  const staticOnly = tpl.replace(/:class="[^"]*"/g, '');
  for (const m of staticOnly.matchAll(/\bclass="([^"]*)"/g)) {
    m[1].split(/\s+/).filter(Boolean).forEach((n) => names.add(n));
  }
  for (const m of tpl.matchAll(/:class="([^"]*)"/g)) {
    // 只取对象**键**（`'x': 条件`），条件里的字符串字面量不是 class 名
    for (const q of m[1].matchAll(/'([^']+)'\s*:/g)) names.add(q[1]);
  }
  return names;
}

function definedClasses(body) {
  const names = new Set();
  for (const m of body.matchAll(/\.([A-Za-z0-9_-]+)\s*\{/g)) names.add(m[1]);
  return names;
}

describe('页面注册与入口', () => {
  const pagesJson = JSON.parse(read('pages.json'));

  it('四页都在 pages/acceptance 子包里，走自定义导航', () => {
    const pack = (pagesJson.subPackages || []).find((p) => p.root === 'pages/acceptance');
    expect(pack).toBeTruthy();
    expect(pack.pages.map((p) => p.path)).toEqual(PAGES);
    expect(pack.pages.every((p) => p.style.navigationStyle === 'custom')).toBe(true);
  });

  it('验收中心不在 tabBar 里（从「我的」进入的功能页）', () => {
    const tabs = (pagesJson.tabBar && pagesJson.tabBar.list ? pagesJson.tabBar.list : []).map((t) => t.pagePath);
    PAGES.forEach((p) => {
      expect(tabs).not.toContain('pages/acceptance/' + p);
    });
  });

  it('「我的」工具宫格有入口，路径指向列表页，图标文件真实存在', () => {
    const profile = read('pages/profile/profile.uvue');
    expect(profile).toContain("key: 'acceptance'");
    expect(profile).toContain("title: '验收中心'");
    expect(profile).toContain("path: '/pages/acceptance/acceptance-list'");
    expect(profile).toContain("icon: 'acceptance'");
    expect(exists('static/icons/profile/acceptance.png')).toBe(true);
  });
});

describe('分层接线（页面 → composable → api）', () => {
  it('页面只 import composable，不直连 api / 不裸发请求', () => {
    PAGES.forEach((p) => {
      const src = read(pageRel(p));
      expect(src).toMatch(/from '\.\/composables\/useAcceptance/);
      expect(src).not.toMatch(/from '\.\.\/\.\.\/\.\.\/api\/acceptance'/);
      expect(src).not.toContain('uni.request(');
      expect(src).not.toContain('getMapped(');
      expect(src).not.toContain('postMapped(');
    });
  });

  it('四个 composable 都在，且提交页的状态层调用统一校验函数', () => {
    const submit = read('pages/acceptance/composables/useAcceptanceSubmit.uts');
    expect(submit).toContain("from '../../../utils/acceptanceForm'");
    expect(submit).toContain('acceptanceValidate(items.value)');
    for (const f of ['useAcceptanceList.uts', 'useAcceptanceDetail.uts', 'useAcceptanceSubmit.uts', 'useAcceptanceHistory.uts']) {
      expect(exists('pages/acceptance/composables/' + f)).toBe(true);
    }
  });

  it('API 只有一个 mock 开关，且真实路径与 mock 数据源同时在场', () => {
    const api = read('api/acceptance.uts');
    const switches = api.match(/export const ACCEPTANCE_USE_MOCK/g) || [];
    expect(switches.length).toBe(1);
    for (const url of ["'/tasks'", "'/tasks/'", "'/history'", "'/upload'", "'/submit'"]) {
      expect(api).toContain(url);
    }
    for (const fn of ['mockAcceptanceTasks', 'mockAcceptanceDetail', 'mockAcceptanceHistory', 'mockAcceptanceSubmit', 'mockAcceptanceUpload']) {
      expect(api).toContain(fn);
    }
    expect(exists('api/acceptanceMock.uts')).toBe(true);
  });

  it('mock 只被 API 层引用（页面与 composable 不直接碰 mock）', () => {
    const files = [
      ...PAGES.map(pageRel),
      'pages/acceptance/composables/useAcceptanceList.uts',
      'pages/acceptance/composables/useAcceptanceDetail.uts',
      'pages/acceptance/composables/useAcceptanceSubmit.uts',
      'pages/acceptance/composables/useAcceptanceHistory.uts',
    ];
    files.forEach((f) => {
      expect(read(f)).not.toContain('acceptanceMock');
    });
  });

  it('类型从 barrel 再导出（页面/composable 只认 types/index）', () => {
    const barrel = read('types/index.uts');
    for (const t of ['AcceptanceTask', 'AcceptanceTaskDetail', 'AcceptanceItem', 'AcceptanceProof', 'AcceptanceSubmitItem', 'AcceptanceSubmitResult', 'AcceptanceHistoryItem']) {
      expect(barrel).toContain(t);
    }
  });
});

describe('三态与关键交互', () => {
  it('列表 / 详情 / 历史都有加载、失败重试、空态', () => {
    const list = read(pageRel('acceptance-list'));
    expect(list).toContain('v-if="loading"');
    expect(list).toContain('failed');
    expect(list).toContain('暂无待验收任务');
    expect(list).toContain('验收历史');

    const detail = read(pageRel('acceptance-detail'));
    expect(detail).toContain('v-if="loading"');
    expect(detail).toContain('@click="reload"');
    expect(detail).toContain('验收单不存在');
    expect(detail).toContain('开始验收');

    const history = read(pageRel('acceptance-history'));
    expect(history).toContain('v-if="loading"');
    expect(history).toContain('@click="onRetry"');
    expect(history).toContain('还没有提交过验收');
  });

  it('提交页：逐项三种结论 + 备注输入 + 图片/文件凭证 + 防重复提交', () => {
    const src = read(pageRel('acceptance-submit'));
    expect(src).toContain('v-for="opt in options"');
    expect(src).toContain('@click="onSelectResult(item.id, opt.value)"');
    expect(src).toContain('<textarea');
    expect(src).toContain(':maxlength="remarkMax"');
    expect(src).toContain('@input="onRemarkInput(item.id, $event)"');
    expect(src).toContain('@click="onAddProof(item.id)"');
    expect(src).toContain("'上传中...' : '+ 添加凭证'");
    expect(src).toContain("p.kind == 'image'");
    expect(src).toContain('提交验收结果');
    expect(src).toContain("submitting ? '提交中...'");

    const composable = read('pages/acceptance/composables/useAcceptanceSubmit.uts');
    expect(composable).toContain('chooseImage');
    expect(composable).toContain('chooseMessageFile');
    expect(composable).toContain('itemList: [\'拍照 / 选择图片\', \'选择文件\']');
    expect(composable).toContain('if (submitting.value) return false');
  });

  it('mock 侧对已提交单据再拒一次（双保险）', () => {
    expect(read('api/acceptanceMock.uts')).toContain('请勿重复提交');
  });

  it('网络图片位带 :webp（ADR-0066）', () => {
    const src = read(pageRel('acceptance-submit'));
    expect(src).toMatch(/<image[\s\S]*?:src="p\.url"[\s\S]*?:webp="true"/);
  });
});

describe('uvue 样式红线（原生端只认这些）', () => {
  const banned = [/\bgap\s*:/, /calc\(/, /var\(--/, /\btransition\s*:/, /\banimation\s*:/, /env\(/, /:hover/, /:first-child/];

  PAGES.forEach((p) => {
    it(pageRel(p) + '：scss + 只写 class 选择器 + 无原生端不支持属性', () => {
      const src = read(pageRel(p));
      const { attrs, body } = styleBlock(src);
      expect(attrs).toContain('lang="scss"');
      expect(body.length).toBeGreaterThan(0);
      banned.forEach((re) => expect(body).not.toMatch(re));

      for (const m of body.matchAll(/([^{}]+)\{/g)) {
        const selector = m[1].trim();
        if (selector.startsWith('@')) continue;
        selector.split(',').forEach((s) => {
          expect(s.trim().startsWith('.')).toBe(true);
        });
      }
    });

    it(pageRel(p) + '：模板 class 与样式定义双向对得上', () => {
      const src = read(pageRel(p));
      const used = templateClasses(src);
      const defined = definedClasses(styleBlock(src).body);
      const missing = [...used].filter((c) => !defined.has(c));
      const unused = [...defined].filter((c) => !used.has(c));
      expect(missing).toEqual([]);
      expect(unused).toEqual([]);
    });
  });
});
