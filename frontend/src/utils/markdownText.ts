/**
 * Markdown → 纯文本投影（#880 / ADR-0044）。
 *
 * 用途：列表摘要等**只需要纯文本**的地方。
 *
 * 为什么不是正则剥标记：正则与渲染器是两套规则，代码块内的符号、转义、嵌套标记都会与
 * 真实渲染不一致，长期必然漂移（典型症状：代码块里的 `*` 被当成强调吃掉）。
 * 本实现用**与渲染同一个解析器**（`parseMarkdownToStructure`）拿节点流再投影，
 * 因此「列表摘要显示的」与「点进去看到的」同源。
 *
 * 另一条设计取舍：列表页**不挂渲染组件**。一页十几条帖子若每条都实例化一个渲染器，
 * 开销与首屏时间都不划算，而摘要本来也不需要排版——投影成一行纯文本即可。
 */
import { getMarkdown, parseMarkdownToStructure } from "stream-markdown-parser"

/** 节点结构的最小视图：解析器的联合类型太大，这里按需取字段，避免逐类型窄化。 */
interface PlainNode {
  type: string
  content?: string
  text?: string
  code?: string
  alt?: string
  raw?: string
  children?: PlainNode[]
  items?: PlainNode[]
}

/**
 * 块级容器：其子节点之间要有分隔，否则相邻段落会粘成一个词。
 * 行内容器（paragraph / heading / link / strong …）反之必须紧贴。
 */
const BLOCK_CONTAINERS = new Set(["root", "blockquote", "list_item", "table", "table_row"])

function project(node: PlainNode): string {
  switch (node.type) {
    case "text":
      return node.content ?? ""
    // 代码块与行内代码**逐字**保留：块内的 # 与 ** 不是语法，这正是正则方案做不对的地方
    case "code_block":
    case "inline_code":
      return node.code ?? ""
    case "image":
      return node.alt ?? ""
    case "thematic_break":
      return ""
    case "hard_break":
      return "\n"
    // 与 escape 渲染口径一致：raw HTML 在正文里是**可见文本**，摘要也不该把它藏起来
    case "html_block":
    case "html_inline":
      return node.raw ?? ""
    default: {
      if (node.items) return node.items.map(project).join(" ")
      if (node.children) {
        const sep = BLOCK_CONTAINERS.has(node.type) ? " " : ""
        return node.children.map(project).join(sep)
      }
      return node.text ?? node.raw ?? ""
    }
  }
}

/** 把 markdown 源串投影成单行纯文本（供列表摘要等场景）。空输入返回空串。 */
export function markdownToPlainText(src: string): string {
  if (!src || !src.trim()) return ""
  // 这里**不需要**传 breaks：单换行无论被解析成软换行还是硬换行，
  // 最后都会被下面的 \s+ 折叠成单个空格，结果一致（breaks 属于 markdown-it 实例选项，
  // 也不在 ParseOptions 里）。final: true 表示输入已完整，避免未闭合标记被当成中间态。
  const nodes = parseMarkdownToStructure(src, getMarkdown(), { final: true }) as unknown as PlainNode[]
  return project({ type: "root", children: nodes }).replace(/\s+/g, " ").trim()
}
