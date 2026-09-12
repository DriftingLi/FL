/**
 * 发布端渲染单点（ADR-0046 / #901 #903）。
 *
 * 「发布端」指**读者真正读到的那份渲染结果**：Web 学员端的章节正文、门户访客看到的
 * 内容精选、移动端学员看到的正文。本文件是它在 Web 侧的唯一定义处——
 * 学员阅读页与讲师/管理员的编辑页预览**调同一份**，不允许各写一套。
 * 预览与发布同源是这批要买的东西：ADR-0044 当初拒绝「复用 Vditor 当编辑器」的理由
 * 正是「它的预览走自有引擎，会与发布后的渲染结果不一致」，现在把这条理由反过来用。
 *
 * 为什么是两个 Marked 实例，而不是全局 `marked.use(...)`：
 * 内容面按**子集**分档（ADR-0046）。章节正文是可信来源（讲师撰写），能力取全集
 * （表格 + 代码高亮 + 公式）；内容精选是**三端交集**（门户与移动端不渲染表格与公式），
 * 能力取交集。全局实例只有一个能力档位，两个面必然互相污染——这里用两个实例把
 * 「哪个面有什么能力」变成代码里看得见的事实。
 *
 * 章节栈**不变**：仍是 marked + highlight.js + `v-html`（ADR-0044 的可信面口径），
 * 只是多接了一个公式扩展；不引入第二条渲染路径，也不并到 markstream。
 */
import { Marked, type Tokens } from 'marked'
import { markedHighlight } from 'marked-highlight'
import hljs from 'highlight.js'
import markedKatex from 'marked-katex-extension'

/** 内容子集档位：章节（可信面全集）/ 内容精选（三端交集）。 */
export type PublishSubset = 'chapter' | 'featured'

/** 内容精选允许的语法（三端交集）——写给作者看的清单，与 API.md 的契约同源。 */
export const FEATURED_SUBSET_SYNTAX = '标题 / 列表 / 引用 / 代码块 / 图片 / 链接'

/** 越界种类 → 给作者看的名字（也是提示文案里的用词，唯一来源）。 */
const OUTSIDE_SUBSET_LABELS: Record<'table' | 'math', string> = {
  table: '表格',
  math: '公式'
}

/** 子集外语法在预览 / 编辑器提示里的统一说法（#902 / #903）——两处文案由它派生，不各写一份。 */
export function outsideSubsetNotice(labels: readonly string[]): string {
  return '以下语法在门户与移动端不渲染，读者看到的是原始文本：' + labels.join('、')
}

function highlightCode(code: string, lang: string): string {
  if (lang && hljs.getLanguage(lang)) {
    return hljs.highlight(code, { language: lang }).value
  }
  return hljs.highlightAuto(code).value
}

/** 与既有章节渲染完全一致的选项（langPrefix / breaks / gfm 都不动，保证既有语法回归为零）。 */
const HIGHLIGHT = { langPrefix: 'hljs language-', highlight: highlightCode }
const BASE_OPTIONS = { breaks: true, gfm: true }

const chapterMarked = new Marked(
  markedHighlight(HIGHLIGHT),
  // trust: false —— `\href` / `\htmlClass` / `\includegraphics` 这类命令不生效：
  // 渲染输出会进 v-html，公式里的命令不能变成第二条注入路径（ADR-0046 安全口径）。
  markedKatex({ trust: false, throwOnError: false }),
  BASE_OPTIONS
)

const featuredMarked = new Marked(markedHighlight(HIGHLIGHT), BASE_OPTIONS)

/*
 * 内容精选是三端交集：表格与公式**都不渲染**。
 * 公式不需要额外配置——不挂 katex 扩展，`$...$` 自然保持字面量（与移动端一致）。
 * 表格则要显式降级：marked 的 gfm 默认带表格，若原样渲染出一张表，
 * 预览就在骗作者——门户与移动端看到的不是这张表。
 */
featuredMarked.use({
  renderer: {
    table(token: Tokens.Table): string {
      return escapeHtml(String(token.raw).trimEnd()).replace(/\n/g, '<br>')
    }
  }
})

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

/**
 * 发布端渲染：markdown 源串 → HTML 串（调用方 v-html）。
 *
 * `subset` 缺省 `chapter`：绝大多数调用方（学员阅读页、讲师预览、AI 生成预览）
 * 都是可信面的章节档。
 */
export function renderPublishMarkdown(source: string, subset: PublishSubset = 'chapter'): string {
  if (!source) return ''
  const renderer = subset === 'featured' ? featuredMarked : chapterMarked
  return renderer.parse(source) as string
}

/**
 * 三端交集子集越界检测（#902）。
 *
 * 判据走**与渲染同一个解析器**的词法结果（marked 的 token 流），不写正则：
 * 正则是第二套规则，代码块里的竖线、转义、嵌套都会与真实渲染不一致
 * （ADR-0044 对「摘要剥离」已经吃过一次同样的教训）。
 *
 * 检测用**章节档**的思路跑：章节档是超集（表格 + 公式都在），越界项在超集里才看得见。
 */
export function detectOutsideSubset(source: string): string[] {
  if (!source) return []
  // 便宜的预筛：GFM 表格必须有竖线、KaTeX 必须有美元号。两者都不含时直接返回，
  // 不在每次输入（管理端编辑器逐键触发）都对全文跑一遍词法。
  if (!source.includes('|') && !source.includes('$')) return []
  const found = new Set<'table' | 'math'>()
  collectOutsideSubset(chapterMarked.lexer(source) as unknown[], found)
  return [...found].map((kind) => OUTSIDE_SUBSET_LABELS[kind])
}

function collectOutsideSubset(tokens: readonly unknown[], found: Set<'table' | 'math'>): void {
  for (const raw of tokens) {
    if (!raw || typeof raw !== 'object') continue
    const token = raw as { type?: string; tokens?: unknown[]; items?: unknown[] }
    if (token.type === 'table') found.add('table')
    else if (token.type === 'inlineKatex' || token.type === 'blockKatex') found.add('math')
    if (Array.isArray(token.tokens)) collectOutsideSubset(token.tokens, found)
    if (Array.isArray(token.items)) collectOutsideSubset(token.items, found)
  }
}
