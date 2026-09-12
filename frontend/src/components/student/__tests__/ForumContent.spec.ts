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
import ForumContent from "../ForumContent.vue"

// CSS 导入在 vitest 下无意义，替身掉（与 AI 域既有做法一致）
vi.mock("markstream-vue/index.css", () => ({}))

afterEach(() => {
  delete (window as unknown as Record<string, unknown>).__pwned
})

function mountContent(content: string, format: "text" | "markdown" = "markdown") {
  return mount(ForumContent, { props: { content, format } })
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
