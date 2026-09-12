// UGC 内容渲染单点（ADR-0044 / #877）。
//
// 这是全仓第一条**学员 UGC** 的渲染路径，本文件存在的首要理由是那条安全闸门。
//
// 与 AI 域既有用例（chat-page-shell.spec）的关键差别：那条把 markstream stub 掉、只断言
// htmlPolicy 这个 **prop 被传对了**；本文件**不 stub**，断言的是真实行为——
// 「把 script 标签发进去，读出来没有 script 元素，且这段字面量以文本可见」。
// 断言 prop 的做法无法发现「prop 传对了但库的默认策略变了 / 被别处覆盖了」。
import { describe, it, expect, vi, afterEach } from "vitest"
import { mount } from "@vue/test-utils"
import { epLite } from "@/test/element-lite"
import ForumContent from "../ForumContent.vue"

// CSS 导入在 vitest 下无意义，替身掉（与 AI 域既有做法一致）
vi.mock("markstream-vue/index.css", () => ({}))

afterEach(() => {
  delete (window as unknown as Record<string, unknown>).__pwned
})

function mountContent(content: string, format: "text" | "markdown" = "markdown") {
  // 按仓库约定装 epLite()（本组件当前不直接用 EP 组件，但约定是「新建 spec 一律用」）
  return mount(ForumContent, { props: { content, format }, global: { plugins: [epLite()] } })
}

describe("ForumContent 安全闸门（UGC 不可信输入）", () => {
  it("markdown 里的 script 标签不执行，且以文本形式可见", () => {
    const w = mountContent("正常文字\n\n<script>window.__pwned = 1</script>\n\n结尾")
    // 不执行：没有任何脚本被运行
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    // 不生成 script 元素
    expect(w.element.querySelector("script")).toBeNull()
    // 也不该被静默吞掉：字面量应当可见（治理面要能看到别人写了什么）
    expect(w.text()).toContain("<script>")
  })

  it("img onerror 不产生带事件处理器的元素", () => {
    const w = mountContent('<img src=x onerror="window.__pwned = 2">')
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    expect(w.element.querySelector("img[onerror]")).toBeNull()
    expect(w.element.querySelector("[onerror]")).toBeNull()
  })

  it("svg onload 等其它事件属性同样不落地", () => {
    const w = mountContent('<svg onload="window.__pwned = 3"></svg><iframe src="javascript:window.__pwned=4"></iframe>')
    expect((window as unknown as Record<string, unknown>).__pwned).toBeUndefined()
    expect(w.element.querySelector("[onload]")).toBeNull()
    expect(w.element.querySelector("iframe")).toBeNull()
  })
})

describe("ForumContent 外链治理（#881 / ADR-0044）", () => {
  /** 取渲染结果里所有 <a> 的 href */
  function hrefsOf(w: ReturnType<typeof mountContent>) {
    return w.findAll('a').map((a) => a.attributes('href') ?? '')
  }

  it('站外链接指向「即将离开本站」中转页，而不是直接指向外站', () => {
    const w = mountContent('见 [手册](https://example.com/doc)')
    const hrefs = hrefsOf(w)
    expect(hrefs.length).toBe(1)
    // 指向中转页而非外站原址（目标地址作为参数携带，故编码后当然含域名——
    // 这里断言的是「href 的起点是中转页路径」，而不是「不含域名」）
    expect(hrefs[0].startsWith('/training/link-out?url=')).toBe(true)
    expect(decodeURIComponent(hrefs[0])).toContain('https://example.com/doc')
    // 相对本站的站外绝对地址同样经中转
    const w2 = mountContent('见 [外站](http://other.example.org/x)')
    expect(w2.findAll('a')[0].attributes('href')?.startsWith('/training/link-out?url=')).toBe(true)
  })

  it('站内链接原样直连，不经中转（不无谓打断站内浏览）', () => {
    const w = mountContent('见 [课程](/training/courses/1)')
    expect(hrefsOf(w)).toEqual(['/training/courses/1'])
  })

  it('伪协议链接不得渲染成可点击链接', () => {
    const w = mountContent('[点我](javascript:window.__pwned=1)')
    expect(hrefsOf(w).some((h) => h.toLowerCase().startsWith('javascript:'))).toBe(false)
  })

  it('linkPolicy=plain：链接渲染为纯文本（管理端治理预览用，不做导航）', () => {
    const w = mount(ForumContent, {
      props: { content: '见 [手册](https://example.com/doc)', format: 'markdown', linkPolicy: 'plain' },
      global: { plugins: [epLite()] }
    })
    expect(w.findAll('a').length).toBe(0)
    expect(w.text()).toContain('手册')
  })
})

describe("ForumContent 子集边界（#876 / ADR-0044）", () => {
  /**
   * 声明子集是「标题/有序无序列表/加粗/行内代码/代码块/引用/链接」，
   * 表格与脚注**不在内**——解析器默认却有，不关掉就成了「声称受限、实则不限」。
   */
  it("表格不渲染成表格（不在声明子集内，且移动端没有对应块渲染能力）", () => {
    const w = mountContent("| a | b |\n| - | - |\n| 1 | 2 |");
    expect(w.element.querySelector("table")).toBeNull();
  });

  it("正文内嵌图片展开成 alt 文本（图片走 images 数组，图文分离）", () => {
    const w = mountContent("见 ![故障图](https://example.com/a.png)");
    expect(w.element.querySelector("img")).toBeNull();
    expect(w.text()).toContain("故障图");
  });
});

describe("ForumContent 两种格式的渲染差异", () => {
  it("format=text：不解释语法，保留换行", () => {
    const w = mountContent("# 不是标题\n**不是加粗**", "text")
    expect(w.text()).toContain("# 不是标题")
    expect(w.text()).toContain("**不是加粗**")
    expect(w.element.querySelector("h1")).toBeNull()
    expect(w.element.querySelector("strong")).toBeNull()
    // 换行靠 CSS 保留（whitespace-pre-wrap），不是靠 <br>
    expect(w.classes()).toContain("whitespace-pre-wrap")
  })

  it("format=markdown：渲染出结构（标题 / 加粗 / 行内代码 / 列表 / 代码块）", () => {
    const w = mountContent("## 排查步骤\n\n- **先断电**\n- 量 `E01` 电压\n\n```\n故障码 E01\n```")
    const html = w.html()
    expect(html).toMatch(/<h2/i)
    expect(html).toMatch(/<strong/i)
    expect(html).toMatch(/<code/i)
    expect(html).toMatch(/<li/i)
    expect(w.text()).toContain("先断电")
  })

  it("format=markdown：单换行即换行（breaks，存量多行纯文本不被折叠）", () => {
    const w = mountContent("第一行\n第二行")
    // breaks 语义下两行不会并成一行——以渲染文本是否含分隔来判断（不断言具体标签）
    const text = w.text()
    expect(text).toContain("第一行")
    expect(text).toContain("第二行")
    expect(text).not.toContain("第一行第二行")
  })

  it("format 缺省按 text 处理（向后兼容：不带该字段的客户端）", () => {
    const w = mount(ForumContent, { props: { content: "# 原样" } })
    expect(w.text()).toContain("# 原样")
    expect(w.element.querySelector("h1")).toBeNull()
  })
})
