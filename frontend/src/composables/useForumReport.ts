/**
 * 论坛举报流程（主题 / 回复共用）：对话框四态 + 理由校验 + 提交。
 *
 * 收编动因：帖子详情与章节讨论各写了一遍同样的 reportVisible/reason/submitting/target
 * 与 submitReport，两处迟早漂移成「一边限 500 字、另一边不限」。边界与文案口径收在这里一处。
 *
 * 用法：解构出的 ref 在模板里可自动解包 ——
 * `const { visible: reportVisible, reason: reportReason, ... } = useForumReport()`。
 */
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi } from '@/api/forum'

export type ForumReportTarget = { kind: 'topic' | 'reply'; id: number }

export function useForumReport() {
  const visible = ref(false)
  const reason = ref('')
  const submitting = ref(false)
  const target = ref<ForumReportTarget | null>(null)

  /** 打开举报对话框。id 由调用方给（主题传主题 id、回复传回复 id），本模块不读路由。 */
  function open(kind: 'topic' | 'reply', id: number) {
    target.value = { kind, id }
    reason.value = ''
    visible.value = true
  }

  async function submit() {
    const text = reason.value.trim()
    if (!target.value) return
    if (text.length < 1 || text.length > 500) {
      ElMessage.warning('举报理由需为 1-500 字')
      return
    }
    submitting.value = true
    try {
      const { kind, id } = target.value
      if (kind === 'topic') await forumApi.reportTopic(id, text)
      else await forumApi.reportReply(id, text)
      ElMessage.success('举报已提交，等待处理')
      visible.value = false
    } catch (e) {
      console.error('举报失败:', e)
      /* 错误已由拦截器提示 */
    } finally {
      submitting.value = false
    }
  }

  return { visible, reason, submitting, open, submit }
}
