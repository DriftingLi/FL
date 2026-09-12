<script setup lang="ts">
/**
 * UGC 内容渲染单点（ADR-0044 / #877）。
 *
 * 全仓**唯一的学员不可信内容渲染路径**。任何要展示帖子正文或回复正文的地方都必须经过这里，
 * 不要各自接 `v-html` 或各自引 markdown 库——安全策略只能有一个落点。
 *
 * 为什么是 markstream 而不是 marked + v-html：
 *   1. 仓库已有该依赖，AI 域已确立「不再有裸 v-html」的收敛约定（`chat-page-shell` 的注释），
 *      学员 UGC 与 AI 输出同属不可信输入，走同一条路既一致又免新依赖；
 *   2. 库文档明确推荐 `html-policy="escape"` 用于不可信 UGC，内置 tag 阻断与属性净化。
 *
 * `escape` 的语义是**把 raw HTML 渲染为文本**而不是丢弃：
 * 这对内容治理是好事——管理员能看到别人究竟写了 `<script>` 字面量，而不是被静默吞掉。
 * 也正因如此，它**不是**内容过滤器，违规内容仍靠举报与管理员删帖治理。
 *
 * `breaks` 不可省：markdown 默认把单个换行折叠成空格，而存量帖子是多行纯文本——
 * 不开它，历史内容会被压成一行（肉眼可见的退化）。
 * 它属于 markdown-it 的实例选项（`MarkdownItOptions`），不在 `ParseOptions` 里，
 * 所以只能经 `customMarkdownIt` 注入——`parseOptions` 传它会被类型检查挡下。
 */
import { computed } from "vue"
import MarkdownRender from "markstream-vue"
import "markstream-vue/index.css"
import { isUnsafeHtmlUrl, type MarkdownIt, type ParsedNode } from "stream-markdown-parser"

/** 「即将离开本站」中转页路径（站外链接一律经它，不直接把读者带走） */
const LINK_OUT_PATH = "/training/link-out"

/** 节点结构的最小视图：解析器的联合类型太大，这里按需取字段做改写。 */
interface LinkishNode {
  type: string
  href?: string
  text?: string
  content?: string
  children?: LinkishNode[]
  items?: LinkishNode[]
}

/**
 * 链接改写（ADR-0044）：
 *   - 伪协议（javascript: 等）→ **就地展开成纯文本**，绝不渲染成可点链接；
 *   - linkPolicy=plain（管理端治理预览）→ 同样展开成纯文本：治理面看的是内容本身，
 *     顺带去掉一处「管理员在后台点到外站」的入口；
 *   - 站内（以 / 或 # 开头）→ 直连；
 *   - 其余站外 → 指向中转页，目标地址以参数带上。
 *
 * 用 flatMap 把链接**就地替换**成它的行内内容而不是 `content` 字符串：
 * 这样 `[**重点**](url)` 里的加粗不会被吃掉。
 *
 * 幂等：中转页地址以 / 开头，二次处理时按站内放行；plain 处理后已无链接节点。
 * 故即使解析器对每层都调用一次本钩子，结果也一致。
 */
function rewriteLinks(nodes: LinkishNode[], policy: "transit" | "plain"): LinkishNode[] {
  return nodes.flatMap((node) => {
    const children = node.children ? rewriteLinks(node.children, policy) : undefined
    const items = node.items ? rewriteLinks(node.items, policy) : undefined
    if (node.type !== "link") return [{ ...node, children, items }]

    const href = node.href ?? ""
    const inline = children?.length ? children : [{ type: "text", content: node.text ?? "" }]
    if (!href || isUnsafeHtmlUrl(href) || policy === "plain") return inline
    if (href.startsWith("/") || href.startsWith("#")) return [{ ...node, href, children: inline }]
    return [{ ...node, href: `${LINK_OUT_PATH}?url=${encodeURIComponent(href)}`, children: inline }]
  })
}

/**
 * 论坛正文的 markdown-it 配置。
 *
 * 只改 `breaks`：单换行即换行（存量多行纯文本不被折叠）。
 * 安全相关的 `validateLink` 等由库按 `html-policy` 自行处理，此处不覆盖——
 * 减少本组件对安全策略的干预面，策略只由 `html-policy` 一个旋钮表达。
 */
function configureForumMarkdown(md: MarkdownIt): MarkdownIt {
  return md.set({ breaks: true })
}

const props = withDefaults(
  defineProps<{
    content: string
    /**
     * 正文格式声明（ADR-0044）。缺省 `text`——与后端 `normalizeContentFormat` 的归一一致，
     * 这样不带该字段的调用方（含尚未适配的客户端过来的数据）按纯文本渲染，不会误解语法。
     */
    format?: "text" | "markdown"
    /**
     * 链接策略（ADR-0044）：
     *   transit（默认）站外链接经「即将离开本站」中转页；
     *   plain 链接一律渲染为纯文本——管理端治理预览用（看内容本身，不做导航）。
     */
    linkPolicy?: "transit" | "plain"
  }>(),
  { format: "text", linkPolicy: "transit" }
)

const isMarkdown = computed(() => props.format === "markdown")

/** 链接治理在 AST 层做（见 rewriteLinks），不做 DOM 事后改写。 */
const parseOptions = computed(() => ({
  final: true,
  postTransformNodes: (nodes: ParsedNode[]) =>
    rewriteLinks(nodes as unknown as LinkishNode[], props.linkPolicy) as unknown as ParsedNode[]
}))
</script>

<!--
  ⚠️ 模板**必须只有一个根节点**：本组件靠 attrs 透传接收调用方的排版类
  （字号/行高/颜色）。根节点之上再放注释或兄弟节点会让组件变成 fragment，
  attrs 透传静默失效——调用方传的类全部不生效，且不报错。
-->
<template>
  <div
    class="forum-content break-words"
    :class="isMarkdown ? 'forum-content--markdown' : 'whitespace-pre-wrap'"
  >
    <MarkdownRender
      v-if="isMarkdown"
      mode="docs"
      :content="content"
      :final="true"
      html-policy="escape"
      :parse-options="parseOptions"
      :custom-markdown-it="configureForumMarkdown"
      :fade="false"
    />
    <template v-else>{{ content }}</template>
  </div>
</template>
