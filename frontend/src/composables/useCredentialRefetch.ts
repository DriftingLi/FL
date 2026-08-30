import { watch } from 'vue'
import { useCredentialStore } from '@/stores/credential'

/**
 * 证件切换即重拉（#387 单点）。
 *
 * 内部 watch credential store 的当前证件 id，替代各页面「事件 + watch」双通知写法
 * （`credential-switched` CustomEvent + 各自 watch store 的重复拉取）。
 *
 * watch 在调用方组件的 setup 作用域内建立，随组件卸载自动销毁 —— 生命周期配对
 * 由 Vue 作用域保证，页面不再手工 add/removeEventListener。
 *
 * Pinia 未激活（如无 store 依赖的页面单测直挂组件）时静默跳过：无证件上下文即无切换信号。
 */
export function useCredentialRefetch(refetch: () => unknown): void {
  let store: ReturnType<typeof useCredentialStore>
  try {
    store = useCredentialStore()
  } catch {
    return
  }
  watch(
    () => store.current?.id,
    () => {
      void refetch()
    }
  )
}
