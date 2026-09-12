/**
 * 正文格式的**偏好 + 切换态**（ADR-0044 / #878 #879）。
 *
 * 偏好（persisted）：
 *   首次（无记录）默认 **纯文本**：不主动改变任何人的表达，Markdown 完全 opt-in。
 *   之后跟随该用户上次的选择——「记住」在这里是省事机制，不是默认机制。
 *   存本地而非账号：这是**客户端表达习惯**，不是业务事实；移动端本批不做 markdown 输入，
 *   也就不存在跨端共享该偏好的需求（ADR-0044 已记）。
 *   发帖与回复**共用同一个键**：同一个人对正文格式的偏好与他写的是主题还是回复无关，
 *   分两个键只会出现「发帖选了 Markdown、回复却还是纯文本」这种莫名其妙的不一致。
 *
 * 切换态（UI）：选项表、是否 Markdown、是否在预览、切换处理，
 *   发帖表单与回复框两处共用——它们必须是同一套控件与同一套口径，各写一份迟早分叉。
 *
 * localStorage 读写一律 try/catch：隐私模式、配额满、被策略禁用时退化为「不记忆」，
 * 而不是让发帖表单整个炸掉。
 */
import { computed, ref, watch } from "vue"
import type { ForumContentFormat } from "@/api/forum"

const STORAGE_KEY = "forum:content-format"

/** 格式选项（发帖与回复共用的同一张表） */
export const FORUM_FORMAT_OPTIONS: Array<{ label: string; value: ForumContentFormat }> = [
  { label: "纯文本", value: "text" },
  { label: "Markdown", value: "markdown" }
]

function readStoredFormat(): ForumContentFormat {
  try {
    return localStorage.getItem(STORAGE_KEY) === "markdown" ? "markdown" : "text"
  } catch {
    return "text"
  }
}

export function useForumContentFormat() {
  const format = ref<ForumContentFormat>(readStoredFormat())
  const isMarkdown = computed(() => format.value === "markdown")
  const previewing = ref(false)

  watch(format, (next) => {
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      /* 存储不可用：忽略，退化为不记忆 */
    }
  })

  function handleFormatChange(v: string) {
    format.value = v === "markdown" ? "markdown" : "text"
    // 切回纯文本时退出预览：纯文本没有可预览的渲染结果
    if (!isMarkdown.value) previewing.value = false
  }

  /** 提交后复位预览：父级会清空正文，停在预览态只会让用户看到一块空预览 */
  function resetPreview() {
    previewing.value = false
  }

  return { format, isMarkdown, previewing, handleFormatChange, resetPreview }
}
