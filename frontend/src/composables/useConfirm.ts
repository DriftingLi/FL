import { ElMessageBox } from 'element-plus'

/**
 * 确认框统一入口：业务代码**禁止直接调用 ElMessageBox**（AGENTS.md「UI 词汇·确认框」约定）。
 *
 * 三个方法对应三种场景：
 * - `confirm`：普通确认（退出登录、退出练习等中性操作）
 * - `confirmDanger`：危险 / 不可逆操作（删除、清空、移除、驳回、撤销类）
 *   —— 红色确认钮 + `autofocus: false`（初始焦点不落确认钮，连按回车不会误执行）
 * - `prompt`：带输入的确认（如驳回投稿填原因），透传 `ElMessageBox.prompt`
 *
 * 只注入统一默认值，不裁剪能力：调用方传入的 options 覆盖默认值，
 * `ElMessageBox` 的其余选项（`distinguishCancelAndClose`、`showCancelButton` 等）原样可用。
 */
const BASE = {
  confirmButtonText: '确定',
  cancelButtonText: '取消',
}

export function useConfirm() {
  /** 普通确认：中性操作（退出登录、退出练习等） */
  function confirm(message: string, title?: string, options: Record<string, unknown> = {}) {
    return ElMessageBox.confirm(message, title ?? '提示', {
      ...BASE,
      type: 'warning',
      ...options,
    })
  }

  /** 危险 / 不可逆操作：删除、清空、移除、驳回、撤销类 */
  function confirmDanger(message: string, title?: string, options: Record<string, unknown> = {}) {
    return ElMessageBox.confirm(message, title ?? '危险操作', {
      ...BASE,
      type: 'warning',
      confirmButtonText: '删除',
      confirmButtonClass: 'el-button--danger',
      autofocus: false,
      ...options,
    })
  }

  /** 带输入的确认，透传 `ElMessageBox.prompt` */
  function prompt(message: string, title?: string, options: Record<string, unknown> = {}) {
    return ElMessageBox.prompt(message, title ?? '提示', {
      ...BASE,
      ...options,
    })
  }

  return { confirm, confirmDanger, prompt }
}
