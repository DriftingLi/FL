// 「即将离开本站」中转页（#881 / ADR-0044）。
//
// 目标地址来自 query，是**用户可控输入**——本文件的重点是它不被当成 HTML。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const push = vi.fn()
const back = vi.fn()
vi.mock("vue-router", () => ({
  useRoute: () => ({ query: currentQuery }),
  useRouter: () => ({ push, back })
}));

// useRoute 的替身读这个可变对象，逐例改写
let currentQuery: Record<string, string> = {}

import LinkOutPage from '../LinkOutPage.vue'

function mountPage(query: Record<string, string>) {
  currentQuery = query;
  return mount(LinkOutPage, { global: { plugins: [epLite()] } });
}

describe('外链中转页（#881 / ADR-0044）', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    delete (window as unknown as Record<string, unknown>).__pwned;
  });
  afterEach(() => {
    delete (window as unknown as Record<string, unknown>).__pwned;
  });

  it('合法站外地址：展示目标 + 继续访问带安全属性', async () => {
    const w = mountPage({ url: 'https://example.com/doc' });
    await flushPromises();
    expect(w.text()).toContain('即将离开本站');
    expect(w.text()).toContain('https://example.com/doc');
    const link = w.find('a');
    expect(link.attributes('href')).toBe('https://example.com/doc');
    expect(link.attributes('target')).toBe('_blank');
    // rel 三件套缺一不可：noopener 防 window.opener 反控，noreferrer 防来源泄漏，nofollow 表明非背书
    expect(link.attributes('rel')).toContain('noopener');
    expect(link.attributes('rel')).toContain('noreferrer');
    expect(link.attributes('rel')).toContain('nofollow');
  });

  it('伪协议地址：不提供继续访问，只给返回', async () => {
    for (const bad of ['javascript:window.__pwned=1', 'data:text/html,<script>1</script>', '']) {
      const w = mountPage({ url: bad });
      await flushPromises();
      expect(w.find('a').exists()).toBe(false);
      expect(w.text()).toContain('无效');
      expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined();
    }
  });

  it('目标地址按文本渲染，不解析为 HTML（中转页不是第二个注入面）', async () => {
    const w = mountPage({ url: 'https://example.com/<script>window.__pwned=1</script>' });
    await flushPromises();
    expect(w.element.querySelector('script')).toBeNull();
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined();
    expect(w.text()).toContain('<script>');
  });
});
