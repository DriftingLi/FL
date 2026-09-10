// useRejectReasonDialog：三域审核共有的「驳回理由弹窗」状态机（#795，ADR-0039）。
//
// 背景（CONTEXT.md「审核（review）」词条）：admin 的「审核」是过载词 —— 题库 / 资料 / 投稿
// 三个状态机互不相同（题库驳回回 draft 可再编辑、资料审核为整型状态、投稿五态）。
// 三域**唯一同构**的是 `reject_reason` 字段与它的 UI 形态：理由输入弹窗 + 确认按钮 loading。
//
// 本件只拥有弹窗自身状态（visible / reason / submitting）与「理由为空不提交」的校验；
// **提交动作由调用方注入**（onSubmit），单条 / 批量 / 详情页触发的差异留在各页闭包里。
// 提交失败时**保留已填理由**并保持弹窗打开，便于修改后重试。
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

export interface RejectReasonDialogOptions {
  /** 提交动作：收到理由文本，由调用方决定调哪个接口（单条 / 批量 / 详情页） */
  onSubmit: (reason: string) => Promise<void> | void
  /** 理由为空时是否拦截提交（默认 true，与三页既有行为一致） */
  requireReason?: boolean
  /** 理由为空时的提示文案 */
  emptyMessage?: string
}

export function useRejectReasonDialog(options: RejectReasonDialogOptions) {
  const visible = ref(false)
  const reason = ref('')
  const submitting = ref(false)

  /** 打开弹窗：清空上一次的理由，避免残留串到新的目标 */
  function open(): void {
    reason.value = ''
    visible.value = true
  }

  /** 关闭弹窗（提交中禁止关闭，避免状态错乱） */
  function close(): void {
    if (submitting.value) return
    visible.value = false
  }

  /** 提交：空理由拦截；成功关弹窗，失败保留理由并保持打开 */
  async function submit(): Promise<void> {
    if (submitting.value) return
    const text = reason.value
    if ((options.requireReason ?? true) && !text.trim()) {
      ElMessage.warning(options.emptyMessage ?? '请填写驳回理由')
      return
    }
    submitting.value = true
    try {
      await options.onSubmit(text)
      visible.value = false
    } catch {
      // 错误提示由调用方/拦截器负责；此处保留理由与弹窗，便于修改后重试
    } finally {
      submitting.value = false
    }
  }

  return { visible, reason, submitting, open, close, submit }
}
