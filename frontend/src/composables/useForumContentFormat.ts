/**
 * 记住学员上次选的正文格式（ADR-0044 / #878）。
 *
 * 首次（无记录）默认 **纯文本**：不主动改变任何人的表达，Markdown 完全 opt-in。
 * 之后跟随该用户上次的选择——「记住」在这里是省事机制，不是默认机制。
 *
 * 存本地而非账号：
 *   - 这是**客户端表达习惯**，不是业务事实，没必要进用户表；
 *   - 移动端本批不做 markdown 输入，也就不存在跨端共享该偏好的需求（ADR-0044 已记）。
 *
 * 发帖与回复**共用同一个键**：同一个人对正文格式的偏好与他写的是主题还是回复无关，
 * 分两个键只会出现「发帖选了 Markdown、回复却还是纯文本」这种莫名其妙的不一致。
 *
 * localStorage 读写一律 try/catch：隐私模式、存储配额满、被策略禁用时退化为「不记忆」，
 * 而不是让发帖表单整个炸掉。
 */
import { ref, watch } from "vue"
import type { ForumContentFormat } from "@/api/forum"

const STORAGE_KEY = "forum:content-format"

function readStoredFormat(): ForumContentFormat {
  try {
    return localStorage.getItem(STORAGE_KEY) === "markdown" ? "markdown" : "text"
  } catch {
    return "text"
  }
}

export function useForumContentFormat() {
  const format = ref<ForumContentFormat>(readStoredFormat())

  watch(format, (next) => {
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      /* 存储不可用：忽略，退化为不记忆 */
    }
  })

  return { format }
}
